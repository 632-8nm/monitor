package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Server struct {
	collector       *Collector
	basicAuthUser   string
	basicAuthPass   string
	allowedOrigins  map[string]struct{}
	corsAllowAll    bool
}

func NewServer() *Server {
	origins := parseAllowedOrigins(os.Getenv("MONITOR_ALLOWED_ORIGINS"))
	return &Server{
		collector:      &Collector{},
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

// StatsHandler 处理 API 请求
func (s *Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	s.applyCORS(w, r)
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.isAuthorized(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="monitor"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	stats := s.collector.CollectAll()
	json.NewEncoder(w).Encode(stats)
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

func (s *Server) Start(addr string) {
	mux := http.NewServeMux()

	// 这行代码的意思是：当收到以 /static/ 开头的请求时，去本机的 static 文件夹找文件
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", s.authMiddleware(http.StripPrefix("/static/", fs)))

	// 首页路由：当直接访问根目录时，返回 index.html
	mux.Handle("/", s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})))

	// API 路由
	mux.HandleFunc("/api/stats", s.StatsHandler)

	fmt.Printf("[%s] 🚀 监控服务启动在地址 %s\n", time.Now().Format("15:04:05"), addr)
	if s.basicAuthUser == "" || s.basicAuthPass == "" {
		fmt.Println("⚠️ 未配置 MONITOR_BASIC_AUTH_USER/PASS，当前为无鉴权模式")
	}
	if s.corsAllowAll {
		fmt.Println("⚠️ 未配置 MONITOR_ALLOWED_ORIGINS，当前为宽松 CORS 模式")
	}
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("❌ 启动失败: %v\n", err)
	}
}
