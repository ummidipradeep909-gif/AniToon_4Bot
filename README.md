# AniToon_4Bot

An independently implemented Telegram group-management bot, designed around a Go Telegram runtime, a Python policy service, PostgreSQL and Redis. The goal is broad Rose-style functionality, delivered in tested stages.

## Implemented

- Go long-polling runtime.
- /start, /help and /id.
- /ban and /unban with live administrator checks, bot Restrict members permission checks, reply/numeric-ID targeting, protected administrator targets and Telegram API error handling.
- Python FastAPI health endpoint and non-enforcing per-user rate telemetry.
- PostgreSQL starter schema and Docker Compose infrastructure.
- GitHub Actions checks for Go tests/vet and Python syntax compilation.

## Moderation command usage

- Reply to a member's message: /ban [reason]
- Numeric target: /ban 123456789 [reason]
- Remove a ban: /unban 123456789

Only numeric IDs and reply targets are supported. Usernames are not resolved because the Bot API does not provide a general username-to-user-ID lookup. Commands require a personally verifiable group administrator; anonymous-admin commands are rejected. The bot must be a group administrator with Restrict members permission.

## Important limitations

This is an incremental implementation, not a completed Rose replacement. Mute, kick, warnings, purge, locks, anti-flood enforcement, anti-raid, CAPTCHA, filters, notes, welcome/goodbye and reports are not implemented yet. The policy endpoint is telemetry only and never automatically punishes users. Do not deploy this as a complete production moderation bot.

## Development

Requirements: Go 1.24+, Python 3.12+, Docker Compose.

1. Copy .env.example to .env and set BOT_TOKEN.
2. Start services: docker compose up -d --build postgres redis policy.
3. Run go test ./... and go vet ./....
4. Run the bot: go run ./cmd/anitoon4bot.

The Go module uses gotgbot v2.0.0-rc.37. GitHub Actions runs Go and Python checks on pushes and pull requests.

## Security

- Never commit bot tokens or credentials.
- Fail closed if the actor's admin state cannot be verified.
- Verify bot permissions before calling Telegram moderation endpoints.
- Protect chat creators and administrators from routine moderation.
- Keep untrusted user-provided reasons as plain text (not HTML).
- Use 64-bit Telegram identifiers.
- Keep policy-service endpoints private or authenticate service-to-service calls.

See docs/ROADMAP.md for the staged feature inventory and docs/architecture/DECISIONS.md for security boundaries.
