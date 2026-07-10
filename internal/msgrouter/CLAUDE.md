# internal/msgrouter/
> L2 | 父级: /CLAUDE.md

router.go: 共享消息路由器，agent 间多轮同步对话基础设施。共享 SQLite DB（_messages.db），双 channel 同步（waiters: sender 等回复，receivers: receiver 等续接）。核心方法：SendAndWait（发起/续接 + 阻塞等回复）、Reply（回复 sender）、WaitNextMessage（receiver 等 sender 续接）、CloseConversation（释放 receiver 等待）。安全阀：MaxRoundsPerConv=30、120s 超时、禁止自言自语

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
