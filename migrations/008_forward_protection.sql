-- Per-bot and per-group forwarded-message protection settings.
CREATE TABLE IF NOT EXISTS forward_policy_configs (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id)
);
