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
- /setflood [limit] [seconds] [delete|mute|ban], /flood and /clearflood for Redis-backed, per-bot/per-group anti-flood protection. Admins are exempt; settings are stored in PostgreSQL.
- /setraid [joins] [seconds] [quarantine_minutes], /raid and /clearraid for Redis-backed join-wave detection. When the threshold is reached, the bot temporarily restricts the regular members in that join wave; Telegram automatically expires the restrictions. Each clone has independent settings.
- /modlog [count] displays the latest 1–20 successful moderation actions made by the current bot in the current group. Logs are stored in PostgreSQL and isolated by bot and group.
- /slowmode [seconds] and /slowmode off configure a per-bot, per-group member message interval (1–3600 seconds). Redis enforces the interval by deleting excess messages; administrators and slash commands are exempt.
- /antilink on|off enables per-bot, per-group link protection. Non-admin messages containing unapproved external links or Telegram invite links are deleted; admins are exempt.
- /allowlink add <domain>, /allowlink remove <domain> and /allowlink list manage the group's trusted domains. Exact domains and their subdomains are allowed; Telegram invite links remain blocked.
- /antiforward on|off enables per-bot, per-group forwarded-message protection. Forwarded messages from regular members are deleted; verified administrators are exempt. The bot fails open if it cannot verify the sender's admin status.
- /addblacklist <phrase>, /unblacklist <phrase> and /blacklist manage up to 100 case-insensitive keyword/phrase rules per bot and group. Text and media captions are checked.
- /blacklistmode delete|warn selects whether matching messages are deleted only or deleted with a warning record. Administrators are exempt; enforcement fails open if the policy or sender role cannot be verified.
- `/setrules <text>`, `/rules`, `/privaterules` and `/clearrules` manage persistent group rules independently for each bot and group. Only verified administrators can set or clear rules; members can view them in the group or request a private copy.
- Mute durations: 30s, 15m, 2h, 3d, 1w, and combinations such as 1d12h. Omit the duration for a permanent mute. Temporary mutes are limited to 30 seconds–366 days.

## Create your own clone bot

