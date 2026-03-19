-- personal_access_tokens: stores hashed PAT credentials.
-- The raw token value is NEVER stored — only key_hash + salt.
-- A PAT is linked to exactly one user via users.provider_id = personal_access_tokens.id.

CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash    VARCHAR(255) NOT NULL,
    salt        VARCHAR(255) NOT NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
