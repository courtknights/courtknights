# ADR-005 — CLI as a First-Class Interface

- **Date:** 2026-03-18
- **Status:** accepted

---

## Context

CourtKnights is an open-source, self-hosted platform. The Web UI is the primary interface for end users, but as an open-source project the Web UI layer may be replaced, customised, or run headless by operators.

There is also a need to support automation — operators and administrators need to script and automate platform operations (e.g. seeding data, managing users, triggering competition workflows) without a browser.

A CLI that mirrors the full capability of the Web UI solves both problems: it gives operators a scriptable interface and makes the platform usable without a browser.

---

## Decision

CourtKnights ships a **standalone CLI binary** that is a separate component from the backend server. The CLI is an **HTTP API client** — it communicates with the backend exclusively through the public HTTP API, exactly as the Web UI does.

The CLI is implemented using **Cobra** (already decided in ADR-002).

### Principles

- **Parity:** every operation available in the Web UI must be available in the CLI
- **External client:** the CLI calls the HTTP API — it shares no internal Go packages with the backend
- **Automation-friendly:** CLI commands are scriptable, support non-interactive mode, and return machine-readable output (JSON) when requested via a flag (e.g. `--output json`)
- **Authentication-aware:** the CLI authenticates via Personal Access Tokens (PAT) — see FEATURE_authentication

### Repository structure

```
cmd/
  server/   # Backend HTTP server entrypoint
  cli/      # Standalone CLI entrypoint — calls the HTTP API
```

---

## Consequences

- Every new feature must expose its operations both as an HTTP endpoint and as a CLI command
- The CLI has no access to internal backend packages — all interaction goes through the HTTP API
- The HTTP API must be expressive enough to support all operations needed by both the Web UI and the CLI
- The CLI authentication path (PAT) must be implemented as part of the authentication feature
- Documentation must cover both the API and the CLI for each feature

---

## Alternatives considered

| Alternative | Reason rejected |
|-------------|-----------------|
| CLI shares internal Go packages with the server | Tight coupling; prevents independent distribution of the CLI |
| No CLI — Web UI only | Eliminates automation and headless deployment options |
