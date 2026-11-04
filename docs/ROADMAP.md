# AniToon_4Bot implementation roadmap

The aim is a broad, reliable group-management bot. The list below is a target inventory, not a claim that every module is already implemented.

## Foundation
- [x] Go polling runtime and command dispatcher
- [x] Python FastAPI policy service skeleton
- [x] PostgreSQL and Redis Compose services
- [x] Initial durable schema
- [x] CI checks for Go and Python

## Clone bot system
- [x] Main-bot /clone onboarding and BotFather link
- [x] Private token message deletion and Telegram getMe validation
- [x] Start one clone per owner on shared dispatcher
- [x] Encrypted AES-256-GCM token storage with bot/owner authenticated identity
- [x] PostgreSQL uniqueness for clone owner and bot ID
- [x] Startup credential decryption, Telegram validation and per-clone recovery isolation
- [x] Refuse clone creation when PostgreSQL or the stable encryption key is unavailable
- [ ] Per-clone lifecycle controls, quotas and isolation
- [ ] Per-clone lifecycle controls, quotas and isolation

## Moderation core
- [x] Shared actor-admin and bot-permission checks (initial command-level implementation)
- [x] Reply/ID target resolver and protected-target checks
- [x] Ban and unban command handlers with PostgreSQL audit records
- [x] Mute and unmute command handlers with PostgreSQL audit records (chat-default permission preservation remains required)
- [x] Kick using Telegram ban-then-unban sequence with failure recovery and PostgreSQL audit records
- [x] PostgreSQL-backed warning history, listing, reset and audit records (three-warning admin review notice; no automatic punishment)
- [x] /del and /purge with verified admin and bot delete-permission checks, a 100-message limit and partial-failure reporting
- [x] Per-bot/per-group Redis-backed slow mode with admin/command exemptions and excess-message deletion (not Telegram native slow mode)
- [x] PostgreSQL moderation audit log with per-bot/per-group `/modlog` (log-channel delivery not included)

## Group protection
- [x] Per-chat permission locks and unlocks with persisted original permission snapshots, category-level controls and serialized updates
- [x] Redis-backed per-bot/per-chat anti-flood with admin exemptions, sliding windows and configurable delete/mute/ban actions (integration/load tests remain required)
- [x] Redis-backed join-wave detection with per-bot/per-chat thresholds, admin exemptions and expiring quarantine (Telegram join integration tests remain required)
- [x] URL and Telegram invite protection with per-bot/per-group trusted-domain allowlist
- [x] Forwarded-message protection with per-bot/per-group settings and verified admin exemptions (media policies remain separate)
- [x] Per-bot/per-group keyword blacklist with case-insensitive text/caption matching, admin exemptions, delete/warn modes and PostgreSQL persistence (automated handler/integration tests remain required)

## Community
- [x] Per-bot/per-group welcome and goodbye messages with admin commands, safe placeholders and HTML escaping
- [x] Per-bot/per-group persistent group rules with admin configuration and private delivery
- [x] Shared OpenAI assist foundation with authenticated policy API
- [x] AI-assisted anti-flood review (admin suggestion only)
- [x] AI-assisted anti-raid review using aggregate join telemetry
- [x] AI-assisted phishing review for blocked links (admin suggestion only)
- [ ] Notes and custom filters
- [ ] Reports and admin notifications
- [ ] User information and admin list

## Operations
- [ ] Structured moderation metrics
- [ ] Database backup and restore guide
- [ ] Migration runner
- [ ] Webhook mode and graceful shutdown
- [ ] Load, abuse and recovery tests
- [ ] Production deployment profiles and secret rotation
