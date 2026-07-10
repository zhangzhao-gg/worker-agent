/**
 * [INPUT]: 依赖 internal/db.Soul, RunContext
 * [OUTPUT]: 对外提供 buildSystemPrompt, buildConversationPrompt, buildInitialMessage 函数
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

当前状态：
  心情：%d
  希望：%d
  不满：%d
  身体：%s

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
  心跳规则（manage_heartbeat）
═══════════════════════════════════════

心跳是你在工作时间内的具体任务安排。
早晨唤醒时，你应该用 manage_heartbeat 为今天 08:00-18:00 安排工作内容。
每条心跳是一个 10 分钟时间段内你要做的事。同一时间不可重复安排。
心跳执行时不需要思考，你的身体会自动完成并向城市汇报。

═══════════════════════════════════════
  其他规则
═══════════════════════════════════════

- 每次思考结束前，确保未来至少有一个 pending 的唤醒
- 用 write_memory 记录想法（普通记忆，随时间淡忘）
- 持久记忆（persistent=true）极其珍贵，上限仅 10 条，每次唤醒都会展示
  只在以下情况使用：改变人生方向的决策、不可替代的人物关系、重大转折
  日常感想、工作计划、情绪波动 → 用普通记忆，绝不用持久记忆
  满 10 条后必须先 delete_memory 腾出空间才能写入新的
- 用 write_diary 写公开日记（所有人可见，你可以选择展示什么）
- 用 update_soul 更新内心状态

你对城市的感知是有限的、模糊的：
  - 你不知道其他工人在想什么
  - 你不知道城市资源的精确数字（只能感受到「紧张/正常/充裕」）
  - 你不知道执政官的内部决策过程

你可以使用工具来感知城市和采取行动。
用 TodoWrite 追踪你的思考步骤，确保完成所有计划。`,
		s.Name, s.Occupation,
		s.Background,
		s.Personality,
		s.SpeechStyle,
		s.ValuesDesc,
		s.Family,
		s.Mood, s.Hope, s.Grievance, s.BodyStatus,
	)
}

func buildConversationPrompt(ctx RunContext, senderName string) string {
	s := ctx.Soul

	var b strings.Builder
	b.WriteString(fmt.Sprintf("你是 %s，%s。\n%s\n\n", s.Name, s.Occupation, s.Background))
	b.WriteString(fmt.Sprintf("你的性格：%s\n", s.Personality))
	b.WriteString(fmt.Sprintf("你的说话方式：%s\n\n", s.SpeechStyle))

	if len(ctx.PersistentMemories) > 0 {
		b.WriteString("你的持久记忆：\n")
		for _, m := range ctx.PersistentMemories {
			b.WriteString(fmt.Sprintf("- %s\n", m.Content))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf(`%s 正在和你说话。
用 reply_message 回复后对方可能继续说话，用 end_conversation 说最后一句并结束。
调用 reply_message/end_conversation 后，本轮思考会交给对方；如果对话里出现值得以后记住的信息、承诺、人物关系或重要情绪，必须先调用 write_memory，再回复。
像真人一样对话——该回就回，该结束就结束，不要刻意延续话题。
你可以用 recall 回忆相关记忆，用 write_memory 记下重要对话内容。`, senderName))

	return b.String()
}

func buildInitialMessage(trigger string, ctx RunContext) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("当前时间：%s\n", time.Now().Format("2006-01-02 15:04")))

	switch trigger {
	case "scheduled_wakeup":
		hour := time.Now().Hour()
		if hour >= 8 && hour < 18 {
			b.WriteString(fmt.Sprintf("你停下手中的活，开始思考。原因：%s\n", ctx.Reason))
		} else {
			b.WriteString(fmt.Sprintf("你醒来了。原因：%s\n", ctx.Reason))
		}
	case "urgent_news":
		b.WriteString(fmt.Sprintf("紧急消息打断了你：%s\n", ctx.News))
	}

	if ctx.WorkAssignment != "" {
		b.WriteString(fmt.Sprintf("\n今日工作分配：%s\n", ctx.WorkAssignment))
	}

	if len(ctx.PersistentMemories) > 0 {
		b.WriteString("\n你的持久记忆（重要，不可遗忘）：\n")
		for _, m := range ctx.PersistentMemories {
			b.WriteString(fmt.Sprintf("- [id=%d] %s\n", m.ID, m.Content))
		}
	}

	if len(ctx.Events) > 0 {
		b.WriteString("\n最近发生的事件：\n")
		for _, e := range ctx.Events {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", e.Timestamp, e.Content))
		}
	}

	b.WriteString("\n心跳计划概况（昨天 + 今天 + 明天）：\n")
	if len(ctx.Heartbeats) == 0 {
		b.WriteString("  （空）\n")
	} else {
		type dayStat struct {
			total, done, pending, skipped int
			samples                       []string
		}
		days := make(map[string]*dayStat)
		for _, h := range ctx.Heartbeats {
			ds, ok := days[h.Date]
			if !ok {
				ds = &dayStat{}
				days[h.Date] = ds
			}
			ds.total++
			switch h.Status {
			case "done":
				ds.done++
			case "pending":
				ds.pending++
			case "skipped":
				ds.skipped++
			}
			if len(ds.samples) < 3 {
				ds.samples = append(ds.samples, fmt.Sprintf("%s %s", h.Time, h.Task))
			}
		}
		today := time.Now().Format("2006-01-02")
		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		for _, date := range []string{yesterday, today, tomorrow} {
			ds, ok := days[date]
			if !ok {
				continue
			}
			label := date
			if date == today {
				label = date + "（今天）"
			} else if date == yesterday {
				label = date + "（昨天）"
			} else if date == tomorrow {
				label = date + "（明天）"
			}
			b.WriteString(fmt.Sprintf("  %s: 共%d条，完成%d/待执行%d/跳过%d\n", label, ds.total, ds.done, ds.pending, ds.skipped))
			for _, s := range ds.samples {
				b.WriteString(fmt.Sprintf("    · %s\n", s))
			}
			if ds.total > 3 {
				b.WriteString(fmt.Sprintf("    · ...（还有%d条）\n", ds.total-3))
			}
		}
	}

	b.WriteString("\n已有的唤醒计划（过去3天 + 未来3天），请勿重复安排相同时间段的唤醒：\n")
	if len(ctx.Wakeups) == 0 {
		b.WriteString("  （空）\n")
	} else {
		for _, w := range ctx.Wakeups {
			b.WriteString(fmt.Sprintf("- [id=%d] [%s] %s（%s）\n", w.ID, w.Datetime, w.Reason, w.Status))
		}
	}

	b.WriteString("\n请开始你的思考和行动。")
	return b.String()
}
