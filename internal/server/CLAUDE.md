# internal/server/
> L2 | 父级: /CLAUDE.md

server.go: 纯 HTTP API + 工人生命周期管理（创建/恢复/查询/删除/手动唤醒/事件推送），DB 持久化重启自动恢复，事件推送含紧急判断+条件唤醒

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
