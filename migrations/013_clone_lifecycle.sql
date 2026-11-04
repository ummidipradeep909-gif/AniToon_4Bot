-- Persistent owner-controlled clone lifecycle state.
ALTER TABLE clone_credentials
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT TRUE;
CREATE INDEX IF NOT EXISTS clone_credentials_enabled_idx
    ON clone_credentials (enabled);
