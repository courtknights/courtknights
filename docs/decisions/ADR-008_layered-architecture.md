# ADR-008 — Layered Architecture: Repository, Manager, Handler

- **Date:** 2026-03-20
- **Status:** Accepted
- **Deciders:** aagea

---

## Context

CourtKnights needs a consistent internal structure that scales across multiple
features (authentication, leagues, players, statistics, …) without the logic of
one concern leaking into another. Early implementation of the authentication
feature went through two iterations:

1. A four-layer model (Repository → Manager → Service → Handler) where the
   Service layer turned out to be a thin delegation wrapper with no logic of its
   own, adding indirection without value.
2. The current decision consolidates orchestration inside the Manager layer via
   **manager composition**, removing the Service layer entirely.

---

## Decision

Every feature in `api/internal/` is structured in three layers:

```
┌──────────────────────────────────────────────────────────┐
│  Handler          (api/internal/api/<feature>/)           │
│  HTTP boundary — decodes requests, encodes responses      │
│  Depends on: Manager interface only (never concrete)      │
├──────────────────────────────────────────────────────────┤
│  Manager          (api/internal/application/<feature>/)   │
│  Business logic and orchestration                         │
│  Exposes: a Go interface consumed by the Handler          │
│  May call: Repositories and/or other Managers             │
├──────────────────────────────────────────────────────────┤
│  Repository       (api/internal/infrastructure/postgres/) │
│  Persistence — SQL queries, no business logic             │
│  Calls: Database driver only                              │
└──────────────────────────────────────────────────────────┘
```

### Manager composition

Every feature has exactly one **feature manager** (named after the feature,
e.g. `AuthManager`). This is the only manager the Handler calls. It owns the
feature's use cases and may be composed of one or more **sub-managers**, each
responsible for a limited, well-defined concern.

A sub-manager may call repositories, infrastructure adapters, or other
sub-managers. The only constraint is that **the Handler never calls a
sub-manager directly** — all entry points go through the feature manager.

```
Handler
  └── AuthManager            (feature manager — one per feature)
        ├── UserManager       (sub-manager — user/PAT business logic)
        ├── JWTManager        (sub-manager — token signing and validation)
        └── OAuthManager      (sub-manager — provider registry and flows)
```

Whether to introduce a sub-manager is a judgement call based on cohesion and
size. A feature manager that handles everything in a single struct is
acceptable for simple features. Sub-managers are introduced when a concern
grows complex enough to deserve its own test surface and its own constructor.

### Manager interfaces

Every manager **must** define a Go interface in the same package as its
implementation. The interface lists only the methods the Handler (or a
composing manager) actually needs. The concrete struct satisfies the interface
implicitly.

Naming convention:

| Artefact | Name | Example |
|----------|------|---------|
| Interface | `<Feature>Manager` | `AuthManager` |
| Concrete struct | `<feature>Manager` (unexported) | `authManager` |
| Constructor | `New<Feature>Manager(…) <Feature>Manager` | `NewAuthManager(…) AuthManager` |

The constructor returns the **interface**, not the struct. This ensures that
nothing outside the package can depend on the concrete type.

The same convention applies to sub-managers: the feature manager holds a field
of the sub-manager's interface type, never the concrete struct.

```go
// api/internal/application/auth/manager.go

type AuthManager interface {
    OAuthRedirectURL(provider user.Provider, state string) (string, error)
    OAuthCallback(ctx context.Context, provider user.Provider, code string) (string, error)
    DeviceInit(ctx context.Context, provider user.Provider) (*oauth2infra.DeviceAuthResponse, error)
    DevicePoll(ctx context.Context, provider user.Provider, deviceCode string) (string, error)
    ExchangePAT(ctx context.Context, rawPAT string) (string, error)
    RefreshJWT(ctx context.Context, tokenStr string) (string, error)
}

type authManager struct { user UserManager; jwt JWTManager; oauth OAuthManager }

func NewAuthManager(u UserManager, j JWTManager, o OAuthManager) AuthManager {
    return &authManager{user: u, jwt: j, oauth: o}
}
```

```go
// api/internal/api/auth/handler.go

type AuthHandler struct { manager auth.AuthManager }  // depends on interface only
```

### Layer rules

#### Repository
- Implements a domain interface (e.g. `user.UserRepository`) defined in
  `api/internal/domain/`.
- Returns domain entities or sentinel errors from `domain/ckerrors/`.
- Contains **no business logic** — no if-statements that encode rules, only
  data mapping and SQL.
- One repository per aggregate root.

#### Manager
- Every manager defines a **Go interface** in the same package.
- The constructor returns the interface — never the concrete struct.
- The **feature manager** (`<Feature>Manager`) owns all use cases and is the
  only entry point for the Handler.
- **Sub-managers** (`<Concern>Manager`) encapsulate a specific, limited
  responsibility and are injected into the feature manager as interfaces.
