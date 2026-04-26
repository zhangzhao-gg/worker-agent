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
	log.Println("[唤醒] 协程启动")

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case signal := <-wakeupCh:
			log.Printf("[唤醒] 收到紧急信号: trigger=%s", signal.Trigger)
			handleWakeup(database, eng, signal.Trigger, signal.News, "")

		case <-ticker.C:
			now := time.Now().Format(time.RFC3339)
			log.Printf("[唤醒] 定时扫描 pending wakeups, now=%s", now)
			entries, err := database.GetPendingWakeups(now)
			if err != nil {
				log.Printf("[唤醒] 查询计划失败: %v", err)
				continue
			}
			log.Printf("[唤醒] 扫描到 %d 条待处理唤醒", len(entries))
			for _, entry := range entries {
				log.Printf("[唤醒] 触发唤醒: id=%d, datetime=%s, reason=%s", entry.ID, entry.Datetime, entry.Reason)
				handleWakeup(database, eng, "scheduled_wakeup", "", entry.Reason)
				database.MarkWakeupDone(entry.ID)
			}

		case <-ctx.Done():
			log.Println("[唤醒] 协程退出")
			return
		}
	}
}

func handleWakeup(database *db.Database, eng *engine.Engine, trigger string, news string, reason string) {
	log.Printf("[唤醒] handleWakeup 开始: trigger=%s, reason=%s, news=%s", trigger, reason, news)

	soul, err := database.GetSoul()
	if err != nil {
		log.Printf("[唤醒] 读取 soul 失败: %v", err)
		return
	}
	log.Printf("[唤醒] soul 加载成功: name=%s", soul.Name)

	summary, _ := database.GetLatestSummary()
	events, _ := database.GetUnprocessedEvents()
	log.Printf("[唤醒] 上下文: summary长度=%d, 未处理事件=%d", len(summary), len(events))

	ctx := engine.RunContext{
		Soul:    soul,
		Summary: summary,
		Events:  events,
		Reason:  reason,
		News:    news,
	}

	log.Println("[唤醒] 调用推理引擎 eng.Run()...")
	if err := eng.Run(trigger, ctx); err != nil {
		log.Printf("[唤醒] 推理引擎错误: %v", err)
	} else {
		log.Println("[唤醒] 推理引擎执行完毕")
	}

	database.MarkEventsProcessed()

	hasPending, _ := database.HasPendingWakeups()
	log.Printf("[唤醒] 检查后续唤醒: hasPending=%v", hasPending)
	if !hasPending {
		tomorrow := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour).Add(8 * time.Hour)
		database.InsertWakeup(tomorrow.Format(time.RFC3339), "兜底唤醒")
		log.Printf("[唤醒] 补插兜底唤醒: %s", tomorrow.Format(time.RFC3339))
	}
}
