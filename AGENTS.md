# Repository Guidelines

## Project Structure & Module Organization
- `cmd/user-service` boots the app, `config` wires environment variables, and `internal/app` composes dependencies for helpers in `pkg/`.
- Keep domain models in `internal/domain`, services in `internal/usecase`, repositories in `internal/adapters/postgres`, and adapters (HTTP, OAuth, brokers) in `internal/adapters`.
- Tests live beside their packages and in `test/` for fixture-heavy suites; migrations belong in `migrations/`, Docker setup in `docker-compose.yml`, and Go modules at the repo root.

## Build, Test, and Development Commands
- `make deps` installs `air` and `golangci-lint`, then tidies `go.mod`/`go.sum`.
- `make lint` runs `golangci-lint run ./...`; `make test` executes `go test ./...`.
- `make run` starts the service locally via `APP_ENV=local go run ./cmd/user-service`.
- Use `make docker-up|docker-down|docker-logs` to manage the Compose stack (Postgres, RabbitMQ, Nginx).
- `make migrate-up` and `migrate-down` call `migrate` with `.env` credentials against `migrations/`.

## Coding Style & Naming Conventions
- Follow Go formatting (`gofmt`/`goimports`) and lint rules enforced by `golangci-lint`.
- Exported identifiers start uppercase; keep JSON tags `snake_case` and prefer short, descriptive names in services and handlers.
- Separate Clean Architecture layers by directory (don’t mix HTTP adapters with domain logic). Keep comments succinct and tied to behavior.

## Testing Guidelines
- Go tests must live in `_test.go` files close to the code they cover or in `test/` for integration helpers.
- Favor table-driven tests with descriptive names (e.g., `TestRegisterUser_Succeeds`). Use interfaces or mocks for brokers and OAuth clients.
- Run `go test ./internal/...` or `go test ./test/...` to isolate suites; include testing notes in PR descriptions.

## Commit & Pull Request Guidelines
- Prefer Conventional Commit prefixes (`feat:`, `fix:`, `docs:`) to signal intent, followed by a brief imperative description.
- PRs should describe what changed, highlight migrations/configuration updates, mention manual test steps, and link related issues. Add screenshots if HTTP responses or logging behavior change.

## Configuration & Security Tips
- Create `.env` from `.env.example` and guard secrets (`DB_*`, `JWT_*`, `RABBITMQ_*`, OAuth keys). Do not commit credentials.
- Use `docker compose` commands from the repo root so networking matches production (Postgres, RabbitMQ, Nginx). Respect `X-Request-ID` propagation if testing middleware.
