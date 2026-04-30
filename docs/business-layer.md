# 业务层 · 技术设计文档
> Worker Agent 的「身体」—— 进程生命周期、心跳协程、唤醒调度、数据层

---

## 1. 定位

业务层是工人进程的主体。它管理工人的「肉身」运作：心跳、调度、数据存储、城市通信。当需要「思考」时，调用 LLM 推理引擎。

```
┌─────────────────────────────────────────────────┐
│                 Worker 进程                       │
│                                                  │
│  main() 启动                                     │
│    │                                             │
│    ├── 初始化 SQLite                              │
│    ├── 初始化 城市 API client                     │
│    ├── 启动 心跳协程（goroutine 1）                 │
│    └── 启动 唤醒调度协程（goroutine 2 / LLM 协程）  │
│                                                  │
│  心跳协程                    唤醒调度协程           │
│  ┌─────────────┐           ┌──────────────────┐  │
│  │ 每分钟扫描   │           │ 每分钟扫描         │  │
│  │ heartbeat_  │           │ wakeup_schedule  │  │
│  │ schedule    │           │                  │  │
│  │      │      │           │ 到时间 → 组装     │  │
│  │      ▼      │           │ context → 调用   │  │
│  │ 发心跳      │           │ ReasoningEngine  │  │
│  │ 处理 news   │  ──紧急──▶│                  │  │
│  │ 标记状态    │           │                  │  │
│  └─────────────┘           └──────────────────┘  │
└─────────────────────────────────────────────────┘
```

---

## 2. 进程生命周期

```go
func main() {
    dbPath := os.Args[1]
    cityAPIURL := os.Args[2]

    // 1. 初始化
    db := NewDatabase(dbPath)
    cityAPI := NewCityAPI(cityAPIURL)
    engine := NewReasoningEngine(db, cityAPI)

    // 2. 唤醒通道
    wakeupCh := make(chan WakeupSignal, 16)

    // 3. 启动双协程
    var wg sync.WaitGroup
    wg.Add(2)
    go RunHeartbeat(db, cityAPI, engine, wakeupCh, &wg)
    go RunWakeup(db, cityAPI, engine, wakeupCh, &wg)

    // 4. 阻塞等待（进程由外部管理终止）
    wg.Wait()
}
```

**一个进程 = 一个工人**。进程启动参数：SQLite 文件路径 + 城市 API 地址。

---

## 3. 心跳协程

心跳协程是工人的「身体」——机械、规律、不思考。

### 3.1 主循环

```
每 60 秒:
    1. 扫描 heartbeat_schedule 表
       WHERE status = 'pending' AND time <= now

    2. 对每条到期计划:
       a. 向城市发送心跳请求
       b. 解析响应中的 news
       c. 若 news 非空:
          - 写入 events 表
          - 调用紧急判断接口
          - 若紧急 → 唤醒 LLM 协程
       d. 标记该计划为 done

    3. 跳过过期未执行的计划 → 标记为 skipped
```

### 3.2 心跳请求

心跳本身代表「工人正在工作」。城市服务器收到心跳后，查询该工人的 session，获知当前工作内容。

```go
func (h *HeartbeatRunner) sendHeartbeat(entry ScheduleEntry) {
    resp := h.cityAPI.Heartbeat(h.workerID)
    if resp.News != "" {
        h.db.InsertEvent(resp.News)
        soul := h.db.GetSoul()
        if h.checkUrgency(resp.News, soul) {
            h.wakeupCh <- WakeupSignal{Trigger: "urgent_news", News: resp.News}
        }
    }
    h.db.UpdateScheduleStatus(entry.ID, "done")
}
```

### 3.3 紧急判断

工人**自己判断**这条 news 对自己是否紧急。复用同一个 LLM 实例（简短 prompt，单轮调用），传入 news + soul 摘要，返回 bool。

不同工人对同一条 news 的紧急程度判断不同：矿工听到「矿井塌方」是紧急的，厨师可能无所谓。

