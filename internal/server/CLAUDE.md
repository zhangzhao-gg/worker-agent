# internal/server/
> L2 | 父级: /CLAUDE.md

server.go: 纯 HTTP API + 城市公告读取 + 工人生命周期管理（创建/恢复/查询/删除/手动唤醒/事件推送/访客聊天/关闭访客聊天），DB 持久化重启自动恢复协程但不注入恢复唤醒，事件推送含紧急判断+条件唤醒，MessageRouter 创建 + wakeupFn 注册实现 agent 间对话唤醒，per-worker reasoning 锁 + chatWith 占用态阻止新访客排队，ContactResolver 接入叙事者创建联系人角色

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
