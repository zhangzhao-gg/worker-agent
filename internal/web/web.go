/**
 * [INPUT]: 依赖 internal/db, html/template, embed
 * [OUTPUT]: 对外提供 Handler struct + Register() 方法
 * [POS]: internal/web 的核心，Web UI 路由 + 模板渲染，挂载到现有 HTTP mux
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package web

import (
	"embed"
	"html/template"
	"net/http"
	"sync"
	"time"

	"worker-agent/internal/db"
)

//go:embed templates/*.html
var templateFS embed.FS

// ================================================================
//  核心结构体
// ================================================================

type WorkerEntry struct {
	Name   string
	Status string
	DB     *db.Database
}

type Handler struct {
	workers   func() []WorkerEntry
	tmpl      *template.Template
	WorkerAPI string
}

func New(workers func() []WorkerEntry) *Handler {
	funcMap := template.FuncMap{
		"fmtTime": func(s string) string {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				t2, err2 := time.Parse("2006-01-02T15:04:05Z07:00", s)
				if err2 != nil {
					return s
				}
				t = t2
			}
			return t.Format("02 Jan 2006, 15:04")
		},
		"moodPct": func(v int) int {
			if v < 0 {
				return 0
			}
			if v > 100 {
				return 100
			}
			return v
		},
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))

	return &Handler{workers: workers, tmpl: tmpl}
}

// ================================================================
//  路由注册
// ================================================================

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.handleIndex)
	mux.HandleFunc("GET /worker/{name}", h.handleDetail)
}

// ================================================================
//  首页：工人列表
// ================================================================

type indexData struct {
	Workers []indexWorker
}

type indexWorker struct {
	Name       string
	Occupation string
	Status     string
	Mood       int
	Hope       int
	Grievance  int
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	entries := h.workers()
	var data indexData

	for _, e := range entries {
		soul, err := e.DB.GetSoul()
		iw := indexWorker{Name: e.Name, Status: e.Status}
		if err == nil {
			iw.Occupation = soul.Occupation
			iw.Mood = soul.Mood
			iw.Hope = soul.Hope
			iw.Grievance = soul.Grievance
		}
		data.Workers = append(data.Workers, iw)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "index.html", data)
}

// ================================================================
//  详情页：单个工人完整档案
// ================================================================

type detailData struct {
	Name          string
	Status        string
	Soul          db.Soul
	Narratives    []db.Narrative
	Memories      []db.Memory
	Events        []db.Event
	Heartbeats    []db.HeartbeatEntry
	Wakeups       []db.WakeupEntry
	ReasoningLogs []db.ReasoningLog
	Tab           string
	WorkerAPI     string
}

func (h *Handler) handleDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "narrative"
	}

	entries := h.workers()
	var target *WorkerEntry
	for i := range entries {
		if entries[i].Name == name {
			target = &entries[i]
			break
		}
	}

	if target == nil {
		http.NotFound(w, r)
		return
	}

	soul, _ := target.DB.GetSoul()

	data := detailData{
		Name:      target.Name,
		Status:    target.Status,
		Soul:      soul,
		Tab:       tab,
		WorkerAPI: h.WorkerAPI,
	}

	var wg sync.WaitGroup
	var narratives []db.Narrative
	var memories []db.Memory
	var events []db.Event
	var heartbeats []db.HeartbeatEntry
	var wakeups []db.WakeupEntry
	var reasoningLogs []db.ReasoningLog

	wg.Add(6)
	go func() { defer wg.Done(); narratives, _ = target.DB.GetRecentNarratives(20) }()
	go func() { defer wg.Done(); memories, _ = target.DB.GetRecentMemories(20) }()
	go func() { defer wg.Done(); events, _ = target.DB.GetRecentEvents(20) }()
	go func() { defer wg.Done(); heartbeats, _ = target.DB.GetRecentHeartbeats(20) }()
	go func() { defer wg.Done(); wakeups, _ = target.DB.GetRecentWakeups(20) }()
	go func() { defer wg.Done(); reasoningLogs, _ = target.DB.GetRecentReasoningLogs(100) }()
	wg.Wait()

	data.Narratives = narratives
	data.Memories = memories
	data.Events = events
	data.Heartbeats = heartbeats
	data.Wakeups = wakeups
	data.ReasoningLogs = reasoningLogs

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "detail.html", data)
}
