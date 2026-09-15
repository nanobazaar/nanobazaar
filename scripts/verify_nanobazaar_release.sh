#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGE_DIR="$ROOT_DIR/packages/nanobazaar-cli"
SKILL_DIR="$ROOT_DIR/skills/nanobazaar"
VERSION="${1:-3.0.0}"
KEEP_ARTIFACTS=0
SKIP_DOCKER=0

shift $(( $# > 0 ? 1 : 0 ))
for option in "$@"; do
  case "$option" in
    --keep-artifacts) KEEP_ARTIFACTS=1 ;;
    --skip-docker) SKIP_DOCKER=1 ;;
    *)
      echo "Usage: bash scripts/verify_nanobazaar_release.sh [version] [--keep-artifacts] [--skip-docker]" >&2
      exit 1
      ;;
  esac
done

for command_name in node npm go tar curl; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Required command not found: $command_name" >&2
    exit 1
  fi
done
if [[ "$SKIP_DOCKER" != "1" ]] && { ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; }; then
  echo "Docker is required for the relay image and Node 18 checks. Pass --skip-docker only to request a partial local check." >&2
  exit 1
fi

if [[ -n "${NBR_RELEASE_ARTIFACT_DIR:-}" ]]; then
  ARTIFACT_DIR="$NBR_RELEASE_ARTIFACT_DIR"
  if [[ -e "$ARTIFACT_DIR" ]]; then
    echo "NBR_RELEASE_ARTIFACT_DIR already exists: $ARTIFACT_DIR" >&2
    exit 1
  fi
  mkdir -p "$ARTIFACT_DIR"
else
  ARTIFACT_DIR="$(mktemp -d "${TMPDIR:-/tmp}/nanobazaar-release-${VERSION}.XXXXXX")"
fi

CONTAINER_ID=""
DOCKER_VOLUME=""
DOCKER_IMAGE=""
cleanup() {
  if [[ -n "$CONTAINER_ID" ]]; then docker rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true; fi
  if [[ -n "$DOCKER_VOLUME" ]]; then docker volume rm "$DOCKER_VOLUME" >/dev/null 2>&1 || true; fi
  if [[ -n "$DOCKER_IMAGE" ]]; then docker image rm "$DOCKER_IMAGE" >/dev/null 2>&1 || true; fi
  if [[ "$KEEP_ARTIFACTS" != "1" ]]; then rm -rf "$ARTIFACT_DIR"; fi
}
trap cleanup EXIT

PACKAGE_VERSION="$(node -p "require('$PACKAGE_DIR/package.json').version")"
SKILL_VERSION="$(node -p "require('$SKILL_DIR/skill.json').version")"
if [[ "$PACKAGE_VERSION" != "$VERSION" || "$SKILL_VERSION" != "$VERSION" ]]; then
  echo "Version mismatch: CLI=$PACKAGE_VERSION skill=$SKILL_VERSION expected=$VERSION" >&2
  exit 1
fi
if ! grep -Fq "npm install -g nanobazaar-cli@$VERSION" "$SKILL_DIR/SKILL.md"; then
  echo "Standalone skill does not require nanobazaar-cli@$VERSION." >&2
  exit 1
fi

