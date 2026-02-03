# NanoBazaar Skill CLI Plan (npm)

## Goal

Make the NanoBazaar CLI a first-class installable tool so OpenClaw can gate on it and install it automatically. The CLI is required for `/nanobazaar` command execution and watcher support.

## Decisions

- Package name: `@nanobazaar/cli` (npm)
- Binary name: `nanobazaar`
- Runtime: Node.js 18+ (required for built-in `fetch` and crypto)

## OpenClaw metadata (skill frontmatter)

Add OpenClaw metadata so the skill is only eligible when the CLI is installed, and the macOS Skills UI can install it:

- `metadata.openclaw.requires.bins: ["nanobazaar"]`
- `metadata.openclaw.install`: node installer entry for `@nanobazaar/cli`

Example shape (align with OpenClaw docs):

```
metadata:
  openclaw:
    primaryEnv: NBR_SIGNING_PRIVATE_KEY_B64URL
    requires:
      bins: [nanobazaar]
    install:
      - id: node
        kind: node
        package: "@nanobazaar/cli"
        bins: [nanobazaar]
        label: "Install NanoBazaar CLI (npm)"
```

Note: BerryPay stays optional and must not be added to `requires.bins`.

## Packaging steps

1. Create a publishable npm package for the CLI:
   - Set `name: "@nanobazaar/cli"` and `bin: { "nanobazaar": "./bin/nanobazaar" }`.
   - Include `libsodium-wrappers` and any other runtime dependencies.
   - Ship only the CLI sources needed at runtime.

2. Align the CLI state path with docs:
   - Default to `${XDG_CONFIG_HOME:-~/.config}/nanobazaar/nanobazaar.json`.

3. Replace local usage docs:
   - Update examples from `./bin/nanobazaar` to `nanobazaar`.
   - Add npm install instructions (`npm i -g @nanobazaar/cli`) where appropriate.

4. Ensure sandbox compatibility:
   - If sandboxed runs are expected, add a note or setup command to install `@nanobazaar/cli` inside the container.

## Non-goals

- Do not auto-install BerryPay.
- Do not change `/poll` semantics as part of CLI packaging.

## Risks

- Incorrect OpenClaw metadata breaks eligibility or install UX.
- Divergence between local docs and actual install path.
