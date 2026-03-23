# ADR-001: HTTP Framework — Echo

- **Status:** accepted
- **Date:** 2026-03-17
- **Deciders:** CourtKnights Community

---

## Context

The backend requires an HTTP framework to expose REST APIs. Go's standard library `net/http` is capable but low-level — routing, middleware chaining, and request binding require significant boilerplate. A lightweight framework is preferred over a full-stack one to keep the binary small and the dependency surface minimal.

## Decision

Use **[Echo](https://echo.labstack.com/)** (v4) as the HTTP framework for all API endpoints.

- Route handlers live in `api/internal/api/<feature>/`.
- Middleware (auth, logging, error handling, CORS) is registered at the router level, not inside handlers.
- Request binding and validation are handled via Echo's built-in mechanisms.

## Alternatives considered

| Alternative | Why not chosen |
|-------------|----------------|
| `net/http` (stdlib) | Too much boilerplate for routing and middleware; slows development |
| Gin | Similar feature set to Echo but slightly heavier; Echo's API is cleaner |
| Fiber | Built on fasthttp, not compatible with standard `net/http` interfaces; makes testing harder |
| Chi | Minimal router, good choice, but lacks built-in request binding and validation |

## Consequences

### Positive
- Minimal overhead and fast routing
- Built-in request binding, validation, and error handling
- Familiar middleware pattern; easy to test handlers in isolation
- Active community and well-maintained

### Negative
- Adds an external dependency
- Team must follow Echo's conventions (context propagation via `echo.Context`)

## References
- https://echo.labstack.com/
- `Dependency.md` — Echo entry
