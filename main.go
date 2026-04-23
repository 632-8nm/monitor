package main

import "os"

func main() {
	server := NewServer()
	addr := os.Getenv("MONITOR_LISTEN_ADDR")
	if addr == "" {
		// Bind to loopback by default; expose publicly via Cloudflare tunnel.
		addr = "127.0.0.1:8080"
	}
	server.Start(addr)
}
