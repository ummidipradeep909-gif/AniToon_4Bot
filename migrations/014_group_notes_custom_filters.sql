-- Make legacy chat filters independent for each bot and add named group notes.
-- Existing rows use bot_id=0 and are intentionally inactive until recreated by an admin.
ALTER TABLE chat_filters
    ADD COLUMN IF NOT EXISTS bot_id BIGINT NOT NULL DEFAULT 0;

ALTER TABLE chat_filters
    DROP CONSTRAINT IF EXISTS chat_filters_chat_id_trigger_key;

CREATE UNIQUE INDEX IF NOT EXISTS chat_filters_bot_chat_trigger_uidx
    ON chat_filters (bot_id, chat_id, lower(trigger));

CREATE INDEX IF NOT EXISTS chat_filters_bot_chat_idx
    ON chat_filters (bot_id, chat_id);

CREATE TABLE IF NOT EXISTS group_notes (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    content TEXT NOT NULL,
    created_by BIGINT NOT NULL,
    updated_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id, name),
    CHECK (length(name) BETWEEN 1 AND 32),
    CHECK (length(content) BETWEEN 1 AND 4000)
);

CREATE INDEX IF NOT EXISTS group_notes_bot_chat_idx
    ON group_notes (bot_id, chat_id, name);
