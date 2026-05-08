# internal/db/
> L2 | 父级: /CLAUDE.md

db.go: 数据层核心，Database struct + 7 张表 CRUD，WAL 模式。New() 读写打开（含自动迁移），NewReadOnly() 只读打开供 dashboard 使用。soul 表含 body_status 文本字段；memories 表 type 区分 memory/persistent/summary + CountPersistentMemories()；heartbeat_schedule 有 UNIQUE(time, date) 约束；wakeup_schedule 同小时 pending 去重；reasoning_logs 表记录推理全链路日志

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
