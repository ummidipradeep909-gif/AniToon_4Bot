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

## Secret handling and rotation status

- Prefer the hosting provider's secret manager over a long-lived environment file.
  If an environment file is used, restrict host access and backups.
- Rotate the Telegram primary bot token through BotFather and update the deployment
  secret before restarting the single bot instance. Coordinate the restart to avoid
  two pollers using the same token.
- Rotate PostgreSQL and Redis passwords with a maintenance window and update all
  dependent connection URLs together. Validate connections before removing the old
  credentials. The supplied Compose profile does not implement dual-credential
  overlap or zero-downtime rotation.
- `CLONE_ENCRYPTION_KEY` is not an ordinary environment secret: changing it
  immediately makes existing encrypted clone tokens unreadable. Do not change it
  until a tested, transactional key-version/keyring migration is implemented.
  Keep a recoverable escrow copy separate from database backups.
- This change does not claim automatic key rotation, clone-token re-encryption,
  a multi-host polling lease, or zero-downtime credential rotation. Those require
  a dedicated follow-up implementation and recovery tests.

## Rollback

1. Stop the bot service before changing versions or credentials:
   `docker compose --env-file .env.production -f compose.production.yaml stop bot`.
2. Restore the previous image/configuration and the matching database backup if a
   migration rollback is required.
3. Restore the matching `CLONE_ENCRYPTION_KEY` escrow value with the database.
4. Start services and verify health, clone recovery, and moderation commands.
5. Do not delete persistent volumes as a rollback technique.

## Operational limitations

The bot currently has no independent HTTP readiness endpoint in polling mode.
Compose therefore uses dependency health gates and process restart policy, not a
false-positive bot healthcheck. The current clone runner is single-process; run
exactly one bot replica for each primary/clone token. Production availability,
TLS termination, firewalling, host patching, off-host encrypted backups, alerting,
and secret-manager integration remain operator responsibilities.
