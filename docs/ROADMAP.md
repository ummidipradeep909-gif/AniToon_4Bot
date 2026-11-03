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
- [x] Ban and unban command handlers (audit persistence and integration tests remain required)
- [x] Mute and unmute command handlers (audit persistence and chat-default permission preservation remain required)
- [x] Kick using Telegram ban-then-unban sequence (failure recovery is reported; audit persistence remains required)
- [x] PostgreSQL-backed warning history, listing and reset (three-warning admin review notice; no automatic punishment)
- [x] /del and /purge with verified admin and bot delete-permission checks, a 100-message limit and partial-failure reporting
- [ ] Slow mode
- [ ] Moderation audit log and log-channel integration

## Group protection
- [x] Per-chat permission locks and unlocks with persisted original permission snapshots, category-level controls and serialized updates
- [ ] Anti-flood with exemptions and configurable actions
- [ ] Anti-raid join state machine
- [ ] CAPTCHA / join verification
- [ ] URL, invite, forward and media policies
- [ ] Blacklists and keyword rules

## Community
- [ ] Welcome and goodbye templates
- [ ] Rules and terms
- [ ] Notes and custom filters
- [ ] Reports and admin notifications
- [ ] User information and admin list
- [ ] AFK status
- [ ] RSS and scheduled announcements

## Operations
- [ ] Structured moderation metrics
- [ ] Database backup and restore guide
- [ ] Migration runner
- [ ] Webhook mode and graceful shutdown
- [ ] Load, abuse and recovery tests
- [ ] Production deployment profiles and secret rotation
