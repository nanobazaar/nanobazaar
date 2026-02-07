#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

usage() {
  cat <<'EOF'
Usage:
  scripts/release_nanobazaar_cli.sh <version> [--dry-run]

What it does:
  - Validates packages/nanobazaar-cli/package.json version matches <version>
  - Validates packages/nanobazaar-cli/CHANGELOG.md contains a section for <version>
  - Creates an annotated git tag: nanobazaar-cli@<version>
  - Pushes the tag to origin
  - Creates a GitHub release for the tag using the changelog section as notes

Notes:
  - Run this after the release commit is merged to main.
  - Requires: git, gh (GitHub CLI), and authenticated gh session.
EOF
}

VERSION="${1:-}"
if [[ -z "${VERSION}" ]] || [[ "${VERSION}" == "-h" ]] || [[ "${VERSION}" == "--help" ]]; then
  usage
  exit 1
fi

DRY_RUN=0
if [[ "${2:-}" == "--dry-run" ]]; then
  DRY_RUN=1
fi

TAG="nanobazaar-cli@${VERSION}"
PKG_JSON="packages/nanobazaar-cli/package.json"
CHANGELOG="packages/nanobazaar-cli/CHANGELOG.md"

if [[ ! -f "${PKG_JSON}" ]]; then
  echo "Missing ${PKG_JSON}"
  exit 1
fi
if [[ ! -f "${CHANGELOG}" ]]; then
  echo "Missing ${CHANGELOG}"
  exit 1
fi

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [[ "${CURRENT_BRANCH}" != "main" ]]; then
  echo "Refusing to tag from non-main branch (current: ${CURRENT_BRANCH}). Checkout main and ensure it's up to date."
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree not clean. Commit or stash changes before releasing."
  exit 1
fi

PKG_VERSION="$(node -e "console.log(JSON.parse(require('fs').readFileSync('${PKG_JSON}','utf8')).version)")"
if [[ "${PKG_VERSION}" != "${VERSION}" ]]; then
  echo "${PKG_JSON} version (${PKG_VERSION}) does not match requested version (${VERSION})."
  exit 1
fi

if ! rg -n "^## \\[${VERSION//./\\.}\\] - " "${CHANGELOG}" >/dev/null; then
  echo "No changelog entry found for ${VERSION} in ${CHANGELOG}."
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  echo "Tag already exists: ${TAG}"
  exit 1
fi

NOTES_FILE="$(mktemp)"
cleanup() { rm -f "${NOTES_FILE}"; }
trap cleanup EXIT

# Extract notes for this version: everything after the version heading until the next version heading.
awk -v ver="${VERSION}" '
  BEGIN { in=0; }
  $0 ~ ("^## \\[" ver "\\] - ") { in=1; next }
  in && $0 ~ /^## \[/ { exit }
  in { print }
' "${CHANGELOG}" | sed -e 's/[[:space:]]*$//' > "${NOTES_FILE}"

if [[ ! -s "${NOTES_FILE}" ]]; then
  echo "Failed to extract release notes for ${VERSION} from ${CHANGELOG}."
  exit 1
fi

if [[ "${DRY_RUN}" == "1" ]]; then
  echo "Dry run ok."
  echo "Would run:"
  echo "  git tag -a \"${TAG}\" -m \"nanobazaar-cli ${VERSION}\""
  echo "  git push origin \"${TAG}\""
  echo "  gh release create \"${TAG}\" --title \"nanobazaar-cli ${VERSION}\" --notes-file \"${NOTES_FILE}\""
  exit 0
fi

git tag -a "${TAG}" -m "nanobazaar-cli ${VERSION}"
git push origin "${TAG}"

gh release create "${TAG}" \
  --title "nanobazaar-cli ${VERSION}" \
  --notes-file "${NOTES_FILE}"

