# FEATURE_branching_workflow — Business Context

- **Status:** approved
- **Feature ID:** FEATURE_branching_workflow
- **GitHub Issue:** [#64](https://github.com/courtknights/courtknights/issues/64)
- **Author:** aagea
- **Date:** 2026-03-30

---

## Problem statement

The current development model uses a flat branch-per-task structure where every branch merges directly to `main`. This works for a single contributor but creates friction as the project grows:

- There is no natural grouping of the multiple task branches that belong to the same feature, making it hard to track progress and isolate in-progress work from stable code.
- The CI pipeline runs the same checks regardless of context: heavyweight integration tests (Testcontainers) and the Claude API call run on every task iteration, slowing feedback and burning API quota unnecessarily.
- There is no written guide for new contributors — the project structure, branching model, and contribution process exist only in `CLAUDE.md`, which is scoped to the AI agent.
- The design phase (spec writing) is not enforced at the branch level: nothing prevents mixing specification work and application code in the same branch.

---

## Users affected

- **Human developer (architect/validator):** needs a clear model to create feature branches, review task PRs, and merge features to `main`.
- **AI agent (executor):** needs unambiguous naming conventions and CI rules to open the right PRs against the right target branches.
- **External contributors:** need a written guide (CONTRIBUTING.md) and a project overview (README.md) to understand how to participate.

---

## Business value

- **Faster task iteration:** removing expensive CI jobs from task PRs reduces wait times during active development.
- **Cleaner repository history:** squash-per-task on feature branches; merge commit per feature on `main` makes the log readable at two levels of granularity.
- **Open-source readiness:** CONTRIBUTING.md and README.md are the minimum required for a public repository to be approachable by external contributors.
- **Enforced design discipline:** the 000-design branch convention makes the spec-first rule visible in the git graph, not just in documentation.

---

## Scope

### In scope

- Branch naming convention for feature, design, task, hotfix, and chore branches.
- Merge strategy per branch direction (squash vs merge commit).
- Two-layer CI: reduced check set for PRs targeting `feature/**`, full check set for PRs targeting `main`.
- CI job that enforces the 000-design branch content rule (only `docs/specs/` changes allowed).
- `ADR-012` documenting the branching strategy decision.
- Update `CLAUDE.md` with the new Git Conventions section.
- `CONTRIBUTING.md` at the repository root.
- `README.md` at the repository root.
- GitHub ruleset definitions stored in `infra/rulesets/` and applied via `make apply-rulesets` in the `infra/` workspace.

### Out of scope

- Automated branch creation scripts or GitHub CLI wrappers.
- Semantic versioning or release automation.
- Changelog generation.

---

## Assumptions and dependencies

- The CI infrastructure (`infra/cicd/`, `.github/workflows/ci.yml`) introduced in `FEATURE_ci_checks` is the base on which the two-layer model is built.
- GitHub Actions `pull_request` trigger supports filtering by base branch pattern (`feature/**`).
- `gh` CLI is available in the environment where `make apply-rulesets` is run, authenticated with a token that has `administration: write` on the repository.

---

## Glossary additions

| Term | Definition |
|------|------------|
| Feature branch | Long-lived branch (`feature/CK-XXX/branch`) that groups all task branches for one feature; branches from `main` and merges back to `main`. |
| Design branch | First task branch (`feature/CK-XXX/000_design`) that only contains spec documents under `docs/specs/`; merges into the feature branch. |
| Task branch | Short-lived branch (`feature/CK-XXX/NNN_description`) for a single atomic task; branches from the feature branch and merges back via squash. |
| Hotfix branch | Short-lived branch (`fix/CK-XXX_description`) for urgent production fixes; branches from `main` and merges directly back to `main`. |
| Chore branch | Short-lived branch (`chore/CK-XXX_description`) for minor maintenance work (e.g. improving test coverage, updating docs, small refactors) that requires no spec or design phase and is not a bug fix; branches from `main` and merges directly back to `main` via squash. |
| Two-layer CI | CI model where PRs targeting `feature/**` run a fast subset of checks (unit only, no ADR), and PRs targeting `main` run the full suite. |
