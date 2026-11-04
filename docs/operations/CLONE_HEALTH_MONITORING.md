# Clone health monitoring and recovery

The primary process runs a background reconciliation loop (default 60-second
interval). It reads the durable clone registry and compares each record's desired
enabled state with the in-process updater state.

- Enabled records missing from memory are revalidated against Telegram and restarted.
- Paused records are not restarted; a stale active poller is stopped where possible.
- Active clones receive a lightweight `getMe` probe. A failed probe is marked
  degraded but does not start a second polling loop; the updater already retries
  transient `getUpdates` failures.
- Recovery retries use capped exponential backoff from 5 seconds to 5 minutes.
- Invalid ciphertext, identity mismatch, owner conflicts, and webhook conflicts
  are surfaced as safe status labels and are not treated as transient success.
- Every recovery rechecks the persisted owner record and enabled flag before
  starting polling, so a stale snapshot cannot override a pause.
- Logs contain bot/owner IDs and status only, never Telegram tokens or ciphertext.

The monitor is single-process/in-memory coordination. Multi-instance lease-based
polling ownership remains Feature 17 and is required before running multiple bot
instances against the same clone registry.
