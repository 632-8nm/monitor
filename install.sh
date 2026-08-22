#!/bin/bash
# One-click installer: build from source on this machine and install the
# Orange Pi system monitor service.
# Usage: run ./install.sh from the repository root
# Custom install directory: INSTALL_DIR=/path/to/dir ./install.sh
# Re-running performs an upgrade: build → overwrite files → restart service
set -e

# ===== Configuration =====
SERVICE_NAME="monitor"
BINARY_NAME="monitor_server"
INSTALL_DIR="${INSTALL_DIR:-/opt/orangepi-monitor}"
ENV_FILE="/etc/default/monitor"

echo "------------------------------------------------"
echo "🚀 Orange Pi monitor one-click install"
echo "   Install dir: $INSTALL_DIR"
echo "------------------------------------------------"

# ===== Pre-flight checks =====
if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go toolchain not found. Install it first, then re-run this script:"
    echo "     sudo apt install golang-go   or   https://go.dev/dl/"
    exit 1
fi

if [ ! -f go.mod ] || ! ls ./*.go >/dev/null 2>&1; then
    echo "❌ Current directory is not the project source root (missing go.mod or *.go files)"
    echo "   cd to the repository root and run: ./install.sh"
    exit 1
fi

if [ ! -f index.html ] || [ ! -d static ]; then
    echo "❌ Missing frontend files (index.html / static/); the repository may be incomplete"
    exit 1
fi

# Real user: handle sudo execution so the service does not run as root
RUN_USER="${SUDO_USER:-$USER}"
if [ -z "$RUN_USER" ] || [ "$RUN_USER" = "root" ]; then
    echo "❌ Cannot determine a non-root run user. Run this script as a regular user (sudo is invoked as needed)"
    exit 1
fi
RUN_GROUP="$(id -gn "$RUN_USER")"
echo "   Run user:   $RUN_USER"

# ===== 1. Build =====
echo "📦 Building the Go backend..."
CGO_ENABLED=0 go build -trimpath -o "$BINARY_NAME" .
chmod +x "$BINARY_NAME"
echo "✅ Build succeeded: $BINARY_NAME"

# ===== 2. Stop the old service (replacing a running binary triggers ETXTBSY) =====
if systemctl list-unit-files "$SERVICE_NAME.service" 2>/dev/null | grep -q "$SERVICE_NAME"; then
    echo "⏸️  Stopping the old service (if running)..."
    sudo systemctl stop "$SERVICE_NAME" 2>/dev/null || true
fi

# ===== 3. Install files into the runtime directory =====
# The source directory is only used for building; the service always runs from INSTALL_DIR
echo "📁 Installing files into $INSTALL_DIR ..."
sudo mkdir -p "$INSTALL_DIR"
sudo rm -rf "$INSTALL_DIR/static"
sudo install -m 755 "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
sudo install -m 644 index.html "$INSTALL_DIR/index.html"
sudo cp -r static "$INSTALL_DIR/static"
sudo chown -R "$RUN_USER:$RUN_GROUP" "$INSTALL_DIR"

# ===== 4. Write the systemd unit file =====
echo "📝 Generating the systemd service configuration..."
sudo tee "/etc/systemd/system/$SERVICE_NAME.service" >/dev/null <<EOT
[Unit]
Description=Orange Pi System Monitor Service
After=network.target

[Service]
Type=simple
User=$RUN_USER
WorkingDirectory=$INSTALL_DIR
EnvironmentFile=-$ENV_FILE
ExecStart=$INSTALL_DIR/$BINARY_NAME
Restart=always
RestartSec=5
# Logs go to journald; view with: journalctl -u monitor -f

[Install]
WantedBy=multi-user.target
EOT

# ===== 5. Create the default environment file (only if missing; never overwrite) =====
if [ ! -f "$ENV_FILE" ]; then
    echo "🔐 Creating the default security configuration: $ENV_FILE"
    sudo tee "$ENV_FILE" >/dev/null <<EOT
MONITOR_LISTEN_ADDR=127.0.0.1:8080
# MONITOR_BASIC_AUTH_USER=admin
# MONITOR_BASIC_AUTH_PASS=change_me
# MONITOR_ALLOWED_ORIGINS=https://monitor.example.com
EOT
fi

# ===== 6. Start the service =====
echo "⚙️  Starting the service and enabling it on boot..."
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl restart "$SERVICE_NAME"

if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo "------------------------------------------------"
    echo "🎉 Install complete! Service '$SERVICE_NAME' is up."
    echo "   Local URL:  http://127.0.0.1:8080"
    echo "   Logs:       journalctl -u $SERVICE_NAME -f"
    echo "   Uninstall:  ./uninstall.sh"
    echo "------------------------------------------------"
else
    echo "❌ The service is not running. Check logs: journalctl -u $SERVICE_NAME -n 50"
    exit 1
fi
