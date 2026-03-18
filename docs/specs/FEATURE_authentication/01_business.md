# FEATURE_authentication — Business Context

- **Last updated:** 2026-03-18
- **Status:** draft
- **Issue:** [#6](https://github.com/courtknights/courtknights/issues/6)

---

## Problem

CourtKnights manages clubs, competitions, teams, and players. Each of these resources requires controlled access: only authorised users should be able to create competitions, validate results, or manage members. Without an identity and authentication system there is no way to enforce any permission model.

Additionally, CourtKnights is an open-source platform intended for self-hosted communities. Lowering the barrier to registration is critical — users should not need to create yet another email/password account. Delegating authentication to well-known identity providers (Google, GitHub) reduces friction and shifts the security burden of credential management to trusted third parties.

As per ADR-005, the CLI is a first-class interface alongside the Web UI. Authentication must therefore support a non-browser path suitable for automation and scripted workflows.

---

## Goal

Establish the **identity and authentication foundation** of the platform:

1. Allow users to sign in via OAuth2 (Google and GitHub)
2. Allow operators and automation scripts to authenticate via Personal Access Tokens (PAT)
3. Issue a platform JWT upon successful authentication — the single token accepted by the API (see ADR-006)
4. Allow clients to renew their JWT without re-authenticating
5. Store a minimal User entity in the database
6. Provide two platform-level roles (`admin`, `user`) as the first access-control layer

This feature is a prerequisite for every other feature that requires knowing who the caller is.

---

## Authentication paths

The platform supports three paths to obtain a JWT. All clients — Web UI, CLI, scripts — receive a JWT and use it identically from that point on (see ADR-006).

### Path 1 — OAuth2 redirect (Web UI, interactive)

The standard browser-based flow. The user is redirected to Google or GitHub, authenticates there, and is redirected back to the platform with a JWT.

### Path 2 — OAuth2 Device Authorization Grant (CLI, interactive)

For interactive CLI sessions where a browser redirect is not possible. The user is shown a URL and a short code in the terminal, opens the URL in a browser, and authorises. The CLI receives a JWT automatically — no redirect required.

### Path 3 — Personal Access Token / PAT (CLI and automation, non-interactive)

A long-lived opaque token that can be exchanged for a JWT without user interaction. Intended for scripts, CI/CD pipelines, and operator tooling.

PATs are created and managed by an `admin`. A bootstrap PAT can be injected at first startup via environment variable to enable fully headless deployments.

---

## User stories

### US-01 — Sign in with Google or GitHub (Web UI)

> As a visitor, I want to sign in using my Google or GitHub account so that I do not need to create and remember a separate password.

**Acceptance criteria:**
- The platform supports OAuth2 login with Google and GitHub
- After a successful login the user receives a JWT
- If the user does not yet exist a new User record is created automatically
- If the user already exists the existing record is used and a JWT is issued

---

### US-02 — Renew a JWT without re-authenticating

> As a signed-in user, I want to renew my session token so that I am not forced to log in again after a short period.

**Acceptance criteria:**
- A refresh endpoint issues a new JWT given a valid, non-expired JWT
- The new JWT resets the expiry clock
- An expired JWT cannot be refreshed — the user must authenticate again

---

### US-03 — Sign in from the CLI using OAuth2 (Device Flow)

> As a CLI user, I want to authenticate with my Google or GitHub account from the terminal so that I can use the CLI interactively without creating a PAT.

**Acceptance criteria:**
- Running `courtknights auth login` starts the Device Authorization Grant flow
- The CLI prints a URL and a user code to the terminal
- The user opens the URL in a browser, authorises, and the CLI receives a JWT automatically
- The JWT is stored locally for subsequent commands
- The flow supports both Google and GitHub providers

---

### US-04 — Authenticate from the CLI or automation using a PAT

> As an operator or automation script, I want to exchange a Personal Access Token for a JWT so that I can call the API without a browser.

**Acceptance criteria:**
- An admin can create a PAT for any user via CLI or API
- The raw PAT value is shown only once at creation time
- A dedicated auth endpoint accepts a PAT and returns a JWT
- A PAT can be revoked by an admin at any time
- A bootstrap admin PAT can be configured at first startup via environment variables — email and name are required alongside the PAT value

---

### US-05 — Platform admin access

> As a platform admin, I want to have an elevated role so that I can perform administrative operations across the platform.

**Acceptance criteria:**
- The `admin` role grants access to platform-wide administrative endpoints and CLI commands
- The first admin is bootstrapped at first startup via environment variable
- Only an existing `admin` can promote another user to `admin`
- Role assignment is not self-service

---

## User entity

The User is the core identity record. It is global — not scoped to a club or competition.

| Field         | Type      | Notes                                                                      |
| ------------- | --------- | -------------------------------------------------------------------------- |
| `id`          | UUID      | Primary key, generated by the platform                                     |
| `email`       | string    | Unique. Retrieved from the OAuth2 provider at login; provided via environment variable for PAT-only users. Never null. |
| `name`        | string    | Display name. Retrieved from the OAuth2 provider; provided via environment variable for PAT-only users.               |
| `role`        | enum      | `admin` or `user`                                                          |
| `provider`    | enum      | `google`, `github`, or `pat`                                               |
| `provider_id` | string    | The external identifier for the provider. OAuth2: external user ID from the provider. PAT: the `id` of the row in the PAT table. Never null. |
| `created_at`  | timestamp | Set on creation                                                            |
| `updated_at`  | timestamp | Updated on every write                                                     |

Each user has exactly one provider. Email is mandatory for all users regardless of provider. The `provider_id` field always references the external identifier for the given provider — for PAT users this is the PAT record id (see ADR-007).

---

## Out of scope

- Email + password authentication
- Additional OAuth2 providers beyond Google and GitHub
- Linking multiple social providers to the same user account
- Per-club or per-competition role management (handled in future features)
- PAT scopes or fine-grained permissions on PATs
- Account deletion or deactivation
- Multi-factor authentication

---

## Dependencies

- **ADR-001** (Echo) — authentication endpoints are implemented as Echo handlers
- **ADR-003** (PostgreSQL) — User entity and PAT hashes are persisted in PostgreSQL
- **ADR-005** (CLI as first-class interface) — drives the requirement for non-browser authentication paths
- **ADR-006** (JWT as single token) — defines the token strategy this feature implements
- **ADR-007** (PAT design) — defines PAT storage, hashing, and identity resolution
- **docs/context/users.md** — defines the role model that this feature underpins
