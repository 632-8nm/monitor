# AGENTS.md — orangepi-monitor 开发指南（供 AI agent 阅读）

本文件面向在本仓库工作的 AI 编码助手（ZCode 等），沉淀项目的架构决策、代码约定与历史结论。**新会话请先通读本文件**，避免重新讨论已经拍板的问题。

## 项目是什么

Orange Pi Zero3（全志 H618，4 核 Cortex-A53，2GB 内存，Debian Bookworm）上的系统监控服务：Go 单二进制（前端 `go:embed` 内嵌）+ systemd 常驻 + Cloudflare Tunnel 公网暴露 + GitHub Actions 全自动部署。生产实例运行在用户家中的板子上。

## 硬性规矩（违反即返工）

1. **所有 git 写操作（add/commit/push/tag）必须先获得用户明确许可**。用户会逐次下达"提交推送"类指令。
2. **仓库中不得出现真实域名**（曾做过两次全历史重写清除）。一律用 `<your-domain>` / `your-domain.example` 占位。`deploy.yml` 中的隧道 SSH 主机名是唯一保留的功能性例外。
3. **提交信息不得携带 Co-Authored-By: Claude 或任何 AI 署名尾注**（用户明确要求 GitHub 不出现 AI Contributor）。提交信息用英文 conventional commits 风格（feat/fix/perf/refactor/docs）。
4. 语言分区：**Go 源码注释、CI 配置、shell 脚本逻辑用英文；README/DEPLOYMENT/前端 UI 用中文**。推送到微信的告警文案用中文。
5. 全仓库 UTF-8 无 BOM、LF 行尾（`.gitattributes` 强制）。bash 脚本被 CRLF 毁掉过，写文件后必须校验。

## 目录结构与构建

```
cmd/monitor/main.go        # 唯一 main 包，读 MONITOR_LISTEN_ADDR 并启动
internal/monitor/          # 核心库（package monitor，不对外暴露）
  sensor.go                # 采集器：快档 2s / 慢档 10s 双 ticker 单 goroutine
  history.go               # 24h 趋势环形缓冲（固定 8640 点，纯内存约 240KB，重启即清——刻意不持久化）
  server.go                # HTTP：/api/stats /api/history /api/system + 内嵌前端
  alert.go                 # Server酱 告警：阈值 + 滞回 + 冷却 + 恢复通知
  probe.go                 # 外网连通性：每 30s TCP 握手公共 DNS
  sysinfo.go               # 静态系统信息（启动读一次）
  assets.go                # //go:embed web 声明
  web/                     # 前端源（必须留在本包内，embed 路径相对源文件）
build.sh / install.sh / uninstall.sh   # 源码构建/安装预编译包/卸载
.github/workflows/deploy.yml           # push main → 云编译 → 隧道 SSH 部署到板子
.github/workflows/release.yml          # push v* tag → 打包发 GitHub Release
```

构建命令：`go build -trimpath -ldflags="-w -X orangepi-monitor/internal/monitor.Version=<版本>" ./cmd/monitor`。版本注入路径含包全路径，移动包时必须同步改 deploy.yml / release.yml / build.sh 三处。提交前跑 `gofmt -l . && go vet ./...`。

前端改动要 bump 缓存版本号（`web/index.html` 里 `app.js?v=N` / `style.css?v=N`）。

## 关键设计决策（不要推翻，除非用户要求）

