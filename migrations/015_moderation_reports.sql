-- Per-bot, per-group opt-in for moderation event notifications.
CREATE TABLE IF NOT EXISTS chat_report_settings (
    bot_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
    notifications_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, chat_id)
);

CREATE INDEX IF NOT EXISTS chat_report_settings_enabled_idx
    ON chat_report_settings (bot_id, chat_id)
    WHERE notifications_enabled = TRUE;
