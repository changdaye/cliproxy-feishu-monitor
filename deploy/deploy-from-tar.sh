#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="cliproxy-feishu-monitor"
SERVICE_NAME="${SERVICE_NAME:-cliproxy-feishu-monitor}"
INSTALL_DIR="${INSTALL_DIR:-/opt/cliproxy-feishu-monitor}"
WORK_ROOT="${WORK_ROOT:-/tmp/${APP_NAME}-deploy}"
TAR_URL="${TAR_URL:-}"
CONFIG_URL="${CONFIG_URL:-}"
CONFIG_FILE="${CONFIG_FILE:-}"
KEEP_WORKDIR="${KEEP_WORKDIR:-0}"

usage() {
  cat <<USAGE
Usage:
  bash deploy-from-tar.sh --tar-url <url> [--config-url <url> | --config-file <path>]

Options:
  --tar-url <url>        Release tar.gz URL (required)
  --config-url <url>     local.runtime.json download URL (optional)
  --config-file <path>   Existing local.runtime.json path on server (optional)
  --install-dir <path>   Install directory, default: /opt/cliproxy-feishu-monitor
  --service-name <name>  systemd service name, default: cliproxy-feishu-monitor
  --keep-workdir         Keep extracted temp files for debugging

Environment fallback when config file/url is not provided:
  CPA_BASE_URL
  CPA_MANAGEMENT_KEY
  FEISHU_WEBHOOK
  FEISHU_SECRET
  POLL_INTERVAL_HOURS
  HEARTBEAT_INTERVAL_HOURS
  HEARTBEAT_ENABLED
  STARTUP_NOTIFICATION_ENABLED
  RUN_SUMMARY_ON_STARTUP
  REQUEST_TIMEOUT_SECONDS
  CONCURRENCY
  FAILURE_ALERT_THRESHOLD
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tar-url)
      TAR_URL="$2"
      shift 2
      ;;
    --config-url)
      CONFIG_URL="$2"
      shift 2
      ;;
    --config-file)
      CONFIG_FILE="$2"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --service-name)
      SERVICE_NAME="$2"
      shift 2
      ;;
    --keep-workdir)
      KEEP_WORKDIR=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[ERROR] Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$TAR_URL" ]]; then
  echo "[ERROR] --tar-url is required" >&2
  usage
  exit 1
fi

if command -v sudo >/dev/null 2>&1; then
  SUDO="sudo"
else
  SUDO=""
fi
if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
  SUDO=""
fi

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[ERROR] Missing required command: $1" >&2
    exit 1
  }
}

fetch() {
  local url="$1"
  local out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$out" "$url"
  else
    echo "[ERROR] curl or wget is required" >&2
    exit 1
  fi
}

generate_runtime_config() {
  local dest="$1"
  : "${CPA_BASE_URL:?CPA_BASE_URL is required when config file/url is not provided}"
  : "${CPA_MANAGEMENT_KEY:?CPA_MANAGEMENT_KEY is required when config file/url is not provided}"
  : "${FEISHU_WEBHOOK:?FEISHU_WEBHOOK is required when config file/url is not provided}"
  : "${FEISHU_SECRET:?FEISHU_SECRET is required when config file/url is not provided}"

  cat > "$dest" <<JSON
{
  "cpa_base_url": "${CPA_BASE_URL}",
  "management_key": "${CPA_MANAGEMENT_KEY}",
  "feishu_webhook": "${FEISHU_WEBHOOK}",
  "feishu_secret": "${FEISHU_SECRET}",
  "poll_interval_hours": ${POLL_INTERVAL_HOURS:-6},
  "heartbeat_interval_hours": ${HEARTBEAT_INTERVAL_HOURS:-3},
  "heartbeat_enabled": ${HEARTBEAT_ENABLED:-true},
  "startup_notification_enabled": ${STARTUP_NOTIFICATION_ENABLED:-true},
  "run_summary_on_startup": ${RUN_SUMMARY_ON_STARTUP:-true},
  "request_timeout_seconds": ${REQUEST_TIMEOUT_SECONDS:-30},
  "concurrency": ${CONCURRENCY:-8},
  "failure_alert_threshold": ${FAILURE_ALERT_THRESHOLD:-3},
  "state_path": "data/runtime-state.json"
}
JSON
}

need_cmd tar
need_cmd bash

rm -rf "$WORK_ROOT"
mkdir -p "$WORK_ROOT"
ARCHIVE_PATH="$WORK_ROOT/${APP_NAME}.tar.gz"
EXTRACT_DIR="$WORK_ROOT/extracted"
mkdir -p "$EXTRACT_DIR"

echo "[1/6] Downloading release tarball..."
fetch "$TAR_URL" "$ARCHIVE_PATH"

echo "[2/6] Extracting package..."
tar -xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR"
PACKAGE_DIR="$(find "$EXTRACT_DIR" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
if [[ -z "$PACKAGE_DIR" ]]; then
  echo "[ERROR] Failed to locate extracted package directory" >&2
  exit 1
fi

if [[ -n "$CONFIG_URL" && -n "$CONFIG_FILE" ]]; then
  echo "[ERROR] Use only one of --config-url or --config-file" >&2
  exit 1
fi

echo "[3/6] Preparing runtime config..."
if [[ -n "$CONFIG_FILE" ]]; then
  cp "$CONFIG_FILE" "$PACKAGE_DIR/local.runtime.json"
elif [[ -n "$CONFIG_URL" ]]; then
  fetch "$CONFIG_URL" "$PACKAGE_DIR/local.runtime.json"
else
  generate_runtime_config "$PACKAGE_DIR/local.runtime.json"
fi
chmod 600 "$PACKAGE_DIR/local.runtime.json"

if [[ ! -x "$PACKAGE_DIR/install-service.sh" ]]; then
  chmod +x "$PACKAGE_DIR/install-service.sh"
fi

echo "[4/6] Installing service files..."
(
  cd "$PACKAGE_DIR"
  SERVICE_NAME="$SERVICE_NAME" INSTALL_DIR="$INSTALL_DIR" bash ./install-service.sh
)

echo "[5/6] Verifying service..."
$SUDO systemctl --no-pager --full status "$SERVICE_NAME" || true

echo "[6/6] Deployment complete."
echo "Install dir: $INSTALL_DIR"
echo "Service name: $SERVICE_NAME"
echo "Useful commands:"
echo "  journalctl -u $SERVICE_NAME -f"
echo "  systemctl status $SERVICE_NAME"
echo "  $INSTALL_DIR/cliproxy-feishu-monitor run-once"

if [[ "$KEEP_WORKDIR" != "1" ]]; then
  rm -rf "$WORK_ROOT"
else
  echo "Workdir kept at: $WORK_ROOT"
fi
