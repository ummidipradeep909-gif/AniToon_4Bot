Multi-instance deployment: clone polling leases

PostgreSQL is the sole authority for clone polling ownership; Redis is not used for distributed ownership.

Lease rules:
- One durable lease row per Telegram bot ID.
- Atomic acquisition; a live lease cannot be stolen.
- Expired takeover increments a monotonic fencing token.
- Renewal requires matching bot ID, instance ID, fencing token, and an unexpired lease. Expired leases cannot be resurrected.
- Any renewal error or ambiguous result is treated as lease loss; the local polling loop is stopped.
- Release is fenced, so a stale process cannot release a newer owner's lease.
- Unclean death leaves a lease to expire before takeover.
- Default TTL 90 seconds; renewal every 20 seconds.

Rollout boundary:
Clone pollers acquire and renew leases. Primary bot polling remains single-instance. Webhook mode also remains single-instance because Telegram has one webhook destination per token. Do not scale the full Compose service yet. All clone-worker replicas must share PostgreSQL and the same stable CLONE_ENCRYPTION_KEY. Never regenerate that key.

Operational note: PostgreSQL outage prevents new acquisitions and stops current pollers when renewal cannot be confirmed. Recovery occurs after PostgreSQL returns and leases expire. Telegram does not accept fencing tokens, so an API request already in flight during lease-loss shutdown cannot be fenced retroactively; this is not an exactly-once guarantee.