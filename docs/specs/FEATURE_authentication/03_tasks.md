# FEATURE_authentication — Tasks

- **Last updated:** 2026-03-18
- **Status:** draft
- **Issue:** [#6](https://github.com/courtknights/courtknights/issues/6)
- **Spec:** [01_business.md](01_business.md) · [02_architecture.md](02_architecture.md)

> Each task maps to one GitHub Issue (label: `agent-task`) and one PR.
> Tasks must be implemented in the order listed — each depends on all prior tasks unless stated otherwise.

---

## Dependency graph

```
T-01 (domain)
  │
  ├── T-02 (schema)          ← no code deps, can start in parallel with T-01
  │     │
  │     └── T-03 (postgres repos) ── depends on T-01 + T-02
  │
  ├── T-04 (jwt adapter)     ← depends on T-01
  │
  └── T-05 (oauth2 adapters) ← depends on T-01
        │
        └── T-06 (manager) ──── depends on T-01 + T-03
              │
              └── T-07 (service) ── depends on T-04 + T-05 + T-06
                    │
                    └── T-08 (router + middleware) ── depends on T-04 + T-07
                          │
                          ├── T-09 (oauth2 redirect handlers)
                          ├── T-10 (device flow handlers)
                          ├── T-11 (pat exchange + jwt refresh handlers)
                          ├── T-12 (pat management API)
                          └── T-13 (user management API)

T-14 (bootstrap) ── depends on T-06 + T-03
```

---

## T-01 — Domain layer: User, PAT, and ckerrors · [#7](https://github.com/courtknights/courtknights/issues/7)

**Files to create:**

| File                                 | Description                                                                                               |
| ------------------------------------ | --------------------------------------------------------------------------------------------------------- |
| `api/api/internal/domain/user/user.go`       | `User` struct; `Role` enum (`admin`, `user`); `Provider` enum (`google`, `github`, `pat`)                 |
| `api/api/internal/domain/user/repository.go` | `UserRepository` interface: `FindByID`, `FindByProvider`, `Upsert`                                        |
| `api/api/internal/domain/pat/pat.go`         | `PAT` struct: `ID`, `KeyHash`, `Salt`, `ExpiresAt`, `CreatedAt`                                           |
| `api/api/internal/domain/pat/repository.go`  | `PATRepository` interface: `FindAll`, `FindByID`, `Save`, `Delete`                                        |
| `api/api/internal/domain/ckerrors/auth.go`   | Sentinel errors: `ErrUserNotFound`, `ErrPATNotFound`, `ErrPATExpired`, `ErrInvalidPAT`, `ErrUnauthorized` |

**Tests:** unit tests for any domain logic (e.g. `User.IsAdmin()`, PAT expiry check).

**Acceptance:** `make test` passes; no external imports in `domain/`.

---

## T-02 — Database schema: users and personal_access_tokens · [#8](https://github.com/courtknights/courtknights/issues/8)

**Files to create:**

| File                                                       | Description                                                       |
| ---------------------------------------------------------- | ----------------------------------------------------------------- |
| `db/schema/users.sql`                                      | `users` table as defined in `02_architecture.md`                  |
| `db/schema/personal_access_tokens.sql`                     | `personal_access_tokens` table as defined in `02_architecture.md` |
| `db/migrations/001_create_users.up.sql`                    | Forward migration: create `users` table                           |
| `db/migrations/001_create_users.down.sql`                  | Reverse migration: drop `users` table                             |
| `db/migrations/002_create_personal_access_tokens.up.sql`   | Forward migration: create `personal_access_tokens` table          |
| `db/migrations/002_create_personal_access_tokens.down.sql` | Reverse migration: drop `personal_access_tokens` table            |

**Acceptance:** migrations run and reverse cleanly against a real PostgreSQL instance.

---

## T-03 — Infrastructure: PostgreSQL repository implementations · [#9](https://github.com/courtknights/courtknights/issues/9)

> Depends on T-01 + T-02.

**Files to create:**

| File                                                  | Description                                                         |
| ----------------------------------------------------- | ------------------------------------------------------------------- |
| `api/api/internal/infrastructure/postgres/user_repository.go` | Implements `UserRepository`: `FindByID`, `FindByProvider`, `Upsert` |
| `api/api/internal/infrastructure/postgres/pat_repository.go`  | Implements `PATRepository`: `FindAll`, `FindByID`, `Save`, `Delete` |

**Tests:** integration tests using Testcontainers (build tag `integration`).
- `FindByProvider` returns `ErrUserNotFound` when no match
- `Upsert` creates on first call; updates on subsequent call with same `(provider, provider_id)`
- `FindByID` returns `ErrPATNotFound` for unknown ID
- `Save` + `Delete` round-trip

**Acceptance:** `make test-int` passes.

---

## T-04 — Infrastructure: JWT adapter · [#10](https://github.com/courtknights/courtknights/issues/10)

> Depends on T-01.

**Files to create:**

| File                                 | Description                                                                                                                                                                   |
| ------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `api/api/internal/infrastructure/jwt/jwt.go` | `Sign(user *domain.User) (string, error)` — issues HS256 JWT (claims: `sub`, `email`, `role`, `exp`); `Validate(token string) (*Claims, error)` — verifies signature + expiry |

**Configuration:** secret key and expiry (`1h`) read from Viper config / env vars (`COURTKNIGHTS_JWT_SECRET`, `COURTKNIGHTS_JWT_EXPIRY`).

**Tests:** unit tests.
- `Sign` → `Validate` round-trip returns correct claims
- `Validate` returns error for expired token
- `Validate` returns error for tampered signature

**Acceptance:** `make test` passes; no DB or network required.

---

## T-05 — Infrastructure: OAuth2 adapters (Google + GitHub) · [#11](https://github.com/courtknights/courtknights/issues/11)

> Depends on T-01.

**Files to create:**

| File                                       | Description                                                                                                                                                                                                        |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `api/api/internal/infrastructure/oauth2/oauth2.go` | `Provider` interface: `AuthCodeURL(state string) string`; `Exchange(ctx, code string) (*UserInfo, error)`; `DeviceAuth(ctx) (*DeviceAuthResponse, error)`; `DevicePoll(ctx, deviceCode string) (*UserInfo, error)` |
| `api/api/internal/infrastructure/oauth2/google.go` | Implements `Provider` for Google                                                                                                                                                                                   |
| `api/api/internal/infrastructure/oauth2/github.go` | Implements `Provider` for GitHub                                                                                                                                                                                   |

**Configuration:** client ID, client secret, and redirect URL per provider, read from Viper config / env vars.

**Tests:** unit tests with mocked HTTP transport.
- `AuthCodeURL` returns a URL containing the correct provider domain
- `Exchange` maps provider response to `UserInfo`
- `DeviceAuth` returns `DeviceAuthResponse` with expected fields

**Acceptance:** `make test` passes.

---

## T-06 — Application: AuthManager · [#12](https://github.com/courtknights/courtknights/issues/12)

> Depends on T-01 + T-03.

**Files to create:**

| File                                   | Description                                                                                                                  |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `api/api/internal/application/auth/manager.go` | `AuthManager` struct with injected `UserRepository` + `PATRepository`; implements all methods listed in `02_architecture.md` |

**Manager methods:**

| Method                                              | Behaviour                                                                                                                                |
| --------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `ResolveByOAuth(provider, providerID, email, name)` | Find user by `(provider, providerID)`; create if not found; return User                                                                  |
| `ResolveByPAT(rawPAT)`                              | Fetch all PATs; hash `rawPAT` against each salt; match `key_hash`; check expiry; find linked User by `provider_id = pat.ID`; return User |
| `CreatePAT(userID, expiresAt)`                      | Generate random key; hash with random salt; save PAT; return raw key (once only)                                                         |
| `RevokePAT(id)`                                     | Delete PAT by ID                                                                                                                         |
| `BootstrapAdmin(email, name, rawPAT)`               | No-op if any user exists; otherwise create admin User (`provider=pat`) + PAT; link via `provider_id`                                     |

**Tests:** unit tests with mocked repositories (`testify/mock`).
- `ResolveByOAuth` creates user on first call; returns existing user on second call
- `ResolveByPAT` returns `ErrInvalidPAT` for wrong key
- `ResolveByPAT` returns `ErrPATExpired` for expired PAT
- `CreatePAT` returns a non-empty raw key; stored hash differs from raw key
- `BootstrapAdmin` is a no-op when users exist

**Acceptance:** `make test` passes; no DB or network required.

---

## T-07 — Application: AuthService · [#13](https://github.com/courtknights/courtknights/issues/13)

> Depends on T-04 + T-05 + T-06.

**Files to create:**

| File                                   | Description                                                                        |
| -------------------------------------- | ---------------------------------------------------------------------------------- |
| `api/api/internal/application/auth/service.go` | `AuthService` struct; orchestrates `AuthManager` + OAuth2 `Provider` + JWT adapter |

**Service methods:**

| Method                                  | Description                                                      |
| --------------------------------------- | ---------------------------------------------------------------- |
| `OAuthRedirectURL(provider, state)`     | Delegates to `Provider.AuthCodeURL`                              |
| `OAuthCallback(ctx, provider, code)`    | Exchange code → UserInfo → ResolveByOAuth → Sign JWT             |
| `DeviceInit(ctx, provider)`             | Delegates to `Provider.DeviceAuth`; returns device auth response |
| `DevicePoll(ctx, provider, deviceCode)` | Poll provider → UserInfo → ResolveByOAuth → Sign JWT             |
| `ExchangePAT(ctx, rawPAT)`              | ResolveByPAT → Sign JWT                                          |
| `RefreshJWT(claims)`                    | Validate claims → Sign new JWT with same user                    |

**Tests:** unit tests with mocked manager + adapters.
- `OAuthCallback` with valid code returns a non-empty JWT
- `ExchangePAT` with invalid PAT returns `ErrInvalidPAT`
- `DevicePoll` while pending returns `authorization_pending` error

**Acceptance:** `make test` passes.

---

## T-08 — API: Router and JWT middleware · [#14](https://github.com/courtknights/courtknights/issues/14)

> Depends on T-04 + T-07.

**Files to create:**

| File                                | Description                                                                                                                                         |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `api/api/internal/api/router.go`            | Creates the Echo instance; registers `common/middleware.go` on `/api/v1/*`; exposes `Mount(group, routes)` to register module route groups          |
| `api/api/internal/api/common/middleware.go` | JWT middleware: reads `Authorization` header → validates with JWT adapter → sets `sub`, `email`, `role` on `echo.Context`; returns `401` on failure |

**Tests:** unit tests for middleware.
- Valid JWT → handler receives correct context values
- Missing header → `401`
- Expired JWT → `401`
- Tampered JWT → `401`

**Acceptance:** `make test` passes; `/auth/*` routes bypass the middleware.

---

## T-09 — API: OAuth2 redirect flow handlers · [#15](https://github.com/courtknights/courtknights/issues/15)

> Depends on T-08.

**Files to create / extend:**

| File                           | Description                                                                               |
| ------------------------------ | ----------------------------------------------------------------------------------------- |
| `api/api/internal/api/auth/handler.go` | `GET /auth/google` → redirect; `GET /auth/google/callback` → JWT; same for `/auth/github` |
| `api/api/internal/api/auth/routes.go`  | Registers the four OAuth2 redirect routes on the auth group                               |

**Tests:** unit tests with mocked `AuthService`.
- `GET /auth/google` returns `302` with a Google URL in `Location`
- `GET /auth/google/callback?code=valid` returns `302` with JWT in redirect URL
- `GET /auth/google/callback?code=invalid` returns `401`

**Acceptance:** `make test` passes.

---

## T-10 — API: Device Authorization flow handlers · [#16](https://github.com/courtknights/courtknights/issues/16)

> Depends on T-08.

**Files to extend:**

| File                           | Description                                                                                  |
| ------------------------------ | -------------------------------------------------------------------------------------------- |
| `api/api/internal/api/auth/handler.go` | `POST /auth/device` → device auth response; `POST /auth/device/token` → JWT or pending error |
| `api/api/internal/api/auth/routes.go`  | Registers the two device flow routes                                                         |

**Tests:** unit tests with mocked `AuthService`.
- `POST /auth/device` with valid provider returns `200` with `user_code` and `verification_uri`
- `POST /auth/device/token` while pending returns `202` with `authorization_pending`
- `POST /auth/device/token` once authorised returns `200` with JWT

**Acceptance:** `make test` passes.

---

## T-11 — API: PAT exchange and JWT refresh handlers · [#17](https://github.com/courtknights/courtknights/issues/17)

> Depends on T-08.

**Files to extend:**

| File                           | Description                                                  |
| ------------------------------ | ------------------------------------------------------------ |
| `api/api/internal/api/auth/handler.go` | `POST /auth/token/pat` → JWT; `POST /auth/refresh` → new JWT |
| `api/api/internal/api/auth/routes.go`  | Registers the two routes                                     |

**Tests:** unit tests with mocked `AuthService`.
- `POST /auth/token/pat` with valid PAT returns `200` with JWT
- `POST /auth/token/pat` with invalid PAT returns `401`
- `POST /auth/refresh` with valid JWT returns `200` with new JWT
- `POST /auth/refresh` with expired JWT returns `401`

**Acceptance:** `make test` passes.

---

## T-12 — API: PAT management endpoints · [#18](https://github.com/courtknights/courtknights/issues/18)

> Depends on T-08.

**Files to create:**

| File                           | Description                                                                        |
| ------------------------------ | ---------------------------------------------------------------------------------- |
| `api/api/internal/api/pats/handler.go` | `POST /api/v1/pats` (admin); `GET /api/v1/pats`; `DELETE /api/v1/pats/:id` (admin) |
| `api/api/internal/api/pats/routes.go`  | Registers the three routes under `/api/v1` group                                   |

**Tests:** unit tests with mocked `AuthService`.
- `POST /api/v1/pats` as admin returns `201` with raw PAT (shown once)
- `POST /api/v1/pats` as non-admin returns `403`
- `DELETE /api/v1/pats/:id` as admin returns `204`
- `DELETE /api/v1/pats/:id` as non-admin returns `403`

**Acceptance:** `make test` passes.

---

## T-13 — API: User management endpoints · [#19](https://github.com/courtknights/courtknights/issues/19)

> Depends on T-08.

**Files to create:**

| File                            | Description                                                  |
| ------------------------------- | ------------------------------------------------------------ |
| `api/api/internal/api/users/handler.go` | `GET /api/v1/users/me`; `PUT /api/v1/users/:id/role` (admin) |
| `api/api/internal/api/users/routes.go`  | Registers the two routes under `/api/v1` group               |

**Tests:** unit tests with mocked `AuthService`.
- `GET /api/v1/users/me` returns `200` with authenticated user profile
- `PUT /api/v1/users/:id/role` as admin returns `200`
- `PUT /api/v1/users/:id/role` as non-admin returns `403`

**Acceptance:** `make test` passes.

---

## T-14 — Bootstrap admin flow · [#20](https://github.com/courtknights/courtknights/issues/20)

> Depends on T-03 + T-06.

**Files to modify:**

| File                                   | Description                                                                                                                                                           |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cmd/server/main.go` (or startup hook) | On startup: if `COURTKNIGHTS_BOOTSTRAP_PAT`, `COURTKNIGHTS_BOOTSTRAP_EMAIL`, and `COURTKNIGHTS_BOOTSTRAP_NAME` are set, call `AuthManager.BootstrapAdmin`; log result |

**Tests:** integration test with Testcontainers.
- Bootstrap runs on empty DB → admin user + PAT created
- Bootstrap is a no-op on non-empty DB → no duplicate users

**Acceptance:** `make test-int` passes.