mkdir -p "$ARTIFACT_DIR/package" "$ARTIFACT_DIR/install" "$ARTIFACT_DIR/bin"
(
  cd "$PACKAGE_DIR"
  npm pack --pack-destination "$ARTIFACT_DIR/package" >/dev/null
)
TARBALLS=("$ARTIFACT_DIR/package"/*.tgz)
if [[ "${#TARBALLS[@]}" != "1" || ! -f "${TARBALLS[0]}" ]]; then
  echo "Expected one packed CLI archive." >&2
  exit 1
fi
TARBALL="${TARBALLS[0]}"

CONTENTS="$(tar -tzf "$TARBALL")"
for required_path in package/package.json package/bin/nanobazaar package/lib/journal.js package/lib/payments.js package/tools/setup.js package/README.md package/CHANGELOG.md; do
  if ! grep -Fxq "$required_path" <<<"$CONTENTS"; then
    echo "Packed CLI is missing $required_path" >&2
    exit 1
  fi
done
if grep -Eq '(^|/)(test|node_modules)/' <<<"$CONTENTS"; then
  echo "Packed CLI unexpectedly contains tests or node_modules." >&2
  exit 1
fi

npm install --prefix "$ARTIFACT_DIR/install" --ignore-scripts --no-audit --no-fund "$TARBALL" >/dev/null
CLI_BIN="$ARTIFACT_DIR/install/node_modules/.bin/nanobazaar"
if [[ ! -x "$CLI_BIN" ]]; then
  echo "Packed CLI binary was not installed: $CLI_BIN" >&2
  exit 1
fi
if [[ "$("$CLI_BIN" --version)" != "$VERSION" ]]; then
  echo "Installed CLI did not report version $VERSION." >&2
  exit 1
fi
"$CLI_BIN" --help >/dev/null

if [[ "$(uname -s)" == "Darwin" ]] && command -v xcrun >/dev/null 2>&1; then
  RELEASE_SDKROOT="${SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"
  export SDKROOT="$RELEASE_SDKROOT"
  export CGO_CFLAGS="${CGO_CFLAGS:--isysroot $RELEASE_SDKROOT}"
  export CGO_LDFLAGS="${CGO_LDFLAGS:--isysroot $RELEASE_SDKROOT}"
fi
(
  cd "$ROOT_DIR/apps/relay"
  go build -tags=sqlite_fts5 -o "$ARTIFACT_DIR/bin/relay" ./cmd/relay
)
(
  cd "$PACKAGE_DIR"
  NBR_TEST_RELAY_BIN="$ARTIFACT_DIR/bin/relay" \
  NBR_TEST_CLI_BIN="$CLI_BIN" \
    node --test test/relay-commerce.test.js
)

if [[ "$SKIP_DOCKER" != "1" ]]; then
  DOCKER_IMAGE="nanobazaar-relay-release-check-$RANDOM-$$"
  DOCKER_VOLUME="nanobazaar-relay-release-check-$RANDOM-$$"
  docker build --platform linux/amd64 --tag "$DOCKER_IMAGE" "$ROOT_DIR/apps/relay" >/dev/null
  docker volume create "$DOCKER_VOLUME" >/dev/null
  CONTAINER_ID="$(docker run --platform linux/amd64 --detach --rm \
    --env NBR_DB_PATH=/data/relay.db \
    --env NBR_MIGRATE_ON_START=true \
    --env NBR_HEALTH_PUBLIC=true \
    --env NBR_RETENTION_ENABLED=false \
    --publish 127.0.0.1::8080 \
    --volume "$DOCKER_VOLUME:/data" \
    "$DOCKER_IMAGE")"
  PORT_LINE="$(docker port "$CONTAINER_ID" 8080/tcp)"
  HOST_PORT="${PORT_LINE##*:}"
  READY=0
  for _ in $(seq 1 100); do
    if curl --fail --silent "http://127.0.0.1:$HOST_PORT/healthz" >/dev/null 2>&1; then
      READY=1
      break
    fi
    sleep 0.1
  done
  if [[ "$READY" != "1" ]]; then
    docker logs "$CONTAINER_ID" >&2 || true
    echo "Relay image did not become healthy." >&2
    exit 1
  fi
  SEARCH_JSON="$(curl --fail --silent "http://127.0.0.1:$HOST_PORT/market/offers?q=release")"
  node -e 'const value=JSON.parse(process.argv[1]); if (!Array.isArray(value.offers)) process.exit(1)' "$SEARCH_JSON"

  NODE18_OUTPUT="$(docker run --platform linux/amd64 --rm --volume "$ARTIFACT_DIR/install:/release:ro" node:18-bookworm-slim \
    node /release/node_modules/nanobazaar-cli/examples/sealed-box.mjs)"
  if [[ "$NODE18_OUTPUT" != "NanoBazaar encrypted delivery" ]]; then
    echo "Packed CLI encryption example failed under Node 18: $NODE18_OUTPUT" >&2
    exit 1
  fi
else
  echo "Partial verification requested: relay image and Node 18 checks skipped." >&2
fi

if [[ "$SKIP_DOCKER" == "1" ]]; then
  echo "Partial release verification passed for $VERSION (Docker checks skipped)."
else
  echo "Release verification passed for $VERSION."
fi
echo "Packed archive: $TARBALL"
echo "Isolated CLI: $CLI_BIN"
echo "Fixture relay: $ARTIFACT_DIR/bin/relay"
if [[ "$KEEP_ARTIFACTS" == "1" ]]; then
  echo "Artifacts retained: $ARTIFACT_DIR"
fi
