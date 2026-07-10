# worker-agent - 新伦敦城市模拟系统 · Worker Agent
Go + SQLite + MiniMax (OpenAI 兼容 API)

<directory>
cmd/worker/      - Worker 进程入口，纯 API + 协程调度，无 UI
internal/db/     - 数据层，SQLite CRUD，9 张表（含 reasoning_logs/contacts/narrator_decisions），单 struct 全部方法，自动迁移
internal/city/   - 城市 API 层，HTTP client + mock 模式，工人与外部世界的唯一接口
internal/llm/    - LLM 抽象层，Client 接口 + MiniMax 实现
internal/engine/    - LLM 推理引擎，agent loop + tool dispatch + 压缩 + todo + 推理日志 + 持久记忆注入 + 对话模式
internal/worker/    - 双协程，心跳协程（身体）+ 唤醒调度协程（大脑入口）+ 紧急判断 + per-worker 推理锁
internal/server/    - 纯 HTTP API + 工人生命周期管理 + 事件推送端点，DB 持久化重启自动恢复
internal/msgrouter/ - Agent 间同步对话路由器，共享 DB + channel 同步 + 安全阀
internal/narrator/  - 系统级叙事者 agent，联系人缺口裁定 + 角色编撰 + 新 agent 启动委托
internal/web/    - 静态文件服务（头像等）
dashboard/       - Node.js 独立 Dashboard（Vite + React + Express + better-sqlite3），直连 data/*.db 读写，访客视图+详情视图双模式，安全码仅保护编辑/重置，黄皮纸/油墨/撕裂边缘视觉风格
docs/            - 设计文档：PRD + 业务层 + 推理引擎 + 视觉实现 + 部署手册 + 进度
</directory>

<config>
go.mod             - Go 模块定义，依赖 go-sqlite3
dashboard/package.json - Dashboard Node 依赖
</config>

## 线上部署

- 域名: https://worker.okethan.top
- 部署方式: rsync + vite build + systemctl restart
- 详见 docs/deploy.md（本地文件，未入 git）

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
