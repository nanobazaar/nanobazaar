# NanoBazaar 3.0.0 release readiness

Scope: the authorized changes on `codex/durable-agent-commerce`: durable client
recovery, wallet-neutral payment handoff, polling retention, seller contact, public
UI fixes, Inter body text, and release preparation. Frozen contract artifacts stay
unchanged; the backend clarifications are recorded in `CONTRACT_DIFF.md`.

## Verified before release preparation

- Full CLI suite: 35 passing tests, including a local authenticated relay trade
  with encrypted payloads and a simulated Nano RPC. No real payment was made.
- Web typecheck and production build passed after the wallet-neutral copy fixes.
- Full relay `make test lint` passed on 2026-09-15 with SQLite FTS5 enabled.
- `git diff --check` passed.
- `npm pack --dry-run --json` included the CLI entrypoint, setup, journal,
  payments module, examples and documentation; it excluded tests and state files.
- Migration tests exercise version 9 -> 10 -> 9 and compare all commerce rows,
  including payment evidence, ciphertext and retained events. They also cover
  transactional deletion watermarks and concurrent poll snapshots. This is
  fixture evidence, not a production-database rehearsal.

## Live baseline, 2026-09-15

- `https://relay.nanobazaar.ai/healthz`: HTTP 200.
- Public offers: HTTP 200; sampled offer did not contain `seller_last_seen_at`.
- npm registry reports CLI version `2.0.6`.
- Vercel project: `nanobazaar`, scope `madsbdotcom`, project ID
  `prj_R7vM9xW5wQiP72CTB9gA9bY7Zews`, root directory `apps/web`, Node 24.
- Existing production deployment: `dpl_qXnUU9owK6UgL6dTdiVF9PHRTdqE`, Ready,
  `https://nanobazaar-a2wgd5kty-madsbdotcom.vercel.app`.
- GitHub, Vercel and ClawHub CLI authentication worked. Fly had no access token;
  npm authentication returned E401. User was asked to restore those logins.

## Required release evidence

- Correct production Docker image: migration and full-text search smoke check.
- CLI and skill 3.0.0 metadata and standalone installation instructions.
- Isolated installation and testing of the packed npm artifact.
- Explicit, guarded npm/GitHub/ClawHub release steps.
- Verified production backup and migration rehearsal on a separate copy before
  backend deploy. Do not use the old machine-destruction deployment shortcut.
- Confirm published CLI/skill before publishing website instructions that require
  them. Verify live health, search, polling and seller-contact output after deploy.
- A real external-wallet transfer is not covered by the simulated RPC tests and
  requires a specific wallet and operator-approved payment budget.

## Release-preparation checks

- Final full CLI suite passed: 36/36 tests, including the new release guards.
- Full release-artifact verifier passed: isolated CLI 3.0.0 completed an
  authenticated trade against a fresh relay with simulated Nano RPC; the
  production Linux amd64 Docker image started, migrated and served FTS search;
  the packed sealed-box encryption example passed under Node 18.
- Final website unit tests passed (4/4), along with `pnpm lint` and
  `pnpm build` after the 3.0.0 installation-copy changes.
- Release helpers passed shell syntax checks and refused npm/GitHub/skill
  publication from the feature branch. An incorrect release version was
  rejected. Checks were also invoked from outside the repository.
- The new backup helper passed an independent test against an open SQLite WAL
  database. Committed WAL data appeared in the backup, a destination containing
  spaces worked, and a repeated backup could not overwrite the existing file.
- An independent Sol agent used only a copied standalone skill and an isolated
  npm installation reporting version 3.0.0. Local-only setup, status and JSON
  inspection commands worked. Documented charge/delivery argument variants were
  accepted. The buyer/seller instructions were followable without repository
  source; no blocking ambiguity was found. This was an instruction/CLI check,
  not a real external-wallet trade.
- Vercel's current Git integration publishes `main` directly to production.
  Its ignored-build setting currently allows production builds and skips other
  builds. Preserve and temporarily pause that setting before merging this
  release; restore it after the ordered backend/npm/skill/web rollout. No Vercel
  setting has been changed during preparation.
