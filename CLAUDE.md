# worker-agent - 新伦敦城市模拟系统 · Worker Agent
Go + SQLite + MiniMax (OpenAI 兼容 API)

<directory>
cmd/worker/      - Worker 进程入口，纯 API + 协程调度，无 UI
internal/db/     - 数据层，SQLite CRUD，7 张表（含 reasoning_logs），单 struct 全部方法
internal/city/   - 城市 API 层，HTTP client + mock 模式，工人与外部世界的唯一接口
internal/llm/    - LLM 抽象层，Client 接口 + MiniMax 实现
internal/engine/ - LLM 推理引擎，agent loop + tool dispatch + 压缩 + todo + 推理日志
internal/worker/ - 双协程，心跳协程（身体）+ 唤醒调度协程（大脑入口）+ 紧急判断
internal/server/ - 纯 HTTP API + 工人生命周期管理 + 事件推送端点，DB 持久化重启自动恢复
dashboard/       - Node.js 独立 Dashboard（Vite + React + Express + better-sqlite3），直连 data/*.db 读写，安全码保护写操作
docs/            - 设计文档，PRD + 业务层 + 推理引擎 + 部署手册 + s_full.py 参考实现
</directory>

<config>
go.mod - Go 模块定义，依赖 go-sqlite3
dashboard/package.json - Dashboard Node 依赖
</config>

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
