#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELAY_DIR="$ROOT_DIR/apps/relay"
FLY_TOML="$RELAY_DIR/deploy/fly.toml"

APP_NAME="${FLY_APP:-}"
DEPLOY_CONFIG="${FLY_DEPLOY_CONFIG:-$FLY_TOML}"
DRY_RUN="${FLY_DRY_RUN:-${DRY_RUN:-0}}"

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  shift
fi

if [[ -z "$APP_NAME" && -f "$FLY_TOML" ]]; then
  APP_NAME="$(awk -F '=' '/^app[[:space:]]*=/{gsub(/[[:space:]\"]/, "", $2); print $2; exit}' "$FLY_TOML")"
fi

if [[ -z "$APP_NAME" ]]; then
  echo "FLY_APP not set and app not found in $FLY_TOML" >&2
  exit 1
fi

if ! command -v fly >/dev/null 2>&1; then
  echo "fly CLI not found. Install from https://fly.io/docs/flyctl/" >&2
  exit 1
fi

echo "Fly deploy:"
echo "  App: $APP_NAME"
echo "  Deploy config: $DEPLOY_CONFIG"
echo ""

if [[ "$DRY_RUN" == "1" ]]; then
  echo "Dry run: (cd $RELAY_DIR && fly deploy --config $DEPLOY_CONFIG --app $APP_NAME)"
  exit 0
fi

(
  cd "$RELAY_DIR"
  fly deploy --config "$DEPLOY_CONFIG" --app "$APP_NAME"
)
