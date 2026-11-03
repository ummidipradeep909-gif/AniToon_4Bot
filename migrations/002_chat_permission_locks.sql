-- Store each group's original Telegram defaults before the first lock so
-- /unlock can restore the exact pre-lock settings after a restart.
CREATE TABLE IF NOT EXISTS chat_permission_locks (
    chat_id BIGINT PRIMARY KEY REFERENCES chats(chat_id) ON DELETE CASCADE,
    original_permissions JSONB NOT NULL,
    locked_types TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
