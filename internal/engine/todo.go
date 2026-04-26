/**
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 TodoManager struct
 * [POS]: internal/engine 的推理辅助，防止 LLM 推理偏移，参考 s_full.py TodoManager 移植
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import "fmt"

// ================================================================
//  核心结构体
// ================================================================

type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

type TodoManager struct {
	items []TodoItem
}

func NewTodoManager() *TodoManager {
	return &TodoManager{}
}

// ================================================================
//  操作
// ================================================================

func (t *TodoManager) Update(items []TodoItem) (string, error) {
	if len(items) > 20 {
		return "", fmt.Errorf("最多 20 条 todo")
	}

	inProgress := 0
	for i, item := range items {
		if item.Content == "" {
			return "", fmt.Errorf("item %d: content 不能为空", i)
		}
		if item.Status != "pending" && item.Status != "in_progress" && item.Status != "completed" {
			return "", fmt.Errorf("item %d: 无效 status '%s'", i, item.Status)
		}
		if item.ActiveForm == "" {
			return "", fmt.Errorf("item %d: activeForm 不能为空", i)
		}
		if item.Status == "in_progress" {
			inProgress++
		}
	}
	if inProgress > 1 {
		return "", fmt.Errorf("同时只能有 1 条 in_progress")
	}

	t.items = items
	return t.Render(), nil
}

func (t *TodoManager) Render() string {
	if len(t.items) == 0 {
		return "No todos."
	}

	marks := map[string]string{"completed": "[x]", "in_progress": "[>]", "pending": "[ ]"}
	done := 0
	result := ""

	for _, item := range t.items {
		mark := marks[item.Status]
		suffix := ""
		if item.Status == "in_progress" {
			suffix = " <- " + item.ActiveForm
		}
		if item.Status == "completed" {
			done++
		}
		result += fmt.Sprintf("%s %s%s\n", mark, item.Content, suffix)
	}
	result += fmt.Sprintf("\n(%d/%d completed)", done, len(t.items))
	return result
}

func (t *TodoManager) HasOpenItems() bool {
	for _, item := range t.items {
		if item.Status != "completed" {
			return true
		}
	}
	return false
}
