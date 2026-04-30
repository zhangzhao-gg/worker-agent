# internal/worker/
> L2 | 父级: /CLAUDE.md

heartbeat.go: 心跳协程 + WakeupSignal 类型 + CheckUrgency（复用同一 LLM，单轮简短 prompt）
wakeup.go: 唤醒调度协程，select 三路复用（紧急 channel / 定时扫描 / 退出信号）+ 防消失兜底

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
