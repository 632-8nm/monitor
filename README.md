# Orange Pi Zero3 系统监控站

本仓库用于管理 Orange Pi Zero3 的系统监控程序，通过 GitHub Actions 实现云端自动编译与持续部署。

> 📦 **部署架构与从零搭建流程见 [DEPLOYMENT.md](DEPLOYMENT.md)**（CI/CD、Cloudflare 隧道、/opt 部署、密钥配置）

## 项目功能

* **实时监控**：网页端动态展示 CPU 负载、频率、温度，内存 / Swap 占用，磁盘、网速、TCP 连接与运行时间。
* **24 小时趋势图**：内存态历史记录（每 10 秒一个点），Canvas 折线图展示 CPU / 温度 / 内存 / 网络趋势，无需数据库即可回答"昨晚温度为什么飙高"。
* **全自动部署**：GitHub Actions 云端交叉编译 arm64 二进制，经 Cloudflare Tunnel 部署到板端，`git push` 即发布。
* **单文件部署**：前端通过 `go:embed` 嵌入二进制，发布与部署只交付一个可执行文件。
* **掉电自愈**：监控程序与内网穿透均注册为 Systemd 服务，开机自启、崩溃自动拉起。

## 项目结构

```
├── cmd/monitor/main.go   # 程序入口（package main）
├── sensor.go             # 指标采集（后台固定周期采样）
├── history.go            # 24h 趋势环形缓冲（内存态）
├── server.go             # HTTP 服务（API + 内嵌前端）
├── assets.go             # go:embed 前端资源声明
├── web/                  # 前端源文件（编译时嵌入二进制）
├── build.sh              # 源码编译 + 安装（本机自建入口）
├── install.sh            # 安装预编译二进制 + 注册 systemd 服务
├── uninstall.sh          # 卸载上述全部产物
└── .github/workflows/    # deploy.yml（CI/CD 部署）、release.yml（tag 发版）
```

> 根目录的 Go 文件组成库包 `package monitor`，`cmd/monitor` 是唯一的 `main` 包。

## 核心架构

* **后端**：Go + `gopsutil`（提供 API 与内嵌前端）
* **前端**：HTML5 / CSS3 / 原生 JavaScript（Canvas 绘制趋势图，零第三方依赖）
* **服务管理**：Systemd（服务名：`monitor`）
* **网络穿透**：Cloudflare Tunnel（自定义域名 your-domain.example）

## 运维命令

* **查看监控服务状态**：`sudo systemctl status monitor`
* **查看运行日志**：`sudo journalctl -u monitor -f`
* **重启监控服务**：`sudo systemctl restart monitor`
* **查看隧道服务状态**：`sudo systemctl status cloudflared`
* **重启隧道服务**：`sudo systemctl restart cloudflared`
* **源码编译并安装**：`./build.sh`（编译后自动执行 install.sh；`BUILD_ONLY=1 ./build.sh` 仅编译）
* **安装预编译包**：`./install.sh`（将当前目录的 `monitor_server` 安装到 `/opt/orangepi-monitor`，支持 `INSTALL_DIR` 自定义）
* **卸载**：`./uninstall.sh`（撤销 install.sh 的全部操作，不影响 cloudflared 隧道）

## 重新部署

当代码更新推送到 GitHub 后，GitHub Actions 会自动：
1. 在云端 runner 交叉编译 Go 程序
2. 停止板上旧服务
3. 替换二进制（前端已内嵌）
4. 重启服务并做存活检查

无需手动干预。

## 自建部署（其他设备）

**方式一：使用 Release 包（推荐，无需 Go 工具链）**

推送 `v*` 标签会自动构建发布包。从 GitHub Releases 下载最新的 `linux-arm64` 压缩包后：

```bash
tar -xzf orangepi-monitor-vX.Y.Z-linux-arm64.tar.gz
cd orangepi-monitor-vX.Y.Z
./install.sh
```

**方式二：从源码构建**

```bash
git clone <本仓库> && cd orangepi-monitor
./build.sh
```

两种方式都安装到 `/opt/orangepi-monitor`（可用 `INSTALL_DIR` 覆盖），注册 systemd 服务并开机自启，默认只监听 `127.0.0.1:8080`。安全配置见下文，卸载执行 `./uninstall.sh`。

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

当未设置 `MONITOR_BASIC_AUTH_USER/PASS` 时，服务以兼容模式运行（无鉴权）。
当未设置 `MONITOR_ALLOWED_ORIGINS` 时，服务以宽松 CORS 模式运行。

## 告警推送（Server酱 → 微信）

设置 `MONITOR_SERVERCHAN_KEY` 后启用告警：温度 / 内存 / 磁盘越过阈值时通过 [Server酱](https://sct.ftqq.com)（微信扫码登录获取 SendKey）推送到微信，回落到阈值以下会再推一条恢复通知。相关环境变量：

* `MONITOR_SERVERCHAN_KEY`：Server酱 SendKey（未设置则告警功能整体关闭）
* `MONITOR_ALERT_TEMP`：温度告警阈值 °C（默认 70，0 禁用）
* `MONITOR_ALERT_MEM`：内存告警阈值 %（默认 90，0 禁用）
* `MONITOR_ALERT_DISK`：磁盘告警阈值 %（默认 90，0 禁用）
* `MONITOR_ALERT_COOLDOWN`：同一告警的重发间隔，分钟（默认 30，配合 Server酱 免费版每日 5 条额度）

本仓库 CI 部署时会把 GitHub Secret `SERVERCHAN_KEY` 自动写入板子的 `/etc/default/monitor`（合并写入，不影响其他配置行）；用 Release 包或源码自建的设备，直接编辑该文件即可。

## 技术亮点

1. **单文件交付**：前端 `go:embed` 进二进制，部署、发版、传输都只有一个文件
2. **零依赖趋势图**：内存环形缓冲 + 原生 Canvas，不引数据库不引前端框架
3. **Cloudflare Tunnel**：无需公网 IP，板子主动出连实现内网穿透
4. **Systemd 管理**：开机自启、崩溃自愈
