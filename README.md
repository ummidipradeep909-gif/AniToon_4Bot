# AniToon_4Bot

A Telegram group-management bot with a Go runtime, Python policy-service foundation, PostgreSQL and Redis. Features are being added incrementally; this is not yet a complete Rose Bot replacement.

## Implemented commands

- /start, /help and /id.
- /clone and /cancel in the main bot's private chat: connect one user-owned Telegram bot created through @BotFather.
- /ban and /unban with administrator checks, bot permission checks, reply/numeric-ID targeting and protected administrator targets.
- /kick using Telegram's ban-then-unban sequence, with an explicit warning if the unban step fails.
- /mute and /unmute with temporary/permanent restrictions and duration validation.
- /warn, /warnings and /resetwarn with per-group PostgreSQL persistence, admin authorization and protected-target checks. At three warnings, the bot flags the member for admin review; it does not automatically punish them.
- /del to delete the replied-to message, and /purge to delete a replied-to range through the command or the latest 1–100 message IDs. Both require a verified group admin and the bot's explicit Delete messages permission; partial failures are counted and reported.
- /lock [type], /unlock [type] and /locks for per-group default-permission locks. Categories: messages, media, stickers, polls, previews, info, invites, pin, topics and reactions. The original Telegram permissions are saved in PostgreSQL and restored when the final lock is removed.
- Mute durations: 30s, 15m, 2h, 3d, 1w, and combinations such as 1d12h. Omit the duration for a permanent mute. Temporary mutes are limited to 30 seconds–366 days.

## Create your own clone bot

The /start screen has a **🤖 Create Your Own Clone Bot** button that opens the official [@BotFather](https://t.me/BotFather).

1. Tap the button and send /newbot to BotFather.
2. Complete BotFather's setup and copy the new bot's API token.
3. Return to the **main AniToon_4Bot private chat** and send /clone.
4. Send the BotFather token as the next private message.
5. The bot deletes the token message, validates the token using Telegram's getMe API, and starts polling the new bot on the shared dispatcher. The clone receives the currently implemented /start, /help, /id, /ban, /unban, /kick, /mute, /unmute, /warn, /warnings, /resetwarn, /del and /purge handlers.

Use /cancel to cancel pending setup. Setup expires after 10 minutes. Only one clone per owner is supported in this first version. A token is accepted only in the main bot's private chat; group token submissions are ignored. Do not share tokens with anyone.

### Clone hosting and credential limitations

- Tokens are held in process memory by the live bot client. They are not stored in PostgreSQL and are never written to logs or echoed in replies.
- Clone connections are lost when the service restarts; the owner must run /clone again to reconnect.
- The clone's existing webhook must be removed before connection. If another host is polling the bot, stop it first.
- This is an initial single-process clone runner, not a production multi-tenant hosting platform. Encrypted persistent token storage, per-owner quotas, clone lifecycle controls, and isolation/rate limits are not implemented yet.
- The owner is responsible for the bot token and for configuring their clone's privacy and group administrator permissions.

## Purge and delete commands

- Reply to a message with /del to delete only that message.
- Reply to the first message with /purge to delete that message through the /purge command (inclusive).
- Use /purge 20 to delete the latest 20 message IDs, including the /purge command.
- A single purge is limited to 100 message IDs. Telegram may refuse messages older than 48 hours or otherwise non-deletable messages; the bot reports successful and failed deletion counts separately.
- Both commands require the invoking user to be a verified group administrator and the bot to be an administrator with Telegram's **Delete messages** permission.

## Group permission locks

- `/lock` locks all supported default permissions for regular members.
- `/lock media` or `/lock stickers` locks only that category; supported types are `messages`, `media`, `stickers`, `polls`, `previews`, `info`, `invites`, `pin`, `topics` and `reactions`.
- `/unlock media` removes one category while preserving the other active locks. `/unlock` restores the complete original permission snapshot.
- `/locks` lists the active lock categories.
- Lock state and the original Telegram default permissions are stored per group in PostgreSQL, so they survive bot restarts. Changes require a verified group administrator and the bot's `can_restrict_members` permission.
- Locking fails closed if Telegram does not return the current default permissions. Changes are serialized per group to prevent concurrent lock commands from overwriting each other.

These commands change group default permissions for regular members; they do not override administrators or existing member-specific permission exceptions.

## Warning commands

- Reply to a member: /warn [reason]
- Numeric target: /warn 123456789 [reason]
- View history: reply with /warnings or use /warnings 123456789
- Reset history: reply with /resetwarn or use /resetwarn 123456789

Warning records are scoped to the group and stored in PostgreSQL. Only verified group admins may add, inspect or reset warnings. The bot will not warn the group owner or administrators. It shows an admin-review notice at three warnings but does not automatically mute or ban. If DATABASE_URL is absent or PostgreSQL is unavailable, these commands explain that warning storage is disabled.

## Moderation usage

- Reply to a member: /ban [reason]
- Numeric target: /ban 123456789 [reason]
- Remove a ban: /unban 123456789
- Reply to a member: /kick [reason]
- Numeric target: /kick 123456789 [reason]
- Reply to a member: /mute 2h [reason]
- Numeric target: /mute 123456789 2h [reason]
- Permanent mute: /mute 123456789
- Remove a restriction: /unmute 123456789 (or reply)

Only numeric IDs and reply targets are supported. Usernames are not resolved. Commands require a personally verifiable group administrator; anonymous-admin commands are rejected. The bot must have the relevant administrator permissions.

**Kick behaviour:** Telegram does not provide a separate kick method. The bot bans and immediately unbans the target. If the unban API call fails, the member remains banned; the bot reports this so an administrator can use /unban.

**Unmute limitation:** /unmute restores all permissions supported by the current Bot API. It does not preserve a member-specific restriction that was applied for another reason. Use it only when you intend to remove that member's restrictions.

## Not implemented yet

Slow mode, anti-flood enforcement, anti-raid, CAPTCHA, filters, notes, welcome/goodbye, reports, durable moderation audit logging, encrypted clone credential persistence, and production clone lifecycle management remain on the roadmap. The Python policy endpoint is telemetry only and does not automatically punish users.

## Development

Requirements: Go 1.24+, Python 3.12+, Docker Compose.

1. Copy .env.example to .env and set BOT_TOKEN and DATABASE_URL (PostgreSQL is required for warning commands).
2. Start services: docker compose up -d --build postgres redis policy. The initial migration creates the base tables. Apply migrations/002_chat_permission_locks.sql after it to enable persistent group locks.
3. Run go test ./... and go vet ./....
4. Run the bot: go run ./cmd/anitoon4bot.

The Go module uses gotgbot v2.0.0-rc.37. GitHub Actions runs Go and Python checks on pushes and pull requests.

## Security

- Never commit bot tokens or credentials.
- Clone token messages are deleted before validation; token strings are never logged or echoed.
- Fail closed if administrator or bot permissions cannot be verified.
- Protect group creators and administrators from routine moderation.
- Keep untrusted user-provided reasons as plain text.
- Use 64-bit Telegram identifiers.
- Keep policy-service endpoints private or authenticate service-to-service calls.

See docs/ROADMAP.md and docs/architecture/DECISIONS.md for implementation status and security boundaries.
