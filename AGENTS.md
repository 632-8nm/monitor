# AGENTS.md — monitor 开发指南（供 AI agent 阅读）

本文件面向在本仓库工作的 AI 编码助手（ZCode 等），沉淀项目的架构决策、代码约定与历史结论。**新会话请先通读本文件**，避免重新讨论已经拍板的问题。

> 项目原名 orangepi-monitor，已全面改名 monitor（仓库/module/包名/板上部署目录 `/opt/monitor`/发版包名）。文档中 Cloudflare 子域占位符写作 `monitor.<your-domain>`。

## 项目是什么

轻量系统监控服务：Go 单二进制（前端 `go:embed` 内嵌）+ systemd 常驻 + Cloudflare Tunnel 公网暴露 + GitHub Actions 全自动部署。**多平台支持**：linux/arm64（生产板 Orange Pi Zero3，全志 H618，2GB 内存，Debian Bookworm）、linux/amd64（PC/服务器）、其他 arm64 板（如 RK3576）；Windows 可编译运行（部分指标平台回退）。生产实例运行在用户家中的板子上。

## 硬性规矩（违反即返工）

1. **所有 git 写操作（add/commit/push/tag）必须先获得用户明确许可**。用户会逐次下达"提交推送"类指令。
2. **仓库中不得出现真实域名**（曾做过两次全历史重写清除）。一律用 `<your-domain>` / `your-domain.example` 占位。`deploy.yml` 中的隧道 SSH 主机名是唯一保留的功能性例外。
3. **提交信息不得携带 Co-Authored-By: Claude 或任何 AI 署名尾注**。提交信息用英文 conventional commits 风格（feat/fix/perf/refactor/docs）。
4. 语言分区：**Go 源码注释、CI 配置、shell 脚本逻辑用英文；README/DEPLOYMENT/前端 UI 用中文**。告警文案用中文（前缀自动带主板型号）。
5. 全仓库 UTF-8 无 BOM、LF 行尾（`.gitattributes` 强制）。写文件后必须校验。

## 目录结构与构建

```
cmd/monitor/main.go        # 唯一 main 包，读 MONITOR_LISTEN_ADDR 并启动
internal/monitor/          # 核心库（package monitor，不对外暴露）
  sensor.go                # 采集器：快档 2s / 慢档 10s 双 ticker 单 goroutine
  history.go               # 24h 趋势环形缓冲（固定 8640 点，纯内存约 240KB，重启即清——刻意不持久化）
  server.go                # HTTP：/api/stats /api/history /api/system + 内嵌前端
  alert.go                 # Server酱 告警：阈值 + 滞回 + 冷却 + 恢复通知
  probe.go                 # 外网连通性：每 30s TCP 握手公共 DNS
  sysinfo.go               # 静态系统信息（启动读一次，fastfetch 同源取数）
  assets.go                # //go:embed web 声明
  web/                     # 前端源（必须留在本包内，embed 路径相对源文件）
build.sh / install.sh / uninstall.sh   # 源码构建（支持 BUILD_ARCH 交叉编译）/安装预编译包/卸载
.github/workflows/deploy.yml           # push main → 多平台编译检查 → 云编译 → 隧道 SSH 部署到板子
.github/workflows/release.yml          # push v* tag → 多架构打包发 GitHub Release（linux arm64+amd64）
```

构建命令：`go build -trimpath -ldflags="-w -X monitor/internal/monitor.Version=<版本>" ./cmd/monitor`。版本注入路径含包全路径，移动包或改 module 名时必须同步改 deploy.yml / release.yml / build.sh 三处。提交前跑 `gofmt -l . && go vet ./...`。

前端改动要 bump 缓存版本号（`web/index.html` 里 `app.js?v=N` / `style.css?v=N`）。

## 关键设计决策（不要推翻，除非用户要求）

