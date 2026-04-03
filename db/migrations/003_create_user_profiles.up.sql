-- Migration 003 (up): create the user_profiles table and backfill existing users.

CREATE TABLE user_profiles (
    user_id       UUID         PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name  VARCHAR(255) NOT NULL,
    city          VARCHAR(255),
    region        VARCHAR(10),
    country       VARCHAR(2),
    gender        VARCHAR(10)  CHECK (gender IN ('male', 'female')),
    date_of_birth DATE,
    category      VARCHAR(10)  CHECK (category IN ('first','second','third','fourth','fifth')),
    preferences   JSONB        NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Backfill profiles for users created before this feature.
INSERT INTO user_profiles (user_id, display_name)
SELECT id, name FROM users
ON CONFLICT (user_id) DO NOTHING;
