# FEATURE_user_profile — Business Context

- **Last updated:** 2026-04-02
- **Status:** draft
- **Issue:** [#93](https://github.com/courtknights/courtknights/issues/93)

---

## Problem

Once a user is created via authentication, there is no way to enrich their identity with personal or sport-specific information. The platform only stores the minimal identity record required for authentication (email, display name from OAuth provider, role). This blocks features that depend on knowing who a player is, how they play, or how to find them within the community.

---

## Goal

Add a **user profile** system that allows each user to maintain a richer public identity beyond the authentication record. The profile is created automatically at registration (pre-filled from the OAuth provider) and can be updated at any time by the owner.

---

## Profile fields

### Identity

| Field | Type | Notes |
|-------|------|-------|
| `display_name` | string | Public name, changeable by the user. Pre-filled from the OAuth provider `name` at registration. |

### Location

| Field | Type | Notes |
|-------|------|-------|
| `city` | string | Optional. Free text — no validation applied. |
| `region` | string | Optional. ISO 3166-2 subdivision code (e.g. `ES-MD` for Community of Madrid). Validated against the country. |
| `country` | string | Optional. ISO 3166-1 alpha-2 code (e.g. `ES`, `US`). Validated on write. |

Country and region are validated using an embedded dataset (no external API). City is free text to accommodate the wide variety of naming conventions across locales. If `region` is provided, `country` must also be provided.

### Preferences

Stored as a **JSONB column** in PostgreSQL. This allows new preference fields to be added in the future without a schema migration.

| Preference | Type | Values |
|------------|------|--------|
| `court_side` | enum | `drive`, `backhand`, `both` |
| `handedness` | enum | `right`, `left` |

The preferences schema is open: additional fields can be introduced in future features by convention, without altering the database schema.

---

## User stories

### US-01 — View my own profile

> As a signed-in user, I want to see my profile so that I can review what information is visible to others.

**Acceptance criteria:**
- `GET /api/v1/users/me/profile` returns the authenticated user's full profile.
- The response includes display name, location fields, and preferences.

---

### US-02 — Update my profile

> As a signed-in user, I want to update my profile so that my display name, location, and sport preferences reflect my current situation.

**Acceptance criteria:**
- `PUT /api/v1/users/me/profile` updates the authenticated user's profile.
- Partial updates are supported — only the fields included in the request body are changed.
- The updated profile is returned in the response.

---

### US-03 — View another user's profile

> As a signed-in user, I want to view another user's profile by their ID so that I can learn about other players in the community.

**Acceptance criteria:**
- `GET /api/v1/users/:id/profile` returns the profile for any user by their UUID.
- Any authenticated user can view any other user's profile.
- Returns `404` if the user does not exist.

---

### US-04 — List users

> As a signed-in user, I want to browse the list of users on the platform so that I can discover other players.

**Acceptance criteria:**
- `GET /api/v1/users` returns a paginated list of users.
- The caller can select which fields to include via a `fields` query parameter (e.g. `?fields=id,display_name,country`).
- Pagination is cursor-based or page-based (decided in architecture phase).
- Any authenticated user can call this endpoint.

---

## Pre-fill at registration

When a new user is created via OAuth (Google or GitHub), their profile is initialised automatically:

- `display_name` ← set from the OAuth provider `name`.
- All other fields are left empty.

The user can update all fields at any time after registration.

---

## Visibility

All profile fields are public to any authenticated user. There are no per-field privacy settings in this iteration.

---

## Out of scope

- Avatar / photo upload
- Additional fields pre-filled from OAuth provider (e.g. avatar URL, locale)
- Per-field privacy settings
- Profile search or filtering beyond field selection on the list endpoint
- Profile deletion (covered by account deletion, out of scope in FEATURE_authentication)

---

## Dependencies

- **FEATURE_authentication** — the User entity and JWT middleware are prerequisites
- **ADR-003** (PostgreSQL) — profile data is persisted in PostgreSQL; preferences use JSONB
- **ADR-001** (Echo) — profile endpoints are implemented as Echo handlers
