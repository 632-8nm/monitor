package monitor

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	// Fast tier (2s): the metrics that create the "live" feel — CPU,
	// memory, load and rate deltas. Costs a handful of /proc reads.
	collectInterval = 2 * time.Second
	// Slow tier (10s): slow-moving metrics — TCP table parsing is the
	// single most expensive collection step, while temps/disk usage
	// barely change between samples.
	slowInterval = 10 * time.Second
)

type SystemStats struct {
	CPUTemp      string        `json:"cpu_temp"`
	CPUUsage     float64       `json:"cpu_usage"`
	CPUUser      float64       `json:"cpu_user"`
	CPUSys       float64       `json:"cpu_sys"`
	CPUIOWait    float64       `json:"cpu_iowait"`
	CPUIdle      float64       `json:"cpu_idle"`
	CPUFreq      float64       `json:"cpu_freq"`
	Cores        []float64     `json:"cpu_cores"`
	Thermals     []ThermalZone `json:"thermals"`
	Load1        float64       `json:"load_1"`
	Load5        float64       `json:"load_5"`
	Load15       float64       `json:"load_15"`
	MemUsage     float64       `json:"mem_usage"`
	MemSummary   string        `json:"mem_summary"`
	SwapUsage    float64       `json:"swap_usage"`
	SwapSummary  string        `json:"swap_summary"`
	DiskUsage    float64       `json:"disk_usage"` // root mount, used by alerting
	DiskSummary  string        `json:"disk_summary"`
	DiskMounts   []MountUsage  `json:"disk_mounts"`
	DiskBusy     float64       `json:"disk_busy"`       // % of time busy over the last slow interval
	DiskLatency  float64       `json:"disk_latency_ms"` // average I/O latency
	DiskIOPS     float64       `json:"disk_iops"`
	WifiLink     float64       `json:"wifi_link"` // link quality (0 = no wireless)
	TopProcs     []ProcInfo    `json:"top_procs"`
	NetDown      float64       `json:"net_down"`
	NetUp        float64       `json:"net_up"`
	NetTotalDown uint64        `json:"net_total_down"`
	NetTotalUp   uint64        `json:"net_total_up"`
	NetDropped   uint64        `json:"net_dropped"`
	NetErrors    uint64        `json:"net_errors"`
	NetOnline    bool          `json:"net_online"`
	NetLatencyMs float64       `json:"net_latency_ms"`
	Connections  uint64        `json:"connections"`
	ConnEstab    uint64        `json:"conn_established"`
	ConnListen   uint64        `json:"conn_listen"`
	ConnTimeWait uint64        `json:"conn_timewait"`
	MemAvailable uint64        `json:"mem_available"`
	MemCached    uint64        `json:"mem_cached"`
	DiskRead     float64       `json:"disk_read"`
	DiskWrite    float64       `json:"disk_write"`
	Uptime       uint64        `json:"uptime"`
}

// ThermalZone is one sysfs thermal zone (cpu-thermal, npu-thermal, ... —
// whatever the board exposes), temperature in °C.
type ThermalZone struct {
	Type string  `json:"type"`
	Temp float64 `json:"temp"`
}

// MountUsage is the usage of one physical filesystem mount point.
type MountUsage struct {
	Mount   string  `json:"mount"`
	Usage   float64 `json:"usage"`
	Summary string  `json:"summary"`
}

// ProcInfo is one entry of the top-processes list — names and numbers only,
// never command-line arguments.
type ProcInfo struct {
	PID  int32   `json:"pid"`
	Name string  `json:"name"`
	CPU  float64 `json:"cpu"`
	Mem  float64 `json:"mem"`
}

type Collector struct {
	mu      sync.Mutex
	current SystemStats
	history *history
	probe   *netProbe

	// Delta state below is only accessed from the collector goroutine; no lock needed
	prevNetRecv   uint64
	prevNetSent   uint64
	prevDiskRead  uint64
	prevDiskWrite uint64
	lastUpdate    time.Time
	// Persistent process objects so CPUPercent() reports the delta since
	// the previous slow tick instead of a lifetime average
	procPrev map[int32]*process.Process
	// Previous cumulative cpu.Times for the user/sys/iowait split
	cpuPrev struct {
		ok                              bool
		user, sys, iowait, steal, total float64
	}
	// Previous cumulative disk stats for busy%/latency over the slow interval
	diskPrev struct {
		ok                  bool
		ioTime, rwTime, ops uint64
		at                  time.Time
	}
}

