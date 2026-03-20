# ADR-008 — Layered Architecture: Repository, Manager, Service, Handler

- **Date:** 2026-03-20
- **Status:** Accepted
- **Deciders:** aagea

---

## Context

CourtKnights needs a consistent internal structure that scales across multiple
features (authentication, leagues, players, statistics, …) without the logic of
one concern leaking into another. Early implementation of the authentication
feature surfaced a concrete problem: the first version of `AuthService` called
infrastructure adapters directly, blurring the line between business logic and
orchestration. A shared, documented architecture contract is required so every
feature follows the same pattern and the violation is caught immediately at
review time.

---

## Decision

Every feature in `internal/` is structured in four layers, each with a strict
rule about which other layers it may call:

```
┌──────────────────────────────────────────────────────┐
│  Handler          (internal/api/<feature>/)           │
│  HTTP boundary — decodes requests, encodes responses  │
│  Calls: Service only                                  │
├──────────────────────────────────────────────────────┤
│  Service          (internal/application/<feature>/)   │
│  Orchestration — composes managers, no own logic      │
│  Calls: Managers only                                 │
├──────────────────────────────────────────────────────┤
│  Manager          (internal/application/<feature>/)   │
│  Business logic — owns rules, errors, invariants      │
│  Calls: Repositories and infrastructure adapters      │
├──────────────────────────────────────────────────────┤
│  Repository       (internal/infrastructure/postgres/) │
│  Persistence — SQL queries, no business logic         │
│  Calls: Database driver only                          │
└──────────────────────────────────────────────────────┘
```

### Layer rules

#### Repository
- Implements a domain interface (e.g. `user.UserRepository`).
- Returns domain entities or sentinel errors from `domain/ckerrors/`.
- Contains **no business logic** — no if-statements that encode rules, only
  data mapping and SQL.
- One repository per aggregate root (e.g. `UserRepository`, `PATRepository`).

#### Manager
- Named `<Feature>Manager` (e.g. `AuthManager`, `JWTManager`, `OAuthManager`).
- Owns all business rules for its bounded concern.
- May call one or more repositories and/or infrastructure adapters
  (JWT adapter, OAuth2 provider, email client, …).
- Never calls another Manager or a Service.
- Returns domain entities or wrapped sentinel errors.
- Each manager covers a single cohesive concern:
  - `AuthManager` → user/PAT resolution, PAT lifecycle, admin bootstrap.
  - `JWTManager` → token signing and validation.
  - `OAuthManager` → provider registry, code exchange, device flow.

#### Service
- Named `<Feature>Service` (e.g. `AuthService`).
- **Pure orchestration** — chains manager calls to implement a use case.
- Contains **no business logic** of its own; if a branch is needed it belongs
  in a Manager.
- Calls only Managers, never Repositories or infrastructure adapters directly.
- Methods map 1-to-1 with use cases (e.g. `OAuthCallback`, `ExchangePAT`).
- Typically 2–5 lines per method.

#### Handler
- Named `<Feature>Handler` (e.g. `AuthHandler`, `PATHandler`).
- Lives in `internal/api/<feature>/`.
- Decodes HTTP request → calls Service → encodes HTTP response.
- Owns HTTP status codes, request validation, and response serialisation.
- No business logic — if a rule appears here it belongs in a Manager.
- Calls only the Service for its feature.

### Dependency direction

```
Handler → Service → Manager → Repository
                  ↘ Infrastructure adapters
```

Arrows point **inward only**. Inner layers are unaware of outer layers.

### Infrastructure adapters

Infrastructure adapters (JWT adapter, OAuth2 providers, database drivers) live
in `internal/infrastructure/` and are injected into Managers via constructor
parameters. They implement interfaces defined in the application layer so that
Managers can be unit-tested with mocks without any external process running.

### Package layout example (authentication feature)

```
internal/
  domain/
    user/           user.go, repository.go
    pat/            pat.go, repository.go
    ckerrors/       auth.go

  application/
    auth/
      manager.go          ← AuthManager  (business logic, calls repos)
      jwt_manager.go      ← JWTManager   (business logic, calls JWT adapter)
      oauth_manager.go    ← OAuthManager (business logic, calls OAuth2 providers)
      service.go          ← AuthService  (orchestration, calls managers only)

  infrastructure/
    postgres/
      user_repository.go  ← implements user.UserRepository
      pat_repository.go   ← implements pat.PATRepository
    jwt/
      jwt.go              ← JWT infrastructure adapter
    oauth2/
      oauth2.go           ← Provider interface
      google.go, github.go

  api/
    auth/
      handler.go          ← AuthHandler  (HTTP, calls AuthService)
      routes.go
    pats/
      handler.go          ← PATHandler   (HTTP, calls AuthService)
      routes.go
    users/
      handler.go          ← UserHandler  (HTTP, calls AuthService)
      routes.go
    router.go
    common/
      middleware.go
```

### Violation checklist (use at PR review)

| Violation | Example | Fix |
|-----------|---------|-----|
| Handler calls Manager directly | `AuthHandler` calls `AuthManager.ResolveByPAT` | Route through `AuthService` |
| Handler calls Repository directly | `AuthHandler` calls `UserRepository.FindByID` | Route through Service → Manager |
| Service contains business logic | Service validates PAT expiry | Move to Manager |
| Service calls Repository directly | `AuthService` calls `UserRepository.Upsert` | Route through Manager |
| Manager calls another Manager | `AuthManager` calls `JWTManager` | Compose in Service |
| Repository contains business rules | Repo checks if PAT is expired | Move to domain or Manager |
| Infrastructure adapter imported in Service | Service imports `jwt` package | Use Manager |

---

## Consequences

### Positive
- Every feature follows the same structure — onboarding is faster.
- Managers are unit-testable without any infrastructure running.
- Services are trivially testable: mock all managers, assert orchestration order.
- Handlers are testable with `net/http/httptest` and a mock Service.
- Business logic is concentrated in Managers — easy to find and change.
- The violation checklist gives reviewers a concrete tool at PR time.

### Negative
- Small features may feel over-engineered (e.g. a read-only endpoint needs
  Handler → Service → Manager → Repository for a single query).
- More files per feature compared to a flat structure.

### Mitigations
- For trivially simple read-only features a Manager may be omitted and the
  Service may call the Repository directly — this must be explicitly noted in
  the PR and justified. It is the exception, not the rule.

---

## Alternatives considered

### Fat service (Handler → Service → Repository)
Skips the Manager layer. Rejected because business logic accumulates in Service
methods, making them long and hard to test in isolation.

### Hexagonal / Ports-and-Adapters (strict)
More complete but requires adapter interfaces at every boundary, adding
significant boilerplate for a small team. The current model is a pragmatic
subset that captures the most valuable invariants without the overhead.

### Single layer (Handler + everything)
Rejected immediately — seen in the first implementation attempt where Service
called infrastructure adapters directly.
