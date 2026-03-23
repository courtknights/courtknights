# FEATURE_ci_checks — Business Context

- **Status:** draft
- **Feature ID:** FEATURE_ci_checks
- **Author:** aagea
- **Date:** 2026-03-22
- **GitHub Issue:** [#3](https://github.com/courtknights/courtknights/issues/3)

---

## Problem statement

CourtKnights follows Spec Driven Development (SDD), which requires every pull request to reference a spec document and maintain minimum test coverage thresholds per layer. These rules are documented in `CLAUDE.md` and `docs/testing/strategy.md` but are not currently enforced automatically.

Without automated enforcement, the rules rely entirely on human discipline and code review attention. As the project grows and more contributors join, it becomes increasingly likely that PRs will be merged without a spec reference or with coverage regressions, silently eroding the quality foundations the SDD model is designed to protect.

## Users affected

This feature affects **project contributors and maintainers** — primarily the architect/human reviewer and AI agents that open PRs. It is an internal developer tooling feature with no end-user-facing impact.

## Business value

- **Enforces SDD consistency:** Every merged PR is guaranteed to trace back to an approved spec, making the codebase auditable and decision rationale always recoverable.
- **Prevents coverage regressions:** Automatically blocks merges that drop coverage below the established minimums, ensuring quality standards are maintained as the codebase grows.
- **Reduces review burden:** Reviewers no longer need to manually check for spec references or run coverage reports — the CI system handles it.
- **Without this feature:** SDD rules remain aspirational. A single missed check can start a pattern of skipped specs or untested code that compounds over time.

## Scope

### In scope

- GitHub Actions workflow that runs on every pull request to `main`.
- **Spec reference check:** Block merge if the PR body does not contain a `Spec: docs/specs/...` line. PRs labelled `no-spec` are exempt (for hotfixes and non-feature work).
- **Backend test coverage check:** Fail the build if coverage for any of the following layers drops below its defined minimum:

  | Layer | Minimum |
  |-------|---------|
  | `api/api/internal/domain/` | ≥ 90% (unit) |
  | `api/api/internal/application/` | ≥ 80% (unit) |
  | `api/api/internal/infrastructure/` | ≥ 70% (integration) |
  | `api/api/internal/transport/http/` | ≥ 80% (unit + integration) |

- **Frontend test coverage check:** Fail the build if overall `web/src/app/` coverage drops below 75%.
- The existing `make test` and `make test-int` commands must be usable from CI without modification.

### Out of scope

- Running acceptance / E2E tests in CI (those live in a separate repo per `docs/testing/strategy.md`).
- Coverage trend reporting or dashboards.
- Notifications (Slack, email) on failure — GitHub's native PR status checks are sufficient.
- Any change to existing test commands or coverage tooling.
- Support for CI platforms other than GitHub Actions.

## Assumptions and dependencies

- The repository is hosted on GitHub and GitHub Actions is the chosen CI platform.
- `make test` and `make test-int` already produce coverage output compatible with `go tool cover`.
- Integration tests require a running PostgreSQL instance; the CI workflow must spin one up (via Docker / GitHub Actions services or Testcontainers).
- The frontend (`cd web && npm test`) is already configured to emit coverage reports (Istanbul / NYC).
- PR authors (human and AI) have the ability to add the `no-spec` label when appropriate.

## Glossary

| Term | Definition |
|------|-----------|
| CI | Continuous Integration — automated pipeline that runs checks on every pull request. |
| Spec reference | A line in the PR body matching `Spec: docs/specs/<path>`, linking the PR to an approved feature specification. |
| `no-spec` label | A GitHub PR label that exempts a PR from the spec reference check (reserved for hotfixes and non-feature work). |
| Coverage minimum | The lowest acceptable line/statement coverage percentage for a given code layer, as defined in `docs/testing/strategy.md`. |
