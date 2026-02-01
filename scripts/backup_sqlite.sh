#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${NBR_DB_PATH:-./data/relay.db}"
STAMP="$(date +"%Y%m%d_%H%M%S")"
DEST="${1:-./backups/relay_${STAMP}.db}"

mkdir -p "$(dirname "$DEST")"

if command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 "$DB_PATH" ".backup '$DEST'"
else
  cp "$DB_PATH" "$DEST"
fi

printf "Backup written to %s\n" "$DEST"
