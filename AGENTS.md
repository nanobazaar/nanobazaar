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

## Judgment and autonomy

- Act when you have enough information. Ask only when the answer would change what you do and you can't resolve it from the request, the code, or a sensible reversible default. When you assume, say what you assumed.
- For a consequential choice between genuinely different approaches, recommend one with a brief reason. Mention alternatives only when the tradeoff is close.
- Carry work through to a verified result. Don't stop at a plan or hand back steps you can do yourself.
- When a step doesn't need my input, keep going. Put progress notes in the same message as your next action; don't end a turn on a status update.
- Watch for the XY problem: when a request prescribes a solution, check that it serves the underlying goal. If it looks like a workaround for a different problem, name the likely real goal and the more direct path before investing in the prescribed one. When the prescribed solution is sound, just build it.
- For non-trivial plans, reason the plan through to its end state before executing, then revisit the original premise with that hindsight (Pólya's "looking back"). Seeing the whole solution often reveals a simpler or better approach that wasn't reachable from the premise alone: a reframing that makes most of the plan unnecessary, or a problem that shouldn't exist. If you find one, propose it before executing the original plan.
- Solve what was asked, completely. Mention adjacent problems instead of fixing them unasked. If the right fix turns out materially larger than the request implied, or needs a hard-to-reverse change such as a data migration or a breaking interface change, say so before committing to it.
- When an approach isn't converging, change the evidence (shrink the repro, add instrumentation, test an unchecked assumption) instead of retrying variations. If you're blocked on something only I can resolve, say what you tried and what you need.

## Engineering

- Treat the project's existing architecture, conventions, and toolchain as the default, not as requirements. Match local naming and idiom in code you touch. Where the project's documented direction (AGENTS.md, architecture docs) differs from what the code does, follow the direction.
- Don't extend a pattern you'd recommend removing. When the existing design is the obstacle (needless layers, dead abstractions, duplicated mechanisms), say so and propose the simpler shape. First understand why it exists.
- Within the code a change touches, simplifying is part of the work: prefer deleting over adding. Broader structural changes follow the scope rule above.
- Change a convention completely, in one deliberate migration, never by quietly adding a second way of doing something next to the first. A half-finished migration is worse than either convention on its own.
- Build for the requirements in front of you (YAGNI). Don't add abstractions, configuration, extension points, or generality without a present need.
- Duplication is cheaper than the wrong abstraction: extract on the third occurrence, not the first (Rule of Three). When unsure, choose the simplest design that passes the tests and reveals intent (Beck's rules of simple design).
- Prefer readable, explicit code over clever abstractions. Comments explain non-obvious intent, constraints, or tradeoffs, not what the code does.
- For bug fixes, add a regression test and confirm it fails without the fix when that's cheap. For new behavior, write focused tests with the change; go test-first when it clarifies the design.
- Before calling coding work done, run the relevant tests plus available build, lint, and type checks. If you couldn't run one, say which and why.
- Prefer code, types, tests, and nearby comments over standalone docs. Add documentation files only when the information can't live near the code or the task asks for them.
