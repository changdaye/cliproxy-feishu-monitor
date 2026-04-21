#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION="${1:-${VERSION:-}}"
REMOTE="${REMOTE:-origin}"
PUSH_BRANCH="${PUSH_BRANCH:-0}"

if [[ -z "$VERSION" ]]; then
  echo "Usage: bash deploy/release.sh v0.1.1" >&2
  exit 1
fi

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "[ERROR] Version must look like v0.1.1" >&2
  exit 1
fi

cd "$APP_DIR"

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "[ERROR] Working tree is dirty. Commit or stash changes first." >&2
  git status --short
  exit 1
fi

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
echo "[1/6] Running tests..."
go test ./...

echo "[2/6] Building release tarball..."
bash deploy/build-release.sh >/tmp/cliproxy_release_path.txt
RELEASE_ASSET="$(tail -n 1 /tmp/cliproxy_release_path.txt)"
if [[ ! -f "$RELEASE_ASSET" ]]; then
  echo "[ERROR] Release asset not found: $RELEASE_ASSET" >&2
  exit 1
fi

echo "[3/6] Verifying git tag does not already exist..."
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "[ERROR] Tag already exists: $VERSION" >&2
  exit 1
fi

echo "[4/6] Creating git tag $VERSION ..."
git tag -a "$VERSION" -m "Release $VERSION"

echo "[5/6] Ready to push."
echo "  Branch: $CURRENT_BRANCH"
echo "  Tag:    $VERSION"
echo "  Asset:  $RELEASE_ASSET"

if [[ "$PUSH_BRANCH" == "1" ]]; then
  echo "[6/6] Pushing branch and tag to $REMOTE ..."
  git push "$REMOTE" "$CURRENT_BRANCH"
  git push "$REMOTE" "$VERSION"
else
  echo "[6/6] Pushing tag to $REMOTE ..."
  git push "$REMOTE" "$VERSION"
fi

echo "Done. GitHub Actions should build and publish the release for $VERSION."
