# FEATURE_ci_checks — Business Context

- **Status:** draft
- **Feature ID:** FEATURE_ci_checks
- **Author:** aagea
- **Date:** 2026-03-22
- **GitHub Issue:** [#3](https://github.com/courtknights/courtknights/issues/3)

---

## Problem statement

CourtKnights follows Spec Driven Development (SDD), which requires every pull request to reference a spec document and maintain minimum test coverage thresholds per layer. These rules are documented in `CLAUDE.md` and `docs/testing/strategy.md` but are not currently enforced automatically.

Without automated enforcement, the rules rely entirely on human discipline and code review attention. As the project grows and more contributors join, it becomes increasingly likely that PRs will be merged without a spec reference, with coverage regressions, or with code that silently contradicts an architectural decision — eroding the quality foundations the SDD model is designed to protect.

## Users affected

This feature affects **project contributors and maintainers** — primarily the architect/human reviewer and AI agents that open PRs. It is an internal developer tooling feature with no end-user-facing impact.

## Business value

- **Enforces SDD consistency:** Every merged PR is guaranteed to trace back to an approved spec, making the codebase auditable and decision rationale always recoverable.
- **Prevents coverage regressions:** Automatically blocks merges that drop coverage below the established minimums, ensuring quality standards are maintained as the codebase grows.
- **Protects architectural integrity:** An AI-assisted check catches code that contradicts active ADRs before it reaches `main`, including when ADRs evolve within the same branch.
- **Reduces review burden:** Reviewers no longer need to manually check for spec references, run coverage reports, or cross-reference ADRs — the CI system handles it.
- **Without this feature:** SDD rules remain aspirational. A single missed check can start a pattern of skipped specs, untested code, or silent ADR violations that compound over time.

## Scope

### In scope

- GitHub Actions workflow that runs on every pull request to `main`.
- **Spec reference check:** Block merge if the PR body does not contain a `Spec: docs/specs/...` line. Additionally, the referenced spec must have an approved acceptance document (`05_acceptance.md` with `Status: approved`) — a spec without signed-off acceptance criteria is treated as incomplete. PRs labelled `no-spec` are exempt (for hotfixes and non-feature work).
- **Backend test coverage check:** Fail the build if coverage for any of the following layers drops below its defined minimum:

  | Layer | Minimum |
  |-------|---------|
  | `api/internal/domain/` | ≥ 90% (unit) |
  | `api/internal/application/` | ≥ 80% (unit) |
  | `api/internal/infrastructure/` | ≥ 70% (integration) |
  | `api/internal/api/` | ≥ 80% (unit + integration) |

- **Frontend test coverage check:** Fail the build if overall `web/src/app/` coverage drops below 75%, and if any public service method lacks at least one unit test (100% method coverage on services).
- **ADR consistency check (AI-assisted):** On every PR, call the Claude API to compare the PR diff against all ADRs present in the PR branch (`docs/decisions/`). The check:
  - Uses the branch version of the ADRs as the source of truth — if the PR updates an ADR and the code reflects it, the check passes.
  - Classifies discrepancies as `blocking` (direct violation) or `warning` (ambiguous area requiring human judgement). Only `blocking` discrepancies fail the check.
  - On failure, posts a GitHub comment on the PR with the conflicting ADR, the relevant code excerpt, and a concrete suggestion for resolving the conflict.
  - Automatically applies the `adr-change` label to any PR that modifies one or more ADR files, regardless of whether the check passes or fails.
- The existing `make test` and `make test-int` commands must be usable from CI without modification.

### Out of scope

- Running acceptance / E2E tests in CI (those live in a separate repo per `docs/testing/strategy.md`).
- Coverage trend reporting or dashboards.
- Notifications (Slack, email) on failure — GitHub's native PR status checks are sufficient.
- Any change to existing test commands or coverage tooling.
- Support for CI platforms other than GitHub Actions.
- Summarising ADRs in PR comments when no violation is detected (informational ADR summaries are out of scope).

## Assumptions and dependencies

- The repository is hosted on GitHub and GitHub Actions is the chosen CI platform.
- `make test` and `make test-int` already produce coverage output compatible with `go tool cover`.
- Integration tests require a running PostgreSQL instance; the CI workflow relies on Testcontainers to spin it up on demand (see [ADR-004](../../decisions/ADR-004_testcontainers.md)).
- The frontend (`cd web && npm test`) is already configured to emit coverage reports (Istanbul / NYC).
- PR authors (human and AI) have the ability to add the `no-spec` label when appropriate.
- A Claude API key is available as a GitHub Actions secret for the ADR consistency check.
- The GitHub Actions bot has permission to post comments and apply labels on PRs.

## Glossary

| Term | Definition |
|------|-----------|
| CI | Continuous Integration — automated pipeline that runs checks on every pull request. |
| Spec reference | A line in the PR body matching `Spec: docs/specs/<path>`, linking the PR to an approved feature specification. |
| `no-spec` label | A GitHub PR label that exempts a PR from the spec reference check (reserved for hotfixes and non-feature work). |
| `adr-change` label | A GitHub PR label applied automatically when a PR modifies one or more ADR files, signalling an architectural decision change. |
| Coverage minimum | The lowest acceptable line/statement coverage percentage for a given code layer, as defined in `docs/testing/strategy.md`. |
| ADR consistency check | An AI-assisted CI step that uses the Claude API to detect code changes that contradict active ADRs in the PR branch. |
| Blocking discrepancy | An ADR violation severe enough to fail the CI check and block the PR merge. |
| Warning discrepancy | An ambiguous area flagged by the ADR check that does not block merge but is noted in the PR comment for human review. |
