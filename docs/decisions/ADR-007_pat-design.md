# ADR-007 — Personal Access Token (PAT) Design

- **Date:** 2026-03-18
- **Updated:** 2026-10-12 — PATs reference their user through `user_id` (#136)
- **Status:** accepted

---

## Context

The platform needs a non-interactive authentication path for CLI automation and scripted workflows (see ADR-006). Personal Access Tokens (PATs) fill this role: a long-lived opaque token that a client can exchange for a JWT without a browser.

Two concerns drive the design:
1. **Security** — the raw token must never be stored; a compromise of the database must not expose valid tokens
2. **Identity consistency** — a PAT-authenticated user must resolve to the same User record as any other authentication path, keeping the identity model uniform

---

## Decision

### Token storage

PATs are stored as a salted hash. The raw token is shown to the user once at creation time and never persisted.

| Field | Notes |
|-------|-------|
| `id` | PAT identifier (UUID) |
| `user_id` | The user the PAT authenticates as. `REFERENCES users(id) ON DELETE CASCADE` |
| `key_hash` | `hash(PAT_KEY, salt)` — the raw key is never stored |
| `salt` | Per-PAT random salt used for hashing |
| `expires_at` | Expiration date of the PAT. Null = no expiry. |

### Identity resolution

Each PAT references its user explicitly through `personal_access_tokens.user_id`.
A user can own any number of PATs, whatever their provider. Creating a PAT never
modifies the user row, so a Google or GitHub user keeps signing in with their
provider.

**PAT authentication flow:**
```
PAT_KEY → hash(PAT_KEY, salt) → PAT table (verify) → pat.user_id → User table (id) → JWT
```

`provider = pat` marks a user with no external identity (the bootstrap admin).
For such a user, `provider_id` is an opaque unique identifier; it is not a PAT
id.

> **Revision (2026-10-12, #136).** The original design stored the PAT `id` in
> `users.provider_id` (with `provider = pat`), mirroring OAuth2 resolution.
> That only worked for users whose provider is `pat`: creating a PAT for an
> OAuth user overwrote the user's provider identity, failed on the email
> uniqueness constraint, and left an orphan PAT. Migration 004 adds `user_id`,
> backfills it from the old link and deletes the PATs no user could resolve.

### Bootstrap PAT

A seed PAT for the initial `admin` user can be injected at first startup via environment variables:

| Variable | Description |
|----------|-------------|
| `COURTKNIGHTS_BOOTSTRAP_PAT` | Raw PAT value |
| `COURTKNIGHTS_BOOTSTRAP_EMAIL` | Email for the bootstrap user |
| `COURTKNIGHTS_BOOTSTRAP_NAME` | Display name for the bootstrap user |

On startup, if no users exist and these variables are set, the platform creates the admin user (`provider = pat`) and then a PAT record that references it.

---

## Consequences

- The raw PAT is never recoverable after creation — if lost, a new one must be generated
- Verifying a PAT requires fetching all PAT records for the user to find a hash match, or storing a lookup index (to be decided at implementation)
- The User identity model has no null fields — `provider_id` is always set regardless of provider type
- Any user, whatever their provider, can own several PATs; deleting a user deletes their PATs
- The admin user and its PAT are created by two separate inserts. If the second fails, the server exits with an error and, because a user now exists, bootstrap is skipped on the next start; the operator must create a PAT another way
- Bootstrap deployments require no browser; fully automated setup is possible via environment variables

---

## Alternatives considered

| Alternative | Reason rejected |
|-------------|-----------------|
| Store raw token encrypted | Encryption is reversible; a key compromise exposes all tokens. Hashing is irreversible. |
| Single global salt | Per-PAT salt prevents rainbow table attacks across tokens |
| PAT directly accepted by all API endpoints | Rejected in ADR-006 — JWT is the only API token |
| PAT id stored in `users.provider_id` (`provider = pat`) | Original design, replaced in #136: a user has a single `provider_id`, so a PAT could only be linked by overwriting the user's provider identity |