- Any manager may call repositories, infrastructure adapters, or other
  managers — always via injected interfaces, never concrete types.
- Returns domain entities or wrapped sentinel errors.
- Fully unit-testable: replace any dependency with a mock of its interface.

#### Handler
- Lives in `api/internal/api/<feature>/`.
- Holds a field of the **feature manager interface** type — never the concrete
  struct.
- Decodes HTTP request → calls the feature manager → encodes HTTP response.
- Owns HTTP status codes, request validation, and response serialisation.
- Contains **no business logic**.
- Mock the interface in handler tests — no application code instantiated.

### Dependency direction

```
Handler → AuthManager (interface) ←── authManager (concrete)
                                          ├── UserManager (interface) ←── userManager
                                          ├── JWTManager  (interface) ←── jwtManager
                                          └── OAuthManager(interface) ←── oauthManager
                                                    │
                                              Repository / Adapter
```

Arrows point **inward only**. Every layer depends on abstractions (interfaces),
never on concrete types from the adjacent layer.

### Infrastructure adapters

Infrastructure adapters (JWT, OAuth2 providers, email, …) live in
`api/internal/infrastructure/` and implement interfaces defined alongside the
manager that uses them. This keeps managers unit-testable without any external
process running.

### Package layout (authentication feature)

```
api/internal/
  domain/
    user/           user.go, repository.go   ← UserRepository interface
    pat/            pat.go, repository.go    ← PATRepository interface
    ckerrors/       auth.go

  application/
    auth/
      manager.go          ← AuthManager interface + authManager struct
      user_manager.go     ← UserManager interface + userManager struct
      jwt_manager.go      ← JWTManager  interface + jwtManager struct
      oauth_manager.go    ← OAuthManager interface + oauthManager struct

  infrastructure/
    postgres/
      user_repository.go  ← implements user.UserRepository
      pat_repository.go   ← implements pat.PATRepository
    jwt/
      jwt.go              ← implements auth.jwtAdapter
    oauth2/
      oauth2.go           ← Provider interface (used by oauthManager)
      google.go, github.go

  api/
    auth/
      handler.go          ← AuthHandler { manager auth.AuthManager }
      routes.go
    pats/
      handler.go          ← PATHandler  { manager auth.AuthManager }
      routes.go
    users/
      handler.go          ← UserHandler { manager auth.AuthManager }
      routes.go
    router.go
    common/
      middleware.go
```

### Violation checklist (use at PR review)

| Violation | Example | Fix |
|-----------|---------|-----|
| Handler depends on concrete manager | `handler.go` imports `*authManager` | Use `auth.AuthManager` interface |
| Handler calls sub-manager directly | Handler holds `auth.UserManager` field | Route through `auth.AuthManager` |
| Handler calls repository directly | Handler calls `UserRepository.FindByID` | Route through Manager |
| Constructor returns concrete struct | `NewAuthManager() *authManager` | Return `AuthManager` interface |
| Manager field is concrete type | `struct { user *userManager }` | Use `UserManager` interface |
| Business logic in Handler | Handler checks PAT expiry | Move to Manager |
| Manager contains no logic — only delegates | Every method is `return other.Method()` | Merge into caller or add real logic |

---

## Consequences

### Positive
- Three layers to understand instead of four — simpler mental model.
- "Everything is a Manager with an interface" is a uniform, learnable pattern.
- Handler tests only need a mock of the `AuthManager` interface — no
  application code instantiated, no DB, no network.
- Sub-manager tests mock their dependencies via interfaces — fully isolated.
- Every concrete type is hidden behind an interface — easy to swap
  implementations (e.g. replace PostgreSQL repo with in-memory for tests).
- Orchestration has a clear home: the feature manager's use-case methods.
- Adding a new use case = adding a method to the interface + implementation.

### Negative
- Every manager requires both an interface and a struct — slightly more
  boilerplate than a struct-only approach.
- The decision of when to introduce a sub-manager is a judgement call — not
  enforced by the type system.
- If a feature has many use cases the feature manager interface can grow large.

### Mitigations
- Naming convention (`<Feature>Manager` interface / `<feature>Manager` struct)
  is applied consistently — tools like `go doc` surface the interface first.
- Feature manager methods should stay short (2–5 lines). If a method grows,
  the logic belongs in a sub-manager.
- Large interfaces can be split into focused sub-interfaces if a handler only
  uses a subset of operations.

---

## Alternatives considered

### Four layers (Repository → Manager → Service → Handler)
Tried first. The Service layer had no logic of its own — it was a delegation
wrapper. Removed in favour of manager composition.

### Flat (Handler + everything)
Rejected — business logic would accumulate in handlers with no clear home.

### Hexagonal / Ports-and-Adapters (strict)
More complete but adds significant interface boilerplate for a small team. The
current model captures the most valuable invariants at lower cost.
