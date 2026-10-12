# FEATURE_clearable_profile_fields — Business Context

- **Status:** in-review
- **Feature ID:** FEATURE_clearable_profile_fields
- **Issue:** [#121](https://github.com/courtknights/courtknights/issues/121)
- **Author:** aagea (issue draft) — expanded by the AI agent
- **Date:** 2026-10-12

---

## Problem statement

Once a profile field is set, the user cannot remove it. `PUT /api/v1/users/me/profile`
treats an omitted field and an explicit `null` the same way — both mean "leave
unchanged" — so there is no way to go back to "not set".

This matters most for personal data. A user who entered their `date_of_birth`
or `gender` cannot withdraw it, which is a privacy concern: the data stays
public to every authenticated user (FEATURE_user_profile, *Visibility*) for as
long as the account exists.

Two inconsistencies make the current endpoint harder to use correctly:

- **`preferences` is replaced wholesale**, while every other field is merged.
  Sending `{"preferences": {"court_side": "drive"}}` silently erases a stored
  `handedness`.
- **Changing `country` alone leaves a stale `region`** from the previous
  country (e.g. `country: FR`, `region: ES-MD`).

## Users affected

Every signed-in user, acting on their own profile (Player in
`docs/context/users.md`). No admin or organiser action is involved: a profile
is only ever edited by its owner.

## Business value

- **Privacy:** users can withdraw optional personal data at any time without
  deleting their account.
- **Predictability:** one standard, documented partial-update rule
  ([RFC 7396 — JSON Merge Patch](https://www.rfc-editor.org/rfc/rfc7396)) for
  every field, including `preferences`. Future clients (the Angular web app,
  the CLI) can rely on generic merge-patch tooling.
- **Data quality:** a profile can no longer end up with a region that belongs
  to another country.

Without it, the only way to remove personal data is a manual database edit.

## Scope

### In scope

**API**

- Replace `PUT /api/v1/users/me/profile` with
  `PATCH /api/v1/users/me/profile`, using JSON Merge Patch semantics:
  - field omitted → unchanged
  - field `null` → cleared
  - field with a value → set
- Remove the `PUT` endpoint. No client consumes it yet.
- Accept both `application/merge-patch+json` and `application/json` request
  bodies.

**Field rules**

- Clearable with `null`: `city`, `region`, `country`, `gender`,
  `date_of_birth`, `category`, `preferences`.
- `display_name: null` → `400`, because the display name is mandatory.
- Empty or whitespace-only strings → `400`. `null` is the only way to clear a
  field.
- `preferences` is merged recursively:
  - `{"preferences": {"handedness": null}}` removes only `handedness`;
  - `{"preferences": {"court_side": "drive"}}` sets `court_side` and keeps the
    other preferences;
  - `{"preferences": null}` clears all preferences.

**Location consistency**

- If `country` is cleared or changed and the request does not include
  `region`, the stored `region` is cleared too.
- `region` sent without `country` is validated against the stored country. It
  is rejected only if the profile has no country, or the region does not
  belong to it. This **supersedes** the "region without country → `400`"
  clause of FEATURE_user_profile AC-05.

### Out of scope

- Per-field privacy settings
- Account or profile deletion
- Stable ordering in `GET /users` (already delivered in #122)
- JSON Patch (RFC 6902) operations (`add`, `remove`, `replace`, …)
- Changes to the read endpoints (`GET /users/me/profile`,
  `GET /users/:id/profile`, `GET /users`) — their response shape is unchanged

## User stories

### US-01 — Withdraw a personal field

> As a signed-in user, I want to remove my date of birth (or gender, city,
> category, …) from my profile, so that I no longer share it.

`PATCH /api/v1/users/me/profile {"date_of_birth": null}` → `200`, and the
field reads as `null` afterwards. Other fields are unchanged.

### US-02 — Update one preference without losing the others

> As a signed-in user, I want to change my preferred court side without
> re-sending my handedness.

`{"preferences": {"court_side": "drive"}}` keeps the stored `handedness`.
`{"preferences": {"handedness": null}}` removes only `handedness`.

### US-03 — Move to another country

> As a signed-in user who moves abroad, I want my old region to disappear when
> I change my country, so that my profile stays consistent.

`{"country": "FR"}` on a profile with `ES` / `ES-MD` → `country: FR`,
`region: null`. `{"country": "FR", "region": "FR-IDF"}` sets both.

### US-04 — Set only my region

> As a signed-in user whose country is already set, I want to send only my
> region.

`{"region": "ES-CT"}` on a profile with `country: ES` → `200`. On a profile
without a country → `400`.

## Assumptions and dependencies

- **FEATURE_user_profile** (#93) is merged to `main` (#112) — done.
- The profile is only written by its owner and by `EnsureExists` at login, so
  concurrent writes to the same profile are rare.
- RFC 7396 semantics apply to the request body as a whole; there is no
  per-field "operation" syntax.
- Location validation keeps using the embedded ISO 3166 dataset
  (`github.com/biter777/countries`) — no new dependency.

## Glossary

| Term | Definition |
|------|-----------|
| JSON Merge Patch | RFC 7396 format for partial updates: the request body is a JSON object; omitted members are unchanged, `null` members are removed, other members replace the target value, and nested objects are merged recursively. Media type `application/merge-patch+json`. |
| Clear (a field) | Set an optional profile field back to "not set" (`null` in responses). |
