# LLM 推理引擎 · 技术设计文档
> Worker Agent 的「大脑」—— 参考 docs/s_full.py 移植的独立推理模块

---

## 1. 定位

推理引擎是一个**纯函数式模块**：业务层注入上下文，引擎内部跑 agent loop，通过 tools 操作数据库和城市 API，跑完即退出。

```
业务层
  │
  │  engine := NewReasoningEngine(db, cityAPI)
  │  engine.Run(trigger, context)
  │
  ▼
┌──────────────────────────────────────────┐
│           LLM 推理引擎                    │
│                                          │
│  systemPrompt ← 业务层注入 soul + context │
│                                          │
│  agentLoop:                              │
│    compression → LLM call → toolDispatch  │
│    ↻ 直到 LLM StopReason != tool_use     │
│                                          │
│  tools: PRD 感知/行动工具                 │
│  todos: 防推理偏移                        │
└──────────────────────────────────────────┘
```

**引擎不拥有任何状态**。db 连接、城市 API client 均由业务层持有并传入。

---

## 2. 模块清单

| 模块 | 来源 | 职责 |
|------|------|------|
| `ReasoningEngine` | 新建 | 入口 struct，持有 db/cityAPI 引用，暴露 `Run()` |
| `agentLoop` | 移植 s_full.py agent_loop | 核心循环：压缩 → LLM 调用 → 工具分发 |
| `toolDispatch` | 移植 s_full.py TOOL_HANDLERS | 工具注册表 + 分发机制 |
| `tools/*` | 新建 | 6 感知 + 6 行动 = 12 个业务工具，加 TodoWrite + compress = 共 14 个 |
| `todos` | 移植 s_full.py TodoManager | 推理过程步骤追踪，防偏移 |
| `compression` | 移植 s_full.py compression | microcompact + autoCompact |
| `systemPrompt` | 新建 | 从注入的 soul 数据动态组装 |

---

## 3. 入口接口

```go
// ReasoningEngine 推理引擎入口
type ReasoningEngine struct {
    db      *Database
    cityAPI *CityAPI
}

func NewReasoningEngine(db *Database, cityAPI *CityAPI) *ReasoningEngine {
    return &ReasoningEngine{db: db, cityAPI: cityAPI}
}

// Run 执行一次完整推理
//
// trigger: 触发类型（仅两种）
//   - "scheduled_wakeup" 所有定时唤醒（早晨/晚间/LLM自行安排），reason 由 context 携带
//   - "urgent_news"      心跳线程紧急唤醒
//
// context 字段说明：
//   - Soul:    soul 表完整数据
//   - Summary: 最近一次摘要
//   - Events:  未处理的 events
//   - Reason:  唤醒原因（仅 scheduled_wakeup 时，来自 wakeup_schedule 表）
//   - News:    触发的 news（仅 urgent_news 时）
func (e *ReasoningEngine) Run(trigger string, ctx RunContext) error {
    // ...
}

type RunContext struct {
    Soul    Soul
    Summary string
    Events  []Event
    Reason  string
    News    string
}
```

业务层调用示例：

```go
// 定时唤醒（早晨/晚间/任意时刻，统一入口）
engine.Run("scheduled_wakeup", RunContext{
    Soul:    db.GetSoul(),
    Summary: db.GetLatestSummary(),
    Events:  db.GetUnprocessedEvents(),
    Reason:  wakeupRow.Reason, // 如 "早晨起床" / "晚间复盘"
})

// 紧急唤醒
engine.Run("urgent_news", RunContext{
    Soul:    db.GetSoul(),
    Summary: db.GetLatestSummary(),
    Events:  db.GetUnprocessedEvents(),
    News:    theUrgentNews,
})
```

---

## 4. System Prompt 组装

根据 trigger 类型和 context 动态组装。核心结构：

