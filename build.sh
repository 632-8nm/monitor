#!/bin/bash
# Build from source and install: compiles the monitor binary, then runs
# ./install.sh to deploy it.
# Usage: ./build.sh                  (build + install)
#        BUILD_ONLY=1 ./build.sh     (build only, no install)
# INSTALL_DIR is passed through to install.sh.
set -e

# ===== Pre-flight checks =====
if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go toolchain not found. Install it first, then re-run this script:"
    echo "     sudo apt install golang-go   or   https://go.dev/dl/"
    exit 1
fi

if [ ! -f go.mod ] || ! ls ./*.go >/dev/null 2>&1; then
    echo "❌ Current directory is not the project source root (missing go.mod or *.go files)"
    echo "   cd to the repository root and run: ./build.sh"
    exit 1
fi

# ===== Build =====
echo "📦 Building the Go backend..."
CGO_ENABLED=0 go build -trimpath -o monitor_server .
chmod +x monitor_server
echo "✅ Build succeeded: monitor_server"

if [ "${BUILD_ONLY:-0}" = "1" ]; then
    echo "BUILD_ONLY=1 set, skipping install. Run ./install.sh to deploy."
    exit 0
fi

# ===== Install (install.sh handles pre-flight, systemd and startup) =====
exec ./install.sh
