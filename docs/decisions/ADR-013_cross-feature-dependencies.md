# ADR-013: Cross-Feature Dependencies

- **Status:** accepted
- **Date:** 2026-10-12
- **Deciders:** aagea

---

## Context

[ADR-008](ADR-008_layered-architecture.md) defines how a single feature is
layered (Handler → feature manager → sub-managers → repositories). It allows a
manager to call "other managers" through injected interfaces, but it does not
say whether, or how, one **feature** may depend on another.

FEATURE_user_profile (#93) was the first real case: the authentication feature
must create a profile on every successful login, which is a use case owned by
the user_profile feature. It was solved in #118 by declaring a narrow interface
inside the consuming package:

```go
// api/internal/application/auth/manager.go
type ProfileInitializer interface {
    EnsureExists(ctx context.Context, userID uuid.UUID, displayName string) error
}
```

`appprofile.ProfileManager` satisfies it structurally, and `cmd/server` wires it
into `NewAuthManager` without an adapter. The AI ADR consistency check
([ADR-011](ADR-011_ai-adr-consistency-check.md)) flagged this on #112 as a
possible blurring of feature boundaries, because no ADR covered it.

The pattern will recur: leagues, teams and matches will all need player
profiles. Without a rule, each feature would solve it differently, and features
would end up importing each other's application packages, which creates import
cycles and makes features impossible to test in isolation.

## Decision

### 1. Depend on a narrow interface declared by the consumer

When feature A needs a use case owned by feature B, feature A declares a Go
interface **in its own application package**, containing only the methods it
calls. Feature A never imports feature B's `application/` or `infrastructure/`
packages.

```go
// application/leagues/manager.go  (consumer)
type PlayerDirectory interface {
    GetByUserID(ctx context.Context, userID uuid.UUID) (*profile.Profile, error)
}
```

- **Naming:** the interface is named after the capability, as seen from the
  consumer (`ProfileInitializer`, `PlayerDirectory`). It does **not** use the
  `Manager` suffix, which ADR-008 reserves for managers owned by the feature
  itself.
- **Placement:** in the consuming feature manager's file, or in a `deps.go` file
  in the same package when there are several.
- **Mocking:** the consumer's tests mock this narrow interface, never the full
  manager interface of feature B.

### 2. Cross-feature calls happen only at the feature-manager level

Only a feature manager (or one of its sub-managers) may call another feature.
Handlers always call **their own** feature manager only, as ADR-008 already
requires. A Handler never holds a dependency on another feature.

### 3. Wiring happens in `cmd/server`

The composition root (`api/cmd/server/server.go`) builds feature B's manager and
passes it to feature A's constructor. Go's structural typing means no adapter is
needed, as long as B's manager has the methods A's interface declares.

### 4. Domain packages may be shared; application and infrastructure may not

| Feature A may import from feature B | Allowed |
|-------------------------------------|---------|
| `domain/<b>/` — entities, value types, enums | ✅ |
| `domain/ckerrors/` — sentinel errors | ✅ (shared by all features) |
| `application/<b>/` | ❌ — declare a narrow interface instead (rule 1) |
| `infrastructure/...` adapters owned by B | ❌ — go through B's feature manager |
| `api/<b>/` | ❌ |

Domain packages are pure types with no dependencies on other layers, so
sharing them avoids duplicating types such as `profile.Category` in every
feature.

### 5. No shared transactions; cross-feature writes must be idempotent

A cross-feature call never shares a database transaction with the caller.
Each feature commits its own writes. Consequently, **any write operation that
one feature exposes to another must be idempotent**, so that retrying the whole
use case repairs a partial failure.

Example: on login, `AuthManager` upserts the user (auth's write) and then calls
`ProfileInitializer.EnsureExists` (profile's write, `INSERT … ON CONFLICT DO
NOTHING`). If the second call fails, login returns an error. The next login
retries and creates the missing profile.

### 6. Synchronous calls only (for now)

Features communicate through direct, synchronous calls via the interfaces
above. Domain events or an in-process event bus are **out of scope** until a
concrete need appears.

Revisit this decision when:
- a single action must notify three or more features, or
- a cross-feature call must not block or fail the caller's use case.

### 7. Dependencies between features must be acyclic and recorded

- Feature dependencies form a directed acyclic graph. If A depends on B, B may
  not depend on A, directly or transitively. Go's import rules enforce this for
  packages; this rule extends it to the wiring done in `cmd/server`.
- Every cross-feature dependency is recorded in
  `docs/testing/regression_map.md`, so that changes to feature B trigger the
  regression tests of feature A.
- The feature's `02_architecture.md` names each cross-feature interface it
  declares and which feature satisfies it.

### Cross-cutting infrastructure

Authentication middleware (`api/internal/api/common`) is used by every
feature's routes and depends on the authentication feature's `JWTManager`.
This is **cross-cutting HTTP infrastructure**, not a cross-feature dependency
under this ADR. It is wired once in `cmd/server` and applied at the router
level.

### Violation checklist (use at PR review)

| Violation | Example | Fix |
|-----------|---------|-----|
| Consumer imports another feature's application package | `application/leagues` imports `application/profile` | Declare a narrow interface in `application/leagues` |
| Consumer depends on the full manager interface | Field typed `appprofile.ProfileManager` | Narrow interface with only the called methods |
| Handler holds another feature's dependency | `leagues.Handler{ profiles appprofile.ProfileManager }` | Route through the leagues feature manager |
| Narrow interface named `…Manager` | `type ProfileManager interface { EnsureExists(…) }` in auth | Name it after the capability (`ProfileInitializer`) |
| Non-idempotent cross-feature write | `profiles.Create(…)` fails on second login | Make the operation idempotent (`EnsureExists`) |
| Shared transaction across features | Passing a `pgx.Tx` from auth into profile | Each feature commits its own writes (rule 5) |
| Dependency not in regression map | New interface, no `regression_map.md` row | Add the row in the same PR |

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| Consumer imports the provider's full `application/<b>` interface | Couples the consumer to methods it never calls, widens the test surface, and creates import cycles as soon as B needs A. This is what #118 removed. |
| Amend ADR-008 instead of a new ADR | ADR-008 is about layering inside a feature. A separate ADR keeps that scope clean and records when and why cross-feature rules were introduced. ADR-008 links here. |
| Each feature duplicates the other feature's domain types | Maximum isolation, but forces mapping between identical types (`Category`, `Gender`, …) with no real benefit. Domain packages have no dependencies, so sharing them is cheap. |
| Shared transaction passed through `context` | Strong consistency, but couples features to the same database and transaction lifecycle, and complicates every repository signature. Idempotent writes give the same end state with far less coupling. |
| In-process domain event bus | Better decoupling for fan-out, but it is new infrastructure without a second use case to justify it. Revisit triggers are listed in rule 6. |

## Consequences

### Positive

- Each feature declares exactly what it needs from others; its tests mock a
  one- or two-method interface.
- No import cycles between application packages, by construction.
- Feature boundaries are visible in code (the narrow interfaces) and in docs
  (`02_architecture.md`, `regression_map.md`).
- The existing `ProfileInitializer` already complies, so no code change is
  needed.

### Negative

- A small amount of interface boilerplate in each consuming feature.
- When feature B renames or changes a method, the compiler error appears in
  `cmd/server` wiring rather than next to the interface declaration.
- No atomicity across features. Partial failures are possible and rely on
  idempotent retries.

## References

- [ADR-008 — Layered Architecture](ADR-008_layered-architecture.md)
- [ADR-011 — AI ADR consistency check](ADR-011_ai-adr-consistency-check.md)
- `docs/specs/FEATURE_user_profile/02_architecture.md` — `ProfileInitializer`
- #118 — introduced `ProfileInitializer`
- #112 — ADR consistency warning that prompted this ADR
- #125 — this decision
