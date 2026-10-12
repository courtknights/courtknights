-- Migration 005 (down): drop the region/country consistency check.

ALTER TABLE user_profiles
    DROP CONSTRAINT IF EXISTS user_profiles_region_country_check;
