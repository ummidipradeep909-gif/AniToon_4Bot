# PostgreSQL and Redis credential rotation

This runbook rotates the existing single PostgreSQL login and Redis default-user
password in a **planned maintenance window**. It is intentionally not a zero-downtime
or automatic rotation mechanism. Both application services (the Go bot and Python
policy service) share these credentials, so stop both consumers before changing
either service password.

## Scope and safety

- Applies to the single-host `compose.production.yaml` profile.
- PostgreSQL uses the `anitoon4` role; the Compose `POSTGRES_PASSWORD` value is
  only an initialization value for a new data directory. Changing that variable
  alone does **not** change the password in an existing PostgreSQL volume.
- Redis currently uses the default ACL user with `requirepass`. Its password is
  supplied to the Redis container and both application connection URLs.
- This procedure causes a short interruption to bot and policy service operations.
- It does not rotate `CLONE_ENCRYPTION_KEY`. Never change that key as part of a
  database/cache credential rotation.
- Never put passwords in shell command arguments, terminal transcripts, tickets,
  commits, or logs. Use the interactive PostgreSQL password prompt and a protected
  environment file. Keep old and new values in a secure operator-only location
  until the rollback window closes.

## 1. Prepare

1. Schedule a maintenance window and notify operators.
2. Confirm you have host access, the current `.env.production`, and a tested
   PostgreSQL backup. Follow [DATABASE_BACKUP_RESTORE.md](DATABASE_BACKUP_RESTORE.md).
3. Make a protected copy of the current environment file outside the repository
   (for rollback), then restrict both files to the operator:
   `chmod 600 .env.production`.
4. Generate two independent URL-safe passwords on the host:
   `openssl rand -hex 32`.
   Generate one for PostgreSQL and a different one for Redis. Do not reuse either
   password for any other service.
5. Update the four connection URLs in the protected environment file with the new
   values. Because hex passwords are URL-safe, no percent-encoding is needed:
   - PostgreSQL: `DATABASE_URL` and `POLICY_DATABASE_URL`
   - Redis: `REDIS_URL` and `POLICY_REDIS_URL`
   Also update `POSTGRES_PASSWORD` and `REDIS_PASSWORD`.
6. Validate Compose interpolation without printing the expanded configuration:
   `docker compose --env-file .env.production -f compose.production.yaml config -q`.
   Do not start or recreate services yet.

## 2. Rotate PostgreSQL

1. Stop both database consumers:
   ```sh
   docker compose --env-file .env.production -f compose.production.yaml stop bot policy
   ```
2. Open an interactive SQL session using the local container socket:
   ```sh
   docker compose --env-file .env.production -f compose.production.yaml exec postgres psql -U anitoon4 -d anitoon4
   ```
3. At the `psql` prompt, run `\\password anitoon4`. Enter the new PostgreSQL
   password at the hidden prompts, then run `\\q`. Do not use an inline
   `ALTER ROLE ... PASSWORD '...'` command because it can leak into shell history,
   process listings, or SQL logs.
4. Confirm the protected environment file contains the new `POSTGRES_PASSWORD`,
   `DATABASE_URL`, and `POLICY_DATABASE_URL`. The Compose `POSTGRES_PASSWORD`
   variable is retained for future fresh-volume initialization; the role password
   was changed explicitly in the existing database.
5. Recreate the consumers with the updated URLs:
   ```sh
   docker compose --env-file .env.production -f compose.production.yaml up -d bot policy
   ```
6. Confirm policy health and inspect application logs for successful connections.
   Do not proceed to Redis rotation if PostgreSQL authentication is failing.

## 3. Rotate Redis

1. Stop both consumers again:
   ```sh
   docker compose --env-file .env.production -f compose.production.yaml stop bot policy
   ```
2. Confirm `REDIS_PASSWORD`, `REDIS_URL`, and `POLICY_REDIS_URL` all contain the
   new Redis password in the protected environment file.
3. Recreate Redis so its `--requirepass` startup argument uses the new password:
   ```sh
   docker compose --env-file .env.production -f compose.production.yaml up -d --no-deps redis
   ```
   The named Redis data volume is preserved. Do not use `down -v`.
4. Verify Redis authentication from inside its container without putting the
   password in the command line:
   ```sh
   docker compose --env-file .env.production -f compose.production.yaml exec -T redis sh -ec 'REDISCLI_AUTH="$REDIS_PASSWORD" redis-cli ping'
   ```
   The expected response is `PONG`.
5. Recreate both consumers:
   ```sh
   docker compose --env-file .env.production -f compose.production.yaml up -d bot policy
   ```

## 4. Verify and close

1. Check service state:
   `docker compose --env-file .env.production -f compose.production.yaml ps`.
2. Check the policy health endpoint from inside the Compose network or host tooling
   that can reach it. It must report healthy database and Redis dependencies.
3. Inspect recent bot and policy logs for connection/authentication failures. Do not
   copy secret-bearing environment output into logs or support tickets.
4. Exercise a harmless database-backed command and a Redis-backed anti-flood/
   slow-mode path in a test group. Confirm moderation behavior remains unchanged;
   AI suggestions must remain advisory and must not independently punish users.
5. Retain the protected rollback copy only for the approved rollback period, then
   securely dispose of it under the operator's secret-retention policy.

## Rollback

- **PostgreSQL:** stop `bot` and `policy`, open the same interactive `psql`
  session, run `\\password anitoon4` and restore the previous password at the
  hidden prompts. Restore the previous protected environment file, validate
  Compose configuration, then start the consumers.
- **Redis:** stop `bot` and `policy`, restore the previous protected environment
  file, recreate Redis with `up -d --no-deps redis`, verify authenticated `PONG`,
  then start the consumers.
- If the database itself was damaged, use the separately tested database restore
  procedure. A password rollback is not a database restore.
- Do not delete PostgreSQL or Redis volumes during rollback.

## Limitations

This implementation is a documented, operator-driven maintenance rotation.
There is no automated credential generator/rotator, dual PostgreSQL role,
Redis named-user overlap, automatic rollback, secret-manager integration, or
zero-downtime guarantee. A future zero-downtime design should introduce separate
application roles and Redis ACL users, staged dual-credential deployment, and
integration tests against the exact production Compose profile before removing
the old credentials.
