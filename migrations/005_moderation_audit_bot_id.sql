-- Distinguish the master bot and each clone in the shared moderation log.
ALTER TABLE moderation_actions
    ADD COLUMN IF NOT EXISTS bot_id BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS moderation_actions_bot_chat_created_idx
    ON moderation_actions (bot_id, chat_id, created_at DESC, id DESC);
