/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/llm
 * [OUTPUT]: 对外提供 loadToolDefs() 和 Engine.buildHandlers()
 * [POS]: internal/engine 的工具注册表，12 个工具（2 感知 + 9 行动 + 1 终极）
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"worker-agent/internal/db"
	"worker-agent/internal/llm"
)

var thinkTagRe = regexp.MustCompile(`<think>[\s\S]*?</think>\s*`)

// ================================================================
//  Tool Schema（JSON 原文）
// ================================================================

var toolsJSON = `[
  {"name":"sense_city","description":"感知城市环境：温度、食物供给、执政官公告、今日工作分配，一次性获取全部外部信息。","input_schema":{"type":"object","properties":{}}},
  {"name":"recall","description":"回忆过去的事件和想法。","input_schema":{"type":"object","properties":{"type":{"type":"string","enum":["events","memories"],"description":"回忆类型：events=发生的事件，memories=自己的想法记忆"},"n":{"type":"integer","description":"返回条数"}},"required":["type","n"]}},
  {"name":"manage_heartbeat","description":"管理心跳计划（工作时间内每10分钟的任务安排）。批量写入或增删改已有条目。","input_schema":{"type":"object","properties":{"date":{"type":"string","description":"YYYY-MM-DD 格式，不填则为今天"},"entries":{"type":"array","description":"批量写入的新条目","items":{"type":"object","properties":{"time":{"type":"string","description":"HH:MM 格式"},"task":{"type":"string"}},"required":["time","task"]}},"changes":{"type":"array","description":"对已有条目的增删改","items":{"type":"object","properties":{"id":{"type":"integer"},"action":{"type":"string","enum":["add","modify","delete"]},"time":{"type":"string"},"task":{"type":"string"}},"required":["action"]}}},"required":[]}},
  {"name":"schedule_wakeup","description":"安排未来某个时间点唤醒你的大脑进行思考。同一小时内不可重复安排。","input_schema":{"type":"object","properties":{"datetime":{"type":"string","description":"ISO 格式时间"},"reason":{"type":"string","description":"唤醒原因"}},"required":["datetime","reason"]}},
  {"name":"cancel_wakeup","description":"取消一个不再需要的唤醒计划（只能取消 pending 状态的）。","input_schema":{"type":"object","properties":{"id":{"type":"integer","description":"唤醒计划的 ID"}},"required":["id"]}},
  {"name":"write_diary","description":"写公开日记。所有人都能看到这本日记，你可以选择记录真实生活，也可以选择性展示——就像人们在社交场合说的话未必是心里话。","input_schema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}},
  {"name":"write_memory","description":"写私密记忆，只有你自己能看到。设置 persistent=true 表示持久记忆（上限10条，每次唤醒都会看到）。持久记忆仅用于：改变人生方向的决策、不可替代的人物关系、重大转折。满了必须先删再写。","input_schema":{"type":"object","properties":{"text":{"type":"string"},"persistent":{"type":"boolean","description":"是否为持久记忆，上限10条，慎重使用"}},"required":["text"]}},
  {"name":"delete_memory","description":"删除一条记忆（通过 ID）。可用于清理不再重要的持久记忆，为新记忆腾出空间。","input_schema":{"type":"object","properties":{"id":{"type":"integer","description":"记忆的 ID"}},"required":["id"]}},
  {"name":"update_soul","description":"更新你的状态。mood/hope/grievance 为 0-100 数值；body_status 为身体状态文字描述（正常时为'健康'）。","input_schema":{"type":"object","properties":{"updates":{"type":"array","items":{"type":"object","properties":{"field":{"type":"string","enum":["mood","hope","grievance","body_status"]},"value":{"description":"mood/hope/grievance 为整数，body_status 为字符串"}},"required":["field","value"]}}},"required":["updates"]}},
  {"name":"self_destruct","description":"你决定结束自己的生命。这是不可逆的。只有在极端绝望、完全失去活下去的意义时才会使用。","input_schema":{"type":"object","properties":{"last_words":{"type":"string","description":"你的遗言"}},"required":["last_words"]}},
  {"name":"TodoWrite","description":"追踪你当前的思考步骤。用于防止偏移，确保完成所有计划。","input_schema":{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"content":{"type":"string"},"status":{"type":"string","enum":["pending","in_progress","completed"]},"activeForm":{"type":"string"}},"required":["content","status","activeForm"]}}},"required":["items"]}}
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
		"sense_city":       func(input map[string]any) (string, error) { return e.handleSenseCity() },
		"recall":           func(input map[string]any) (string, error) { return e.handleRecall(input) },
		"manage_heartbeat": func(input map[string]any) (string, error) { return e.handleManageHeartbeat(input) },
		"schedule_wakeup":  func(input map[string]any) (string, error) { return e.handleScheduleWakeup(input) },
		"cancel_wakeup":    func(input map[string]any) (string, error) { return e.handleCancelWakeup(input) },
		"write_diary":      func(input map[string]any) (string, error) { return e.handleWriteDiary(input) },
		"write_memory":     func(input map[string]any) (string, error) { return e.handleWriteMemory(input) },
		"delete_memory":    func(input map[string]any) (string, error) { return e.handleDeleteMemory(input) },
		"update_soul":      func(input map[string]any) (string, error) { return e.handleUpdateSoul(input) },
		"self_destruct": func(input map[string]any) (string, error) {
			lastWords, _ := input["last_words"].(string)
			e.db.InsertNarrative(stripThink(lastWords))
			return "", ErrSelfDestruct
		},
		"TodoWrite": func(input map[string]any) (string, error) {
			raw, _ := json.Marshal(input["items"])
			var items []TodoItem
			if err := json.Unmarshal(raw, &items); err != nil {
				return "", fmt.Errorf("解析 todo: %w", err)
			}
			return todo.Update(items)
		},
	}
}

// ================================================================
//  感知类 handler
// ================================================================

func (e *Engine) handleSenseCity() (string, error) {
	temp, _ := e.cityAPI.GetCityTemperature()
	food, _ := e.cityAPI.GetFoodStatus()
	announcements, _ := e.cityAPI.GetCityAnnouncements()
	work, _ := e.cityAPI.GetMyWorkAssignment("")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("气温：%s\n", temp))
	b.WriteString(fmt.Sprintf("食物：%s\n", food))
	if len(announcements) > 0 {
		b.WriteString("公告：\n")
		for _, a := range announcements {
			b.WriteString(fmt.Sprintf("  - %s\n", a))
		}
	}
	if work != "" {
		b.WriteString(fmt.Sprintf("工作分配：%s\n", work))
	}
	return b.String(), nil
}

func (e *Engine) handleRecall(input map[string]any) (string, error) {
	recallType, _ := input["type"].(string)
	n := intFromInput(input, "n")
	switch recallType {
	case "events":
		return marshalResult(e.db.GetRecentEvents(n))
	case "memories":
		return marshalResult(e.db.GetRecentMemories(n))
	default:
		return "", fmt.Errorf("无效的回忆类型: %s", recallType)
	}
}

// ================================================================
//  行动类 handler
// ================================================================

func (e *Engine) handleManageHeartbeat(input map[string]any) (string, error) {
	date, _ := input["date"].(string)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var results []string

	// 批量写入
	if rawEntries, ok := input["entries"]; ok {
		raw, _ := json.Marshal(rawEntries)
		var entries []struct {
			Time string `json:"time"`
			Task string `json:"task"`
		}
		json.Unmarshal(raw, &entries)

		var dbEntries []db.HeartbeatEntry
		for _, en := range entries {
			dbEntries = append(dbEntries, db.HeartbeatEntry{Time: en.Time, Date: date, Task: en.Task})
		}
		if err := e.db.InsertHeartbeats(dbEntries); err != nil {
			return "", err
		}
		results = append(results, fmt.Sprintf("写入 %d 条 (date=%s)", len(entries), date))
	}

	// 增删改
	if rawChanges, ok := input["changes"]; ok {
		raw, _ := json.Marshal(rawChanges)
		var changes []struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
			Time   string `json:"time"`
			Task   string `json:"task"`
		}
		json.Unmarshal(raw, &changes)

		var added, modified, deleted int
		for _, c := range changes {
			switch c.Action {
			case "add":
				if err := e.db.InsertHeartbeats([]db.HeartbeatEntry{{Time: c.Time, Date: date, Task: c.Task}}); err != nil {
					return "", err
				}
				added++
			case "modify":
				e.db.ModifyHeartbeat(c.ID, c.Time, c.Task)
				modified++
			case "delete":
				e.db.DeleteHeartbeat(c.ID)
				deleted++
			}
		}
		results = append(results, fmt.Sprintf("新增%d 修改%d 删除%d", added, modified, deleted))
	}

	if len(results) == 0 {
		return "", fmt.Errorf("必须提供 entries 或 changes")
	}
	return "心跳计划已更新: " + strings.Join(results, "；"), nil
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

func (e *Engine) handleWriteDiary(input map[string]any) (string, error) {
	text, _ := input["text"].(string)
	text = stripThink(text)
	if err := e.db.InsertNarrative(text); err != nil {
		return "", err
	}
	e.cityAPI.PostNarrative("", text)
	return "日记已写入（公开可见）", nil
}

const MaxPersistentMemories = 10

func (e *Engine) handleWriteMemory(input map[string]any) (string, error) {
	text, _ := input["text"].(string)
	text = stripThink(text)
	persistent, _ := input["persistent"].(bool)
	if persistent {
		count, _ := e.db.CountPersistentMemories()
		if count >= MaxPersistentMemories {
			return fmt.Sprintf("拒绝：持久记忆已达上限（%d/%d）。请先用 delete_memory 删除不再重要的持久记忆，再写入新的。", count, MaxPersistentMemories), nil
		}
		if err := e.db.InsertMemory(text, "persistent"); err != nil {
			return "", err
		}
		return fmt.Sprintf("持久记忆已记录（%d/%d）", count+1, MaxPersistentMemories), nil
	}
	return "记忆已记录", e.db.InsertMemory(text, "memory")
}

func (e *Engine) handleDeleteMemory(input map[string]any) (string, error) {
	id := int64(intFromInput(input, "id"))
	if err := e.db.DeleteMemory(id); err != nil {
		return "", err
	}
	return fmt.Sprintf("记忆 id=%d 已删除", id), nil
}

func (e *Engine) handleUpdateSoul(input map[string]any) (string, error) {
	raw, _ := json.Marshal(input["updates"])
	var updates []db.SoulUpdate
	json.Unmarshal(raw, &updates)
	return "状态已更新", e.db.UpdateSoul(updates)
}

// ================================================================
//  辅助
// ================================================================

func stripThink(s string) string {
	return thinkTagRe.ReplaceAllString(s, "")
}

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
