#!/usr/bin/env bash
set -Eeuo pipefail
SERVICE_NAME="${SERVICE_NAME:-cliproxy-feishu-monitor}"
journalctl -u "$SERVICE_NAME" -f
