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
- [x] Start one clone per owner on shared dispatcher (in-memory only; restart reconnect required)
- [ ] Encrypted persistent token storage and clone recovery
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
- [x] URL and Telegram invite protection with per-bot/per-group trusted-domain allowlist (forward and media policies remain separate)
- [ ] Blacklists and keyword rules

## Community
- [ ] Welcome and goodbye templates
- [ ] Rules and terms
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
