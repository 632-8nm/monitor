package monitor

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// alertCheckInterval is how often thresholds are evaluated; notifications
	// themselves are sent asynchronously so a slow API never blocks collection.
	alertCheckInterval = 10 * time.Second
	defaultTempAlert   = 70.0
	defaultMemAlert    = 90.0
	defaultDiskAlert   = 90.0
	defaultCooldown    = 30 * time.Minute
	// Hysteresis: a rule only leaves the breach state once the value drops
	// this far below the threshold, which prevents flapping notifications.
	tempHysteresis = 5.0
	pctHysteresis  = 5.0
)

// alertRule watches one metric against a threshold. All fields are only
// accessed from the single alert-check goroutine.
type alertRule struct {
	name       string
	env        string
	unit       string
	threshold  float64
	hysteresis float64
	value      func(SystemStats) float64

	breachSince time.Time // nonzero while in breach
	notified    bool      // a breach notification was sent for this episode
	lastSent    time.Time
}

// maxThermal returns the hottest thermal zone (falling back to the legacy
// cpu_temp string), so NPU/GPU zones are covered whenever present.
func maxThermal(stats SystemStats) float64 {
	if len(stats.Thermals) > 0 {
		max := stats.Thermals[0].Temp
		for _, z := range stats.Thermals {
			if z.Temp > max {
				max = z.Temp
			}
		}
		return max
	}
	return parseTemp(stats.CPUTemp)
}

// Alerter pushes threshold alerts to WeChat via ServerChan (sct.ftqq.com).
// Alerting is disabled unless MONITOR_SERVERCHAN_KEY is set.
type Alerter struct {
	enabled  bool
	key      string
	cooldown time.Duration
	rules    []*alertRule
	client   *http.Client
}

// NewAlerterFromEnv reads MONITOR_SERVERCHAN_KEY plus threshold overrides
// (value 0 disables a rule) and the cooldown in minutes.
func NewAlerterFromEnv() *Alerter {
	a := &Alerter{
		key:      strings.TrimSpace(os.Getenv("MONITOR_SERVERCHAN_KEY")),
		cooldown: envMinutes("MONITOR_ALERT_COOLDOWN", defaultCooldown),
		client:   &http.Client{Timeout: 10 * time.Second},
	}
	a.enabled = a.key != ""
	a.rules = []*alertRule{
		{name: "温度", env: "MONITOR_ALERT_TEMP", unit: "°C", threshold: envFloat("MONITOR_ALERT_TEMP", defaultTempAlert), hysteresis: tempHysteresis, value: maxThermal},
		{name: "内存", env: "MONITOR_ALERT_MEM", unit: "%", threshold: envFloat("MONITOR_ALERT_MEM", defaultMemAlert), hysteresis: pctHysteresis, value: func(s SystemStats) float64 { return s.MemUsage }},
		{name: "磁盘", env: "MONITOR_ALERT_DISK", unit: "%", threshold: envFloat("MONITOR_ALERT_DISK", defaultDiskAlert), hysteresis: pctHysteresis, value: func(s SystemStats) float64 { return s.DiskUsage }},
	}
	return a
}

// Check evaluates every rule against the snapshot; safe to call from a
// fixed-period loop. Delivery happens on its own goroutine.
func (a *Alerter) Check(stats SystemStats) {
	if !a.enabled {
		return
	}
	now := time.Now()
	for _, r := range a.rules {
		if r.threshold <= 0 { // rule disabled via env
			continue
		}
		v := r.value(stats)
		switch {
		case v >= r.threshold:
			if r.breachSince.IsZero() {
				r.breachSince = now
			}
			if now.Sub(r.lastSent) >= a.cooldown {
				r.notified = true
				r.lastSent = now
				go a.notify(
					fmt.Sprintf("⚠️ Orange Pi %s告警", r.name),
					fmt.Sprintf("**%s**: %.1f%s，超过阈值 %.0f%s\n\n> %s",
						r.name, v, r.unit, r.threshold, r.unit, now.Format("2006-01-02 15:04:05")))
			}
		case !r.breachSince.IsZero() && v <= r.threshold-r.hysteresis:
			episode := now.Sub(r.breachSince).Round(time.Minute)
			r.breachSince = time.Time{}
			if r.notified {
				r.notified = false
				go a.notify(
					fmt.Sprintf("✅ Orange Pi %s已恢复", r.name),
					fmt.Sprintf("**%s**: %.1f%s 已回落到阈值以下（本次持续约 %s）", r.name, v, r.unit, episode))
			}
		}
	}
}

// notify delivers one ServerChan message; failures are logged, never fatal.
func (a *Alerter) notify(title, desp string) {
	api := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", a.key)
	resp, err := a.client.PostForm(api, url.Values{"title": {title}, "desp": {desp}})
	if err != nil {
		fmt.Printf("❌ Alert delivery failed: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Alert delivery returned %s\n", resp.Status)
	}
}

func envFloat(name string, def float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		fmt.Printf("⚠️ Ignoring invalid %s=%q\n", name, raw)
		return def
	}
	return f
}

func envMinutes(name string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	m, err := strconv.Atoi(raw)
	if err != nil || m < 0 {
		return def
	}
	return time.Duration(m) * time.Minute
}
