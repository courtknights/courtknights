# ADR-002: CLI and Configuration — Cobra + Viper

- **Status:** accepted
- **Date:** 2026-03-17
- **Deciders:** CourtKnights Community

---

## Context

Every Go application in this project needs a CLI entrypoint and a unified configuration system. Configuration values must be injectable from multiple sources to support different environments (local development, CI, production) without code changes.

## Decision

Use **[Cobra](https://github.com/spf13/cobra)** for the CLI layer and **[Viper](https://github.com/spf13/viper)** for configuration management.

- Every application entrypoint (`cmd/`) exposes a Cobra root command.
- All configuration parameters support three sources, resolved in this priority order:
  1. **Environment variable** (highest priority)
  2. **CLI flag**
  3. **Default value** — set to sensible local values to enable quick testing without additional setup
- Viper binds flags, environment variables, and defaults in a single configuration layer.

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| `flag` (stdlib) | No subcommand support, no env var binding, too limited for a multi-command CLI |
| `urfave/cli` | Good library but Cobra is the de facto standard in the Go ecosystem; better tooling and docs |
| Manual env var parsing | Error-prone and does not scale; no priority resolution between sources |
| `envconfig` | Env-only; does not cover CLI flags or defaults in a unified way |

## Consequences

### Positive
- Single source of truth for all configuration regardless of origin
- Sensible local defaults allow `go run ./cmd/...` to work out of the box
- Cobra's command tree maps naturally to the project's multi-service structure
- Both libraries are widely adopted and well-maintained

### Negative
- Two additional dependencies (Cobra and Viper)
- Viper's global state must be avoided — use instance-based configuration (`viper.New()`)

## References
- https://github.com/spf13/cobra
- https://github.com/spf13/viper
- `Dependency.md` — Cobra and Viper entries
