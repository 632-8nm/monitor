#!/bin/bash
# 一键安装脚本：安装现成的 monitor_server 二进制并注册 systemd 服务。
# 前端已内嵌在二进制中，无需其他文件。
# 适用于 Release 包解压目录，或已执行过 ./build.sh 的源码目录。
# 用法：./install.sh          （在包含 monitor_server 的目录执行）
#        INSTALL_DIR=/path ./install.sh   自定义安装目录
# 重复执行即为升级：覆盖文件 → 重启服务。
set -e

# ===== 配置项 =====
SERVICE_NAME="monitor"
BINARY_NAME="monitor_server"
INSTALL_DIR="${INSTALL_DIR:-/opt/monitor}"
ENV_FILE="/etc/default/monitor"

echo "------------------------------------------------"
echo "🚀 监控服务安装"
echo "   安装目录: $INSTALL_DIR"
echo "------------------------------------------------"

# ===== 前置检查：当前目录必须有可执行包 =====
if [ ! -f "$BINARY_NAME" ]; then
    echo "❌ 当前目录未找到 $BINARY_NAME。"
    echo "   源码用户:    先执行 ./build.sh（编译并安装一步到位）"
    echo "   Release 用户: 先从 GitHub Releases 下载压缩包并解压"
    exit 1
fi

# 真实用户：处理 sudo 执行的场景，避免服务以 root 运行
RUN_USER="${SUDO_USER:-$USER}"
if [ -z "$RUN_USER" ] || [ "$RUN_USER" = "root" ]; then
    echo "❌ 无法确定非 root 运行用户，请以普通用户身份执行本脚本（安装过程会按需调用 sudo）"
    exit 1
fi
RUN_GROUP="$(id -gn "$RUN_USER")"
echo "   运行用户: $RUN_USER"

# ===== 1. 停止旧服务（避免替换运行中的二进制触发 ETXTBSY） =====
if systemctl list-unit-files "$SERVICE_NAME.service" 2>/dev/null | grep -q "$SERVICE_NAME"; then
    echo "⏸️  停止旧服务（若在运行）..."
    sudo systemctl stop "$SERVICE_NAME" 2>/dev/null || true
fi

# ===== 2. 安装二进制到运行目录 =====
echo "📁 正在安装二进制到 $INSTALL_DIR ..."
sudo mkdir -p "$INSTALL_DIR"
sudo install -m 755 "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
sudo chown -R "$RUN_USER:$RUN_GROUP" "$INSTALL_DIR"

# ===== 3. 写入 systemd 服务文件 =====
echo "📝 正在生成系统服务配置..."
sudo tee "/etc/systemd/system/$SERVICE_NAME.service" >/dev/null <<EOT
[Unit]
Description=System Monitor Service
After=network.target

[Service]
Type=simple
User=$RUN_USER
WorkingDirectory=$INSTALL_DIR
EnvironmentFile=-$ENV_FILE
ExecStart=$INSTALL_DIR/$BINARY_NAME
Restart=always
RestartSec=5
# 日志走 journald，查看: journalctl -u monitor -f

[Install]
WantedBy=multi-user.target
EOT

# ===== 4. 生成默认环境变量文件（若不存在，不覆盖已有配置） =====
if [ ! -f "$ENV_FILE" ]; then
    echo "🔐 正在创建默认安全配置文件: $ENV_FILE"
    sudo tee "$ENV_FILE" >/dev/null <<EOT
MONITOR_LISTEN_ADDR=127.0.0.1:8080
# MONITOR_BASIC_AUTH_USER=admin
# MONITOR_BASIC_AUTH_PASS=change_me
# MONITOR_ALLOWED_ORIGINS=https://monitor.example.com
# 告警推送（Server酱，https://sct.ftqq.com 微信扫码获取 SendKey）
# MONITOR_SERVERCHAN_KEY=SCTxxxxxxxx
# MONITOR_ALERT_TEMP=70     (温度阈值 °C，0 = 禁用)
# MONITOR_ALERT_MEM=90      (内存阈值 %，0 = 禁用)
# MONITOR_ALERT_DISK=90     (磁盘阈值 %，0 = 禁用)
# MONITOR_ALERT_COOLDOWN=30 (同一告警重发间隔，分钟)
# MONITOR_TERMINAL=1          (启用网页终端，需在可信网络使用)
# MONITOR_ADMIN_USER=admin     (管理员用户名，默认 admin)
# MONITOR_ADMIN_PASS=123456    (管理员密码，默认 123456，务必修改)
EOT
fi

# ===== 5. 启动服务 =====
echo "⚙️  正在启动服务并设置开机自启..."
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl restart "$SERVICE_NAME"

if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo "------------------------------------------------"
    echo "🎉 安装完成！服务 '$SERVICE_NAME' 已就绪。"
    echo "   本机访问: http://127.0.0.1:8080"
    echo "   查看日志: journalctl -u $SERVICE_NAME -f"
    echo "   卸载:     ./uninstall.sh"
    echo "------------------------------------------------"
else
    echo "❌ 服务未能正常运行，查看日志排查: journalctl -u $SERVICE_NAME -n 50"
    exit 1
fi
