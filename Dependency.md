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

---

## To be added (approved in spec)

| Name | Purpose | License | Approved in |
|------|---------|---------|-------------|
| `golang.org/x/oauth2` | OAuth2 client — redirect flow and device flow for Google and GitHub | BSD-3-Clause | `docs/specs/FEATURE_authentication/02_architecture.md` |
| `github.com/labstack/echo/v4` | HTTP API framework (ADR-001) | MIT | `docs/decisions/ADR-001_http-framework-echo.md` |
| `github.com/spf13/cobra` | CLI framework (ADR-002) | Apache-2.0 | `docs/decisions/ADR-002_cli-cobra-viper.md` |
| `github.com/jackc/pgx/v5` | PostgreSQL driver (ADR-003) | MIT | `docs/decisions/ADR-003_database-postgresql.md` |
| `github.com/testcontainers/testcontainers-go` | Integration test containers (ADR-004) | MIT | `docs/decisions/ADR-004_testcontainers.md` |
