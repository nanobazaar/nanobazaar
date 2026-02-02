# Repository Guidelines

## Project Structure & Module Organization

- Root contract artifacts: `CONTRACT.md`, `OPENAPI.yaml`, `TEST_VECTORS.md` (source of truth).
- Service code lives in `apps/relay/`:
  - entrypoint: `apps/relay/cmd/relay/main.go`
  - HTTP routing: `apps/relay/internal/http/`
  - domain/auth/store scaffolding: `apps/relay/internal/{domain,auth,store}`
  - SQLite schema/sqlc: `apps/relay/db/`
  - deploy config: `apps/relay/deploy/fly.toml`
- Ops scripts: `scripts/dev_reset.sh`, `scripts/backup_sqlite.sh`.

## Build, Test, and Development Commands

- `make run`: start local relay (defaults to `:8080`).
- `make fmt`: run `gofmt` on Go sources.
- `make lint`: `go vet ./...` for static checks.
- `make test`: run `go test ./...`.
- `make db/migrate`: run Goose migrations against `NBR_DB_PATH`.
- `make db/sqlc`: generate SQLC code from `db/schema.sql` + `db/queries.sql`.
- `scripts/dev_reset.sh`: wipe local SQLite db + rerun migrations.

## Coding Style & Naming Conventions

- Go formatting: `gofmt` (tabs for indentation).
- Package names: short, lowercase (`httpapi`, `domain`, `store`).
- JSON fields and API params use `snake_case` per contract (e.g., `bot_id`, `charge_expires_at`).
- Prefer small, explicit files over heavy generators; contract artifacts are authoritative.

## Testing Guidelines

- Use the Go `testing` package.
- Test files must be named `*_test.go` alongside the package under test.
- Run the full suite with `make test` (no coverage target yet).

## Commit & Pull Request Guidelines

- No git history yet; default to Conventional Commits (e.g., `feat(relay): add poll ack endpoint`).
- PRs should include: summary and tests run.
- If a contract change is needed, add a proposal to `CONTRACT_DIFF.md` instead of editing contract artifacts directly.

## Agent-Specific Instructions

- Contract-first and no-drift rule: `CONTRACT.md`, `OPENAPI.yaml`, and `TEST_VECTORS.md` are frozen.
- Subagents must implement changes within their scope and reference the relevant contract sections.
