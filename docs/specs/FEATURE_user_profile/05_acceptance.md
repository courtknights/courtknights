# FEATURE_user_profile — Acceptance Criteria

- **Last updated:** 2026-04-03
- **Status:** approved
- **Issue:** [#93](https://github.com/courtknights/courtknights/issues/93)
- **Author:** aagea


> Acceptance criteria are verified manually by the human reviewer before closing the feature issue. They complement — not replace — the automated test suite defined in `04_tests.md`.

---

## AC-01 — Profile initialised on first login

**Given** a user authenticates for the first time via OAuth (Google or GitHub)
**When** the OAuth callback completes successfully
**Then** a profile row exists in `user_profiles` for that user, with `display_name` set to the name provided by the OAuth provider and all other fields empty.

**And** if the same user authenticates again, the existing profile is not overwritten.

---

## AC-02 — View own profile

**Given** a signed-in user
**When** they call `GET /api/v1/users/me/profile`
**Then** the response is `200 OK` with a JSON body containing:
- `user_id`, `display_name`, `updated_at` (always present)
- `city`, `region`, `country`, `gender`, `date_of_birth`, `category`, `preferences` (present; may be `null` if not set)

---

## AC-03 — Update own profile (partial)

**Given** a signed-in user with an existing profile
**When** they call `PUT /api/v1/users/me/profile` with a subset of fields
**Then** the response is `200 OK` with the updated profile; only the submitted fields have changed; all other fields retain their previous values.

---

## AC-04 — Update own profile (full)

**Given** a signed-in user
**When** they call `PUT /api/v1/users/me/profile` with all fields populated and valid
**Then** the response is `200 OK` and every field in the response matches the submitted values.

---

## AC-05 — Location validation on update

**Given** a signed-in user
**When** they submit a `PUT /api/v1/users/me/profile` with an invalid ISO 3166-1 country code
**Then** the response is `400 Bad Request`.

**And** when they submit a valid country code with a region code that does not belong to that country
**Then** the response is `400 Bad Request`.

**And** when they submit a region code without a country code
**Then** the response is `400 Bad Request`.

---

## AC-06 — View another user's profile

**Given** a signed-in user and a second user with a profile
**When** the first user calls `GET /api/v1/users/:id/profile` with the second user's UUID
**Then** the response is `200 OK` with the second user's full profile.

---

## AC-07 — Profile not found

**Given** a signed-in user
**When** they call `GET /api/v1/users/:id/profile` with a UUID that does not correspond to any user
**Then** the response is `404 Not Found`.

---

## AC-08 — List users (default pagination)

**Given** at least one user with a profile exists
**When** a signed-in user calls `GET /api/v1/users` with no query parameters
**Then** the response is `200 OK` with:
- `data`: array of user objects ordered by `display_name` ascending
- `page`: `1`
- `page_size`: `20`
- `total`: total number of users in the platform

---

## AC-09 — List users (field selection)

**Given** multiple users with profiles
**When** a signed-in user calls `GET /api/v1/users?fields=id,display_name,country`
**Then** each object in `data` contains only `id`, `display_name`, and `country`; no other fields are present.

---

## AC-10 — List users (pagination bounds)

**Given** a signed-in user
**When** they call `GET /api/v1/users?page_size=101`
**Then** the response is `400 Bad Request`.

**And** when they call `GET /api/v1/users?page=0`
**Then** the response is `400 Bad Request`.

---

## AC-11 — Unauthenticated access is rejected

**Given** an unauthenticated request (no JWT or expired JWT)
**When** any profile endpoint is called (`GET /users/me/profile`, `PUT /users/me/profile`, `GET /users/:id/profile`, `GET /users`)
**Then** the response is `401 Unauthorized`.

---

## AC-12 — Migration is reversible

**Given** a PostgreSQL instance with the `users` table and existing user rows
**When** migration `003_create_user_profiles.up.sql` is applied
**Then** the `user_profiles` table exists and contains one row per pre-existing user.

**And** when `003_create_user_profiles.down.sql` is applied
**Then** the `user_profiles` table is dropped and the `users` table is unchanged.
