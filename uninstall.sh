#!/bin/bash
# 卸载脚本：撤销 install.sh 的全部操作。
# 用法：./uninstall.sh
# 如自定义过安装目录：INSTALL_DIR=/path/to/dir ./uninstall.sh
# 注意：cloudflared 隧道不属于 install.sh 的安装范围，本脚本不会触碰。
set -e

# ===== 配置项（须与 install.sh 保持一致） =====
SERVICE_NAME="monitor"
INSTALL_DIR="${INSTALL_DIR:-/opt/monitor}"
ENV_FILE="/etc/default/monitor"

# 防呆：拒绝危险的安装目录（空值会经 :- 展开为默认目录，此处只需拦截 /）
if [ "$INSTALL_DIR" = "/" ]; then
    echo "❌ 非法的 INSTALL_DIR: '$INSTALL_DIR'，拒绝执行"
    exit 1
fi

echo "------------------------------------------------"
echo "🧹 监控服务卸载"
echo "   将删除: systemd 服务 / $INSTALL_DIR / $ENV_FILE"
echo "------------------------------------------------"

read -r -p "确认卸载？(y/N) " confirm || confirm=""
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    echo "已取消，未做任何改动"
    exit 0
fi

echo "⏹️  停止并移除 systemd 服务..."
sudo systemctl stop "$SERVICE_NAME" 2>/dev/null || true
sudo systemctl disable "$SERVICE_NAME" 2>/dev/null || true
sudo rm -f "/etc/systemd/system/$SERVICE_NAME.service"
sudo systemctl daemon-reload
sudo systemctl reset-failed 2>/dev/null || true

echo "🗑️  删除部署目录 $INSTALL_DIR ..."
sudo rm -rf "$INSTALL_DIR"

echo "🗑️  删除环境变量文件 $ENV_FILE ..."
sudo rm -f "$ENV_FILE"

echo "------------------------------------------------"
echo "✅ 卸载完成"
echo "------------------------------------------------"
