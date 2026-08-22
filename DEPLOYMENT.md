# Deployment Architecture & Setup Guide (orangepi-monitor)

> This document records the full setup-from-scratch flow and the current production architecture, for maintainers.
> All sensitive values use placeholders — no real tokens or passwords are included.

## 1. Overall architecture

```
Developer machine (Windows)
   │  write code → go vet / go test (local checks)
   │  git commit + push
   ▼
GitHub (orangepi-monitor repository)
   │  triggers GitHub Actions (workflow: deploy.yml)
   ▼
CI/CD (GitHub cloud runner, ubuntu-latest)
   │  ① cross-compile arm64 binary (CGO_ENABLED=0 GOOS=linux GOARCH=arm64)
   │  ② SSH into the board via Cloudflare Tunnel (ssh.<your-domain>)
   ▼
Board (Orange Pi Zero 3)
   │  stop service → scp binary → restart
   ▼
/opt/orangepi-monitor/  (systemd: monitor, listening on :8080)
```

> Note: the frontend is embedded into the binary (`go:embed`); deploys ship
> the single `monitor_server` executable.

## 2. Production deployment layout

| Item | Value |
|---|---|
| Deploy directory | `/opt/orangepi-monitor/` |
| Binary | `/opt/orangepi-monitor/monitor_server` |
| Frontend | embedded in the binary (`go:embed`) |
| systemd service | `monitor.service` |
| Listen port | `8080` |
| Public entry | `orangepi-monitor.<your-domain>` (Cloudflare Tunnel → http://localhost:8080) |
| CI deploy entry | `ssh.<your-domain>` (Cloudflare Tunnel → ssh://localhost:22) |

## 3. Board-side components

| Component | Description | State |
|---|---|---|
| `cloudflared` | Cloudflare Tunnel client (token mode) | systemd service, always on |
| `/opt/orangepi-monitor/` | production deploy directory | updated by CI |
| `monitor.service` | systemd unit pointing to `/opt/orangepi-monitor/monitor_server` | always on |

## 4. Cloudflare-side configuration

| Item | Configuration | Purpose |
|---|---|---|
| Domain | `<your-domain>` | hosted on Cloudflare |
| Tunnel | token mode (`tunnel run --token`) | the board connects out to Cloudflare |
| Public Hostname | `orangepi-monitor.<your-domain>` → `http://localhost:8080` | public web access |
| Public Hostname | `ssh.<your-domain>` → `ssh://localhost:22` | CI/CD deploy SSH entry (shared by both projects) |
| Service Token | created under Access → Service Auth | CI authentication (format ID:SECRET) |
| Access policy | `ci-deploy` (Service Auth + token) | allows CI's cloudflared connection |

## 5. GitHub-side configuration

| Item | Value | Description |
|---|---|---|
| Repository | `<your-org>/orangepi-monitor` | — |
| Workflow | `.github/workflows/deploy.yml` | push to main triggers cloud build + deploy |
| Workflow | `.github/workflows/release.yml` | push a `v*` tag builds & publishes the release tarball |
| Secret: `BOARD_SSH_KEY` | board's `~/.ssh/deploy` private key | cloud SSH login to the board (shared by both projects) |
| Secret: `CLOUDFLARED_TOKEN` | `ClientID:ClientSecret` | cloudflared access authentication (shared by both projects) |

## 6. Setup from scratch

### 6.1 Prepare the board
```bash
# Install cloudflared
curl -L --output /usr/local/bin/cloudflared \
  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64
chmod +x /usr/local/bin/cloudflared

# Passwordless sudo (systemctl/journalctl/tee only, used by CI)
sudo tee /etc/sudoers.d/orangepi-systemd <<'EOF'
orangepi ALL=(ALL) NOPASSWD: /usr/bin/systemctl, /bin/systemctl, /usr/bin/journalctl, /usr/bin/tee
EOF
sudo chmod 440 /etc/sudoers.d/orangepi-systemd

# Generate the CI deploy key
ssh-keygen -t ed25519 -N "" -f ~/.ssh/deploy
cat ~/.ssh/deploy.pub >> ~/.ssh/authorized_keys

# Create the production deploy directory
sudo mkdir -p /opt/orangepi-monitor
sudo chown orangepi:orangepi /opt/orangepi-monitor

# Configure the systemd service (log output goes to journald; do not point
# WorkingDirectory at the removed actions-runner directory)
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
# Logs go to journald; view with: sudo journalctl -u monitor -f

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now monitor
```

### 6.2 Cloudflare configuration
1. Domain hosted on Cloudflare (`<your-domain>`)
2. cloudflared installed on the board, joined to the tunnel with a token
3. Add Public Hostnames:
   - `orangepi-monitor.<your-domain>` → `http://localhost:8080`
   - `ssh.<your-domain>` → `ssh://localhost:22` (shared by both projects)
4. Access → Service Auth: create a Service Token (note the Client ID/Secret)
5. Access → attach a `ci-deploy` policy (Service Auth) to `ssh.<your-domain>`

### 6.3 GitHub configuration
1. Add two repository secrets (values shared with remote-wakeup):
   - `BOARD_SSH_KEY` = full text of the board's `~/.ssh/deploy` private key
   - `CLOUDFLARED_TOKEN` = `ClientID:ClientSecret`
2. Push `.github/workflows/deploy.yml` (pushing to main deploys automatically)

### 6.4 Verify
Push a commit to main and check whether the GitHub Actions "Build & Deploy" run succeeds,
whether `/opt/orangepi-monitor/monitor_server` on the board was updated, and whether the service restarted.

## 7. Day-to-day maintenance

```bash
# Check service
systemctl status monitor
# Follow logs
journalctl -u monitor -f
# Manual restart
sudo systemctl restart monitor
# Change configuration (environment variables)
sudo nano /etc/default/monitor && sudo systemctl restart monitor
```

## 8. Rollback

CI deploys fixed-version binaries built in the cloud. To roll back:
- `git revert` the offending change and push (redeploys the previous version)
- or manually replace `/opt/orangepi-monitor/monitor_server` with the previous binary and restart
