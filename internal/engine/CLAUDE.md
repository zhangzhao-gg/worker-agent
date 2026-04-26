# internal/engine/
> L2 | 父级: /CLAUDE.md

engine.go: 入口 struct，持有 db/cityAPI 引用，暴露 Run()
prompt.go: system prompt 动态组装，soul + context → 完整 prompt
loop.go: agent loop 推理循环，参考 s_full.py 移植
tools.go: 14 个工具注册表（6 感知 + 6 行动 + 2 元）+ handler 实现
todo.go: TodoManager，推理步骤追踪，防偏移
compress.go: 上下文压缩管线，microcompact + autoCompact

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
