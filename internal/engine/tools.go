/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/llm
 * [OUTPUT]: 对外提供 loadToolDefs() 和 Engine.buildHandlers()
 * [POS]: internal/engine 的工具注册表，14 个工具（6 感知 + 6 行动 + 2 元）
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"worker-agent/internal/db"
	"worker-agent/internal/llm"
)

// ================================================================
//  Tool Schema（JSON 原文）
// ================================================================

var toolsJSON = `[
  {"name":"get_city_temperature","description":"感知当前城市温度，返回模糊描述。","input_schema":{"type":"object","properties":{}}},
  {"name":"get_food_status","description":"感知食物供给状态，返回模糊描述。","input_schema":{"type":"object","properties":{}}},
  {"name":"get_city_announcements","description":"获取执政官公告列表。","input_schema":{"type":"object","properties":{}}},
  {"name":"get_my_work_assignment","description":"获取今天城市分配给你的工作任务。","input_schema":{"type":"object","properties":{}}},
  {"name":"get_recent_events","description":"回忆最近发生的事件。","input_schema":{"type":"object","properties":{"n":{"type":"integer","description":"返回条数"}},"required":["n"]}},
  {"name":"get_memories","description":"回忆自己之前的想法和记忆。","input_schema":{"type":"object","properties":{"n":{"type":"integer","description":"返回条数"}},"required":["n"]}},
  {"name":"write_heartbeat_schedule","description":"批量写入今天的心跳计划。每条含时间和任务描述。","input_schema":{"type":"object","properties":{"entries":{"type":"array","items":{"type":"object","properties":{"time":{"type":"string","description":"HH:MM 格式"},"task":{"type":"string","description":"心跳任务描述"}},"required":["time","task"]}}},"required":["entries"]}},
  {"name":"update_heartbeat_schedule","description":"增删改心跳计划中的条目。","input_schema":{"type":"object","properties":{"changes":{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer","description":"计划条目 ID"},"action":{"type":"string","enum":["add","modify","delete"]},"time":{"type":"string"},"task":{"type":"string"}},"required":["action"]}}},"required":["changes"]}},
  {"name":"schedule_wakeup","description":"安排未来某个时间点唤醒你的大脑进行思考。同一小时内不会重复安排。","input_schema":{"type":"object","properties":{"datetime":{"type":"string","description":"ISO 格式时间"},"reason":{"type":"string","description":"唤醒原因"}},"required":["datetime","reason"]}},
  {"name":"cancel_wakeup","description":"取消一个不再需要的唤醒计划（只能取消 pending 状态的）。","input_schema":{"type":"object","properties":{"id":{"type":"integer","description":"唤醒计划的 ID"}},"required":["id"]}},
  {"name":"write_narrative","description":"写下你的对外叙事，会被同步到城市日志，其他人可以看到。","input_schema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}},
  {"name":"write_memory","description":"写下私人记忆，只有你自己能看到。","input_schema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}},
  {"name":"update_soul","description":"批量更新你的情绪状态。只能修改 mood、hope、grievance。值无范围限制，由你自主决定。","input_schema":{"type":"object","properties":{"updates":{"type":"array","items":{"type":"object","properties":{"field":{"type":"string","enum":["mood","hope","grievance"]},"value":{"type":"integer","description":"情绪值，无范围限制"}},"required":["field","value"]}}},"required":["updates"]}},
  {"name":"TodoWrite","description":"追踪你当前的思考步骤。用于防止偏移，确保完成所有计划。","input_schema":{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"content":{"type":"string"},"status":{"type":"string","enum":["pending","in_progress","completed"]},"activeForm":{"type":"string"}},"required":["content","status","activeForm"]}}},"required":["items"]}},
  {"name":"compress","description":"手动触发上下文压缩，当对话过长时使用。","input_schema":{"type":"object","properties":{}}}
]`

func loadToolDefs() []llm.ToolDef {
	var tools []llm.ToolDef
	json.Unmarshal([]byte(toolsJSON), &tools)
	return tools
}

// ================================================================
//  Handler 注册
// ================================================================

