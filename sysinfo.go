package monitor

import (
	"os"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
)

// Version is the monitor's own build version, injected at compile time via
// -ldflags "-X orangepi-monitor.Version=..."; "dev" for local builds.
var Version = "dev"

// SystemInfo is the static machine identity, read once at startup and served
// through /api/system. Deliberately excludes hostname and IP addresses.
type SystemInfo struct {
	OS         string `json:"os"` // PRETTY_NAME from /etc/os-release
	Kernel     string `json:"kernel"`
	BoardModel string `json:"board"` // /proc/device-tree/model
	CPUModel   string `json:"cpu"`   // model name from cpuinfo
	Cores      int    `json:"cores"`
	Version    string `json:"version"` // monitor build version
}

func readSystemInfo() SystemInfo {
	info := SystemInfo{
		Cores:   runtime.NumCPU(),
		Version: Version,
	}

	if raw, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
				info.OS = strings.Trim(strings.TrimSpace(v), `"`)
			}
		}
	}

	if hi, err := host.Info(); err == nil {
		info.Kernel = hi.KernelVersion
		if info.OS == "" {
			info.OS = strings.TrimSpace(hi.Platform + " " + hi.PlatformVersion)
		}
	}

	// Board model lives in the device tree with a trailing NUL byte
	if raw, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		info.BoardModel = strings.TrimRight(string(raw), "\x00")
	}

	if ci, err := cpu.Info(); err == nil && len(ci) > 0 && ci[0].ModelName != "" {
		info.CPUModel = strings.TrimSpace(ci[0].ModelName)
	}

	return info
}
