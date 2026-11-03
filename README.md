# AniToon_4Bot

An independently implemented Telegram group-management bot, designed around a Go Telegram runtime, a Python policy service, PostgreSQL and Redis. The goal is broad Rose-style functionality, delivered in tested stages.

## Implemented command handlers

- /start, /help and /id.
- /ban and /unban with live administrator checks, bot Restrict members permission checks, reply/numeric-ID targeting and protected administrator targets.
- /kick removes a member using Telegram's supported ban-then-unban sequence. If the second API call fails, the bot explicitly reports that the member may remain banned and gives the recovery command.
- /mute and /unmute with the same permission gates, protected-target checks, temporary/permanent restrictions and duration validation.
- Mute durations accept 30s, 15m, 2h, 3d, 1w and combinations such as 1d12h. Omit the duration for a permanent mute. Temporary mutes must be 30 seconds to 366 days.
- Python FastAPI health endpoint and non-enforcing per-user rate telemetry.
- PostgreSQL starter schema and Docker Compose infrastructure.
- GitHub Actions checks for Go tests/vet and Python syntax compilation.

## Create your own bot button\n\nThe /start message includes a **Create Your Own Bot** inline button that opens Telegram's official [@BotFather](https://t.me/BotFather). Users can create a new Telegram bot there with /newbot. The button is an onboarding shortcut only: personal AniToon clone provisioning, secure token intake/encryption, and multi-instance hosting are not implemented yet. Never send a bot token to an untrusted person or paste it into a public chat.\n\n## Moderation usage

- Reply to a member: /ban [reason]
- Numeric target: /ban 123456789 [reason]
- Remove a ban: /unban 123456789
- Reply to a member: /kick [reason]
- Numeric target: /kick 123456789 [reason]
- Reply to a member: /mute 2h [reason]
- Numeric target: /mute 123456789 2h [reason]
- Permanent mute: /mute 123456789
- Remove a restriction: /unmute 123456789 (or reply)

Only numeric IDs and reply targets are supported. Usernames are not resolved. Commands require a personally verifiable group administrator; anonymous-admin commands are rejected. The bot must be an administrator with Restrict members permission.

**Kick behaviour:** Telegram does not provide a separate kick method. The bot bans and immediately unbans the target. If the unban API call fails, the member remains banned; the bot reports this explicitly so an administrator can use /unban.

**Unmute limitation:** Telegram's unrestrict operation requires sending permissive chat permissions. This implementation restores all permissions supported by the current Bot API; it does not yet preserve a member-specific restriction that was applied for another reason. Use /unmute only when you intend to remove that member's restrictions.

## Important limitations

This is an incremental implementation, not a completed Rose replacement. Warnings, purge, locks, anti-flood enforcement, anti-raid, CAPTCHA, filters, notes, welcome/goodbye and reports are not implemented yet. The policy endpoint is telemetry only and never automatically punishes users. Do not deploy this as a complete production moderation bot.

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
- Keep untrusted user-provided reasons as plain text.
- Use 64-bit Telegram identifiers.
- Keep policy-service endpoints private or authenticate service-to-service calls.

See docs/ROADMAP.md for the staged feature inventory and docs/architecture/DECISIONS.md for security boundaries.
