# ADR-014: Local OAuth2 Fake Provider

- **Status:** accepted
- **Date:** 2026-10-12
- **Deciders:** aagea

---

## Context

The local development stack (`docker-compose.dev.yml`) used
`ghcr.io/navikt/mock-oauth2-server` to stand in for Google and GitHub. It is an
**OpenID Connect** mock, and it does not match what CourtKnights actually calls:

- **GitHub login fails.** `GitHubProvider` requests the scopes
  `read:user user:email`, as real GitHub requires. The mock rejects any
  authorize request without `openid`:
  `invalid request: the scope must include an openid value`.
- **GitHub user data has the wrong shape.** `GitHubProvider` parses GitHub's
  `/user` response (numeric `id`, `login`). The mock returns OIDC claims
  (`sub` as a string), so every mock GitHub user would get `provider_id = "0"`.
- **The device flow cannot be tested for any provider.** The mock exposes no
  device authorization endpoint (RFC 8628). The CLI login path
  (FEATURE_authentication US-03, `courtknights auth login`) cannot be exercised
  locally. As a result, bugs in that path went unnoticed (#128).

The future `e2e/` workspace (Playwright) will need the same capability:
scripted logins for several distinct users, without a browser form in the
middle.

## Decision

Replace the third-party mock with a small **OAuth2 fake provider written in
Go**, maintained in this repository.

### Location and module

- It lives in a new **`tools/`** workspace, which holds development and test
  tooling: `tools/oauthfake/`. `infra/` remains for deployment.
- `tools/oauthfake` is its own Go module
  (`github.com/courtknights/courtknights/tools/oauthfake`), added to `go.work`
  next to `./api` (extends [ADR-009](ADR-009_go-workspace.md)).
- It uses the **standard library only**: no direct dependencies.
- It shares no code with `api/`. It reproduces the **providers' public HTTP
  contracts**, not CourtKnights internals.

### Behaviour

One fake serves both providers under path prefixes, `/google/...` and
`/github/...`, each mimicking the real provider's response shapes:

| Endpoint (per provider) | Purpose |
|-------------------------|---------|
| `GET /authorize` | HTML login form (login, email, name). With `fake_user=<login>`, it approves immediately and redirects (scripted logins). |
| `POST /authorize` | Form submission → redirect to `redirect_uri?code=…&state=…` |
| `POST /token` | `authorization_code` and `urn:ietf:params:oauth:grant-type:device_code` grants, with RFC 8628 error responses (`authorization_pending`, `slow_down`, `access_denied`, `expired_token`) |
| `POST /device/code` | Device authorization: `device_code`, `user_code`, `verification_uri`, `expires_in`, `interval` |
| `GET`/`POST /device` | Page to enter a user code and approve or deny it |
| `POST /device/approve`, `POST /device/deny` | Test API: approve or deny a pending user code non-interactively |
| `GET /google/userinfo` | Google OIDC userinfo shape: `sub`, `email`, `name` |
| `GET /github/user` | GitHub `/user` shape: numeric `id`, `login`, `name` (`email` is `null`, as for a private email) |
| `GET /github/user/emails` | GitHub `/user/emails` shape: primary address with its `verified` flag (added by #131) |

- **Stable identities:** the same `login` always yields the same Google `sub`
  and the same numeric GitHub `id`. Distinct logins never collide, so several
  test users can coexist.
- **In-memory state:** codes and tokens are held in memory with expiry. A
  restart clears them.
- **Never for production:** the fake accepts any client ID and secret. It is
  started only by `docker-compose.dev.yml` and by tests.

### Testing and CI

- The fake has its own tests (`go test` with `httptest`). They run in the
  existing backend CI jobs through a `make tools-test` target.
- The fake is excluded from the backend coverage threshold, which keeps
  measuring `api/` only.

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| Keep `navikt/mock-oauth2-server` and add a `--github-scopes` flag so the dev setup can request `openid` | Fixes only the GitHub redirect flow. The device flow stays untestable for both providers, and production config gains a dev-only option. |
| WireMock or another generic stub server | A new third-party image, and the stateful RFC 8628 polling would have to be expressed as WireMock scenarios. More configuration than code, and harder to keep in sync with provider shapes. |
| Document the limitation only | No code change, but GitHub login and the CLI device flow stay untestable locally, and bugs such as #128 stay hidden. |
| Put the fake in `infra/dev/` | Mixes development tooling with deployment manifests and scripts. |
| Put the fake inside the `api` module (e.g. `api/cmd/oauthfake`) | Ships test tooling inside the production module, and tempts it to import `api/internal` packages, coupling the fake to the code it is meant to test. |

## Consequences

### Positive

- Both Google and GitHub logins, and the device flow for both, can be tested
  locally and in future e2e tests.
- Scripted logins for any number of distinct users (`fake_user=…`,
  `/device/approve`).
- No third-party image to track for the OAuth mock.
- Device-flow bugs become reproducible (#128).

### Negative

- About 400 lines of code plus tests to maintain.
- The fake follows our understanding of each provider's contract. If Google
  or GitHub change their responses, the fake will not notice; only real-provider
  testing catches that.
- One more Go module in the workspace.

## References

- [ADR-009 — Go workspace](ADR-009_go-workspace.md)
- [ADR-005 — CLI as a first-class interface](ADR-005_cli-first-class-citizen.md)
- RFC 6749 (OAuth 2.0), RFC 8628 (Device Authorization Grant)
- GitHub REST API — `GET /user`
- #123 — this decision
- #128 — device-flow bugs found while designing it
