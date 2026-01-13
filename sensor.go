package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemStats 定义了汇总数据的结构
type SystemStats struct {
	CPUTemp    string  `json:"cpu_temp"`
	CPUUsage   float64 `json:"cpu_usage"`
	MemUsage   float64 `json:"mem_usage"`
	MemSummary string  `json:"mem_summary"` // 例如 "1.2GB / 4.0GB"
}

// Collector 采集器，封装所有采集行为
type Collector struct{}

func (c *Collector) GetCPUTemp() string {
	if runtime.GOOS == "windows" {
		return "45.5°C"
	}
	// Orange Pi 路径
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return "N/A"
	}
	raw := strings.TrimSpace(string(data))
	// 转换逻辑：将 45123 转为 45.1
	if len(raw) >= 3 {
		return fmt.Sprintf("%s.%s°C", raw[:2], raw[2:3])
	}
	return raw + "°C"
}

func (c *Collector) GetCPUUsage() float64 {
	percent, _ := cpu.Percent(0, false)
	if len(percent) > 0 {
		return percent[0]
	}
	return 0.0
}

func (c *Collector) GetMemStats() (float64, string) {
	v, _ := mem.VirtualMemory()
	usage := v.UsedPercent
	summary := fmt.Sprintf("%.1fGB / %.1fGB", float64(v.Used)/1e9, float64(v.Total)/1e9)
	return usage, summary
}

// CollectAll 汇总所有指标
func (c *Collector) CollectAll() SystemStats {
	memUsage, memSum := c.GetMemStats()
	return SystemStats{
		CPUTemp:    c.GetCPUTemp(),
		CPUUsage:   c.GetCPUUsage(),
		MemUsage:   memUsage,
		MemSummary: memSum,
	}
}
