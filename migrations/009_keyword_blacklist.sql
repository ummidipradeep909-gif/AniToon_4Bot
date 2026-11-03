-- Per-bot, per-group keyword blacklist and enforcement mode.
CREATE TABLE IF NOT EXISTS keyword_policy_configs (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    mode TEXT NOT NULL DEFAULT 'delete' CHECK (mode IN ('delete','warn')),
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id)
);

CREATE TABLE IF NOT EXISTS keyword_blacklist (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    phrase TEXT NOT NULL CHECK (length(phrase) BETWEEN 2 AND 320),
    added_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id, phrase)
);
CREATE INDEX IF NOT EXISTS idx_keyword_blacklist_bot_chat
    ON keyword_blacklist(bot_id,chat_id);
