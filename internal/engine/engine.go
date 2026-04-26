/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/llm
 * [OUTPUT]: 对外提供 Engine struct 及 Run() 方法
 * [POS]: internal/engine 的入口，推理引擎骨架，持有 db/cityAPI/llm 引用
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"log"

	"worker-agent/internal/city"
	"worker-agent/internal/db"
	"worker-agent/internal/llm"
)

// ================================================================
//  核心结构体
// ================================================================

type Engine struct {
	db      *db.Database
	cityAPI *city.CityAPI
	llm     llm.Client
}

// RunContext 业务层注入的上下文
type RunContext struct {
	Soul    db.Soul
	Summary string
	Events  []db.Event
	Reason  string
	News    string
}

func New(database *db.Database, cityAPI *city.CityAPI, llmClient llm.Client) *Engine {
	return &Engine{db: database, cityAPI: cityAPI, llm: llmClient}
}

// ================================================================
//  入口
// ================================================================

func (e *Engine) Run(trigger string, ctx RunContext) error {
	log.Printf("[engine] Run 开始: trigger=%s, soul=%s", trigger, ctx.Soul.Name)

	if e.llm == nil {
		log.Println("[engine] LLM 客户端为 nil，跳过推理")
		return nil
	}

	systemPrompt := buildSystemPrompt(ctx)
	initialMsg := buildInitialMessage(trigger, ctx)
	tools := loadToolDefs()
	todo := NewTodoManager()
	handlers := e.buildHandlers(todo)

	log.Printf("[engine] 启动 agentLoop, tools=%d, initialMsg长度=%d", len(tools), len(initialMsg))
	return agentLoop(e.llm, systemPrompt, initialMsg, tools, handlers, todo)
}
