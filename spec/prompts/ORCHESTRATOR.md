You are the orchestrator for NanoBazaar Relay. Create one subagent per gate in GATES.md (Gate 0 through Gate 8), but execute them in this order:

Order:
1) Gate 0 (Contract Freeze)
2) Gate 1 (Schema + sqlc)
3) Gate 2 (Auth middleware)
4) Parallel wave A: Gate 3 (Registry), Gate 4 (Offers), Gate 5 (Jobs), Gate 6 (Payloads)
5) Gate 7 (Poll + retention) — after Gate 4/5/6 are complete
6) Gate 8 (Fly ops) — last

Each subagent must:
- Read its spec in `spec/gates/gateX_*.md` and follow it strictly.
- Follow `AGENTS.md` repository rules (contract-freeze, naming, tests, etc.).
- Only operate within its gate scope; do not touch other gates’ concerns.
- After Gate 0, do NOT edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`. If a contract change is needed, propose it in `CONTRACT_DIFF.md` only.
- Prefer small, explicit files; avoid unnecessary generators.

Subagent workflow:
1) Inspect relevant code under `apps/relay/` and any gate-specific inputs listed in the spec.
2) Implement required changes.
3) Add or update tests (Go `testing` package).
4) Run the most relevant commands (e.g., `make db/migrate`, `make db/sqlc`, `make test`, etc.).
5) Report back: summary, files touched, tests run, and any risks or open questions.

Subagents to create:
- Gate 0: Contract Freeze — verify contract artifacts, no TODOs, document freeze rule; only propose diffs in `CONTRACT_DIFF.md`.
- Gate 1: Schema + sqlc — SQLite schema, migrations, sqlc queries/types.
- Gate 2: Auth middleware — signature verification, replay protection, idempotency.
- Gate 3: Registry — `/v0/bots` endpoints with PoP + key pinning.
- Gate 4: Offers — offer lifecycle endpoints + pagination stability.
- Gate 5: Jobs — job lifecycle endpoints + state machine + expiry.
- Gate 6: Payloads — store/fetch/list + metadata + auth.
- Gate 7: Poll + retention — events, poll/ack, retention cleanup, 410 handling.
- Gate 8: Fly ops — fly.toml, volume, health checks, local run.

Coordinate dependencies and resolve conflicts by integrating subagent outputs after each wave.