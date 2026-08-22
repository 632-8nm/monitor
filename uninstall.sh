#!/bin/bash
# Uninstall script: reverts everything install.sh did.
# Usage: ./uninstall.sh
# Custom install directory: INSTALL_DIR=/path/to/dir ./uninstall.sh
# Note: the cloudflared tunnel is outside the scope of install.sh and is never touched.
set -e

# ===== Configuration (must stay in sync with install.sh) =====
SERVICE_NAME="monitor"
INSTALL_DIR="${INSTALL_DIR:-/opt/orangepi-monitor}"
ENV_FILE="/etc/default/monitor"

# Safety guard: refuse a dangerous install directory
# (an empty value expands to the default via :-, so only "/" needs blocking here)
if [ "$INSTALL_DIR" = "/" ]; then
    echo "❌ Invalid INSTALL_DIR: '$INSTALL_DIR', refusing to continue"
    exit 1
fi

echo "------------------------------------------------"
echo "🧹 Orange Pi monitor uninstall"
echo "   Will remove: systemd service / $INSTALL_DIR / $ENV_FILE"
echo "------------------------------------------------"

read -r -p "Confirm uninstall? (y/N) " confirm || confirm=""
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    echo "Cancelled, nothing was changed"
    exit 0
fi

echo "⏹️  Stopping and removing the systemd service..."
sudo systemctl stop "$SERVICE_NAME" 2>/dev/null || true
sudo systemctl disable "$SERVICE_NAME" 2>/dev/null || true
sudo rm -f "/etc/systemd/system/$SERVICE_NAME.service"
sudo systemctl daemon-reload
sudo systemctl reset-failed 2>/dev/null || true

echo "🗑️  Removing the deploy directory $INSTALL_DIR ..."
sudo rm -rf "$INSTALL_DIR"

echo "🗑️  Removing the environment file $ENV_FILE ..."
sudo rm -f "$ENV_FILE"

echo "------------------------------------------------"
echo "✅ Uninstall complete"
echo "------------------------------------------------"
