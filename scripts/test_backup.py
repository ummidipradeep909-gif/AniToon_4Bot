import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

MODULE_PATH = Path(__file__).with_name("backup.py")
SPEC = importlib.util.spec_from_file_location("anitoon_backup", MODULE_PATH)
backup = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(backup)


class BackupManifestTests(unittest.TestCase):
    def test_valid_manifest_and_ciphertext_checksum(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / "db.dump.age"
            archive.write_bytes(b"encrypted bytes")
            manifest = root / "db.manifest.json"
            manifest.write_text(json.dumps({
                "manifest_version": 1,
                "database": "anitoon",
                "created_at": "2026-01-01T00:00:00+00:00",
                "application_commit": "a" * 40,
                "schema_migration_version": "16",
                "format": "postgresql-custom-dump",
                "encryption": "age",
                "ciphertext_file": archive.name,
                "ciphertext_sha256": backup._sha256(archive),
            }), encoding="utf-8")
            backup.load_manifest(manifest)
            self.assertEqual(backup._sha256(archive), json.loads(manifest.read_text())["ciphertext_sha256"])

    def test_rejects_checksum_tampering(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / "db.dump.age"
            archive.write_bytes(b"ciphertext")
            manifest = root / "db.manifest.json"
            manifest.write_text(json.dumps({
                "manifest_version": 1, "database": "db", "created_at": "now",
                "created_at": "2026-01-01T00:00:00+00:00",
                "format": "postgresql-custom-dump", "encryption": "age",
                "application_commit": "a" * 40, "schema_migration_version": "16",
                "ciphertext_file": archive.name, "ciphertext_sha256": "0" * 64,
            }), encoding="utf-8")
            args = type("Args", (), {"manifest": str(manifest)})()
            with self.assertRaisesRegex(RuntimeError, "SHA-256"):
                backup.verify_backup(args)

    def test_rejects_path_traversal(self):
        with tempfile.TemporaryDirectory() as directory:
            manifest = Path(directory) / "bad.json"
            manifest.write_text(json.dumps({
                "manifest_version": 1, "database": "db", "created_at": "now",
                "created_at": "2026-01-01T00:00:00+00:00",
                "format": "postgresql-custom-dump", "encryption": "age",
                "application_commit": "a" * 40, "schema_migration_version": "16",
                "ciphertext_file": "../outside.dump.age", "ciphertext_sha256": "a" * 64,
            }), encoding="utf-8")
            with self.assertRaisesRegex(RuntimeError, "archive path"):
                backup.load_manifest(manifest)

    def test_rejects_symlinked_manifest(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / "target.json"
            target.write_text("{}", encoding="utf-8")
            link = root / "link.json"
            link.symlink_to(target)
            with self.assertRaisesRegex(RuntimeError, "symbolic link"):
                backup.load_manifest(link)

    def test_rejects_unsupported_manifest_version(self):
        with tempfile.TemporaryDirectory() as directory:
            manifest = Path(directory) / "bad.json"
            manifest.write_text(json.dumps({
                "manifest_version": 2, "database": "db", "created_at": "now",
                "created_at": "2026-01-01T00:00:00+00:00",
                "format": "postgresql-custom-dump", "encryption": "age",
                "application_commit": "a" * 40, "schema_migration_version": "16",
                "ciphertext_file": "db.dump.age", "ciphertext_sha256": "a" * 64,
            }), encoding="utf-8")
            with self.assertRaisesRegex(RuntimeError, "unsupported"):
                backup.load_manifest(manifest)


if __name__ == "__main__":
    unittest.main()
