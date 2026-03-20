# CourtKnights — Direct Dependencies

All direct dependencies must be registered here before use.
Format: **Name** | **Purpose** | **License**

---

## Go

| Name | Purpose | License |
|------|---------|---------|
| `github.com/google/uuid` | UUID generation and parsing for entity IDs | BSD-3-Clause |
| `github.com/stretchr/testify` | Test assertions and mock framework (`assert`, `require`, `mock`) | MIT |
| `github.com/golang-jwt/jwt/v5` | JWT signing and validation (HS256) | MIT |
| `github.com/spf13/viper` | Configuration management — env vars, config files (ADR-002) | MIT |
| `golang.org/x/oauth2` | OAuth2 client — redirect flow and device flow for Google and GitHub | BSD-3-Clause |
| `github.com/labstack/echo/v4` | HTTP API framework — routing, middleware, request/response handling (ADR-001) | MIT |
| `github.com/jackc/pgx/v5` | PostgreSQL driver and connection pool (ADR-003) | MIT |
| `github.com/testcontainers/testcontainers-go` | Integration test containers — spins up real PostgreSQL (ADR-004) | MIT |

---

## To be added (approved in spec)

| Name | Purpose | License | Approved in |

| `github.com/spf13/cobra` | CLI framework (ADR-002) | Apache-2.0 | `docs/decisions/ADR-002_cli-cobra-viper.md` |
