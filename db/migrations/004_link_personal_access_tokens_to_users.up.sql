-- Migration 004 (up): PATs reference their user explicitly (#136).
--
-- Before this migration a PAT was linked to a user by storing the PAT id in
-- users.provider_id (provider = 'pat'). Creating a PAT for an OAuth user
-- overwrote that user's provider_id, which broke their provider sign-in and
-- left orphan PATs behind.

ALTER TABLE personal_access_tokens
    ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Backfill: the only PAT a user could resolve was the one whose id was stored
-- in users.provider_id.
UPDATE personal_access_tokens p
SET user_id = u.id
FROM users u
WHERE u.provider = 'pat'
  AND u.provider_id = p.id::text;

-- Every other PAT is an orphan: no user can be resolved from it, so it could
-- never authenticate. Remove it.
DELETE FROM personal_access_tokens WHERE user_id IS NULL;

ALTER TABLE personal_access_tokens
    ALTER COLUMN user_id SET NOT NULL;

CREATE INDEX idx_personal_access_tokens_user_id ON personal_access_tokens (user_id);