- **快/慢两档采集**：CPU/内存/负载/速率在 2s 档；TCP 连接表解析（单次最贵）、温度、磁盘占用、进程 TOP、挂载点、WiFi 在 10s 档。API 只读共享快照（互斥锁保护的 SystemStats，快档保留慢档字段），请求频率不影响采集。
- **进程 TOP5**：跨 tick 复用 `process.Process` 对象使 CPUPercent 算区间增量；只取进程名不碰命令行参数（隐私红线）。
- **WiFi**：`/proc/net/wireless` 的 dBm 有效（-20 ~ -200）时优先用 dBm 分级，占位值（-256，H618 老驱动）时退回 link quality（满值 70）。
- **温度区**：sysfs 枚举按 `MONITOR_THERMAL_ZONES`（默认 `cpu,npu`，子串匹配）过滤，如 rockchip 可设 `cpu,npu,soc`；告警取各区最大值。
- **磁盘**：挂载点按设备去重（剔除 /var/log.hdd 这类 bind）；小分区自适应 MB 单位；I/O 质量从 IoTime/读写耗时差值算，只统计物理设备（mmc/sd/nvme/vd/hd 前缀）。
- **趋势图不持久化**是刻意取舍（用户知情），内存恒定 ~240KB 不会增长。持久化在路线图上但未排期。
- **敏感信息分级**（用户当前选择"暴露管理"而非鉴权）：进程名/内核版本等已在公网页面展示，这是用户明确接受的权衡，**不要反复劝告启用鉴权**；但登录用户名、IP、对端地址、SSID、MAC 永远不许上页面。
- `sysinfo.go` 的 OS/CPU 显示对齐 fastfetch 的取数逻辑（NAME 首词 + /etc/debian_version + 架构；CPU 用 device-tree compatible 最后一条去厂商前缀），主频用静态 cpuinfo_max_freq，实时频率只在处理器卡。页面标题与告警前缀动态使用主板型号。

## 环境变量（/etc/default/monitor）

`MONITOR_LISTEN_ADDR`（默认 127.0.0.1:8080）、`MONITOR_BASIC_AUTH_USER/PASS`（未设=无鉴权兼容模式）、`MONITOR_ALLOWED_ORIGINS`、`MONITOR_SERVERCHAN_KEY`（设了才启用告警）、`MONITOR_ALERT_TEMP/MEM/DISK`（默认 70/90/90，0 禁用）、`MONITOR_ALERT_COOLDOWN`（默认 30 分钟，保护免费版每日 5 条额度）、`MONITOR_THERMAL_ZONES`（默认 cpu,npu）、`MONITOR_TERMINAL`（=1 启用网页终端；WS↔PTY 桥在 terminal.go，仅 Linux 有 PTY，Windows 返回 unavailable）。`MONITOR_ADMIN_USER/PASS`（默认 admin/123456——**内置默认口令，必须要求部署时改**）管理 admin 后台（admin.go 的内存会话，HttpOnly cookie）：**网页终端收在 admin 登录后面**，未登录点终端按钮先弹管理员登录表单，登录后才开 PTY；`/api/admin/session|login|logout` 三个端点 + 终端标题栏登出按钮。告警文案与前端 UI 用中文，admin.go 内网 API 复用同一套 session 校验。

## 部署链路（push main 后自动发生）

多平台编译检查（linux amd64/arm64 + windows amd64）→ 云端交叉编译 arm64 → cloudflared access tcp 隧道 SSH 到板子 → 停服务 → **tar 压缩单流传输（重试×3，先解压到 mktemp 临时目录再 mv，防截断）** → 把 `secrets.SERVERCHAN_KEY` 合并写入板上 env 文件（grep -v 旧行再追加；必须与隧道同一步骤内，隧道随步骤销毁）→ 启动（重试×3）→ `systemctl is-active` 检查。失败兜底：重启旧版服务再退出，不留停机板子。

板上：`/opt/monitor/monitor_server` 单文件，systemd 服务名 `monitor`（日志走 journald）。CI 部署不触碰 unit 文件——改 unit 需登板手动操作。

## 板子访问（局域网）

`ssh <user>@<board-lan-ip>`（密码登录；本机曾配置免密）。sudo 需要密码，sudoers 仅对 systemctl/journalctl/tee 免密（CI 专用）。Windows 开发机 SSH 老版本不支持 `StrictHostKeyChecking=accept_new`，用 `no`。跨平台测试可 `GOOS=linux GOARCH=arm64` 编译后 scp 到板子临时端口实跑验证（用完清理，勿碰 8080 正式服务）。

## Windows 开发机备忘

- Go 代理：`GOPROXY=https://goproxy.cn,direct`（默认代理被墙）。
- 本机跑监控：温度返回假值 45.5°C、thermals 为空、disk_busy 不可用（IoTime 不上报）——均为平台回退，正常。
- Git Bash 的 grep 单引号模式有解析怪癖，批量文本处理用 node 脚本更稳；复杂替换优先用编辑工具而非多层转义的命令行脚本。
- 单元测试没有建立，验证靠 go vet + 本地/板端冒烟（curl API + 检查字段）。

## 路线图（用户已知晓、未排期）

历史持久化（重启不丢趋势，方案：每分钟落一个 JSON 快照、启动回载）；告警渠道扩展（Telegram/Bark）；鉴权（Basic Auth 代码就绪或 Cloudflare Access，用户明确搁置）；CI 加 lint/test 步骤；gopsutil v3→v4；更多发版架构（linux/arm、windows/amd64）按需加矩阵即可。
