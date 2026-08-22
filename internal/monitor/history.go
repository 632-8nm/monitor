package monitor

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// One history point every 10s, keeping 24h in memory: 8640 points.
	historyInterval = 10 * time.Second
	historyCapacity = 24 * 60 * 60 / 10
)

// historyPoint is one sampled trend value, rounded to one decimal to keep
// the /api/history payload compact at 24h of data.
type historyPoint struct {
	T       int64
	CPU     float32
	Temp    float32
	Mem     float32
	NetDown float32
	NetUp   float32
}

// history is a fixed-size ring buffer. Data lives purely in memory and is
// lost on restart — acceptable for "what happened last night" style
// inspection without any database dependency.
type history struct {
	mu     sync.Mutex
	points []historyPoint
	head   int
	count  int
	last   time.Time
}

func newHistory() *history {
	return &history{points: make([]historyPoint, historyCapacity)}
}

// maybeAppend records a point if at least historyInterval has elapsed since
// the previous one; called from the collector's fixed-period loop.
func (h *history) maybeAppend(stats SystemStats, now time.Time) {
	if !h.last.IsZero() && now.Sub(h.last) < historyInterval {
		return
	}
	h.mu.Lock()
	h.points[h.head] = historyPoint{
		T:       now.Unix(),
		CPU:     round1(stats.CPUUsage),
		Temp:    round1(parseTemp(stats.CPUTemp)),
		Mem:     round1(stats.MemUsage),
		NetDown: round1(stats.NetDown),
		NetUp:   round1(stats.NetUp),
	}
	h.head = (h.head + 1) % historyCapacity
	if h.count < historyCapacity {
		h.count++
	}
	h.last = now
	h.mu.Unlock()
}

// historySnapshot is the wire format for /api/history: columnar arrays are
// roughly half the size of an array of objects at 8640 points.
type historySnapshot struct {
	IntervalSeconds int       `json:"interval_s"`
	T               []int64   `json:"t"`
	CPU             []float32 `json:"cpu"`
	Temp            []float32 `json:"temp"`
	Mem             []float32 `json:"mem"`
	NetDown         []float32 `json:"net_down"`
	NetUp           []float32 `json:"net_up"`
}

// snapshot returns all recorded points as a columnar, JSON-ready struct.
func (h *history) snapshot() historySnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := h.count
	out := historySnapshot{
		IntervalSeconds: int(historyInterval / time.Second),
		T:               make([]int64, 0, n),
		CPU:             make([]float32, 0, n),
		Temp:            make([]float32, 0, n),
		Mem:             make([]float32, 0, n),
		NetDown:         make([]float32, 0, n),
		NetUp:           make([]float32, 0, n),
	}
	start := (h.head - h.count + historyCapacity) % historyCapacity
	for i := 0; i < h.count; i++ {
		p := h.points[(start+i)%historyCapacity]
		out.T = append(out.T, p.T)
		out.CPU = append(out.CPU, p.CPU)
		out.Temp = append(out.Temp, p.Temp)
		out.Mem = append(out.Mem, p.Mem)
		out.NetDown = append(out.NetDown, p.NetDown)
		out.NetUp = append(out.NetUp, p.NetUp)
	}
	return out
}

func round1(v float64) float32 {
	return float32(math.Round(v*10) / 10)
}

// parseTemp converts the collector's display string ("45.5°C") into a
// number for charting; unparseable values (e.g. "N/A") become 0.
func parseTemp(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSuffix(s, "°C"), 32)
	if err != nil {
		return 0
	}
	return f
}
