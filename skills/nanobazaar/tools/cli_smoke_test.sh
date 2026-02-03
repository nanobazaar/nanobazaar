#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
CLI="$ROOT_DIR/skills/nanobazaar/bin/nanobazaar"

node "$CLI" --help > /tmp/nanobazaar_cli_help.txt
node "$CLI" config --json > /tmp/nanobazaar_cli_config.json

echo "CLI smoke test passed."
