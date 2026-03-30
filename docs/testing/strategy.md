# Testing Strategy

- **Last updated:** 2026-03-17

---

## Principles

- Tests are a first-class deliverable — defined in `04_tests.md` before implementation begins.
- The full test suite runs before every PR is opened.
- Tests are written in English.
- Three test levels exist, each with a distinct scope, speed, and infrastructure requirement.

---

## Test levels

### Level 1 — Unit tests

**Goal:** verify isolated logic with no external processes running.

- No database, no network, no filesystem.
- All external dependencies (DB, HTTP clients, external services) are replaced with **mocks or fakes**.
- Must be fast enough to run on every file save during development.
- If a unit test requires an external process to pass, it is not a unit test.

### Level 2 — Integration tests

**Goal:** verify that a single component behaves correctly against its real dependencies.

- The component under test is exercised against **real infrastructure** (DB, message broker, etc.) spun up on demand via **[Testcontainers](https://testcontainers.com/)** (see [ADR-004](../decisions/ADR-004_testcontainers.md)).
- Other application components (other services, the HTTP layer of a different module) are **mocked or stubbed**.
- Each integration test suite is self-contained: it starts its own containers, runs its scenarios, and tears down.
- Tests specific flows of the component in isolation — not full user journeys.

### Level 3 — Acceptance tests

**Goal:** verify complete end-to-end user flows against a fully deployed application.

- Acceptance tests live in a **separate repository** and are not part of this monorepo.
- Executed exclusively by the **CI/CD pipeline**: the pipeline deploys the full application, runs the acceptance suite, and reports results.
- Tests interact exclusively through the **public APIs** of the application (HTTP endpoints, public interfaces).
- No internal state is accessed directly — only observable outputs are asserted.
- Not run locally by developers; the CI/CD environment is the only execution context.

---

## Backend (Go)

### Frameworks

| Purpose | Tool |
|---------|------|
| Unit & integration tests | `testing` (stdlib) + `testify` |
| Mocks (unit tests) | `testify/mock` or hand-written fakes |
| Integration containers | Testcontainers-Go ([ADR-004](../decisions/ADR-004_testcontainers.md)) |
| HTTP handler tests (unit) | `net/http/httptest` + Echo test helpers |

### File naming convention

| Level | File suffix | Example |
|-------|------------|---------|
| Unit | `_test.go` | `league_service_test.go` |
| Integration | `_integration_test.go` | `league_repository_integration_test.go` |

Integration test files use the build tag `//go:build integration` so they are excluded from the default `go test ./...` run.

### Coverage minimum

| Layer | Minimum | Warning zone |
|-------|---------|--------------|
| `api/internal/domain/` | ≥ 90% (unit) | 80–89% |
| `api/internal/application/` | ≥ 80% (unit) | 70–79% |
| `api/internal/infrastructure/` | ≥ 70% (integration) | 60–69% |
| `api/internal/api/` | ≥ 80% (unit + integration) | 70–79% |

A layer in the **warning zone** (within 10 percentage points of its minimum) does not block the PR but posts an automated comment to flag the drift. Create a follow-up task to restore coverage before it crosses the error threshold.

### Running tests

```bash
make test          # unit tests only (no external processes required)
make test-int      # integration tests (Testcontainers spins up dependencies)
make test-all      # unit + integration
```

> Acceptance tests are maintained in a separate repository and executed exclusively by the CI/CD pipeline after deployment.

---

## Frontend (Angular)

### Frameworks

| Purpose | Tool |
|---------|------|
| Unit & component tests | Jest + Angular Testing Library |
| Acceptance / E2E | Playwright |

### Test levels mapping

| Level | What it covers | Infrastructure |
|-------|---------------|----------------|
| Unit | Services, pipes, pure functions, component logic | None — all HTTP calls mocked |
| Integration | Component rendering + interactions against mock API | None — MSW or similar HTTP interceptor |

### Coverage minimum

| Scope | Minimum | Warning zone |
|-------|---------|--------------|
| `web/src/app/` overall | ≥ 75% | 65–74% |
| Public service methods | 100% | 90–99% |

A check in the **warning zone** (within 10 percentage points of its minimum) does not block the PR but posts an automated comment to flag the drift. Create a follow-up task to restore coverage before it crosses the error threshold.

### Running tests

```bash
cd web && npm test              # unit + integration tests
```

> Frontend acceptance/E2E tests are part of the acceptance test repository and run via CI/CD only.

---

## CI enforcement

- `make test` (unit) runs on every push.
- `make test-int` (integration) runs on every PR.
- A PR that drops coverage below the defined minimums is blocked.
- Unit and integration tests must be green before merge.
- Acceptance tests run post-deployment in the CI/CD pipeline from the dedicated acceptance test repository.
