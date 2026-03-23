# ADR-009: Go Workspace with `go.work`

- **Status:** accepted
- **Date:** 2026-03-23
- **Deciders:** aagea

---

## Context

The `FEATURE_repo_workspaces` restructuring moves the Go source tree into an `api/`
subdirectory with its own `go.mod`. Without additional configuration, `go` tooling
and editors operating from the repository root would not find the module, requiring
developers to `cd api/` before running any Go command.

Go 1.18 introduced native workspace support via `go.work`. The project runs Go 1.25,
which fully supports this feature.

## Decision

Add a `go.work` file at the repository root that references the `api/` module:

```
go 1.25

use ./api
```

The file is **committed** to the repository. `go.work.sum` is committed alongside it.

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| Always `cd api/` to run Go commands | Breaks editor tooling configured at the repo root; requires developers and CI to know about the subdirectory convention |
| Symlinks (`go.mod` → `api/go.mod`) | Fragile, not portable across operating systems, not supported by Go tooling |
| Single `go.mod` at root with `api/` as a subdirectory | Contradicts the workspace goal — the `api/` directory would not be an independent module; toolchain isolation is lost |
| Gitignore `go.work` (local-only) | CI and other contributors would need to recreate it manually; inconsistent environments |

## Consequences

### Positive
- `go build`, `go test`, `go run`, and editor language servers work from the repository
  root without any extra configuration.
- The workspace is extensible: adding a second Go module (e.g. a future CLI tool or
  migration runner) requires only a new `use` line in `go.work`.
- Standard Go tooling — no third-party dependency.

### Negative
- `go.work.sum` must be kept in sync; developers must run `go work sync` after adding
  a new Go module to the workspace.
- `GOWORK=off` must be set in any context that needs to resolve the module in isolation
  (e.g. a CI job that publishes only `api/` independently).

## References

- https://go.dev/ref/mod#workspaces
- `docs/specs/FEATURE_repo_workspaces/02_architecture.md`
