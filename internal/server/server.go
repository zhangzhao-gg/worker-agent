/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/engine, internal/worker, internal/llm, internal/narrator
 * [OUTPUT]: 对外提供 Server struct，HTTP API 入口 + 城市公告读取 + 工人生命周期管理 + 事件推送端点 + 实时推理/对话占用状态 + 对话关闭端点
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
	"sync/atomic"
	"time"

	"worker-agent/internal/city"
	"worker-agent/internal/db"
	"worker-agent/internal/engine"
	"worker-agent/internal/llm"
	"worker-agent/internal/msgrouter"
	"worker-agent/internal/narrator"
	"worker-agent/internal/worker"
)

// ================================================================
//  核心结构体
// ================================================================

type Server struct {
	dataDir   string
	cityAPI   *city.CityAPI
	llmClient llm.Client
	msgRouter *msgrouter.MessageRouter
	narrator  *narrator.Narrator
	workers   map[string]*runningWorker
	mu        sync.RWMutex
}

type runningWorker struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	DBPath    string `json:"db_path"`
	database  *db.Database
	wakeupCh  chan<- worker.WakeupSignal
	reasoning *atomic.Bool
	chatWith  atomic.Value
	cancel    context.CancelFunc
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

	msgDBPath := filepath.Join(dataDir, "_messages.db")
	router, err := msgrouter.New(msgDBPath)
	if err != nil {
		log.Fatalf("[server] 创建消息路由器失败: %v", err)
	}

	s := &Server{
		dataDir:   dataDir,
		cityAPI:   cityAPI,
		llmClient: llmClient,
		msgRouter: router,
		workers:   make(map[string]*runningWorker),
	}
	s.initNarrator()

	router.SetWakeupFn(func(workerName string, signal msgrouter.ConversationSignal) bool {
		s.mu.RLock()
		rw, exists := s.workers[sanitizeName(workerName)]
		s.mu.RUnlock()
		if !exists {
			return false
		}
		ws := worker.WakeupSignal{
			Trigger: "conversation",
			Conversation: &worker.ConversationContext{
				ConversationID: signal.ConversationID,
				SenderName:     signal.SenderName,
				Content:        signal.Content,
			},
		}
		select {
		case rw.wakeupCh <- ws:
			return true
		default:
			return false
		}
	})

	return s
}

