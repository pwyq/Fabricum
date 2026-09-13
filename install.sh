#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

for command in node npm go cmake; do
  require_command "$command"
done

if [[ -d "$ROOT_DIR/.git" || -f "$ROOT_DIR/.git" ]]; then
  require_command git
fi

printf 'Installing Fabricum dependencies…\n'
npm ci
go mod download

printf 'Installing local Git hooks…\n'
node scripts/git/install-hooks.mjs

printf 'Installing pinned native codec tools…\n'
node scripts/install-native-codecs.mjs

printf 'Building the Fabricum executable…\n'
node scripts/build.mjs

EXECUTABLE="$ROOT_DIR/bin/fabricum"
if [[ -f "$ROOT_DIR/bin/fabricum.exe" ]]; then
  EXECUTABLE="$ROOT_DIR/bin/fabricum.exe"
fi

printf '\nFabricum is ready.\n'
printf 'GUI:  %s\n' "$EXECUTABLE"
printf 'Run the executable directly to open the GUI, or pass --source and --output for path-driven use.\n'
