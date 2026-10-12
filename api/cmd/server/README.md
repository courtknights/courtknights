# courtknights-api

HTTP API server for the CourtKnights platform.

## Usage

```
courtknights-api [flags]
```

Every flag can also be set via its corresponding environment variable.
CLI flags take precedence over environment variables; environment variables
take precedence over flag defaults.

---

## API endpoints

### Public — no authentication required

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/auth/:provider` | Redirect to the OAuth2 provider login page (`provider`: `google` \| `github`) |
| `GET` | `/auth/:provider/callback` | OAuth2 authorization code callback |
| `POST` | `/auth/device` | Start a device authorization flow (CLI login) |
| `POST` | `/auth/device/token` | Poll for a device flow token |
| `POST` | `/auth/token/pat` | Exchange a Personal Access Token for a JWT |
| `POST` | `/auth/refresh` | Refresh an existing JWT |

### Protected — `Authorization: Bearer <jwt>` required

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/users/me` | Return the authenticated user's profile |
| `PUT` | `/api/v1/users/:id/role` | Update a user's role (admin only) |
| `POST` | `/api/v1/pats` | Create a Personal Access Token |
| `GET` | `/api/v1/pats` | List the authenticated user's Personal Access Tokens |
| `DELETE` | `/api/v1/pats/:id` | Revoke a Personal Access Token |

---

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
| `--google-redirect-url` | `COURTKNIGHTS_GOOGLE_REDIRECT_URL` | _(empty)_ | Callback URL registered in Google Cloud Console |
| `--google-auth-url` | `COURTKNIGHTS_GOOGLE_AUTH_URL` | _(Google production URL)_ | Override authorization endpoint — use for local mocks |
| `--google-token-url` | `COURTKNIGHTS_GOOGLE_TOKEN_URL` | _(Google production URL)_ | Override token endpoint — use for local mocks |
| `--google-device-auth-url` | `COURTKNIGHTS_GOOGLE_DEVICE_AUTH_URL` | _(Google production URL)_ | Override device authorization endpoint — use for local mocks |
| `--google-userinfo-url` | `COURTKNIGHTS_GOOGLE_USERINFO_URL` | _(Google production URL)_ | Override userinfo endpoint — use for local mocks |

When `--google-client-id` is empty the Google provider is disabled and returns `501`.

### GitHub OAuth2

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--github-client-id` | `COURTKNIGHTS_GITHUB_CLIENT_ID` | _(empty)_ | GitHub OAuth2 client ID |
| `--github-client-secret` | `COURTKNIGHTS_GITHUB_CLIENT_SECRET` | _(empty)_ | GitHub OAuth2 client secret |
| `--github-redirect-url` | `COURTKNIGHTS_GITHUB_REDIRECT_URL` | _(empty)_ | Callback URL registered in the GitHub OAuth App |
| `--github-auth-url` | `COURTKNIGHTS_GITHUB_AUTH_URL` | _(GitHub production URL)_ | Override authorization endpoint — use for local mocks |
| `--github-token-url` | `COURTKNIGHTS_GITHUB_TOKEN_URL` | _(GitHub production URL)_ | Override token endpoint — use for local mocks |
| `--github-device-auth-url` | `COURTKNIGHTS_GITHUB_DEVICE_AUTH_URL` | _(GitHub production URL)_ | Override device authorization endpoint — use for local mocks |
| `--github-userinfo-url` | `COURTKNIGHTS_GITHUB_USERINFO_URL` | _(GitHub production URL)_ | Override userinfo endpoint — use for local mocks |

When `--github-client-id` is empty the GitHub provider is disabled and returns `501`.

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
| `oauth-mock` | built from `tools/oauthfake` | `8090` | OAuth2 fake for Google and GitHub: redirect flow and device flow ([ADR-014](../../../docs/decisions/ADR-014_local-oauth-fake.md)) |

The fake accepts any client ID and secret. The same login always maps to the
same provider user (a stable Google `sub` and a stable numeric GitHub `id`), so
several test users can coexist.

### Step 1 — start the stack

```sh
docker compose -f docker-compose.dev.yml up -d --build --wait
```

### Step 2 — apply migrations

```sh
migrate -path db/migrations \
  -database "postgres://courtknights:courtknights@localhost:5432/courtknights?sslmode=disable" \
  up
```

### Step 3 — run the server

Point every OAuth2 endpoint override at the local fake:

```sh
make build
./api/build/courtknights-api \
  --google-client-id        mock-client \
  --google-client-secret    mock-secret \
  --google-redirect-url     http://localhost:8080/auth/google/callback \
  --google-auth-url         http://localhost:8090/google/authorize \
  --google-token-url        http://localhost:8090/google/token \
  --google-device-auth-url  http://localhost:8090/google/device/code \
  --google-userinfo-url     http://localhost:8090/google/userinfo \
  --github-client-id        mock-client \
  --github-client-secret    mock-secret \
  --github-redirect-url     http://localhost:8080/auth/github/callback \
  --github-auth-url         http://localhost:8090/github/authorize \
  --github-token-url        http://localhost:8090/github/token \
  --github-device-auth-url  http://localhost:8090/github/device/code \
  --github-userinfo-url     http://localhost:8090/github/user
```

### Step 4a — log in via the browser (redirect flow)

Go through **our server**, not the fake directly:

```
http://localhost:8080/auth/google
http://localhost:8080/auth/github
```

The server redirects to the fake's sign-in form. Enter any login (for example
`alice`); email and name are optional and default to `alice@example.com` and
`alice`. The fake redirects back to the callback, and the server returns a JWT.

**Scripted login (no form):** append `fake_user=<login>` (and optionally
`fake_email`, `fake_name`, or `fake_email_verified=false` to simulate an
unverified email) to the fake's authorize URL, and it approves immediately:

```sh
auth=$(curl -s -o /dev/null -w "%{redirect_url}" http://localhost:8080/auth/github)
cb=$(curl -s -o /dev/null -w "%{redirect_url}" "$auth&fake_user=alice")
curl -s -o /dev/null -w "%{redirect_url}\n" "$cb"   # → /?token=<JWT>
```

> `users.email` is unique and linking several providers to one account is out
> of scope (FEATURE_authentication). Logging in with GitHub using an email that
> already exists for a Google user fails with `401`. Use different logins per
> provider (e.g. `alice` on Google, `gh-alice` on GitHub).

### Step 4b — log in via the device flow (CLI path)

```sh
curl -s -X POST localhost:8080/auth/device -H 'Content-Type: application/json' \
  -d '{"provider":"github"}'
# → { "DeviceCode": "…", "UserCode": "BCDF-GHJK", "VerificationURI": "http://localhost:8090/github/device", … }
```

Approve the request either by opening the `VerificationURI` in a browser, or
non-interactively:

```sh
curl -s -X POST localhost:8090/github/device/approve -d user_code=BCDF-GHJK -d login=alice
# deny instead: POST localhost:8090/github/device/deny -d user_code=BCDF-GHJK
```

Then exchange the device code for a JWT:

```sh
curl -s -X POST localhost:8080/auth/device/token -H 'Content-Type: application/json' \
  -d '{"provider":"github","device_code":"<DeviceCode>"}'
```

### Step 5 — bootstrap an admin user (optional)

```sh
./api/build/courtknights-api \
  --bootstrap-email admin@example.com \
  --bootstrap-name  "Admin" \
  --bootstrap-pat   my-dev-pat
```

The PAT can then be used directly in the `Authorization: Bearer <pat>` header,
or exchanged for a JWT via `POST /auth/token/pat`.

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
