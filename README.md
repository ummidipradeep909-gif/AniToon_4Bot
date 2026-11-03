# AniToon_4Bot

A modular Telegram group-management bot project inspired by the breadth of Rose Bot. This is an independent implementation; it does not use Rose branding or claim feature parity.

## Architecture

- **Go Telegram runtime** — Bot API polling, command routing and Telegram actions.
- **Python policy service** — risk and automation decisions behind a private service boundary.
- **PostgreSQL** — durable chat settings, membership records and moderation audit history.
- **Redis** — rate counters, short-lived state, anti-flood windows and caching.
- **Telegram-only administration** — no web dashboard is planned.

## Development status

This repository is being built in verified stages. A feature is not considered complete until authorization, bot permissions, persistence, error handling and tests are implemented together.

| Area | Status |
|---|---|
| Go Telegram client foundation | Foundation prepared |
| Python policy service | Health endpoint and non-enforcing rate telemetry |
| PostgreSQL schema | Initial chat, member, moderation and filter tables |
| Redis integration | Connection and counter foundation |
| Moderation commands | In progress / not yet production-ready |
| Anti-spam, anti-raid and CAPTCHA | Planned |
| Locks, filters, notes, rules, welcome and reports | Planned |
| Deployment hardening and CI | Planned |

## Security requirements

- Never commit bot tokens, API keys or database credentials.
- Check the invoking user's admin rights and the bot's required rights before every moderation action.
- Never let the bot moderate chat creators or administrators unless Telegram explicitly allows the operation.
- Persist an audit record for every successful enforcement action.
- Escape untrusted Telegram text before inserting it into HTML.
- Use signed/authorized internal service communication when the policy service is deployed beyond a private Docker network.

## Local development

Requirements: Go 1.24+, Python 3.12+, Docker Compose.

1. Copy `.env.example` to `.env` and set `BOT_TOKEN`.
2. Start infrastructure: `docker compose up -d --build postgres redis policy`.
3. Run Go checks: `go test ./...`.
4. Start the bot: `go run ./cmd/anitoon4bot`.

The initial source bundle and reference material are being integrated incrementally. Do not deploy this project as a production moderation bot until the command modules and automated tests are complete.

## Reference material

The legacy Python bot archive is used only as a feature checklist and compatibility reference. It is not imported into the new Go runtime. Preserve upstream license notices for vendored dependencies and review them before redistribution.
