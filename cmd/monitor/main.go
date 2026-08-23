// Command monitor runs the Orange Pi system monitor: it collects system
// metrics in a background loop and serves the embedded web dashboard.
package main

import (
	"os"

	"monitor/internal/monitor"
)

func main() {
	server := monitor.NewServer()
	addr := os.Getenv("MONITOR_LISTEN_ADDR")
	if addr == "" {
		// Bind to loopback by default; expose publicly via Cloudflare tunnel.
		addr = "127.0.0.1:8080"
	}
	server.Start(addr)
}
