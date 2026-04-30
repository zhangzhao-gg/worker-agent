/**
 * [INPUT]: 依赖 internal/llm
 * [OUTPUT]: 对外提供 agentLoop 函数
 * [POS]: internal/engine 的推理循环，参考 s_full.py agent_loop 移植
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

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
//  Agent Loop 配置
// ================================================================

type loopConfig struct {
	Client   llm.Client
	Prompt   string
	Tools    []llm.ToolDef
	Handlers ToolHandlerMap
	Todo     *TodoManager
	LogFn    LogFunc
}

// ================================================================
//  Agent Loop
// ================================================================

func agentLoop(cfg loopConfig, initialMessage string) error {
	messages := []llm.Message{
		{Role: "user", Content: initialMessage},
	}

	log.Println("╔══════════════════════════════════════════════════════════")
	log.Println("║ AGENT LOOP 开始")
	log.Printf("║ 初始消息: %s", truncate(initialMessage, 120))
	log.Println("╚══════════════════════════════════════════════════════════")

	cfg.LogFn(0, "input", initialMessage)
	roundsWithoutTodo := 0

	for round := 0; round < MaxAgentRounds; round++ {
		// ── 压缩管线 ──
		microcompact(messages)
		tokens := estimateTokens(messages)
		if tokens > TokenThreshold {
			log.Printf("  ⚙ 触发自动压缩 (tokens≈%d)", tokens)
			messages = autoCompact(cfg.Client, messages)
		}

		// ── LLM 调用 ──
		log.Println("┌──────────────────────────────────────────────────────────")
		log.Printf("│ Round %d  |  messages=%d  |  tokens≈%d", round+1, len(messages), tokens)
		log.Println("│ 调用 LLM...")

		resp, err := cfg.Client.Chat(cfg.Prompt, messages, cfg.Tools)
		if err != nil {
			log.Printf("│ ✘ LLM 调用失败: %v", err)
			log.Println("└──────────────────────────────────────────────────────────")
			return fmt.Errorf("LLM 调用失败: %w", err)
		}

		messages = append(messages, resp.Message)

		// ── LLM 文本输出 ──
		if resp.Message.Content != "" {
			log.Printf("│ 💬 LLM: %s", truncate(resp.Message.Content, 200))
			cfg.LogFn(round+1, "llm_text", resp.Message.Content)
		}

		// ── 推理结束? ──
		if resp.StopReason != "tool_calls" {
			log.Printf("│ ■ 推理结束 (reason=%s)", resp.StopReason)
			log.Println("└──────────────────────────────────────────────────────────")
			log.Println("╔══════════════════════════════════════════════════════════")
			log.Printf("║ AGENT LOOP 完成  |  共 %d 轮", round+1)
			log.Println("╚══════════════════════════════════════════════════════════")
			cfg.LogFn(round+1, "finish", resp.StopReason)
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
			cfg.LogFn(round+1, "tool_call", name+"("+args+")")

			var output string
			handler, ok := cfg.Handlers[name]
			if !ok {
				output = "Unknown tool: " + name
			} else {
				var input map[string]any
				json.Unmarshal([]byte(args), &input)
				result, err := handler(input)
				if errors.Is(err, ErrSelfDestruct) {
					log.Println("│       ☠ 工人选择自我终结")
					log.Println("└──────────────────────────────────────────────────────────")
					cfg.LogFn(round+1, "self_destruct", name)
					return ErrSelfDestruct
				}
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

			cfg.LogFn(round+1, "tool_result", name+": "+truncate(output, 500))

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
		if cfg.Todo.HasOpenItems() && roundsWithoutTodo >= 3 {
			log.Println("  ⚠ Todo 偏移提醒（3 轮未更新）")
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: "<reminder>你还有未完成的 todo 步骤，请检查并更新。</reminder>",
			})
		}

		// ── 手动压缩 ──
		if manualCompress {
			log.Println("  ⚙ 手动压缩触发")
			messages = autoCompact(cfg.Client, messages)
		}
	}

	log.Println("╔══════════════════════════════════════════════════════════")
	log.Printf("║ ✘ AGENT LOOP 超限  |  达到最大轮次 %d", MaxAgentRounds)
	log.Println("╚══════════════════════════════════════════════════════════")
	return fmt.Errorf("推理达到最大轮次 %d", MaxAgentRounds)
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "..."
}
