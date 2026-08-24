package monitor

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"
)

type Server struct {
	collector      *Collector
	alerter        *Alerter
	system         SystemInfo
	terminal       bool
	admin          *adminAuth
	basicAuthUser  string
	basicAuthPass  string
	allowedOrigins map[string]struct{}
	corsAllowAll   bool
}

func NewServer() *Server {
	origins := parseAllowedOrigins(os.Getenv("MONITOR_ALLOWED_ORIGINS"))
	system := readSystemInfo()
	alerter := NewAlerterFromEnv()
	alerter.board = system.BoardModel // notification titles carry the board model
	admin := newAdminAuth()
	// The web terminal needs both the feature switch and admin credentials
	system.Terminal = terminalEnabled() && admin.configured()
	return &Server{
		collector:      &Collector{history: newHistory(), probe: &netProbe{}},
		alerter:        alerter,
		system:         system,
		terminal:       terminalEnabled(),
		admin:          admin,
		basicAuthUser:  os.Getenv("MONITOR_BASIC_AUTH_USER"),
		basicAuthPass:  os.Getenv("MONITOR_BASIC_AUTH_PASS"),
		allowedOrigins: origins,
		corsAllowAll:   len(origins) == 0,
	}
}

func parseAllowedOrigins(raw string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(item)
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return origins
}

func (s *Server) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if s.corsAllowAll {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		return
	}
	if _, ok := s.allowedOrigins[origin]; ok {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
}

func (s *Server) isAuthorized(r *http.Request) bool {
	// If auth is not fully configured, keep compatibility mode.
	if s.basicAuthUser == "" || s.basicAuthPass == "" {
		return true
	}
	user, pass, ok := r.BasicAuth()
	return ok && user == s.basicAuthUser && pass == s.basicAuthPass
}

// preflight handles CORS and authentication for API endpoints. It returns
// false when the request has already been fully answered (OPTIONS preflight
// or an auth failure), so handlers can bail out early.
func (s *Server) preflight(w http.ResponseWriter, r *http.Request) bool {
	s.applyCORS(w, r)
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	if !s.isAuthorized(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="monitor"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// StatsHandler serves API requests
func (s *Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	if !s.preflight(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	stats := s.collector.Snapshot()
	json.NewEncoder(w).Encode(stats)
}

// HistoryHandler serves the in-memory trend points (24h, one point per 10s)
func (s *Server) HistoryHandler(w http.ResponseWriter, r *http.Request) {
	if !s.preflight(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(s.collector.HistorySnapshot())
}

// SystemHandler serves the static machine identity (OS, kernel, board, CPU)
func (s *Server) SystemHandler(w http.ResponseWriter, r *http.Request) {
	if !s.preflight(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(s.system)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthorized(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="monitor"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type nocacheFS struct{ h http.Handler }

func (n *nocacheFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	n.h.ServeHTTP(w, r)
}

func (s *Server) Start(addr string) {
	mux := http.NewServeMux()

	// Serve the embedded frontend (index.html + static/) from the binary.
	// web/ mirrors the URL layout, so /static/... maps directly to
	// web/static/... — no prefix stripping needed.
	webRoot, err := fs.Sub(embeddedFS, "web")
	if err != nil {
		fmt.Printf("❌ Failed to open embedded frontend: %v\n", err)
		return
	}
	fileServer := &nocacheFS{http.FileServer(http.FS(webRoot))}
	mux.Handle("/static/", s.authMiddleware(fileServer))

	// Root route: serve the embedded index.html for direct visits to /
	mux.Handle("/", s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index, err := embeddedFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})))

	// API routes
	mux.HandleFunc("/api/stats", s.StatsHandler)
	mux.HandleFunc("/api/history", s.HistoryHandler)
	mux.HandleFunc("/api/system", s.SystemHandler)
	mux.HandleFunc("/ws/terminal", s.TerminalHandler) // self-guards when disabled

	// Admin session endpoints (guard the terminal and future admin areas)
	mux.HandleFunc("/api/admin/session", s.AdminSessionHandler)
	mux.HandleFunc("/api/admin/login", s.AdminLoginHandler)
	mux.HandleFunc("/api/admin/logout", s.AdminLogoutHandler)

	// Start fixed-period background collection; the API only reads snapshots
	s.collector.Start()

	// Threshold alerts: evaluated on a fixed period, delivered asynchronously
	if s.alerter.enabled {
		go func() {
			ticker := time.NewTicker(alertCheckInterval)
			defer ticker.Stop()
			for range ticker.C {
				s.alerter.Check(s.collector.Snapshot())
			}
		}()
	}

	fmt.Printf("[%s] 🚀 Monitor server listening on %s\n", time.Now().Format("15:04:05"), addr)
	if s.basicAuthUser == "" || s.basicAuthPass == "" {
		fmt.Println("⚠️ MONITOR_BASIC_AUTH_USER/PASS not set — running without authentication")
	}
	if s.corsAllowAll {
		fmt.Println("⚠️ MONITOR_ALLOWED_ORIGINS not set — running with permissive CORS")
	}
	if s.alerter.enabled {
		fmt.Printf("🔔 Alerting enabled via ServerChan (cooldown %s)\n", s.alerter.cooldown)
	}
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("❌ Failed to start: %v\n", err)
	}
}
