/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/llm, internal/msgrouter
 * [OUTPUT]: 对外提供 Engine struct 及 Run()/RunConversation() 方法
 * [POS]: internal/engine 的入口，推理引擎骨架，持有 db/cityAPI/llm/router 引用
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
	"worker-agent/internal/msgrouter"
)

var ErrSelfDestruct = errors.New("工人选择自我终结")
var ErrConversationDone = errors.New("对话回复完成")

// ================================================================
//  核心结构体
// ================================================================

type Engine struct {
	db         *db.Database
	cityAPI    *city.CityAPI
	llm        llm.Client
	router     *msgrouter.MessageRouter
	workerName string
}

// RunContext 业务层注入的上下文
type RunContext struct {
	Soul               db.Soul
	PersistentMemories []db.Memory
	Events             []db.Event
	Wakeups            []db.WakeupEntry
	Heartbeats         []db.HeartbeatEntry
	WorkAssignment     string
	Reason             string
	News               string
}

func New(database *db.Database, cityAPI *city.CityAPI, llmClient llm.Client, router *msgrouter.MessageRouter, workerName string) *Engine {
	return &Engine{db: database, cityAPI: cityAPI, llm: llmClient, router: router, workerName: workerName}
}

// ================================================================
//  入口：正常推理
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

// ================================================================
//  入口：对话模式
// ================================================================

func (e *Engine) RunConversation(convID, senderName, message string, ctx RunContext) error {
	log.Printf("[engine] RunConversation: from=%s, conv=%s, soul=%s", senderName, convID, ctx.Soul.Name)

	if e.llm == nil {
		if e.router != nil {
			e.router.Reply(convID, e.workerName, "[无法回复：LLM 离线]", true)
		}
		return nil
	}

	sessionID := fmt.Sprintf("%s_conv_%s", ctx.Soul.Name, time.Now().Format("20060102_150405"))
	logFn := func(round int, logType string, content string) {
		e.db.InsertReasoningLog(sessionID, round, logType, content)
	}

	todo := NewTodoManager()
	loopCfg := loopConfig{
		Client:    e.llm,
		Prompt:    buildConversationPrompt(ctx, senderName),
		Tools:     loadConversationToolDefs(),
		Handlers:  e.buildConversationHandlers(convID, senderName, todo),
		Todo:      todo,
		LogFn:     logFn,
		MaxRounds: MaxAgentRounds,
	}

	initialMsg := fmt.Sprintf("当前时间：%s\n\n%s 对你说：「%s」\n\n请思考后回复。用 reply_message 回复并继续对话，或用 end_conversation 说最后一句话结束对话。",
		time.Now().Format("2006-01-02 15:04"), senderName, message)

	return agentLoop(loopCfg, initialMsg)
}
