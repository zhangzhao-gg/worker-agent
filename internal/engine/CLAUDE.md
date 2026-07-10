# internal/engine/
> L2 | 父级: /CLAUDE.md

engine.go: 入口 struct，持有 db/cityAPI/llm/router/workerName，暴露 Run() + RunConversation()，RunContext 含 PersistentMemories，生成 sessionID 并注入推理日志回调，对话入口提示回复前先处理必要记忆
prompt.go: system prompt 动态组装，soul（含 body_status）+ 持久记忆 + context → 完整 prompt；buildConversationPrompt 为对话模式精简 prompt，并约束 reply/end 前先 write_memory
conversation.go: 对话模式工具集（reply_message/end_conversation/recall/write_memory），receiver 侧多轮对话，reply_message 阻塞等 sender 续接，end_conversation 返回 ErrConversationDone 优雅退出
loop.go: agent loop 推理循环，每轮写 reasoning_logs + 首次 stop 注入审视提示（review 机制）+ 自动压缩 + MaxRounds 支持，终止型对话工具固定最后执行避免吞掉 write_memory
tools.go: 13 个工具注册表（2 感知: sense_city/recall + 8 行动: manage_heartbeat/schedule_wakeup/cancel_wakeup/write_diary/write_memory/delete_memory/update_soul/send_message + 1 终极: self_destruct + 2 元: TodoWrite）。持久记忆 10 条硬上限，心跳/唤醒时间重复直接报错，send_message 支持多轮续接（传 conversation_id）30 轮封顶
todo.go: TodoManager，推理步骤追踪，防偏移
compress.go: 上下文压缩管线，microcompact + autoCompact

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
