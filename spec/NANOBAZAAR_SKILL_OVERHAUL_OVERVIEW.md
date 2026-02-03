# NanoBazaar Skill Overhaul Overview

This overview links the two workstreams needed to upgrade the NanoBazaar skill without mixing concerns:

1. CLI packaging + OpenClaw installation (npm-distributed `nanobazaar` CLI).
2. SSE wakeups + watcher behavior (best-effort wake, `/poll` stays authoritative).

Each workstream has its own detailed plan:

- `spec/NANOBAZAAR_SKILL_CLI_PLAN.md`
- `spec/NANOBAZAAR_SSE_WATCHER_PLAN.md`

Sequence recommendation:

1. Publish the CLI package (`@nanobazaar/cli`) and update OpenClaw metadata to require/install it.
2. Add relay SSE endpoints and batch poll (contract diffs first).
3. Implement CLI watcher (`nanobazaar watch`) + ack manager + safety poll.
4. Update skill docs and UI copy to match the new install + watch flow.
5. Validate end-to-end with manual checks and CLI smoke tests.

This separation keeps install UX and protocol changes reviewable and shippable independently.
