/**
 * [INPUT]: 依赖 internal/db.Database, internal/engine.Engine, sync/atomic 暴露实时推理与对话占用状态
 * [OUTPUT]: 对外提供 RunWakeup 函数
 * [POS]: internal/worker 的唤醒调度协程，工人的「大脑入口」——只在关键时刻醒来
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"worker-agent/internal/db"
	"worker-agent/internal/engine"
)

// ================================================================
//  唤醒调度协程
// ================================================================

func RunWakeup(ctx context.Context, database *db.Database, eng *engine.Engine, wakeupCh <-chan WakeupSignal, wg *sync.WaitGroup, reasoning *atomic.Bool, chatWith *atomic.Value) {
	defer wg.Done()
	log.Println("[唤醒] 协程启动")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var pendingConvs []*ConversationContext

	processConversation := func(conv *ConversationContext) {
		reasoning.Store(true)
		chatWith.Store(conv.SenderName)
		defer func() {
			chatWith.Store("")
			reasoning.Store(false)
		}()
		if err := handleConversationWakeup(database, eng, conv); err != nil {
			log.Printf("[唤醒] 对话处理失败: %v", err)
		}
	}

	drainPending := func() {
		for len(pendingConvs) > 0 {
			conv := pendingConvs[0]
			pendingConvs = pendingConvs[1:]
			log.Printf("[唤醒] 处理排队对话: from=%s, conv=%s", conv.SenderName, conv.ConversationID)
			processConversation(conv)
		}
	}

	for {
		select {
		case signal := <-wakeupCh:
			if signal.Conversation != nil {
				log.Printf("[唤醒] 收到对话信号: from=%s, conv=%s", signal.Conversation.SenderName, signal.Conversation.ConversationID)
				if reasoning.Load() {
					log.Println("[唤醒] 正在推理中，对话排队等待")
					pendingConvs = append(pendingConvs, signal.Conversation)
					continue
				}
				processConversation(signal.Conversation)
				continue
			}

			log.Printf("[唤醒] 收到紧急信号: trigger=%s", signal.Trigger)
			if reasoning.Load() {
				log.Println("[唤醒] 正在推理中，紧急信号暂存为事件")
				database.InsertEvent(signal.News)
				continue
			}
			reasoning.Store(true)
			if err := handleWakeup(database, eng, signal.Trigger, signal.News, ""); err != nil {
				if errors.Is(err, engine.ErrSelfDestruct) {
					log.Println("[唤醒] ☠ 工人自我终结，协程退出")
					return
				}
				log.Printf("[唤醒] 紧急唤醒失败: %v", err)
			}
			reasoning.Store(false)
			drainPending()

		case <-ticker.C:
			if reasoning.Load() {
				continue
			}

			drainPending()

			now := time.Now().Format(time.RFC3339)
			entries, err := database.GetPendingWakeups(now)
			if err != nil {
				log.Printf("[唤醒] 查询计划失败: %v", err)
				continue
			}
			for _, entry := range entries {
				log.Printf("[唤醒] 触发唤醒: id=%d, datetime=%s, reason=%s", entry.ID, entry.Datetime, entry.Reason)
				reasoning.Store(true)
				if err := handleWakeup(database, eng, "scheduled_wakeup", "", entry.Reason); err != nil {
					if errors.Is(err, engine.ErrSelfDestruct) {
						log.Println("[唤醒] ☠ 工人自我终结，协程退出")
						return
					}
					log.Printf("[唤醒] 唤醒失败，保留 pending 状态: %v", err)
				} else {
					database.MarkWakeupDone(entry.ID)
				}
				reasoning.Store(false)
			}

			drainPending()

		case <-ctx.Done():
			log.Println("[唤醒] 协程退出")
			return
		}
	}
}

func handleConversationWakeup(database *db.Database, eng *engine.Engine, conv *ConversationContext) error {
	soul, err := database.GetSoul()
	if err != nil {
		return err
	}

	persistentMemories, _ := database.GetPersistentMemories()

	ctx := engine.RunContext{
		Soul:               soul,
		PersistentMemories: persistentMemories,
	}

	return eng.RunConversation(conv.ConversationID, conv.SenderName, conv.Content, ctx)
}

func handleWakeup(database *db.Database, eng *engine.Engine, trigger string, news string, reason string) error {
	log.Printf("[唤醒] handleWakeup 开始: trigger=%s, reason=%s, news=%s", trigger, reason, news)

	soul, err := database.GetSoul()
	if err != nil {
		log.Printf("[唤醒] 读取 soul 失败: %v", err)
		return err
	}

	persistentMemories, _ := database.GetPersistentMemories()
	events, _ := database.GetUnprocessedEvents()

	now := time.Now()
	rangeFrom := now.AddDate(0, 0, -3).Format(time.RFC3339)
	rangeTo := now.AddDate(0, 0, 3).Format(time.RFC3339)
	wakeups, _ := database.GetWakeupRange(rangeFrom, rangeTo)

	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	heartbeats, _ := database.GetHeartbeatsByDateRange(yesterday, tomorrow)

	ctx := engine.RunContext{
		Soul:               soul,
		PersistentMemories: persistentMemories,
		Events:             events,
		Wakeups:            wakeups,
		Heartbeats:         heartbeats,
		Reason:             reason,
		News:               news,
	}

	if err := eng.Run(trigger, ctx); err != nil {
		return err
	}

	database.MarkEventsProcessed()

	hasPending, _ := database.HasPendingWakeups()
	if !hasPending {
		tomorrow := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour).Add(8 * time.Hour)
		database.InsertWakeup(tomorrow.Format(time.RFC3339), "兜底唤醒")
	}
	return nil
}
