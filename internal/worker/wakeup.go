/**
 * [INPUT]: 依赖 internal/db.Database, internal/engine.Engine
 * [OUTPUT]: 对外提供 RunWakeup 函数
 * [POS]: internal/worker 的唤醒调度协程，工人的「大脑入口」——只在关键时刻醒来
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"worker-agent/internal/db"
	"worker-agent/internal/engine"
)

// ================================================================
//  唤醒调度协程
// ================================================================

func RunWakeup(ctx context.Context, database *db.Database, eng *engine.Engine, wakeupCh <-chan WakeupSignal, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case signal := <-wakeupCh:
			handleWakeup(database, eng, signal.Trigger, signal.News, "")

		case <-ticker.C:
			now := time.Now().Format(time.RFC3339)
			entries, err := database.GetPendingWakeups(now)
			if err != nil {
				log.Printf("[唤醒] 查询计划失败: %v", err)
				continue
			}
			for _, entry := range entries {
				handleWakeup(database, eng, "scheduled_wakeup", "", entry.Reason)
				database.MarkWakeupDone(entry.ID)
			}

		case <-ctx.Done():
			return
		}
	}
}

func handleWakeup(database *db.Database, eng *engine.Engine, trigger string, news string, reason string) {
	soul, err := database.GetSoul()
	if err != nil {
		log.Printf("[唤醒] 读取 soul 失败: %v", err)
		return
	}

	summary, _ := database.GetLatestSummary()
	events, _ := database.GetUnprocessedEvents()

	ctx := engine.RunContext{
		Soul:    soul,
		Summary: summary,
		Events:  events,
		Reason:  reason,
		News:    news,
	}

	if err := eng.Run(trigger, ctx); err != nil {
		log.Printf("[唤醒] 推理引擎错误: %v", err)
	}

	database.MarkEventsProcessed()

	// 防消失兜底：LLM 忘安排下一次 wakeup 时，自动补次日早晨
	hasPending, _ := database.HasPendingWakeups()
	if !hasPending {
		tomorrow := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour).Add(8 * time.Hour)
		database.InsertWakeup(tomorrow.Format(time.RFC3339), "兜底唤醒")
	}
}
