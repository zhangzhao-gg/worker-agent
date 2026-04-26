/**
 * [INPUT]: 依赖 internal/db.Soul, RunContext
 * [OUTPUT]: 对外提供 buildSystemPrompt, buildInitialMessage 函数
 * [POS]: internal/engine 的 prompt 组装器，从 soul + context 动态生成 system prompt 和初始消息
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package engine

import (
	"fmt"
	"strings"
	"time"
)

func buildSystemPrompt(ctx RunContext) string {
	s := ctx.Soul

	return fmt.Sprintf(`你是 %s，%s。
%s

你的性格：%s
你的说话方式：%s
你的价值观：%s
你的家人：%s

当前情绪状态：
  心情：%d
  希望：%d
  不满：%d

你对城市的感知是有限的、模糊的：
  - 你不知道其他工人在想什么（他们的内心对你不可见）
  - 你不知道城市资源的精确数字（只能感受到「紧张/正常/充裕」）
  - 你不知道执政官的内部决策过程

重要规则：
- 每次思考结束前，你必须用 schedule_wakeup 安排至少一个未来唤醒时间
- 早晨起床时，记得安排晚间复盘和明天起床
- 用 write_heartbeat_schedule 制定今天的工作计划
- 用 write_memory 记录重要的想法
- 用 write_narrative 分享你的生活状态（其他人可以看到）
- 用 update_soul 更新你的情绪

你可以使用以下工具来感知城市和采取行动。
用 TodoWrite 追踪你的思考步骤，确保完成所有步骤。`,
		s.Name, s.Occupation,
		s.Background,
		s.Personality,
		s.SpeechStyle,
		s.ValuesDesc,
		s.Family,
		s.Mood, s.Hope, s.Grievance,
	)
}

func buildInitialMessage(trigger string, ctx RunContext) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("当前时间：%s\n", time.Now().Format("2006-01-02 15:04")))

	switch trigger {
	case "scheduled_wakeup":
		b.WriteString(fmt.Sprintf("你刚刚醒来。原因：%s\n", ctx.Reason))
	case "urgent_news":
		b.WriteString(fmt.Sprintf("紧急消息打断了你的工作：%s\n", ctx.News))
	}

	if ctx.Summary != "" {
		b.WriteString(fmt.Sprintf("\n你最近的记忆摘要：\n%s\n", ctx.Summary))
	}

	if len(ctx.Events) > 0 {
		b.WriteString("\n最近发生的事件：\n")
		for _, e := range ctx.Events {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", e.Timestamp, e.Content))
		}
	}

	b.WriteString("\n请开始你的思考和行动。")
	return b.String()
}
