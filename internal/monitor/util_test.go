package monitor

import (
	"testing"
)

func TestFormatTemp(t *testing.T) {
	cases := []struct {
		milli int
		want  string
	}{
		{40600, "40.6°C"},
		{9500, "9.5°C"},   // single-digit: the old string slicing broke here
		{-2500, "-2.5°C"}, // sub-zero environments
		{0, "0.0°C"},
	}
	for _, c := range cases {
		if got := formatTemp(c.milli); got != c.want {
			t.Errorf("formatTemp(%d) = %q, want %q", c.milli, got, c.want)
		}
	}
}

func TestParseTemp(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"45.5°C", 45.5},
		{"9.5°C", 9.5},
		{"N/A", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseTemp(c.in); got != c.want {
			t.Errorf("parseTemp(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFmtDiskSize(t *testing.T) {
	cases := []struct {
		used, total float64
		want        string
	}{
		{4.4e9, 62.2e9, "4.4 / 62.2 GB"},
		{19e6, 49e6, "19 / 49 MB"}, // small partitions must not read "0.0 GB"
	}
	for _, c := range cases {
		if got := fmtDiskSize(c.used, c.total); got != c.want {
			t.Errorf("fmtDiskSize(%v, %v) = %q, want %q", c.used, c.total, got, c.want)
		}
	}
}

func TestUsefulThermal(t *testing.T) {
	t.Setenv("MONITOR_THERMAL_ZONES", "")
	cases := []struct {
		zone string
		want bool
	}{
		{"cpu-thermal", true},
		{"npu-thermal", true},
		{"ve-thermal", false},  // sunxi video engine noise
		{"ddr-thermal", false}, // DDR controller noise
		{"gpu-thermal", false},
	}
	for _, c := range cases {
		if got := usefulThermal(c.zone); got != c.want {
			t.Errorf("usefulThermal(%q) = %v, want %v", c.zone, got, c.want)
		}
	}

	// Override: rockchip kernels label the SoC sensor "soc-thermal"
	t.Setenv("MONITOR_THERMAL_ZONES", "cpu,npu,soc")
	if !usefulThermal("soc-thermal") {
		t.Error("env override did not admit soc-thermal")
	}
}

func TestPhysicalDisk(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"mmcblk0", true},
		{"sda", true},
		{"nvme0n1", true},
		{"vda", true},
		{"loop0", false},
		{"zram0", false},
		{"ram0", false},
	}
	for _, c := range cases {
		if got := physicalDisk(c.name); got != c.want {
			t.Errorf("physicalDisk(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRound1(t *testing.T) {
	if got := round1(45.46); got != 45.5 {
		t.Errorf("round1(45.46) = %v, want 45.5", got)
	}
}
