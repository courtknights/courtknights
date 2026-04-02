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
- Before opening the feature→main PR: update `docs/testing/regression_map.md` and include it in the PR

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

### Branch types

CourtKnights uses a hierarchical branching model (ADR-012). Five branch types exist:

| Type | Pattern | Branches from | Merges to | Merge strategy |
|------|---------|---------------|-----------|----------------|
| Feature | `feature/CK-XXX/branch` | `main` | `main` | Merge commit |
| Design | `feature/CK-XXX/000_design` | `feature/CK-XXX/branch` | `feature/CK-XXX/branch` | Squash |
| Task | `feature/CK-XXX/NNN_description` | `feature/CK-XXX/branch` | `feature/CK-XXX/branch` | Squash |
| Hotfix | `fix/CK-XXX_description` | `main` | `main` | Squash |
| Chore | `chore/CK-XXX_description` | `main` | `main` | Squash |

Where `NNN` matches the `TASK-NNN` number from `03_tasks.md` (zero-padded to three digits).

**Important:** task branches target the feature branch (`feature/CK-XXX/branch`), not `main` directly.

### Chore branch exception

A chore branch is for minor maintenance work that does not introduce new product functionality and does not require a spec (dependency upgrades, small refactors, documentation fixes, CI maintenance). A GitHub Issue is still required for traceability. The `spec-ref` CI job is skipped automatically for `chore/**` and `fix/**` branches.

### Commit messages

`[CK-{issue-number}] {message}` — imperative mood, present tense.

- Task commit example: `[CK-68] add ADR-012 hierarchical branching strategy`
- Chore commit example: `[CK-66] upgrade golangci-lint to v1.58`

### GitHub Issues and sub-issues

Each feature has a parent GitHub Issue (the feature issue). Every task issue created from `03_tasks.md` must be registered as a sub-issue of its feature issue using the GitHub sub-issues API:

```bash
gh api repos/{owner}/{repo}/issues/{feature-issue}/sub_issues \
  --method POST --field sub_issue_id={task-issue-id}
```

This links task progress directly to the feature issue in the GitHub UI. Task issues close automatically when the feature branch merges to `main` (via `Closes #N` in the feature PR body). Do not close them manually before that point.

### PR requirements

Every PR must:
- Include `Closes #N` (exact syntax) in the PR **body** — this triggers GitHub's auto-close on merge. `References` or `See #N` do not close the issue.
- Include `Spec: docs/specs/...` in the PR body (waived automatically for `chore/**` and `fix/**` branches).
- Pass all CI checks before merge.

---

## What Claude Should Always Do

- Follow the mandatory reading order before any task: `context/` → `decisions/` → `testing/strategy.md` → feature spec.
- Implement one task at a time (one PR per task).
- Run feature tests + full regression suite before opening any PR.
- Update `docs/testing/regression_map.md` in the feature→main PR, not after the merge.
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
