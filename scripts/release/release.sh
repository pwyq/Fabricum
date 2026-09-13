#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

VERSION_TAG="${1:-}"
RELEASE_BRANCH="main"

if [[ -z "$VERSION_TAG" ]]; then
  echo "Usage: ./scripts/release/release.sh v0.1.0"
  exit 1
fi

echo "Checking working tree..."
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Git working tree is not clean. Commit or stash changes first."
  exit 1
fi

echo "Fetching release refs..."
git fetch --tags --prune-tags origin

echo "Validating release tag..."
node scripts/release/validate-release-tag.cjs "$VERSION_TAG" --mode preflight

CURRENT_BRANCH="$(git branch --show-current)"
if [[ -z "$CURRENT_BRANCH" ]]; then
  echo "Release must be dispatched from a branch, not detached HEAD."
  exit 1
fi

if ! UPSTREAM="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null)"; then
  echo "Branch $CURRENT_BRANCH has no upstream. Push it before releasing."
  exit 1
fi

if [[ "$UPSTREAM" != "origin/$CURRENT_BRANCH" ]]; then
  echo "Branch $CURRENT_BRANCH must track origin/$CURRENT_BRANCH before releasing."
  exit 1
fi

if [[ "$(git rev-parse HEAD)" != "$(git rev-parse "$UPSTREAM")" ]]; then
  echo "Branch $CURRENT_BRANCH is not synchronized with $UPSTREAM. Push it before releasing."
  exit 1
fi

if ! git merge-base --is-ancestor HEAD "origin/$RELEASE_BRANCH"; then
  echo "Release commit must be on origin/$RELEASE_BRANCH so GitHub can compare later commits with this release."
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI is required to dispatch the release workflow."
  echo "Install it from https://cli.github.com/ and authenticate with 'gh auth login'."
  exit 1
fi

echo "Dispatching release workflow for $VERSION_TAG from $CURRENT_BRANCH..."
gh workflow run release.yml --ref "$CURRENT_BRANCH" --field version_tag="$VERSION_TAG"

echo "Release workflow dispatched. Monitor it with: gh run watch"
