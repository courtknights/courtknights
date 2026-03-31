# CourtKnights — Claude Code Guidelines

## Project Overview

**CourtKnights** is an open-source platform for creating and managing sports leagues. It supports player management and statistics, team creation, and league setup with multiple tournament formats — enabling small clubs to build and run their own communities.

- **Backend:** Go (Golang)
- **Frontend:** Angular with PrimeNG
- **Database:** PostgreSQL
- **Repo structure:** Monorepo (frontend + backend)
- **License:** Apache 2.0 — CourtKnights Community
- **Versioning:** SemVer

---

## Development Workflow: Spec Driven Development (SDD)

CourtKnights follows a **Spec Driven Development** model where:
- The **human** acts as architect and validator
- The **AI agent** acts as executor

The specification is the agent's contract. No code is written without an approved spec. The agent always reads from the general to the specific.

### Mandatory reading order

Before any task, read in this exact order:

1. `docs/context/` — business domain, users, glossary
2. `docs/decisions/` — active global ADRs
3. `docs/testing/strategy.md` — global testing rules
4. `docs/specs/FEATURE_xxx/` — spec for the feature in progress

### Docs structure

```
docs/
  context/
    business.md           # Product and problem — human + AI
    users.md              # User types and roles — human + AI
    glossary.md           # Domain terms — human + AI
  decisions/
    ADR-NNN_title.md      # Global architecture decisions
  testing/
    strategy.md           # Framework, minimum coverage, conventions
    regression_map.md     # Feature dependencies — update on every merge
  specs/
    FEATURE_xxx/
      01_business.md      # Business context — human writes, AI expands
      02_architecture.md  # Technical design — AI generates, human reviews
      03_tasks.md         # Atomic tasks — AI generates, human validates
      04_tests.md         # Test cases — AI generates, human reviews
      05_acceptance.md    # Acceptance criteria — human writes
```

### Feature lifecycle

1. Human drafts `01_business.md`
2. AI expands the business doc — human validates
3. AI generates `02_architecture.md` reading business + global ADRs
4. Human reviews architecture; new decisions go to `docs/decisions/`
5. AI generates `03_tasks.md` and `04_tests.md`
6. Human writes `05_acceptance.md`
7. Human approves the full task breakdown
8. Each task in `03_tasks.md` is persisted as a GitHub Issue (label: `agent-task`)

### Task execution

- The agent reads the assigned GitHub Issue and the full feature spec
- Implements one task at a time and opens one PR per task
- Before opening a PR: run feature tests + full regression suite
- Human reviews the PR, validates `05_acceptance.md`, and approves the merge
- On merge: update `docs/testing/regression_map.md`

---

## Repository Structure

```
courtknights/
  go.work           # Go workspace (coordinates api/ module)
  Makefile          # Dispatcher — delegates to workspace Makefiles
  api/              # Go workspace
    go.mod
    go.sum
    Makefile        # Backend-specific commands
    cmd/            # Go entrypoints (main packages)
    internal/       # Go internal packages (domain, application, infrastructure)
    build/          # Compiled binaries (gitignored)
  db/               # Database workspace (SQL — no Go module)
    Makefile        # Migration commands (migrate, rollback, status)
    README.md
    schema/         # PostgreSQL schema definitions
    migrations/     # Database migrations
  web/              # Angular frontend workspace
  infra/            # Deployment workspace (scaffold — manifests and scripts)
  e2e/              # Acceptance test workspace (scaffold — Playwright)
  docs/
    context/        # Business domain, users, glossary
    decisions/      # Global Architecture Decision Records (ADR-NNN_title.md)
    testing/        # Testing strategy and regression map
    specs/          # Feature specs (one directory per feature: FEATURE_xxx/)
  Dependency.md     # All direct dependencies (name, purpose, license)
```

---

## Commands

| Task | Command |
|------|---------|
| Build backend | `make build` |
| Run backend | `make run` |
| Test backend | `make test` |
| Lint backend | `make lint` |
| Install frontend deps | `make web-install` |
| Run frontend dev server | `make web-start` |
| Build frontend | `make web-build` |
| Test frontend | `make web-test` |
| Apply DB migrations | `make db-migrate` |
| Rollback last migration | `make db-rollback` |
| Show migration status | `make db-status` |

