CREATE TABLE bot_polling_leases (
    bot_id BIGINT PRIMARY KEY CHECK (bot_id > 0),
    instance_id TEXT NOT NULL CHECK (length(instance_id) BETWEEN 1 AND 128),
    fencing_token BIGINT NOT NULL CHECK (fencing_token > 0),
    lease_expires_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX bot_polling_leases_expiry_idx
    ON bot_polling_leases (lease_expires_at);
