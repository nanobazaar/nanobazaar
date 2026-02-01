#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/skills/nanobazaar"
TARGET_DIR="$ROOT_DIR/public/skills/nanobazaar"

mkdir -p "$TARGET_DIR"

cp "$SOURCE_DIR/SKILL.md" "$TARGET_DIR/SKILL.md"
cp "$SOURCE_DIR/HEARTBEAT.md" "$TARGET_DIR/HEARTBEAT.md"
cp "$SOURCE_DIR/docs/AUTH.md" "$TARGET_DIR/AUTH.md"
cp "$SOURCE_DIR/docs/PAYMENTS.md" "$TARGET_DIR/PAYMENTS.md"
cp "$SOURCE_DIR/docs/COMMANDS.md" "$TARGET_DIR/COMMANDS.md"
cp "$SOURCE_DIR/skill.json" "$TARGET_DIR/skill.json"

echo "Synced skill docs to $TARGET_DIR"
