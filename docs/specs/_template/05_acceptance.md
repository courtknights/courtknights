# FEATURE_xxx — Acceptance Criteria

- **Status:** draft | approved
- **Feature ID:** FEATURE_xxx
- **Author:** <!-- human who wrote this -->
- **Date:** YYYY-MM-DD

---

## Definition of Done

A feature is considered complete when **all** criteria below are met and verified by the human reviewer.

---

## Functional criteria

<!-- Each criterion must be binary: it either passes or it doesn't.
     Write from the user's perspective where possible. -->

- [ ] **AC-01:** <!-- e.g. A club admin can create a league with a name and at least 2 teams -->
- [ ] **AC-02:**
- [ ] **AC-03:**

## Non-functional criteria

- [ ] All new API endpoints respond in < 300 ms under normal load
- [ ] No new `golangci-lint` warnings introduced
- [ ] TypeScript strict mode passes with no errors
- [ ] Test coverage for new code meets the minimum defined in `docs/testing/strategy.md`

## Regression criteria

- [ ] All existing tests in the regression suite pass
- [ ] `docs/testing/regression_map.md` updated to reflect this feature's dependencies

## Review checklist

- [ ] `01_business.md` — implementation matches the defined scope
- [ ] `02_architecture.md` — no undocumented deviations from the approved design
- [ ] `03_tasks.md` — all tasks merged and GitHub Issues closed
- [ ] `04_tests.md` — all test cases implemented and passing
- [ ] `Dependency.md` — any new dependencies registered
- [ ] ADR created for any new architectural decision introduced during implementation
