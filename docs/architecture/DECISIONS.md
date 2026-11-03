# Architecture decisions

## Runtime split

Go owns Telegram update polling, command dispatch and Telegram API operations. Python is a separately deployable policy service for risk signals and future automation. The Python service must not directly perform Telegram moderation.

## Persistence

PostgreSQL stores durable chat configuration, membership metadata, moderation history, warnings and filters. Redis stores ephemeral counters and rate windows. Durable moderation actions must be written to PostgreSQL.

## Authorization boundary

Every enforcement command must:
1. Resolve the target unambiguously (reply or numeric user ID).
2. Verify the actor is an administrator with the required privilege.
3. Verify the bot is an administrator with the corresponding Bot API right.
4. Protect chat creators and administrators from routine moderation.
5. Validate duration and reason inputs.
6. Perform the Telegram action and record the outcome.
7. Report errors without exposing secrets or internal stack traces.

## Reliability

- Use 64-bit Telegram identifiers.
- Use bounded timeouts for external calls.
- Keep handlers small and move persistence/business logic into services.
- Use migrations for schema changes.
- Test permission failures, Telegram API errors and duplicate updates.
- Do not enable punitive automod until thresholds, exemptions, appeals and audit records exist.

## Administration

All configuration and controls are delivered through Telegram commands and callbacks. There is no web dashboard in the product scope.
