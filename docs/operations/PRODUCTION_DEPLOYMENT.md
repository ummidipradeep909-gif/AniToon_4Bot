# Production deployment profile

This repository provides a production-oriented Docker Compose profile in
`compose.production.yaml`. It is a starting deployment baseline, not a
managed secret store or a multi-replica architecture.

## Before deployment

1. Use a dedicated production host and a private, access-controlled directory.
2. Copy `.env.production.example` to `.env.production`; never commit the real file.
3. Generate independent high-entropy values:
   - `POSTGRES_PASSWORD` and `REDIS_PASSWORD`: `openssl rand -hex 32`.
   - `CLONE_ENCRYPTION_KEY`: `openssl rand -base64 32`, exactly once for this installation.
   - `AI_SERVICE_TOKEN`: generate a separate random secret.
   - `WEBHOOK_SECRET_TOKEN`: generate a separate URL-safe secret if webhook mode is selected.
4. Put the same database and Redis passwords into their connection URLs. Hex passwords are URL-safe; if you choose other characters, percent-encode URL credentials.
5. Set `BOT_TOKEN`. Keep AI disabled until `OPENAI_API_KEY` is deliberately configured.
6. Restrict file permissions: `chmod 600 .env.production`.
7. Back up the PostgreSQL database and separately escrow the stable clone encryption key before first production rollout.

## Start

Validate the fully expanded configuration without printing it into logs:

```sh
docker compose --env-file .env.production -f compose.production.yaml config -q
docker compose --env-file .env.production -f compose.production.yaml build
docker compose --env-file .env.production -f compose.production.yaml up -d
docker compose --env-file .env.production -f compose.production.yaml ps
```

The profile keeps PostgreSQL and Redis off published host ports, enables Redis authentication,
uses persistent data volumes, waits for database/cache/policy health before starting
the bot, runs the bot as the non-root user from the existing distroless image,
drops Linux capabilities, enables read-only root filesystems where supported,
and rotates JSON container logs.

The policy service is exposed only on the Compose network. Do not publish port 8000
on a public interface. Put webhook mode behind an HTTPS reverse proxy and forward
only the configured webhook path to the bot listener. Polling is the default.

## PostgreSQL connection pool sizing

The Go process uses bounded pgx pool settings configured with `DB_POOL_MAX_CONNS`,
`DB_POOL_MIN_CONNS`, `DB_POOL_MAX_LIFETIME_SECONDS`,
`DB_POOL_MAX_IDLE_SECONDS` and `DB_POOL_HEALTH_CHECK_SECONDS`.
Defaults are conservative for a single process. For multi-instance deployments,
calculate the total maximum as:

`replica_count × DB_POOL_MAX_CONNS + policy_service_connections + operational_headroom`

Keep that total below PostgreSQL's usable connection limit, leaving capacity for
administration, migrations and failover. Do not copy the single-instance default
unchanged to every replica without checking the database plan. Invalid values
fail startup rather than being silently clamped.

## Secret handling and rotation status

- Prefer the hosting provider's secret manager over a long-lived environment file.
  If an environment file is used, restrict host access and backups.
- Rotate the Telegram primary bot token through BotFather and update the deployment
  secret before restarting the single bot instance. Coordinate the restart to avoid
  two pollers using the same token.
- PostgreSQL and Redis password rotation is an operator-driven maintenance task
  that interrupts the bot and policy service briefly. Follow the staged
  [PostgreSQL and Redis credential rotation runbook](CREDENTIAL_ROTATION.md).
  Changing `POSTGRES_PASSWORD` alone does not change the password stored in an
  existing PostgreSQL data volume; the runbook explicitly changes the database role.
- The runbook updates the Go bot and Python policy service connection URLs together,
  verifies both dependencies, and documents rollback. It does not provide
  dual-credential overlap or zero-downtime rotation.
- `CLONE_ENCRYPTION_KEY` is not an ordinary environment secret: changing it
  immediately makes existing encrypted clone tokens unreadable. Do not change it
  until a tested, transactional key-version/keyring migration is implemented.
  Keep a recoverable escrow copy separate from database backups.
- No automatic secret rotation, secret-manager integration, multi-host polling
  lease, or zero-downtime credential rotation is claimed.

## Rollback

1. Stop the bot and policy service before changing database/cache credentials.
2. Follow the credential-specific rollback steps in
   [CREDENTIAL_ROTATION.md](CREDENTIAL_ROTATION.md).
3. Restore the previous image/configuration and the matching database backup if a
   migration rollback is required.
4. Restore the matching `CLONE_ENCRYPTION_KEY` escrow value with the database.
5. Start services and verify health, clone recovery, and moderation commands.
6. Do not delete persistent volumes as a rollback technique.

## Operational limitations

The bot currently has no independent HTTP readiness endpoint in polling mode.
Compose therefore uses dependency health gates and process restart policy, not a
false-positive bot healthcheck. The current clone runner is single-process; run
exactly one bot replica for each primary/clone token. Production availability,
TLS termination, firewalling, host patching, off-host encrypted backups, alerting,
and secret-manager integration remain operator responsibilities.
