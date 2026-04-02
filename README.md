# CourtKnights

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Open-source platform for creating and managing padel leagues and competitions.

---

## What is CourtKnights?

Small padel clubs and communities manage their leagues manually — WhatsApp groups, spreadsheets, paper. No single source of truth, no history, no statistics. CourtKnights removes that friction: it gives any club or informal community the tools to run their own leagues, track results, and follow player progression — without paying for enterprise software.

Self-hostable, community-driven, Apache 2.0.

---

## Architecture

CourtKnights is a monorepo with five workspaces:

| Workspace | Stack | Purpose |
|-----------|-------|---------|
| `api/` | Go (Echo v4) | REST API — domain logic, application services, infrastructure adapters |
| `web/` | Angular + PrimeNG | Frontend — standalone components, feature-based structure |
| `db/` | PostgreSQL + golang-migrate | Schema definitions and migrations |
| `infra/` | Manifests and scripts | Deployment scaffold |
| `e2e/` | Playwright | Acceptance tests scaffold |

```
courtknights/
  api/      # Go backend
  web/      # Angular frontend
  db/       # PostgreSQL schema and migrations
  infra/    # Deployment manifests
  e2e/      # Acceptance tests
  docs/     # Specs, ADRs, context
```

---

## Quick start

### Prerequisites

| Tool | Minimum version |
|------|----------------|
| Go | 1.25 |
| Node.js | 20 |
| Docker | any recent |
| `golang-migrate` | any recent |
| `gh` CLI | any recent |
| `make` | any |

> Integration tests use [Testcontainers](docs/decisions/ADR-004_testcontainers.md) — Docker must be running. No manual PostgreSQL setup required for tests.

### Setup

```bash
git clone git@github.com:courtknights/courtknights.git
cd courtknights

# Backend
make build
make test

# Frontend
make web-install
make web-start

# Database (requires a running PostgreSQL instance)
make db-migrate
```

All commands are dispatched from the root `Makefile`. See the individual workspace `Makefile` for more targets.

---

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a PR. CourtKnights follows **Spec Driven Development (SDD)** — no feature code is written without an approved spec.

---

## Architecture decisions

All major technical decisions are documented as ADRs in [`docs/decisions/`](docs/decisions/).

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
