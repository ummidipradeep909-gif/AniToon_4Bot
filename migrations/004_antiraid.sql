-- Per-bot anti-raid policy; each clone can protect the same group independently.
CREATE TABLE IF NOT EXISTS raid_configs (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    join_limit INTEGER NOT NULL CHECK (join_limit BETWEEN 3 AND 100),
    window_seconds INTEGER NOT NULL CHECK (window_seconds BETWEEN 5 AND 300),
    quarantine_minutes INTEGER NOT NULL CHECK (quarantine_minutes BETWEEN 1 AND 60),
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id)
);
