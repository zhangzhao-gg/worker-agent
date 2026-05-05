# Worker Agent

新伦敦城市模拟系统 — 每个工人是一个独立 AI Agent，拥有记忆、情绪、工作节律和自主决策能力。

## 概述

让 LLM 扮演城市工人的自主 Agent 系统。不是聊天机器人，是持续运行的角色：

- **心跳**：身体每 10 分钟机械执行一次任务汇报
- **唤醒**：大脑在关键时刻醒来思考 — 早晨规划、晚间复盘、突发决策
- **记忆**：私人想法持久化，跨会话延续
- **情绪**：mood / hope / grievance 三维度，影响决策，被世界塑造
- **叙事**：对外可见的生活记录，同步到城市日志

## 架构

```
┌─────────────────────────────────────────────────┐
│  Worker Agent (Go)                              │
│  ├── 心跳协程 — 身体，机械执行                    │
│  ├── 唤醒协程 — 大脑，自主思考                    │
│  ├── 推理引擎 — Agent Loop + Tool Dispatch       │
│  └── HTTP API — 创建/管理/事件推送               │
├─────────────────────────────────────────────────┤
│  Dashboard (Node.js)                            │
│  ├── Express API — 直连 SQLite 读写              │
│  └── React UI — 黄皮纸/油墨/撕裂边缘视觉风格      │
├─────────────────────────────────────────────────┤
│  SQLite (WAL)                                   │
│  每个工人一个 .db 文件，7 张表                    │
└─────────────────────────────────────────────────┘
```

## 快速开始

```bash
# 启动 Worker Agent
go build -o worker ./cmd/worker
./worker --port 8080

# 启动 Dashboard
cd dashboard && npm install && npm run dev
# 前端 :4000 + API :3001

# 创建一个工人
curl -X POST http://localhost:8080/api/workers \
  -H 'Content-Type: application/json' \
  -d '{"name":"乔布斯","occupation":"矿工","background":"...","personality":"..."}'
```

## 工人的一天

```
07:30  大脑唤醒 → 规划今天工作，安排心跳计划
08:00  身体开始执行 → 每10分钟一次心跳汇报
12:00  午休 → 身体自动完成
18:00  收工
18:30  大脑唤醒 → 晚间复盘，安排明天
23:00  睡眠 → 等待下一次唤醒
```

## 技术栈

- **Worker**: Go + SQLite (WAL) + MiniMax (OpenAI 兼容 API)
- **Dashboard**: Vite + React + Express + better-sqlite3
- **通信**: Worker 写 DB，Dashboard 直连 DB 读写，完全解耦

## Dashboard 视觉风格

黄皮纸 + 油墨文字 + 撕裂不规则边缘 + 墙面做旧背景，打字机风格 UI。详见 `docs/ink-effect.md`。

## 线上部署

- 域名: https://worker.okethan.top
- 服务器: Ubuntu 24.04，Nginx 反代
- 部署方式: rsync + vite build + systemctl restart

详见 `docs/deploy.md`。

## 文档索引

| 文档 | 内容 |
|------|------|
| `prd.md` | 产品需求文档，整体设计 |
| `docs/business-layer.md` | 业务层技术设计 — 进程生命周期、心跳、唤醒调度 |
| `docs/llm-reasoning-engine.md` | 推理引擎技术设计 — Agent Loop、Tool Dispatch、压缩 |
| `docs/ink-effect.md` | Dashboard 视觉实现 — 黄皮纸、撕裂边缘、油墨效果 |
| `docs/deploy.md` | 部署手册 — rsync、systemd、Nginx 配置 |
| `docs/progress.md` | 开发进度跟踪 |
