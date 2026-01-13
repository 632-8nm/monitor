# Orange Pi Zero3 系统监控站

本仓库用于管理 Orange Pi Zero3 的系统监控程序，通过 GitHub Actions 实现代码的自动编译与持续部署。

**🌐 监控演示：** [https://632-8nm.github.io/orangepi-monitor/](https://632-8nm.github.io/orangepi-monitor/)

## 项目功能

* **实时监控**：网页端动态展示 CPU 负载、频率、温度，以及内存占用、系统运行时间和实时网速。
* **全自动部署**：基于 GitHub Actions Runner，本地 `git push` 后，板子会自动完成代码拉取、编译并重启服务。
* **动态寻址**：利用 GitHub Gist 记录内网穿透后的实时 URL，确保前端能准确获取后端数据。
* **掉电自愈**：监控程序与内网穿透服务均注册为 Systemd 服务，支持开机自启。

## 核心架构

* **后端**：Go (Golang) + `gopsutil`
* **前端**：HTML5 / CSS3 / JavaScript (Vanilla JS)
* **服务管理**：Systemd (服务名：`monitor`)
* **网络穿透**：Cloudflare Tunnel

## 运维命令

* **查看服务状态**：`systemctl status monitor`
* **查看运行日志**：`journalctl -u monitor -f`
* **初始化服务**：`./setup_monitor.sh`（仅在环境变更或初次部署时执行）