- **快/慢两档采集**：CPU/内存/负载/速率在 2s 档；TCP 连接表解析（单次最贵）、温度、磁盘占用、进程 TOP、挂载点在 10s 档。API 只读共享快照（互斥锁保护的 SystemStats，快档保留慢档字段），请求频率不影响采集。
- **进程 TOP5**：跨 tick 复用 `process.Process` 对象使 CPUPercent 算区间增量；只取进程名不碰命令行参数（隐私红线）。
- **WiFi**：该板驱动 `/proc/net/wireless` 不报 dBm（-256 占位），用 link quality（满值 70）。
- **温度区**：sysfs 枚举后只保留 type 含 cpu/npu 的区（ve/ddr 是无头板的噪音，用户拍板移除）；告警取各区最大值。
- **磁盘**：挂载点按设备去重（剔除 /var/log.hdd 这类 bind）；小分区自适应 MB 单位；I/O 质量从 IoTime/读写耗时差值算，只统计物理设备（mmc/sd/nvme/vd/hd 前缀）。
- **趋势图不持久化**是刻意取舍（用户知情），内存恒定 ~240KB 不会增长。持久化在路线图上但未排期。
- **敏感信息分级**（用户当前选择"暴露管理"而非鉴权）：进程名/内核版本等已在公网页面展示，这是用户明确接受的权衡，**不要反复劝告启用鉴权**；但登录用户名、IP、对端地址、SSID、MAC 永远不许上页面。
- `sysinfo.go` 的 OS/CPU 显示对齐 fastfetch 的取数逻辑（NAME 首词 + /etc/debian_version + 架构；CPU 用 device-tree compatible 最后一条去厂商前缀），主频用静态 cpuinfo_max_freq，实时频率只在处理器卡。

## 环境变量（/etc/default/monitor）

`MONITOR_LISTEN_ADDR`（默认 127.0.0.1:8080）、`MONITOR_BASIC_AUTH_USER/PASS`（未设=无鉴权兼容模式）、`MONITOR_ALLOWED_ORIGINS`、`MONITOR_SERVERCHAN_KEY`（设了才启用告警）、`MONITOR_ALERT_TEMP/MEM/DISK`（默认 70/90/90，0 禁用）、`MONITOR_ALERT_COOLDOWN`（默认 30 分钟，保护免费版每日 5 条额度）。

## 部署链路（push main 后自动发生）

云端交叉编译 arm64 → cloudflared access tcp 隧道 SSH 到板子 → 停服务 → **tar 压缩单流传输（重试×3，先解压到 mktemp 临时目录再 mv，防截断）** → 把 `secrets.SERVERCHAN_KEY` 合并写入板上 env 文件（grep -v 旧行再追加，不动其他配置；必须与隧道同一步骤内，隧道随步骤销毁）→ 启动（重试×3）→ `systemctl is-active` 检查。失败兜底：重启旧版服务再退出，不留停机板子。

板上：`/opt/orangepi-monitor/monitor_server` 单文件，systemd 服务名 `monitor`（日志走 journald），`systemctl cat monitor` 可查现状。CI 部署不触碰 unit 文件——改 unit 需登板手动操作。

## 板子访问（局域网）

`ssh <user>@<board-lan-ip>`（密码登录；本机曾配置免密）。sudo 需要密码，sudoers 仅对 systemctl/journalctl/tee 免密（CI 专用）。Windows 开发机 SSH 老版本不支持 `StrictHostKeyChecking=accept_new`，用 `no`。跨平台测试可 `GOOS=linux GOARCH=arm64` 编译后 scp 到板子临时端口实跑验证（用完清理，勿碰 8080 正式服务）。

## Windows 开发机备忘

- Go 代理：`GOPROXY=https://goproxy.cn,direct`（默认代理被墙）。
- 本机跑监控：温度返回假值 45.5°C、thermals 为空、disk_busy 不可用（IoTime 不上报）——均为平台回退，正常。
- Git Bash 的 grep 单引号模式有解析怪癖，批量文本处理用 node 脚本更稳。
- 单元测试没有建立，验证靠 go vet + 本地/板端冒烟（curl API + 检查字段）。

## 路线图（用户已知晓、未排期）

历史持久化（重启不丢趋势，方案：每分钟落一个 JSON 快照、启动回载）；告警渠道扩展（Telegram/Bark）；鉴权（Basic Auth 代码就绪或 Cloudflare Access，用户明确搁置）；CI 加 lint/test 步骤；gopsutil v3→v4。
