# FEATURE_repo_workspaces — Acceptance Criteria

- **Status:** approved
- **Feature ID:** FEATURE_repo_workspaces
- **Author:** aagea
- **Date:** 2026-03-23

---

## Definition of Done

A feature is considered complete when **all** criteria below are met and verified by the
human reviewer.

---

## Functional criteria

- [ ] **AC-01:** The repository root contains exactly five workspace directories: `api/`,
      `db/`, `web/`, `infra/`, and `e2e/`. No other workspace-level directories exist at
      the root.

- [ ] **AC-02:** The Go source tree (`cmd/`, `internal/`) and its module files (`go.mod`,
      `go.sum`) live exclusively under `api/`. No Go source files or `go.mod` exist at the
      repository root.

- [ ] **AC-03:** `db/schema/` and `db/migrations/` exist at the repository root level
      under `db/`. The `db/Makefile` exposes `migrate`, `rollback`, and `status` targets
      using the `golang-migrate` CLI.

- [ ] **AC-04:** A `go.work` file exists at the repository root referencing `./api`.
      `go.work.sum` is present and committed. Running `go build ./api/cmd/server` and
      `go test ./api/...` from the repository root succeeds.

- [ ] **AC-05:** The root `Makefile` contains only dispatcher targets (`$(MAKE) -C
      <workspace>`). No `go` or `npm` commands appear directly in it.

- [ ] **AC-06:** `api/Makefile` reproduces all backend targets from the original root
      `Makefile` and works when invoked from `api/` or via the root dispatcher.

- [ ] **AC-07:** `web/Makefile` exposes `install`, `start`, `build`, and `test` targets
      wrapping the corresponding `npm` commands.

- [ ] **AC-08:** `infra/` and `e2e/` are scaffolded with the files specified in
      `02_architecture.md`. Neither is wired into any CI job or Makefile target beyond
      the root scaffold.

- [ ] **AC-09:** The stale artefacts (`api/` stub, `ui/`, `services/`) from the previous
      incomplete restructuring no longer exist in the repository.

---

## Non-functional criteria

- [ ] `make lint` passes with no new warnings after the restructuring.
- [ ] No `any` types introduced in TypeScript (not applicable — no frontend changes).
- [ ] No new `golangci-lint` warnings introduced.
- [ ] `GOWORK=off go build ./cmd/server` from `api/` compiles successfully — the `api/`
      module is self-contained and does not depend on `go.work` to resolve its own
      dependencies.

---

## Regression criteria

- [ ] `make test` (unit) passes with the same results as before the restructuring.
- [ ] `make test-int` (integration) passes with the same results as before the
      restructuring.
- [ ] `make web-test` passes with the same results as before the restructuring.
- [ ] `docs/testing/regression_map.md` updated to reflect this feature.

---

## Review checklist

- [ ] `01_business.md` — all in-scope items implemented; out-of-scope items untouched.
- [ ] `02_architecture.md` — no undocumented deviations from the approved design.
- [ ] `03_tasks.md` — all seven tasks merged and their GitHub Issues closed.
- [ ] `04_tests.md` — all verifications executed and passing.
- [ ] `ADR-009` and `ADR-010` exist in `docs/decisions/` and accurately reflect the
      implementation.
- [ ] `Dependency.md` — `golang-migrate` CLI registered as a direct dependency.
