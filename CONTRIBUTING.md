# Contributing to CourtKnights

CourtKnights follows **Spec Driven Development (SDD)**. All contributions — features, fixes, and maintenance — flow through a defined process described in this guide.

---

## Prerequisites

| Tool | Minimum version | Purpose |
|------|----------------|---------|
| Go | 1.25 | Backend development |
| Node.js | 20 | Frontend development |
| Docker | any recent | Integration tests (Testcontainers spins up PostgreSQL automatically) |
| `gh` CLI | any recent | PR and issue management |
| `make` | any | Task runner |

Clone the repository and verify your setup:

```bash
git clone git@github.com:courtknights/courtknights.git
cd courtknights

# Backend
make build
make test

# Frontend (once the web workspace is initialised)
make web-install
make web-test

# Database migrations (requires a running PostgreSQL instance)
make db-migrate
```

> Integration tests do not require a manually running PostgreSQL. Testcontainers starts and stops a container automatically during the test run — Docker must be running.

---

## Development workflow (SDD)

Every product feature follows this lifecycle before any code is written:

```
1. Human drafts docs/specs/FEATURE_xxx/01_business.md
2. AI expands the business doc — human validates
3. AI generates 02_architecture.md — human reviews; new decisions go to docs/decisions/
4. AI generates 03_tasks.md and 04_tests.md
5. Human writes 05_acceptance.md
6. Human approves the full task breakdown
7. Each task in 03_tasks.md is created as a GitHub Issue (label: agent-task)
   └─ Each task issue is registered as a sub-issue of the feature issue
8. Development proceeds one task at a time, one PR per task
```

The spec is the contract. No feature code is written without an approved spec in `docs/specs/FEATURE_xxx/`.

---

## Branching model

CourtKnights uses a hierarchical branching model (see [ADR-012](docs/decisions/ADR-012_branching-strategy.md)).

### Branch types

| Type | Pattern | Branches from | Merges to | Merge strategy |
|------|---------|---------------|-----------|----------------|
| Feature | `feature/CK-XXX/branch` | `main` | `main` | Merge commit |
| Design | `feature/CK-XXX/000_design` | `feature/CK-XXX/branch` | `feature/CK-XXX/branch` | Squash |
| Task | `feature/CK-XXX/NNN_description` | `feature/CK-XXX/branch` | `feature/CK-XXX/branch` | Squash |
| Hotfix | `fix/CK-XXX_description` | `main` | `main` | Squash |
| Chore | `chore/CK-XXX_description` | `main` | `main` | Squash |

Where `CK-XXX` is the GitHub Issue number and `NNN` is the zero-padded task number from `03_tasks.md`.

### Examples

```bash
feature/CK-64/branch          # long-lived feature branch
feature/CK-64/000_design      # design branch — spec docs only
feature/CK-64/001_adr         # TASK-001
feature/CK-64/002_ci          # TASK-002
fix/CK-99_nil-pointer-league  # hotfix
chore/CK-66_upgrade-lint      # chore
```

### Feature lifecycle

```
main
 └─ feature/CK-64/branch          ← long-lived, no direct commits
     ├─ feature/CK-64/000_design  ← spec docs only → squash merge
     ├─ feature/CK-64/001_adr     ← TASK-001 → squash merge
     ├─ feature/CK-64/002_ci      ← TASK-002 → squash merge
     └─ ... (all tasks merged)
          └─ feature/CK-64/branch → merge commit → main
```

Task issues close automatically when the feature branch merges to `main` (via `Closes #N` in the feature PR body).

### Design branch content rule

A design branch (`*/000_design`) may only modify files under `docs/specs/`. CI enforces this: any change outside that path fails the `design-content-check` job.

### Chore branch exception

Use a **chore branch** for minor maintenance that does not introduce new product functionality:

- Dependency upgrades
- Small refactors
- Documentation fixes
- CI and tooling maintenance
- Test coverage improvements

**When to use a chore vs. a feature:** if the work requires a spec and acceptance criteria, open a feature. If it is self-contained maintenance with a clear done condition, use a chore.

Chore branches branch from `main` and squash-merge back to `main`. They run the **full CI suite**. A GitHub Issue is required for traceability, but no `docs/specs/FEATURE_xxx/` directory is needed — the `spec-ref` CI job is skipped automatically.

---

## Commit messages

```
[CK-{issue-number}] {imperative mood message}
```

Examples:

```
[CK-68] add ADR-012 hierarchical branching strategy
[CK-64] implement two-layer CI workflow
[CK-66] upgrade golangci-lint to v1.58
[CK-99] fix nil pointer in league handler
```

---

## Pull request requirements

Every PR must include the following in the **body** (not the title):

```
Closes #N
Spec: docs/specs/FEATURE_xxx/02_architecture.md
```

- `Closes #N` triggers GitHub's auto-close on merge. `References #N` or `See #N` do not.
- `Spec:` is waived automatically for `chore/**` and `fix/**` branches — the CI `spec-ref` job skips those branch types.

### Approvals

| PR direction | Approvals required |
|--------------|-------------------|
| Task → `feature/**/branch` | 1 |
| Design → `feature/**/branch` | 1 |
| Feature branch → `main` | 1 |
| Chore → `main` | 1 |
| Hotfix → `main` | 1 |

---

## CI checks

### What runs and when

| Job | Task PRs (→ `feature/**/branch`) | Feature/chore/hotfix PRs (→ `main`) |
|-----|----------------------------------|-------------------------------------|
| `design-content-check` | Only for `*/000_design` branches | — |
| `spec-ref` | ✅ | ✅ (skipped for `chore/**` and `fix/**`) |
| `backend-cov-unit` | ✅ (unit tests only, no Docker) | — |
| `backend-cov-full` | — | ✅ (unit + integration) |
| `frontend-cov` | ✅ | ✅ |
| `adr-consistency` | — | ✅ |

### Running checks locally

```bash
# Spec reference check
PR_BODY="Closes #71\nSpec: docs/specs/FEATURE_branching_workflow/02_architecture.md" \
PR_LABELS="" \
make cicd-spec-ref

# Backend unit coverage only
PHASES=unit make cicd-backend-cov

# Backend full coverage (unit + integration — requires Docker)
PHASES=all make cicd-backend-cov

# Frontend coverage
make cicd-frontend-cov

# ADR consistency (requires ANTHROPIC_API_KEY and GITHUB_TOKEN)
PR_NUMBER=80 \
GITHUB_TOKEN=<token> \
ANTHROPIC_API_KEY=<key> \
GITHUB_REPOSITORY=courtknights/courtknights \
make cicd-adr
```

---

## Issues and task tracking

- Product features are tracked as GitHub Issues with their associated spec in `docs/specs/FEATURE_xxx/`.
- Task issues generated from `03_tasks.md` carry the `agent-task` label and are registered as sub-issues of the feature issue.
- To link a task issue as a sub-issue:

```bash
gh api repos/courtknights/courtknights/issues/{feature-issue-id}/sub_issues \
  --method POST \
  --field sub_issue_id=$(gh api repos/courtknights/courtknights/issues/{task-issue-number} --jq '.id')
```

---

## Admin setup

### Applying branch protection rulesets

Branch protection rules are stored in `infra/rulesets/` and applied via:

```bash
make apply-rulesets
```

This requires a token with `administration: write` permission on the repository. The default `GITHUB_TOKEN` in GitHub Actions does not have this scope — use a fine-grained PAT or a GitHub App token.

To target a different repository:

```bash
make apply-rulesets GITHUB_REPO=org/repo
```

The target is idempotent: running it a second time updates existing rulesets without creating duplicates.
