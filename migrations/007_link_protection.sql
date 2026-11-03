-- Per-bot and per-group link policy and trusted domain allowlist.
CREATE TABLE IF NOT EXISTS link_policy_configs (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id)
);

CREATE TABLE IF NOT EXISTS link_allowlist (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    domain TEXT NOT NULL CHECK (length(domain) BETWEEN 3 AND 253),
    added_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id, domain)
);
CREATE INDEX IF NOT EXISTS idx_link_allowlist_bot_chat
    ON link_allowlist(bot_id, chat_id);
