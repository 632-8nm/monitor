#!/bin/bash
# Installer for a ready-to-run monitor package: installs the binary and
# frontend files into INSTALL_DIR and registers the systemd service.
# Works from either an extracted release tarball or a source tree where
# ./build.sh has already produced the binary.
# Usage: ./install.sh          (from the directory containing the package)
#        INSTALL_DIR=/path ./install.sh   to override the install location
# Re-running performs an upgrade: overwrite files → restart service.
set -e

# ===== Configuration =====
SERVICE_NAME="monitor"
BINARY_NAME="monitor_server"
INSTALL_DIR="${INSTALL_DIR:-/opt/orangepi-monitor}"
ENV_FILE="/etc/default/monitor"

echo "------------------------------------------------"
echo "🚀 Orange Pi monitor install"
echo "   Install dir: $INSTALL_DIR"
echo "------------------------------------------------"

# ===== Pre-flight checks: a runnable package must be present =====
if [ ! -f "$BINARY_NAME" ]; then
    echo "❌ $BINARY_NAME not found in the current directory."
    echo "   From source:    run ./build.sh (it compiles and installs in one go)"
    echo "   From a release: download a tarball from GitHub Releases and extract it first"
    exit 1
fi

if [ ! -f index.html ] || [ ! -d static ]; then
    echo "❌ Missing frontend files (index.html / static/); the package is incomplete"
    exit 1
fi

# Real user: handle sudo execution so the service does not run as root
RUN_USER="${SUDO_USER:-$USER}"
if [ -z "$RUN_USER" ] || [ "$RUN_USER" = "root" ]; then
    echo "❌ Cannot determine a non-root run user. Run this script as a regular user (sudo is invoked as needed)"
    exit 1
fi
RUN_GROUP="$(id -gn "$RUN_USER")"
echo "   Run user:    $RUN_USER"

# ===== 1. Stop the old service (replacing a running binary triggers ETXTBSY) =====
if systemctl list-unit-files "$SERVICE_NAME.service" 2>/dev/null | grep -q "$SERVICE_NAME"; then
    echo "⏸️  Stopping the old service (if running)..."
    sudo systemctl stop "$SERVICE_NAME" 2>/dev/null || true
fi

# ===== 2. Install files into the runtime directory =====
echo "📁 Installing files into $INSTALL_DIR ..."
sudo mkdir -p "$INSTALL_DIR"
sudo rm -rf "$INSTALL_DIR/static"
sudo install -m 755 "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
sudo install -m 644 index.html "$INSTALL_DIR/index.html"
sudo cp -r static "$INSTALL_DIR/static"
sudo chown -R "$RUN_USER:$RUN_GROUP" "$INSTALL_DIR"

# ===== 3. Write the systemd unit file =====
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

# ===== 4. Create the default environment file (only if missing; never overwrite) =====
if [ ! -f "$ENV_FILE" ]; then
    echo "🔐 Creating the default security configuration: $ENV_FILE"
    sudo tee "$ENV_FILE" >/dev/null <<EOT
MONITOR_LISTEN_ADDR=127.0.0.1:8080
# MONITOR_BASIC_AUTH_USER=admin
# MONITOR_BASIC_AUTH_PASS=change_me
# MONITOR_ALLOWED_ORIGINS=https://monitor.example.com
EOT
fi

# ===== 5. Start the service =====
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
