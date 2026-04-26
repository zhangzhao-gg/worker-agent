/**
 * [INPUT]: 依赖 internal/llm
 * [OUTPUT]: 对外提供 agentLoop 函数
 * [POS]: internal/engine 的推理循环，参考 s_full.py agent_loop 移植
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"encoding/json"
	"fmt"
	"log"

	"worker-agent/internal/llm"
)

// ================================================================
//  配置
// ================================================================

const (
	MaxAgentRounds = 30
	MaxToolOutput  = 50000
)

// ================================================================
//  工具 handler 类型
// ================================================================

type ToolHandler func(input map[string]any) (string, error)

type ToolHandlerMap map[string]ToolHandler

// ================================================================
//  Agent Loop
// ================================================================

func agentLoop(client llm.Client, systemPrompt string, initialMessage string, tools []llm.ToolDef, handlers ToolHandlerMap, todo *TodoManager) error {
	messages := []llm.Message{
		{Role: "user", Content: initialMessage},
	}

	roundsWithoutTodo := 0

	for round := 0; round < MaxAgentRounds; round++ {
		// ── 压缩管线 ──
		microcompact(messages)
		if estimateTokens(messages) > TokenThreshold {
			log.Println("[engine] 触发自动压缩")
			messages = autoCompact(client, messages)
		}

		// ── LLM 调用 ──
		resp, err := client.Chat(systemPrompt, messages, tools)
		if err != nil {
			return fmt.Errorf("LLM 调用失败: %w", err)
		}

		messages = append(messages, resp.Message)

		// ── 推理结束? ──
		if resp.StopReason != "tool_calls" {
			log.Printf("[engine] 推理结束 (reason: %s, rounds: %d)", resp.StopReason, round+1)
			return nil
		}

		// ── 工具分发 ──
		usedTodo := false
		manualCompress := false

		for _, call := range resp.Message.ToolCalls {
			name := call.Function.Name

			if name == "compress" {
				manualCompress = true
			}
			if name == "TodoWrite" {
				usedTodo = true
			}

			var output string
			handler, ok := handlers[name]
			if !ok {
				output = "Unknown tool: " + name
			} else {
				var input map[string]any
				json.Unmarshal([]byte(call.Function.Arguments), &input)
				result, err := handler(input)
				if err != nil {
					output = "Error: " + err.Error()
				} else {
					output = result
				}
			}

			if len(output) > MaxToolOutput {
				output = output[:MaxToolOutput] + "... [truncated]"
			}

			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    output,
				ToolCallID: call.ID,
			})

			log.Printf("[tool] %s: %s", name, truncate(output, 200))
		}

		// ── Todo 偏移检查 ──
		if usedTodo {
			roundsWithoutTodo = 0
		} else {
			roundsWithoutTodo++
		}
		if todo.HasOpenItems() && roundsWithoutTodo >= 3 {
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: "<reminder>你还有未完成的 todo 步骤，请检查并更新。</reminder>",
			})
		}

		// ── 手动压缩 ──
		if manualCompress {
			log.Println("[engine] 手动压缩触发")
			messages = autoCompact(client, messages)
		}
	}

	return fmt.Errorf("推理达到最大轮次 %d", MaxAgentRounds)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