func (e *Engine) buildHandlers(todo *TodoManager) ToolHandlerMap {
	return ToolHandlerMap{
		// ── 感知 ──
		"get_city_temperature":   func(input map[string]any) (string, error) { return e.cityAPI.GetCityTemperature() },
		"get_food_status":        func(input map[string]any) (string, error) { return e.cityAPI.GetFoodStatus() },
		"get_city_announcements": func(input map[string]any) (string, error) { return marshalResult(e.cityAPI.GetCityAnnouncements()) },
		"get_my_work_assignment": func(input map[string]any) (string, error) { return e.cityAPI.GetMyWorkAssignment("") },
		"get_recent_events":      func(input map[string]any) (string, error) { return marshalResult(e.db.GetRecentEvents(intFromInput(input, "n"))) },
		"get_memories":           func(input map[string]any) (string, error) { return marshalResult(e.db.GetRecentMemories(intFromInput(input, "n"))) },

		// ── 行动 ──
		"write_heartbeat_schedule":  func(input map[string]any) (string, error) { return e.handleWriteHeartbeats(input) },
		"update_heartbeat_schedule": func(input map[string]any) (string, error) { return e.handleUpdateHeartbeats(input) },
		"schedule_wakeup":           func(input map[string]any) (string, error) { return e.handleScheduleWakeup(input) },
		"cancel_wakeup":             func(input map[string]any) (string, error) { return e.handleCancelWakeup(input) },
		"write_narrative":           func(input map[string]any) (string, error) { return e.handleWriteNarrative(input) },
		"write_memory":              func(input map[string]any) (string, error) { return e.handleWriteMemory(input) },
		"update_soul":               func(input map[string]any) (string, error) { return e.handleUpdateSoul(input) },

		// ── 元工具 ──
		"TodoWrite": func(input map[string]any) (string, error) {
			raw, _ := json.Marshal(input["items"])
			var items []TodoItem
			if err := json.Unmarshal(raw, &items); err != nil {
				return "", fmt.Errorf("解析 todo: %w", err)
			}
			return todo.Update(items)
		},
		"compress": func(input map[string]any) (string, error) {
			return "上下文压缩将在本轮结束后执行", nil
		},
	}
}

// ================================================================
//  行动类 handler 实现
// ================================================================

func (e *Engine) handleWriteHeartbeats(input map[string]any) (string, error) {
	raw, _ := json.Marshal(input["entries"])
	var entries []struct {
		Time string `json:"time"`
		Task string `json:"task"`
	}
	json.Unmarshal(raw, &entries)

	today := time.Now().Format("2006-01-02")
	var dbEntries []db.HeartbeatEntry
	for _, en := range entries {
		dbEntries = append(dbEntries, db.HeartbeatEntry{Time: en.Time, Date: today, Task: en.Task})
	}
	if err := e.db.InsertHeartbeats(dbEntries); err != nil {
		return "", err
	}
	return fmt.Sprintf("写入 %d 条心跳计划", len(entries)), nil
}

func (e *Engine) handleUpdateHeartbeats(input map[string]any) (string, error) {
	// TODO: 解析 changes 数组，执行 add/modify/delete
	return "心跳计划已更新", nil
}

func (e *Engine) handleScheduleWakeup(input map[string]any) (string, error) {
	dt, _ := input["datetime"].(string)
	reason, _ := input["reason"].(string)
	if err := e.db.InsertWakeup(dt, reason); err != nil {
		return "", err
	}
	return fmt.Sprintf("已安排唤醒: %s (%s)", dt, reason), nil
}

func (e *Engine) handleCancelWakeup(input map[string]any) (string, error) {
	id := int64(intFromInput(input, "id"))
	if err := e.db.CancelWakeup(id); err != nil {
		return "", err
	}
	return fmt.Sprintf("已取消唤醒: id=%d", id), nil
}

func (e *Engine) handleWriteNarrative(input map[string]any) (string, error) {
	text, _ := input["text"].(string)
	if err := e.db.InsertNarrative(text); err != nil {
		return "", err
	}
	e.cityAPI.PostNarrative("", text)
	return "叙事已记录并同步到城市", nil
}

func (e *Engine) handleWriteMemory(input map[string]any) (string, error) {
	text, _ := input["text"].(string)
	return "记忆已记录", e.db.InsertMemory(text, "memory")
}

func (e *Engine) handleUpdateSoul(input map[string]any) (string, error) {
	raw, _ := json.Marshal(input["updates"])
	var updates []db.SoulUpdate
	json.Unmarshal(raw, &updates)
	return "情绪已更新", e.db.UpdateSoul(updates)
}

// ================================================================
//  辅助
// ================================================================

func intFromInput(input map[string]any, key string) int {
	switch v := input[key].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 5
	}
}

func marshalResult[T any](data T, err error) (string, error) {
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(data)
	return string(b), nil
}
