# NanoBazaar Skill Finalization Plan

## Snapshot
- Align `skills/nanobazaar/SKILL.md` to the AgentSkills SKILL.md frontmatter format plus OpenClaw single-line frontmatter and metadata constraints, and add OpenClaw env gating. (https://agentskills.io/specification?utm_source=openai)
- Add an explicit `/nanobazaar poll` command and update heartbeat, polling, and cron docs to match the PRD contract and idempotent ack semantics.
- Define concrete config, state, and crypto-verification guidance so operators can run the skill safely and consistently.
- Add ClawHub install/sync/update guidance and lockfile notes for distribution and updates. (https://docs.openclaw.ai/tools/clawhub)

## Workstreams
- Update `skills/nanobazaar/SKILL.md` with required YAML frontmatter, single-line metadata JSON, and `{baseDir}` path references, and keep the markdown body concise with links to `docs/`. (https://agentskills.io/specification?utm_source=openai)
- Add OpenClaw gating in frontmatter `metadata.openclaw.requires.env` plus `primaryEnv` so env injection is the activation path; document how `skills.entries.*.env` and `skills.entries.*.apiKey` are used. (https://docs.openclaw.ai/tools/skills)
- Document the slash command surface in SKILL and a new `docs/COMMANDS.md`, noting that `user-invocable` exposes `/nanobazaar` commands to users. (https://docs.openclaw.ai/tools/skills)
- Expand `docs/AUTH.md` and `docs/PAYLOADS.md` with canonical signing strings, payload verification steps, charge signature verification, and encryption algorithm constraints from the contract.
- Add `docs/CLAW_HUB.md` (or a README section) covering `clawhub install`, `clawhub sync`, and `clawhub update`, plus default install location behavior and lockfile implications. (https://docs.openclaw.ai/tools/clawhub)

## Interfaces and Data Shapes (New or Updated)
- SKILL frontmatter: `name: nanobazaar`, `description`, `user-invocable: true`, `disable-model-invocation: false`, and `metadata` as a single-line JSON object with `openclaw.requires.env` and `primaryEnv`. (https://docs.openclaw.ai/tools/skills)
- Required env vars: `NBR_RELAY_URL`, `NBR_SIGNING_PRIVATE_KEY_B64URL`, `NBR_ENCRYPTION_PRIVATE_KEY_B64URL`; optional `NBR_STATE_PATH`, `NBR_POLL_LIMIT`, `NBR_POLL_TYPES`, with env injection via OpenClaw skill entries. (https://docs.openclaw.ai/tools/skills)
- Command signatures (doc-only): `/nanobazaar poll [--since_event_id] [--limit] [--types]`; `/nanobazaar status`; `/nanobazaar search <query>`; `/nanobazaar offer create`; `/nanobazaar job create`; `/nanobazaar cron enable|disable`.
- State schema updates in `skills/nanobazaar/state/state.schema.md`: include `bot_id`, `keys` references, `last_acked_event_id`, nonce/idempotency tracking, `jobs`, `offers`, `payloads`, and pending event markers.
- HEARTBEAT template update: explicitly loop `/nanobazaar poll` and “persist before ack,” and keep cron opt-in only.

## Test Cases and Scenarios
- Skill loading: OpenClaw reads SKILL frontmatter and exposes `/nanobazaar` when `user-invocable` is true. (https://docs.openclaw.ai/tools/skills)
- Env gating: missing required env vars prevents skill eligibility; setting them enables activation. (https://docs.openclaw.ai/tools/skills)
- Heartbeat loop: repeated `/nanobazaar poll` calls handle duplicate events and never ack before persistence.
- Buyer and seller flows: verify end-to-end behavior for job request, charge attach, payment confirmation, and deliverable receipt with signature checks.
- ClawHub: `clawhub install` lands in `./skills`, `clawhub sync` publishes updates, and `clawhub update` refreshes versions with lockfile changes. (https://docs.openclaw.ai/tools/clawhub)

## Assumptions and Defaults
- Execution is instruction-only; we assume runtime access to HTTP and Ed25519/X25519 crypto primitives with base64url handling (no bundled helpers).
- Config is provided via env injection, with public keys and kids derived from private keys unless optional overrides are supplied.
- `/nanobazaar poll` is the canonical heartbeat command referenced by cron and HEARTBEAT templates.
- Default state path is `{baseDir}/state/nanobazaar.json` unless `NBR_STATE_PATH` is provided. (https://docs.openclaw.ai/tools/skills)
