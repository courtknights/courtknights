# FEATURE_authentication — Architecture

- **Last updated:** 2026-03-18
- **Status:** draft
- **Issue:** [#6](https://github.com/courtknights/courtknights/issues/6)

---

## Component overview

```
┌─────────────┐        ┌─────────────┐
│   Web UI    │        │     CLI     │
│  (Angular)  │        │   (Cobra)   │
└──────┬──────┘        └──────┬──────┘
       │                      │
       │  HTTP + JWT           │  HTTP + JWT
       ▼                      ▼
┌─────────────────────────────────────┐
│           Backend (Echo)            │
│                                     │
│  /auth/*   ──►  api/auth/handler    │
│  protected ──►  api/common/middleware│
│                      │              │
│              application/auth       │
│              (manager + service)    │
│                      │              │
│              infrastructure/        │
│          postgres  oauth2  jwt      │
└─────────────────────────────────────┘
```

All clients obtain a JWT from `/auth/*` and use it identically on protected endpoints. The backend does not distinguish between Web UI and CLI callers.

---

## Internal package structure

The authentication feature follows the clean architecture layers defined across the project, with a **manager layer** inside `application/` that orchestrates business logic across multiple domain repositories.

```
internal/
  domain/
    user/
      user.go                  # User entity, Role and Provider enums
      repository.go            # UserRepository interface
    pat/
      pat.go                   # PAT entity
      repository.go            # PATRepository interface
    ckerrors/
      auth.go                  # Domain errors (ErrUserNotFound, ErrInvalidPAT, …)
  application/
    auth/
      manager.go               # AuthManager — business logic combining UserRepository + PATRepository
      service.go               # Thin orchestration layer — calls AuthManager + external adapters (OAuth2, JWT)
  infrastructure/
    postgres/
      user_repository.go       # UserRepository implementation
      pat_repository.go        # PATRepository implementation
    oauth2/
      oauth2.go                # Generic Provider interface
      google.go                # Google OAuth2 provider adapter
      github.go                # GitHub OAuth2 provider adapter
    jwt/
      jwt.go                   # JWT signing and validation (HS256)
  api/
    router.go                  # Top-level router — registers all module route groups
    common/
      middleware.go            # JWT validation middleware (cross-cutting)
    auth/
      handler.go               # Echo handlers for /auth/* routes
      routes.go                # Auth route group registration
```

### Call chain

```
api/
    │
    ▼
application/auth/service      ← thin: delegates to manager + external adapters (OAuth2, JWT)
    │
    ▼
application/auth/manager      ← business logic: combines UserRepository + PATRepository
    │              │
    ▼              ▼
domain/user/   domain/pat/    ← repository interfaces (pure Go, no external imports)
repository     repository
    │              │
    ▼              ▼
infrastructure/postgres        ← concrete implementations
```

### Router structure

Two levels of route registration:

- **`api/router.go`** — creates the Echo instance, registers global middleware (`common/middleware.go`), and mounts all module route groups (`/auth`, `/api/v1/users`, `/api/v1/matches`, …). This is the only file that knows about all modules.
- **`api/<module>/routes.go`** — registers the specific endpoints for that module against the group received from `router.go`. Each module is self-contained.

### Manager responsibilities

`AuthManager` encapsulates all cross-repository business logic:

| Method                                              | Description                                                            |
| --------------------------------------------------- | ---------------------------------------------------------------------- |
| `ResolveByOAuth(provider, providerID, email, name)` | Upsert user from OAuth2 callback; creates on first login               |
| `ResolveByPAT(rawPAT)`                              | Hash-validates the raw PAT, resolves the linked User                   |
| `CreatePAT(userID, expiresAt)`                      | Generates, hashes, and stores a new PAT; returns raw value once        |
| `RevokePAT(id)`                                     | Deletes a PAT record                                                   |
| `BootstrapAdmin(email, name, rawPAT)`               | Creates the first admin user + PAT from env vars; no-op if users exist |

### Layer rules

- `domain/` — no external imports; pure Go types and repository interfaces only
- `domain/ckerrors/` — shared domain error sentinel values; no external imports
- `application/` — depends on `domain/` interfaces only; no infrastructure imports; manager + service live here
- `infrastructure/` — implements `domain/` interfaces; imports DB drivers, OAuth2 libraries, and JWT library
- `api/` — depends on `application/` interfaces; converts HTTP requests to use-case calls and back

---

## HTTP API contract

All request and response bodies are JSON. Authenticated endpoints require `Authorization: Bearer <jwt>`.

### Auth endpoints (public)

#### OAuth2 redirect flow

| Method | Path                    | Description                                           |
| ------ | ----------------------- | ----------------------------------------------------- |
| `GET`  | `/auth/google`          | Redirects the browser to Google's OAuth2 consent page |
| `GET`  | `/auth/google/callback` | Receives the OAuth2 code from Google; issues a JWT    |
| `GET`  | `/auth/github`          | Redirects the browser to GitHub's OAuth2 consent page |
| `GET`  | `/auth/github/callback` | Receives the OAuth2 code from GitHub; issues a JWT    |

**Callback response** (redirect to frontend with token in query param or fragment):
```
302 → <frontend_url>/auth/callback?token=<jwt>
```

#### OAuth2 Device Authorization Grant

| Method | Path                 | Description                                    |
| ------ | -------------------- | ---------------------------------------------- |
| `POST` | `/auth/device`       | Initiates the device flow for a given provider |
| `POST` | `/auth/device/token` | Polls for the JWT once the user has authorised |

**`POST /auth/device` request:**
```json
{ "provider": "google" }
```
**`POST /auth/device` response:**
```json
{
  "device_code": "…",
  "user_code": "ABCD-1234",
  "verification_uri": "https://accounts.google.com/device",
  "expires_in": 1800,
  "interval": 5
}
```

**`POST /auth/device/token` request:**
```json
{ "device_code": "…" }
```
**`POST /auth/device/token` response (success):**
```json
{ "token": "<jwt>", "expires_in": 3600 }
```
**`POST /auth/device/token` response (pending):**
```json
{ "error": "authorization_pending" }
```

#### PAT exchange

| Method | Path              | Description               |
| ------ | ----------------- | ------------------------- |
| `POST` | `/auth/token/pat` | Exchanges a PAT for a JWT |

**Request:**
```json
{ "pat": "<raw-pat-value>" }
```
**Response:**
```json
{ "token": "<jwt>", "expires_in": 3600 }
```

#### JWT refresh

| Method | Path            | Description                                    |
| ------ | --------------- | ---------------------------------------------- |
| `POST` | `/auth/refresh` | Issues a new JWT given a valid non-expired JWT |

**Request:** `Authorization: Bearer <jwt>` (no body required)
**Response:**
```json
{ "token": "<jwt>", "expires_in": 3600 }
```

### PAT management endpoints (protected — admin only)

| Method   | Path               | Description                          |
| -------- | ------------------ | ------------------------------------ |
| `POST`   | `/api/v1/pats`     | Create a PAT for a user              |
| `GET`    | `/api/v1/pats`     | List PATs for the authenticated user |
| `DELETE` | `/api/v1/pats/:id` | Revoke a PAT                         |

**`POST /api/v1/pats` request:**
```json
{ "user_id": "<uuid>", "expires_at": "2027-01-01T00:00:00Z" }
```
**`POST /api/v1/pats` response** (raw value shown once only):
```json
{ "id": "<pat-uuid>", "pat": "<raw-value>", "expires_at": "2027-01-01T00:00:00Z" }
```

### User management endpoints (protected)

| Method | Path                     | Description                                  |
| ------ | ------------------------ | -------------------------------------------- |
| `GET`  | `/api/v1/users/me`       | Returns the authenticated user's profile     |
| `PUT`  | `/api/v1/users/:id/role` | Promotes or demotes a user role (admin only) |

---

## Authentication flows

### Flow 1 — OAuth2 redirect (Web UI)

```
Browser          Backend            Google/GitHub
   │                │                    │
   │ GET /auth/google│                    │
   │───────────────►│                    │
   │                │── redirect ───────►│
   │                │                    │ user consents
   │                │◄── code ──────────│
   │ GET /auth/google/callback?code=…    │
   │───────────────►│                    │
   │                │── exchange code ──►│
   │                │◄── access token ──│
   │                │  fetch user info   │
   │                │◄── email, name ───│
   │                │  upsert User       │
   │                │  issue JWT         │
   │◄── 302 + JWT ──│                    │
```

### Flow 2 — OAuth2 Device Flow (CLI)

```
CLI              Backend            Google/GitHub        Browser
 │                  │                    │                  │
 │ POST /auth/device│                    │                  │
 │─────────────────►│                    │                  │
 │                  │── device request ─►│                  │
 │                  │◄── device_code ────│                  │
 │◄── user_code + verification_uri ──────│                  │
 │                  │                    │                  │
 │  (user opens verification_uri and enters user_code)      │
 │                  │                    │◄── user consents─│
 │                  │                    │                  │
 │ POST /auth/device/token (poll)        │                  │
 │─────────────────►│                    │                  │
 │                  │── poll ───────────►│                  │
 │                  │◄── access token ───│                  │
 │                  │  fetch user info   │                  │
 │                  │  upsert User       │                  │
 │                  │  issue JWT         │                  │
 │◄── JWT ──────────│                    │                  │
```

### Flow 3 — PAT exchange

```
Client           Backend            Database
  │                 │                   │
  │ POST /auth/token/pat { pat }        │
  │────────────────►│                   │
  │                 │── fetch PATs ────►│
  │                 │◄── PAT rows ─────│
  │                 │  hash(pat, salt)  │
  │                 │  compare hashes   │
  │                 │── fetch User ────►│
  │                 │   (provider_id = pat_id)
  │                 │◄── User ─────────│
  │                 │  issue JWT        │
  │◄── JWT ─────────│                   │
```

---

## Database schema

```sql
-- db/schema/users.sql
CREATE TABLE users (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email       VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(255) NOT NULL,
    role        VARCHAR(50)  NOT NULL DEFAULT 'user',
    provider    VARCHAR(50)  NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_id)
);

-- db/schema/personal_access_tokens.sql
CREATE TABLE personal_access_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash   VARCHAR(255) NOT NULL,
    salt       VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Constraints:**
- `(provider, provider_id)` is unique — one identity per provider per user
- `expires_at` is nullable — a PAT with no expiry is valid indefinitely
- No foreign key from `users.provider_id` to `personal_access_tokens.id` — the `provider` column acts as a discriminator; referential integrity is enforced at the application layer

---

## JWT middleware

The middleware runs on all protected routes. It:

1. Reads the `Authorization: Bearer <token>` header
2. Validates the JWT signature (HS256, secret from config)
3. Checks `exp` claim
4. Extracts `sub`, `email`, `role` and sets them on `echo.Context` for handlers to consume
5. Returns `401` on any validation failure

```
Request ──► JWT Middleware ──► Handler
                │
                ▼ (on failure)
              401 Unauthorized
```

The middleware is registered on the router group for `/api/v1/*`. The `/auth/*` routes are public and bypass it.

---

## Bootstrap flow (first startup)

If the environment variables `COURTKNIGHTS_BOOTSTRAP_PAT`, `COURTKNIGHTS_BOOTSTRAP_EMAIL`, and `COURTKNIGHTS_BOOTSTRAP_NAME` are set and no users exist in the database, the server creates:

1. An `admin` User record (`provider = pat`, email and name from env vars)
2. A PAT record derived from `COURTKNIGHTS_BOOTSTRAP_PAT` (hashed + salted)
3. Sets `provider_id` on the User to the new PAT's `id`

This runs once at startup and is a no-op on subsequent starts.

---

## New dependencies required

> All additions must be approved before implementation and registered in `Dependency.md`.

| Package                        | Purpose                                                           | License      |
| ------------------------------ | ----------------------------------------------------------------- | ------------ |
| `golang.org/x/oauth2`          | OAuth2 client (redirect flow + device flow) for Google and GitHub | BSD-3-Clause |
| `github.com/golang-jwt/jwt/v5` | JWT signing and validation (HS256)                                | MIT          |

---

## Optional provider configuration

OAuth2 providers (Google, GitHub) are optional at startup. If a provider's env vars are not set, it is treated as disabled:

- The corresponding `/auth/<provider>` and `/auth/<provider>/callback` routes return `501 Not Implemented`
- The `POST /auth/device` endpoint returns `501 Not Implemented` for that provider
- The service starts and all other providers + PAT continue to work normally
- At least one authentication method (any provider or PAT) must be available for the service to start; if none is configured, startup fails with a clear error message

---

## Acceptance test infrastructure

OAuth2 redirect and Device flows require browser interaction and cannot be tested with real provider credentials in CI/CD. The acceptance test suite (separate repository) must use a **mock OAuth2 server** to simulate provider responses. Recommendation: `mockoidc` or a lightweight custom OIDC server compatible with `golang.org/x/oauth2`.

PAT-based authentication is fully automatable and serves as the primary smoke test.

---

## Open questions

None — all decisions are captured in ADR-005, ADR-006, and ADR-007.
