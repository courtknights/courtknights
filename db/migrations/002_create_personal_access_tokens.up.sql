-- Migration 002 (up): create the personal_access_tokens table.

CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash    VARCHAR(255) NOT NULL,
    salt        VARCHAR(255) NOT NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
