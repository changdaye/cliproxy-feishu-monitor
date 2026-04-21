#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SERVICE_NAME="${SERVICE_NAME:-cliproxy-feishu-monitor}"
INSTALL_DIR="${INSTALL_DIR:-/opt/cliproxy-feishu-monitor}"
UNIT_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
BINARY_NAME="cliproxy-feishu-monitor"

if command -v sudo >/dev/null 2>&1; then
  SUDO="sudo"
else
  SUDO=""
fi
if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
  SUDO=""
fi

if [[ ! -f "$APP_DIR/local.runtime.json" ]]; then
  echo "[ERROR] Missing $APP_DIR/local.runtime.json" >&2
  echo "Copy local.runtime.json.example to local.runtime.json and fill in secrets first." >&2
  exit 1
fi

if [[ ! -f "$APP_DIR/$BINARY_NAME" ]]; then
  if [[ -f "$APP_DIR/dist/$BINARY_NAME" ]]; then
    cp "$APP_DIR/dist/$BINARY_NAME" "$APP_DIR/$BINARY_NAME"
  else
    (cd "$APP_DIR" && go build -o "$BINARY_NAME" .)
  fi
fi

chmod +x "$APP_DIR/$BINARY_NAME" "$APP_DIR/deploy/run-once.sh" "$APP_DIR/deploy/service-status.sh" "$APP_DIR/deploy/service-logs.sh"

$SUDO mkdir -p "$INSTALL_DIR/data"
$SUDO cp "$APP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
$SUDO cp "$APP_DIR/local.runtime.json" "$INSTALL_DIR/local.runtime.json"
$SUDO chmod 600 "$INSTALL_DIR/local.runtime.json"

$SUDO tee "$UNIT_PATH" >/dev/null <<UNIT
[Unit]
Description=CLIProxyAPI Feishu Monitor
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$BINARY_NAME serve
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT

$SUDO systemctl daemon-reload
$SUDO systemctl enable --now "$SERVICE_NAME"

echo
$SUDO systemctl --no-pager --full status "$SERVICE_NAME" || true

echo
printf 'Deployment completed. Useful commands:\n'
printf '  journalctl -u %s -f\n' "$SERVICE_NAME"
printf '  systemctl status %s\n' "$SERVICE_NAME"
printf '  %s/%s run-once\n' "$INSTALL_DIR" "$BINARY_NAME"