```
你是 {name}，{occupation}。
{background}

你的性格：{personality}
你的说话方式：{speech_style}
你的价值观：{values}
你的家人：{family}

当前情绪状态（LLM 直接写入绝对值，无范围限制）：
  心情：{mood}
  希望：{hope}
  不满：{grievance}

你对城市的感知是有限的、模糊的：
  - 你不知道其他工人在想什么（他们的内心对你不可见）
  - 你不知道城市资源的精确数字（只能感受到「紧张/正常/充裕」）
  - 你不知道执政官的内部决策过程

---
当前情境：{trigger 对应的情境描述}
{context 中的 summary / events / news 等}
---

重要规则：
- 每次思考结束前，你必须用 schedule_wakeup 安排至少一个未来唤醒时间
- 早晨起床时，记得安排晚间复盘和明天起床

你可以使用以下工具来感知城市和采取行动。
用 TodoWrite 追踪你的思考步骤，确保完成所有步骤。
```

**关键设计**：soul 的静态字段构成身份，动态字段（mood/hope/grievance）影响语气和决策倾向。不同 trigger 注入不同的情境段落。

---

## 5. Agent Loop

参考 s_full.py 的 `agent_loop` 移植，去掉 background drain、inbox check、REPL 相关逻辑。

```go
func agentLoop(messages []Message, systemPrompt string, tools []Tool, handlers ToolHandlerMap) error {
    for {
        // ── 压缩管线 ──
        microcompact(messages)
        if estimateTokens(messages) > TokenThreshold {
            messages = autoCompact(messages)
        }

        // ── LLM 调用 ──
        resp, err := client.CreateMessage(model, systemPrompt, messages, tools)
        if err != nil {
            return err
        }
        messages = append(messages, resp.AsAssistantMessage())

        if resp.StopReason != "tool_use" {
            return nil // 推理结束
        }

        // ── 工具分发 ──
        var results []ToolResult
        for _, call := range resp.ToolCalls {
            handler, ok := handlers[call.Name]
            if !ok {
                results = append(results, ToolResult{ID: call.ID, Content: "Unknown tool: " + call.Name})
                continue
            }
            output, err := handler(call.Input)
            if err != nil {
                results = append(results, ToolResult{ID: call.ID, Content: "Error: " + err.Error()})
                continue
            }
            results = append(results, ToolResult{ID: call.ID, Content: output})
        }

        // ── todo 偏移检查 ──
        if todoHasOpenItems && roundsWithoutTodo >= 3 {
            // inject <reminder>
        }

        messages = append(messages, resultsAsMessage(results))
    }
}
```

### 与 s_full.py 的差异

| s_full.py | 推理引擎 |
|--------|----------|
| 循环前 drain background notifications | 删除，无后台任务 |
| 循环前 check lead inbox | 删除，无消息总线 |
| REPL 驱动，history 跨轮次 | 单次调用，messages 不跨 Run |
| tools 包含 spawn_teammate 等 | 只有 PRD 12 个业务工具 + TodoWrite + compress |

---

## 6. Tool Dispatch

参考 s_full.py 的注册表 + 分发模式：

```go
// ToolHandler 统一工具处理签名
type ToolHandler func(input map[string]interface{}) (string, error)

// ToolHandlerMap 工具注册表
type ToolHandlerMap map[string]ToolHandler

// 注册示例
var handlers = ToolHandlerMap{
    "tool_name": func(input map[string]interface{}) (string, error) {
        return handlerFunction(input)
    },
    // ...
}

// Tool schema 定义
var tools = []Tool{
    {Name: "...", Description: "...", InputSchema: Schema{...}},
    // ...
}
```

LLM 返回 tool_use → 查 handlers map → 执行 → 返回 ToolResult。机制完全复用。

---

## 7. 工具定义

### 7.1 感知类

| Tool | 参数 | 返回 | 实现 |
|------|------|------|------|
| `get_city_temperature` | 无 | 模糊描述（如「寒冷，锅炉负荷很高」） | cityAPI 调用 |
| `get_food_status` | 无 | 模糊描述（如「配给紧张」） | cityAPI 调用 |
| `get_city_announcements` | 无 | 执政官公告列表 | cityAPI 调用 |
| `get_my_work_assignment` | 无 | 今日任务描述 | cityAPI 调用 |
| `get_recent_events` | `n int` | 最近 n 条 events | db 查询 events 表 |
| `get_memories` | `n int` | 最近 n 条记忆 | db 查询 memories 表 |

