# internal/db/
> L2 | 父级: /CLAUDE.md

db.go: 数据层核心，Database struct + 6 张表 CRUD，WAL 模式。New() 读写打开，NewReadOnly() 只读打开供 dashboard 使用

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
