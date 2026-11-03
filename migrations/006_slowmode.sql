-- Per-bot and per-group slow-mode policy. The bot enforces this by
-- deleting excess messages; Telegram's native slow-mode setting is not changed.
CREATE TABLE IF NOT EXISTS slowmode_configs (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    interval_seconds INTEGER NOT NULL CHECK (interval_seconds BETWEEN 1 AND 3600),
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id)
);
