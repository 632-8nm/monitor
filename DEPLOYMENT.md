# 部署架构与搭建流程（orangepi-monitor）

> 本文档记录本项目从零搭建的完整流程与当前生产架构，供维护者参考。
> 敏感信息一律使用占位符，不包含真实 token / 密码。

## 1. 整体架构

```
开发者本地 (Windows)
   │  写代码 → go vet / go test（本地自测）
   │  git commit + push
   ▼
GitHub (orangepi-monitor 仓库)
   │  触发 GitHub Actions（workflow: deploy.yml）
   ▼
CI/CD（GitHub 云端 runner，ubuntu-latest）
   │  ① 交叉编译 arm64 二进制（CGO_ENABLED=0 GOOS=linux GOARCH=arm64）
   │  ② 通过 Cloudflare Tunnel (ssh.<your-domain>) SSH 到板子
   ▼
板子（Orange Pi Zero 3）
   │  停服务 → 压缩流传二进制 → 重启
   ▼
/opt/orangepi-monitor/  （systemd: monitor，监听 :8080）
```

> 注：前端已通过 `go:embed` 嵌入二进制，部署只交付单个
> `monitor_server` 可执行文件。

## 2. 生产部署位置

| 项 | 值 |
|---|---|
| 部署目录 | `/opt/orangepi-monitor/` |
| 二进制 | `/opt/orangepi-monitor/monitor_server`（含内嵌前端） |
| systemd 服务 | `monitor.service` |
| 监听端口 | `8080` |
| 公网入口 | `orangepi-monitor.<your-domain>`（Cloudflare 隧道 → http://localhost:8080） |
| CI 部署入口 | `ssh.<your-domain>`（Cloudflare 隧道 → ssh://localhost:22） |

## 3. 板子侧组件

| 组件 | 说明 | 状态 |
|---|---|---|
| `cloudflared` | Cloudflare 隧道客户端（token 模式） | systemd 服务，常驻 |
| `/opt/orangepi-monitor/` | 生产部署目录 | 由 CI 更新 |
| `monitor.service` | systemd 单元，指向 `/opt/orangepi-monitor/monitor_server` | 常驻 |

## 4. Cloudflare 侧配置

| 项 | 配置 | 用途 |
|---|---|---|
| 域名 | `<your-domain>` | 托管在 Cloudflare |
| 隧道 | token 模式（tunnel run --token） | 板子主动连 Cloudflare |
| Public Hostname | `orangepi-monitor.<your-domain>` → `http://localhost:8080` | 公网访问 Web |
| Public Hostname | `ssh.<your-domain>` → `ssh://localhost:22` | CI/CD 部署 SSH 入口（两项目共用） |
| Service Token | 在 Access → Service Auth 创建 | CI 认证（格式 ID:SECRET） |
| Access 策略 | `ci-deploy`（Service Auth + token） | 放行 CI 的 cloudflared 连接 |

## 5. GitHub 侧配置

| 项 | 值 | 说明 |
|---|---|---|
| 仓库 | `<your-org>/orangepi-monitor` | — |
| Workflow | `.github/workflows/deploy.yml` | push main 触发，云端编译 + 部署 |
| Workflow | `.github/workflows/release.yml` | push `v*` 标签触发，构建并发布 Release 包 |
| Secret: `BOARD_SSH_KEY` | 板子 `~/.ssh/deploy` 私钥 | 云端 SSH 登录板子（两项目共用） |
| Secret: `CLOUDFLARED_TOKEN` | `ClientID:ClientSecret` | cloudflared access 认证（两项目共用） |

## 6. 从零搭建步骤

### 6.1 板子准备
```bash
# 安装 cloudflared
curl -L --output /usr/local/bin/cloudflared \
  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64
chmod +x /usr/local/bin/cloudflared

# 配置免密 sudo（仅 systemctl/journalctl/tee，供 CI 使用）
sudo tee /etc/sudoers.d/orangepi-systemd <<'EOF'
orangepi ALL=(ALL) NOPASSWD: /usr/bin/systemctl, /bin/systemctl, /usr/bin/journalctl, /usr/bin/tee
EOF
sudo chmod 440 /etc/sudoers.d/orangepi-systemd

# 生成 CI 部署密钥
ssh-keygen -t ed25519 -N "" -f ~/.ssh/deploy
cat ~/.ssh/deploy.pub >> ~/.ssh/authorized_keys

# 创建生产部署目录
sudo mkdir -p /opt/orangepi-monitor
sudo chown orangepi:orangepi /opt/orangepi-monitor

# 配置 systemd 服务（日志走 journald；WorkingDirectory 勿指向已删除的 actions-runner 目录）
sudo tee /etc/systemd/system/monitor.service <<'EOF'
[Unit]
Description=Orange Pi System Monitor Service
After=network.target

[Service]
Type=simple
User=orangepi
WorkingDirectory=/opt/orangepi-monitor
EnvironmentFile=-/etc/default/monitor
ExecStart=/opt/orangepi-monitor/monitor_server
Restart=always
RestartSec=5
# 日志走 journald，查看: sudo journalctl -u monitor -f

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now monitor
```

### 6.2 Cloudflare 配置
1. 域名托管在 Cloudflare（`<your-domain>`）
2. 板子安装 cloudflared，用 token 接入隧道
3. 加 Public Hostname：
   - `orangepi-monitor.<your-domain>` → `http://localhost:8080`
   - `ssh.<your-domain>` → `ssh://localhost:22`（两项目共用）
4. Access → Service Auth 创建 Service Token（记下 Client ID/Secret）
5. Access → 为 `ssh.<your-domain>` 配置 `ci-deploy` 策略（Service Auth）

### 6.3 GitHub 配置
1. 仓库加两个 Secret（与 remote-wakeup 共用同一份值）：
   - `BOARD_SSH_KEY` = 板子 `~/.ssh/deploy` 私钥全文
   - `CLOUDFLARED_TOKEN` = `ClientID:ClientSecret`
2. 推送 `.github/workflows/deploy.yml`（push main 自动部署）

### 6.4 验证
推一次代码到 main，观察 GitHub Actions 的 Build & Deploy 是否 success，
板子 `/opt/orangepi-monitor/monitor_server` 是否更新、服务是否重启。

## 7. 日常维护

```bash
# 查看服务
systemctl status monitor
# 查看日志
journalctl -u monitor -f
# 手动重启
sudo systemctl restart monitor
# 改配置（环境变量）
sudo nano /etc/default/monitor && sudo systemctl restart monitor
```

## 8. 回滚

CI 部署的是云端编译的固定版本二进制。回滚方式：
- 用 `git revert` 回退代码后 push（触发重新部署旧版）
- 或手动替换 `/opt/orangepi-monitor/monitor_server` 为上一版二进制并重启
