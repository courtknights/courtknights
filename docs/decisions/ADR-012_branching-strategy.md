# ADR-012: Hierarchical Branching Strategy

- **Status:** accepted
- **Date:** 2026-03-30
- **Deciders:** aagea

---

## Context

CourtKnights follows Spec Driven Development, where every feature starts with a spec and is executed through a series of atomic tasks. The original development model used a flat branch-per-task structure where every branch merged directly to `main`. This created three problems:

1. **No grouping of related work.** All task branches from a feature lived at the same level as hotfixes and unrelated work, making it hard to track feature progress or isolate in-progress code from stable `main`.

2. **Uniform CI cost.** The full CI suite — including Testcontainers-based integration tests and a Claude API call — ran on every task PR, regardless of whether the PR was an incremental step in an ongoing feature or a final integration to `main`. This slowed feedback and burned API quota unnecessarily.

3. **No enforcement of the spec-first rule at the branch level.** Nothing prevented mixing spec documents and application code in the same branch, undermining the SDD model.

---

## Decision

Adopt a **hierarchical feature branch model** with two branch levels and a set of short-lived auxiliary branch types for work that does not require a full feature lifecycle.

### Branch types

| Type | Pattern | Branches from | Merges to | Merge strategy |
|------|---------|---------------|-----------|----------------|
| Feature | `feature/CK-XXX/branch` | `main` | `main` | Merge commit |
| Design | `feature/CK-XXX/000_design` | `feature/CK-XXX/branch` | `feature/CK-XXX/branch` | Squash |
| Task | `feature/CK-XXX/NNN_description` | `feature/CK-XXX/branch` | `feature/CK-XXX/branch` | Squash |
| Hotfix | `fix/CK-XXX_description` | `main` | `main` | Squash |
| Chore | `chore/CK-XXX_description` | `main` | `main` | Squash |

Where `NNN` matches the `TASK-NNN` number from `03_tasks.md` (zero-padded to three digits) and `description` is a short kebab-case slug.

### Feature lifecycle

```
main
 └─ feature/CK-XXX/branch          ← long-lived, no direct commits
     ├─ feature/CK-XXX/000_design  ← spec docs only, squash → feature branch
     ├─ feature/CK-XXX/001_foo     ← TASK-001, squash → feature branch
     ├─ feature/CK-XXX/002_bar     ← TASK-002, squash → feature branch
     └─ ...
          └─ (all tasks merged) → feature/CK-XXX/branch → merge commit → main
```

### Design branch content rule

The design branch (`*/000_design`) may only modify files under `docs/specs/`. CI enforces this automatically: any change outside that path fails the `design-content-check` job.

### Chore branch exception

A **chore branch** is used for minor maintenance work that:
- Does not introduce new product functionality.
- Does not require a spec or design phase.
- Is not an urgent production fix (that would be a hotfix).

Typical candidates: improving test coverage, updating documentation, small refactors, dependency upgrades, CI maintenance.

Chore branches branch from `main` and squash-merge back to `main`. They run the full CI suite. A GitHub Issue is required for traceability, but no `docs/specs/FEATURE_xxx/` directory is created. The `spec-ref` CI job is skipped for chore and hotfix branches.

### Pull request naming convention

| PR direction | Title format | Example |
|--------------|--------------|---------|
| Task → `feature/**/branch` | GitHub issue title (verbatim) | `[CK-64] TASK-001: Create ADR-012 (branching strategy)` |
| Feature branch → `main` | `[CK-{issue}] {title}` | `[CK-64] branching strategy and contribution docs` |
| Chore → `main` | `[CK-{issue}] {title}` | `[CK-66] improve backend test coverage` |
| Hotfix → `main` | `[CK-{issue}] {title}` | `[CK-99] fix nil pointer in league handler` |

Where `{issue}` is the GitHub Issue number associated with the feature branch, chore, or hotfix — not the individual task issue number.

For task PRs the title is the verbatim GitHub issue title, so the issue and PR are trivially linked without any formatting ceremony.

### Branch protection and bypass policy

GitHub rulesets are applied to `main` and `feature/**/branch` via `infra/rulesets/` and `make apply-rulesets`. Both rulesets include an `OrganizationAdmin` bypass actor with `bypass_mode: "pull_request"`.

