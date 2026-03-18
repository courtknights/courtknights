# ADR-006 — JWT as the Single Authentication Token

- **Date:** 2026-03-18
- **Status:** accepted

---

## Context

The platform needs a unified way to authorise requests across all protected endpoints. Clients are heterogeneous — browser-based Web UI, standalone CLI, and automation scripts — and each has a different way of obtaining credentials. The API layer must not need to know or care about which authentication method a client used.

## Decision

**JWT is the only token accepted by the API.** Every protected endpoint validates a JWT in the `Authorization: Bearer` header — nothing else.

All authentication methods (OAuth2 redirect, OAuth2 Device Flow, PAT exchange) are handled exclusively by the `/auth/*` endpoints. These endpoints are responsible for validating the incoming credentials and issuing a JWT. From that point, the client uses the JWT for all subsequent API calls.

```
credentials  →  /auth/*  →  JWT  →  API calls
```

### JWT specification

| Property          | Value                                            |
| ----------------- | ------------------------------------------------ |
| Signing algorithm | HS256                                            |
| Expiry            | 1 hour                                           |
| Payload claims    | `sub` (user UUID), `email`, `role`, `iat`, `exp` |

### Refresh

A `/auth/refresh` endpoint accepts a valid, non-expired JWT and returns a new JWT with a reset expiry. This avoids forcing the user to re-authenticate on every session.

### PAT exchange

Personal Access Tokens are long-lived opaque tokens stored as a hash in the database. They are never accepted directly by protected endpoints — they must be exchanged for a JWT via `POST /auth/token/pat`. Once the JWT is obtained, the PAT is no longer involved.

---

## Consequences

- The API middleware is simple: validate JWT signature and expiry, extract claims. No special-casing per authentication method.
- All clients follow the same pattern regardless of how they obtained the JWT.
- The `/auth/*` endpoints are the only place where non-JWT credentials are processed.
- If the signing algorithm or expiry policy needs to change in the future, only the auth layer is affected.

---

## Alternatives considered

| Alternative                              | Reason rejected                                                             |
| ---------------------------------------- | --------------------------------------------------------------------------- |
| Accept both JWT and PAT on all endpoints | Complicates middleware; two token types to validate and maintain            |
| Opaque session tokens                    | Stateful; requires server-side session storage; does not scale horizontally |
| API keys instead of PAT                  | Same concept, less standard naming for a developer-facing platform          |
