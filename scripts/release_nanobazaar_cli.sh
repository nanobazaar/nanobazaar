#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGE_DIR="$ROOT_DIR/packages/nanobazaar-cli"
VERSION="${1:-}"
MODE="${2:-check}"
CONFIRM="${3:-}"

cd "$ROOT_DIR"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/release_nanobazaar_cli.sh <version> check
  bash scripts/release_nanobazaar_cli.sh <version> npm --confirm-publish
  bash scripts/release_nanobazaar_cli.sh <version> github --confirm-release

"check" is local and may run from a dirty feature branch. Publishing requires a
clean main checkout exactly at origin/main. Run npm first, then GitHub.
EOF
}

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
  usage >&2
  exit 1
fi
if [[ "$MODE" != "check" && "$MODE" != "npm" && "$MODE" != "github" ]]; then
  usage >&2
  exit 1
fi

PACKAGE_VERSION="$(node -p "require('$PACKAGE_DIR/package.json').version")"
if [[ "$PACKAGE_VERSION" != "$VERSION" ]]; then
  echo "CLI package version is $PACKAGE_VERSION; expected $VERSION." >&2
  exit 1
fi
if ! grep -Fq "## [$VERSION] - " "$PACKAGE_DIR/CHANGELOG.md"; then
  echo "CLI changelog has no $VERSION release section." >&2
  exit 1
fi

if [[ "$MODE" == "check" ]]; then
  exec bash "$ROOT_DIR/scripts/verify_nanobazaar_release.sh" "$VERSION"
fi

if [[ "$(git -C "$ROOT_DIR" branch --show-current)" != "main" ]]; then
  echo "Publishing requires the main branch." >&2
  exit 1
fi
if [[ -n "$(git -C "$ROOT_DIR" status --porcelain --untracked-files=all)" ]]; then
  echo "Publishing requires a clean working tree." >&2
  exit 1
fi
git -C "$ROOT_DIR" fetch origin main
if [[ "$(git -C "$ROOT_DIR" rev-parse HEAD)" != "$(git -C "$ROOT_DIR" rev-parse origin/main)" ]]; then
  echo "Publishing requires HEAD to equal origin/main." >&2
  exit 1
fi

TAG="nanobazaar-cli@$VERSION"
if [[ "$MODE" == "npm" ]]; then
  if [[ "$CONFIRM" != "--confirm-publish" ]]; then
    echo "Refusing npm publication without --confirm-publish." >&2
    exit 1
  fi
  npm whoami >/dev/null
  if npm view "nanobazaar-cli@$VERSION" version >/dev/null 2>&1; then
    echo "nanobazaar-cli@$VERSION is already published." >&2
    exit 1
  fi
  PUBLISH_TMP="$(mktemp -d "${TMPDIR:-/tmp}/nanobazaar-publish-${VERSION}.XXXXXX")"
  ARTIFACT_DIR="$PUBLISH_TMP/artifacts"
  trap 'rm -rf "$PUBLISH_TMP"' EXIT
  NBR_RELEASE_ARTIFACT_DIR="$ARTIFACT_DIR" \
    bash "$ROOT_DIR/scripts/verify_nanobazaar_release.sh" "$VERSION" --keep-artifacts
  TARBALLS=("$ARTIFACT_DIR/package"/*.tgz)
  if [[ "${#TARBALLS[@]}" != "1" || ! -f "${TARBALLS[0]}" ]]; then
    echo "Verified npm archive not found." >&2
    exit 1
  fi
  npm publish --access public "${TARBALLS[0]}"
  exit 0
fi

if [[ "$CONFIRM" != "--confirm-release" ]]; then
  echo "Refusing GitHub release without --confirm-release." >&2
  exit 1
fi
if [[ "$(npm view "nanobazaar-cli@$VERSION" version)" != "$VERSION" ]]; then
  echo "Publish nanobazaar-cli@$VERSION to npm before the GitHub release." >&2
  exit 1
fi
if git -C "$ROOT_DIR" ls-remote --exit-code --tags origin "refs/tags/$TAG" >/dev/null 2>&1; then
  echo "Remote tag already exists: $TAG" >&2
  exit 1
fi
if gh release view "$TAG" >/dev/null 2>&1; then
  echo "GitHub release already exists: $TAG" >&2
  exit 1
fi
gh auth status >/dev/null
VERIFIED_SHA="$(git -C "$ROOT_DIR" rev-parse HEAD)"
NOTES_FILE="$(mktemp "${TMPDIR:-/tmp}/nanobazaar-release-notes.XXXXXX")"
trap 'rm -f "$NOTES_FILE"' EXIT
awk -v ver="$VERSION" '
  $0 ~ ("^## \\[" ver "\\] - ") { in_release=1; next }
  in_release && /^## \[/ { exit }
  in_release { print }
' "$PACKAGE_DIR/CHANGELOG.md" > "$NOTES_FILE"
if [[ ! -s "$NOTES_FILE" ]]; then
  echo "Could not extract GitHub release notes." >&2
  exit 1
fi
gh release create "$TAG" --target "$VERIFIED_SHA" --title "nanobazaar-cli $VERSION" --notes-file "$NOTES_FILE"
