# Regression Map

Tracks dependencies between features. Update this file every time a feature is merged.
When a feature is modified, all dependent features in this map must have their tests re-run.

- **Last updated:** 2026-03-20

---

## How to use this map

1. Find the feature being modified in the **Feature** column.
2. Re-run tests for all features listed in its **Depended on by** column.
3. Update this file if the merge introduces new dependencies.

---

## Map

| Feature | Description | Depends on | Depended on by |
|---------|-------------|------------|----------------|
| FEATURE_authentication / domain | `user`, `pat`, `ckerrors` domain entities | — | All auth layers |
| FEATURE_authentication / postgres | `UserRepository`, `PATRepository` (PostgreSQL) | domain | application/auth, cmd/server |
| FEATURE_authentication / infrastructure/jwt | JWT sign + validate | domain | application/auth, api/common |
| FEATURE_authentication / infrastructure/oauth2 | Google + GitHub OAuth2 providers | domain | application/auth |
| FEATURE_authentication / application | `UserManager`, `JWTManager`, `OAuthManager`, `AuthManager` | domain, postgres, jwt, oauth2 | All API handlers |
| FEATURE_authentication / api/auth | OAuth2 redirect, device flow, PAT exchange, JWT refresh handlers | application/auth, api/common | — |
| FEATURE_authentication / api/pats | PAT management handlers (`/api/v1/pats`) | application/auth, api/common | — |
| FEATURE_authentication / api/users | User management handlers (`/api/v1/users`) | application/auth, api/common | — |
| FEATURE_authentication / cmd/server | Server entrypoint, wiring, bootstrap hook | All of the above | — |
