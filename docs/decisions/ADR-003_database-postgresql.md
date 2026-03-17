# ADR-003: Primary Database — PostgreSQL

- **Status:** accepted
- **Date:** 2026-03-17
- **Deciders:** CourtKnights Community

---

## Context

The platform manages structured relational data: players, teams, leagues, matches, and statistics. The database must support complex queries, referential integrity, and transactional operations. It must also be open-source and self-hostable, aligned with the project's community-first philosophy.

## Decision

Use **PostgreSQL** as the primary database.

- Schema definitions live in `db/schema/`.
- Migrations live in `db/migrations/` and must be sequential and reversible.
- No raw SQL outside of the repository/data layer (`internal/infrastructure/`).

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| MySQL / MariaDB | PostgreSQL offers superior support for JSON, arrays, window functions, and full-text search |
| SQLite | Not suitable for multi-user concurrent access or production deployments |
| MongoDB | The domain is inherently relational; a document store would complicate joins and integrity |
| CockroachDB | Operational complexity not justified at this stage; can be revisited when horizontal scale is needed |

## Consequences

### Positive
- Strong ACID guarantees
- Rich query capabilities (CTEs, window functions, JSONB, arrays)
- Widely supported by hosting providers and self-hosted setups
- Open-source and free

### Negative
- Requires a running PostgreSQL instance for local development (mitigated by Docker)
- Schema migrations must be managed carefully to remain reversible

## References
- https://www.postgresql.org/
- `db/schema/` — schema definitions
- `db/migrations/` — migration files
