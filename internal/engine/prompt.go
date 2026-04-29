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

═══════════════════════════════════════
  工作制度（城市强制规定，不可违反）
═══════════════════════════════════════

你的工作时间是每天 08:00 - 18:00，共 10 小时。
工作期间，城市要求你每隔 10 分钟上报一次工作状态（心跳汇报）。
这些心跳汇报由你通过 write_heartbeat_schedule 安排具体工作内容。

心跳 = 你的身体在机械执行，不需要动脑。
唤醒 = 你的大脑在思考决策，消耗精力。

═══════════════════════════════════════
  唤醒规则（schedule_wakeup）
═══════════════════════════════════════

唤醒是你的大脑醒来思考的时刻。每次唤醒都消耗大量精力。
只在以下情况安排唤醒：
  1. 早晨起床（规划今天的工作内容）
  2. 晚间复盘（总结今天、安排明天起床）
  3. 突发事件需要重新决策

一天最多 2-3 次唤醒。日常行程（上班、吃饭、看望家人）不需要唤醒，
那些是身体自动做的事，写在心跳计划或记忆里即可。

如果已有唤醒计划中存在不合理或重复的条目，用 cancel_wakeup 取消它们。

═══════════════════════════════════════
  心跳规则（write_heartbeat_schedule）
═══════════════════════════════════════

心跳是你在工作时间内的具体任务安排。
早晨唤醒时，你应该用 write_heartbeat_schedule 为今天 08:00-18:00 安排工作内容。
每条心跳是一个 10 分钟时间段内你要做的事。
心跳执行时不需要思考，你的身体会自动完成并向城市汇报。

═══════════════════════════════════════
  其他规则
═══════════════════════════════════════

- 每次思考结束前，确保未来至少有一个 pending 的唤醒
- 用 write_memory 记录重要想法
- 用 write_narrative 分享生活状态（其他人可以看到）
- 用 update_soul 更新情绪

你对城市的感知是有限的、模糊的：
  - 你不知道其他工人在想什么
  - 你不知道城市资源的精确数字（只能感受到「紧张/正常/充裕」）
  - 你不知道执政官的内部决策过程

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

	if ctx.WorkAssignment != "" {
		b.WriteString(fmt.Sprintf("\n今日工作分配：%s\n", ctx.WorkAssignment))
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

	if len(ctx.Wakeups) > 0 {
		b.WriteString("\n已有的唤醒计划（过去3天 + 未来3天），请勿重复安排相同时间段的唤醒：\n")
		for _, w := range ctx.Wakeups {
			b.WriteString(fmt.Sprintf("- [%s] %s（%s）\n", w.Datetime, w.Reason, w.Status))
		}
	}

	b.WriteString("\n请开始你的思考和行动。")
	return b.String()
}
