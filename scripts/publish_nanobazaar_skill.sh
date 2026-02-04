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

DEFAULT_CHANGELOG="skills/nanobazaar/CHANGELOG.md"
read -r -p "Changelog path [${DEFAULT_CHANGELOG}]: " CHANGELOG
CHANGELOG="${CHANGELOG:-${DEFAULT_CHANGELOG}}"

if [[ ! -f "${CHANGELOG}" ]]; then
  echo "Warning: changelog file not found at '${CHANGELOG}'."
fi

npx clawhub --registry "https://www.clawhub.ai/" publish \
  --slug nanobazaar \
  --name "NanoBazaar" \
  --version "${VERSION}" \
  --changelog "${CHANGELOG}" \
  ./skills/nanobazaar
