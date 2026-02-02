#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELAY_DIR="$ROOT_DIR/apps/relay"
FLY_TOML="$RELAY_DIR/deploy/fly.toml"

APP_NAME="${FLY_APP:-}"
REGION="${FLY_REGION:-}"
VOLUME_NAME="${FLY_VOLUME:-}"
DB_PATH="${NBR_DB_PATH:-/data/relay.db}"
DOCKERFILE="${FLY_DOCKERFILE:-Dockerfile.migrate}"
DRY_RUN="${FLY_DRY_RUN:-${DRY_RUN:-0}}"
WAIT_SECONDS="${FLY_MIGRATE_WAIT_SECONDS:-300}"
KEEP_MACHINE="${FLY_MIGRATE_KEEP_MACHINE:-0}"

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  shift
fi

if [[ -z "$APP_NAME" && -f "$FLY_TOML" ]]; then
  APP_NAME="$(awk -F '=' '/^app[[:space:]]*=/{gsub(/[[:space:]\"]/, "", $2); print $2; exit}' "$FLY_TOML")"
fi

if [[ -z "$REGION" && -f "$FLY_TOML" ]]; then
  REGION="$(awk -F '=' '/^primary_region[[:space:]]*=/{gsub(/[[:space:]\"]/, "", $2); print $2; exit}' "$FLY_TOML")"
fi

if [[ -z "$VOLUME_NAME" && -f "$FLY_TOML" ]]; then
  VOLUME_NAME="$(awk -F '=' '/^source[[:space:]]*=/{gsub(/[[:space:]\"]/, "", $2); print $2; exit}' "$FLY_TOML")"
fi

if [[ -z "$APP_NAME" ]]; then
  echo "FLY_APP not set and app not found in $FLY_TOML" >&2
  exit 1
fi

if [[ -z "$REGION" ]]; then
  echo "FLY_REGION not set and primary_region not found in $FLY_TOML" >&2
  exit 1
fi

if [[ -z "$VOLUME_NAME" ]]; then
  echo "FLY_VOLUME not set and volume source not found in $FLY_TOML" >&2
  exit 1
fi

if ! command -v fly >/dev/null 2>&1; then
  echo "fly CLI not found. Install from https://fly.io/docs/flyctl/" >&2
  exit 1
fi

echo "Running migrations on Fly:"
echo "  App: $APP_NAME"
echo "  Region: $REGION"
echo "  Volume: $VOLUME_NAME"
echo "  DB: $DB_PATH"
echo "  Dockerfile: $DOCKERFILE"
echo "  Wait seconds: $WAIT_SECONDS"
echo "  Keep machine: $KEEP_MACHINE"
echo ""
echo "Note: ensure the volume is not attached to a running machine."
echo ""

cd "$RELAY_DIR"

MACHINE_NAME="migrate-$(date +%Y%m%d%H%M%S)"
CMD=(
  fly machine run .
  --app "$APP_NAME"
  --region "$REGION"
  --volume "${VOLUME_NAME}:/data"
  --dockerfile "$DOCKERFILE"
  --restart "no"
  --detach
  --name "$MACHINE_NAME"
  --env "NBR_DB_PATH=$DB_PATH"
)

if [[ "$DRY_RUN" == "1" ]]; then
  printf "Dry run: "
  printf "%q " "${CMD[@]}"
  printf "\n"
  exit 0
fi

OUTPUT="$("${CMD[@]}" 2>&1)" || {
  printf "%s\n" "$OUTPUT"
  exit 1
}
printf "%s\n" "$OUTPUT"

MACHINE_ID="$(printf "%s\n" "$OUTPUT" | awk '/Machine ID:/ {print $3; exit}')"
if [[ -z "$MACHINE_ID" ]]; then
  echo "Failed to parse Machine ID from fly output." >&2
  exit 1
fi

echo ""
echo "Waiting for migration machine to stop..."
DEADLINE=$((SECONDS + WAIT_SECONDS))
LAST_STATUS=""
while true; do
  STATUS_OUT="$(fly machine status "$MACHINE_ID" -a "$APP_NAME" 2>&1 || true)"
  LAST_STATUS="$STATUS_OUT"
  STATE="$(printf "%s\n" "$STATUS_OUT" | awk -F': ' '/State:/ {print $2; exit}')"
  EXIT_STATUS="$(printf "%s\n" "$STATUS_OUT" | awk -F': ' '/Exit status:/ {print $2; exit} /Exit code:/ {print $2; exit}')"

  if [[ "$STATE" == "stopped" || "$STATE" == "destroyed" ]]; then
    if [[ -z "$EXIT_STATUS" ]]; then
      echo "Could not determine migration exit status." >&2
      printf "%s\n" "$STATUS_OUT"
      exit 1
    fi
    if [[ "$EXIT_STATUS" != "0" ]]; then
      echo "Migration machine exited with status $EXIT_STATUS." >&2
      printf "%s\n" "$STATUS_OUT"
      echo "Keeping machine for inspection: $MACHINE_ID"
      exit 1
    fi
    break
  fi

  if (( SECONDS >= DEADLINE )); then
    echo "Timed out waiting for migration machine to stop." >&2
    printf "%s\n" "$STATUS_OUT"
    exit 1
  fi
  sleep 2
done

if [[ "$KEEP_MACHINE" == "1" ]]; then
  echo "Keeping migration machine: $MACHINE_ID"
  exit 0
fi

fly machine destroy -a "$APP_NAME" -f "$MACHINE_ID"
