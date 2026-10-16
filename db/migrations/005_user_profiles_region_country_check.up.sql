-- Migration 005 (up): a profile's region must belong to its country (#121).
--
-- ISO 3166-2 region codes start with the ISO 3166-1 alpha-2 country code and a
-- hyphen (ES-MD for ES), so the check rejects a region without a country or
-- from another country. Full code validity stays in the domain
-- (profile.ValidateLocation). Every row written so far went through that
-- validation, so existing data already satisfies the constraint; if a row does
-- not, this migration fails instead of silently changing data.

ALTER TABLE user_profiles
    ADD CONSTRAINT user_profiles_region_country_check
    CHECK (region IS NULL OR (country IS NOT NULL AND region LIKE country || '-%'));
