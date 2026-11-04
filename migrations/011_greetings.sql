-- Per-bot, per-group welcome and goodbye configuration.
CREATE TABLE IF NOT EXISTS greeting_configs (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    welcome_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    goodbye_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    welcome_template TEXT NOT NULL DEFAULT E'👋 Welcome {mention} to {chatname}!\nPlease read the group rules.',
    goodbye_template TEXT NOT NULL DEFAULT '👋 {fullname} has left {chatname}.',
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id),
    CHECK (length(welcome_template) BETWEEN 1 AND 4000),
    CHECK (length(goodbye_template) BETWEEN 1 AND 4000)
);
CREATE INDEX IF NOT EXISTS greeting_configs_chat_idx ON greeting_configs(chat_id);
