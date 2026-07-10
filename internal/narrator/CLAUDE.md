# internal/narrator/
> L2 | 父级: /CLAUDE.md

narrator.go: 系统级叙事者 agent，持有独立 DB + LLM + StartFunc，处理联系人缺口；工具为 decide_character / write_character / start_agent，输出 ask/create/reject，记录 narrator_decisions 与 reasoning_logs，不参与普通 worker 心跳

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
