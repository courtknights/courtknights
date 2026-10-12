-- personal_access_tokens: stores hashed PAT credentials.
-- The raw token value is NEVER stored — only key_hash + salt.
-- A PAT belongs to exactly one user via user_id; deleting the user deletes its PATs.

CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash    VARCHAR(255) NOT NULL,
    salt        VARCHAR(255) NOT NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_personal_access_tokens_user_id ON personal_access_tokens (user_id);
