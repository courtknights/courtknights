# FEATURE_user_profile — Architecture

- **Last updated:** 2026-04-02
- **Status:** approved
- **Issue:** [#93](https://github.com/courtknights/courtknights/issues/93)

---

## Component overview

```
┌─────────────────────────────────────────────────────┐
│                   Backend (Echo)                    │
│                                                     │
│  /api/v1/users      ──►  api/users/handler          │
│  /api/v1/users/me/profile                           │
│  /api/v1/users/:id/profile                          │
│                          │                          │
│                          ▼                          │
│               application/profile                   │
│               (ProfileManager)                      │
│                          │                          │
│               domain/profile/                       │
│               (Profile entity + repository iface)   │
│                          │                          │
│               infrastructure/postgres               │
│               (ProfileRepository impl)              │
└─────────────────────────────────────────────────────┘
```

---

## Internal package structure

```
api/internal/
  domain/
    profile/
      profile.go          # Profile entity, enums (Gender, Category, CourtSide, Handedness)
      preferences.go      # Preferences value object (JSONB-mapped struct)
      repository.go       # ProfileRepository interface
      validate.go         # Location validation (ISO 3166-1 / ISO 3166-2)
  application/
    profile/
      manager.go          # ProfileManager — business logic
  infrastructure/
    postgres/
      profile_repository.go  # ProfileRepository implementation
  api/
    users/
      handler.go          # Extended with profile endpoints (existing file)
      routes.go           # Extended with new routes (existing file)
```

The existing `api/users/` module is extended rather than replaced. `AuthManager` is also extended to call `ProfileRepository.EnsureExists` on every login (idempotent profile initialisation).

---

## Domain layer

### Profile entity

```go
// domain/profile/profile.go

type Gender   string
type Category string
type CourtSide  string
type Handedness string

const (
    GenderMale   Gender = "male"
    GenderFemale Gender = "female"

    CategoryFirst  Category = "first"
    CategorySecond Category = "second"
    CategoryThird  Category = "third"
    CategoryFourth Category = "fourth"
    CategoryFifth  Category = "fifth"

    CourtSideDrive    CourtSide = "drive"
    CourtSideBackhand CourtSide = "backhand"
    CourtSideBoth     CourtSide = "both"

    HandednessRight Handedness = "right"
    HandednessLeft  Handedness = "left"
)

type Profile struct {
    UserID      uuid.UUID
    DisplayName string
    City        *string
    Region      *string   // ISO 3166-2 code, e.g. "ES-MD"
    Country     *string   // ISO 3166-1 alpha-2 code, e.g. "ES"
    Gender      *Gender
    DateOfBirth *time.Time
    Category    *Category
    Preferences Preferences
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Preferences value object

```go
// domain/profile/preferences.go

// Preferences holds sport-specific settings stored as JSONB.
// All fields are optional. New fields can be added here without a schema migration.
type Preferences struct {
    CourtSide  *CourtSide  `json:"court_side,omitempty"`
    Handedness *Handedness `json:"handedness,omitempty"`
}
```

### Location validation

```go
// domain/profile/validate.go

// ValidateLocation returns an error if country or region are not valid ISO codes,
// or if region is provided without country.
// Uses an embedded dataset — no external API call.
func ValidateLocation(country, region *string) error
```

Implemented using `github.com/biter777/countries` (see New dependencies).

Rules:
- `country` must be a valid ISO 3166-1 alpha-2 code if provided.
- `region` must be a valid ISO 3166-2 code for the given `country` if provided.
- If `region` is provided, `country` must also be provided.

### ProfileRepository interface

```go
// domain/profile/repository.go

type ProfileRepository interface {
    // EnsureExists creates the profile with the given display name if it does not exist.
    // No-op if a profile already exists for the user. Safe to call on every login.
    EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error

    // FindByUserID returns the profile for the given user.
    // Returns ckerrors.ErrProfileNotFound if no profile exists.
    FindByUserID(ctx context.Context, userID uuid.UUID) (*Profile, error)

    // Update applies a partial update to the profile.
    // Only non-nil fields in the patch are written.
    Update(ctx context.Context, userID uuid.UUID, patch ProfilePatch) (*Profile, error)

    // List returns a paginated list of profiles joined with basic user data.
    List(ctx context.Context, params ListParams) ([]*ProfileListItem, int64, error)
}

// ProfilePatch carries the fields to update. Nil means "leave unchanged".
type ProfilePatch struct {
    DisplayName *string
    City        *string
    Region      *string
    Country     *string
    Gender      *Gender
    DateOfBirth *time.Time
    Category    *Category
    Preferences *Preferences
}

// ListParams controls pagination and field selection for the list endpoint.
type ListParams struct {
    Page     int
    PageSize int
    Fields   []string // subset of allowed fields; empty = all fields
}

// ProfileListItem is a projection used by the list endpoint.
// All fields are pointers so that omitted fields serialise as null / absent.
type ProfileListItem struct {
    ID          *uuid.UUID
    DisplayName *string
    City        *string
    Region      *string
    Country     *string
    Gender      *Gender
    Category    *Category
}
```

---

## Application layer

### ProfileManager

```go
// application/profile/manager.go

type ProfileManager struct {
    profiles profile.ProfileRepository
}

func NewProfileManager(profiles profile.ProfileRepository) *ProfileManager

// EnsureExists delegates to the repository (called by AuthManager on login).
func (m *ProfileManager) EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error

// GetByUserID returns a profile, returning ErrProfileNotFound if absent.
func (m *ProfileManager) GetByUserID(ctx context.Context, userID uuid.UUID) (*profile.Profile, error)

// Update validates the patch (location codes, enum values) and persists it.
func (m *ProfileManager) Update(ctx context.Context, userID uuid.UUID, patch profile.ProfilePatch) (*profile.Profile, error)

// List returns a paginated list with the requested fields.
func (m *ProfileManager) List(ctx context.Context, params profile.ListParams) ([]*profile.ProfileListItem, int64, error)
```

Validation in `Update`:
1. If `Country` or `Region` are set, call `profile.ValidateLocation`.
2. Enum values (`Gender`, `Category`, `CourtSide`, `Handedness`) are typed — invalid values are rejected at deserialisation.

### AuthManager change

`AuthManager.ResolveByOAuth` and `AuthManager.BootstrapAdmin` are extended to call `ProfileManager.EnsureExists` after upserting the user. The call is idempotent (no-op on subsequent logins). `ProfileManager` is injected into `AuthManager` at wiring time.

---

## Infrastructure layer

### Database schema

```sql
-- db/schema/user_profiles.sql
CREATE TABLE user_profiles (
    user_id       UUID         PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name  VARCHAR(255) NOT NULL,
    city          VARCHAR(255),
    region        VARCHAR(10),
    country       VARCHAR(2),
    gender        VARCHAR(10)  CHECK (gender IN ('male', 'female')),
    date_of_birth DATE,
    category      VARCHAR(10)  CHECK (category IN ('first','second','third','fourth','fifth')),
    preferences   JSONB        NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

A migration adds this table. No changes to `users` table.

### ProfileRepository (postgres)

- `EnsureExists`: `INSERT INTO user_profiles ... ON CONFLICT (user_id) DO NOTHING`
- `FindByUserID`: simple SELECT by `user_id`
- `Update`: dynamic UPDATE building only the non-nil patch fields; returns the updated row
- `List`: `SELECT u.id, p.display_name, p.city, ... FROM users u JOIN user_profiles p ON p.user_id = u.id ORDER BY p.display_name LIMIT $1 OFFSET $2`

---

## HTTP API contract

All endpoints are protected (JWT required). Registered under `/api/v1`.

### Get own profile

```
GET /api/v1/users/me/profile
Authorization: Bearer <jwt>
```

**Response 200:**
```json
{
  "user_id": "<uuid>",
  "display_name": "Álvaro Agea",
  "city": "Madrid",
  "region": "ES-MD",
  "country": "ES",
  "gender": "male",
  "date_of_birth": "1990-05-14",
  "category": "third",
  "preferences": {
    "court_side": "drive",
    "handedness": "right"
  },
  "updated_at": "2026-04-02T12:00:00Z"
}
```

### Update own profile

```
PUT /api/v1/users/me/profile
Authorization: Bearer <jwt>
Content-Type: application/json
```

**Request** (all fields optional — partial update):
```json
{
  "display_name": "Álvaro",
  "city": "Madrid",
  "region": "ES-MD",
  "country": "ES",
  "gender": "male",
  "date_of_birth": "1990-05-14",
  "category": "third",
  "preferences": {
    "court_side": "drive"
  }
}
```

**Response 200:** updated profile (same shape as GET).

**Errors:**
- `400` — invalid ISO code, unknown enum value, or region without country.

### Get profile by ID

```
GET /api/v1/users/:id/profile
Authorization: Bearer <jwt>
```

**Response 200:** same shape as GET own profile.
**Response 404:** user or profile not found.

### List users

```
GET /api/v1/users?page=1&page_size=20&fields=id,display_name,country,category
Authorization: Bearer <jwt>
```

**Query params:**

| Param | Default | Notes |
|-------|---------|-------|
| `page` | `1` | 1-based page number |
| `page_size` | `20` | Max `100` |
| `fields` | all | Comma-separated subset of: `id`, `display_name`, `city`, `region`, `country`, `gender`, `category` |

**Response 200:**
```json
{
  "data": [
    { "id": "<uuid>", "display_name": "Álvaro", "country": "ES", "category": "third" }
  ],
  "total": 42,
  "page": 1,
  "page_size": 20
}
```

---

## Routing changes

`api/users/routes.go` adds:

```
GET  /api/v1/users                     → handler.ListUsers
GET  /api/v1/users/me/profile          → handler.GetMyProfile
PUT  /api/v1/users/me/profile          → handler.UpdateMyProfile
GET  /api/v1/users/:id/profile         → handler.GetUserProfile
```

Existing routes (`GET /api/v1/users/me`, `PUT /api/v1/users/:id/role`) are unchanged.

---

## New dependencies required

> All additions must be approved before implementation and registered in `Dependency.md`.

| Package | Purpose | License |
|---------|---------|---------|
| `github.com/biter777/countries` | ISO 3166-1 alpha-2 country and ISO 3166-2 subdivision validation — embedded dataset, no external API | MIT |

---

## Migration

A new migration file is added under `db/migrations/`:

```
db/migrations/
  NNNN_create_user_profiles.up.sql
  NNNN_create_user_profiles.down.sql
```

The `up` migration creates `user_profiles` and backfills existing users:

```sql
CREATE TABLE user_profiles (...);

-- Backfill profiles for users created before this feature.
INSERT INTO user_profiles (user_id, display_name)
SELECT id, name FROM users
ON CONFLICT (user_id) DO NOTHING;
```

---

## Open questions

None — all decisions are captured above or in the referenced ADRs.
