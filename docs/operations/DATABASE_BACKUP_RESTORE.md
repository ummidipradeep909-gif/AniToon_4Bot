# PostgreSQL backup and restore

This guide covers the durable PostgreSQL data used by AniToon_4Bot and the encryption key required to recover clone-bot credentials. Practice a restore before relying on backups in production.

## Encrypted backup tool (Feature 18)

The repository includes `scripts/backup.py`. It uses PostgreSQL custom-format dumps and [age](https://age-encryption.org/) public-key encryption. The age recipient is public and can be stored in deployment configuration; the matching age identity is private and must be kept separately from the backup destination. The script does not put database passwords in process arguments: libpq receives credentials through a mode-0600 temporary password file, removed after the operation.

Install PostgreSQL client tools (`pg_dump`, `pg_restore`), age, Python 3.12+, Go 1.24+, and the application source revision matching the backup.

### Create an encrypted backup

Configure these values in the protected shell environment or a secret-file loader:

- `DATABASE_URL`: source PostgreSQL connection URL.
- `BACKUP_AGE_RECIPIENT`: age public recipient string (not a secret).
- The script records the checked-out Git commit automatically and rejects a conflicting `APP_COMMIT` override.
- The script derives the latest migration version from the numbered SQL files. `EXPECTED_MIGRATION_VERSION` may be set to require an explicit version.

Then run:

```bash
umask 077
python scripts/backup.py create --output /secure/off-host-staging/anitoon
```

The script writes a `.dump.age` archive and a JSON manifest. It creates plaintext dumps only in a mode-0600 temporary file and deletes that file on success or failure. The encrypted archive and manifest are atomically renamed into place. Copy both files to an access-controlled off-host destination and apply retention there. The SHA-256 value detects accidental archive corruption; it is not a digital signature or a substitute for access control. Age authenticated encryption protects the dump contents.

### Verify an encrypted backup

Configure `BACKUP_AGE_IDENTITY_FILE` to the path of the private age identity file, then run:

```bash
python scripts/backup.py verify /secure/off-host-staging/anitoon/<backup>.manifest.json
```

Verification checks the manifest schema, rejects path traversal and non-regular archive files, validates the ciphertext SHA-256 digest, decrypts to a private temporary file, and runs `pg_restore --list`. It does not connect to or modify a database.

### Run an isolated restore drill

The restore drill is intentionally destructive to its target database: `pg_restore --clean --if-exists` replaces objects in that database. It refuses to run unless the target is explicitly marked isolated, the exact confirmation phrase is present, and the target database name ends in `_restore` or `_drill`. It also rejects a target with the same host/port/database identity as the source. This is a guardrail, not a substitute for verifying the connection details yourself.

Set:

- `DATABASE_URL`: source/production database URL used only for identity comparison.
- `RESTORE_DATABASE_URL`: connection URL for a dedicated disposable restore database.
- `RESTORE_TARGET_ISOLATED=true`.
- `RESTORE_TARGET_CONFIRMATION=I_CONFIRM_ISOLATED_RESTORE`.
- `BACKUP_AGE_IDENTITY_FILE`: private age identity file.
- `CLONE_ENCRYPTION_KEY`: the exact existing stable clone key, supplied through the existing secret-file mechanism or protected environment.

The target must be a disposable database named with the `_restore` or `_drill` suffix, e.g. `anitoon4_restore`. Run from the exact Git commit recorded in the manifest; the tool rejects a different checked-out revision and verifies that the restored migration version matches the manifest:

```bash
python scripts/backup.py restore-drill /secure/off-host-staging/anitoon/<backup>.manifest.json
```

The script decrypts and restores only to the confirmed target, then runs `go run ./cmd/backup-verify` against it. That verifier is read-only: it compares every applied migration version, filename and SHA-256 checksum against the embedded SQL in the checked-out application revision, and authenticates each stored clone token ciphertext with the existing `CLONE_ENCRYPTION_KEY`. It validates the token's bot-ID prefix without printing the token. A mismatch fails the drill; the verifier never applies migrations or changes production.

A successful restore drill is evidence for that exact backup, application revision, migration set, age identity and clone encryption key. Repeat it after application migrations, key-management changes, PostgreSQL major-version changes, or backup pipeline changes. Do not infer recoverability from a successful `pg_dump` alone.

### Limitations and operator responsibilities

- Scheduling, off-host upload, retention, alerting, restore orchestration and RPO/RTO monitoring are not automated by this feature. Run the tool from a protected backup host or a dedicated scheduler and alert on failures and backup age.
- Keep the age private identity and `CLONE_ENCRYPTION_KEY` in separate secure escrow systems. Losing either makes the corresponding backup or saved clone credentials unrecoverable.
- The SHA-256 manifest is not signed. Protect the manifest and archive with access controls and immutable/off-host storage.
- The script requires a compatible `pg_dump`/ `pg_restore` client version. Test PostgreSQL major-version compatibility before production recovery.
- The isolated restore command uses `--clean`; never point it at production. Do not set the confirmation flags in a normal application environment.
- Temporary plaintext dumps are removed on normal exit and handled exceptions. For high-assurance environments, use an encrypted scratch filesystem and ensure host crash-dump/swap policy is appropriate.

## What must be backed up

| Item | Why it matters | Backup approach |
|---|---|---|
| PostgreSQL database | Group settings, moderation audit/history, warnings, clone credential ciphertext, notes, filters and other durable configuration | Regular `pg_dump` custom-format backup |
| `CLONE_ENCRYPTION_KEY` | Decrypts saved clone bot tokens encrypted with AES-256-GCM | Secure secret manager / offline encrypted escrow, separate from the database |
| Runtime configuration and other secrets | Bot token, database credentials, Redis connection, AI service token and provider credentials | Deployment secret manager and configuration backup |
| Migration files / application version | Required to interpret and upgrade the restored schema | Keep the matching Git revision and its `migrations/` directory |

**Critical:** a database backup without the exact matching `CLONE_ENCRYPTION_KEY` cannot recover saved clone bot tokens. Losing or rotating that key without a planned re-encryption migration makes existing clone credentials unrecoverable. Never put the key in Git, the database dump, a ticket, or an unencrypted backup.

Redis holds expiring counters and rate-limit/cooldown state; durable group configuration and audit records are stored in PostgreSQL. Redis state can be allowed to rebuild after recovery. Do not treat a Redis snapshot as a substitute for the PostgreSQL backup.

## Before starting

- Use a protected backup host and an encrypted destination with restricted access.
- Ensure sufficient free disk space for the dump and a second copy.
- Use a dedicated backup identity where possible; the Compose example uses the configured PostgreSQL superuser.
- Never put passwords or bot tokens directly in command-line arguments or commit them to scripts.
- Record the application Git commit, backup timestamp (UTC), database version and migration level alongside each backup.
- Keep at least one encrypted copy off the application host and periodically test that it can be read.

The example commands below assume the repository root and the Compose service named `postgres` from `docker-compose.yml`.

## Create a PostgreSQL backup

Create a timestamped custom-format dump on the host:

```bash
mkdir -p backups
chmod 700 backups
BACKUP="backups/anitoon4-$(date -u +%Y%m%dT%H%M%SZ).dump"
umask 077
docker compose exec -T postgres pg_dump \
  -U anitoon4 -d anitoon4 -Fc \
  > "$BACKUP"
```

Check that the dump is non-empty and readable:

```bash
test -s "$BACKUP"
docker compose exec -T postgres pg_restore --list < "$BACKUP" | head
sha256sum "$BACKUP" > "$BACKUP.sha256"
```

Copy both files to the approved encrypted backup destination. Protect the checksum file with the dump; a checksum detects accidental corruption but does not provide confidentiality or authenticity by itself.

For a non-Compose PostgreSQL server, run `pg_dump -Fc --no-owner --dbname="$DATABASE_URL" > "$BACKUP"` using a secret-safe credential mechanism such as a restricted `.pgpass` file. Do not paste a credential-bearing URL into a shared terminal recording or support log.

## Back up clone encryption and runtime secrets

1. Retrieve the current `CLONE_ENCRYPTION_KEY` from the deployment secret manager.
2. Store it in a separately access-controlled encrypted secret backup. Record the key version/identifier, not the key value, in the backup manifest.
3. Back up the primary bot token, database credentials, `AI_SERVICE_TOKEN`, and any enabled provider credentials through the deployment's secret-management process.
4. Verify that an authorized operator can retrieve the secret backup. Do not print secrets to logs or include them in the database dump.

The clone encryption key must remain byte-for-byte identical to the key used when the clone credentials were written. Generating a new key is not a recovery procedure.

## Restore into a separate database first

A restore should be rehearsed against a fresh database or isolated PostgreSQL instance before replacing production data. Stop application writers or use a consistent backup window for a planned cutover.

1. Verify the backup checksum:

   ```bash
   sha256sum -c "$BACKUP.sha256"
   ```

2. Create a separate empty target database:

   ```bash
   docker compose exec postgres createdb \
     -U anitoon4 -T template0 anitoon4_restore
   ```

3. Restore the custom-format dump into that target:

   ```bash
   docker compose exec -T postgres pg_restore \
     -U anitoon4 -d anitoon4_restore \
     --no-owner --no-acl \
     < "$BACKUP"
   ```

   The `anitoon4` role must exist and have the required privileges. For an external PostgreSQL server, create the target database and role using your database administrator's approved procedure, then use `pg_restore --dbname="$RESTORE_DATABASE_URL"`.

4. Verify schema and data before switching application traffic:

   ```bash
   docker compose exec postgres psql -U anitoon4 -d anitoon4_restore \
     -c "SELECT current_database(), current_timestamp;"
   docker compose exec postgres psql -U anitoon4 -d anitoon4_restore \
     -c "\\dt"
   docker compose exec postgres psql -U anitoon4 -d anitoon4_restore \
     -c "SELECT COUNT(*) AS clone_credentials FROM clone_credentials;"
   ```

   The Go application now maintains a `schema_migrations` ledger with numeric version, filename, SHA-256 checksum and application timestamp. Verify it with `SELECT version, filename, checksum, applied_at FROM schema_migrations ORDER BY version;`. A fresh database should show the complete ordered migration set for the application revision. The runner validates checksums and fails startup if a previously applied migration was edited; do not infer that a restore failed solely because a dump from an older release has no migration ledger. After restoring an older backup, deploy the matching application revision and let the runner apply the remaining compatible migrations. Always rehearse this upgrade on an isolated restore first.

5. Restore the exact matching `CLONE_ENCRYPTION_KEY` and runtime secrets through the secret manager. Point both the Go bot and Python policy service at the restored database (the Compose policy service's `DATABASE_URL` is currently configured in `docker-compose.yml`; update that configuration as part of the controlled cutover). Recreate/restart the services only after validating the configuration.
6. Check application startup logs for PostgreSQL connectivity, clone credential decryption/recovery, and migration/schema errors. Logs must not contain tokens or encryption key material.
7. Test a non-destructive bot command, a read-only group configuration lookup, and an authorized moderation-log/report query in a test group. Confirm clone bots recover only when the matching key is present and that paused clones remain paused.

## Production cutover and rollback

- Announce a maintenance window and stop all application instances that can write to the production database.
- Take a fresh pre-cutover dump before changing connection settings.
- Restore and validate on the separate target first. Change the Go and policy-service database connection settings together so both components use the same intended database.
- Start one application instance, inspect health and logs, then resume normal traffic. Avoid running two independent polling instances for the same Telegram bot.
- Keep the previous database untouched and access-controlled until the new deployment has passed its observation period. Roll back by stopping writers and switching both services back to the previous database and known-good application revision.
- Do not run schema migrations against the old database as part of rollback unless that rollback plan explicitly supports them.

## Migration handling and Docker notes

- Docker's `/docker-entrypoint-initdb.d` scripts run only when the PostgreSQL data directory is first initialized. Adding a new SQL file to `migrations/` does **not** automatically apply it to an existing named volume.
- The current Compose setup mounts `./migrations` into the PostgreSQL initialization directory. For an existing installation, inspect the SQL and apply the required migration files explicitly in their documented numeric order using an authorized operator and a maintenance window.
- Do not blindly re-run every SQL file on an existing database. Check whether each statement is safe/idempotent and whether the migration has already been applied.
- The Go bot applies embedded migrations automatically after PostgreSQL connectivity is confirmed. If migration execution fails, the bot refuses to start so it cannot operate against a partially upgraded schema. The Python policy service does not run migrations independently.
- A PostgreSQL logical dump does not back up Docker volume metadata, host configuration, TLS certificates or secret-manager contents. Back those up separately where applicable.

## Suggested backup schedule and retention

For a small production deployment, start with a daily encrypted logical backup and retain (for example) 7 daily, 4 weekly and 3 monthly copies. Adjust retention to the service's recovery-point objective, data volume, legal requirements and available storage. Alert on failed backups and monitor backup age. Periodically restore a recent backup into an isolated environment; a successful dump command alone does not prove recoverability.

## Recovery checklist

- [ ] Correct application revision and ordered migration files identified.
- [ ] PostgreSQL dump checksum verified and dump listing readable.
- [ ] Restore completed into an isolated target database.
- [ ] Expected tables and representative records verified.
- [ ] Exact `CLONE_ENCRYPTION_KEY` retrieved from secure escrow.
- [ ] Bot, database, Redis and policy-service secrets/configuration restored securely.
- [ ] Go bot and Python policy service point to the same restored database.
- [ ] Startup, clone recovery and read-only command checks passed.
- [ ] Pre-cutover database and rollback revision retained until sign-off.
