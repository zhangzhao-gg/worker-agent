/**
 * [INPUT]: 依赖 internal/llm, encoding/json
 * [OUTPUT]: 对外提供 estimateTokens, microcompact, autoCompact 函数
 * [POS]: internal/engine 的上下文压缩管线，参考 s_full.py compression 移植
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"encoding/json"
	"fmt"
	"log"

	"worker-agent/internal/llm"
)

const TokenThreshold = 100000

// ================================================================
//  Token 估算
// ================================================================

func estimateTokens(messages []llm.Message) int {
	b, _ := json.Marshal(messages)
	return len(b) / 4
}

// ================================================================
//  Microcompact：清理早期 tool result
// ================================================================

func microcompact(messages []llm.Message) {
	var toolIndices []int
	for i, msg := range messages {
		if msg.Role == "tool" {
			toolIndices = append(toolIndices, i)
		}
	}
	if len(toolIndices) <= 3 {
		return
	}
	for _, idx := range toolIndices[:len(toolIndices)-3] {
		if len(messages[idx].Content) > 100 {
			messages[idx].Content = "[cleared]"
		}
	}
}

// ================================================================
//  AutoCompact：摘要替换
// ================================================================

func autoCompact(client llm.Client, messages []llm.Message) []llm.Message {
	convText := ""
	for _, msg := range messages {
		convText += fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content)
	}
	if len(convText) > 80000 {
		convText = convText[:80000]
	}

	resp, err := client.Chat(
		"你是一个摘要助手。请用中文简要总结以下对话，保留所有关键信息、决策和待办事项。",
		[]llm.Message{{Role: "user", Content: "请总结以下对话：\n" + convText}},
		nil,
	)
	if err != nil {
		log.Printf("[compress] 摘要生成失败，跳过压缩: %v", err)
		return messages
	}

	return []llm.Message{
		{Role: "user", Content: "[上下文已压缩]\n" + resp.Message.Content},
		{Role: "assistant", Content: "好的，我已理解之前的上下文。继续工作。"},
	}
}
