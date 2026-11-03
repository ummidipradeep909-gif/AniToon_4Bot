# AniToon_4Bot implementation roadmap

The aim is a broad, reliable group-management bot. The list below is a target inventory, not a claim that every module is already implemented.

## Foundation
- [x] Go polling runtime and command dispatcher
- [x] Python FastAPI policy service skeleton
- [x] PostgreSQL and Redis Compose services
- [x] Initial durable schema
- [x] CI checks for Go and Python

## Moderation core
- [ ] Shared actor-admin and bot-permission authorization
- [ ] Reply/ID target resolver and protected-target checks
- [x] Ban and unban command handlers (initial version; audit persistence and integration tests remain required)\n- [ ] Kick, mute and unmute
- [ ] Warning history, thresholds and reset
- [ ] Purge and message deletion
- [ ] Slow mode
- [ ] Moderation audit log and log-channel integration

## Group protection
- [ ] Per-chat locks and unlocks
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
