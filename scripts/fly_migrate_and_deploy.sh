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
DEPLOY_CONFIG="${FLY_DEPLOY_CONFIG:-$FLY_TOML}"
DRY_RUN="${FLY_DRY_RUN:-${DRY_RUN:-0}}"
YES="${FLY_YES:-${YES:-0}}"

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  shift
fi

if [[ "${1:-}" == "--yes" ]]; then
  YES=1
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

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 not found; required for JSON parsing." >&2
  exit 1
fi

echo "Fly migrate + deploy:"
echo "  App: $APP_NAME"
echo "  Region: $REGION"
echo "  Volume: $VOLUME_NAME"
echo "  DB: $DB_PATH"
echo "  Dockerfile: $DOCKERFILE"
echo "  Deploy config: $DEPLOY_CONFIG"
echo ""

VOL_JSON="$(fly volumes list -a "$APP_NAME" -j)"
VOL_INFO="$(
python3 - <<'PY' "$VOL_JSON" "$VOLUME_NAME"
import json, sys
data = json.loads(sys.argv[1])
name = sys.argv[2]
for vol in data:
    if vol.get("name") == name:
        print(json.dumps(vol))
        break
else:
    print("")
PY
)"

if [[ -z "$VOL_INFO" ]]; then
  echo "Volume '$VOLUME_NAME' not found for app '$APP_NAME'." >&2
  exit 1
fi

ATTACHED_MACHINE="$(
python3 - <<'PY' "$VOL_INFO"
import json, sys
data = json.loads(sys.argv[1])
print(data.get("attached_machine_id") or "")
PY
)"

if [[ -n "$ATTACHED_MACHINE" ]]; then
  echo "Volume is attached to machine: $ATTACHED_MACHINE"
  if [[ "$YES" != "1" && "$DRY_RUN" != "1" ]]; then
    printf "Destroy machine %s to detach volume? [y/N] " "$ATTACHED_MACHINE"
    read -r RESP
    if [[ "$RESP" != "y" && "$RESP" != "Y" ]]; then
      echo "Aborting."
      exit 1
    fi
  fi

  if [[ "$DRY_RUN" == "1" ]]; then
    echo "Dry run: fly machine stop $ATTACHED_MACHINE -a $APP_NAME --wait-timeout 1m"
    echo "Dry run: fly machine destroy $ATTACHED_MACHINE -a $APP_NAME --force"
  else
    fly machine stop "$ATTACHED_MACHINE" -a "$APP_NAME" --wait-timeout 1m || true
    fly machine destroy "$ATTACHED_MACHINE" -a "$APP_NAME" --force
  fi

  if [[ "$DRY_RUN" != "1" ]]; then
    VOL_JSON="$(fly volumes list -a "$APP_NAME" -j)"
    DETACHED="$(
    python3 - <<'PY' "$VOL_JSON" "$VOLUME_NAME"
import json, sys
data = json.loads(sys.argv[1])
name = sys.argv[2]
for vol in data:
    if vol.get("name") == name:
        print("1" if not vol.get("attached_machine_id") else "0")
        break
else:
    print("0")
PY
    )"
    if [[ "$DETACHED" != "1" ]]; then
      echo "Volume is still attached; aborting before migration." >&2
      exit 1
    fi
  fi
else
  echo "Volume is not attached to any machine."
fi

if [[ "$DRY_RUN" == "1" ]]; then
  echo "Dry run: scripts/fly_migrate.sh --dry-run"
  echo "Dry run: (cd $RELAY_DIR && fly deploy --config $DEPLOY_CONFIG --app $APP_NAME)"
  exit 0
fi

FLY_APP="$APP_NAME" \
FLY_REGION="$REGION" \
FLY_VOLUME="$VOLUME_NAME" \
NBR_DB_PATH="$DB_PATH" \
FLY_DOCKERFILE="$DOCKERFILE" \
  "$ROOT_DIR/scripts/fly_migrate.sh"

(
  cd "$RELAY_DIR"
  fly deploy --config "$DEPLOY_CONFIG" --app "$APP_NAME"
)
