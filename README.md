# User Service

Production-ready Go microservice implementing user management with Clean Architecture, Echo, GORM, RabbitMQ, and OAuth integrations.

## Features

- Two-step registration via Tarantool microservice
- Classic and OAuth2 (Google/GitHub) authentication with JWT issuance
- RBAC integration for role and permission checks
- Postgres persistence via GORM with UUID primary keys
- RabbitMQ or NATS event publication for user lifecycle events (configurable via `MESSAGE_BROKER`)
- Clean Architecture layering (domain, usecase, port contracts, adapters, app)
- Structured logging, request ID propagation, CORS, health check, and graceful shutdown
- Docker Compose stack with Postgres, RabbitMQ, Nginx
- TDD-first unit and integration tests using `go test`

## Getting Started

1. Copy `.env.example` to `.env` and adjust secrets.
2. Run migrations using `make migrate-up` (requires `golang-migrate`).
3. Launch the stack: `make docker-up`.
4. Access the service via `http://localhost:8000` (proxied through Nginx).

## Make Targets

- `make deps` — install Go dependencies and linters
- `make lint` — run `golangci-lint`
- `make test` — execute unit and integration tests
- `make run` — run the service locally
- `make docker-up` / `make docker-down` — manage Docker Compose
- `make docker-logs` — tail container logs

## Testing

The service is built using TDD with unit tests covering business services and handlers. Run `make test` for the full suite.

## Observability

Requests carry an `X-Request-ID` header. Structured logs are emitted via Zerolog. Health endpoint: `GET /health`.

## Architecture Overview

```
cmd/                          # main entrypoint
config/                       # environment configuration loading
internal/domain/              # domain entities and value objects
internal/usecase/             # business logic (auth, user, manage)
internal/adapters/http/        # Echo router, handlers, middleware
internal/adapters/postgres/    # GORM repositories
internal/adapters/{rbac,broker,filestorage,imageprocessor,tarantool}/ # external clients
internal/app/                 # composition root / DI
pkg/                          # shared utility packages (logging, HTTP helpers)
migrations/                   # database migrations
test/                         # unit and integration tests
```

Layering rules: `usecase` depends only on `domain` + `port`-like interfaces; adapters implement those interfaces and are wired in `app`. HTTP and broker transports stay under `internal/adapters`.
