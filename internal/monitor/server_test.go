package monitor

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHistoryCacheReusesEncoding(t *testing.T) {
	s := &Server{
		collector: &Collector{history: newHistory()},
	}
	s.collector.history.maybeAppend(SystemStats{CPUUsage: 42}, time.Now())

	rec1 := httptest.NewRecorder()
	s.HistoryHandler(rec1, httptest.NewRequest("GET", "/api/history", nil))
	if rec1.Code != 200 {
		t.Fatalf("first request: %d", rec1.Code)
	}

	// Mutate the buffer WITHOUT advancing past the cache TTL: a re-encode
	// would change the payload, a cache hit must return identical bytes
	s.collector.history.maybeAppend(SystemStats{CPUUsage: 99}, time.Now().Add(2*time.Second))
	rec2 := httptest.NewRecorder()
	s.HistoryHandler(rec2, httptest.NewRequest("GET", "/api/history", nil))
	if string(rec2.Body.Bytes()) != string(rec1.Body.Bytes()) {
		t.Fatal("cache miss inside the TTL window: payload changed without new data")
	}
}

func TestHistoryCacheServesBothEncodings(t *testing.T) {
	s := &Server{collector: &Collector{history: newHistory()}}
	// Enough points that gzip has something to chew on (a single ~100B
	// payload grows slightly under gzip, which is normal)
	base := time.Now().Add(-500 * historyInterval)
	for i := 0; i < 500; i++ {
		s.collector.history.maybeAppend(SystemStats{CPUUsage: float64(i%40) + 10}, base.Add(time.Duration(i)*historyInterval))
	}

	plain := httptest.NewRecorder()
	s.HistoryHandler(plain, httptest.NewRequest("GET", "/api/history", nil))
	if plain.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("plain request must not be gzip-encoded")
	}

	gz := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/history", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	s.HistoryHandler(gz, req)
	if gz.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("gzip-advertising client did not get compressed bytes")
	}
	if gz.Body.Len() >= plain.Body.Len() {
		t.Fatal("gzip form is not smaller than plain JSON")
	}
}

func TestLimitConcurrencyShedsLoad(t *testing.T) {
	release := make(chan struct{})
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(limitConcurrency(slow))
	defer srv.Close()

	var wg sync.WaitGroup
	codes := make(chan int, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(srv.URL)
			if err != nil {
				codes <- -1
				return
			}
			codes <- resp.StatusCode
			resp.Body.Close()
		}()
	}
	// Give in-flight requests a moment to fill the semaphore, then let
	// them all finish and collect the results
	time.Sleep(200 * time.Millisecond)
	close(release)
	wg.Wait()
	close(codes)

	ok, busy := 0, 0
	for c := range codes {
		switch c {
		case 200:
			ok++
		case 503:
			busy++
		}
	}
	if ok != maxConcurrent {
		t.Fatalf("%d requests passed, want exactly %d (the semaphore size)", ok, maxConcurrent)
	}
	if busy != 40-maxConcurrent {
		t.Fatalf("%d requests shed with 503, want %d", busy, 40-maxConcurrent)
	}
}