// Start launches the background collection loops: one synchronous sample of
// each tier first establishes the delta baseline and fills the snapshot,
// then fast (2s) and slow (10s) tickers refresh it from a single goroutine;
// API requests only read the snapshot. The internet probe runs on its own
// schedule.
func (c *Collector) Start() {
	c.probe.start()
	c.collectFast()
	c.collectSlow()
	go func() {
		fast := time.NewTicker(collectInterval)
		slow := time.NewTicker(slowInterval)
		defer fast.Stop()
		defer slow.Stop()
		for {
			select {
			case <-fast.C:
				c.collectFast()
			case <-slow.C:
				c.collectSlow()
			}
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

// usefulThermal reports whether a sysfs thermal zone is worth exposing.
// Only CPU/NPU sensors are kept: the video-engine and DDR-controller
// sensors sunxi SoCs also expose are idle noise on a headless board.
func usefulThermal(zoneType string) bool {
	t := strings.ToLower(zoneType)
	return strings.Contains(t, "cpu") || strings.Contains(t, "npu")
}

// physicalDisk reports whether a disk name is a real device rather than a
// loop device or zram, which would skew I/O quality numbers.
func physicalDisk(name string) bool {
	for _, p := range []string{"mmc", "sd", "nvme", "vd", "hd"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// readWiFiLink returns the wireless link quality from /proc/net/wireless
// (0 means no wireless interface). This board's driver reports quality out
// of 70 and leaves the dBm columns at placeholder values (-256), so quality
// is the only usable signal-strength metric.
func readWiFiLink() float64 {
	raw, err := os.ReadFile("/proc/net/wireless")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		iface := strings.TrimSuffix(fields[0], ":")
		if !strings.HasPrefix(iface, "wlan") && !strings.HasPrefix(iface, "wl") {
			continue
		}
		// fields: iface status link level noise ...
		quality, err := strconv.ParseFloat(fields[2], 64)
		if err != nil || quality <= 0 {
			continue
		}
		return quality
	}
	return 0
}

// fmtDiskSize renders used/total with a unit that fits small partitions
// (a 46 MB /var/log would read "0.0 GB" otherwise).
func fmtDiskSize(used, total float64) string {
	if total < 1e9 {
		return fmt.Sprintf("%.0f / %.0f MB", used/1e6, total/1e6)
	}
	return fmt.Sprintf("%.1f / %.1f GB", used/1e9, total/1e9)
}

// readThermals enumerates /sys/class/thermal/thermal_zone* so the dashboard
// can show the CPU/NPU temperature sources the board exposes.
// Returns nil on platforms without sysfs (e.g. Windows dev machines).
func readThermals() []ThermalZone {
	entries, err := os.ReadDir("/sys/class/thermal")
	if err != nil {
		return nil
	}
	var zones []ThermalZone
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "thermal_zone") {
			continue
		}
		base := "/sys/class/thermal/" + e.Name()
		typeRaw, err := os.ReadFile(base + "/type")
		if err != nil {
			continue
		}
		zoneType := strings.TrimSpace(string(typeRaw))
		if !usefulThermal(zoneType) {
			continue
		}
		tempRaw, err := os.ReadFile(base + "/temp")
		if err != nil {
			continue
		}
		var milli int
		if _, err := fmt.Sscanf(string(tempRaw), "%d", &milli); err != nil {
			continue
		}
		zones = append(zones, ThermalZone{Type: zoneType, Temp: float64(milli) / 1000.0})
	}
	return zones
}

// collectFast refreshes the live-feel metrics (CPU, memory, load, network
// and disk I/O rates) into the shared snapshot, preserving the slow-tier
// fields. Interval 0 for cpu.Percent computes the delta against the previous
// call, i.e. the previous fast period; only the fixed-period loop calls this,
// so the interval is constant.
func (c *Collector) collectFast() {
	perCore, _ := cpu.Percent(0, true)
	usage := 0.0
	for _, v := range perCore {
		usage += v
	}
	if len(perCore) > 0 {
		usage /= float64(len(perCore))
	}

	// CPU time breakdown (user/sys/iowait/idle) from cumulative deltas
	var userPct, sysPct, ioPct, idlePct float64
	if times, err := cpu.Times(false); err == nil && len(times) > 0 {
		t := times[0]
		total := t.Total()
		if c.cpuPrev.ok && total > c.cpuPrev.total {
			elapsed := total - c.cpuPrev.total
			userPct = ((t.User + t.Nice) - c.cpuPrev.user) / elapsed * 100
			sysPct = (t.System - c.cpuPrev.sys) / elapsed * 100
			ioPct = ((t.Iowait - c.cpuPrev.iowait) + (t.Steal - c.cpuPrev.steal)) / elapsed * 100
			idlePct = 100 - userPct - sysPct - ioPct
			if idlePct < 0 {
				idlePct = 0
			}
		}
		c.cpuPrev.ok, c.cpuPrev.user, c.cpuPrev.sys = true, t.User+t.Nice, t.System
		c.cpuPrev.iowait, c.cpuPrev.steal, c.cpuPrev.total = t.Iowait, t.Steal, total
	}
	v, _ := mem.VirtualMemory()
	swap, _ := mem.SwapMemory()
	loadAvg, _ := load.Avg()

	now := time.Now()
	prevUpdate := c.lastUpdate
	duration := now.Sub(prevUpdate).Seconds()

	// Network rates + cumulative counters (since boot) + drop/error counters
	io, _ := net.IOCounters(false)
	var downSpeed, upSpeed float64
	var totalDown, totalUp, dropped, errs uint64
	if len(io) > 0 {
		totalDown, totalUp = io[0].BytesRecv, io[0].BytesSent
		dropped, errs = io[0].Dropin+io[0].Dropout, io[0].Errin+io[0].Errout
		if duration > 0 && !prevUpdate.IsZero() {
			downSpeed = float64(io[0].BytesRecv-c.prevNetRecv) / 1024 / duration
			upSpeed = float64(io[0].BytesSent-c.prevNetSent) / 1024 / duration
		}
		c.prevNetRecv = io[0].BytesRecv
		c.prevNetSent = io[0].BytesSent
	}

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

	load1, load5, load15 := 0.0, 0.0, 0.0
	if loadAvg != nil {
		load1, load5, load15 = loadAvg.Load1, loadAvg.Load5, loadAvg.Load15
	}

	c.mu.Lock()
	s := c.current
	s.CPUUsage = usage
	s.CPUUser = userPct
	s.CPUSys = sysPct
	s.CPUIOWait = ioPct
	s.CPUIdle = idlePct
	s.CPUFreq = c.GetCPUFreq()
	s.Cores = perCore
	s.Load1, s.Load5, s.Load15 = load1, load5, load15
	s.MemUsage = v.UsedPercent
	s.MemSummary = fmt.Sprintf("%.2f / %.2f GB", float64(v.Used)/1e9, float64(v.Total)/1e9)
	s.MemAvailable = v.Available
	s.MemCached = v.Cached
	s.SwapUsage = swap.UsedPercent
	s.SwapSummary = fmt.Sprintf("%.2f / %.2f GB", float64(swap.Used)/1e9, float64(swap.Total)/1e9)
	s.NetDown = downSpeed
	s.NetUp = upSpeed
	s.NetTotalDown = totalDown
	s.NetTotalUp = totalUp
	s.NetDropped = dropped
	s.NetErrors = errs
	s.NetOnline = c.probe.online.Load()
	s.NetLatencyMs = float64(c.probe.rttMs.Load())
	s.DiskRead = diskReadSpeed
	s.DiskWrite = diskWriteSpeed
	c.current = s
	snapshot := s
	c.mu.Unlock()

	c.history.maybeAppend(snapshot, now)
}

// collectSlow refreshes the slow-moving metrics (TCP state counts, thermal
// zones, disk usage, uptime) into the shared snapshot. TCP table parsing is
// the single most expensive collection step, which is why it lives here.
func (c *Collector) collectSlow() {
	// TCP connections broken down by state (counts only, never endpoints)
	connections, _ := net.Connections("tcp")
	var connCount, estab, listen, timewait uint64
	for _, cn := range connections {
		switch cn.Status {
		case "ESTABLISHED":
			estab++
		case "LISTEN":
			listen++
		case "TIME_WAIT":
			timewait++
		}
		connCount++
	}

	// Disk I/O quality over the slow interval: busy% from IoTime deltas,
	// average latency from Read/Write time deltas, plus IOPS. Only
	// physical devices count (loop/zram would skew the numbers).
	var busyPct, latencyMs, iops float64
	ioStats, _ := disk.IOCounters()
	var ioTime, rwTime, ops uint64
	for name, d := range ioStats {
		if !physicalDisk(name) {
			continue
		}
		ioTime += d.IoTime
		rwTime += d.ReadTime + d.WriteTime
		ops += d.ReadCount + d.WriteCount
	}
	nowSlow := time.Now()
	if c.diskPrev.ok && ioTime >= c.diskPrev.ioTime {
		seconds := nowSlow.Sub(c.diskPrev.at).Seconds()
		if seconds > 0 {
			busyPct = float64(ioTime-c.diskPrev.ioTime) / 1000 / seconds * 100
			if busyPct > 100 {
				busyPct = 100
			}
			dOps := ops - c.diskPrev.ops
			iops = float64(dOps) / seconds
			if dOps > 0 {
				latencyMs = float64(rwTime-c.diskPrev.rwTime) / float64(dOps)
			}
		}
	}
	c.diskPrev.ok, c.diskPrev.ioTime, c.diskPrev.rwTime, c.diskPrev.ops, c.diskPrev.at =
		true, ioTime, rwTime, ops, nowSlow

	thermals := readThermals()
	cpuTemp := c.GetCPUTemp()
	uptime, _ := host.Uptime()
	wifiLink := readWiFiLink()

	// Physical mount points (/, /var/log, external drives...). Bind mounts
	// like /var/log.hdd share the root device — dedupe by device so only
	// the first mount of each device is shown.
	var mounts []MountUsage
	var rootUsage float64
	var rootSummary string
	seenDevices := make(map[string]bool)
	if parts, err := disk.Partitions(false); err == nil {
		for _, m := range parts {
			if seenDevices[m.Device] {
				continue
			}
			u, err := disk.Usage(m.Mountpoint)
			if err != nil || u == nil || u.Total == 0 {
				continue
			}
			seenDevices[m.Device] = true
			summary := fmtDiskSize(float64(u.Used), float64(u.Total))
			mounts = append(mounts, MountUsage{
				Mount:   m.Mountpoint,
				Usage:   u.UsedPercent,
				Summary: summary,
			})
			if m.Mountpoint == "/" {
				rootUsage = u.UsedPercent
				rootSummary = summary
			}
			if len(mounts) >= 6 {
				break
			}
		}
	}

	c.mu.Lock()
	s := c.current
	s.CPUTemp = cpuTemp
	s.Thermals = thermals
	s.Connections = connCount
	s.ConnEstab = estab
	s.ConnListen = listen
	s.ConnTimeWait = timewait
	s.DiskUsage = rootUsage
	s.DiskSummary = rootSummary
	s.DiskMounts = mounts
	s.DiskBusy = busyPct
	s.DiskLatency = latencyMs
	s.DiskIOPS = iops
	s.WifiLink = wifiLink
	s.TopProcs = c.topProcs()
	s.Uptime = uptime
	c.current = s
	c.mu.Unlock()
}

// topProcs returns the five processes with the highest CPU usage over the
// last slow interval. Process objects are kept across calls so CPUPercent()
// computes a per-interval delta rather than a lifetime average.
func (c *Collector) topProcs() []ProcInfo {
	pids, err := process.Pids()
	if err != nil {
		return nil
	}
	if c.procPrev == nil {
		c.procPrev = make(map[int32]*process.Process)
	}
	alive := make(map[int32]*process.Process, len(pids))
	stats := make([]ProcInfo, 0, len(pids))
	for _, pid := range pids {
		p, ok := c.procPrev[pid]
		if !ok {
			var err error
			p, err = process.NewProcess(pid)
			if err != nil {
				continue
			}
		}
		alive[pid] = p
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		cpu, _ := p.CPUPercent()
		mem, _ := p.MemoryPercent()
		stats = append(stats, ProcInfo{PID: pid, Name: name, CPU: cpu, Mem: float64(mem)})
	}
	c.procPrev = alive
	sort.Slice(stats, func(i, j int) bool { return stats[i].CPU > stats[j].CPU })
	if len(stats) > 5 {
		stats = stats[:5]
	}
	return stats
}
