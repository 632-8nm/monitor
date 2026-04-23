#!/bin/bash

# 配置项
SERVICE_NAME="monitor"
BINARY_NAME="monitor_server" 
PROJECT_DIR=$(pwd)           
USER_NAME=$USER
ENV_FILE="/etc/default/monitor"

echo "------------------------------------------------"
echo "🚀 Orange Pi 监控服务一键配置工具"
echo "------------------------------------------------"

# 1. 编译 Go 程序 (编译当前目录下的所有文件)
echo "📦 正在编译 Go 后端程序..."
go build -o $BINARY_NAME . 
if [ $? -ne 0 ]; then
    echo "❌ 编译失败，请检查 Go 代码是否有 main 函数。"
    exit 1
fi
chmod +x $BINARY_NAME
echo "✅ 编译成功: $BINARY_NAME"

# 2. 写入 Systemd 服务文件
echo "📝 正在生成系统服务配置..."
sudo bash -c "cat <<EOT > /etc/systemd/system/$SERVICE_NAME.service
[Unit]
Description=Orange Pi System Monitor Service
After=network.target

[Service]
Type=simple
User=$USER_NAME
WorkingDirectory=$PROJECT_DIR
EnvironmentFile=-$ENV_FILE
ExecStart=$PROJECT_DIR/$BINARY_NAME
Restart=always
RestartSec=5
StandardOutput=append:$PROJECT_DIR/service.log
StandardError=append:$PROJECT_DIR/service.log

[Install]
WantedBy=multi-user.target
EOT"

# 2.1 生成默认环境变量文件（若不存在）
if [ ! -f "$ENV_FILE" ]; then
    echo "🔐 正在创建默认安全配置文件: $ENV_FILE"
    sudo bash -c "cat <<EOT > $ENV_FILE
MONITOR_LISTEN_ADDR=127.0.0.1:8080
# MONITOR_BASIC_AUTH_USER=admin
# MONITOR_BASIC_AUTH_PASS=change_me
# MONITOR_ALLOWED_ORIGINS=https://monitor.your-domain.example,https://orangepi-monitor.your-domain.example
EOT"
fi

# 3. 启动并激活服务
echo "⚙️ 正在启动服务并设置开机自启..."
sudo systemctl daemon-reload
sudo systemctl enable $SERVICE_NAME
sudo systemctl restart $SERVICE_NAME

if systemctl is-active --quiet $SERVICE_NAME; then
    echo "------------------------------------------------"
    echo "🎉 部署完成！"
    echo "✅ 服务 '$SERVICE_NAME' 已就绪。"
    echo "------------------------------------------------"
fi