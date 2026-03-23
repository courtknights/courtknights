# ADR-010: Database Migration Tool — `golang-migrate`

- **Status:** accepted
- **Date:** 2026-03-23
- **Deciders:** aagea

---

## Context

CourtKnights uses PostgreSQL as its primary database (ADR-003). As the schema evolves,
migrations must be applied and rolled back in a controlled, versioned manner across
local development, CI, and production environments.

The `FEATURE_repo_workspaces` restructuring moves database artefacts into a dedicated
`db/` workspace. A migration tool must be chosen that fits the following constraints:

- SQL-first: migrations are plain SQL files, not Go code or a proprietary DSL.
- CLI-driven: runnable from `db/Makefile` without compiling the application.
- PostgreSQL native support.
- Actively maintained and widely adopted.

## Decision

Use **[`golang-migrate`](https://github.com/golang-migrate/migrate)** as the database
migration tool.

- Migrations are plain SQL files stored in `db/migrations/`.
- File naming follows the `golang-migrate` convention:
  `{version}_{description}.up.sql` / `{version}_{description}.down.sql`
  (e.g. `000001_create_users.up.sql`).
- The `migrate` CLI is invoked from `db/Makefile` via the `DB_URL` environment variable.
- Version tracking is handled by `golang-migrate`'s built-in `schema_migrations` table
  in PostgreSQL.

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| `goose` | Also SQL-first and widely used, but requires Go code for complex migrations and the CLI is less ergonomic for pure-SQL workflows |
| `atlas` | More powerful schema diffing, but introduces a schema definition language (HCL/SQL) and a heavier toolchain — over-engineered for current needs |
| Manual SQL scripts + `psql` | No version tracking, no rollback support, error-prone in CI |
| Embedded migrations in the Go binary | Couples schema management to the application release cycle; prevents running migrations independently |

## Consequences

### Positive
- Migrations are plain `.sql` files — readable and editable by anyone without Go
  knowledge.
- Rollback is a first-class operation (`migrate down 1`).
- The `migrate` CLI can be used in CI without building the application.
- If needed in the future, `golang-migrate` also ships a Go library that can be embedded
  in the application for automated startup migrations.

### Negative
- The `migrate` CLI binary must be installed separately in developer environments and CI
  (not managed by `go.mod`).
- `golang-migrate` does not support transactional DDL on databases that lack it; for
  PostgreSQL this is not an issue.
- No built-in schema diffing — migrations must be written by hand.

## References

- https://github.com/golang-migrate/migrate
- [ADR-003](ADR-003_database-postgresql.md) — PostgreSQL as primary database
- `docs/specs/FEATURE_repo_workspaces/02_architecture.md` — `db/` workspace design
