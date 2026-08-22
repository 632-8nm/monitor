# Orange Pi Zero3 System Monitor

This repository manages the system monitor program for an Orange Pi Zero3, with automated build and continuous deployment powered by GitHub Actions.

**🌐 Live demo:** [https://orangepi-monitor.your-domain.example/](https://orangepi-monitor.your-domain.example/)

> 📦 **For the deployment architecture and setup-from-scratch guide, see [DEPLOYMENT.md](DEPLOYMENT.md)** (CI/CD, Cloudflare Tunnel, /opt deployment, secrets)

## Features

* **Real-time monitoring**: the web dashboard shows CPU load, frequency and temperature, memory usage, uptime and live network speeds.
* **Fully automated deployment**: GitHub Actions cross-compiles the arm64 binary in the cloud and deploys it to the board over a Cloudflare Tunnel — just push to `main`.
* **Self-hosted frontend**: pages are served directly by the Go backend, no separate static hosting required.
* **Power-loss resilient**: both the monitor and the tunnel client run as systemd services with auto-start on boot.

## Architecture

* **Backend**: Go (Golang) + `gopsutil` (serves both the API and the static frontend)
* **Frontend**: HTML5 / CSS3 / vanilla JavaScript
* **Service management**: systemd (service name: `monitor`)
* **Tunneling**: Cloudflare Tunnel (custom domain)
* **Domain**: your-domain.example

## Operations

* **Check monitor status**: `sudo systemctl status monitor`
* **Follow logs**: `sudo journalctl -u monitor -f`
* **Restart the monitor**: `sudo systemctl restart monitor`
* **Check tunnel status**: `sudo systemctl status cloudflared`
* **Restart the tunnel**: `sudo systemctl restart cloudflared`
* **Install/upgrade from source**: `./install.sh` (builds locally and installs into `/opt/orangepi-monitor`; override with `INSTALL_DIR`; re-running performs an upgrade)
* **Uninstall**: `./uninstall.sh` (reverts everything `install.sh` did; does not touch the cloudflared tunnel)

## Redeployment

Every push to GitHub `main` triggers GitHub Actions to:
1. Cross-compile the Go binary on a cloud runner
2. Stop the service on the board
3. Replace the binary and frontend files
4. Restart the service

No manual intervention required.

## Install from source (other devices)

To self-host on another board without this repository's CI/CD:

```bash
git clone <this repo> && cd orangepi-monitor
./install.sh
```

The script builds locally, installs into `/opt/orangepi-monitor`, registers the systemd service (`monitor`) and enables it on boot. By default it only listens on `127.0.0.1:8080`. For security options (Basic Auth / CORS) see the "Security" section below. To remove everything, run `./uninstall.sh`.

## Service configuration

* **Monitor service**: `/etc/systemd/system/monitor.service`
* **Tunnel service**: `/etc/systemd/system/cloudflared.service`
* **Port**: 8080
* **Public URL**: https://orangepi-monitor.your-domain.example/

## Security (recommended for production)

Basic security capabilities can be enabled via environment variables:

* `MONITOR_LISTEN_ADDR`: listen address (default `127.0.0.1:8080`, keeping the default is recommended)
* `MONITOR_BASIC_AUTH_USER`: Basic Auth username
* `MONITOR_BASIC_AUTH_PASS`: Basic Auth password
* `MONITOR_ALLOWED_ORIGINS`: CORS allowlist, comma-separated (e.g. `https://orangepi-monitor.your-domain.example/`)

When `MONITOR_BASIC_AUTH_USER/PASS` are not set, the server runs in compatibility mode (no authentication).
When `MONITOR_ALLOWED_ORIGINS` is not set, the server runs with permissive CORS.

## Highlights

1. **All-in-one backend**: a single Go binary serves both the API and static files, keeping the architecture simple
2. **Cloudflare Tunnel**: no public IP needed — the board reaches the internet through Cloudflare
3. **Custom domain**: a stable entry point on the your-domain.example domain
4. **systemd managed**: reliable services with auto-start on boot and automatic recovery
