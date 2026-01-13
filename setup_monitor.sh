#!/bin/bash

# 配置项
SERVICE_NAME="monitor"
BINARY_NAME="monitor_server" # 你编译后的二进制文件名
PROJECT_DIR=$(pwd)           # 假设你在项目根目录下执行脚本
USER_NAME=$USER

echo "------------------------------------------------"
echo "🚀 Orange Pi 监控服务一键配置工具"
echo "------------------------------------------------"

# 1. 编译 Go 程序 (确保依赖已安装)
echo "📦 正在编译 Go 后端程序..."
go build -o $BINARY_NAME ./server.go ./sensor.go
if [ $? -ne 0 ]; then
    echo "❌ 编译失败，请检查 Go 环境或代码错误。"
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
ExecStart=$PROJECT_DIR/$BINARY_NAME
Restart=always
RestartSec=5
StandardOutput=append:$PROJECT_DIR/service.log
StandardError=append:$PROJECT_DIR/service.log

[Install]
WantedBy=multi-user.target
EOT"

# 3. 启动并激活服务
echo "⚙️ 正在启动服务并设置开机自启..."
sudo systemctl daemon-reload
sudo systemctl enable $SERVICE_NAME
sudo systemctl restart $SERVICE_NAME

# 4. 检查服务状态
if systemctl is-active --quiet $SERVICE_NAME; then
    echo "------------------------------------------------"
    echo "🎉 部署完成！"
    echo "✅ 监控服务已启动并设为开机自启。"
    echo "🔍 查看实时日志: tail -f service.log"
    echo "🛑 停止服务命令: sudo systemctl stop $SERVICE_NAME"
    echo "------------------------------------------------"
else
    echo "❌ 服务启动失败，请运行 'journalctl -u $SERVICE_NAME' 查看原因。"
fi