### 7.2 行动类

| Tool | 参数 | 作用 | 实现 |
|------|------|------|------|
| `write_heartbeat_schedule` | `entries []HeartbeatEntry{Time, Task}` | 批量写入心跳计划 | db 写 heartbeat_schedule 表 |
| `update_heartbeat_schedule` | `changes []ScheduleChange{ID, Action, ...}` | 增删改心跳计划 | db 改 heartbeat_schedule 表 |
| `schedule_wakeup` | `datetime string, reason string` | 安排未来 LLM 唤醒 | db 写 wakeup_schedule 表 |
| `write_narrative` | `text string` | 写对外叙事 | db 写 narratives 表 + cityAPI 同步到城市日志 |
| `write_memory` | `text string` | 写私人记忆 | db 写 memories 表 |
| `update_soul` | `updates []SoulUpdate{Field, Value}` | 批量修改情绪值（无范围限制） | db 改 soul 表动态字段 |

### 7.3 元工具

| Tool | 作用 |
|------|------|
| `TodoWrite` | 推理步骤追踪，items 含 content/status/activeForm |
| `compress` | 手动触发上下文压缩 |

---

## 8. Tool Schema 定义

Go 侧用 `json.RawMessage` 持有 schema，不为 JSON Schema 建类型系统。Tool 定义直接内嵌 JSON：

