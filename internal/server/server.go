/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/engine, internal/worker, internal/llm
 * [OUTPUT]: 对外提供 Server struct，HTTP API 入口 + 工人生命周期管理 + 事件推送端点
 * [POS]: internal/server 的唯一成员，纯 API + 协程管理 + 城市事件接收，Web UI 已分离至 cmd/dashboard
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"worker-agent/internal/city"
	"worker-agent/internal/db"
	"worker-agent/internal/engine"
	"worker-agent/internal/llm"
	"worker-agent/internal/worker"
)

// ================================================================
//  核心结构体
// ================================================================

type Server struct {
	dataDir   string
	cityAPI   *city.CityAPI
	llmClient llm.Client
	workers   map[string]*runningWorker
	mu        sync.RWMutex
}

type runningWorker struct {
	Name     string             `json:"name"`
	Status   string             `json:"status"`
	DBPath   string             `json:"db_path"`
	database *db.Database
	wakeupCh chan<- worker.WakeupSignal
	cancel   context.CancelFunc
}

// ================================================================
//  API 请求/响应
// ================================================================

type createRequest struct {
	Name        string `json:"name"`
	Occupation  string `json:"occupation"`
	Background  string `json:"background"`
	Personality string `json:"personality"`
	SpeechStyle string `json:"speech_style"`
	ValuesDesc  string `json:"values_desc"`
	Family      string `json:"family"`
	Avatar      string `json:"avatar"`
}

type workerInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ================================================================
//  构造
// ================================================================

func New(dataDir string, cityAPI *city.CityAPI, llmClient llm.Client) *Server {
	os.MkdirAll(dataDir, 0755)
	return &Server{
		dataDir:   dataDir,
		cityAPI:   cityAPI,
		llmClient: llmClient,
		workers:   make(map[string]*runningWorker),
	}
}

// ================================================================
//  自动恢复：扫描已有 DB，重启工人
// ================================================================

func (s *Server) Resume() {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		log.Printf("[server] 扫描数据目录失败: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".db")
		dbPath := filepath.Join(s.dataDir, entry.Name())

		database, err := db.New(dbPath)
		if err != nil {
			log.Printf("[server] 恢复 %s 失败（打开 DB）: %v", name, err)
			continue
		}

		if _, err := database.GetSoul(); err != nil {
			log.Printf("[server] 跳过 %s（无 soul 数据）", name)
			database.Close()
			continue
		}

		wakeupTime := time.Now().Add(5 * time.Second).Format(time.RFC3339)
		database.InsertWakeup(wakeupTime, "刚才愣了一下，重新审视所有唤醒计划，确保未来有合理的唤醒安排")
		log.Printf("[server] 为 %s 插入恢复审视唤醒", name)

		s.startWorker(name, dbPath, database)
		log.Printf("[server] 恢复工人: %s", name)
	}
}

// ================================================================
//  HTTP 路由
// ================================================================

func (s *Server) ListenAndServe(port int) error {
	mux := http.NewServeMux()

	// ── API ──
	mux.HandleFunc("POST /api/workers", s.handleCreate)
	mux.HandleFunc("GET /api/workers", s.handleList)
	mux.HandleFunc("GET /api/workers/{name}", s.handleGet)
	mux.HandleFunc("PUT /api/workers/{name}", s.handleUpdate)
	mux.HandleFunc("POST /api/workers/{name}/wakeup", s.handleManualWakeup)
	mux.HandleFunc("POST /api/workers/{name}/event", s.handlePushEvent)
	mux.HandleFunc("DELETE /api/workers/{name}", s.handleDelete)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[server] 启动 HTTP 服务: %s", addr)
	return http.ListenAndServe(addr, corsMiddleware(mux))
}

