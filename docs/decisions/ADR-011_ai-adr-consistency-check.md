# ADR-011: AI-assisted ADR Consistency Check in CI

- **Status:** accepted
- **Date:** 2026-03-24
- **Deciders:** aagea

---

## Context

CourtKnights follows Spec Driven Development, where Architecture Decision Records (ADRs) capture the binding rules of the codebase. Manually verifying that every PR respects all active ADRs is error-prone and places a high burden on the human reviewer.

ADRs can also evolve within a feature branch: a PR may update an ADR and change the code accordingly. Any consistency check must use the ADRs from the PR branch as the source of truth, not from `main`.

A purely rule-based static check cannot cover the full semantic surface of ADRs — they are written in natural language and often involve architectural judgement (e.g. "the Handler must never depend on a concrete Manager"). Large language models are well-suited to this kind of structured natural-language reasoning over a bounded context.

---

## Decision

Use the **Claude API** (`claude-sonnet-4-6`) as part of the CI pipeline to check whether the PR diff contradicts any active ADR in the PR branch.

### Model choice

`claude-sonnet-4-6` — balances reasoning quality and latency for CI use. The ADR corpus and diff are bounded in size, so the full context fits within a single request.

### Script location

`infra/cicd/check_adr_consistency.py` — CI tooling, not application code. Invoked via `infra/cicd/Makefile` target `check-adr`, dispatched from the root `Makefile` as `make cicd-adr`.

### Response contract

Claude is prompted to return a structured JSON object only:

```json
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
```

Claude must not return free-form text — only this JSON object. The script validates the schema before acting on the response.

### Severity model

| Severity | Meaning | CI outcome |
|----------|---------|-----------|
| `BLOCKING` | Direct, unambiguous violation of a decision in an ADR | Fails the check — PR cannot be merged |
| `WARNING` | Ambiguous area that merits human attention but is not a clear-cut violation | Check passes — noted in the PR comment |

### GitHub integration

- On any violation (blocking or warning): post a comment on the PR with the conflicting ADR, the diff excerpt, and a concrete suggestion.
- If `adr_files_modified` is non-empty: apply the `adr-change` label to the PR, regardless of whether violations were found.
- On `BLOCKING` violation: exit with a non-zero status to fail the CI job.

### Source of truth

The ADR files are read from the **checked-out PR branch** (`docs/decisions/ADR-*.md`). If a PR updates an ADR and the code reflects the updated decision, the check must pass.

### Secret management

The Anthropic API key is stored as a GitHub Actions repository secret (`ANTHROPIC_API_KEY`) and injected as an environment variable at runtime. It is never written to disk or logged.

### Python dependencies

Installed inline in the CI step — not added to any application dependency file:

| Package | Purpose |
|---------|---------|
| `anthropic` | Claude API client |
| `PyGithub` | Post PR comments and apply labels via GitHub REST API |

---

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| Rule-based static analysis (AST / regex) | ADRs are written in natural language with architectural semantics that cannot be fully expressed as code patterns. Too many false negatives. |
| Manual review only | Already the current state. Does not scale; relies entirely on reviewer attention. |
| GPT-4 / other LLM | Claude is the project's established AI toolchain. Using a single provider reduces secret management overhead and keeps the dependency surface consistent. |
| Claude Opus 4.6 | Higher reasoning capability but slower and more expensive for a task that fits well within Sonnet's capabilities. Revisit if false negative rate proves unacceptable. |
| Inline script in `ci.yml` | Breaks the workspace model. Scripts must live in `infra/cicd/` and be runnable locally via `make cicd-adr`. |

---

## Consequences

### Positive

- Architectural rules are enforced automatically on every PR — no reliance on reviewer memory.
- The check adapts to ADR evolution: updating an ADR in the same branch is sufficient to unblock the PR.
- The `adr-change` label provides a visible audit trail of PRs that introduced architectural changes.
- The concrete suggestion in the comment turns a blocking failure into actionable guidance.

### Negative

- Requires an `ANTHROPIC_API_KEY` secret in the repository — one more credential to manage.
- LLM output is non-deterministic: the same diff may produce slightly different comments across runs. The JSON schema contract reduces but does not eliminate this variability.
- False positives (blocking a valid PR) are possible. Maintainers must be able to override by updating the relevant ADR in the same branch.
- Adds latency to the CI pipeline (one API call per PR push). Expected p50 response time: 5–15 seconds.

---

## References

- `docs/specs/FEATURE_ci_checks/01_business.md` — feature business context
- `docs/specs/FEATURE_ci_checks/02_architecture.md` — implementation design
- `docs/decisions/` — ADR corpus used as input to the check
- [Anthropic API docs](https://docs.anthropic.com)
