# Orange Pi Zero3 系统监控站

本仓库用于管理 Orange Pi Zero3 的系统监控程序，通过 GitHub Actions 实现代码的自动编译与持续部署。

**🌐 监控演示：** [https://orangepi-monitor.your-domain.example/](https://orangepi-monitor.your-domain.example/)

> 📦 **部署架构与从零搭建流程见 [DEPLOYMENT.md](DEPLOYMENT.md)**（CI/CD、Cloudflare 隧道、/opt 部署、密钥配置）

## 项目功能

* **实时监控**：网页端动态展示 CPU 负载、频率、温度，以及内存占用、系统运行时间和实时网速。
* **全自动部署**：基于 GitHub Actions Runner，本地 `git push` 后，板子会自动完成代码拉取、编译并重启服务。
* **自托管前端**：前端页面直接托管在开发板上，通过同一个后端服务提供，无需额外的静态托管服务。
* **掉电自愈**：监控程序与内网穿透服务均注册为 Systemd 服务，支持开机自启。

## 核心架构

* **后端**：Go (Golang) + `gopsutil`（同时提供 API 和前端静态文件）
* **前端**：HTML5 / CSS3 / JavaScript (Vanilla JS)
* **服务管理**：Systemd (服务名：`monitor`)
* **网络穿透**：Cloudflare Tunnel（自定义域名）
* **域名**：your-domain.example

## 运维命令

* **查看监控服务状态**：`sudo systemctl status monitor`
* **查看运行日志**：`sudo journalctl -u monitor -f`
* **重启监控服务**：`sudo systemctl restart monitor`
* **查看隧道服务状态**：`sudo systemctl status cloudflared`
* **重启隧道服务**：`sudo systemctl restart cloudflared`
* **初始化服务**：`./setup_monitor.sh`（仅在环境变更或初次部署时执行）

## 重新部署

当代码更新推送到 GitHub 后，GitHub Actions 会自动：
1. 在 runner 上编译 Go 程序
2. 停止旧服务
3. 用新程序替换
4. 重启服务

无需手动干预。

## 服务配置

* **监控服务**：`/etc/systemd/system/monitor.service`
* **隧道服务**：`/etc/systemd/system/cloudflared.service`
* **服务端口**：8080
* **访问域名**：https://orangepi-monitor.your-domain.example/

## 安全配置（建议在生产启用）

可通过环境变量开启基础安全能力：

* `MONITOR_LISTEN_ADDR`：监听地址（默认 `127.0.0.1:8080`，建议保持默认）
* `MONITOR_BASIC_AUTH_USER`：Basic Auth 用户名
* `MONITOR_BASIC_AUTH_PASS`：Basic Auth 密码
* `MONITOR_ALLOWED_ORIGINS`：CORS 白名单，逗号分隔（例如 `https://orangepi-monitor.your-domain.example/`）

当未设置 `MONITOR_BASIC_AUTH_USER/PASS` 时，服务会以兼容模式运行（无鉴权）。
当未设置 `MONITOR_ALLOWED_ORIGINS` 时，服务会以宽松 CORS 模式运行。

## 技术亮点

1. **前后端一体化**：Go 后端同时提供 API 和静态文件服务，简化架构
2. **Cloudflare Tunnel**：无需公网 IP，通过 Cloudflare 实现内网穿透
3. **自定义域名**：使用自有域名 your-domain.example，提供稳定的访问入口
4. **Systemd 管理**：服务稳定可靠，支持开机自启和自动恢复
