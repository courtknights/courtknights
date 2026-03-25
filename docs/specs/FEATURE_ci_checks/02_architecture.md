# FEATURE_ci_checks — Architecture

- **Status:** approved
- **Feature ID:** FEATURE_ci_checks
- **Author:** AI — reviewed by aagea
- **Date:** 2026-03-24
- **GitHub Issue:** [#3](https://github.com/courtknights/courtknights/issues/3)

---

## Overview

This feature adds a GitHub Actions CI pipeline that runs four automated checks on every pull request targeting `main`. The checks are independent and run in parallel as separate jobs within a single workflow file.

```
PR opened / updated
        │
        ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  ci.yml  (GitHub Actions workflow)                                        │
│                                                                           │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐  ┌──────────────────┐ │
│  │ spec-ref    │  │ backend-cov  │  │ frontend-cov│  │ adr-consistency  │ │
│  │ (bash)      │  │ (bash + Go)  │  │ (Node/bash) │  │ (Python + Claude)│ │
│  └─────────────┘  └──────────────┘  └─────────────┘  └──────────────────┘ │
└───────────────────────────────────────────────────────────────────────────┘
```

All four jobs must pass for the PR to be mergeable. Each job posts its own GitHub status check.

---

## File layout

```
.github/
  workflows/
    ci.yml                      # Workflow definition — thin orchestrator, delegates to make

infra/
  cicd/
    Makefile                    # cicd workspace Makefile — one target per check
    check_spec_ref.sh           # Job 1 — spec reference check
    check_backend_coverage.sh   # Job 2 — backend coverage per layer
    check_frontend_coverage.sh  # Job 3 — frontend coverage
    check_adr_consistency.py    # Job 4 — ADR consistency via Claude API

Makefile                        # Root dispatcher — adds cicd-* targets
```

The `ci.yml` workflow calls `make cicd-<check>` for each job. This keeps the workflow thin and makes every check runnable locally without triggering GitHub Actions.

The root `Makefile` adds dispatcher targets following the same pattern as `db-*` and `web-*`:

```makefile
cicd-spec-ref:
	$(MAKE) -C infra/cicd check-spec-ref

cicd-backend-cov:
	$(MAKE) -C infra/cicd check-backend-cov

cicd-frontend-cov:
	$(MAKE) -C infra/cicd check-frontend-cov

cicd-adr:
	$(MAKE) -C infra/cicd check-adr
```

The `infra/cicd/Makefile` owns the implementation details (environment variables, script paths, tool invocations). The workflow passes required inputs (PR number, tokens) as environment variables.

---

## Job 1 — Spec reference check

### Trigger exemption

If the PR carries the `no-spec` label, the job exits with success immediately without inspecting the PR body.

### Logic

```
1. Fetch PR body from GitHub Actions event payload (${{ github.event.pull_request.body }})
2. Check whether the body matches the pattern: Spec: docs/specs/
3. If no match → exit 1 (blocks PR)
4. If match    → exit 0
```

### Implementation

A bash script (`check_spec_ref.sh`) receives the PR body as an environment variable and the label list as a second variable. No external dependencies.

---

## Job 2 — Backend coverage check

### Strategy

Go's `go test -coverprofile` produces a single coverage file listing each source line. Coverage is computed **per layer** by filtering the coverage file to only the packages belonging to each layer, then running `go tool cover -func` on the filtered output.

### Layer → package mapping

| Layer | Package prefix | Minimum |
|-------|---------------|---------|
| Domain | `courtknights/api/internal/domain/` | ≥ 90% |
| Application | `courtknights/api/internal/application/` | ≥ 80% |
| Infrastructure | `courtknights/api/internal/infrastructure/` | ≥ 70% |
| API (handlers) | `courtknights/api/internal/api/` | ≥ 80% |

### Two-phase coverage run

- **Phase 1 (unit):** `go test -coverprofile=coverage_unit.out -covermode=atomic ./...` from `api/` — integration files excluded via build tag `//go:build integration`.
- **Phase 2 (integration):** `go test -tags=integration -coverprofile=coverage_int.out -covermode=atomic ./...` from `api/` — Testcontainers spins up PostgreSQL on demand (ADR-004).

Coverage for each layer is computed from the appropriate phase:

| Layer | Phase used |
|-------|-----------|
| Domain | unit |
| Application | unit |
| Infrastructure | integration |
| API (handlers) | unit + integration (merged) |

Merging unit + integration profiles for the API layer is done with `go tool covdata` or by concatenating profiles with deduplication.

### CI requirements

- Docker must be available on the runner (required by Testcontainers — ADR-004).
- Use `ubuntu-latest` runner with Docker pre-installed.

### Implementation

A bash script (`check_backend_coverage.sh`) that:
1. Runs both test phases.
2. Filters each coverage file by layer prefix using `grep`.
3. Pipes filtered output to `go tool cover -func` and extracts the `total:` line.
4. Compares each total against its threshold; exits 1 on first failure with a clear message indicating which layer failed and by how much.

---

## Job 3 — Frontend coverage check

### Strategy

Angular tests (Jest) are configured to emit an Istanbul JSON coverage report at `web/coverage/coverage-summary.json`. The job parses this report and enforces two thresholds:

1. **Global statement coverage** for `web/src/app/`: ≥ 75%.
2. **Per-service method coverage**: every file matching `web/src/app/**/*.service.ts` must have 100% function coverage.

### Implementation

A bash script (`check_frontend_coverage.sh`) that:
1. Runs `cd web && npm test -- --coverage --watchAll=false`.
2. Parses `web/coverage/coverage-summary.json` with `jq`.
3. Checks global statement percentage against 75%.
4. Iterates over service files and checks their function coverage for 100%.
5. Exits 1 with a list of failing files/thresholds.

### Assumption

`web/package.json` must have Jest configured to emit `coverage-summary.json` (Istanbul default). If not, a `coverageReporters: ["json-summary"]` entry must be added — this is a prerequisite, not part of this feature's scope.

---

## Job 4 — ADR consistency check

### Overview

A Python script calls the Claude API (claude-sonnet-4-6) with the PR diff and all ADR files from the **PR branch** as context. Claude returns a structured assessment. The job posts a GitHub comment and/or applies the `adr-change` label based on the response.

### Script inputs

| Input | Source |
|-------|--------|
| PR diff | `gh pr diff $PR_NUMBER` |
| ADR files | All `docs/decisions/ADR-*.md` files read from the checked-out branch |
| PR number | `${{ github.event.pull_request.number }}` |
| GitHub token | `${{ secrets.GITHUB_TOKEN }}` |
| Anthropic API key | `${{ secrets.ANTHROPIC_API_KEY }}` |

### Claude prompt structure

```
system:
  You are an architecture consistency reviewer for the CourtKnights project.
  Your task is to detect whether the provided PR diff contradicts any of the
  active Architecture Decision Records (ADRs).

  Classify each finding as:
  - BLOCKING: a direct, unambiguous violation of a decision stated in an ADR.
  - WARNING:  an area of concern that may need human attention but is not a
              clear-cut violation.

  Respond ONLY with a JSON object matching this schema:
  {
    "violations": [
      {
        "severity": "BLOCKING" | "WARNING",
        "adr": "ADR-NNN",
        "summary": "<one-line description>",
        "diff_excerpt": "<relevant lines from the diff>",
        "suggestion": "<concrete action to resolve the conflict>"
      }
    ],
    "adr_files_modified": ["ADR-NNN", ...]
  }

  If there are no violations, return { "violations": [], "adr_files_modified": [...] }.

user:
  ## Active ADRs
  <contents of each docs/decisions/ADR-*.md file>

  ## PR diff
  <output of gh pr diff>
```

### Decision logic

```
parse JSON response
│
├── adr_files_modified is non-empty
│     └── apply label `adr-change` via GitHub API
│
├── violations contains BLOCKING entries
│     ├── post GitHub comment (see comment format below)
│     └── exit 1  ← blocks PR
│
├── violations contains WARNING entries only
│     ├── post GitHub comment (warnings section only)
│     └── exit 0  ← does not block
│
└── violations is empty
      └── exit 0  (no comment posted)
```

### GitHub comment format

```markdown
## ADR Consistency Check

### Blocking violations

**ADR-001 — HTTP Framework (Echo)**
> `handler.go:42` uses `net/http` directly instead of Echo's context.
>
> **Diff excerpt:**
> ```go
> - w http.ResponseWriter, r *http.Request
> ```
> **Suggestion:** Replace with `echo.Context` and register the handler via `e.GET(...)`.

---

### Warnings

**ADR-008 — Layered Architecture**
> Manager constructor returns a concrete struct instead of the interface.
> This may be intentional for an internal helper, but verify it does not
> leak to the Handler layer.
>
> **Suggestion:** Review whether `NewXxxManager` should return `XxxManager` interface.
```

### Python dependencies

| Package | Purpose |
|---------|---------|
| `anthropic` | Claude API client |
| `PyGithub` | Post PR comments and apply labels |

These are installed inline in the CI step (`pip install anthropic PyGithub`) and are not added to any application dependency file.

---

## Secrets and permissions required

| Secret / permission | Used by | Notes |
|--------------------|---------|-------|
| `GITHUB_TOKEN` | Jobs 1, 4 | Automatically available in GitHub Actions. Needs `pull-requests: write` to post comments and apply labels. |
| `ANTHROPIC_API_KEY` | Job 4 | Must be added as a repository secret by the maintainer. |

The workflow must declare:

```yaml
permissions:
  pull-requests: write
  contents: read
```

---

## Workflow trigger and concurrency

```yaml
on:
  pull_request:
    branches: [main]
    types: [opened, synchronize, reopened, labeled, unlabeled]

concurrency:
  group: ci-${{ github.event.pull_request.number }}
  cancel-in-progress: true
```

`labeled` / `unlabeled` events are included so that adding `no-spec` to an existing PR re-runs the spec reference job and clears the block.

`cancel-in-progress: true` ensures that pushing a new commit cancels the in-flight run for the same PR, avoiding redundant queue build-up.

---

## ADR-011 — AI-assisted ADR consistency check in CI

The decision to use the Claude API for automated architecture review inside CI is captured in [ADR-011](../../decisions/ADR-011_ai-adr-consistency-check.md).

Key decisions recorded there:
- Model: `claude-sonnet-4-6`
- Script location: `infra/cicd/check_adr_consistency.py` (not application code)
- Structured JSON response contract (schema validated before acting)
- Severity model: `BLOCKING` (fails CI) vs `WARNING` (noted, does not block)
- `adr-change` label applied automatically when ADR files are modified
- ADR source of truth: PR branch, not `main`

---

## Constraints and non-decisions

- No new application code is added to `api/` or `web/`.
- Existing `Makefile` targets are not modified — only new `cicd-*` dispatcher targets are added to the root `Makefile`.
- Scripts under `infra/cicd/` are not subject to Go or Angular conventions — they are CI tooling.
- Scripts under `infra/cicd/` are not covered by the project's test suite. The CI jobs themselves act as their functional validation.
- Every check in `infra/cicd/Makefile` must be runnable locally by passing the required environment variables manually (e.g. `PR_BODY="..." make -C infra/cicd check-spec-ref`).
