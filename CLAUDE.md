# worker-agent - 新伦敦城市模拟系统 · Worker Agent
Go + SQLite + MiniMax (OpenAI 兼容 API)

<directory>
cmd/worker/      - Worker 进程入口，纯 API + 协程调度，无 UI
cmd/dashboard/   - Dashboard 进程入口，只读扫描 data/*.db 渲染 Web UI
internal/db/     - 数据层，SQLite CRUD，6 张表，单 struct 全部方法
internal/city/   - 城市 API 层，HTTP client + mock 模式，工人与外部世界的唯一接口
internal/llm/    - LLM 抽象层，Client 接口 + MiniMax 实现
internal/engine/ - LLM 推理引擎，agent loop + tool dispatch + 压缩 + todo
internal/worker/ - 双协程，心跳协程（身体）+ 唤醒调度协程（大脑入口）
internal/server/ - 纯 HTTP API + 工人生命周期管理，DB 持久化重启自动恢复
internal/web/    - Web UI，embed 内嵌模板，工人列表 + 详情档案页
docs/            - 设计文档，PRD + 业务层 + 推理引擎 + s_full.py 参考实现
</directory>

<config>
go.mod - Go 模块定义，依赖 go-sqlite3
</config>

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
