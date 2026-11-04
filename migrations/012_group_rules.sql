-- Rules are isolated by bot and group so clones can maintain independent policies.
CREATE TABLE IF NOT EXISTS group_rules (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    rules TEXT NOT NULL,
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id),
    CHECK (length(rules) BETWEEN 1 AND 14000)
);
