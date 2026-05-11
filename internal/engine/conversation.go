/**
 * [INPUT]: 依赖 internal/msgrouter, internal/llm
 * [OUTPUT]: 对外提供 loadConversationToolDefs(), buildConversationHandlers()
 * [POS]: internal/engine 的对话模式工具集，receiver 侧专用，支持多轮
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"encoding/json"
	"fmt"

	"worker-agent/internal/llm"
	"worker-agent/internal/msgrouter"
)

// ================================================================
//  对话模式 Tool Schema
// ================================================================

var conversationToolsJSON = `[
  {"name":"reply_message","description":"回复对方并等待对方下一句话。对方可能继续说，也可能不再回复（超时）。","input_schema":{"type":"object","properties":{"content":{"type":"string","description":"你的回复内容"}},"required":["content"]}},
  {"name":"end_conversation","description":"说最后一句话并结束对话。对方不会再回复。","input_schema":{"type":"object","properties":{"content":{"type":"string","description":"你的最后一句话"}},"required":["content"]}},
  {"name":"recall","description":"回忆过去的事件和想法。","input_schema":{"type":"object","properties":{"type":{"type":"string","enum":["events","memories"],"description":"回忆类型"},"n":{"type":"integer","description":"返回条数"}},"required":["type","n"]}},
  {"name":"write_memory","description":"记下重要的对话内容作为私密记忆。","input_schema":{"type":"object","properties":{"text":{"type":"string"},"persistent":{"type":"boolean","description":"是否为持久记忆，上限10条"}},"required":["text"]}}
]`

func loadConversationToolDefs() []llm.ToolDef {
	var tools []llm.ToolDef
	json.Unmarshal([]byte(conversationToolsJSON), &tools)
	return tools
}

// ================================================================
//  对话模式 Handler
// ================================================================

func (e *Engine) buildConversationHandlers(convID, senderName string, todo *TodoManager) ToolHandlerMap {
	return ToolHandlerMap{
		"reply_message": func(input map[string]any) (string, error) {
			content, _ := input["content"].(string)
			content = stripThink(content)
			if e.router == nil {
				return "", fmt.Errorf("消息路由未初始化")
			}
			if err := e.router.Reply(convID, e.workerName, content, false); err != nil {
				return "", err
			}
			nextMsg, err := e.router.WaitNextMessage(convID, msgrouter.RecvTimeout)
			if err != nil {
				return "回复已送达，但对方没有继续说话。对话自然结束。", ErrConversationDone
			}
			return fmt.Sprintf("%s 继续说：「%s」", senderName, nextMsg), nil
		},

		"end_conversation": func(input map[string]any) (string, error) {
			content, _ := input["content"].(string)
			content = stripThink(content)
			if e.router == nil {
				return "", fmt.Errorf("消息路由未初始化")
			}
			if err := e.router.Reply(convID, e.workerName, content, true); err != nil {
				return "", err
			}
			return "对话已结束", ErrConversationDone
		},

		"recall": func(input map[string]any) (string, error) {
			return e.handleRecall(input)
		},

		"write_memory": func(input map[string]any) (string, error) {
			return e.handleWriteMemory(input)
		},
	}
}
