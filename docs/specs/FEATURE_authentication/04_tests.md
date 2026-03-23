# FEATURE_authentication — Test Cases

- **Last updated:** 2026-03-18
- **Status:** draft
- **Issue:** [#6](https://github.com/courtknights/courtknights/issues/6)
- **Spec:** [03_tasks.md](03_tasks.md)

> Test files follow the naming convention defined in `docs/testing/strategy.md`:
> - Unit: `*_test.go`
> - Integration: `*_integration_test.go` with build tag `//go:build integration`
>
> Integration tests spin up a dedicated PostgreSQL container via Testcontainers in `TestMain`.
> Each suite owns its container — no shared state between suites.

---

## T-01 — Domain layer

**File:** `api/api/internal/domain/user/user_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestUser_IsAdmin_ReturnsTrueForAdminRole` | unit | `User{Role: RoleAdmin}.IsAdmin()` returns `true` |
| 2 | `TestUser_IsAdmin_ReturnsFalseForUserRole` | unit | `User{Role: RoleUser}.IsAdmin()` returns `false` |
| 3 | `TestRole_String_ReturnsExpectedValues` | unit | `RoleAdmin.String() == "admin"`, `RoleUser.String() == "user"` |
| 4 | `TestProvider_String_ReturnsExpectedValues` | unit | `ProviderGoogle.String() == "google"`, `ProviderGitHub.String() == "github"`, `ProviderPAT.String() == "pat"` |

**File:** `api/api/internal/domain/pat/pat_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 5 | `TestPAT_IsExpired_ReturnsTrueWhenPastExpiry` | unit | PAT with `ExpiresAt` in the past returns `true` |
| 6 | `TestPAT_IsExpired_ReturnsFalseWhenFutureExpiry` | unit | PAT with `ExpiresAt` in the future returns `false` |
| 7 | `TestPAT_IsExpired_ReturnsFalseWhenNeverExpires` | unit | PAT with `ExpiresAt == nil` returns `false` |

---

## T-02 — Database schema

> No Go tests. Validated by running migrations up and down against a real PostgreSQL instance.

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `migration_001_up` | manual | `001_create_users.up.sql` executes without error; `users` table exists with all columns |
| 2 | `migration_001_down` | manual | `001_create_users.down.sql` drops `users` table cleanly |
| 3 | `migration_002_up` | manual | `002_create_personal_access_tokens.up.sql` creates table; `id` is UUID PK |
| 4 | `migration_002_down` | manual | `002_create_personal_access_tokens.down.sql` drops table cleanly |
| 5 | `migration_full_roundtrip` | manual | Running all migrations up then all down leaves the DB in the original empty state |

---

## T-03 — PostgreSQL repository implementations

### Infrastructure setup (shared across all integration tests in this package)

```go
//go:build integration

func TestMain(m *testing.M) {
    // Start PostgreSQL container via Testcontainers
    // Run migrations
    // Run tests
    // Teardown container
}
```

**File:** `api/api/internal/infrastructure/postgres/user_repository_integration_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestUserRepository_Upsert_CreatesNewUser` | integration | Upsert a new user; `FindByID` returns it with correct fields |
| 2 | `TestUserRepository_Upsert_UpdatesExistingUser` | integration | Upsert same `(provider, provider_id)` twice; second call updates `name`; only one row exists |
| 3 | `TestUserRepository_FindByID_ReturnsUser` | integration | Insert user; `FindByID` returns correct user |
| 4 | `TestUserRepository_FindByID_ReturnsErrUserNotFound` | integration | `FindByID` with unknown UUID returns `ckerrors.ErrUserNotFound` |
| 5 | `TestUserRepository_FindByProvider_ReturnsUser` | integration | Insert user; `FindByProvider(provider, providerID)` returns correct user |
| 6 | `TestUserRepository_FindByProvider_ReturnsErrUserNotFound` | integration | `FindByProvider` with unknown `(provider, providerID)` returns `ckerrors.ErrUserNotFound` |
| 7 | `TestUserRepository_UniqueConstraint_ProviderProviderID` | integration | Inserting two users with same `(provider, provider_id)` returns DB error |

**File:** `api/api/internal/infrastructure/postgres/pat_repository_integration_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 8 | `TestPATRepository_Save_CreatesPAT` | integration | Save a PAT; `FindByID` returns it with correct `key_hash` and `salt` |
| 9 | `TestPATRepository_FindByID_ReturnsErrPATNotFound` | integration | `FindByID` with unknown UUID returns `ckerrors.ErrPATNotFound` |
| 10 | `TestPATRepository_FindAll_ReturnsAllPATs` | integration | Insert 3 PATs; `FindAll` returns all 3 |
| 11 | `TestPATRepository_Delete_RemovesPAT` | integration | Save then Delete a PAT; `FindByID` returns `ckerrors.ErrPATNotFound` |
| 12 | `TestPATRepository_Delete_UnknownIDReturnsError` | integration | `Delete` with unknown UUID returns error |

---

## T-04 — JWT adapter

**File:** `api/api/internal/infrastructure/jwt/jwt_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestJWT_Sign_ReturnsNonEmptyToken` | unit | `Sign` with valid user returns a non-empty token string |
| 2 | `TestJWT_Sign_Validate_RoundTrip` | unit | `Validate(Sign(user))` returns claims with correct `sub`, `email`, `role` |
| 3 | `TestJWT_Validate_ReturnsErrorForExpiredToken` | unit | Token with `exp` in the past returns error |
| 4 | `TestJWT_Validate_ReturnsErrorForTamperedSignature` | unit | Token with last character changed returns error |
| 5 | `TestJWT_Validate_ReturnsErrorForEmptyToken` | unit | Empty string returns error |
| 6 | `TestJWT_Validate_ReturnsErrorForWrongSecret` | unit | Token signed with secret A fails validation with secret B |
| 7 | `TestJWT_Claims_ContainCorrectExpiry` | unit | `exp` claim equals `iat + 1h` |

---

## T-05 — OAuth2 adapters

**File:** `api/api/internal/infrastructure/oauth2/google_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestGoogle_AuthCodeURL_ContainsGoogleDomain` | unit | Returned URL contains `accounts.google.com` |
| 2 | `TestGoogle_AuthCodeURL_ContainsState` | unit | Returned URL contains the provided `state` parameter |
| 3 | `TestGoogle_Exchange_ReturnsUserInfo` | unit | Mocked HTTP response returns `UserInfo` with `email` and `name` |
| 4 | `TestGoogle_Exchange_ReturnsErrorForInvalidCode` | unit | Mocked HTTP 400 response returns error |
| 5 | `TestGoogle_DeviceAuth_ReturnsDeviceAuthResponse` | unit | Mocked HTTP response returns `DeviceAuthResponse` with `user_code` and `verification_uri` |
| 6 | `TestGoogle_DevicePoll_ReturnsPendingError` | unit | Mocked HTTP response with `error=authorization_pending` returns the expected error |
| 7 | `TestGoogle_DevicePoll_ReturnsUserInfoWhenAuthorised` | unit | Mocked HTTP success response returns `UserInfo` |

**File:** `api/api/internal/infrastructure/oauth2/github_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 8 | `TestGitHub_AuthCodeURL_ContainsGitHubDomain` | unit | Returned URL contains `github.com` |
| 9 | `TestGitHub_Exchange_ReturnsUserInfo` | unit | Mocked HTTP response returns `UserInfo` with `email` and `name` |
| 10 | `TestGitHub_DeviceAuth_ReturnsDeviceAuthResponse` | unit | Mocked HTTP response returns `DeviceAuthResponse` |
| 11 | `TestGitHub_DevicePoll_ReturnsPendingError` | unit | Mocked HTTP `authorization_pending` response returns expected error |

---

## T-06 — AuthManager

**File:** `api/api/internal/application/auth/manager_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestAuthManager_ResolveByOAuth_CreatesUserOnFirstLogin` | unit | `FindByProvider` returns `ErrUserNotFound`; `Upsert` is called; user is returned |
| 2 | `TestAuthManager_ResolveByOAuth_ReturnsExistingUser` | unit | `FindByProvider` returns a user; `Upsert` is NOT called; same user is returned |
| 3 | `TestAuthManager_ResolveByPAT_ReturnsUserForValidPAT` | unit | Hash matches; PAT not expired; `FindByProvider` returns linked user |
| 4 | `TestAuthManager_ResolveByPAT_ReturnsErrInvalidPATForWrongKey` | unit | No hash match across all PATs; returns `ckerrors.ErrInvalidPAT` |
| 5 | `TestAuthManager_ResolveByPAT_ReturnsErrPATExpiredForExpiredPAT` | unit | Hash matches but PAT is expired; returns `ckerrors.ErrPATExpired` |
| 6 | `TestAuthManager_CreatePAT_ReturnsRawKey` | unit | `Save` is called; returned raw key is non-empty; stored `key_hash` differs from raw key |
| 7 | `TestAuthManager_CreatePAT_HashDiffersFromRawKey` | unit | `key_hash` stored in repo does not equal the returned raw key |
| 8 | `TestAuthManager_RevokePAT_CallsDelete` | unit | `Delete` is called with the correct PAT ID |
| 9 | `TestAuthManager_BootstrapAdmin_CreatesAdminWhenNoUsersExist` | unit | `FindAll` returns empty; admin user + PAT are created via `Upsert` + `Save` |
| 10 | `TestAuthManager_BootstrapAdmin_IsNoOpWhenUsersExist` | unit | `FindAll` returns one user; neither `Upsert` nor `Save` is called |

---

## T-07 — AuthService

**File:** `api/api/internal/application/auth/service_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestAuthService_OAuthRedirectURL_DelegatesToProvider` | unit | Returns the URL from `Provider.AuthCodeURL` unchanged |
| 2 | `TestAuthService_OAuthCallback_ReturnsJWTForValidCode` | unit | `Exchange` succeeds; `ResolveByOAuth` returns user; `Sign` returns token; token is non-empty |
| 3 | `TestAuthService_OAuthCallback_ReturnsErrorForInvalidCode` | unit | `Exchange` returns error; service returns same error; `Sign` is NOT called |
| 4 | `TestAuthService_DeviceInit_ReturnsDelegatedResponse` | unit | Returns `DeviceAuthResponse` from `Provider.DeviceAuth` unchanged |
| 5 | `TestAuthService_DevicePoll_ReturnsJWTWhenAuthorised` | unit | `DevicePoll` returns `UserInfo`; `ResolveByOAuth` + `Sign` called; token is non-empty |
| 6 | `TestAuthService_DevicePoll_ReturnsPendingError` | unit | `DevicePoll` returns `authorization_pending` error; propagated to caller |
| 7 | `TestAuthService_ExchangePAT_ReturnsJWTForValidPAT` | unit | `ResolveByPAT` returns user; `Sign` returns token |
| 8 | `TestAuthService_ExchangePAT_ReturnsErrInvalidPAT` | unit | `ResolveByPAT` returns `ErrInvalidPAT`; service propagates error; `Sign` NOT called |
| 9 | `TestAuthService_ExchangePAT_ReturnsErrPATExpired` | unit | `ResolveByPAT` returns `ErrPATExpired`; propagated to caller |
| 10 | `TestAuthService_RefreshJWT_ReturnsNewTokenForValidClaims` | unit | Valid claims → `Sign` returns new token with same `sub`, `email`, `role` |

---

## T-08 — Router and JWT middleware

**File:** `api/api/internal/api/common/middleware_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestJWTMiddleware_PassesWithValidToken` | unit | Valid JWT in `Authorization` header → handler is called; `sub`, `email`, `role` set on context |
| 2 | `TestJWTMiddleware_Returns401WithMissingHeader` | unit | No `Authorization` header → `401 Unauthorized` |
| 3 | `TestJWTMiddleware_Returns401WithMalformedHeader` | unit | `Authorization: NotBearer token` → `401 Unauthorized` |
| 4 | `TestJWTMiddleware_Returns401WithExpiredToken` | unit | Expired JWT → `401 Unauthorized` |
| 5 | `TestJWTMiddleware_Returns401WithTamperedToken` | unit | Tampered JWT → `401 Unauthorized` |
| 6 | `TestJWTMiddleware_PublicRoutesBypassMiddleware` | unit | Request to `/auth/*` route reaches handler without JWT |

---

## T-09 — OAuth2 redirect flow handlers

**File:** `api/api/internal/api/auth/handler_oauth_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestAuthHandler_GetGoogle_Returns302WithGoogleURL` | unit | `GET /auth/google` → `302`; `Location` header contains `accounts.google.com` |
| 2 | `TestAuthHandler_GetGitHub_Returns302WithGitHubURL` | unit | `GET /auth/github` → `302`; `Location` header contains `github.com` |
| 3 | `TestAuthHandler_GoogleCallback_Returns302WithJWTOnSuccess` | unit | `GET /auth/google/callback?code=valid&state=x` → `302`; redirect URL contains `token=` |
| 4 | `TestAuthHandler_GoogleCallback_Returns401OnInvalidCode` | unit | `OAuthCallback` returns error → `401 Unauthorized` |
| 5 | `TestAuthHandler_GitHubCallback_Returns302WithJWTOnSuccess` | unit | `GET /auth/github/callback?code=valid&state=x` → `302`; redirect URL contains `token=` |
| 6 | `TestAuthHandler_GitHubCallback_Returns401OnInvalidCode` | unit | `OAuthCallback` returns error → `401 Unauthorized` |

---

## T-10 — Device Authorization flow handlers

**File:** `api/api/internal/api/auth/handler_device_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestAuthHandler_PostDevice_Returns200WithDeviceAuthResponse` | unit | `POST /auth/device {"provider":"google"}` → `200`; body contains `user_code` and `verification_uri` |
| 2 | `TestAuthHandler_PostDevice_Returns400ForUnknownProvider` | unit | `POST /auth/device {"provider":"unknown"}` → `400 Bad Request` |
| 3 | `TestAuthHandler_PostDevice_Returns400ForMissingProvider` | unit | `POST /auth/device {}` → `400 Bad Request` |
| 4 | `TestAuthHandler_PostDeviceToken_Returns200WithJWTWhenAuthorised` | unit | `POST /auth/device/token {"device_code":"x"}` → `200`; body contains `token` |
| 5 | `TestAuthHandler_PostDeviceToken_Returns202WhenPending` | unit | `DevicePoll` returns `authorization_pending` → `202`; body contains `error: authorization_pending` |
| 6 | `TestAuthHandler_PostDeviceToken_Returns400ForMissingDeviceCode` | unit | `POST /auth/device/token {}` → `400 Bad Request` |

---

## T-11 — PAT exchange and JWT refresh handlers

**File:** `api/api/internal/api/auth/handler_pat_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestAuthHandler_PostTokenPAT_Returns200WithJWT` | unit | `POST /auth/token/pat {"pat":"valid"}` → `200`; body contains `token` and `expires_in` |
| 2 | `TestAuthHandler_PostTokenPAT_Returns401ForInvalidPAT` | unit | `ExchangePAT` returns `ErrInvalidPAT` → `401 Unauthorized` |
| 3 | `TestAuthHandler_PostTokenPAT_Returns401ForExpiredPAT` | unit | `ExchangePAT` returns `ErrPATExpired` → `401 Unauthorized` |
| 4 | `TestAuthHandler_PostTokenPAT_Returns400ForMissingBody` | unit | `POST /auth/token/pat {}` → `400 Bad Request` |
| 5 | `TestAuthHandler_PostRefresh_Returns200WithNewJWT` | unit | `POST /auth/refresh` with valid JWT → `200`; body contains new `token` |
| 6 | `TestAuthHandler_PostRefresh_Returns401WithExpiredJWT` | unit | `POST /auth/refresh` with expired JWT → `401 Unauthorized` |
| 7 | `TestAuthHandler_PostRefresh_Returns401WithMissingJWT` | unit | `POST /auth/refresh` with no `Authorization` header → `401 Unauthorized` |

---

## T-12 — PAT management endpoints

**File:** `api/api/internal/api/pats/handler_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestPATHandler_PostPATs_Returns201WithRawPATAsAdmin` | unit | Admin JWT; `POST /api/v1/pats` → `201`; body contains `id`, `pat`, `expires_at` |
| 2 | `TestPATHandler_PostPATs_Returns403AsNonAdmin` | unit | Non-admin JWT; `POST /api/v1/pats` → `403 Forbidden` |
| 3 | `TestPATHandler_PostPATs_Returns401WithNoJWT` | unit | No JWT → `401 Unauthorized` |
| 4 | `TestPATHandler_GetPATs_Returns200WithList` | unit | `GET /api/v1/pats` with valid JWT → `200`; body is an array |
| 5 | `TestPATHandler_DeletePAT_Returns204AsAdmin` | unit | Admin JWT; `DELETE /api/v1/pats/:id` → `204 No Content` |
| 6 | `TestPATHandler_DeletePAT_Returns403AsNonAdmin` | unit | Non-admin JWT; `DELETE /api/v1/pats/:id` → `403 Forbidden` |
| 7 | `TestPATHandler_DeletePAT_Returns404ForUnknownID` | unit | `RevokePAT` returns `ErrPATNotFound` → `404 Not Found` |

---

## T-13 — User management endpoints

**File:** `api/api/internal/api/users/handler_test.go`

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestUserHandler_GetMe_Returns200WithUserProfile` | unit | Valid JWT → `200`; body contains `id`, `email`, `name`, `role` |
| 2 | `TestUserHandler_GetMe_Returns401WithNoJWT` | unit | No JWT → `401 Unauthorized` |
| 3 | `TestUserHandler_PutRole_Returns200AsAdmin` | unit | Admin JWT; `PUT /api/v1/users/:id/role {"role":"admin"}` → `200` |
| 4 | `TestUserHandler_PutRole_Returns403AsNonAdmin` | unit | Non-admin JWT; `PUT /api/v1/users/:id/role` → `403 Forbidden` |
| 5 | `TestUserHandler_PutRole_Returns404ForUnknownUser` | unit | `FindByID` returns `ErrUserNotFound` → `404 Not Found` |
| 6 | `TestUserHandler_PutRole_Returns400ForInvalidRole` | unit | `{"role":"superadmin"}` → `400 Bad Request` |

---

## T-14 — Bootstrap admin flow

**File:** `api/api/internal/infrastructure/postgres/bootstrap_integration_test.go`

```go
//go:build integration
```

Testcontainers setup: PostgreSQL container started in `TestMain`; migrations applied before each test; DB truncated between tests.

| # | Test name | Type | Description |
|---|-----------|------|-------------|
| 1 | `TestBootstrap_CreatesAdminUserAndPATOnEmptyDB` | integration | Env vars set; `BootstrapAdmin` called on empty DB; one user with `role=admin` and `provider=pat` exists; one PAT linked via `provider_id` |
| 2 | `TestBootstrap_IsNoOpWhenUsersAlreadyExist` | integration | Insert one user; call `BootstrapAdmin`; user count remains 1; PAT count remains 0 |
| 3 | `TestBootstrap_AdminCanAuthenticateWithBootstrapPAT` | integration | Full flow: bootstrap → `ResolveByPAT(rawPAT)` returns the admin user |
