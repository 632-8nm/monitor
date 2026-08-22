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
	OS         string `json:"os"` // e.g. Debian GNU/Linux 12.6 (bookworm) aarch64
	Kernel     string `json:"kernel"`
	BoardModel string `json:"board"` // /proc/device-tree/model
	CPUModel   string `json:"cpu"`   // SoC from device-tree compatible
	Cores      int    `json:"cores"`
	Version    string `json:"version"` // monitor build version
}

func readSystemInfo() SystemInfo {
	info := SystemInfo{
		Cores:   runtime.NumCPU(),
		Version: Version,
	}

	if hi, err := host.Info(); err == nil {
		info.Kernel = hi.KernelVersion
		info.OS = readOSName(hi)
	}

	// Board model lives in the device tree with a trailing NUL byte
	if raw, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		info.BoardModel = strings.TrimRight(string(raw), "\x00")
	}

	info.CPUModel = readCPUModel()

	return info
}

// readOSName builds a compact OS line: "<distro> <version> <arch>", e.g.
// "Debian 12.6 aarch64". The distro is the first word of os-release NAME
// (dropping the "GNU/Linux" suffix); the version prefers /etc/debian_version
// because VERSION_ID lacks the point release. Falls back to platform info
// on systems without os-release (e.g. Windows dev machines).
func readOSName(hi *host.InfoStat) string {
	fields := make(map[string]string)
	if raw, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if k, v, ok := strings.Cut(line, "="); ok {
				fields[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
			}
		}
	}

	version := fields["VERSION_ID"]
	if raw, err := os.ReadFile("/etc/debian_version"); err == nil {
		if dv := strings.TrimSpace(string(raw)); dv != "" {
			version = dv
		}
	}

	var parts []string
	if name := fields["NAME"]; name != "" {
		distro := strings.Fields(name)[0]
		parts = append(parts, distro)
	}
	if version != "" {
		parts = append(parts, version)
	}
	if len(parts) == 0 {
		return strings.TrimSpace(hi.Platform + " " + hi.PlatformVersion)
	}
	name := strings.Join(parts, " ")
	if hi.KernelArch != "" {
		name += " " + hi.KernelArch
	}
	return name
}

// readCPUModel returns the SoC name the way fastfetch derives it: the last
// entry of /proc/device-tree/compatible with the vendor prefix stripped
// ("allwinner,sun50i-h616" → "sun50i-h616"; the board entry comes first,
// the SoC last). Falls back to the cpuinfo model name.
func readCPUModel() string {
	if raw, err := os.ReadFile("/proc/device-tree/compatible"); err == nil {
		entries := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		if len(entries) > 0 {
			soc := entries[len(entries)-1]
			if i := strings.LastIndex(soc, ","); i >= 0 {
				soc = soc[i+1:]
			}
			if soc != "" {
				return soc
			}
		}
	}
	if ci, err := cpu.Info(); err == nil && len(ci) > 0 && ci[0].ModelName != "" {
		return strings.TrimSpace(ci[0].ModelName)
	}
	return ""
}