```go
type Tool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"input_schema"`
}
```

完整 schema（JSON 原文，代码中以 `json.RawMessage` 嵌入）：

```json
[
  {"name": "get_city_temperature", "description": "感知当前城市温度，返回模糊描述。",
   "input_schema": {"type": "object", "properties": {}}},

  {"name": "get_food_status", "description": "感知食物供给状态，返回模糊描述。",
   "input_schema": {"type": "object", "properties": {}}},

  {"name": "get_city_announcements", "description": "获取执政官公告列表。",
   "input_schema": {"type": "object", "properties": {}}},

  {"name": "get_my_work_assignment", "description": "获取今天城市分配给你的工作任务。",
   "input_schema": {"type": "object", "properties": {}}},

  {"name": "get_recent_events", "description": "回忆最近发生的事件。",
   "input_schema": {"type": "object", "properties": {"n": {"type": "integer", "description": "返回条数"}}, "required": ["n"]}},

  {"name": "get_memories", "description": "回忆自己之前的想法和记忆。",
   "input_schema": {"type": "object", "properties": {"n": {"type": "integer", "description": "返回条数"}}, "required": ["n"]}},

  {"name": "write_heartbeat_schedule", "description": "批量写入今天的心跳计划。每条含时间和任务描述。",
   "input_schema": {"type": "object", "properties": {"entries": {"type": "array", "items": {"type": "object", "properties": {"time": {"type": "string", "description": "HH:MM 格式"}, "task": {"type": "string", "description": "心跳任务描述"}}, "required": ["time", "task"]}}}, "required": ["entries"]}},

  {"name": "update_heartbeat_schedule", "description": "增删改心跳计划中的条目。",
   "input_schema": {"type": "object", "properties": {"changes": {"type": "array", "items": {"type": "object", "properties": {"id": {"type": "integer", "description": "计划条目 ID"}, "action": {"type": "string", "enum": ["add", "modify", "delete"]}, "time": {"type": "string"}, "task": {"type": "string"}}, "required": ["action"]}}}, "required": ["changes"]}},

  {"name": "schedule_wakeup", "description": "安排未来某个时间点唤醒你的大脑进行思考。可多次调用安排多个时间点。",
   "input_schema": {"type": "object", "properties": {"datetime": {"type": "string", "description": "ISO 格式时间"}, "reason": {"type": "string", "description": "唤醒原因"}}, "required": ["datetime", "reason"]}},

  {"name": "write_narrative", "description": "写下你的对外叙事，会被同步到城市日志，其他人可以看到。",
   "input_schema": {"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"]}},

  {"name": "write_memory", "description": "写下私人记忆，只有你自己能看到。",
   "input_schema": {"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"]}},

  {"name": "update_soul", "description": "批量更新你的情绪状态。只能修改 mood、hope、grievance。值无范围限制，由你自主决定。",
   "input_schema": {"type": "object", "properties": {"updates": {"type": "array", "items": {"type": "object", "properties": {"field": {"type": "string", "enum": ["mood", "hope", "grievance"]}, "value": {"type": "integer", "description": "情绪值，无范围限制"}}, "required": ["field", "value"]}}}, "required": ["updates"]}},

  {"name": "TodoWrite", "description": "追踪你当前的思考步骤。用于防止偏移，确保完成所有计划。",
   "input_schema": {"type": "object", "properties": {"items": {"type": "array", "items": {"type": "object", "properties": {"content": {"type": "string"}, "status": {"type": "string", "enum": ["pending", "in_progress", "completed"]}, "activeForm": {"type": "string"}}, "required": ["content", "status", "activeForm"]}}}, "required": ["items"]}},

  {"name": "compress", "description": "手动触发上下文压缩，当对话过长时使用。",
   "input_schema": {"type": "object", "properties": {}}}
]
```

---

## 9. Todos 机制

参考 s_full.py 的 `TodoManager` 移植：

- LLM 通过 `TodoWrite` 工具写入步骤清单
- 每个 item 有 `content`、`status`（pending/in_progress/completed）、`activeForm`
- 最多 20 条，同时只能有 1 条 `in_progress`
- agentLoop 中：连续 3 轮未更新 todo 且存在未完成项 → 注入 `<reminder>` 提醒

**作用**：工人的 LLM 在复杂推理（如早晨制定全天计划）时，用 todo 锚定步骤，防止遗漏或发散。

---

## 10. 压缩机制

### 10.1 microcompact

每次 LLM 调用前执行。清理早期 ToolResult 中超过 100 字符的内容，只保留最近 3 条完整结果。

```go
func microcompact(messages []Message) {
    // 找到所有 ToolResult 块
    // 保留最近 3 条完整
    // 早期的截断为 "[cleared]"
}
```

### 10.2 autoCompact

当 `estimateTokens(messages) > TokenThreshold` 时自动触发：
1. 调用 LLM 生成摘要
2. 用摘要替换整个 messages 数组
3. （可选）若开启 debug 模式，保存完整对话到 `.transcripts/` 目录，用于回溯工人思考过程

也可通过 `compress` 工具手动触发。

### 10.3 TokenThreshold

默认 100000。工人单次推理通常不会达到这个阈值，但紧急唤醒后连续多轮 tool 调用可能逼近。

---

## 11. 数据流总览

```
业务层 ──trigger+context──▶ ReasoningEngine.Run()
                                │
                                ▼
                        组装 systemPrompt
                                │
                                ▼
                          agentLoop 开始
                                │
                    ┌───────────┴───────────┐
                    ▼                       ▼
              compression              LLM 调用
                    │                       │
                    │                       ▼
                    │               toolDispatch
                    │              ┌────┴────┐
                    │              ▼         ▼
                    │          感知 tools  行动 tools
                    │           │           │
                    │           ▼           ▼
                    │       cityAPI        db
                    │                       │
                    └───────────┬───────────┘
                                │
                                ▼
                        StopReason == end_turn
                                │
                                ▼
                            引擎退出
                        业务层检查 db 变更
```

---

## 12. 错误处理

| 场景 | 处理 |
|------|------|
| tool handler 返回 error | 捕获，返回 `"Error: " + err.Error()` 作为 ToolResult |
| LLM 调用工具名不存在 | 返回 `"Unknown tool: " + name` |
| LLM 无限循环（超过 N 轮） | agentLoop 设置最大轮次（默认 30），超出强制退出 |
| 压缩失败 | 降级：跳过压缩，继续推理 |
| cityAPI 超时 | tool handler 返回错误描述，LLM 自行判断下一步 |

---

## 13. 配置项

| 配置 | 默认值 | 说明 |
|------|--------|------|
| `Model` | 环境变量 `MODEL_ID` | LLM 模型 |
| `TokenThreshold` | 100000 | 自动压缩阈值 |
| `MaxAgentRounds` | 30 | agentLoop 最大轮次 |
| `MaxToolOutput` | 50000 | ToolResult 最大字符数 |