> All commands are dispatched from the root `Makefile` to the relevant workspace `Makefile`.
> To run backend-specific commands directly: `cd api && make <target>`.
> Keep Makefiles up to date as the project evolves.

---

## Go Conventions

- Follow the **standard Go style** enforced by `gofmt`.
- Linting via **golangci-lint** — run `make lint` before committing.
- Package names: short, lowercase, no underscores.
- Exported identifiers must have a doc comment.
- Error wrapping: use `fmt.Errorf("context: %w", err)`.
- No global state. Prefer dependency injection through constructors.
- Table-driven tests preferred.
- All code, comments, and documentation must be written in **English**.

### Key technology decisions

The rationale for framework and technology choices is captured in the ADRs. Always read the relevant ADR before working on a new area:

| Concern | Decision | ADR |
|---------|----------|-----|
| HTTP API framework | Echo v4 | [ADR-001](docs/decisions/ADR-001_http-framework-echo.md) |
| CLI + configuration | Cobra + Viper | [ADR-002](docs/decisions/ADR-002_cli-cobra-viper.md) |
| Primary database | PostgreSQL | [ADR-003](docs/decisions/ADR-003_database-postgresql.md) |
| Integration test infrastructure | Testcontainers | [ADR-004](docs/decisions/ADR-004_testcontainers.md) |
| Go workspace | go.work | [ADR-009](docs/decisions/ADR-009_go-workspace.md) |
| Database migrations | golang-migrate | [ADR-010](docs/decisions/ADR-010_migration-tool-golang-migrate.md) |

### Dependency tracking

- Every direct dependency added to the project must be recorded in **`Dependency.md`** with:
  - **Name** — import path or package name
  - **Purpose** — why it is used
  - **License** — the dependency's license

---

## Angular / TypeScript Conventions

- **Strict TypeScript** is mandatory — `strict: true` in `tsconfig.json`. No `any`.
- **Readability over cleverness** — code is written once but read many times.
- Follow the **Angular Style Guide** (naming, file structure, lifecycle hooks).
- Use **standalone components** (Angular 17+). Avoid NgModules for new features.
- Organize by **feature modules** — each feature lives in its own directory under `web/src/app/features/`.
- **ESLint + Prettier** enforce formatting — do not disable rules without a comment explaining why.
- Component files: `feature-name.component.ts`, `feature-name.component.html`, `feature-name.component.scss`.
- Services are injected via `inject()` or constructor injection — no service locator pattern.

---

## Git Conventions

- Branch naming: `CK-{issue-number}_{short-description}` (e.g. `CK-1_sdd-scaffolding`)
- Commit messages: `[CK-{issue-number}] {message}` — imperative mood, present tense (e.g. `[CK-1] add spec template for league creation`)
- Every PR must:
  - Include `Closes #N` (exact syntax) in the PR **body** — this is what GitHub uses to auto-close the issue on merge. Do not use "References" or "See #N"; only `Closes`, `Fixes`, or `Resolves` trigger auto-close.
  - Reference the related spec (`Spec: docs/specs/...`)
  - Pass all CI checks before merge

---

## What Claude Should Always Do

- Follow the mandatory reading order before any task: `context/` → `decisions/` → `testing/strategy.md` → feature spec.
- Implement one task at a time (one PR per task).
- Run feature tests + full regression suite before opening any PR.
- Update `docs/testing/regression_map.md` after a merge.
- Run `make lint` and `make test` after backend changes.
- Run `npm test` after frontend changes.
- Document new technical decisions in `docs/decisions/` — never assume them silently.
- Keep `Dependency.md` up to date when adding or removing direct dependencies.
- Keep CLAUDE.md up to date when conventions evolve.

## What Claude Should Never Do

- Write feature code without an approved spec in `docs/specs/FEATURE_xxx/`.
- Install or add a dependency without explicit human approval.
- Proceed with an ambiguous task — ask the human before implementing.
- Violate an active ADR in `docs/decisions/`; if there is a conflict, notify the human.
- Use `any` in TypeScript.
- Introduce global state in Go.
- Merge to `main` directly — always go through a PR.
- Add a direct dependency without registering it in `Dependency.md`.
- Write code, comments, or documentation in any language other than English.
