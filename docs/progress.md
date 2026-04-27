# Worker Agent 开发进度

## 项目状态：可运行 ✔

系统已跑通完整链路：创建工人 → LLM 思考 → 感知城市 → 制定计划 → 写叙事/记忆 → 更新情绪 → 安排下次唤醒。

---

## 已完成

### 核心架构
- **双协程模型**：心跳协程（身体）+ 唤醒调度协程（大脑入口）
- **SQLite WAL 模式**：6 张表（soul / heartbeat_schedule / wakeup_schedule / events / memories / narratives）
- **LLM Agent Loop**：最多 30 轮推理，含 microcompact + autoCompact 压缩管线
- **14 个工具**：6 感知 + 6 行动 + 2 元工具（TodoWrite / compress）
- **TodoManager**：防止 LLM 推理偏移，3 轮未更新自动提醒

### 接入层
- **MiniMax M2.7**：OpenAI 兼容 API，base URL `https://api.minimaxi.com`
- **城市 API Mock 模式**：Frostpunk 风格测试数据（温度/食物/公告/工作/新闻）
- **自动读取 .env**：内置轻量 loadEnv，无需第三方依赖

### HTTP 服务
- `POST /api/workers` — 创建工人（含 soul 人设 + avatar）
- `GET /api/workers` — 列出所有工人
- `GET /api/workers/{name}` — 查询单个工人详情
- **重启自动恢复**：扫描 data/*.db，恢复所有工人协程
- **Resume 补插唤醒**：无 pending wakeup 时自动插入，防止大脑永睡

### Web UI
- **首页**：工人卡片列表，显示名字/职业/状态/三维情绪条
- **详情页**：左栏身份+情绪+背景，右栏五个标签：
  - NARRATIVE LOG — 对外叙事时间线
  - PRIVATE MEMORY — 私人记忆
  - CITY EVENTS — 收到的城市事件
  - WORK SCHEDULE — 心跳工作计划表
  - WAKEUP SCHEDULE — 唤醒计划表
- **头像支持**：创建时传 avatar 路径，静态文件服务 `/static/avatars/`
- 风格：维多利亚蒸汽朋克，Newsreader 字体，复古墨水色调

### 健壮性
- **唤醒失败重试**：LLM 调用失败时 wakeup 保留 pending，下次扫描自动重试
- **防消失兜底**：LLM 忘记安排 wakeup 时，自动补插次日早晨唤醒
- **时间格式兼容**：InsertWakeup 同时支持 RFC3339 和无时区 ISO 格式
- **LLM key 可选**：未配置时服务正常启动，仅推理功能不可用
- **全链路 ASCII 日志**：每轮推理清晰展示工具调用、参数、结果

---

## 已验证场景

| 场景 | 状态 |
|------|------|
| 创建工人并触发首次思考 | ✔ |
| LLM 感知城市状态（温度/食物/公告/工作） | ✔ |
| LLM 制定心跳计划（write_heartbeat_schedule） | ✔ |
| LLM 安排唤醒（schedule_wakeup） | ✔ |
| LLM 写叙事和记忆 | ✔ |
| LLM 更新情绪（update_soul） | ✔ |
| 心跳协程自动执行计划并收取 news | ✔ |
| 重启后自动恢复工人 | ✔ |
| Web UI 展示所有数据 | ✔ |
| LLM 调用失败后 wakeup 重试 | ✔ |

---

## 待完成

### 功能
- [ ] 手动唤醒 API（`POST /api/workers/{name}/wakeup`）
- [ ] 修改人设 API（`PUT /api/workers/{name}`）
- [ ] 城市 API 真实 HTTP 对接（目前仅 mock）
- [ ] `update_heartbeat_schedule` 工具实现（目前是 TODO stub）
- [ ] 心跳协程收到 news 后的紧急判断需要 LLM 调用，nil client 时应跳过

### 优化
- [ ] LLM 回复中 `<think>` 标签过滤（MiniMax M2.7 思维链输出）
- [ ] Web UI 自动刷新（当前需手动刷新）
- [ ] 工人停止/删除 API
- [ ] 多工人并发时的 LLM 调用限流
- [ ] 压缩管线实战验证（长对话场景）

### 部署
- [ ] Dockerfile
- [ ] 生产环境配置指南

---

## 运行方式

```bash
# 启动（自动读取 .env 中的 LLM_API_KEY）
go run ./cmd/worker/

# 创建工人
curl -X POST http://localhost:8080/api/workers \
  -H "Content-Type: application/json" \
  -d '{"name":"乔布斯","occupation":"矿工","background":"..."}'

# 打开 UI
open http://localhost:8080
```

---

*最后更新：2026-04-27*
