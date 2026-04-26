/**
 * [INPUT]: 依赖 net/http, encoding/json, llm.go 中的类型
 * [OUTPUT]: 对外提供 MiniMax struct（实现 Client 接口）
 * [POS]: internal/llm 的 MiniMax 实现，OpenAI 兼容 API
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ================================================================
//  MiniMax 客户端
// ================================================================

type MiniMax struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewMiniMax(baseURL, apiKey, model string) *MiniMax {
	return &MiniMax{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// ================================================================
//  OpenAI 兼容 API 类型（私有）
// ================================================================

type chatRequest struct {
	Model     string       `json:"model"`
	Messages  []Message    `json:"messages"`
	Tools     []apiToolDef `json:"tools,omitempty"`
	MaxTokens int          `json:"max_tokens,omitempty"`
}

type apiToolDef struct {
	Type     string      `json:"type"`
	Function apiFunction `json:"function"`
}

type apiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ================================================================
//  Chat 实现
// ================================================================

func (m *MiniMax) Chat(systemPrompt string, messages []Message, tools []ToolDef) (*Response, error) {
	// 组装消息：system prompt + 用户消息
	apiMessages := make([]Message, 0, len(messages)+1)
	apiMessages = append(apiMessages, Message{Role: "system", Content: systemPrompt})
	apiMessages = append(apiMessages, messages...)

	// 转换工具定义为 OpenAI 格式
	var apiTools []apiToolDef
	for _, t := range tools {
		apiTools = append(apiTools, apiToolDef{
			Type: "function",
			Function: apiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	reqBody := chatRequest{
		Model:     m.model,
		Messages:  apiMessages,
		Tools:     apiTools,
		MaxTokens: 8000,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求: %w", err)
	}

	req, err := http.NewRequest("POST", m.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构建请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("解析响应: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("API 返回空 choices")
	}

	choice := chatResp.Choices[0]
	return &Response{
		Message:    choice.Message,
		StopReason: choice.FinishReason,
	}, nil
}
