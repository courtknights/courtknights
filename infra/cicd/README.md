# infra/cicd

CI check scripts for the CourtKnights repository.

Each check is a standalone script invoked by its Makefile target. All checks
are runnable locally without triggering GitHub Actions by setting the required
environment variables and calling `make -C infra/cicd <target>` from the
repository root, or `make cicd-<target>` from the root dispatcher.

---

## Targets and required environment variables

### `check-spec-ref`

Verifies that a PR body contains a `Spec: docs/specs/` reference and that the
referenced spec has an approved `05_acceptance.md`.

| Variable | Description |
|----------|-------------|
| `PR_BODY` | Full text of the PR description |
| `PR_LABELS` | Comma-separated list of PR labels |

```bash
PR_BODY="..." PR_LABELS="" make cicd-spec-ref
```

---

### `check-backend-cov`

Runs unit and integration tests and checks per-layer coverage thresholds.

No additional environment variables required beyond a working Go environment
and Docker (for Testcontainers).

```bash
make cicd-backend-cov
```

---

### `check-frontend-cov`

Runs the Angular test suite with coverage and checks global and per-service
thresholds.

No additional environment variables required beyond Node.js and installed
`web/` dependencies.

```bash
make cicd-frontend-cov
```

---

### `check-adr`

Calls the Claude API to check whether the PR diff contradicts any active ADR.
Posts a GitHub comment and applies labels as needed.

| Variable | Description |
|----------|-------------|
| `PR_NUMBER` | GitHub PR number |
| `GITHUB_TOKEN` | GitHub token with `pull-requests: write` |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GITHUB_REPOSITORY` | Repository in `owner/repo` format |

```bash
PR_NUMBER=42 GITHUB_TOKEN=... ANTHROPIC_API_KEY=... GITHUB_REPOSITORY=courtknights/courtknights make cicd-adr
```