```go
// CheckUrgency 调用 LLM 判断 news 对该工人是否紧急
func checkUrgency(llmClient llm.Client, news string, soul Soul) bool {
    prompt := fmt.Sprintf("你是%s（%s）。以下消息对你来说需要立刻停下手头工作去思考吗？只回答 yes 或 no。\n消息：%s", soul.Name, soul.Occupation, news)
    resp := llmClient.Chat("你是一个判断助手，只回答 yes 或 no。", ...)
    return strings.TrimSpace(resp) == "yes"
}
```

### 3.4 唤醒 LLM

心跳协程发现紧急 news 时，需要唤醒 LLM 协程。通过 channel 触发：

```go
// 心跳协程 → LLM 协程
h.wakeupCh <- WakeupSignal{
    Trigger: "urgent_news",
    News:    news,
}
```

---

## 4. 唤醒调度协程（LLM 协程）

LLM 协程是工人的「大脑入口」——只在关键时刻醒来。

### 4.1 主循环

用 Go select 三路复用：紧急 channel、定时扫描、退出信号。

```go
func RunWakeup(db *Database, cityAPI *CityAPI, engine *ReasoningEngine, wakeupCh <-chan WakeupSignal, wg *sync.WaitGroup) {
    defer wg.Done()
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case signal := <-wakeupCh:
            // 心跳协程发来紧急唤醒，最高优先级
            handleWakeup(db, engine, signal.Trigger, map[string]any{"news": signal.News})

        case <-ticker.C:
            // 每 60 秒扫描 wakeup_schedule 表
            now := time.Now().Format(time.RFC3339)
            for _, entry := range db.GetPendingWakeups(now) {
                handleWakeup(db, engine, "scheduled_wakeup", map[string]any{"reason": entry.Reason})
                db.MarkWakeupDone(entry.ID)
            }

        case <-ctx.Done():
            // 进程退出信号
            return
        }
    }
}
```

### 4.2 处理唤醒

无论是定时唤醒还是紧急唤醒，处理流程一致：

```go
func handleWakeup(db *Database, engine *ReasoningEngine, trigger string, extra map[string]any) {
    // 1. 从 db 组装 context
    ctx := map[string]any{
        "soul":    db.GetSoul(),
        "summary": db.GetLatestSummary(),
        "events":  db.GetUnprocessedEvents(),
    }
    for k, v := range extra {
        ctx[k] = v
    }

    // 2. 调用推理引擎
    engine.Run(trigger, ctx)

    // 3. 标记本轮处理过的 events
    db.MarkEventsProcessed()

    // 4. 防消失兜底
    // LLM 不可靠——如果它忘了安排下一次 wakeup，wakeup_schedule 就空了，
    // 唤醒调度协程扫不到任何 pending 条目，工人的大脑永远不会再醒来。
    // 所以每次推理退出后检查：如果未来没有任何 pending wakeup，自动补一条次日早晨唤醒。
    if !db.HasPendingWakeups() {
        db.InsertWakeup(tomorrowMorning(), "兜底唤醒")
    }
}
```

### 4.3 唤醒类型

仅两种 trigger，业务层不区分早晨/晚间/任意唤醒：

| trigger | 触发来源 | context 特殊字段 |
|---------|----------|-----------------|
| `scheduled_wakeup` | wakeup_schedule 表到期 | `reason`（来自表中 reason 字段） |
| `urgent_news` | 心跳协程通过 channel 传入 | `news` |

LLM 读 reason（如 "早晨起床" / "晚间复盘"）自行判断该做什么，业务层不预设逻辑。

---

## 5. SQLite 数据层

### 5.1 DDL

