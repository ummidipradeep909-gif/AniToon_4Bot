-- Anti-flood settings are scoped to both the Telegram bot and group so each
-- clone can maintain independent moderation policy for the same chat.
CREATE TABLE IF NOT EXISTS flood_configs (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    message_limit INTEGER NOT NULL CHECK (message_limit BETWEEN 2 AND 50),
    window_seconds INTEGER NOT NULL CHECK (window_seconds BETWEEN 3 AND 300),
    action TEXT NOT NULL CHECK (action IN ('delete', 'mute', 'ban')),
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id)
);
