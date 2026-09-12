#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STYLE_GUIDE="docs/development.md"

violations=0

is_generated_file() {
  local file="$1"
  local filename
  local relative_path

  filename="$(basename "$file")"
  relative_path="${file#"$ROOT_DIR"/}"

  # Ignore files with "generated" in the filename.
  [[ "${filename,,}" == *generated* ]] && return 0

  # Ignore files inside any directory named exactly "generated".
  [[ "/${relative_path,,}/" == */generated/* ]]
}

check_file() {
  local file="$1"
  local limit="$2"

  # Stylesheets are intentionally allowed to grow with the visual system.
  [[ "${file,,}" == *.css ]] && return

  is_generated_file "$file" && return

  local lines
  lines="$(wc -l < "$file")"
  lines="${lines//[[:space:]]/}"

  if (( lines <= limit )); then
    return
  fi

  local relative_path="${file#"$ROOT_DIR"/}"
  local excess=$((lines - limit))

  printf '  - %s: %d LOC (limit %d, %d over)\n' \
    "$relative_path" \
    "$lines" \
    "$limit" \
    "$excess"

  violations=$((violations + 1))
}

check_find() {
  local target="$1"
  local limit="$2"
  shift 2

  [[ -d "$target" ]] || return

  while IFS= read -r -d '' file; do
    check_file "$file" "$limit"
  done < <(find "$target" -type f "$@" -print0)
}

printf 'Checking file LOC limits…\n'

# Production Go files: 250 LOC.
check_find \
  "$ROOT_DIR/back-end" \
  250 \
  -name '*.go' ! -name '*_test.go'

# Go test files: 800 LOC.
check_find \
  "$ROOT_DIR/back-end" \
  800 \
  -name '*_test.go'

# Front-end source files: 300 LOC. The browser editor is intentionally split
# from stylesheets, which are excluded above.
check_find \
  "$ROOT_DIR/front-end/static" \
  300 \
  \( \
    -name '*.js' \
    -o -name '*.mjs' \
    -o -name '*.cjs' \
  \)

# Front-end test files: 800 LOC.
check_find \
  "$ROOT_DIR/front-end/tests" \
  800 \
  \( \
    -name '*.js' \
    -o -name '*.mjs' \
    -o -name '*.cjs' \
  \)

# Repository scripts: 250 LOC.
check_find \
  "$ROOT_DIR/scripts" \
  250 \
  \( \
    -name '*.js' \
    -o -name '*.mjs' \
    -o -name '*.cjs' \
  \)

if (( violations > 0 )); then
  printf '\n%d file(s) exceed the LOC limits.\n' "$violations"
  printf 'Refactor oversized files according to %s.\n' "$STYLE_GUIDE"
  exit 1
fi

printf 'File LOC check passed.\n'
