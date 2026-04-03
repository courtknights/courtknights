# FEATURE_user_profile — Test Cases

- **Last updated:** 2026-04-02
- **Status:** approved
- **Issue:** [#93](https://github.com/courtknights/courtknights/issues/93)
- **Spec:** [01_business.md](01_business.md) · [02_architecture.md](02_architecture.md) · [03_tasks.md](03_tasks.md)

> Tests are grouped by task and level. All unit tests run with `make test`. All integration tests run with `make test-int`. Coverage minimums follow [docs/testing/strategy.md](../../testing/strategy.md).

---

## T-01 — Domain: ValidateLocation (unit)

**File:** `api/internal/domain/profile/validate_test.go`
**Build tag:** none (unit)
**Minimum coverage:** ≥ 90%

| ID | Description | Input | Expected outcome |
|----|-------------|-------|-----------------|
| VL-01 | Both nil — no validation | `country=nil`, `region=nil` | `nil` |
| VL-02 | Valid country, no region | `country="ES"`, `region=nil` | `nil` |
| VL-03 | Valid country + valid matching region | `country="ES"`, `region="ES-MD"` | `nil` |
| VL-04 | Invalid country code | `country="XX"`, `region=nil` | error |
| VL-05 | Valid country + region from a different country | `country="ES"`, `region="FR-75"` | error |
| VL-06 | Valid country + invalid region format | `country="ES"`, `region="INVALID"` | error |
| VL-07 | Region provided without country | `country=nil`, `region="ES-MD"` | error |
| VL-08 | Empty string country (not nil) | `country=""`, `region=nil` | error |
| VL-09 | Empty string region with valid country | `country="US"`, `region=""` | error |
| VL-10 | Case-sensitive country code (lowercase) | `country="es"`, `region=nil` | error (codes must be uppercase) |

---

## T-03 — Infrastructure: ProfileRepository (integration)

**File:** `api/internal/infrastructure/postgres/profile_repository_integration_test.go`
**Build tag:** `//go:build integration`
**Suite hook:** add cases to the existing `TestMain` in `integration_test.go`
**Minimum coverage:** ≥ 70%

### EnsureExists

| ID | Description | Precondition | Expected outcome |
|----|-------------|-------------|-----------------|
| PR-01 | Creates a profile for a new user | User exists in `users`, no profile | Row inserted in `user_profiles`; `display_name` matches input |
| PR-02 | No-op when profile already exists | Profile already exists for user | No error; existing row unchanged |
| PR-03 | Non-existent user ID | User not in `users` table | FK violation error propagated |

### FindByUserID

| ID | Description | Precondition | Expected outcome |
|----|-------------|-------------|-----------------|
| PR-04 | Returns profile for existing user | Profile exists | `*Profile` with correct field values |
| PR-05 | Returns ErrProfileNotFound when missing | No profile for the user | `ckerrors.ErrProfileNotFound` |
| PR-06 | All nullable fields are null | Profile exists with only `display_name` set | `*Profile` with all pointers nil except `DisplayName` |

### Update

| ID | Description | Patch | Expected outcome |
|----|-------------|-------|-----------------|
| PR-07 | Partial update — only display_name | `{DisplayName: ptr("New")}` | Updated row returned; other fields unchanged |
| PR-08 | Update all fields at once | All fields non-nil | Returned profile matches every patch field |
| PR-09 | Update preferences | `{Preferences: &Preferences{CourtSide: ptr(CourtSideDrive)}}` | JSONB updated; other preference fields unchanged |
| PR-10 | Update non-existent profile | No profile for user | Error or zero rows affected (implementation-defined) |

### List

| ID | Description | Params | Expected outcome |
|----|-------------|--------|-----------------|
| PR-11 | Returns all profiles ordered by display_name | Page=1, PageSize=100 | Slice ordered by `display_name` ASC; total count matches DB rows |
| PR-12 | Pagination — first page | Page=1, PageSize=2, 5 profiles in DB | 2 items returned; total=5 |
| PR-13 | Pagination — last page | Page=3, PageSize=2, 5 profiles in DB | 1 item returned; total=5 |
| PR-14 | Page beyond total | Page=10, PageSize=2, 5 profiles in DB | Empty slice; total=5 |

---

## T-04 — Application: ProfileManager (unit)

**File:** `api/internal/application/profile/manager_test.go`
**Build tag:** none (unit)
**Dependencies mocked:** `ProfileRepository` via `testify/mock`
**Minimum coverage:** ≥ 80%

### EnsureExists

| ID | Description | Mock behaviour | Expected outcome |
|----|-------------|---------------|-----------------|
| PM-01 | Delegates to repository successfully | Repo returns nil | nil error |
| PM-02 | Propagates repository error | Repo returns error | Same error returned |

### GetByUserID

| ID | Description | Mock behaviour | Expected outcome |
|----|-------------|---------------|-----------------|
| PM-03 | Returns profile from repository | Repo returns `*Profile` | Same `*Profile` returned |
| PM-04 | Propagates ErrProfileNotFound | Repo returns `ErrProfileNotFound` | `ErrProfileNotFound` returned |

### Update — validation

| ID | Description | Patch | Expected outcome |
|----|-------------|-------|-----------------|
| PM-05 | No location fields — skips validation, calls repo | `{DisplayName: ptr("X")}` | Repo called; updated profile returned |
| PM-06 | Valid country + valid region — calls repo | `{Country: ptr("ES"), Region: ptr("ES-MD")}` | Repo called; updated profile returned |
| PM-07 | Invalid country — returns 400-style error before calling repo | `{Country: ptr("XX")}` | Error returned; repo **not** called |
| PM-08 | Region without country — returns error before calling repo | `{Region: ptr("ES-MD")}` | Error returned; repo **not** called |
| PM-09 | Valid country, mismatched region — returns error | `{Country: ptr("ES"), Region: ptr("FR-75")}` | Error returned; repo **not** called |

### List

| ID | Description | Mock behaviour | Expected outcome |
|----|-------------|---------------|-----------------|
| PM-10 | Delegates params to repository | Repo returns slice + count | Same slice and count returned |
| PM-11 | Propagates repository error | Repo returns error | Same error returned |

---

## T-05 — Auth integration: profile init on login (unit)

**File:** `api/internal/application/auth/manager_test.go` (extend existing)
**Mocks file:** `api/internal/application/auth/mocks_test.go` (add `MockProfileManager`)
**Build tag:** none (unit)

| ID | Description | Scenario | Expected outcome |
|----|-------------|----------|-----------------|
| AI-01 | OAuthCallback calls EnsureExists after user upsert | OAuth flow completes; mock ProfileManager returns nil | JWT signed and returned; EnsureExists called once with correct userID and displayName |
| AI-02 | OAuthCallback propagates EnsureExists error | Mock ProfileManager returns error | Error propagated; JWT not issued |
| AI-03 | DevicePoll calls EnsureExists after user resolution | Device flow completes; mock returns nil | JWT signed; EnsureExists called once |
| AI-04 | DevicePoll propagates EnsureExists error | Mock returns error | Error propagated |
| AI-05 | BootstrapAdmin calls EnsureExists after admin user created | Bootstrap flow; mock returns nil | No error; EnsureExists called once |
| AI-06 | BootstrapAdmin propagates EnsureExists error | Mock returns error | Error propagated |
| AI-07 | Existing auth unit tests still pass | No behaviour change other than new EnsureExists call | All existing assertions pass with mock injected |

---

## T-06 — API: own-profile and get-by-ID handlers (unit)

**File:** `api/internal/api/users/handler_profile_test.go`
**Build tag:** none (unit)
**Test infrastructure:** `net/http/httptest` + Echo test helpers
**Dependencies mocked:** `ProfileManager` via `testify/mock`
**Minimum coverage:** ≥ 80%

### GET /users/me/profile

| ID | Description | Mock behaviour | Expected HTTP |
|----|-------------|---------------|--------------|
| HP-01 | Returns own profile | Manager returns `*Profile` | 200 with full profile JSON |
| HP-02 | Profile not found | Manager returns `ErrProfileNotFound` | 404 |
| HP-03 | Missing JWT / invalid sub claim | JWT context absent | 401 (middleware) or 400 |

### PUT /users/me/profile

| ID | Description | Request body | Mock behaviour | Expected HTTP |
|----|-------------|-------------|---------------|--------------|
| HP-04 | Partial update — display_name only | `{"display_name":"X"}` | Manager returns updated `*Profile` | 200 with updated profile JSON |
| HP-05 | Full update — all fields | All fields present | Manager returns `*Profile` | 200 |
| HP-06 | Invalid country code | `{"country":"XX"}` | Manager returns validation error | 400 |
| HP-07 | Region without country | `{"region":"ES-MD"}` | Manager returns validation error | 400 |
| HP-08 | Unknown enum value for gender | `{"gender":"unknown"}` | Deserialisation or manager error | 400 |
| HP-09 | Empty body (all fields omitted) | `{}` | Manager called with all-nil patch; returns existing profile | 200 |

### GET /users/:id/profile

| ID | Description | Mock behaviour | Expected HTTP |
|----|-------------|---------------|--------------|
| HP-10 | Returns profile for valid user ID | Manager returns `*Profile` | 200 with profile JSON |
| HP-11 | Profile not found | Manager returns `ErrProfileNotFound` | 404 |
| HP-12 | Malformed UUID in path | — | 400 |

---

## T-07 — API: list users handler (unit)

**File:** `api/internal/api/users/handler_list_test.go`
**Build tag:** none (unit)
**Test infrastructure:** `net/http/httptest` + Echo test helpers
**Dependencies mocked:** `ProfileManager` via `testify/mock`
**Minimum coverage:** ≥ 80%

| ID | Description | Query string | Mock behaviour | Expected HTTP / body |
|----|-------------|-------------|---------------|---------------------|
| LU-01 | Default params | (none) | Manager returns 2 items, total=2 | 200; `page=1`, `page_size=20`, `total=2` |
| LU-02 | Explicit page and page_size | `?page=2&page_size=5` | Manager called with Page=2, PageSize=5 | 200; response reflects params |
| LU-03 | Field selection | `?fields=id,display_name,country` | Manager called with Fields=["id","display_name","country"] | 200; items contain only requested fields |
| LU-04 | page_size exceeds max (100) | `?page_size=101` | — | 400 |
| LU-05 | page less than 1 | `?page=0` | — | 400 |
| LU-06 | Unknown field name in fields | `?fields=id,unknown_field` | — | 400 |
| LU-07 | Empty result set | (none) | Manager returns empty slice, total=0 | 200; `"data":[]`, `"total":0` |
| LU-08 | Manager returns error | (none) | Manager returns error | 500 |
