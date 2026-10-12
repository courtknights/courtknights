# oauthfake

Local fake of the **Google** and **GitHub** OAuth2 providers, for development
and tests only ([ADR-014](../../docs/decisions/ADR-014_local-oauth-fake.md)).
It accepts any client ID and secret. Never expose it outside a development
environment.

It is started by `docker-compose.dev.yml` as the `oauth-mock` service on port
`8090`. See [`api/cmd/server/README.md`](../../api/cmd/server/README.md#local-development)
for the full local login walkthrough.

## Endpoints

Every provider is served under its own prefix: `/google/...` and `/github/...`.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/{provider}/authorize` | Sign-in form. With `fake_user=<login>` (optional `fake_email`, `fake_name`), it approves immediately. |
| `POST` | `/{provider}/authorize` | Form submission → `redirect_uri?code=…&state=…` (or `error=access_denied`) |
| `POST` | `/{provider}/token` | `authorization_code` and device-code grants, with RFC 8628 errors |
| `POST` | `/{provider}/device/code` | Starts a device authorization request |
| `GET`/`POST` | `/{provider}/device` | Page to enter a user code and approve or deny it |
| `POST` | `/{provider}/device/approve` | Test API: `user_code`, `login` (optional `email`, `name`) → `204` |
| `POST` | `/{provider}/device/deny` | Test API: `user_code` → `204` |
| `GET` | `/google/userinfo` | Google OIDC shape: `sub`, `email`, `email_verified`, `name` |
| `GET` | `/github/user` | GitHub `/user` shape: numeric `id`, `login`, `name`, `email` |
| `GET` | `/healthz` | `204` |

Identities are stable. The same `login` always yields the Google `sub`
`fake-google-<login>` and the same numeric GitHub `id`. Email and name default
to `<login>@example.com` and `<login>`.

Device responses mirror each provider: Google returns `verification_url`, and
GitHub returns `verification_uri`. Polling faster than `interval` returns
`slow_down` and adds 5 seconds to the interval (RFC 8628 §3.5).

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `:8080` | Listen address |
| `--base-url` | `http://localhost:8090` | Externally reachable URL, used in device verification URIs |
| `--code-ttl` | `10m` | Lifetime of authorization and device codes |
| `--device-interval` | `1s` | Polling interval announced for the device flow |

## Development

```sh
make -C tools/oauthfake test    # also run by `make test` and in CI (`make tools-test`)
make -C tools/oauthfake lint
go run ./tools/oauthfake --addr :8090
```

Standard library only: no direct dependencies.
