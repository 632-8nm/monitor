package monitor

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// stubTransport records notify() calls instead of hitting the network.
type stubTransport struct {
	hits   int
	titles []string
	on回应OK bool // always succeeds
}

func (t *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.hits++
	body, _ := io.ReadAll(req.Body)
	for _, line := range strings.Split(string(body), "&") {
		if strings.HasPrefix(line, "title=") {
			t.titles = append(t.titles, line)
		}
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
}

func newTestAlerter(threshold float64, transport *stubTransport) *Alerter {
	a := &Alerter{
		enabled:  true,
		key:      "testkey",
		cooldown: time.Hour, // default: one notification per test
		client:   &http.Client{Transport: transport},
		rules: []*alertRule{
			{name: "温度", unit: "°C", threshold: threshold, hysteresis: tempHysteresis, value: maxThermal},
		},
	}
	return a
}

func waitAsync() { time.Sleep(80 * time.Millisecond) }

func TestAlertBreachCooldownRecovery(t *testing.T) {
	stub := &stubTransport{}
	a := newTestAlerter(70, stub)

	// Breach: 75 C over the 70 C threshold
	a.Check(SystemStats{CPUTemp: "75.0°C"})
	waitAsync()
	if stub.hits != 1 {
		t.Fatalf("breach produced %d notifications, want 1", stub.hits)
	}

	// Still breaching within the cooldown window: must NOT re-notify
	a.Check(SystemStats{CPUTemp: "76.0°C"})
	waitAsync()
	if stub.hits != 1 {
		t.Fatalf("cooldown ignored: %d notifications after repeat check", stub.hits)
	}

	// Between threshold and threshold-hysteresis: still in breach, no recovery
	a.Check(SystemStats{CPUTemp: "67.0°C"})
	waitAsync()
	if stub.hits != 1 {
		t.Fatalf("hysteresis ignored: recovery fired inside the dead zone")
	}

	// Clear recovery: back under 65 C
	a.Check(SystemStats{CPUTemp: "50.0°C"})
	waitAsync()
	if stub.hits != 2 {
		t.Fatalf("recovery produced %d new notifications, want 1 (total 2)", stub.hits-1)
	}
	if len(stub.titles) == 0 || !strings.Contains(stub.titles[len(stub.titles)-1], "%E5%B7%B2%E6%81%A2%E5%A4%8D") && !strings.Contains(stub.titles[len(stub.titles)-1], "已恢复") {
		t.Logf("last title: %v", stub.titles[len(stub.titles)-1])
	}
}

func TestAlertDisabledRule(t *testing.T) {
	stub := &stubTransport{}
	a := newTestAlerter(0, stub) // threshold 0 = rule disabled
	a.Check(SystemStats{CPUTemp: "99.0°C"})
	waitAsync()
	if stub.hits != 0 {
		t.Fatalf("disabled rule fired %d notifications", stub.hits)
	}
}

func TestAlertLessRule(t *testing.T) {
	// WiFi-style rule: weak signal (low value) breaches
	stub := &stubTransport{}
	a := &Alerter{
		enabled:  true,
		key:      "testkey",
		cooldown: time.Hour,
		client:   &http.Client{Transport: stub},
		rules: []*alertRule{
			{name: "WiFi 信号", threshold: 35, hysteresis: 5, less: true, value: func(s SystemStats) float64 {
				if s.WifiLink == 0 {
					return 100 // no wireless = never trip
				}
				return s.WifiLink
			}},
		},
	}

	// No wireless interface: must never fire
	a.Check(SystemStats{WifiLink: 0})
	waitAsync()
	if stub.hits != 0 {
		t.Fatalf("wired board tripped the WiFi rule")
	}

	// Weak signal: breach
	a.Check(SystemStats{WifiLink: 20})
	waitAsync()
	if stub.hits != 1 {
		t.Fatalf("weak WiFi produced %d notifications, want 1", stub.hits)
	}

	// Inside the hysteresis dead zone (35 < v < 40): still breaching
	a.Check(SystemStats{WifiLink: 37})
	waitAsync()
	if stub.hits != 1 {
		t.Fatalf("hysteresis dead zone fired recovery")
	}

	// Recovered above 40
	a.Check(SystemStats{WifiLink: 50})
	waitAsync()
	if stub.hits != 2 {
		t.Fatalf("recovery produced %d new notifications, want 1", stub.hits-1)
	}
}

func TestAlertNetOfflineDirection(t *testing.T) {
	// Regression: the offline rule must be a plain high-threshold rule.
	// An earlier version combined online=0/offline=1 with a less
	// comparison, which made 0 <= 1 always true — the dashboard spammed
	// offline alerts while the network was perfectly fine.
	stub := &stubTransport{}
	a := &Alerter{
		enabled:  true,
		key:      "testkey",
		cooldown: time.Hour,
		client:   &http.Client{Transport: stub},
		rules: []*alertRule{
			{name: "外网", threshold: 1, hysteresis: 1, value: func(s SystemStats) float64 {
				if s.NetOnline {
					return 0
				}
				return 1
			}},
		},
	}
	a.Check(SystemStats{NetOnline: true})
	waitAsync()
	if stub.hits != 0 {
		t.Fatalf("online state fired the offline rule (%d notifications)", stub.hits)
	}
	a.Check(SystemStats{NetOnline: false})
	waitAsync()
	if stub.hits != 1 {
		t.Fatalf("offline produced %d notifications, want 1", stub.hits)
	}
	a.Check(SystemStats{NetOnline: true})
	waitAsync()
	if stub.hits != 2 {
		t.Fatalf("recovery produced %d new notifications, want 1", stub.hits-1)
	}
}

func TestMaxThermalUsesHottestZone(t *testing.T) {
	stats := SystemStats{
		CPUTemp:  "60.0°C",
		Thermals: []ThermalZone{{Type: "cpu-thermal", Temp: 55}, {Type: "npu-thermal", Temp: 72}},
	}
	if got := maxThermal(stats); got != 72 {
		t.Fatalf("maxThermal = %v, want 72 (hottest zone wins)", got)
	}
	stats.Thermals = nil
	if got := maxThermal(stats); got != 60 {
		t.Fatalf("fallback to cpu_temp string failed: %v", got)
	}
}
