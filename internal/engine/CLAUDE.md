# internal/engine/
> L2 | 父级: /CLAUDE.md

engine.go: 入口 struct，持有 db/cityAPI/llm 引用，暴露 Run()，RunContext 含 PersistentMemories，生成 sessionID 并注入推理日志回调
prompt.go: system prompt 动态组装，soul（含 body_status）+ 持久记忆 + context → 完整 prompt
loop.go: agent loop 推理循环，每轮写 reasoning_logs + 首次 stop 注入审视提示（review 机制）+ 自动压缩
tools.go: 12 个工具注册表（2 感知: sense_city/recall + 7 行动: manage_heartbeat/schedule_wakeup/cancel_wakeup/write_diary/write_memory/delete_memory/update_soul + 1 终极: self_destruct + 2 元: TodoWrite）。持久记忆 10 条硬上限，心跳/唤醒时间重复直接报错
todo.go: TodoManager，推理步骤追踪，防偏移
compress.go: 上下文压缩管线，microcompact + autoCompact

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