func (s *Server) initNarrator() {
	narratorDB, err := db.New(filepath.Join(s.dataDir, "_narrator.db"))
	if err != nil {
		log.Printf("[server] 创建叙事者 DB 失败: %v", err)
		return
	}
	s.narrator = narrator.New(narratorDB, s.llmClient, s.createNarratedWorker)
	log.Printf("[server] 叙事者已初始化")
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
	mux.HandleFunc("POST /api/workers/{name}/chat", s.handleChat)
	mux.HandleFunc("POST /api/workers/{name}/chat/close", s.handleCloseChat)
	mux.HandleFunc("POST /api/workers/{name}/event", s.handlePushEvent)
	mux.HandleFunc("DELETE /api/workers/{name}", s.handleDelete)
	mux.HandleFunc("GET /api/city/announcements", s.handleCityAnnouncements)

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

func (s *Server) ResolveContact(req engine.ContactRequest) (string, error) {
	if strings.TrimSpace(req.Target) == "" {
		return "联系受阻：目标为空", nil
	}
	if s.contactReachable(req.Target, req.Database) {
		log.Printf("[contacts] 可直接联系: requester=%s target=%s", req.Requester, req.Target)
		return "", nil
	}
	if rejected, reason, err := contactRejected(req.Target, req.Database); err != nil {
		return "", err
	} else if rejected {
		log.Printf("[contacts] 叙事者曾拒绝: requester=%s target=%s reason=%s", req.Requester, req.Target, reason)
		return fmt.Sprintf("叙事者拒绝让「%s」成为可联系角色：%s", req.Target, reason), nil
	}

	if s.narrator == nil {
		log.Printf("[contacts] 叙事者未初始化: requester=%s target=%s", req.Requester, req.Target)
		return fmt.Sprintf("叙事者暂时沉默。请先补充「%s」是谁，以及你为什么要联系他。", req.Target), nil
	}

	log.Printf("[contacts] 交给叙事者裁定: requester=%s target=%s message=%q", req.Requester, req.Target, req.Message)
	soul, _ := req.Database.GetSoul()
	memories, _ := req.Database.GetRecentMemories(8)
	contacts, _ := req.Database.GetContacts()
	outcome := s.narrator.Resolve(narrator.Request{
		Requester:        req.Requester,
		Target:           req.Target,
		Message:          req.Message,
		RequesterSoul:    soul,
		RequesterMemory:  memories,
		RequesterContact: contacts,
	})
	log.Printf("[contacts] 叙事者裁定: requester=%s target=%s outcome=%s worker=%s reason=%s question=%s",
		req.Requester, req.Target, outcome.Kind, outcome.Character.Name, outcome.Reason, outcome.Question)
	_ = saveContactOutcome(req.Target, req.Database, outcome)
	return outcome.Message, nil
}

func (s *Server) contactReachable(name string, database *db.Database) bool {
	if s.workerRunning(name) {
		return true
	}
	contact, ok, _ := database.GetContact(name)
	return ok && contact.Status == "active" && contact.TargetWorker != "" && s.workerRunning(contact.TargetWorker)
}

func contactRejected(name string, database *db.Database) (bool, string, error) {
	contact, ok, err := database.GetContact(name)
	if err != nil || !ok || contact.Status != "rejected" {
		return false, "", err
	}
	return true, contact.RejectionReason, nil
}

func saveContactOutcome(name string, database *db.Database, outcome narrator.Outcome) error {
	contact := db.Contact{Name: name, Kind: "unresolved", Status: "unresolved", CreatedBy: "narrator"}
	switch outcome.Kind {
	case narrator.Create:
		contact.Kind = "worker"
		contact.Status = "active"
		contact.Relation = outcome.Character.Relation
		contact.TargetWorker = outcome.Character.Name
		contact.Notes = outcome.Character.Notes
	case narrator.Reject:
		contact.Status = "rejected"
		contact.RejectionReason = outcome.Reason
	default:
		contact.Notes = outcome.Question
	}
	log.Printf("[contacts] 写回联系人: name=%s status=%s kind=%s target=%s", contact.Name, contact.Status, contact.Kind, contact.TargetWorker)
	return database.UpsertContact(contact)
}

func (s *Server) createNarratedWorker(profile narrator.CharacterProfile) error {
	if strings.TrimSpace(profile.Name) == "" {
		return fmt.Errorf("角色名为空")
	}

	slug := sanitizeName(profile.Name)
	s.mu.RLock()
	_, running := s.workers[slug]
	s.mu.RUnlock()
	if running {
		log.Printf("[narrator] 工人已存在，跳过创建: %s", profile.Name)
		return nil
	}

	dbPath := filepath.Join(s.dataDir, slug+".db")
	database, err := db.New(dbPath)
	if err != nil {
		return err
	}

	if _, err := database.GetSoul(); err != nil {
		soul := db.Soul{
			Name:        profile.Name,
			Occupation:  firstNonEmpty(profile.Occupation, "新伦敦居民"),
			Background:  profile.Background,
			Personality: profile.Personality,
			SpeechStyle: profile.SpeechStyle,
			ValuesDesc:  profile.ValuesDesc,
			Family:      profile.Family,
			Mood:        50,
			Hope:        50,
			Grievance:   0,
			BodyStatus:  "健康",
		}
		if err := database.InitSoul(soul); err != nil {
			database.Close()
			return err
		}
		firstWakeup := time.Now().Add(5 * time.Second).Format(time.RFC3339)
		_ = database.InsertWakeup(firstWakeup, "叙事者让你进入新伦敦，开始理解自己的处境")
	}

	s.startWorker(slug, dbPath, database)
	log.Printf("[server] 叙事者创建工人: %s", profile.Name)
	return nil
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

// GET /api/city/announcements — 读取今日城市公告
func (s *Server) handleCityAnnouncements(w http.ResponseWriter, r *http.Request) {
	announcements, err := s.cityAPI.GetCityAnnouncements()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if announcements == nil {
		announcements = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"announcements": announcements})
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

	chatWith, _ := rw.chatWith.Load().(string)
	resp := map[string]any{
		"name":          rw.Name,
		"status":        rw.Status,
		"soul":          soul,
		"reasoning":     rw.reasoning.Load(),
		"chatting_with": chatWith,
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

// POST /api/workers/{name}/chat — 访客对话
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
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
		Content        string `json:"content"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		http.Error(w, "content 为必填项", http.StatusBadRequest)
		return
	}

	if body.ConversationID == "" && rw.reasoning.Load() {
		chatWith, _ := rw.chatWith.Load().(string)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]any{
			"error":         busyMessage(chatWith),
			"reasoning":     true,
			"chatting_with": chatWith,
		})
		return
	}

	reply, err := s.msgRouter.SendAndWait("visitor", slug, body.Content, body.ConversationID, 120*time.Second)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		log.Printf("[server] chat 失败: %s, err=%v", name, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"reply":           reply.Content,
		"ended":           reply.Ended,
		"conversation_id": reply.ConversationID,
	})
	log.Printf("[server] chat: visitor → %s, ended=%v", name, reply.Ended)
}

func busyMessage(chatWith string) string {
	if chatWith != "" {
		return "对方正在跟别人聊天"
	}
	return "对方正在沉思"
}

// POST /api/workers/{name}/chat/close — 访客关闭对话
func (s *Server) handleCloseChat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ConversationID == "" {
		http.Error(w, "conversation_id 为必填项", http.StatusBadRequest)
		return
	}

	closed := s.msgRouter.CloseConversation(body.ConversationID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"closed": closed})
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
	eng := engine.New(database, s.cityAPI, s.llmClient, s.msgRouter, name)
	eng.SetContactResolver(s)
	wakeupCh := make(chan worker.WakeupSignal, 16)
	reasoning := &atomic.Bool{}

	rw := &runningWorker{
		Name:      name,
		Status:    "running",
		DBPath:    dbPath,
		database:  database,
		wakeupCh:  wakeupCh,
		reasoning: reasoning,
		cancel:    cancel,
	}

	s.mu.Lock()
	s.workers[name] = rw
	s.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go worker.RunHeartbeat(ctx, database, s.cityAPI, s.llmClient, name, wakeupCh, &wg)
	go worker.RunWakeup(ctx, database, eng, wakeupCh, &wg, reasoning, &rw.chatWith)

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

func (s *Server) workerRunning(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.workers[sanitizeName(name)]
	return ok
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
