package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Server struct {
	collector *Collector
}

func NewServer() *Server {
	return &Server{
		collector: &Collector{},
	}
}

// StatsHandler 处理 API 请求
func (s *Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	stats := s.collector.CollectAll()
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) Start(port string) {
	// 静态文件服务
	fs := http.FileServer(http.Dir("./"))
	http.Handle("/", fs)

	// API 路由
	http.HandleFunc("/api/stats", s.StatsHandler)

	fmt.Printf("[%s] 🚀 监控服务启动在端口 %s\n", time.Now().Format("15:04:05"), port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Printf("❌ 启动失败: %v\n", err)
	}
}
