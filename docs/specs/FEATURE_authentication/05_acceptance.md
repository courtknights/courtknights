# FEATURE_authentication — Acceptance Criteria

- **Last updated:** 2026-03-18
- **Status:** draft — pending human review and sign-off
- **Issue:** [#6](https://github.com/courtknights/courtknights/issues/6)
- **Author:** <!-- your name here -->

> This document is written and owned by the human architect.
> The AI agent uses it as the final gate before a feature is considered complete.
> A PR cannot be merged unless all criteria below are met.

---

## How to use this document

Each section is a scenario. Mark each criterion with one of:
- `[ ]` — not yet verified
- `[x]` — verified and passing
- `[~]` — partially met (add a note)
- `[!]` — blocked or out of scope (add a reason)

---

## Note on acceptance test infrastructure

OAuth2 redirect and Device flows require browser interaction and external provider consent, which cannot be automated with real credentials. The acceptance test suite (separate repository, executed by CI/CD) must use a **mock OAuth2 server** (e.g. `mockoidc`) to simulate Google and GitHub responses. PAT-based authentication does not require a mock and serves as the primary smoke test for the authentication system.

---

## AC-01 — Optional provider configuration

> The service starts and operates correctly with only one OAuth2 provider configured.

- [ ] If only `COURTKNIGHTS_OAUTH_GOOGLE_*` env vars are set, the service starts and Google login works; GitHub endpoints return `501 Not Implemented`
- [ ] If only `COURTKNIGHTS_OAUTH_GITHUB_*` env vars are set, the service starts and GitHub login works; Google endpoints return `501 Not Implemented`
- [ ] If both providers are configured, both work correctly
- [ ] If neither provider is configured but a bootstrap PAT is set, the service starts and PAT authentication works
- [ ] Missing provider credentials do not cause a startup crash — only the affected endpoints are disabled

---

## AC-02 — OAuth2 login via Web UI (Google)

> A user can log in to the platform using their Google account from the Web UI.
> Verified using a mock OAuth2 server in the acceptance test environment.

- [ ] `GET /auth/google` redirects the browser to the OAuth2 consent page
- [ ] After consent, the callback receives a valid code and the backend issues a JWT
- [ ] The backend fetches the user's email and name from the provider and creates a User record on first login
- [ ] On subsequent logins with the same Google account, no duplicate User is created
- [ ] The response redirects to the frontend with a valid JWT in the URL
- [ ] The JWT contains `sub` (user UUID), `email`, `role`, and `exp` (1 hour from issue time)

---

## AC-03 — OAuth2 login via Web UI (GitHub)

> A user can log in to the platform using their GitHub account from the Web UI.
> Verified using a mock OAuth2 server in the acceptance test environment.

- [ ] `GET /auth/github` redirects the browser to the OAuth2 consent page
- [ ] After consent, the backend creates or returns the User record linked to the GitHub identity
- [ ] The response redirects to the frontend with a valid JWT
- [ ] A user who previously logged in with Google and attempts to log in with GitHub with the same email is treated as a different identity (separate User records, one per provider)

---

## AC-04 — OAuth2 Device Authorization Flow (CLI)

> A CLI user can authenticate using the Device Authorization Grant without a browser redirect.
> Verified using a mock OAuth2 server in the acceptance test environment.

- [ ] `POST /auth/device {"provider":"google"}` returns `device_code`, `user_code`, `verification_uri`, `expires_in`, and `interval`
- [ ] While the user has not yet authorised, `POST /auth/device/token` returns `authorization_pending`
- [ ] Once the user authorises, `POST /auth/device/token` returns a valid JWT
- [ ] The same flow works for GitHub (`"provider":"github"`)
- [ ] An unknown provider returns `400 Bad Request`
- [ ] A provider that is not configured returns `501 Not Implemented`

---

## AC-05 — PAT authentication

> A user or automation process can authenticate using a Personal Access Token.

- [ ] An admin can create a PAT via `POST /api/v1/pats`; the raw value is returned once only
- [ ] `POST /auth/token/pat {"pat":"<raw-value>"}` returns a valid JWT
- [ ] An invalid PAT returns `401 Unauthorized`
- [ ] An expired PAT returns `401 Unauthorized`
- [ ] After a PAT is revoked via `DELETE /api/v1/pats/:id`, authenticating with it returns `401`
- [ ] A non-admin attempting to create a PAT receives `403 Forbidden`

---

## AC-06 — JWT token lifecycle

> Issued JWTs expire after 1 hour and can be refreshed before expiry.

- [ ] A JWT issued by any authentication flow expires exactly 1 hour after issue
- [ ] `POST /auth/refresh` with a valid non-expired JWT returns a new JWT with a new `exp`
- [ ] `POST /auth/refresh` with an expired JWT returns `401 Unauthorized`
- [ ] `POST /auth/refresh` with a tampered JWT returns `401 Unauthorized`

---

## AC-07 — Protected API access

> All `/api/v1/*` endpoints require a valid JWT. Public `/auth/*` endpoints do not.

- [ ] A request to any `/api/v1/*` endpoint without a JWT returns `401 Unauthorized`
- [ ] A request to any `/api/v1/*` endpoint with an expired JWT returns `401 Unauthorized`
- [ ] A request to any `/auth/*` endpoint without a JWT is processed normally
- [ ] A valid JWT from any authentication method (OAuth2, Device Flow, PAT) grants access to protected endpoints identically

---

## AC-08 — Role-based access control

> Admin users have elevated permissions over regular users.

- [ ] A user with `role=admin` can promote or demote another user via `PUT /api/v1/users/:id/role`
- [ ] A user with `role=user` attempting the same action receives `403 Forbidden`
- [ ] `GET /api/v1/users/me` returns the correct profile for both `admin` and `user` roles

---

## AC-09 — Bootstrap admin on first startup

> The platform can be initialised without a Web UI using environment variables.

- [ ] If `COURTKNIGHTS_BOOTSTRAP_PAT`, `COURTKNIGHTS_BOOTSTRAP_EMAIL`, and `COURTKNIGHTS_BOOTSTRAP_NAME` are set and no users exist, the server creates an admin user and a linked PAT on startup
- [ ] The admin user can immediately authenticate using the bootstrap PAT via `POST /auth/token/pat`
- [ ] If users already exist, the bootstrap is skipped — no duplicate users are created
- [ ] If the bootstrap env vars are not set, startup proceeds normally with no side effects

---

## AC-10 — Data integrity

> The User and PAT data model is consistent and safe.

- [ ] Two users cannot share the same `(provider, provider_id)` combination
- [ ] Every user has a non-null email
- [ ] PAT raw values are never stored — only the hash and salt are persisted
- [ ] A deleted PAT cannot be used to authenticate

---

## AC-11 — Test suite

> The implementation is covered by automated tests.

- [ ] `make test` passes with no failures
- [ ] `make test-int` passes with no failures
- [ ] Unit test coverage meets the minimums defined in `docs/testing/strategy.md`
- [ ] No external processes are required to run `make test`

---

## Out of scope (explicitly excluded from this feature)

- Email + password authentication
- Linking multiple social providers to a single user account
- Fine-grained permissions beyond `admin` / `user`
- Per-organization user scoping
- PAT storage in an external vault (deferred)
- Mobile or native app authentication flows

---

## Sign-off

| Role      | Name  | Date       | Status   |
| --------- | ----- | ---------- | -------- |
| Architect | aagea | 2026-03-18 | [x] done |
