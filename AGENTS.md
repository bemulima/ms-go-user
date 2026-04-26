# Repository Guidelines

<!-- codex-agent-bootstrap:start -->
## Agent Bootstrap
- Before planning or changing files, read all available project instructions:
  - `docs/*`
  - `prompts/*`
- Treat `prompts/git-workflow.md` as the required workflow for issue, branch, commit, push, Pull Request, merge, and issue-closing behavior.
- If a listed directory does not exist, continue with the instructions that are present.
<!-- codex-agent-bootstrap:end -->

<!-- codex-shared-policy:start -->
## Codex Shared Policy (Managed)

Source of truth: `prompts/codex-shared-agents.md`

This file is the canonical shared policy for repo-level `AGENTS.md` files in
git-backed repositories under `/Users/marat/Developments/microservices`.

## Cache policy
- Use only repo-local `.cache` for temporary build and tool artifacts.
- Do not create or rely on repo-local `.gocache`.
- For Go commands, prefer these locations:
  - `XDG_CACHE_HOME=$PWD/.cache`
  - `GOCACHE=$PWD/.cache/go-build`
  - `GOMODCACHE=$PWD/.cache/gomod`
  - `GOBIN=$PWD/.cache/bin`
- Put disposable local binaries in `.cache/bin`.
- Treat `.cache` as disposable local state. Do not commit it.

## Workspace hygiene
- Do not introduce extra cache directories when `.cache` can be used instead.
- Keep temporary logs, generated reports, and ad-hoc tooling output under
  `.cache` when practical.
- Do not store persistent project data in `.cache`.
- If a repo has stricter local requirements, document them in that repo's
  `AGENTS.md` below the managed shared block.

## Scope
- This policy is synced into repo-level `AGENTS.md` files by
  `prompts/scripts/sync_agents.py`.
- The canonical source of truth is this file, not the generated copies.
<!-- codex-shared-policy:end -->

## Project Structure & Module Organization
- `cmd/user-service` boots the app, `config` wires environment variables, and `internal/app` composes dependencies for helpers in `pkg/`.
- Keep domain models in `internal/domain`, services in `internal/usecase`, repositories in `internal/adapters/postgres`, and adapters (HTTP, OAuth, brokers) in `internal/adapters`.
- Tests live beside their packages and in `test/` for fixture-heavy suites; migrations belong in `migrations/`, Docker setup in `docker-compose.yml`, and Go modules at the repo root.

## Build, Test, and Development Commands
- `make deps` installs `air` and `golangci-lint`, then tidies `go.mod`/`go.sum`.
- `make lint` runs `golangci-lint run ./...`; `make test` executes `go test ./...`.
- `make run` starts the service locally via `APP_ENV=local go run ./cmd/user-service`.
- Use `make docker-up|docker-down|docker-logs` to manage the Compose stack. The app compose starts Postgres and Nginx; shared NATS comes from the infra messaging stack.
- `make migrate-up` and `migrate-down` call `migrate` with `.env` credentials against `migrations/`.

## Coding Style & Naming Conventions
- Follow Go formatting (`gofmt`/`goimports`) and lint rules enforced by `golangci-lint`.
- Exported identifiers start uppercase; keep JSON tags `snake_case` and prefer short, descriptive names in services and handlers.
- Separate Clean Architecture layers by directory (don’t mix HTTP adapters with domain logic). Keep comments succinct and tied to behavior.

## Testing Guidelines
- Go tests must live in `_test.go` files close to the code they cover or in `test/` for integration helpers.
- Favor table-driven tests with descriptive names (e.g., `TestRegisterUser_Succeeds`). Use interfaces or mocks for brokers and OAuth clients.
- Run `go test ./internal/...` or `go test ./test/...` to isolate suites; include testing notes in PR descriptions.

## Admin API Notes
- `GET /admin/v1/users?page=1&per=50` returns `{totalCount, users}` (per: 10..100, default 50).

## Commit & Pull Request Guidelines
- Prefer Conventional Commit prefixes (`feat:`, `fix:`, `docs:`) to signal intent, followed by a brief imperative description.
- PRs should describe what changed, highlight migrations/configuration updates, mention manual test steps, and link related issues. Add screenshots if HTTP responses or logging behavior change.

## Configuration & Security Tips
- Create `.env` from `.env.example` and guard secrets (`DB_*`, `JWT_*`, OAuth keys). Do not commit credentials.
- Use `docker compose` commands from the repo root so networking matches production-like local runs (Postgres and Nginx here, shared NATS from infra/messaging). Respect `X-Request-ID` propagation if testing middleware.
