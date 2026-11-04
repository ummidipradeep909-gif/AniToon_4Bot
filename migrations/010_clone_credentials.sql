-- One durable clone per Telegram owner, and one owner per clone bot.
-- Bot tokens are AES-256-GCM ciphertext; the encryption key is held outside PostgreSQL.
CREATE TABLE IF NOT EXISTS clone_credentials (
    bot_id BIGINT PRIMARY KEY,
    owner_id BIGINT NOT NULL UNIQUE,
    encrypted_token BYTEA NOT NULL,
    nonce BYTEA NOT NULL CHECK (octet_length(nonce) = 12),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
