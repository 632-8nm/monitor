package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// collectInterval is the background sampling period. CPU usage and network/disk
// I/O rates are derived from deltas between consecutive samples, so a fixed
// period keeps the results independent of API request frequency.
const collectInterval = 2 * time.Second

type SystemStats struct {
	CPUTemp      string  `json:"cpu_temp"`
	CPUUsage     float64 `json:"cpu_usage"`
	CPUFreq      float64 `json:"cpu_freq"`
	Load1        float64 `json:"load_1"`
	Load5        float64 `json:"load_5"`
	Load15       float64 `json:"load_15"`
	MemUsage     float64 `json:"mem_usage"`
	MemSummary   string  `json:"mem_summary"`
	SwapUsage    float64 `json:"swap_usage"`
	SwapSummary  string  `json:"swap_summary"`
	DiskUsage    float64 `json:"disk_usage"`
	DiskSummary  string  `json:"disk_summary"`
	NetDown      float64 `json:"net_down"`
	NetUp        float64 `json:"net_up"`
	Connections  uint64  `json:"connections"`
	MemAvailable uint64  `json:"mem_available"`
	MemCached    uint64  `json:"mem_cached"`
	DiskRead     float64 `json:"disk_read"`
	DiskWrite    float64 `json:"disk_write"`
	Uptime       uint64  `json:"uptime"`
}

type Collector struct {
	mu      sync.Mutex
	current SystemStats
	history *history

	// Delta state below is only accessed from the collector goroutine; no lock needed
	prevNetRecv   uint64
	prevNetSent   uint64
	prevDiskRead  uint64
	prevDiskWrite uint64
	lastUpdate    time.Time
}

// Start launches the background collection loop: one synchronous sample first
// establishes the delta baseline, then snapshots refresh at a fixed period;
// API requests read them through Snapshot.
func (c *Collector) Start() {
	c.collect()
	go func() {
		ticker := time.NewTicker(collectInterval)
		defer ticker.Stop()
		for range ticker.C {
			c.collect()
		}
	}()
}

// Snapshot returns the most recently collected system stats
func (c *Collector) Snapshot() SystemStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// HistorySnapshot returns the recorded trend points for /api/history
func (c *Collector) HistorySnapshot() historySnapshot {
	return c.history.snapshot()
}

func (c *Collector) GetCPUTemp() string {
	if runtime.GOOS == "windows" {
		return "45.5°C"
	}
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return "N/A"
	}
	raw := strings.TrimSpace(string(data))
	if len(raw) >= 3 {
		return fmt.Sprintf("%s.%s°C", raw[:2], raw[2:3])
	}
	return raw + "°C"
}

func (c *Collector) GetCPUFreq() float64 {
	data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if err == nil {
		var freq int
		fmt.Sscanf(string(data), "%d", &freq)
		return float64(freq) / 1000.0
	}
	info, _ := cpu.Info()
	if len(info) > 0 {
		return info[0].Mhz
	}
	return 0
}

func (c *Collector) collect() {
	// Interval 0: compute the delta against the previous call (i.e. the previous
	// sampling period). Since this function is only invoked by the fixed-period
	// background loop, the interval is constant.
	cpuPercent, _ := cpu.Percent(0, false)
	v, _ := mem.VirtualMemory()
	swap, _ := mem.SwapMemory()
	loadAvg, _ := load.Avg()
	diskStat, _ := disk.Usage("/")
	uptime, _ := host.Uptime()

	now := time.Now()
	prevUpdate := c.lastUpdate
	duration := now.Sub(prevUpdate).Seconds()

	// Network rates
	io, _ := net.IOCounters(false)
	var downSpeed, upSpeed float64
	if len(io) > 0 {
		if duration > 0 && !prevUpdate.IsZero() {
			downSpeed = float64(io[0].BytesRecv-c.prevNetRecv) / 1024 / duration
			upSpeed = float64(io[0].BytesSent-c.prevNetSent) / 1024 / duration
		}
		c.prevNetRecv = io[0].BytesRecv
		c.prevNetSent = io[0].BytesSent
	}

	connections, _ := net.Connections("tcp")
	connCount := uint64(len(connections))

	// Disk I/O rates
	diskIO, _ := disk.IOCounters()
	var diskReadSpeed, diskWriteSpeed float64
	var totalDiskRead, totalDiskWrite uint64
	for _, d := range diskIO {
		totalDiskRead += d.ReadBytes
		totalDiskWrite += d.WriteBytes
	}
	if duration > 0 && !prevUpdate.IsZero() {
		diskReadSpeed = float64(totalDiskRead-c.prevDiskRead) / 1024 / duration
		diskWriteSpeed = float64(totalDiskWrite-c.prevDiskWrite) / 1024 / duration
	}
	c.prevDiskRead = totalDiskRead
	c.prevDiskWrite = totalDiskWrite

	c.lastUpdate = now

	usage := 0.0
	if len(cpuPercent) > 0 {
		usage = cpuPercent[0]
	}

	load1, load5, load15 := 0.0, 0.0, 0.0
	if loadAvg != nil {
		load1, load5, load15 = loadAvg.Load1, loadAvg.Load5, loadAvg.Load15
	}

	stats := SystemStats{
		CPUTemp:      c.GetCPUTemp(),
		CPUUsage:     usage,
		CPUFreq:      c.GetCPUFreq(),
		Load1:        load1,
		Load5:        load5,
		Load15:       load15,
		MemUsage:     v.UsedPercent,
		MemSummary:   fmt.Sprintf("%.2f / %.2f GB", float64(v.Used)/1e9, float64(v.Total)/1e9),
		MemAvailable: v.Available,
		MemCached:    v.Cached,
		SwapUsage:    swap.UsedPercent,
		SwapSummary:  fmt.Sprintf("%.2f / %.2f GB", float64(swap.Used)/1e9, float64(swap.Total)/1e9),
		DiskUsage:    diskStat.UsedPercent,
		DiskSummary:  fmt.Sprintf("%.2f / %.2f GB", float64(diskStat.Used)/1e9, float64(diskStat.Total)/1e9),
		DiskRead:     diskReadSpeed,
		DiskWrite:    diskWriteSpeed,
		NetDown:      downSpeed,
		NetUp:        upSpeed,
		Connections:  connCount,
		Uptime:       uptime,
	}

	c.mu.Lock()
	c.current = stats
	c.mu.Unlock()

	c.history.maybeAppend(stats, now)
}
