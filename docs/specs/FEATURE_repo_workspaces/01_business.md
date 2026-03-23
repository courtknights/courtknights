# FEATURE_repo_workspaces — Business Context

- **Status:** draft
- **Feature ID:** FEATURE_repo_workspaces
- **Author:** aagea
- **Date:** 2026-03-22

---

## Problem statement

The CourtKnights repository currently mixes Go backend code, Angular frontend code, and documentation in a flat structure at the root level. As the project grows to include deployment manifests and acceptance tests, this layout makes it harder to:

- Understand the boundaries between concerns at a glance.
- Run toolchain-specific commands (Go, Node, k8s) without navigating implicit conventions.
- Onboard contributors who only need to work on one area of the stack.
- Evolve each concern (backend, frontend, infra, tests) independently.

## Users affected

This is an internal developer experience improvement. It affects:

- **Backend contributors** — Go developers working in `api/`.
- **Frontend contributors** — Angular developers working in `web/`.
- **DevOps / infra contributors** — working in `infra/`.
- **QA / acceptance test contributors** — working in `e2e/`.
- **AI agents** — which must navigate the repository to implement tasks.

No end-user-facing behaviour changes.

## Business value

- **Explicit workspace boundaries** — each contributor knows exactly where their code lives and what toolchain applies.
- **Independent toolchain management** — Go modules, Node packages, k8s manifests, and Playwright configs each live in their own workspace with their own dependency management.
- **Scalable CI** — CI jobs can target individual workspaces, avoiding unnecessary rebuilds and test runs.
- **Cleaner onboarding** — a frontend developer never needs to understand the Go module layout; a backend developer never needs to touch `web/`.
- **Without this change:** the flat structure becomes increasingly confusing as `infra/` and `e2e/` are added, and the root `go.mod` continues to collide conceptually with the rest of the repo.

## Scope

### In scope

- Reorganise the repository into four workspaces:

  | Workspace | Folder | Toolchain |
  |-----------|--------|-----------|
  | Backend | `api/` | Go — own `go.mod`, coordinated via `go.work` at root |
  | Frontend | `web/` | Angular / Node — already exists, consolidated |
  | Deployment | `infra/` | Bash scripts, Kubernetes manifests |
  | Acceptance tests | `e2e/` | Playwright |

- Move all Go source code (`cmd/`, `internal/`, `db/`) and the Go `go.mod` / `go.sum` into `api/`.
- Add a `go.work` file at the repository root referencing the `api/` module.
- Update all Go import paths to reflect the new module root.
- Add a local `Makefile` in `api/` for backend-specific commands.
- Update the root `Makefile` to delegate to workspace `Makefile`s (e.g. `make api/test`, `make web/build`).
- Scaffold `infra/` and `e2e/` with the minimum structure needed (README, placeholder config).
- Update `CLAUDE.md`, `docs/`, and all documentation references to reflect the new structure.

### Out of scope

- Implementing any deployment manifests in `infra/` (structure only).
- Writing actual Playwright tests in `e2e/` (scaffold only).
- Splitting into multiple Git repositories.
- Any changes to application logic, APIs, or database schema.

## Assumptions and dependencies

- Go 1.22+ is in use, which fully supports `go.work` workspaces.
- The root `go.mod` module path (e.g. `github.com/courtknights/courtknights`) will become the module path in `api/go.mod`. All internal imports (e.g. `github.com/courtknights/courtknights/internal/...`) will be updated accordingly.
- The existing `web/` directory stays named `web/` — no rename needed.
- CI pipelines do not yet exist (covered by issue #3); this restructure must be compatible with the future CI design.
- Node / npm configuration in `web/` requires no changes beyond path references in the root `Makefile`.

## Glossary

| Term | Definition |
|------|-----------|
| Workspace | A self-contained directory within the monorepo with its own toolchain, dependency manifest, and local `Makefile`. |
| `go.work` | Go's native workspace file (Go 1.18+) that coordinates multiple Go modules within a single repository. |
| Scaffold | Creating the minimum directory structure and placeholder files needed to establish a workspace, without implementing its full content. |
