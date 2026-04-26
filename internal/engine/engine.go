/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/llm
 * [OUTPUT]: 对外提供 Engine struct 及 Run() 方法
 * [POS]: internal/engine 的入口，推理引擎骨架，持有 db/cityAPI/llm 引用
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
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
	systemPrompt := buildSystemPrompt(ctx)
	initialMsg := buildInitialMessage(trigger, ctx)
	tools := loadToolDefs()
	todo := NewTodoManager()
	handlers := e.buildHandlers(todo)

	return agentLoop(e.llm, systemPrompt, initialMsg, tools, handlers, todo)
}
