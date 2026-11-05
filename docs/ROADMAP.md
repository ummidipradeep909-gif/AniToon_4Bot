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
- [x] Owner-only per-clone status, pause, resume and removal controls with persistent state, atomic shared quota and independent polling isolation

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
- [x] AI-assisted contextual keyword blacklist review with evasion candidates and Redis cooldown (admin suggestion only)
- [x] Per-bot/per-group named notes and exact-match custom reply filters with admin-managed CRUD
- [x] On-demand 24-hour/7-day moderation reports and opt-in per-bot/per-group ban, unban and kick alerts (scheduled delivery/log-channel routing remain future work)
- [x] User information, reply/numeric-ID lookup, expanded chat/user ID output and live group administrator list (Telegram does not expose private contact details or reliable join dates through these methods)

## Operations
- [x] Admin-only aggregate moderation metrics by day/week/month from confirmed audit records (automated protection-event telemetry remains separate)
- [x] PostgreSQL backup/restore guide covering encrypted dumps, clone-key escrow, isolated restore validation, migrations and rollback
- [x] Embedded PostgreSQL migration runner with numeric ordering, transactional execution, advisory locking, checksum/history validation, startup fail-fast behavior and PostgreSQL integration tests
- [x] Primary-bot webhook mode with HTTPS URL/secret validation, platform PORT support, clone polling compatibility and SIGINT/SIGTERM graceful updater shutdown
- [x] Hardened single-host production Compose profile for private PostgreSQL/Redis, health-gated dependencies, persistent volumes, and container hardening
- [x] Operator-driven PostgreSQL and Redis password rotation runbook with maintenance sequencing, dependency validation and rollback
- [x] Docker Secrets overlay and file-based secret loading for bot, database, Redis, policy authentication, AI provider and stable clone encryption credentials; legacy environment variables remain supported
- [ ] Automated/zero-downtime credential rotation with dual credentials and managed secret-manager integration
- [x] Encrypted PostgreSQL backup tooling with age public-key encryption, SHA-256 manifests, archive verification and isolated restore drill safeguards
- [x] Read-only migration-ledger and clone ciphertext authentication verification for restored databases
- [x] Backup manifest unit tests and CI coverage
- [ ] Scheduled off-host backup orchestration, retention policy enforcement and alerting
- [ ] Load, abuse and recovery tests
- [x] PostgreSQL-backed clone polling leases, fenced takeover, renewal and stop-on-lease-loss
- [ ] Primary-bot polling supervisor and multi-instance deployment readiness


- [x] Feature 16: owner-only private dashboard for process uptime, runtime memory/goroutines, PostgreSQL pool health, Redis connectivity, and aggregate clone health.
