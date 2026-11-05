# Current System Guide

Source reviewed: `main` commit `3bd19f02e1534129b02488a8d2d59922edc4b3c8`, 5 November 2026.

## Architecture
- **Go/Gotgbot:** Telegram updates, command dispatch, moderation, clone lifecycle, and AI-service client.
- **PostgreSQL:** durable group settings, warnings, audit records, encrypted clone credentials, migration history, and clone polling leases.
- **Redis:** ephemeral anti-flood/raid/slow-mode counters and cooldowns.
- **Python/FastAPI:** authenticated policy telemetry and optional AI advice. It does not perform Telegram moderation.
- **Docker Compose:** development and hardened single-host production services, with a Docker Secrets overlay.

## Startup and runtime
1. Go loads required BOT_TOKEN, DATABASE_URL, REDIS_URL and the existing CLONE_ENCRYPTION_KEY. Each supports a NAME_FILE secret path; direct and file values cannot both be set.
2. The bot connects to Redis and PostgreSQL, checks dependencies, then applies embedded SQL migrations in numeric order. A PostgreSQL advisory lock serializes migration runners. Migration failures or stored filename/checksum mismatches stop startup.
3. A shared Gotgbot dispatcher registers command, moderation, clone and community handlers.
4. Saved clones are decrypted and validated individually. Paused clones remain stopped. Enabled clone polling starts only after acquiring its PostgreSQL lease.
5. Primary bot uses polling by default or HTTPS webhook mode when configured; clones use polling.
6. SIGINT/SIGTERM stops update loops before closing PostgreSQL and Redis.

The Python service independently opens PostgreSQL and Redis connections; its /health endpoint checks both.

## Implemented features
- General: /start, /help, /id, /info, /admins.
- Clone management (main bot private chat): /clone, /cancel, /clonestatus, /clonepause, /cloneresume, /cloneremove.
- Moderation: ban, unban, kick, mute, unmute, warnings, warning reset, delete, purge, permission locks, moderation audit log, reports, metrics and opt-in event notifications.
- Protection: anti-flood, anti-raid temporary join-wave quarantine, slow mode, link protection/trusted domains, forwarded-message protection, keyword blacklist delete/warn modes.
- Community: welcome/goodbye templates, group rules, named notes and exact-match custom reply filters.
- Owner: private /dashboard for configured BOT_OWNER_ID, with uptime, Go memory/goroutines, CPU core count, PostgreSQL pool/health, Redis health and aggregate clone health.

## Authorization
Privileged commands require an identifiable human actor and a live Telegram administrator check. Verification errors do not grant access. The bot separately checks Telegram permissions required for each action and protects group creators/admins from routine moderation. Anonymous-admin commands are rejected. Targets are replies or numeric Telegram IDs, not usernames.

## AI safety
Both POST /v1/policy/evaluate and POST /v1/ai/analyze require X-AI-Service-Token. The Python service compares it in constant time and fails closed if it is unconfigured. Go bounds request time and response size and accepts only advisory assist mode with allow/admin_review. AI can suggest review; it cannot ban, mute, delete or otherwise punish users. Configured deterministic protections remain authoritative.

## Clone recovery and scaling
Clone health reconciliation runs every 60 seconds; recovery uses capped exponential backoff from 5 seconds to 5 minutes. Clone polling leases use PostgreSQL with 90-second TTL and 20-second renewal. Takeover increments a fencing token; renewal errors/ambiguous results stop the local poller. Stale instances cannot renew or release a newer lease.

**Scaling limit:** clone pollers are coordinated, but the primary bot has no distributed polling supervisor. Do not horizontally scale the complete service or run multiple primary pollers with the same token yet. Telegram API side effects are not exactly-once because Telegram does not accept the fencing token.

## Storage, backups and secrets
PostgreSQL is durable state; Redis is ephemeral and its counters/windows are not restored by a PostgreSQL dump. scripts/backup.py creates age-encrypted PostgreSQL custom-format backups and SHA-256 manifests; restore drills require a separately named isolated database. cmd/backup-verify checks migration history and saved clone ciphertext without printing tokens.

Keep the exact existing CLONE_ENCRYPTION_KEY escrowed separately. Never regenerate it: losing/changing it makes saved clone tokens unrecoverable. Scheduled off-host backup uploads, retention, alerts and measured RPO/RTO remain operator responsibilities. Automated zero-downtime DB/Redis credential rotation and clone-key rotation are not implemented.

## CI snapshot and limitations
On the reviewed commit GitHub reported successful `go`, `python` and `backup-tooling` checks. Go CI runs tests, race tests, vet and Compose validation; Python CI compiles the app and runs unit tests; backup CI exercises encrypted PostgreSQL backup creation and verification.

PR #20 is merged and secures both internal policy endpoints. This does not certify that the entire security audit is complete. At review time GitHub metadata showed main had no branch protection or required status checks configured.
