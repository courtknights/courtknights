# ADR-007 — Personal Access Token (PAT) Design

- **Date:** 2026-03-18
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
| `key_hash` | `hash(PAT_KEY, salt)` — the raw key is never stored |
| `salt` | Per-PAT random salt used for hashing |
| `expires_at` | Expiration date of the PAT. Null = no expiry. |

### Identity resolution

The PAT `id` is stored as the `provider_id` in the User table (with `provider = pat`). This makes PAT resolution identical in structure to OAuth2 resolution:

```
OAuth2: (provider=google,  provider_id=google_user_id) → User
PAT:    (provider=pat,     provider_id=pat_id)         → User
```

**PAT authentication flow:**
```
PAT_KEY → hash(PAT_KEY, salt) → PAT table (verify) → pat_id → User table (provider_id = pat_id) → JWT
```

### Bootstrap PAT

A seed PAT for the initial `admin` user can be injected at first startup via environment variables:

| Variable | Description |
|----------|-------------|
| `COURTKNIGHTS_BOOTSTRAP_PAT` | Raw PAT value |
| `COURTKNIGHTS_BOOTSTRAP_EMAIL` | Email for the bootstrap user |
| `COURTKNIGHTS_BOOTSTRAP_NAME` | Display name for the bootstrap user |

On startup, if no users exist and these variables are set, the platform creates the admin user and the PAT record automatically.

---

## Consequences

- The raw PAT is never recoverable after creation — if lost, a new one must be generated
- Verifying a PAT requires fetching all PAT records for the user to find a hash match, or storing a lookup index (to be decided at implementation)
- The User identity model has no null fields — `provider_id` is always set regardless of provider type
- Bootstrap deployments require no browser; fully automated setup is possible via environment variables

---

## Alternatives considered

| Alternative | Reason rejected |
|-------------|-----------------|
| Store raw token encrypted | Encryption is reversible; a key compromise exposes all tokens. Hashing is irreversible. |
| Single global salt | Per-PAT salt prevents rainbow table attacks across tokens |
| PAT directly accepted by all API endpoints | Rejected in ADR-006 — JWT is the only API token |
