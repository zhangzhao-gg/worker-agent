/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/llm
 * [OUTPUT]: 对外提供 Engine struct 及 Run() 方法
 * [POS]: internal/engine 的入口，推理引擎骨架，持有 db/cityAPI/llm 引用
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"errors"
	"fmt"
	"log"
	"time"

	"worker-agent/internal/city"
	"worker-agent/internal/db"
	"worker-agent/internal/llm"
)

var ErrSelfDestruct = errors.New("工人选择自我终结")

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
	Soul           db.Soul
	Summary        string
	Events         []db.Event
	Wakeups        []db.WakeupEntry
	WorkAssignment string
	Reason         string
	News           string
}

func New(database *db.Database, cityAPI *city.CityAPI, llmClient llm.Client) *Engine {
	return &Engine{db: database, cityAPI: cityAPI, llm: llmClient}
}

// ================================================================
//  入口
// ================================================================

// LogFunc 推理日志回调，sessionID 已被闭包捕获
type LogFunc func(round int, logType string, content string)

func (e *Engine) Run(trigger string, ctx RunContext) error {
	log.Printf("[engine] Run 开始: trigger=%s, soul=%s", trigger, ctx.Soul.Name)

	if e.llm == nil {
		log.Println("[engine] LLM 客户端为 nil，跳过推理")
		return nil
	}

	if ctx.WorkAssignment == "" {
		ctx.WorkAssignment, _ = e.cityAPI.GetMyWorkAssignment(ctx.Soul.Name)
	}

	sessionID := fmt.Sprintf("%s_%s", ctx.Soul.Name, time.Now().Format("20060102_150405"))
	logFn := func(round int, logType string, content string) {
		if err := e.db.InsertReasoningLog(sessionID, round, logType, content); err != nil {
			log.Printf("[engine] 写入推理日志失败: %v", err)
		}
	}

	todo := NewTodoManager()
	loopCfg := loopConfig{
		Client:   e.llm,
		Prompt:   buildSystemPrompt(ctx),
		Tools:    loadToolDefs(),
		Handlers: e.buildHandlers(todo),
		Todo:     todo,
		LogFn:    logFn,
	}

	initialMsg := buildInitialMessage(trigger, ctx)
	log.Printf("[engine] 启动 agentLoop, tools=%d, initialMsg长度=%d, session=%s", len(loopCfg.Tools), len(initialMsg), sessionID)
	return agentLoop(loopCfg, initialMsg)
}
