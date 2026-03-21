# courtknights-api

HTTP API server for the CourtKnights platform.

## Usage

```
courtknights-api [flags]
```

Every flag can also be set via its corresponding environment variable.
Environment variables take precedence over flag defaults but flags passed
explicitly on the command line take precedence over environment variables.

## Configuration reference

### Server

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--port` | `COURTKNIGHTS_SERVER_PORT` | `8080` | HTTP listen port |

### Database

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--db-url` | `COURTKNIGHTS_DATABASE_URL` | _(required)_ | PostgreSQL connection URL |

### JWT

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--jwt-secret` | `COURTKNIGHTS_JWT_SECRET` | _(required)_ | JWT signing secret |
| `--jwt-expiry` | `COURTKNIGHTS_JWT_EXPIRY` | `1h` | JWT token lifetime (Go duration, e.g. `30m`, `2h`) |

### Google OAuth2

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--google-client-id` | `COURTKNIGHTS_GOOGLE_CLIENT_ID` | _(optional)_ | Google OAuth2 client ID |
| `--google-client-secret` | `COURTKNIGHTS_GOOGLE_CLIENT_SECRET` | _(optional)_ | Google OAuth2 client secret |
| `--google-redirect-url` | `COURTKNIGHTS_GOOGLE_REDIRECT_URL` | _(optional)_ | Google OAuth2 redirect URL |

When `--google-client-id` is empty the Google provider is disabled.

### GitHub OAuth2

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--github-client-id` | `COURTKNIGHTS_GITHUB_CLIENT_ID` | _(optional)_ | GitHub OAuth2 client ID |
| `--github-client-secret` | `COURTKNIGHTS_GITHUB_CLIENT_SECRET` | _(optional)_ | GitHub OAuth2 client secret |
| `--github-redirect-url` | `COURTKNIGHTS_GITHUB_REDIRECT_URL` | _(optional)_ | GitHub OAuth2 redirect URL |

When `--github-client-id` is empty the GitHub provider is disabled.

### Bootstrap admin

The bootstrap flow creates a single admin user on first startup.
It is **skipped automatically** when any of the three fields is empty or when
users already exist in the database.

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--bootstrap-email` | `COURTKNIGHTS_BOOTSTRAP_EMAIL` | _(optional)_ | Admin email address |
| `--bootstrap-name` | `COURTKNIGHTS_BOOTSTRAP_NAME` | _(optional)_ | Admin display name |
| `--bootstrap-pat` | `COURTKNIGHTS_BOOTSTRAP_PAT` | _(optional)_ | Plain-text Personal Access Token for the admin |

## Example

```sh
export COURTKNIGHTS_DATABASE_URL="postgres://user:pass@localhost:5432/courtknights"
export COURTKNIGHTS_JWT_SECRET="supersecret"
export COURTKNIGHTS_GOOGLE_CLIENT_ID="..."
export COURTKNIGHTS_GOOGLE_CLIENT_SECRET="..."
export COURTKNIGHTS_GOOGLE_REDIRECT_URL="http://localhost:8080/auth/google/callback"

courtknights-api --port 9090
```

## Build

```sh
make build          # produces build/courtknights-api
make run            # build + run with default settings
```
