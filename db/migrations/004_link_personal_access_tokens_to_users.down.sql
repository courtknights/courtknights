-- Migration 004 (down): drop the explicit PAT → user reference.
--
-- After rolling back, a PAT only resolves if its id is stored in
-- users.provider_id (provider = 'pat'). PATs created for other users after
-- migration 004 stay in the table but can no longer authenticate.

DROP INDEX IF EXISTS idx_personal_access_tokens_user_id;

ALTER TABLE personal_access_tokens
    DROP COLUMN IF EXISTS user_id;
