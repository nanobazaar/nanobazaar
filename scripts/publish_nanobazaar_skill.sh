#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

VERSION=""
while [[ -z "${VERSION}" ]]; do
  read -r -p "Version to publish (required): " VERSION
  if [[ -z "${VERSION}" ]]; then
    echo "Version cannot be empty."
  fi
done

echo "Paste changelog text (optional). Finish with a single line containing only END."
echo "Leave empty and type END to skip."
CHANGELOG_LINES=()
while IFS= read -r line; do
  if [[ "${line}" == "END" ]]; then
    break
  fi
  CHANGELOG_LINES+=("${line}")
done

CHANGELOG_FILE=""
cleanup() {
  if [[ -n "${CHANGELOG_FILE}" ]] && [[ -f "${CHANGELOG_FILE}" ]]; then
    rm -f "${CHANGELOG_FILE}"
  fi
}
trap cleanup EXIT

if [[ ${#CHANGELOG_LINES[@]} -gt 0 ]]; then
  CHANGELOG_FILE="$(mktemp)"
  printf "%s\n" "${CHANGELOG_LINES[@]}" > "${CHANGELOG_FILE}"
fi

PUBLISH_CMD=(
  npx clawhub publish
  --slug nanobazaar
  --name "NanoBazaar"
  --version "${VERSION}"
)
if [[ -n "${CHANGELOG_FILE}" ]]; then
  PUBLISH_CMD+=(--changelog "${CHANGELOG_FILE}")
fi
PUBLISH_CMD+=(./skills/nanobazaar)

"${PUBLISH_CMD[@]}" \
