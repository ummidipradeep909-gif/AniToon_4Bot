# Docker Secrets deployment

The original `.env` / `.env.production` environment-variable deployments remain
supported. Go and the Python policy service additionally accept `NAME_FILE`
variables; file contents are trimmed and never included in errors. Supplying both
a non-empty `NAME` and `NAME_FILE` is rejected as ambiguous.

## Prepare secret files

Create a host-only `secrets/` directory beside the Compose files and restrict it
to the deployment operator (`chmod 700 secrets`; each file `chmod 600`). Populate
these files using the exact values already in use by the installation:

- `postgres_password`, `redis_password`
- `database_url`, `policy_database_url`, `redis_url`, `policy_redis_url`
- `bot_token`, `ai_service_token`, `openai_api_key`
- `clone_encryption_key`, `webhook_secret_token`

The four connection URL files must match the server credentials. AI files may
contain empty values when AI is disabled. Webhook secret must be populated when
webhook mode is selected. Keep the current `clone_encryption_key` byte-for-byte
unchanged: do not generate a replacement during migration to Docker Secrets.
Back it up separately from PostgreSQL data.

## Start with Docker Secrets

Validate both Compose files without printing expanded configuration:

```sh
docker compose --env-file .env.production -f compose.production.yaml -f compose.secrets.yaml config -q
docker compose --env-file .env.production -f compose.production.yaml -f compose.secrets.yaml build
docker compose --env-file .env.production -f compose.production.yaml -f compose.secrets.yaml up -d
```

Compose file-backed secrets are mounted read-only under `/run/secrets`. PostgreSQL
uses its native `POSTGRES_PASSWORD_FILE` support; Redis reads its password from
the mounted file; Go and Python resolve the corresponding `*_FILE` paths. The
secret files are host-side inputs and must not be added to version control.

## Compatibility and failure behavior

For legacy deployments, keep using the existing environment variables. Required
bot token, PostgreSQL URL, Redis URL and clone encryption key are validated before
the Go bot starts. The policy service requires its database URL, Redis URL and
service-authentication token. Missing/unreadable required secrets stop startup;
there is no fallback to an empty credential. PostgreSQL and Redis connection
failures stop the Go bot instead of silently disabling storage-backed protections.
Optional AI provider credentials are only used when AI is explicitly enabled.

Docker Compose local file-backed secrets are not an encrypted cloud secret manager:
protect the host, backups and deployment access. On Swarm or a managed platform,
use its native secret facility and supply the same mounted file paths. Never print
`docker compose config` output, environment dumps, or secret files into CI logs.

## Rollback

To roll back the deployment mechanism, restore the previous image/configuration
and use the existing protected environment file containing the same live values.
Do not rotate credentials or change the clone key as part of this rollback. Keep
PostgreSQL and Redis volumes intact, and verify database, cache, clone recovery,
and moderation behavior before closing the maintenance window.
