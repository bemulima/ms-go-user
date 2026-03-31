# User Service

Production-ready Go microservice implementing user management with Clean Architecture, Echo, GORM, retained NATS RPC, and OAuth integrations.

## Features

- Two-step registration via Tarantool microservice
- Classic and OAuth2 (Google/GitHub) authentication with JWT issuance
- RBAC integration for role and permission checks
- Postgres persistence via GORM with UUID primary keys
- Retained messaging role: NATS RPC for user/auth/RBAC coordination
- Clean Architecture layering (domain, usecase, port contracts, adapters, app)
- Structured logging, request ID propagation, CORS, health check, and graceful shutdown
- Docker Compose stack with Postgres and Nginx; shared NATS is provided by the infra messaging stack
- TDD-first unit and integration tests using `go test`

## Messaging Status

- Supported target transports for this service are HTTP and retained NATS RPC.
- Retained Core NATS RPC for this service is limited to `user.create-user`, which is a mutating request/reply subject and must stay idempotent under retries and queue-group-safe under multi-instance deployment.
- RabbitMQ has been physically removed from this service. The service no longer supports RabbitMQ as a transport choice.

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

## API

### Admin endpoints

- `GET /admin/v1/users?page=1&per=50` — list users (per: 10..100, default 50)
- `GET /admin/v1/users/:id` — get user by ID
- `POST /admin/v1/users` — create user
- `PATCH /admin/v1/users/:id` — update user
- `PATCH /admin/v1/users/:id/status` — change status
- `PATCH /admin/v1/users/:id/role` — change role

Response shape for list:
```json
{
  "data": {
    "totalCount": 123,
    "users": []
  }
}
```

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
internal/adapters/{rbac,filestorage,imageprocessor,tarantool}/ # external clients
internal/app/                 # composition root / DI
pkg/                          # shared utility packages (logging, HTTP helpers)
migrations/                   # database migrations
test/                         # unit and integration tests
```

Layering rules: `usecase` depends only on `domain` + `port`-like interfaces; adapters implement those interfaces and are wired in `app`. HTTP and retained NATS RPC transports stay under `internal/adapters`.
