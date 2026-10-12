# FEATURE_clearable_profile_fields — Acceptance Criteria

- **Last updated:** 2026-10-12
- **Status:** approved
- **Issue:** [#121](https://github.com/courtknights/courtknights/issues/121)
- **Author:** aagea (drafted by the AI agent from issue #121)

> This document is owned by the human architect. It was drafted by the AI
> agent from the requirements of #121 and approved by the architect.

---

## AC-01 — PATCH replaces PUT

**Given** a signed-in user
**When** they call `PATCH /api/v1/users/me/profile` with `Content-Type: application/merge-patch+json` or `application/json`
**Then** the response is `200 OK` with the updated profile, in the same shape as `GET /api/v1/users/me/profile`.
**And** `PUT /api/v1/users/me/profile` no longer exists (`405 Method Not Allowed`).
**And** any other `Content-Type` returns `415 Unsupported Media Type`.

---

## AC-02 — Omitted fields are unchanged

**Given** a profile with every field set
**When** the user sends `{"category": "second"}`
**Then** only `category` changes; every other field, including `preferences`, keeps its value.
**And** `{}` returns `200` with the profile unchanged.

---

## AC-03 — `null` clears an optional field

**Given** a profile with `city`, `region`, `country`, `gender`, `date_of_birth`, `category` and `preferences` set
**When** the user sends `null` for one of them
**Then** that field reads as `null` (or `preferences` as `{}`) in the response and in later `GET` calls, and the other fields are unchanged.

---

## AC-04 — `display_name` cannot be cleared

**When** the user sends `{"display_name": null}`
**Then** the response is `400 Bad Request` and the profile is unchanged.

---

## AC-05 — Blank strings are rejected

**When** the user sends an empty or whitespace-only string for any string field (e.g. `{"city": "  "}`)
**Then** the response is `400 Bad Request` and the profile is unchanged.

---

## AC-06 — Preferences are merged recursively

**Given** stored preferences `{"court_side": "backhand", "handedness": "left"}`
**When** the user sends `{"preferences": {"court_side": "drive"}}`
**Then** preferences are `{"court_side": "drive", "handedness": "left"}`.
**And** `{"preferences": {"handedness": null}}` removes only `handedness`.
**And** `{"preferences": null}` clears all preferences.

---

## AC-07 — Changing or clearing the country clears a stale region

**Given** a profile with `country: ES`, `region: ES-MD`
**When** the user sends `{"country": "FR"}`
**Then** `country` is `FR` and `region` is `null`.
**And** `{"country": null}` clears both.
**And** `{"country": "FR", "region": "FR-IDF"}` sets both.

---

## AC-08 — Region alone is validated against the stored country

**Given** a profile with `country: ES`
**When** the user sends `{"region": "ES-CT"}`
**Then** the response is `200` and `region` is `ES-CT`.
**And** `{"region": "FR-IDF"}` returns `400`.
**And** on a profile without a country, `{"region": "ES-MD"}` returns `400`.

This supersedes the "region without country → 400" clause of FEATURE_user_profile AC-05.

---

## AC-09 — Invalid input is rejected without side effects

**When** the user sends an unknown member, an invalid enum, a malformed `date_of_birth`, an invalid country code, or a body that is not a JSON object
**Then** the response is `400 Bad Request` and the profile is unchanged.

---

## AC-10 — Unauthenticated access is rejected

**When** `PATCH /api/v1/users/me/profile` is called without a JWT or with an expired JWT
**Then** the response is `401 Unauthorized`.

---

## AC-11 — Migration is reversible

**Given** a database with valid profiles
**When** migration `005_user_profiles_region_country_check` is applied and rolled back
**Then** both directions succeed and the profile data is unchanged.

---

## Sign-off

| Role      | Name | Date | Status |
| --------- | ---- | ---- | ------ |
| Architect | aagea | 2026-10-12 | [x] approved |
