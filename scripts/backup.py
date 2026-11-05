#!/usr/bin/env python3
"""Encrypted PostgreSQL backup, verification and isolated restore drill.

Requires pg_dump/pg_restore and age. Uses age public-key encryption (not
passwords in command arguments). Credentials are passed to libpq through a
mode-0600 temporary PGPASSFILE, never through process arguments.
"""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import urllib.parse
import uuid

MANIFEST_VERSION = 1
SAFE_NAME = re.compile(r"^[A-Za-z0-9_.-]{1,63}$")


def _required(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        raise RuntimeError(f"required setting {name} is missing")
    return value


def _pg_config(url: str) -> tuple[dict[str, str], str]:
    parsed = urllib.parse.urlparse(url)
    if parsed.scheme not in ("postgres", "postgresql") or not parsed.hostname:
        raise RuntimeError("database URL is invalid")
    db = urllib.parse.unquote(parsed.path.lstrip("/"))
    if not db or not SAFE_NAME.fullmatch(db):
        raise RuntimeError("database name is invalid for backup tooling")
    env = os.environ.copy()
    env.update({
        "PGHOST": parsed.hostname,
        "PGPORT": str(parsed.port or 5432),
        "PGUSER": urllib.parse.unquote(parsed.username or ""),
        "PGDATABASE": db,
        "PGSSLMODE": urllib.parse.parse_qs(parsed.query).get("sslmode", ["prefer"])[0],
    })
    password = urllib.parse.unquote(parsed.password or "")
    if "\\n" in password or "\\r" in password:
        raise RuntimeError("database password contains unsupported line breaks")
    if password:
        # A private temporary password file avoids credentials in argv and logs.
        passfile = tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", delete=False)
        os.chmod(passfile.name, 0o600)
        host = parsed.hostname.replace("\\", "\\\\").replace(":", "\\:")
        user = env["PGUSER"].replace("\\", "\\\\").replace(":", "\\:")
        pwd = password.replace("\\", "\\\\").replace(":", "\\:")
        passfile.write(f"{host}:{env['PGPORT']}:{db}:{user}:{pwd}\n")
        passfile.close()
        env["PGPASSFILE"] = passfile.name
        env["PGPASSWORD"] = ""
    return env, db


def _run(args: list[str], env: dict[str, str]) -> None:
    subprocess.run(args, env=env, check=True, stdin=subprocess.DEVNULL)


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _cleanup_pgpass(env: dict[str, str]) -> None:
    path = env.get("PGPASSFILE")
    if path:
        try:
            os.unlink(path)
        except FileNotFoundError:
            pass


def create_backup(args: argparse.Namespace) -> None:
    url = _required("DATABASE_URL")
    recipient = _required("BACKUP_AGE_RECIPIENT")
    root = Path(args.output).resolve()
    root.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(root, 0o700)
    env, database = _pg_config(url)
    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    stem = f"{database}-{stamp}-{uuid.uuid4().hex[:8]}"
    encrypted_tmp = root / f".{stem}.dump.age.tmp"
    encrypted = root / f"{stem}.dump.age"
    manifest_tmp = root / f".{stem}.manifest.json.tmp"
    manifest = root / f"{stem}.manifest.json"
    try:
        with tempfile.NamedTemporaryFile(prefix="anitoon-backup-", suffix=".dump", delete=False) as plain:
            plaintext = Path(plain.name)
        os.chmod(plaintext, 0o600)
        try:
            _run(["pg_dump", "--no-password", "--format=custom", "--file", str(plaintext)], env)
            _run(["age", "--recipient", recipient, "--output", str(encrypted_tmp), str(plaintext)], env)
        finally:
            plaintext.unlink(missing_ok=True)
        digest = _sha256(encrypted_tmp)
        metadata = {
            "manifest_version": MANIFEST_VERSION,
            "database": database,
            "created_at": dt.datetime.now(dt.timezone.utc).isoformat(),
            "format": "postgresql-custom-dump",
            "encryption": "age",
            "ciphertext_file": encrypted.name,
            "ciphertext_sha256": digest,
            "application_commit": os.environ.get("APP_COMMIT", "unknown"),
            "schema_migration_version": os.environ.get("EXPECTED_MIGRATION_VERSION", "unknown"),
        }
        with manifest_tmp.open("x", encoding="utf-8") as stream:
            os.chmod(manifest_tmp, 0o600)
            json.dump(metadata, stream, sort_keys=True, indent=2)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(encrypted_tmp, encrypted)
        os.replace(manifest_tmp, manifest)
        print(json.dumps({"backup": encrypted.name, "manifest": manifest.name, "sha256": digest}))
    finally:
        encrypted_tmp.unlink(missing_ok=True)
        manifest_tmp.unlink(missing_ok=True)
        _cleanup_pgpass(env)


def load_manifest(path: Path) -> dict:
    if path.is_symlink() or not path.is_file():
        raise RuntimeError("manifest must be a regular file")
    data = json.loads(path.read_text(encoding="utf-8"))
    required = {"manifest_version", "database", "created_at", "format", "encryption", "ciphertext_file", "ciphertext_sha256"}
    if not required.issubset(data) or data["manifest_version"] != MANIFEST_VERSION:
        raise RuntimeError("backup manifest is incomplete or unsupported")
    name = data["ciphertext_file"]
    if not isinstance(name, str) or Path(name).name != name or not name.endswith(".dump.age"):
        raise RuntimeError("backup manifest contains an invalid archive path")
    if not re.fullmatch(r"[0-9a-f]{64}", str(data["ciphertext_sha256"])):
        raise RuntimeError("backup manifest checksum is invalid")
    return data


def verify_backup(args: argparse.Namespace) -> Path:
    manifest_path = Path(args.manifest).resolve()
    metadata = load_manifest(manifest_path)
    archive = manifest_path.parent / metadata["ciphertext_file"]
    if archive.is_symlink() or not archive.is_file():
        raise RuntimeError("encrypted backup archive is missing or not a regular file")
    actual = _sha256(archive)
    if actual != metadata["ciphertext_sha256"]:
        raise RuntimeError("backup archive SHA-256 does not match its manifest")
    identity = _required("BACKUP_AGE_IDENTITY_FILE")
    if not Path(identity).is_file():
        raise RuntimeError("backup decryption identity file is unavailable")
    with tempfile.NamedTemporaryFile(prefix="anitoon-verify-", suffix=".dump", delete=False) as plain:
        plaintext = Path(plain.name)
    os.chmod(plaintext, 0o600)
    try:
        _run(["age", "--decrypt", "--identity", identity, "--output", str(plaintext), str(archive)], os.environ.copy())
        _run(["pg_restore", "--list", str(plaintext)], os.environ.copy())
    finally:
        plaintext.unlink(missing_ok=True)
    return archive


def restore_drill(args: argparse.Namespace) -> None:
    verify_backup(args)
    confirmation = _required("RESTORE_TARGET_CONFIRMATION")
    if confirmation != "I_CONFIRM_ISOLATED_RESTORE":
        raise RuntimeError("isolated restore confirmation is missing")
    if _required("RESTORE_TARGET_ISOLATED") != "true":
        raise RuntimeError("restore target is not explicitly marked isolated")
    source_url = _required("DATABASE_URL")
    target_url = _required("RESTORE_DATABASE_URL")
    source_env, source_db = _pg_config(source_url)
    target_env, target_db = _pg_config(target_url)
    # Reject same endpoint/database and same database name conservatively.
    if (source_env["PGHOST"], source_env["PGPORT"], source_db) == (target_env["PGHOST"], target_env["PGPORT"], target_db):
        _cleanup_pgpass(source_env)
        _cleanup_pgpass(target_env)
        raise RuntimeError("restore target resolves to the source production database")
    if not target_db.endswith(("_restore", "_drill")):
        _cleanup_pgpass(source_env)
        _cleanup_pgpass(target_env)
        raise RuntimeError("restore target database name must end in _restore or _drill")
    if source_db == target_db and source_env["PGHOST"] == target_env["PGHOST"]:
        _cleanup_pgpass(source_env)
        _cleanup_pgpass(target_env)
        raise RuntimeError("restore target is not sufficiently isolated")
    manifest_path = Path(args.manifest).resolve()
    metadata = load_manifest(manifest_path)
    archive = manifest_path.parent / metadata["ciphertext_file"]
    identity = _required("BACKUP_AGE_IDENTITY_FILE")
    with tempfile.NamedTemporaryFile(prefix="anitoon-restore-", suffix=".dump", delete=False) as plain:
        plaintext = Path(plain.name)
    os.chmod(plaintext, 0o600)
    try:
        _run(["age", "--decrypt", "--identity", identity, "--output", str(plaintext), str(archive)], os.environ.copy())
        # Destructive operation is limited to the separately confirmed target URL.
        _run(["pg_restore", "--no-password", "--clean", "--if-exists", "--no-owner", "--no-privileges", "--exit-on-error", "--dbname", target_db, str(plaintext)], target_env)
        # Run the application's read-only migration verifier against restored DB.
        verify_env = os.environ.copy()
        verify_env["DATABASE_URL"] = target_url
        _run(["go", "run", "./cmd/backup-verify"], verify_env)
        print(json.dumps({"restore_drill": "passed", "target_database": target_db, "archive": archive.name}))
    finally:
        plaintext.unlink(missing_ok=True)
        _cleanup_pgpass(source_env)
        _cleanup_pgpass(target_env)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    create = sub.add_parser("create")
    create.add_argument("--output", default="backups")
    create.set_defaults(func=create_backup)
    verify = sub.add_parser("verify")
    verify.add_argument("manifest")
    verify.set_defaults(func=verify_backup)
    restore = sub.add_parser("restore-drill")
    restore.add_argument("manifest")
    restore.set_defaults(func=restore_drill)
    args = parser.parse_args()
    try:
        args.func(args)
    except (OSError, ValueError, subprocess.CalledProcessError, RuntimeError) as exc:
        # Commands intentionally do not print exception chains or environment data.
        raise SystemExit(f"backup operation failed: {type(exc).__name__}: {exc}") from None


if __name__ == "__main__":
    main()
