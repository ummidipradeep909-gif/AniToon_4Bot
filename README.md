# AniToon_4Bot

A modular Telegram group-management bot project inspired by the breadth of Rose Bot. This is an independent implementation; it does not use Rose branding or claim feature parity.

## Architecture

- **Go Telegram runtime** — Bot API polling, command routing and Telegram actions.
- **Python policy service** — risk and automation decisions behind a private service boundary.
- **PostgreSQL** — durable chat settings, membership records and moderation audit history.
- **Redis** — rate counters, short-lived state, anti-flood windows and caching.
- **Telegram-only administration** — no web dashboard is planned.

## What is implemented now

- Go polling runtime with `/start`, `/help` and `/id`.
- Environment validation and basic configuration tests.
- FastAPI policy service with dependency health checks and a non-enforcing per-user message counter.
- PostgreSQL starter schema for chats, members, moderation history, filters and warnings.
- Docker Compose services for PostgreSQL, Redis and the policy service.
- GitHub Actions checks for Go tests/vet and Python syntax compilation.

**Important:** moderation commands, anti-spam enforcement and the broader Rose-style feature set are not implemented yet. The policy endpoint currently returns `allow` and does not punish users. Do not deploy this as a production moderation bot.

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md) for the staged feature inventory. A feature is considered complete only after actor authorization, bot-permission checks, persistence, audit logging, error handling and tests are implemented together.

## Local development

Requirements: Go 1.24+, Python 3.12+, Docker Compose.

1. Copy `.env.example` to `.env` and set `BOT_TOKEN`.
2. Start infrastructure: `docker compose up -d --build postgres redis policy`.
3. Run Go checks: `go test ./...` and `go vet ./...`.
4. Start the bot: `go run ./cmd/anitoon4bot`.

For local Go execution, the Go toolchain must be able to download the `gotgbot` module from GitHub.

## Security requirements

- Never commit bot tokens, API keys or database credentials.
- Check the invoking user's admin rights and the bot's required rights before every moderation action.
- Never let the bot moderate chat creators or administrators unless Telegram explicitly allows the operation.
- Persist an audit record for every successful enforcement action.
- Escape untrusted Telegram text before inserting it into HTML.
- Keep internal policy endpoints private or authenticate service-to-service calls.
- Use 64-bit Telegram identifiers.

## Reference and licensing

The legacy Python bot source was reviewed as a feature checklist, not imported into the new Go runtime. The roadmap records the intended modules without claiming they already exist. Preserve upstream license notices for any vendored dependencies and review them before redistribution.