// POST /api/workers — 创建工人
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效 JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Occupation == "" {
		http.Error(w, "name 和 occupation 为必填项", http.StatusBadRequest)
		return
	}

	slug := sanitizeName(req.Name)

	s.mu.RLock()
	_, exists := s.workers[slug]
	s.mu.RUnlock()
	if exists {
		http.Error(w, "工人已存在: "+req.Name, http.StatusConflict)
		return
	}

	dbPath := filepath.Join(s.dataDir, slug+".db")
	database, err := db.New(dbPath)
	if err != nil {
		http.Error(w, "创建数据库失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	soul := db.Soul{
		Name:        req.Name,
		Occupation:  req.Occupation,
		Background:  req.Background,
		Personality: req.Personality,
		SpeechStyle: req.SpeechStyle,
		ValuesDesc:  req.ValuesDesc,
		Family:      req.Family,
		Avatar:      req.Avatar,
		Mood:        50,
		Hope:        50,
		Grievance:   0,
	}
	if err := database.InitSoul(soul); err != nil {
		database.Close()
		http.Error(w, "写入 soul 失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 插入首次唤醒：5 秒后立即开始思考
	firstWakeup := time.Now().Add(5 * time.Second).Format(time.RFC3339)
	database.InsertWakeup(firstWakeup, "首次起床")

	s.startWorker(slug, dbPath, database)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(workerInfo{Name: req.Name, Status: "running"})

	log.Printf("[server] 创建工人: %s (%s)", req.Name, req.Occupation)
}

// GET /api/workers — 列出所有工人
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]workerInfo, 0, len(s.workers))
	for _, rw := range s.workers {
		list = append(list, workerInfo{Name: rw.Name, Status: rw.Status})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// GET /api/workers/{name} — 查询单个工人
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	slug := sanitizeName(name)

	s.mu.RLock()
	rw, exists := s.workers[slug]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "工人不存在: "+name, http.StatusNotFound)
		return
	}

	soul, _ := rw.database.GetSoul()

	resp := map[string]any{
		"name":      rw.Name,
		"status":    rw.Status,
		"soul":      soul,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// PUT /api/workers/{name} — 修改人设
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	slug := sanitizeName(name)

	s.mu.RLock()
	rw, exists := s.workers[slug]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "工人不存在: "+name, http.StatusNotFound)
		return
	}

	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "无效 JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := rw.database.UpdateSoulFields(updates); err != nil {
		http.Error(w, "更新失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	log.Printf("[server] 更新人设: %s, fields=%v", name, updates)
}

// POST /api/workers/{name}/wakeup — 手动唤醒
func (s *Server) handleManualWakeup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	slug := sanitizeName(name)

	s.mu.RLock()
	rw, exists := s.workers[slug]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "工人不存在: "+name, http.StatusNotFound)
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Reason == "" {
		body.Reason = "手动唤醒"
	}

	wakeupTime := time.Now().Add(3 * time.Second).Format(time.RFC3339)
	rw.database.InsertWakeup(wakeupTime, body.Reason)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "wakeup_scheduled", "reason": body.Reason})
	log.Printf("[server] 手动唤醒: %s, reason=%s", name, body.Reason)
}

// POST /api/workers/{name}/event — 城市推送事件
func (s *Server) handlePushEvent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	slug := sanitizeName(name)

	s.mu.RLock()
	rw, exists := s.workers[slug]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "工人不存在: "+name, http.StatusNotFound)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		http.Error(w, "content 为必填项", http.StatusBadRequest)
		return
	}

	rw.database.InsertEvent(body.Content)

	soul, err := rw.database.GetSoul()
	urgent := err == nil && s.llmClient != nil && worker.CheckUrgency(s.llmClient, body.Content, soul)

	if urgent {
		select {
		case rw.wakeupCh <- worker.WakeupSignal{Trigger: "urgent_news", News: body.Content}:
		default:
			log.Printf("[server] wakeupCh 已满，事件已存入 events 表等待下次唤醒: %s", name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "received", "urgent": urgent})
	log.Printf("[server] 推送事件: %s, urgent=%v, content=%s", name, urgent, body.Content)
}

// DELETE /api/workers/{name} — 停止并删除工人
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	slug := sanitizeName(name)

	s.mu.Lock()
	rw, exists := s.workers[slug]
	if !exists {
		s.mu.Unlock()
		http.Error(w, "工人不存在: "+name, http.StatusNotFound)
		return
	}
	delete(s.workers, slug)
	s.mu.Unlock()

	rw.cancel()
	rw.database.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	log.Printf("[server] 删除工人: %s", name)
}

// ================================================================
//  工人生命周期
// ================================================================

func (s *Server) startWorker(name string, dbPath string, database *db.Database) {
	log.Printf("[server] startWorker: name=%s, llmClient=%v", name, s.llmClient != nil)
	ctx, cancel := context.WithCancel(context.Background())
	eng := engine.New(database, s.cityAPI, s.llmClient)
	wakeupCh := make(chan worker.WakeupSignal, 16)

	rw := &runningWorker{
		Name:     name,
		Status:   "running",
		DBPath:   dbPath,
		database: database,
		wakeupCh: wakeupCh,
		cancel:   cancel,
	}

	s.mu.Lock()
	s.workers[name] = rw
	s.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go worker.RunHeartbeat(ctx, database, s.cityAPI, s.llmClient, name, wakeupCh, &wg)
	go worker.RunWakeup(ctx, database, eng, wakeupCh, &wg)

	// 监控协程退出
	go func() {
		wg.Wait()
		s.mu.Lock()
		rw.Status = "stopped"
		s.mu.Unlock()
		log.Printf("[server] 工人停止: %s", name)
	}()
}

// ================================================================
//  辅助
// ================================================================

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sanitizeName(name string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ".", "_")
	return strings.ToLower(replacer.Replace(name))
}
