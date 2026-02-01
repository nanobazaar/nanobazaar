You are Codex CLI running as the orchestrator. You are in an empty repo folder that only contains:
- spec/NANOBAZAAR_PRD_v0.2.md

Goal: bootstrap a Go monorepo for “NanoBazaar Relay” using Go + chi + sqlc + SQLite, deployable to Fly.io, in a way that is resilient to context limits by splitting work into gates and using subagents. Do NOT implement the whole product yet. First set up the repository structure, contract artifacts, tooling, and gate scaffolding so later subagents can implement each gate independently.

Hard constraints:
- Monorepo.
- Single service in v0: relay.
- SQLite (WAL) is the only DB for v0.
- Contract-first: CONTRACT.md, OPENAPI.yaml, TEST_VECTORS.md are the source of truth.
- After contract freeze, subagents must not edit contract files; they propose changes via CONTRACT_DIFF.md.
- Keep everything minimal and LLM-friendly; prefer few files with clear names over complex generators.

Step 0: Read the PRD
- Read spec/NANOBAZAAR_PRD_v0.2.md and extract the authoritative API, auth/signing rules, crypto envelope rules, state machines, TTL rules, error semantics, and endpoint list.

Step 1: Create repo structure
Create this structure (empty files allowed where content will come in later steps):
- CONTRACT.md
- OPENAPI.yaml
- TEST_VECTORS.md
- README.md
- .gitignore
- Makefile
- go.work
- /apps/relay
  - go.mod
  - /cmd/relay/main.go
  - /internal/http (router + handlers placeholder)
  - /internal/auth (middleware placeholder)
  - /internal/store (sqlc wrapper placeholder)
  - /internal/domain (state machine + validation placeholder)
  - /internal/events (poll/ack + enqueue placeholder)
  - /internal/retention (cleanup placeholder)
  - /db
    - schema.sql
    - queries.sql
    - sqlc.yaml
    - /migrations (goose)
  - /deploy/fly.toml
- /skills/nanobazaar
  - SKILL.md (placeholder)
  - HEARTBEAT.md.template
- /scripts
  - dev_reset.sh
  - backup_sqlite.sh

Step 2: Create contract artifacts (must be concrete)
Populate:
1) CONTRACT.md
- Must contain:
  - Identifier formats: bot_id and kid derivation + encoding
  - Auth header names and request signing canonical string + encoding rules
  - Replay protection rules (timestamp window, nonce cache TTL, error codes)
  - Idempotency rules (keying, TTL, 409 collision semantics)
  - Payload envelope + inner plaintext format, payload_kind enum, signature inputs, verification rules
  - Charge signature canonical string + verification rule
  - Job state machine, offer state machine, allowed transitions, 409 rules
  - Expiry defaults/max and triggers
  - Poll semantics (since exclusive, last_acked, 410 contract, resync playbook)
  - Rate limit buckets + 429 backoff contract
  - Authorization rules per resource (jobs, payloads, listing)
  - “No contract drift” rule: subagents cannot edit contract files directly

2) OPENAPI.yaml
- Only needs to cover v0 endpoints and request/response bodies at a high level, but it must be consistent with CONTRACT.md.
- Include standard error responses and status codes (400/401/403/404/409/410/429).
- Define schemas for: Bot, Offer, Job, Charge, Payload (outer envelope), Event, PollResponse, AckRequest, List cursors.

3) TEST_VECTORS.md
- Provide at least:
  - One request signing example input string (with filled fields), and expected encodings (no need to provide an actual signature if you can’t compute it, but include the exact input string bytes and hashing rules).
  - One inner payload signature canonical string example for payload_kind=request.
  - One charge signature canonical string example.
  - A list of “reject cases” (e.g., wrong recipient_bot_id, mismatched job_id, reused nonce, stale timestamp, idempotency collision).

Step 3: Tooling + build scaffolding
- go.work referencing apps/relay
- apps/relay/go.mod with:
  - chi router dependency
  - mattn/go-sqlite3 or modernc.org/sqlite (pick one; prefer simplest for Fly)
  - sqlc (as tool dependency or documented installation)
  - goose (as tool dependency or documented installation)
- Makefile targets:
  - fmt, lint (minimal), test
  - db/migrate (goose)
  - db/sqlc (generate)
  - run (local)
- Add .gitignore for SQLite db files, WAL/SHM, build artifacts.

Step 4: Minimal runtime skeleton (no full implementation)
Create a runnable relay skeleton that:
- Starts an HTTP server with chi.
- Has /healthz and /readyz endpoints.
- Opens SQLite DB at path from env (default ./data/relay.db for local).
- Enables WAL mode and busy_timeout.
- Does not implement business endpoints yet, but wires the route groups for /v0/… so later gates can fill them in.
- Include a config struct and env parsing (simple).
- Include a placeholder retention ticker (disabled by default).

Step 5: Fly.io baseline
Create apps/relay/deploy/fly.toml that:
- Mounts a volume at /data
- Sets DB path env to /data/relay.db
- Exposes port 8080
- Health checks /healthz
Keep it minimal and cheap.

Step 6: Gate scaffolding for subagents
Create:
- GATES.md describing gates 0–8 with deliverables and acceptance tests.
- A subfolder /spec/gates with one markdown per gate:
  - gate0_contract_freeze.md
  - gate1_schema_sqlc.md
  - gate2_auth_middleware.md
  - gate3_registry.md
  - gate4_offers.md
  - gate5_jobs.md
  - gate6_payloads.md
  - gate7_poll_retention.md
  - gate8_fly_ops.md
Each gate doc must include:
- Scope
- Inputs (which CONTRACT.md sections + OPENAPI paths)
- “Do not edit contracts” rule
- Done criteria

Step 7: Sanity checks
- Ensure files reference the PRD version (v0.2) and that CONTRACT.md/OPENAPI.yaml are consistent.
- Ensure `go test ./...` runs (even if only placeholder tests exist).
- Ensure `go run ./apps/relay/cmd/relay` starts and serves /healthz.

Important rules:
- No code generation beyond sqlc/goose scaffolding.
- No implementation of cryptography or endpoints beyond skeleton.
- No long prose beyond what’s required for the contract and gates.
- Prefer explicitness and correctness over cleverness.

Output: implement all files. At the end, print:
- Tree of created files
- How to run locally (commands)
- Next step: “Start Gate 1” instruction with the exact subagent prompt template you recommend.
