package monitor

import (
	"encoding/json"
	"os"
	"time"
)

const (
	// persistFile lives in the working directory (/opt/monitor on the
	// board): one atomic JSON snapshot per minute, reloaded on startup so
	// the 24h trend survives reboots and deployments.
	persistFile     = "history.json"
	persistInterval = time.Minute
	// Points older than the retention window are dropped during reload —
	// a file written before a long outage must not resurrect stale data.
	persistMaxAge = 24 * time.Hour
)

// save writes the current buffer to disk atomically: data goes to a
// temp file first, then renames over the target, so a crash or power
// loss mid-write can never truncate the previous snapshot.
func (h *history) save(path string) error {
	snap := h.snapshot()
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// load restores a previously saved snapshot into the ring buffer.
// Malformed or structurally inconsistent files are ignored silently —
// a corrupt snapshot must never keep the service from starting.
func (h *history) load(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // first run: no file yet
	}
	var snap historySnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return
	}
	n := len(snap.T)
	if n == 0 || len(snap.CPU) != n || len(snap.Temp) != n || len(snap.Mem) != n || len(snap.NetDown) != n || len(snap.NetUp) != n {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	var lastT int64
	for i, t := range snap.T {
		if time.Since(time.Unix(t, 0)) > persistMaxAge {
			continue
		}
		h.points[h.head] = historyPoint{
			T:       t,
			CPU:     snap.CPU[i],
			Temp:    snap.Temp[i],
			Mem:     snap.Mem[i],
			NetDown: snap.NetDown[i],
			NetUp:   snap.NetUp[i],
		}
		h.head = (h.head + 1) % historyCapacity
		if h.count < historyCapacity {
			h.count++
		}
		lastT = t
	}
	if lastT != 0 {
		// Keep the 10s write cadence aligned with the reloaded tail
		h.last = time.Unix(lastT, 0)
	}
}
