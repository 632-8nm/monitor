#!/bin/bash
# 源码构建脚本：编译 monitor 二进制，然后执行 ./install.sh 完成安装。
# 用法：./build.sh                （编译 + 安装）
#        BUILD_ONLY=1 ./build.sh  （仅编译，不安装）
# INSTALL_DIR 会透传给 install.sh。
set -e

# ===== 前置检查 =====
if ! command -v go >/dev/null 2>&1; then
    echo "❌ 未检测到 Go 工具链，请先安装后再执行本脚本："
    echo "     sudo apt install golang-go   或   https://go.dev/dl/"
    exit 1
fi

if [ ! -f go.mod ] || ! ls ./*.go >/dev/null 2>&1; then
    echo "❌ 当前目录不是项目源码根目录（缺少 go.mod 或 *.go 文件）"
    echo "   请 cd 到仓库根目录后重新执行：./build.sh"
    exit 1
fi

# ===== 编译 =====
VERSION_VAL="local-$(git rev-parse --short HEAD 2>/dev/null || date +%Y%m%d)"
echo "📦 正在编译 Go 后端 (版本: $VERSION_VAL)..."
CGO_ENABLED=0 go build -trimpath -ldflags="-w -X orangepi-monitor.Version=$VERSION_VAL" -o monitor_server ./cmd/monitor
chmod +x monitor_server
echo "✅ 编译成功: monitor_server"

if [ "${BUILD_ONLY:-0}" = "1" ]; then
    echo "已设置 BUILD_ONLY=1，跳过安装。需要部署时执行 ./install.sh。"
    exit 0
fi

# ===== 安装（前置检查、systemd 与启动均由 install.sh 处理） =====
exec ./install.sh
