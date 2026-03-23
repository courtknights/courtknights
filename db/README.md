# db — Database workspace

This workspace contains the PostgreSQL schema definitions and migration files for
CourtKnights. It has no Go module — it owns SQL files only.

## Structure

```
db/
  schema/       # Reference schema definitions (documentation purposes)
  migrations/   # Migration files managed by golang-migrate
  Makefile      # Migration commands
```

## Migrations

Migrations are managed with [golang-migrate](https://github.com/golang-migrate/migrate).
Each migration consists of a pair of SQL files:

```
{version}_{description}.up.sql
{version}_{description}.down.sql
```

For example:

```
000001_create_users.up.sql
000001_create_users.down.sql
```

The version prefix must be a zero-padded integer. Migrations are applied in ascending
version order and rolled back in descending order.

## Available commands

Override `DB_URL` with your connection string before running any target.

| Command | Description |
|---------|-------------|
| `make migrate` | Apply all pending migrations |
| `make rollback` | Revert the last applied migration |
| `make status` | Show current migration version |

```sh
DB_URL=postgres://user:pass@localhost:5432/mydb?sslmode=disable make migrate
```

## Prerequisites

The `migrate` CLI must be installed in your environment. Installation:

```sh
# macOS
brew install golang-migrate

# or via Go
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```
