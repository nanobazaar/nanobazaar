#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${NBR_DB_PATH:-./data/relay.db}"
STAMP="$(date +"%Y%m%d_%H%M%S")"
DEST="${1:-./backups/relay_${STAMP}.db}"

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required for a WAL-safe live backup." >&2
  exit 1
fi

if [[ ! -f "$DB_PATH" ]]; then
  echo "SQLite database not found: $DB_PATH" >&2
  exit 1
fi

case "$DEST" in
  *"'"*|*$'\n'*|*$'\r'*)
    echo "Backup destination cannot contain quotes or newlines: $DEST" >&2
    exit 1
    ;;
esac

DEST_DIR="$(dirname "$DEST")"
mkdir -p "$DEST_DIR"
DEST_DIR="$(cd "$DEST_DIR" && pwd)"
DEST="$DEST_DIR/$(basename "$DEST")"

if [[ -e "$DEST" ]]; then
  echo "Refusing to overwrite existing backup: $DEST" >&2
  exit 1
fi

sqlite3 "$DB_PATH" ".backup '$DEST'"

INTEGRITY="$(sqlite3 "$DEST" "PRAGMA integrity_check;")"
if [[ "$INTEGRITY" != "ok" ]]; then
  rm -f "$DEST"
  echo "Backup integrity_check failed: $INTEGRITY" >&2
  exit 1
fi

printf "Backup written to %s\n" "$DEST"
printf "Backup integrity_check: ok\n"
