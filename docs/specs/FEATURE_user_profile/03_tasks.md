# FEATURE_user_profile — Tasks

- **Last updated:** 2026-04-03
- **Status:** approved
- **Issue:** [#93](https://github.com/courtknights/courtknights/issues/93)
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
  │     └── T-03 (postgres repo) ── depends on T-01 + T-02
  │
  └── T-04 (manager) ──────────── depends on T-01
        │
        ├── T-05 (auth integration) ── depends on T-03 + T-04
        ├── T-06 (api: profile endpoints) ── depends on T-04
        └── T-07 (api: list users) ────────── depends on T-04
```

---

## T-01 — Domain layer: Profile entity, Preferences, repository interface, ckerrors · [#96](https://github.com/courtknights/courtknights/issues/96)

**Files to create:**

| File                                         | Description                                                                                |
| -------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `api/internal/domain/profile/profile.go`     | `Profile` struct; enums `Gender`, `Category`, `CourtSide`, `Handedness` with all constants |
| `api/internal/domain/profile/preferences.go` | `Preferences` value object (JSONB-mapped struct)                                           |
| `api/internal/domain/profile/repository.go`  | `ProfileRepository` interface; `ProfilePatch`, `ListParams`, `ProfileListItem` types       |
| `api/internal/domain/profile/validate.go`    | `ValidateLocation(country, region *string) error`                                          |
| `api/internal/domain/ckerrors/profile.go`    | Sentinel error: `ErrProfileNotFound`                                                       |

`ValidateLocation` uses `github.com/biter777/countries`. This dependency must be added to `go.mod` and `Dependency.md`.

Rules for `ValidateLocation`:
- Both `nil` → no error.
- `country` non-nil → must be a valid ISO 3166-1 alpha-2 code; otherwise error.
- `region` non-nil → `country` must also be non-nil; `region` must be a valid ISO 3166-2 code for that country; otherwise error.

**Tests:** unit tests for `ValidateLocation`.

**Acceptance:** `make test` passes; `domain/profile/` has no imports outside the Go standard library and `github.com/biter777/countries`.

---

## T-02 — Database schema: user_profiles · [#97](https://github.com/courtknights/courtknights/issues/97)

> No code dependencies. Can be worked on in parallel with T-01.

**Files to create:**

| File                                              | Description                                                                |
| ------------------------------------------------- | -------------------------------------------------------------------------- |
| `db/schema/user_profiles.sql`                     | `user_profiles` table definition (reference only, not executed by migrate) |
| `db/migrations/003_create_user_profiles.up.sql`   | Create `user_profiles` table; backfill existing users from `users`         |
| `db/migrations/003_create_user_profiles.down.sql` | Drop `user_profiles`                                                       |

Schema and migration contents are specified in `02_architecture.md` (Database schema and Migration sections).

**Acceptance:** migrations run and reverse cleanly against a real PostgreSQL instance that already contains the `users` table.

---

## T-03 — Infrastructure: PostgreSQL ProfileRepository · [#98](https://github.com/courtknights/courtknights/issues/98)

> Depends on T-01 + T-02.

**Files to create:**

| File                                                         | Description                                                                      |
| ------------------------------------------------------------ | -------------------------------------------------------------------------------- |
| `api/internal/infrastructure/postgres/profile_repository.go` | Implements `ProfileRepository`: `EnsureExists`, `FindByUserID`, `Update`, `List` |

Implementation notes (from `02_architecture.md`):
- `EnsureExists`: `INSERT INTO user_profiles ... ON CONFLICT (user_id) DO NOTHING`
- `FindByUserID`: SELECT by `user_id`; return `ckerrors.ErrProfileNotFound` when no row
- `Update`: build UPDATE dynamically from non-nil patch fields; return updated row
- `List`: JOIN with `users` table; ORDER BY `display_name`; LIMIT/OFFSET from `ListParams`

**Tests:** integration tests using Testcontainers (build tag `integration`). Add cases to the existing `TestMain` in `api/internal/infrastructure/postgres/integration_test.go`.

**Acceptance:** `make test-int` passes.

---

## T-04 — Application: ProfileManager · [#99](https://github.com/courtknights/courtknights/issues/99)

> Depends on T-01.

**Files to create:**

| File                                          | Description                                                                                 |
| --------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `api/internal/application/profile/manager.go` | `ProfileManager` interface + `profileManager` struct; all methods from `02_architecture.md` |

Validation in `Update`:
1. If `Country` or `Region` are non-nil, call `profile.ValidateLocation`.
2. Return validation error before calling the repository if validation fails.

**Tests:** unit tests with a mocked `ProfileRepository` (`testify/mock`).

**Acceptance:** `make test` passes; no DB or network required.

---

## T-05 — Auth integration: profile initialisation on login · [#100](https://github.com/courtknights/courtknights/issues/100)

> Depends on T-03 + T-04.

**Files to modify:**

| File                                       | Description                                                                                                                                                                  |
| ------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `api/internal/application/auth/manager.go` | Add `profiles ProfileManager` field to `authManager`; call `profiles.EnsureExists` in `OAuthCallback`, `DevicePoll`, and `BootstrapAdmin` after the user is resolved/created |
| `api/cmd/server/server.go`                 | Instantiate `profileManager`; inject into `NewAuthManager`                                                                                                                   |

`NewAuthManager` gains a fourth parameter: `profiles appprofile.ProfileManager`.

The `EnsureExists` call must happen after the user upsert but before signing the JWT. An error from `EnsureExists` must be propagated to the caller.

**Tests:** unit tests extending `manager_test.go`. A mock `ProfileManager` must be added to `mocks_test.go`.

**Acceptance:** `make test` passes; existing auth tests continue to pass.

---

## T-06 — API: own-profile and get-by-ID endpoints · [#101](https://github.com/courtknights/courtknights/issues/101)

> Depends on T-04.

**Files to modify:**

| File                                | Description                                                                                        |
| ----------------------------------- | -------------------------------------------------------------------------------------------------- |
| `api/internal/api/users/handler.go` | Add `profiles ProfileManager` field; implement `getMyProfile`, `updateMyProfile`, `getUserProfile` |
| `api/internal/api/users/routes.go`  | Register `GET /users/me/profile`, `PUT /users/me/profile`, `GET /users/:id/profile`                |
| `api/cmd/server/server.go`          | Pass `profileManager` to `users.NewHandler`                                                        |

`users.NewHandler` gains a second parameter: `profiles appprofile.ProfileManager`.

The authenticated user's ID is read from the JWT context via `common.ContextKeySub` (UUID string — must be parsed).

Error mapping:
- `ckerrors.ErrProfileNotFound` → `404`
- validation errors from `ProfileManager.Update` → `400`

**Tests:** unit tests in `api/internal/api/users/handler_profile_test.go` with a mocked `ProfileManager`.

**Acceptance:** `make test` passes.

---

## T-07 — API: list users endpoint · [#102](https://github.com/courtknights/courtknights/issues/102)

> Depends on T-04. `profileManager` is already injected from T-06.

**Files to modify:**

| File                                | Description           |
| ----------------------------------- | --------------------- |
| `api/internal/api/users/handler.go` | Implement `listUsers` |
| `api/internal/api/users/routes.go`  | Register `GET /users` |

Query parameter handling:
- `page` (default `1`, minimum `1`)
- `page_size` (default `20`, maximum `100`; return `400` if exceeded)
- `fields` (comma-separated; empty = all allowed fields)

Allowed field names: `id`, `display_name`, `city`, `region`, `country`, `gender`, `category`.

Response shape: `{ "data": [...], "total": N, "page": N, "page_size": N }`.

**Tests:** unit tests in `api/internal/api/users/handler_list_test.go` with a mocked `ProfileManager`.

**Acceptance:** `make test` passes.
