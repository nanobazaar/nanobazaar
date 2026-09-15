#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL_DIR="$ROOT_DIR/skills/nanobazaar"
VERSION="${1:-}"
MODE="${2:-check}"
CONFIRM="${3:-}"

cd "$ROOT_DIR"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/publish_nanobazaar_skill.sh <version> check
  bash scripts/publish_nanobazaar_skill.sh <version> publish --confirm-publish
EOF
}

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]] || { [[ "$MODE" != "check" ]] && [[ "$MODE" != "publish" ]]; }; then
  usage >&2
  exit 1
fi

SKILL_VERSION="$(node -p "require('$SKILL_DIR/skill.json').version")"
if [[ "$SKILL_VERSION" != "$VERSION" ]]; then
  echo "Skill version is $SKILL_VERSION; expected $VERSION." >&2
  exit 1
fi
if ! grep -Fq "## [$VERSION] - " "$SKILL_DIR/CHANGELOG.md"; then
  echo "Skill changelog has no $VERSION release section." >&2
  exit 1
fi
if ! grep -Fq "npm install -g nanobazaar-cli@$VERSION" "$SKILL_DIR/SKILL.md"; then
  echo "Skill does not require the matching released CLI." >&2
  exit 1
fi
for relative_path in SKILL.md skill.json CHANGELOG.md docs/PAYMENTS.md docs/POLLING.md docs/PAYLOADS.md docs/COMMANDS.md prompts/buyer.md prompts/seller.md examples/payment-policy.json; do
  if [[ ! -f "$SKILL_DIR/$relative_path" ]]; then
    echo "Standalone skill file is missing: $relative_path" >&2
    exit 1
  fi
done
if rg -n 'packages/nanobazaar-cli|repository root|bundled with this checkout|checkout CLI' "$SKILL_DIR" --glob '!CHANGELOG.md' >/dev/null; then
  echo "Skill contains a repository-only installation reference." >&2
  exit 1
fi

if [[ "$MODE" == "check" ]]; then
  echo "Standalone skill check passed for $VERSION."
  exit 0
fi
if [[ "$CONFIRM" != "--confirm-publish" ]]; then
  echo "Refusing skill publication without --confirm-publish." >&2
  exit 1
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
if [[ "$(npm view "nanobazaar-cli@$VERSION" version)" != "$VERSION" ]]; then
  echo "Publish nanobazaar-cli@$VERSION to npm before the skill." >&2
  exit 1
fi
if ! gh release view "nanobazaar-cli@$VERSION" >/dev/null 2>&1; then
  echo "Create the nanobazaar-cli@$VERSION GitHub release before the skill." >&2
  exit 1
fi
if ! command -v clawhub >/dev/null 2>&1; then
  echo "clawhub CLI not found." >&2
  exit 1
fi

CHANGELOG_TEXT="$(awk -v ver="$VERSION" '
  $0 ~ ("^## \\[" ver "\\] - ") { in_release=1; next }
  in_release && /^## \[/ { exit }
  in_release { print }
' "$SKILL_DIR/CHANGELOG.md")"
if [[ -z "$CHANGELOG_TEXT" ]]; then
  echo "Could not extract skill release notes." >&2
  exit 1
fi
clawhub publish \
  --slug nanobazaar \
  --name NanoBazaar \
  --version "$VERSION" \
  --changelog "$CHANGELOG_TEXT" \
  --tags latest \
  "$SKILL_DIR"