```sql
-- 灵魂：单行表，工人的全部人格
CREATE TABLE soul (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    -- 静态字段
    name        TEXT NOT NULL,
    occupation  TEXT NOT NULL,
    background  TEXT,
    personality TEXT,
    speech_style TEXT,
    values_desc TEXT,           -- PRD 中为 values，因 SQL 关键字冲突改名
    family      TEXT,
    -- 动态字段（LLM 可修改）
    mood        INTEGER DEFAULT 50,
    hope        INTEGER DEFAULT 50,
    grievance   INTEGER DEFAULT 0
);

-- 心跳计划
CREATE TABLE heartbeat_schedule (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    time    TEXT NOT NULL,           -- HH:MM 格式
    date    TEXT NOT NULL,           -- YYYY-MM-DD
    task    TEXT NOT NULL,           -- 任务描述
    status  TEXT DEFAULT 'pending'   -- pending / done / skipped
);

-- LLM 唤醒计划
CREATE TABLE wakeup_schedule (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    datetime TEXT NOT NULL,          -- ISO 格式
    reason   TEXT,                   -- 唤醒原因
    status   TEXT DEFAULT 'pending'  -- pending / done
);

-- 事件记录（心跳收到的 news）
CREATE TABLE events (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    content   TEXT NOT NULL,
    processed INTEGER DEFAULT 0      -- 0=未处理 1=已处理
);

-- 私人记忆
CREATE TABLE memories (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    content   TEXT NOT NULL,
    type      TEXT DEFAULT 'memory'  -- memory / summary
);

-- 对外叙事
CREATE TABLE narratives (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    content   TEXT NOT NULL
);
```

### 5.2 Database 结构体

单 struct 挂全部方法，不抽 interface。数据层是薄 CRUD 壳，项目复杂度在推理引擎不在这里。后续若需 mock 测试，再按消费方加 interface，Go 隐式接口零重构成本。

```go
type Database struct {
    db *sql.DB
}

// NewDatabase 打开或创建 SQLite 文件，执行建表
func NewDatabase(dbPath string) *Database

// ── soul ──
func (d *Database) GetSoul() Soul
func (d *Database) UpdateSoul(updates []SoulUpdate) // SoulUpdate{Field: "mood", Value: 35}

// ── heartbeat_schedule ──
func (d *Database) GetPendingHeartbeats(now string) []HeartbeatEntry
func (d *Database) InsertHeartbeats(entries []HeartbeatEntry)
func (d *Database) UpdateHeartbeatStatus(id int64, status string)

// ── wakeup_schedule ──
func (d *Database) GetPendingWakeups(now string) []WakeupEntry
func (d *Database) InsertWakeup(datetime string, reason string)
func (d *Database) MarkWakeupDone(id int64)

// ── events ──
func (d *Database) InsertEvent(content string)
func (d *Database) GetUnprocessedEvents() []Event
func (d *Database) GetRecentEvents(n int) []Event

// ── memories ──
func (d *Database) InsertMemory(content string, memType string) // memType: "memory" | "summary"
func (d *Database) GetRecentMemories(n int) []Memory
func (d *Database) GetLatestSummary() string

// ── narratives ──
func (d *Database) InsertNarrative(content string)

// ── 辅助 ──
func (d *Database) MarkEventsProcessed()
func (d *Database) HasPendingWakeups() bool
```

### 5.3 协程安全

SQLite 在 WAL 模式下支持单写多读。两个 goroutine 共享同一个连接需要加锁，或者各自持有独立连接。

推荐方案：**每个 goroutine 独立连接，开启 WAL 模式**。

```go
db, _ := sql.Open("sqlite3", dbPath)
db.Exec("PRAGMA journal_mode=WAL")
```

---

## 6. 城市 API 层

城市 API 是工人与外部世界的唯一接口。此处只定义接口约定，具体协议待补充。

> 工人的坐标由城市侧统一管理。心跳响应中的 news 已经过城市侧空间匹配（事件坐标 + 辐射半径 vs 工人坐标），worker-agent 无需处理空间逻辑。

```go
// CityAPI 封装与城市服务器的全部 HTTP 通信
type CityAPI struct {
    baseURL string
    client  *http.Client
}

// NewCityAPI 初始化 HTTP client
func NewCityAPI(baseURL string) *CityAPI

// ── 心跳 ──
func (c *CityAPI) Heartbeat(workerID string) HeartbeatResponse
// HeartbeatResponse.News: string（空串表示无新闻）

// ── 叙事同步 ──
func (c *CityAPI) PostNarrative(text string)
// write_narrative tool 写完 db 后调用，同步到城市日志

// ── 感知接口（供推理引擎 tools 调用）──
func (c *CityAPI) GetCityTemperature() string
func (c *CityAPI) GetFoodStatus() string
func (c *CityAPI) GetCityAnnouncements() []string
func (c *CityAPI) GetMyWorkAssignment() string
```