**Why `pull_request` and not `always`:** The bypass allows an org admin to merge a PR without satisfying the approval requirement, but it does not grant direct-push access. This means all changes — including those from org admins — must still go through a pull request and have CI checks pass. The bypass exists solely to unblock self-merge in a repository with very few contributors (currently a single-contributor project), where requiring a second reviewer would halt all progress.

**Direct push is intentionally not allowed, even for org admins.** If a genuine emergency requires bypassing CI (e.g., a critical hotfix that CI cannot validate), the correct path is a `fix/CK-XXX_description` branch with a PR — not a direct push to `main`. The hotfix branch type exists precisely for this scenario.

**If the contributor base grows** and a proper review workflow becomes feasible, remove the bypass actor entirely from both rulesets.

**Deletion protection on feature branches is intentionally omitted.** Feature branches are temporary: they are created for a single feature and deleted after the feature PR merges to `main`. Applying the `deletion` rule would block this routine cleanup and, crucially, would prevent deletion even by the org admin (since the bypass is `pull_request`-only and deletion is not a PR operation). Traceability of completed work is preserved by the merge commit on `main` and the closed GitHub Issues, not by retaining the branch ref.

**`do_not_enforce_on_create: true` on `required_status_checks` for feature branches.** This setting exempts the initial creation of a `feature/**/branch` ref from the status-check requirement. Without it, GitHub blocks `git push origin feature/CK-XXX/branch` even when the branch points to a commit that already passed CI on `main`, because the checks have not run in the context of the new ref. This is a GitHub platform limitation, not a gap in coverage: no new code is introduced when creating a branch from `main`. The setting does not affect PRs — any task or design branch PR targeting `feature/**/branch` is still required to have all three status checks pass before merging.

### Two-layer CI

PRs targeting `feature/**/branch` run a fast subset of checks (unit tests only, no Testcontainers, no ADR consistency check). PRs targeting `main` run the full suite.

| Job | PRs → `feature/**/branch` | PRs → `main` |
|-----|--------------------------|--------------|
| `design-content-check` | Only for `*/000_design` branches | — |
| `spec-ref` | ✅ | ✅ (skipped for `chore/**` and `fix/**`) |
| `backend-cov-unit` | ✅ | — |
| `backend-cov-full` | — | ✅ |
| `frontend-cov` | ✅ | ✅ |
| `adr-consistency` | — | ✅ |

---

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| Flat branch-per-task to `main` | Current state. Does not group related work; applies full CI cost to every incremental task PR; no design phase enforcement. |
| Git Flow (develop + release branches) | Introduces a permanent `develop` branch that adds merge ceremony without clear benefit for a project with a single release stream. Overkill for current team size. |
| Trunk-based development (no feature branches) | Requires feature flags to hide in-progress work; incompatible with the SDD model where a feature may span many tasks over several days. |
| GitHub merge queues | Addresses merge ordering but not the CI cost split or the spec-first enforcement. Complementary, not a replacement. |

---

## Consequences

### Positive

- Feature progress is visible as a named branch in the repository; all related task branches are grouped under it.
- Task PR CI is faster: unit tests only, no Testcontainers spin-up, no Claude API call.
- The full integration suite still runs exactly once before any feature code reaches `main`.
- The design branch content check enforces the spec-first rule at the git level, not just in documentation.
- The chore branch exception provides a lightweight path for maintenance work without the overhead of the full feature lifecycle.
- Branch protection rules are stored as code in `infra/rulesets/` and applied via `make apply-rulesets` — changes go through PR review.

### Negative

- More branch types to learn; contributors need to read `CONTRIBUTING.md` to understand the model.
- Feature branches are long-lived and must be kept in sync with `main` manually (rebase or merge) when `main` advances during feature development.
- Branch protection for `feature/**/branch` requires a wildcard pattern; GitHub's ruleset engine must support `feature/**/branch` glob syntax.

---

## References

- `docs/specs/FEATURE_branching_workflow/` — full feature spec
- `docs/specs/FEATURE_branching_workflow/02_architecture.md` — CI job matrix and design branch content check implementation
- [GitHub rulesets documentation](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets)
- `CONTRIBUTING.md` — contributor guide with branch naming examples
