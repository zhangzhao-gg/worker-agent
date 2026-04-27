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
	"strings"

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

	log.Println("╔══════════════════════════════════════════════════════════")
	log.Println("║ AGENT LOOP 开始")
	log.Printf("║ 初始消息: %s", truncate(initialMessage, 120))
	log.Println("╚══════════════════════════════════════════════════════════")

	roundsWithoutTodo := 0

	for round := 0; round < MaxAgentRounds; round++ {
		// ── 压缩管线 ──
		microcompact(messages)
		tokens := estimateTokens(messages)
		if tokens > TokenThreshold {
			log.Printf("  ⚙ 触发自动压缩 (tokens≈%d)", tokens)
			messages = autoCompact(client, messages)
		}

		// ── LLM 调用 ──
		log.Println("┌──────────────────────────────────────────────────────────")
		log.Printf("│ Round %d  |  messages=%d  |  tokens≈%d", round+1, len(messages), tokens)
		log.Println("│ 调用 LLM...")

		resp, err := client.Chat(systemPrompt, messages, tools)
		if err != nil {
			log.Printf("│ ✘ LLM 调用失败: %v", err)
			log.Println("└──────────────────────────────────────────────────────────")
			return fmt.Errorf("LLM 调用失败: %w", err)
		}

		messages = append(messages, resp.Message)

		// ── LLM 文本输出 ──
		if resp.Message.Content != "" {
			log.Printf("│ 💬 LLM: %s", truncate(resp.Message.Content, 200))
		}

		// ── 推理结束? ──
		if resp.StopReason != "tool_calls" {
			log.Printf("│ ■ 推理结束 (reason=%s)", resp.StopReason)
			log.Println("└──────────────────────────────────────────────────────────")
			log.Println("╔══════════════════════════════════════════════════════════")
			log.Printf("║ AGENT LOOP 完成  |  共 %d 轮", round+1)
			log.Println("╚══════════════════════════════════════════════════════════")
			return nil
		}

		// ── 工具分发 ──
		log.Printf("│ 🔧 工具调用: %d 个", len(resp.Message.ToolCalls))
		usedTodo := false
		manualCompress := false

		for i, call := range resp.Message.ToolCalls {
			name := call.Function.Name
			args := call.Function.Arguments

			if name == "compress" {
				manualCompress = true
			}
			if name == "TodoWrite" {
				usedTodo = true
			}

			log.Printf("│   [%d] %s(%s)", i+1, name, truncate(args, 100))

			var output string
			handler, ok := handlers[name]
			if !ok {
				output = "Unknown tool: " + name
			} else {
				var input map[string]any
				json.Unmarshal([]byte(args), &input)
				result, err := handler(input)
				if err != nil {
					output = "Error: " + err.Error()
					log.Printf("│       ✘ %s", truncate(output, 150))
				} else {
					output = result
					log.Printf("│       ✔ %s", truncate(output, 150))
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
		}
		log.Println("└──────────────────────────────────────────────────────────")

		// ── Todo 偏移检查 ──
		if usedTodo {
			roundsWithoutTodo = 0
		} else {
			roundsWithoutTodo++
		}
		if todo.HasOpenItems() && roundsWithoutTodo >= 3 {
			log.Println("  ⚠ Todo 偏移提醒（3 轮未更新）")
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: "<reminder>你还有未完成的 todo 步骤，请检查并更新。</reminder>",
			})
		}

		// ── 手动压缩 ──
		if manualCompress {
			log.Println("  ⚙ 手动压缩触发")
			messages = autoCompact(client, messages)
		}
	}

	log.Println("╔══════════════════════════════════════════════════════════")
	log.Printf("║ ✘ AGENT LOOP 超限  |  达到最大轮次 %d", MaxAgentRounds)
	log.Println("╚══════════════════════════════════════════════════════════")
	return fmt.Errorf("推理达到最大轮次 %d", MaxAgentRounds)
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
