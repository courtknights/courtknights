# FEATURE_branching_workflow — Acceptance Criteria

- **Status:** approved
- **Feature ID:** FEATURE_branching_workflow
- **Author:** aagea
- **Date:** 2026-03-30

---

## Definition of done

The feature is done when:
- All seven tasks are merged into `feature/CK-64/branch`.
- The feature branch is merged to `main` with a merge commit.
- All CI jobs pass on the feature→main PR.
- `make apply-rulesets` has been run and branch protection is active on `main` and `feature/**/branch`.
- `docs/testing/regression_map.md` is updated.

---

## Functional criteria

### Branching model

- [ ] Five branch types are defined and documented: feature, design, task, hotfix, chore.
- [ ] The chore branch exception is documented: no spec required, branches from `main`, squash-merges to `main`, runs full CI suite.
- [ ] ADR-012 exists at `docs/decisions/ADR-012_branching-strategy.md` with status `Accepted` and covers all five branch types.

### Two-layer CI

- [ ] A PR from a task branch (`feature/CK-XXX/NNN_*`) targeting `feature/**/branch` triggers exactly: `spec-ref`, `backend-cov-unit`, `frontend-cov` (TC-001).
- [ ] A PR from `feature/**/branch` targeting `main` triggers exactly: `spec-ref`, `backend-cov-full`, `frontend-cov`, `adr-consistency` (TC-002).
- [ ] A PR from a `chore/**` branch targeting `main` triggers `backend-cov-full`, `frontend-cov`, `adr-consistency` — `spec-ref` is skipped.
- [ ] A PR from a `fix/**` branch targeting `main` triggers `backend-cov-full`, `frontend-cov`, `adr-consistency` — `spec-ref` is skipped.
- [ ] `backend-cov-unit` runs only unit tests; Docker is not required (TC-006).
- [ ] `backend-cov-full` runs both unit and integration tests (TC-007).

### Design branch content check

- [ ] A PR from `feature/CK-XXX/000_design` modifying only `docs/specs/` passes `design-content-check` (TC-003).
- [ ] A PR from `feature/CK-XXX/000_design` modifying any file outside `docs/specs/` fails `design-content-check` with a clear error listing the offending files (TC-004).
- [ ] `design-content-check` does not run on branches that do not end in `/000_design` (TC-005).

### Ruleset configuration

- [ ] `infra/rulesets/main.json` and `infra/rulesets/feature-branch.json` exist with valid JSON.
- [ ] `make apply-rulesets` applies both rulesets without error.
- [ ] Running `make apply-rulesets` a second time is idempotent (no error, no duplicate ruleset).
- [ ] `main` is protected: PR required, 1 approval required, status checks required, force push and deletion blocked.
- [ ] `feature/**/branch` is protected: PR required, 1 approval required, status checks required, force push and deletion blocked.

### Documentation

- [ ] `CLAUDE.md` Git Conventions section describes all five branch types with naming pattern, origin, target, and merge strategy. No references to the old flat branch model remain (TC-010).
- [ ] `CONTRIBUTING.md` exists at the repository root and covers: all five branch types with examples, the chore exception, local setup, PR requirements (including `spec-ref` waiver for chore/fix), local CI execution, and admin setup for `make apply-rulesets`.
- [ ] `README.md` exists at the repository root with elevator pitch, architecture overview, quick-start, and links to `CONTRIBUTING.md` and `LICENSE` (TC-009).
- [ ] `CONTRIBUTING.md` covers all required sections per TASK-005 done conditions (TC-008).

---

## Non-functional criteria

- [ ] `backend-cov-unit` completes in under 2 minutes on GitHub-hosted runners (no Docker spin-up).
- [ ] The `apply-rulesets` target produces no output other than progress messages on success.
- [ ] All documentation is written in English.

---

## Regression criteria

- [ ] Existing CI jobs (`spec-ref`, `frontend-cov`, `adr-consistency`, `backend-cov`) continue to pass on a PR targeting `main` after the workflow changes.
- [ ] No coverage thresholds regress as a result of this feature (no application code is changed).

---

## Review checklist

- [ ] ADR-012 reviewed and accepted.
- [ ] CI workflow validated with test PRs (TC-001 through TC-007).
- [ ] Chore branch scenario validated: PR from `chore/CK-66_improve-coverage` to `main` skips `spec-ref`.
- [ ] `make apply-rulesets` run by repository admin; branch protection confirmed active in GitHub UI.
- [ ] `CONTRIBUTING.md` reviewed for accuracy.
- [ ] `README.md` quick-start verified on a clean checkout (TC-009).
- [ ] `CLAUDE.md` reviewed — no old flat-branch references remain (TC-010).
