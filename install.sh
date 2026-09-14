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

printf '\nFabricum development environment is ready.\n'
printf 'Build a development executable with: npm run build\n'
printf 'Build the standalone release executable with: npm run release:binary\n'
