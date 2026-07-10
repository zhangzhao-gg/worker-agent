# internal/narrator/
> L2 | 父级: /CLAUDE.md

narrator.go: 系统级叙事者 agent，持有独立 DB + LLM + StartFunc，处理联系人缺口；多轮执行 decide_character / write_character / start_agent 三工具流程，ask/reject/start 成功才终止，记录 narrator_decisions 与 reasoning_logs，不参与普通 worker 心跳

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