The /start screen has a **🤖 Create Your Own Clone Bot** button that opens the official [@BotFather](https://t.me/BotFather).

1. Tap the button and send /newbot to BotFather.
2. Complete BotFather's setup and copy the new bot's API token.
3. Return to the **main AniToon_4Bot private chat** and send /clone.
4. Send the BotFather token as the next private message.
5. The bot deletes the token message, validates the token using Telegram's getMe API, encrypts it with AES-256-GCM, persists it in PostgreSQL, and then starts polling the new bot on the shared dispatcher. The clone receives the currently implemented /start, /help, /id, /ban, /unban, /kick, /mute, /unmute, /warn, /warnings, /resetwarn, /del, /purge, /lock, /unlock, /locks, /setflood, /flood, /clearflood, /setraid, /raid, /clearraid, /modlog, /slowmode, /antilink, /allowlink, /antiforward, /addblacklist, /unblacklist, /blacklist and /blacklistmode, /setrules, /rules, /privaterules and /clearrules handlers.

Use /cancel to cancel pending setup. Setup expires after 10 minutes. Only one clone per owner is supported. PostgreSQL enforces one row per owner and one owner per bot ID. A token is accepted only in the main bot's private chat; group token submissions are ignored. Do not share tokens with anyone.

### Persistent clone credentials and recovery

- Clone tokens are encrypted with AES-256-GCM before PostgreSQL storage. The table stores ciphertext and a fresh random 12-byte nonce only; plaintext tokens are never stored or logged.
- The bot ID and owner ID are authenticated as AES-GCM additional authenticated data. Changing either identity makes decryption fail.
- PostgreSQL enforces one clone per owner (`UNIQUE(owner_id)`) and one owner per clone bot (`PRIMARY KEY(bot_id)`). The primary AniToon_4Bot is never stored as a user's clone.
- New clones are validated with Telegram first, persisted before polling starts, and removed from storage if polling cannot start. A database failure never starts an unpersisted clone.
- At startup, saved credentials are decrypted, revalidated with Telegram and restarted independently. A failed clone recovery is logged by numeric IDs only and does not prevent the main bot from starting.
- New clone creation is refused unless both PostgreSQL and a valid encryption key are available. The main bot can still start when either is unavailable; clone recovery is skipped in that case.
- The clone's existing webhook must be removed before connection. If another host is polling the bot, stop it first.
- This remains a single-process clone runner, not a multi-instance hosting platform. Per-clone lifecycle controls, quotas, and rate-limit isolation are not implemented.
- The owner is responsible for the bot token and for configuring their clone's privacy and group administrator permissions.

**Encryption key setup (required for persistent clones):**

Generate the key once:

`openssl rand -base64 32`

Set the resulting value as `CLONE_ENCRYPTION_KEY` in the runtime environment. It must be a stable base64-encoded 32-byte key: do not regenerate it during normal deployments or restarts. Back it up securely, separately from the database backup. Losing or changing the key makes every saved clone token unrecoverable; restoring PostgreSQL without the matching key is insufficient.

Apply `migrations/010_clone_credentials.sql` to existing databases before deploying this version. New Docker-managed databases run it during initialisation.

## Welcome and goodbye messages

Welcome and goodbye settings are isolated by the current bot ID and group ID. Each clone can therefore use different templates in the same group.

- `/welcome` and `/goodbye` show the current state; use `/welcome on|off` and `/goodbye on|off` to toggle them.
- `/setwelcome <text>` and `/setgoodbye <text>` set custom templates; `/resetwelcome` and `/resetgoodbye` restore the defaults.
- Available placeholders: `{first}`, `{last}`, `{fullname}`, `{username}`, `{mention}`, `{id}`, and `{chatname}`.
- User and group values are HTML-escaped. Templates are plain text, up to 1000 characters.
- Only verified group administrators can change settings. Bots are not greeted. Membership promotions and restriction changes do not trigger greetings.
- Messages require Telegram `chat_member` updates and bot permission to send messages.

Apply `migrations/011_greetings.sql` to existing databases. New Docker-managed databases run it during initialisation.

## Group rules

Rules are stored separately by bot ID and group ID, so the main bot and every clone can have different rules in the same group.

- `/setrules <text>` saves or replaces this bot's rules for the current group. Only verified group administrators can change rules.
- `/rules` displays the saved rules in the group.
- `/privaterules` sends the rules to the invoking user's private chat. The user must have opened the bot and sent `/start` first so Telegram permits the bot to initiate the private conversation.
- `/clearrules` removes only this bot's rules for the current group. Only verified group administrators can clear them.
- Rules are plain text, not HTML or Markdown. Control characters other than newlines and tabs are rejected. The text is limited to 4000 UTF-16 units to stay within Telegram's message-size limit.
- PostgreSQL is required. If storage is unavailable, changes are not reported as successful.

Apply `migrations/012_group_rules.sql` to existing databases. New Docker-managed databases run migration SQL files during initialisation.

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

## Anti-flood protection

- Enable with `/setflood 5 10 delete`: more than five messages from one member within ten seconds triggers deletion of the over-limit messages.
- Choose `mute` or `ban` instead of `delete` to apply a 10-minute mute or a ban when the threshold is crossed. These actions require the bot's Restrict members permission; deletion mode requires Delete messages.
- `/flood` displays this bot's settings for the current group. `/clearflood` disables them.
- Limits are 2–50 messages and windows are 3–300 seconds. Admins are exempt. Each clone bot has its own settings, isolated by bot ID and group ID.
- Redis sorted sets perform atomic sliding-window counting with automatic expiry; PostgreSQL stores the configuration. If either service is unavailable, detection fails open rather than punishing members without a verified configuration.
- The moderation action is applied at most once per user per minute. Mute duration is currently fixed at 10 minutes.

Apply `migrations/003_antiflood.sql` to existing databases. New Docker-managed databases run it during initialisation.

## Moderation audit log

- `/modlog` shows the latest 10 actions; `/modlog 20` shows up to 20. Only verified group administrators can view it.
- Confirmed ban, unban, kick, mute, unmute, warn, warning reset, delete and purge actions are recorded with bot ID, group ID, actor ID, target ID (when applicable), reason/summary and timestamp.
- Main bot and clone records are isolated by bot ID. Audit persistence uses PostgreSQL.
- If Telegram confirms an action but the audit write fails, the action remains completed and the command reports that the log could not be saved.

Apply `migrations/005_moderation_audit_bot_id.sql` and `migrations/006_slowmode.sql` to existing databases. New Docker-managed databases run them during initialisation.

## Anti-raid protection

- Enable in a supergroup with `/setraid 5 10 10`: five or more new member joins within ten seconds trigger a raid response.
- The bot temporarily restricts the regular members in the detected join wave for ten minutes. Telegram automatically expires each restriction; the bot does not automatically ban new members.
- Configure 3–100 joins, a 5–300 second detection window and a 1–60 minute quarantine duration. `/raid` displays the current settings; `/clearraid` disables detection (existing Telegram restrictions continue until their expiry).
- Only group administrators can change settings. The bot must have `can_restrict_members`. Group administrators and bots are not quarantined.
- Redis tracks join events atomically; PostgreSQL stores per-bot/per-group settings. If either service is unavailable, the bot fails open and does not quarantine users.
- Telegram `chat_member` updates must be enabled for both the main bot and clone polling. The bot must be an administrator to receive these updates reliably.

Apply `migrations/004_antiraid.sql` to existing databases. New Docker-managed databases run it during initialisation.

## Slow mode

- `/slowmode 10` allows each regular member one message every 10 seconds. Supported intervals are 1–3600 seconds.
- `/slowmode` shows the current setting; `/slowmode off` disables it.
- Administrators and slash commands are exempt. Each clone has independent settings per group.
- The bot enforces this by deleting messages sent before a member's interval expires. This is not Telegram's native slow-mode setting. The bot needs **Delete messages**, PostgreSQL and Redis.
- If the database or Redis is unavailable, enforcement fails open and does not delete messages.

Apply `migrations/006_slowmode.sql` to existing databases. New Docker-managed databases run it during initialisation.

## Link protection

- `/antilink on` enables protection; `/antilink off` disables it; `/antilink` shows the current setting.
- `/allowlink add example.com`, `/allowlink remove example.com` and `/allowlink list` manage up to 100 visible trusted domains.
- Detects plain URLs, www links, bare domains, Telegram invite links and hidden text links in message entities. Text and media captions are inspected.
- Telegram invite links (`t.me/+...`, `joinchat` and `addlist`) are blocked even if `t.me` is allowlisted.
- Domain matching includes subdomains. Admins and bots are exempt. The bot needs Delete messages and PostgreSQL.
- If settings or allowlist storage cannot be verified, the handler fails open and does not delete messages.

For existing databases, apply these migrations individually and in order: `psql "$DATABASE_URL" -f migrations/007_link_protection.sql`, then `psql "$DATABASE_URL" -f migrations/008_forward_protection.sql`, then `psql "$DATABASE_URL" -f migrations/009_keyword_blacklist.sql`. New Docker-managed databases run migration SQL files during initialisation.

## Forward protection

- `/antiforward on` enables protection; `/antiforward off` disables it; `/antiforward` shows the current setting.
- Forwarded messages from regular members are deleted. Verified group administrators and bots are exempt.
- Detection uses Telegram's `forward_origin` field, including user, hidden-user, chat and channel origins.
- Settings are isolated by bot ID and group ID in PostgreSQL. The bot needs **Delete messages** permission.
- If the policy cannot be read or the sender's administrator status cannot be verified, the bot fails open and leaves the message untouched.

Apply `migrations/008_forward_protection.sql` to existing databases. New Docker-managed databases run it during initialisation.

## Keyword blacklist

- `/addblacklist bad phrase` adds a phrase; `/unblacklist bad phrase` removes it; `/blacklist` lists up to 100 phrases.
- Matching is case-insensitive and checks message text and media captions. Phrase matching uses substring semantics, not whole-word matching.
- `/blacklistmode delete` deletes a match; `/blacklistmode warn` deletes it and records a warning. `/blacklistmode` shows the selected action.
- Rules and mode are isolated by bot ID and group ID. Only verified group admins can manage them; regular-member messages are checked. Admins and bots are exempt.
- Phrase length is 2–80 characters. If database settings or admin status cannot be verified, the bot fails open and leaves the message untouched.

Apply `migrations/009_keyword_blacklist.sql` to existing databases. New Docker-managed databases run it during initialisation.

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

Notes, custom filters, reports, and production clone lifecycle management remain on the roadmap. The Python policy endpoint is telemetry only and does not automatically punish users.

## Development

Requirements: Go 1.24+, Python 3.12+, Docker Compose.

1. Copy .env.example to .env and set BOT_TOKEN, DATABASE_URL, REDIS_URL and CLONE_ENCRYPTION_KEY (generate the latter with `openssl rand -base64 32` and keep it stable/backed up) (PostgreSQL and Redis are required for anti-flood and anti-raid; PostgreSQL is also required for warnings, locks and slow-mode configuration; Redis is required for slow-mode enforcement).
2. Start services: docker compose up -d --build postgres redis policy. The initial migration creates the base tables. For existing databases, apply migrations/002_chat_permission_locks.sql through migrations/012_group_rules.sql in numeric order (one `psql -f` command per file) to enable the corresponding group-management features.
3. Run go test ./... and go vet ./....
4. Run the bot: go run ./cmd/anitoon4bot.

The Go module uses gotgbot v2.0.0-rc.37. GitHub Actions runs Go and Python checks on pushes and pull requests.

## Security

- Never commit bot tokens or credentials.
- Clone token messages are deleted before validation; tokens are encrypted with AES-256-GCM at rest and never logged or echoed. Back up the stable `CLONE_ENCRYPTION_KEY`; losing it makes saved clone tokens unrecoverable.
- Fail closed if administrator or bot permissions cannot be verified.
- Protect group creators and administrators from routine moderation.
- Keep untrusted user-provided reasons as plain text.
- Use 64-bit Telegram identifiers.
- Keep policy-service endpoints private or authenticate service-to-service calls.

See docs/ROADMAP.md and docs/architecture/DECISIONS.md for implementation status and security boundaries.
