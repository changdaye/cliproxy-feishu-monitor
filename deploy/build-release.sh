#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST_DIR="$APP_DIR/dist"
RELEASE_DIR="$DIST_DIR/cliproxy-feishu-monitor-linux-x86_64"
ARCHIVE_PATH="$DIST_DIR/cliproxy-feishu-monitor-linux-x86_64.tar.gz"

rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

(
  cd "$APP_DIR"
  GOOS=linux GOARCH=amd64 go build -o "$RELEASE_DIR/cliproxy-feishu-monitor" .
)

cp "$APP_DIR/README.md" "$RELEASE_DIR/README.md"
cp "$APP_DIR/local.runtime.json.example" "$RELEASE_DIR/local.runtime.json.example"
cp "$APP_DIR/deploy/install.sh" "$RELEASE_DIR/install-service.sh"
cp "$APP_DIR/deploy/run-once.sh" "$RELEASE_DIR/run-once.sh"
cp "$APP_DIR/deploy/service-status.sh" "$RELEASE_DIR/service-status.sh"
cp "$APP_DIR/deploy/service-logs.sh" "$RELEASE_DIR/service-logs.sh"
cp "$APP_DIR/deploy/deploy-from-tar.sh" "$RELEASE_DIR/deploy-from-tar.sh"
cp "$APP_DIR/local.runtime.server.json.example" "$RELEASE_DIR/local.runtime.server.json.example"
chmod +x "$RELEASE_DIR/cliproxy-feishu-monitor" "$RELEASE_DIR/install-service.sh" "$RELEASE_DIR/run-once.sh" "$RELEASE_DIR/service-status.sh" "$RELEASE_DIR/service-logs.sh" "$RELEASE_DIR/deploy-from-tar.sh"

tar -C "$DIST_DIR" -czf "$ARCHIVE_PATH" "$(basename "$RELEASE_DIR")"
echo "$ARCHIVE_PATH"
