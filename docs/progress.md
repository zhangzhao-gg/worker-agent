# Worker Agent 开发进度

## 项目状态：可运行 ✔

系统已跑通完整链路：创建工人 → LLM 思考 → 感知城市 → 制定计划 → 写叙事/记忆 → 更新情绪 → 安排下次唤醒。

---

## 已完成

### 核心架构
- **双进程分离**：Worker 进程（:8080 纯 API + 协程）+ Dashboard 进程（:8081 只读 UI）
- **双协程模型**：心跳协程（身体）+ 唤醒调度协程（大脑入口）
- **SQLite WAL 模式**：7 张表（含 reasoning_logs），支持多进程并发读写
- **LLM Agent Loop**：最多 30 轮推理，含 microcompact + autoCompact 压缩管线
- **16 个工具**：6 感知 + 8 行动（含 cancel_wakeup / self_destruct）+ 2 元工具
- **TodoManager**：防止 LLM 推理偏移，3 轮未更新自动提醒

### 唤醒/心跳制度
- **工作制度**：朝八晚六（08:00-18:00），每 10 分钟心跳汇报
- **唤醒规则**：一天最多 2-3 次（起床规划/晚间复盘/突发事件），LLM 在 prompt 中被明确约束
- **心跳规则**：工作时间内的任务安排，身体自动执行，不需要 LLM 推理
- **cancel_wakeup**：LLM 可自主清理冗余唤醒
- **self_destruct**：LLM 在极端绝望时可选择自我终结，协程自动退出
- **唤醒上下文**：推理时注入过去3天+未来3天的唤醒记录 + 今日工作分配
- **重启审视**：每次重启无条件触发一次 LLM 推理，审视计划合理性

### 接入层
- **MiniMax M2.7**：OpenAI 兼容 API，base URL `https://api.minimaxi.com`
- **城市 API Mock 模式**：Frostpunk 风格测试数据，按天固定（同日内一致）
- **Mock News**：1% 概率触发，避免心跳频繁产生事件
- **自动读取 .env**：内置轻量 loadEnv，无需第三方依赖

### HTTP 服务
- `POST /api/workers` — 创建工人（含 soul 人设 + avatar）
- `GET /api/workers` — 列出所有工人
- `GET /api/workers/{name}` — 查询单个工人详情
- `PUT /api/workers/{name}` — 修改工人人设
- `POST /api/workers/{name}/wakeup` — 手动唤醒工人
- `DELETE /api/workers/{name}` — 停止并删除工人
- **CORS 支持**：Dashboard 跨域调用 Worker API
- **重启自动恢复**：扫描 data/*.db，恢复所有工人协程 + 强制审视唤醒

### Web UI（独立 Dashboard 进程）
- **首页**：工人卡片列表，显示名字/职业/状态/三维情绪条
- **详情页**：左栏身份+情绪+背景，右栏六个标签：
  - NARRATIVE LOG — 对外叙事时间线
  - PRIVATE MEMORY — 私人记忆
  - CITY EVENTS — 收到的城市事件
  - WORK SCHEDULE — 心跳工作计划表
  - WAKEUP SCHEDULE — 唤醒计划表
  - REASONING — LLM 推理全链路日志（input/llm_text/tool_call/tool_result/finish）
- **手动唤醒**：输入框 + 按钮，直接从 UI 触发 LLM 推理
- **人设编辑**：左栏可直接编辑 occupation/background/personality/speech_style/values/family
- **长连接池**：只读打开 DB，状态从 pending wakeup 推断
- **头像支持**：静态文件服务 `/static/avatars/`
- 风格：维多利亚蒸汽朋克，Newsreader 字体，复古墨水色调

### 健壮性
- **唤醒失败重试**：LLM 调用失败时 wakeup 保留 pending，下次扫描自动重试
- **防消失兜底**：LLM 忘记安排 wakeup 时，自动补插次日早晨唤醒
- **心跳 nil 保护**：未配置 LLM 时紧急判断跳过，不 panic
- **`<think>` 过滤**：MiniMax 思维链标签不写入 DB，日志保留原样打印
- **时间格式兼容**：InsertWakeup 同时支持 RFC3339 和无时区 ISO 格式
- **LLM key 可选**：未配置时服务正常启动，仅推理功能不可用
- **全链路 ASCII 日志**：每轮推理清晰展示工具调用、参数、结果
- **推理日志持久化**：reasoning_logs 表记录每次推理全链路，Dashboard REASONING 标签页实时展示

---

## 已验证场景

| 场景 | 状态 |
|------|------|
| 创建工人并触发首次思考 | ✔ |
| LLM 感知城市状态（温度/食物/公告/工作） | ✔ |
| LLM 制定心跳计划（write_heartbeat_schedule） | ✔ |
| LLM 增删改心跳计划（update_heartbeat_schedule） | ✔ |
| LLM 安排唤醒（schedule_wakeup） | ✔ |
| LLM 取消唤醒（cancel_wakeup） | ✔ |
| LLM 自我终结（self_destruct） | ✔ |
| LLM 写叙事和记忆 | ✔ |
| LLM 更新情绪（update_soul） | ✔ |
| 心跳协程自动执行计划并收取 news | ✔ |
| 重启后自动恢复 + 触发审视唤醒 | ✔ |
| Dashboard 独立进程展示所有数据 | ✔ |
| Dashboard 手动唤醒 + 人设编辑 | ✔ |
| LLM 调用失败后 wakeup 重试 | ✔ |
| 手动唤醒 API | ✔ |
| 修改人设 API | ✔ |
| 停止/删除工人 API | ✔ |
| 推理日志写入 DB + UI 展示 | ✔ |
| UI 自动刷新 | ✔ |

---

## 待完成

### 功能
- [ ] 城市 API 真实 HTTP 对接（目前仅 mock）

### 优化
- [x] Web UI 自动刷新（JS polling，reasoning 5s，其他 30s）
- [ ] 多工人并发时的 LLM 调用限流
- [ ] 压缩管线实战验证（长对话场景）

### 部署
- [ ] Dockerfile
- [ ] 生产环境配置指南

---

## 运行方式

```bash
# 终端 1: Worker 引擎（API :8080）
go run ./cmd/worker/

# 终端 2: Dashboard UI（:8081）
go run ./cmd/dashboard/

# 创建工人
curl -X POST http://localhost:8080/api/workers \
  -H "Content-Type: application/json" \
  -d '{"name":"乔布斯","occupation":"矿工","background":"...","personality":"...","speech_style":"...","values_desc":"...","family":"..."}'

# 手动唤醒
curl -X POST http://localhost:8080/api/workers/乔布斯/wakeup \
  -d '{"reason":"紧急通知"}'

# 修改人设
curl -X PUT http://localhost:8080/api/workers/乔布斯 \
  -d '{"occupation":"锅炉工"}'

# 停止工人
curl -X DELETE http://localhost:8080/api/workers/乔布斯

# 打开 UI
open http://localhost:8081
```

---

*最后更新：2026-04-30*