> 城市 API 的具体 HTTP 协议待补充。

---

## 7. 一天的生命流程（数据视角）

```
08:00  wakeup_schedule 触发 morning_wakeup
       │
       ▼
       LLM 推理引擎启动
       ├── get_my_work_assignment() → 城市 API
       ├── get_memories(3) → db.memories
       ├── write_heartbeat_schedule([
       │     {time: "09:00", task: "采矿"},
       │     {time: "09:10", task: "采矿"},
       │     ...
       │     {time: "17:00", task: "收工"}
       │   ]) → db.heartbeat_schedule
       ├── schedule_wakeup("20:00", "晚间复盘") → db.wakeup_schedule
       ├── write_narrative("新的一天开始了...") → db.narratives
       └── update_soul("mood", 60) → db.soul
       引擎退出

09:00  心跳协程扫描到 pending 条目
       ├── 发心跳 → 城市 API
       ├── 收到 news: "" → 跳过
       └── 标记 done

09:10  心跳协程扫描
       ├── 发心跳 → 城市 API
       ├── 收到 news: "矿井塌方，3人受伤"
       ├── 写入 events 表
       ├── 调用紧急判断 → true
       └── 唤醒 LLM

       LLM 推理引擎启动 (urgent_news)
       ├── 读取 events / soul
       ├── update_heartbeat_schedule(暂停采矿，加入救援)
       ├── write_narrative("听到矿井塌方的消息...")
       ├── write_memory("今天矿井出事了，很担心...")
       └── update_soul("mood", 25)
       引擎退出

20:00  wakeup_schedule 触发 evening_review
       LLM 推理引擎启动
       ├── get_recent_events(10)
       ├── write_memory("今日总结：...", type="summary")
       ├── schedule_wakeup("明天08:00", "早晨起床")
       ├── write_narrative("漫长的一天结束了...")
       └── update_soul([{mood: 35}, {hope: 40}, {grievance: 60}])
       引擎退出
```

---

## 8. 协程间通信

```
心跳协程                          LLM 协程
    │                                │
    │   wakeupCh (chan WakeupSignal) │
    │ ──────────────────────────▶    │
    │   WakeupSignal{                │
    │     Trigger: "urgent_news",    │
    │     News: "..."}               │
    │                                │
    │   共享 SQLite（WAL 模式）        │
    │ ◀────────────────────────────▶ │
```

只有一个方向的主动通信：心跳协程 → LLM 协程（紧急唤醒）。其余协作通过 SQLite 间接完成。

---

## 9. 初始化

工人进程首次启动时，需要初始化 soul 表和首次 wakeup_schedule。

```go
// SoulConfig 外部传入的工人配置
type SoulConfig struct {
    Name        string `json:"name"`        // "Edmund Ashby"
    Occupation  string `json:"occupation"`  // "矿工"
    Background  string `json:"background"`
    Personality string `json:"personality"`
    // ...
}

func initializeWorker(db *Database, cfg SoulConfig) {
    // 1. 建表
    db.CreateTables()

    // 2. 写入 soul
    db.InitSoul(cfg)

    // 3. 写入首次唤醒时间（明早 08:00）
    db.InsertWakeup(tomorrowMorning(), "首次起床")
}
```

---

## 10. 异常兜底

PRD 要求：Go 代码层面验证时间合理性，防止 LLM 口误导致工人「消失」。

| 场景 | 兜底策略 |
|------|----------|
| LLM 未安排任何 wakeup | 推理引擎退出后，业务层检查 wakeup_schedule 是否有 pending 条目。若无，自动插入次日 08:00 |
| LLM 安排的时间在过去 | 写入 wakeup_schedule 前校验，拒绝过去的时间 |
| 心跳计划全部为空 | 不干预，工人可以选择「今天不工作」，但会被城市感知到 |
| 进程异常退出 | 外部进程管理器负责重启，SQLite 数据不丢失 |
