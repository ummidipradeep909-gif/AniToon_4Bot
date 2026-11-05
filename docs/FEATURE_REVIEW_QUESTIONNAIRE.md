# AniToon_4Bot Feature Review

Reply with one option per question, e.g. `1B, 2A, 3D`. These are planning choices only; they do not change the bot automatically.

Security invariants remain fixed: AI is advisory-only; per-bot/per-group isolation remains; privileged commands fail closed when Telegram cannot verify an administrator; never regenerate the existing `CLONE_ENCRYPTION_KEY`.

1. Onboarding: A) current welcome/help B) guided button-based setup C) language selection D) guided setup plus languages.
2. Clone alerts: A) status only B) private degraded/recovered alerts C) recovery history D) alerts plus history.
3. Multi-instance: A) keep primary single-instance B) build primary polling supervisor C) add replica-count preflight D) B and C.
4. Dashboard: A) current metrics B) handler latency/throughput C) per-clone uptime D) resource history/alerts.
5. Moderation confirmation: A) current commands B) confirm ban/kick C) reason templates D) confirmation plus templates.
6. Warnings: A) current three-warning review notice B) configurable review threshold C) warning expiry and admin notifications D) B and C, without automatic punishment.
7. Anti-flood: A) configurable delete/mute/ban B) default delete C) graduated trust-based limits D) warning stage before configured action.
8. Anti-raid: A) current temporary quarantine B) alert-only mode C) manual review queue D) B and C; never AI auto-ban.
9. Links: A) current allowlist B) review uncertain links C) allowlist import/export D) B and C.
10. Community: A) current templates/rules/notes/filters B) rotating welcome templates C) note categories/search D) filter match statistics.
11. Reports: A) current on-demand reports B) scheduled daily/weekly reports C) audit-log channel D) B and C.
12. AI: A) current advisory-only B) evidence snippets C) per-group opt-in/confidence threshold D) B and C.
13. Backups: A) manual encrypted backup/restore drill B) scheduled retention C) off-host storage/alerts D) all three plus scheduled restore drills.
14. Security audit: A) permission checks B) clone isolation/key lifecycle C) container/network/dependency supply chain D) all, with severity-ranked fixes and regression tests.
15. Performance: A) keep conservative defaults pending measurements B) repeatable load-test baseline C) measured slow-query plans/indexes D) B and C.

Send your selections in chat to create a prioritized implementation plan. No option authorizes weaker moderation safety or secret handling.