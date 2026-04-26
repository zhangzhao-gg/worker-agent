/**
 * [INPUT]: 依赖 encoding/json
 * [OUTPUT]: 对外提供 Client 接口, Message/ToolCall/ToolDef/Response 类型
 * [POS]: internal/llm 的核心抽象，定义 LLM 交互契约，被 engine/worker 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package llm

import "encoding/json"

// ================================================================
//  LLM 客户端接口
// ================================================================

type Client interface {
	Chat(systemPrompt string, messages []Message, tools []ToolDef) (*Response, error)
}

// ================================================================
//  消息类型（OpenAI 兼容格式）
// ================================================================

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ================================================================
//  工具定义
// ================================================================

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ================================================================
//  响应
// ================================================================

type Response struct {
	Message    Message
	StopReason string // "stop" / "tool_calls" / "length"
}
