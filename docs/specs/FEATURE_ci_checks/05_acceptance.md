# FEATURE_ci_checks — Acceptance Criteria

- **Status:** approved
- **Feature ID:** FEATURE_ci_checks
- **Author:** aagea
- **Date:** 2026-03-25
- **GitHub Issue:** [#3](https://github.com/courtknights/courtknights/issues/3)

---

## How to validate

Each criterion is verified by opening a real pull request to `main` on the GitHub repository
and observing the resulting CI status checks and PR comments.

All four checks must appear as individual GitHub status checks on every PR.

---

## AC-001: Spec reference check blocks PRs without a spec line

**Given** a pull request to `main` whose body does not contain a line matching `Spec: docs/specs/`
**When** the CI workflow runs
**Then** the `spec-ref` status check fails and the PR cannot be merged.

---

## AC-002: Spec reference check blocks PRs whose spec has no approved acceptance document

**Given** a pull request to `main` whose body contains a valid `Spec: docs/specs/<path>` line
**And** the referenced spec directory has no `05_acceptance.md`, or the file exists but its `Status:` is not `approved`
**When** the CI workflow runs
**Then** the `spec-ref` status check fails with a message indicating the acceptance document is missing or not approved.

---

## AC-002b: Spec reference check passes when the spec line is present and acceptance is approved

**Given** a pull request to `main` whose body contains a valid `Spec: docs/specs/<path>` line
**And** the referenced spec directory contains a `05_acceptance.md` with `Status: approved`
**When** the CI workflow runs
**Then** the `spec-ref` status check passes.

---

## AC-003: `no-spec` label exempts a PR from the spec reference check

**Given** a pull request to `main` without a spec line
**And** the PR has the `no-spec` label applied
**When** the CI workflow runs
**Then** the `spec-ref` status check passes.

---

## AC-004: Adding `no-spec` to a previously blocked PR re-runs the check and unblocks it

**Given** a pull request that previously failed the `spec-ref` check
**When** a maintainer adds the `no-spec` label
**Then** the workflow re-triggers automatically and the `spec-ref` check passes.

---

## AC-005: Backend coverage check blocks PRs that drop a layer below its threshold

**Given** a pull request whose changes cause any backend layer's coverage to fall below its defined minimum
**When** the CI workflow runs
**Then** the `backend-cov` status check fails, and the output names the failing layer, the actual coverage percentage, and the required minimum.

---

## AC-006: Backend coverage check passes when all layers meet their thresholds

**Given** a pull request where all backend layers meet or exceed their coverage minimums
**When** the CI workflow runs
**Then** the `backend-cov` status check passes.

---

## AC-007: Frontend coverage check blocks PRs that drop overall coverage below 75%

**Given** a pull request whose changes cause overall `web/src/app/` statement coverage to fall below 75%
**When** the CI workflow runs
**Then** the `frontend-cov` status check fails with a message stating the actual and required percentages.

---

## AC-008: Frontend coverage check blocks PRs where a service method lacks a test

**Given** a pull request that introduces or modifies a public method on a `*.service.ts` file without a corresponding unit test
**When** the CI workflow runs
**Then** the `frontend-cov` status check fails, naming the affected service file.

---

## AC-009: ADR consistency check blocks PRs with a blocking ADR violation

**Given** a pull request whose diff directly contradicts an active ADR in `docs/decisions/`
**When** the CI workflow runs
**Then** the `adr-consistency` status check fails, and a comment is posted on the PR listing the violated ADR, the relevant diff excerpt, and a concrete suggestion.

---

## AC-010: ADR consistency check posts a warning comment but does not block on ambiguous violations

**Given** a pull request with a change that is flagged as a potential concern but not a clear ADR violation
**When** the CI workflow runs
**Then** the `adr-consistency` status check passes, and a comment is posted on the PR under `### Warnings`.

---

## AC-011: ADR consistency check passes silently when there are no violations

**Given** a pull request with no ADR violations or warnings
**When** the CI workflow runs
**Then** the `adr-consistency` status check passes and no comment is posted on the PR.

---

## AC-012: PR that modifies an ADR file receives the `adr-change` label automatically

**Given** a pull request whose diff includes changes to one or more `docs/decisions/ADR-*.md` files
**When** the CI workflow runs
**Then** the `adr-change` label is applied to the PR automatically, regardless of whether the check passes or fails.

---

## AC-013: ADR consistency check uses the PR branch version of ADRs as source of truth

**Given** a pull request that both updates an ADR and includes code reflecting the updated decision
**When** the CI workflow runs
**Then** the `adr-consistency` status check passes — the updated ADR on the branch is used, not the version on `main`.

---

## AC-014: A new push to a PR branch cancels the in-flight CI run

**Given** a pull request with a CI run already in progress
**When** a new commit is pushed to the PR branch
**Then** the previous CI run is cancelled and only the new run completes.

---

## AC-015: Every check is runnable locally without triggering GitHub Actions

**Given** a developer with the required environment variables set
**When** they run `make cicd-spec-ref`, `make cicd-backend-cov`, `make cicd-frontend-cov`, or `make cicd-adr` from the repository root
**Then** each command executes the corresponding check locally and exits with the correct code.

---

## AC-016: Existing `make test` and `make test-int` commands are unmodified

**Given** the CI feature is fully implemented
**When** a developer runs `make test` or `make test-int`
**Then** both commands behave identically to before this feature was introduced — no flags, targets, or output have changed.
