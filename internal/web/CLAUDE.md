# internal/web/
> L2 | 父级: /CLAUDE.md

web.go: Web UI handler，embed 内嵌模板，首页工人列表 + 详情页六标签（叙事/记忆/事件/工作计划/唤醒计划/推理日志）
templates/index.html: 工人卡片列表页，维多利亚蒸汽朋克风格，30s 自动刷新
templates/detail.html: 工人详情档案页，左栏身份+情绪+手动唤醒+人设编辑，右栏六标签，reasoning 5s 刷新/其他 30s

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
