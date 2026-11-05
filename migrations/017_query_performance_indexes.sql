-- Align warning-history lookups with their deterministic newest-first ordering.
-- The replacement index includes the primary-key tie-breaker used by /warnings.
CREATE INDEX IF NOT EXISTS chat_warnings_lookup_ordered_idx
    ON chat_warnings (chat_id, user_id, created_at DESC, id DESC);

DROP INDEX IF EXISTS chat_warnings_lookup_idx;
