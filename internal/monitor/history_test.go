package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// appendN feeds n synthetic points into the buffer at 10s intervals.
func appendN(t *testing.T, h *history, n int, cpuBase float64) {
	t.Helper()
	base := time.Now().Add(-time.Duration(n) * historyInterval)
	for i := 0; i < n; i++ {
		now := base.Add(time.Duration(i) * historyInterval)
		h.maybeAppend(SystemStats{CPUUsage: cpuBase + float64(i)}, now)
	}
}

func TestHistoryRingOverwrite(t *testing.T) {
	h := newHistory()
	// Feed more points than the capacity: the oldest must be overwritten
	appendN(t, h, historyCapacity+50, 0)
	snap := h.snapshot()
	if len(snap.T) != historyCapacity {
		t.Fatalf("buffer holds %d points, want %d", len(snap.T), historyCapacity)
	}
	// Chronological order: the first kept point is the 51st fed
	if snap.T[0] != snap.T[len(snap.T)-1]-int64((historyCapacity-1)*10) {
		t.Fatalf("points are not evenly spaced: first=%d last=%d", snap.T[0], snap.T[len(snap.T)-1])
	}
	if snap.CPU[0] >= snap.CPU[len(snap.CPU)-1] {
		t.Fatalf("oldest point survived the overwrite: first CPU=%v", snap.CPU[0])
	}
}

func TestHistoryIntervalThrottle(t *testing.T) {
	h := newHistory()
	// Two writes within one interval: only the first lands
	h.maybeAppend(SystemStats{CPUUsage: 1}, time.Now())
	h.maybeAppend(SystemStats{CPUUsage: 2}, time.Now().Add(time.Second))
	if got := h.snapshot(); len(got.T) != 1 {
		t.Fatalf("interval throttle failed: %d points after 1s gap", len(got.T))
	}
}

func TestHistorySaveLoadRoundTrip(t *testing.T) {
	h := newHistory()
	appendN(t, h, 100, 10)
	path := filepath.Join(t.TempDir(), "history.json")
	if err := h.save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	fresh := newHistory()
	fresh.load(path)
	got := fresh.snapshot()
	if len(got.T) != 100 {
		t.Fatalf("reloaded %d points, want 100", len(got.T))
	}
	for i := range got.CPU {
		if got.CPU[i] != float32(10)+float32(i) {
			t.Fatalf("point %d drifted: got %v want %v", i, got.CPU[i], float32(10)+float32(i))
		}
	}
	if fresh.last.IsZero() {
		t.Fatal("reload did not restore the write cadence anchor")
	}
}

func TestHistoryLoadSkipsStaleAndCorrupt(t *testing.T) {
	dir := t.TempDir()

	// Corrupt file: load must silently ignore it
	corrupt := filepath.Join(dir, "corrupt.json")
	if err := writeFile(corrupt, "{not json"); err != nil {
		t.Fatal(err)
	}
	h := newHistory()
	h.load(corrupt)
	if got := h.snapshot(); len(got.T) != 0 {
		t.Fatalf("corrupt snapshot loaded %d points", len(got.T))
	}

	// Inconsistent column lengths: ignored
	bad := filepath.Join(dir, "bad.json")
	if err := writeFile(bad, `{"interval_s":10,"t":[1,2,3],"cpu":[1]}`); err != nil {
		t.Fatal(err)
	}
	h2 := newHistory()
	h2.load(bad)
	if got := h2.snapshot(); len(got.T) != 0 {
		t.Fatalf("inconsistent snapshot loaded %d points", len(got.T))
	}

	// Stale points (older than retention) are dropped
	old := time.Now().Add(-48 * time.Hour).Unix()
	stale := filepath.Join(dir, "stale.json")
	if err := writeFile(stale, fmt.Sprintf(
		`{"interval_s":10,"t":[%d,%d],"cpu":[1,2],"temp":[0,0],"mem":[0,0],"net_down":[0,0],"net_up":[0,0]}`,
		old, time.Now().Unix()-60)); err != nil {
		t.Fatal(err)
	}
	h3 := newHistory()
	h3.load(stale)
	if got := h3.snapshot(); len(got.T) != 1 {
		t.Fatalf("stale filter kept %d of 2 points", len(got.T))
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
