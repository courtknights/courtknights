# courtknights-api

HTTP API server for the CourtKnights platform.

## Usage

```
courtknights-api [flags]
```

Every flag can also be set via its corresponding environment variable.
Environment variables take precedence over flag defaults; explicit CLI flags
take precedence over environment variables.

## Configuration reference

### Server

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--port` | `COURTKNIGHTS_SERVER_PORT` | `8080` | HTTP listen port |

### Database

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--db-url` | `COURTKNIGHTS_DATABASE_URL` | `postgres://courtknights:courtknights@localhost:5432/courtknights` | PostgreSQL connection URL |

### JWT

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--jwt-secret` | `COURTKNIGHTS_JWT_SECRET` | `dev-secret-change-in-production` | JWT signing secret |
| `--jwt-expiry` | `COURTKNIGHTS_JWT_EXPIRY` | `1h` | JWT token lifetime (Go duration, e.g. `30m`, `2h`) |

> **Warning:** the default JWT secret is only suitable for local development.
> Always set a strong secret in staging and production.

### Google OAuth2

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--google-client-id` | `COURTKNIGHTS_GOOGLE_CLIENT_ID` | _(empty)_ | Google OAuth2 client ID |
| `--google-client-secret` | `COURTKNIGHTS_GOOGLE_CLIENT_SECRET` | _(empty)_ | Google OAuth2 client secret |
| `--google-redirect-url` | `COURTKNIGHTS_GOOGLE_REDIRECT_URL` | _(empty)_ | Google OAuth2 redirect URL |

When `--google-client-id` is empty the Google provider is disabled.

### GitHub OAuth2

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--github-client-id` | `COURTKNIGHTS_GITHUB_CLIENT_ID` | _(empty)_ | GitHub OAuth2 client ID |
| `--github-client-secret` | `COURTKNIGHTS_GITHUB_CLIENT_SECRET` | _(empty)_ | GitHub OAuth2 client secret |
| `--github-redirect-url` | `COURTKNIGHTS_GITHUB_REDIRECT_URL` | _(empty)_ | GitHub OAuth2 redirect URL |

When `--github-client-id` is empty the GitHub provider is disabled.

### Bootstrap admin

Creates a single admin user on first startup. Skipped automatically when any
field is empty or when users already exist in the database.

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--bootstrap-email` | `COURTKNIGHTS_BOOTSTRAP_EMAIL` | _(empty)_ | Admin email address |
| `--bootstrap-name` | `COURTKNIGHTS_BOOTSTRAP_NAME` | _(empty)_ | Admin display name |
| `--bootstrap-pat` | `COURTKNIGHTS_BOOTSTRAP_PAT` | _(empty)_ | Plain-text Personal Access Token for the admin |

---

## Local development

The project ships a `docker-compose.dev.yml` at the repository root that starts
everything needed to run the server locally without real cloud credentials.

### What it starts

| Service | Image | Port | Purpose |
|---------|-------|------|---------|
| `postgres` | `postgres:16-alpine` | `5432` | Primary database |
| `oauth-mock` | `ghcr.io/navikt/mock-oauth2-server:2.1.10` | `8090` | OAuth2 mock for Google and GitHub flows |

### Step 1 — start the stack

```sh
docker compose -f docker-compose.dev.yml up -d
```

Wait for Postgres to be healthy (the compose healthcheck handles this):

```sh
docker compose -f docker-compose.dev.yml ps
```

### Step 2 — apply migrations

```sh
# example using golang-migrate:
migrate -path db/migrations -database "postgres://courtknights:courtknights@localhost:5432/courtknights?sslmode=disable" up
```

### Step 3 — run the server

All defaults point at the Docker stack, so a bare invocation is enough:

```sh
make build
./build/courtknights-api
```

To enable the OAuth mock providers, point the redirect URLs at the mock server.
The mock exposes one issuer per path segment (`/google`, `/github`):

```sh
./build/courtknights-api \
  --google-client-id     mock-client \
  --google-client-secret mock-secret \
  --google-redirect-url  http://localhost:8080/auth/google/callback \
  --github-client-id     mock-client \
  --github-client-secret mock-secret \
  --github-redirect-url  http://localhost:8080/auth/github/callback
```

The mock OAuth2 server authorization endpoint (interactive login UI):

```
http://localhost:8090/google/authorize
http://localhost:8090/github/authorize
```

Token and JWKS endpoints (for reference):

```
http://localhost:8090/google/token
http://localhost:8090/google/jwks
http://localhost:8090/github/token
http://localhost:8090/github/jwks
```

### Step 4 — bootstrap an admin user (optional)

```sh
./build/courtknights-api \
  --bootstrap-email admin@example.com \
  --bootstrap-name  "Admin" \
  --bootstrap-pat   my-dev-pat
```

Use the PAT value directly in the `Authorization: Bearer <pat>` header for
authenticated requests during development.

### Tear down

```sh
docker compose -f docker-compose.dev.yml down -v   # -v removes the data volume
```

---

## Build

```sh
make build   # produces build/courtknights-api
make run     # build + run with default settings
```
