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

Every feature in `internal/` is structured in three layers:

```
┌──────────────────────────────────────────────────────┐
│  Handler          (internal/api/<feature>/)           │
│  HTTP boundary — decodes requests, encodes responses  │
│  Calls: the feature's root Manager only               │
├──────────────────────────────────────────────────────┤
│  Manager          (internal/application/<feature>/)   │
│  Business logic and orchestration                     │
│  May call: Repositories and/or other Managers         │
├──────────────────────────────────────────────────────┤
│  Repository       (internal/infrastructure/postgres/) │
│  Persistence — SQL queries, no business logic         │
│  Calls: Database driver only                          │
└──────────────────────────────────────────────────────┘
```

### Manager composition

The Manager layer is the only layer with internal structure. A feature is
implemented by one or more managers that form a tree:

- **Leaf managers** focus on a single cohesive concern and call only
  repositories **or** a single category of infrastructure adapter — never both
  and never another manager.
- **Root manager** owns the feature's use cases. It is injected with leaf
  managers and composes their calls to implement each operation. It never calls
  repositories or infrastructure adapters directly.

The Handler always calls the **root manager** of the feature.

```
Handler
  └── AuthManager (root)
        ├── UserManager  (leaf — calls UserRepository + PATRepository)
        ├── JWTManager   (leaf — calls JWT infrastructure adapter)
        └── OAuthManager (leaf — calls OAuth2 provider adapters)
```

### Layer rules

#### Repository
- Implements a domain interface (e.g. `user.UserRepository`).
- Returns domain entities or sentinel errors from `domain/ckerrors/`.
- Contains **no business logic** — no if-statements that encode rules, only
  data mapping and SQL.
- One repository per aggregate root.

#### Manager (leaf)
- Focuses on a single cohesive concern.
- Calls repositories **or** one category of infrastructure adapter, never both.
- Never calls another manager.
- Returns domain entities or wrapped sentinel errors.
- Fully unit-testable via injected mock repositories/adapters.

#### Manager (root)
- Named after the feature (e.g. `AuthManager`).
- Owns the feature's use cases — one method per use case.
- Calls only leaf managers, never repositories or adapters directly.
- Contains orchestration logic (sequencing, error mapping across managers).
- Injected with its leaf managers via constructor.

#### Handler
- Lives in `internal/api/<feature>/`.
- Decodes HTTP request → calls root Manager → encodes HTTP response.
- Owns HTTP status codes, request validation, and response serialisation.
- Contains **no business logic**.
- Calls only the root Manager of its feature.

### Dependency direction

```
Handler → Manager (root) → Manager (leaf) → Repository
                                           → Infrastructure adapter
```

Arrows point **inward only**. Inner layers are unaware of outer layers.

### Infrastructure adapters

Infrastructure adapters (JWT, OAuth2 providers, email, …) live in
`internal/infrastructure/` and implement interfaces defined next to the leaf
manager that uses them. This keeps the manager unit-testable without any
external process.

### Package layout (authentication feature)

```
internal/
  domain/
    user/           user.go, repository.go
    pat/            pat.go, repository.go
    ckerrors/       auth.go

  application/
    auth/
      manager.go          ← AuthManager  (root: composes leaf managers)
      user_manager.go     ← UserManager  (leaf: repos only)
      jwt_manager.go      ← JWTManager   (leaf: JWT adapter only)
      oauth_manager.go    ← OAuthManager (leaf: OAuth2 providers only)

  infrastructure/
    postgres/
      user_repository.go  ← implements user.UserRepository
      pat_repository.go   ← implements pat.PATRepository
    jwt/
      jwt.go              ← JWT infrastructure adapter
    oauth2/
      oauth2.go, google.go, github.go

  api/
    auth/
      handler.go          ← AuthHandler (HTTP → AuthManager)
      routes.go
    pats/
      handler.go          ← PATHandler  (HTTP → AuthManager)
      routes.go
    users/
      handler.go          ← UserHandler (HTTP → AuthManager)
      routes.go
    router.go
    common/
      middleware.go
```

### Violation checklist (use at PR review)

| Violation | Example | Fix |
|-----------|---------|-----|
| Handler calls leaf manager directly | Handler calls `UserManager.ResolveByPAT` | Route through root `AuthManager` |
| Handler calls repository directly | Handler calls `UserRepository.FindByID` | Route through Manager |
| Root manager calls repository directly | `AuthManager` calls `UserRepository.Upsert` | Delegate to `UserManager` |
| Leaf manager calls another manager | `UserManager` calls `JWTManager.Sign` | Move to root manager |
| Manager contains no logic — only delegates | Manager method is a single `return other.Method()` | Merge into caller or add real logic |
| Business logic in Handler | Handler checks PAT expiry | Move to Manager |

---

## Consequences

### Positive
- Three layers to understand instead of four — simpler mental model.
- "Everything is a Manager" is a uniform abstraction.
- Leaf managers are unit-testable without any infrastructure.
- Root manager tests mock leaf managers — fast, no DB, no network.
- Handler tests mock the root manager — isolated HTTP concerns.
- Orchestration has a clear home: the root manager's use-case methods.
- Adding a new use case = adding a method to the root manager.

### Negative
- The distinction between root and leaf managers must be understood by every
  contributor — it is not enforced by the type system.
- If a feature has many use cases the root manager file can grow large.

### Mitigations
- A clear naming convention (`<Feature>Manager` for root, `<Concern>Manager`
  for leaf) signals the role without extra documentation.
- Root manager methods should stay short (2–5 lines). If a method grows, the
  logic belongs in a leaf manager.

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
