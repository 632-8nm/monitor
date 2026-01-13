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
	// 这行代码的意思是：当收到以 /static/ 开头的请求时，去本机的 static 文件夹找文件
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 首页路由：当直接访问根目录时，返回 index.html
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// API 路由
	http.HandleFunc("/api/stats", s.StatsHandler)

	fmt.Printf("[%s] 🚀 监控服务启动在端口 %s\n", time.Now().Format("15:04:05"), port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Printf("❌ 启动失败: %v\n", err)
	}
}
