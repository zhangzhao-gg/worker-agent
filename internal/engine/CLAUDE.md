# internal/engine/
> L2 | 父级: /CLAUDE.md

engine.go: 入口 struct，持有 db/cityAPI/llm 引用，暴露 Run()，生成 sessionID 并注入推理日志回调
prompt.go: system prompt 动态组装，soul + context → 完整 prompt，心跳/唤醒为空时显示（空）
loop.go: agent loop 推理循环，每轮写 reasoning_logs + 首次 stop 注入审视提示（review 机制）
tools.go: 17 个工具注册表（6 感知 + 9 行动含 cancel_wakeup/delete_memory/self_destruct + 2 元）+ handler 实现，write_memory 支持 persistent 标记
todo.go: TodoManager，推理步骤追踪，防偏移
compress.go: 上下文压缩管线，microcompact + autoCompact

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
