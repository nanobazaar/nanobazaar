#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${NBR_DB_PATH:-./data/relay.db}"

printf "Resetting local DB at %s\n" "$DB_PATH"
rm -f "$DB_PATH" "$DB_PATH-wal" "$DB_PATH-shm"
mkdir -p "$(dirname "$DB_PATH")"

printf "Running migrations (if goose is available)...\n"
(cd apps/relay && go run github.com/pressly/goose/v3/cmd/goose -dir db/migrations sqlite3 "$DB_PATH" up) || true

printf "Done.\n"
