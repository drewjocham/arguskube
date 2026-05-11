# api-gov

API governance service: tracks OpenAPI specs, captures live traffic, detects drift
between declared contracts and observed behavior, and surfaces breaking changes.

## Architecture

```
api-gov/
├── backend/     Go API server (chi, pgx, pgvector) — spec storage, drift detection
├── middleware/  Python (FastAPI) — drop-in middleware that pushes specs + traffic
├── agents/      Python (LangGraph) — Sentinel/Architect/Healer analysis agents
├── docker/      Compose stack: postgres + pgvector, redis, otel-collector
├── k8s/         Kubernetes manifests for production deployment
└── docs/        ADRs + design notes
```

The middleware lives inside the customer's FastAPI app and reports to the Go backend.
The backend persists specs in Postgres, embeds endpoints into pgvector, and hands
drift detection off to the agents.

## Quick start

```
make dev          # docker compose: API on :8080, agents on :8001
make test         # all tests (Go + agents + middleware)
make lint         # golangci-lint + ruff
```

See `make help` for the full target list.

## Configuration

Copy `.env.example` to `.env` and fill in the required keys. Loaders read from the
local `.env`; the docker-compose stack reads the same file via `env_file:`.

| Variable | Purpose | Default |
|---|---|---|
| `API_GOV_URL` | Where the middleware POSTs specs/traffic | `http://localhost:8080` |
| `API_GOV_API_KEY` | Bearer token for the backend | (none, anonymous) |
| `API_GOV_SAMPLE_RATE` | Traffic sample 0.0–1.0 | `1.0` |
| `API_GOV_MAX_BODY_BYTES` | Largest request body the middleware will capture | `1048576` (1 MiB) |
| `DATABASE_URL` | Postgres DSN for the backend | from compose |
| `OPENAI_API_KEY` | Drift-detection LLM (agents) | (required for agents) |

## Component docs

- [middleware](middleware/README.md) — drop-in install + tuning
- [backend](backend/README.md) — REST API reference
- [agents](agents/README.md) — analysis graph and prompts
- [docs/adr](docs/adr/) — architecture decision records

## License

Internal — see `LICENSE` once published.
