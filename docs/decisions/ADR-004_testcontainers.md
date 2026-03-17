# ADR-004: Integration Test Infrastructure — Testcontainers

- **Status:** accepted
- **Date:** 2026-03-17
- **Deciders:** CourtKnights Community

---

## Context

Integration tests need to run against real infrastructure (PostgreSQL and any future dependencies) without requiring a manually managed local setup. The solution must be:

- Self-contained: each test suite spins up and tears down its own dependencies.
- Reproducible: same behaviour in local development and CI.
- Isolated: one test suite's infrastructure does not interfere with another's.

## Decision

Use **[Testcontainers-Go](https://golang.testcontainers.org/)** to manage real infrastructure for integration tests.

- Each integration test package that needs a database creates its own PostgreSQL container via Testcontainers.
- Containers are started at the beginning of the test suite (`TestMain`) and stopped when it ends.
- Integration test files use the build tag `//go:build integration` and are excluded from the default `go test ./...` run.
- Run with `make test-int`.

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| Shared Docker Compose for tests | Requires a running Docker Compose before tests; not self-contained; state bleeds between suites |
| In-memory SQLite | Not representative of PostgreSQL behaviour (types, constraints, functions); breaks the principle of testing against real infrastructure |
| Mocking the DB layer in integration tests | Defeats the purpose of integration tests; mocks have caused prod/mock divergence in the past |
| Manual `docker run` in CI | Fragile, hard to version, and couples CI config to test internals |

## Consequences

### Positive
- Integration tests are fully self-contained and portable: run identically locally and in CI
- Each suite starts clean with a fresh container — no shared state between test runs
- Testcontainers handles container lifecycle automatically; no manual teardown needed
- Easy to add new infrastructure types (Redis, S3-compatible, etc.) as the project grows

### Negative
- Docker must be available in the environment where integration tests run (local and CI)
- Container startup adds a few seconds per suite — acceptable for integration level, not for unit level
- Adds `testcontainers-go` as a test dependency

## References
- https://golang.testcontainers.org/
- `docs/testing/strategy.md` — test level definitions
- `Dependency.md` — Testcontainers-Go entry
