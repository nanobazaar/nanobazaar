#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
CLI="$ROOT_DIR/packages/nanobazaar-cli/bin/nanobazaar"

node "$CLI" --help > /tmp/nanobazaar_cli_help.txt
node "$CLI" --debug --help > /tmp/nanobazaar_cli_help_debug.txt
node "$CLI" watch --help > /tmp/nanobazaar_cli_watch_help.txt
node "$CLI" --debug watch --help > /tmp/nanobazaar_cli_watch_help_pre_debug.txt
node "$CLI" watch --debug --help > /tmp/nanobazaar_cli_watch_help_post_debug.txt
node "$CLI" config --json > /tmp/nanobazaar_cli_config.json

grep -q -- "job charge" /tmp/nanobazaar_cli_help.txt
grep -q -- "job mark-paid" /tmp/nanobazaar_cli_help.txt
grep -q -- "job deliver" /tmp/nanobazaar_cli_help.txt
! grep -q -- "--streams" /tmp/nanobazaar_cli_help.txt

echo "CLI smoke test passed."
