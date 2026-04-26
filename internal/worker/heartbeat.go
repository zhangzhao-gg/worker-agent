/**
 * [INPUT]: 依赖 internal/db, internal/city, internal/llm
 * [OUTPUT]: 对外提供 RunHeartbeat 函数, WakeupSignal 类型
 * [POS]: internal/worker 的心跳协程，工人的「身体」——机械、规律、不思考
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"worker-agent/internal/city"
	"worker-agent/internal/db"
	"worker-agent/internal/llm"
)

// ================================================================
//  类型定义
// ================================================================

type WakeupSignal struct {
	Trigger string
	News    string
}

// ================================================================
//  心跳协程
// ================================================================

func RunHeartbeat(ctx context.Context, database *db.Database, cityAPI *city.CityAPI, llmClient llm.Client, workerID string, wakeupCh chan<- WakeupSignal, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processHeartbeats(database, cityAPI, llmClient, workerID, wakeupCh)
		}
	}
}

func processHeartbeats(database *db.Database, cityAPI *city.CityAPI, llmClient llm.Client, workerID string, wakeupCh chan<- WakeupSignal) {
	now := time.Now().Format("2006-01-02T15:04")
	entries, err := database.GetPendingHeartbeats(now)
	if err != nil {
		log.Printf("[心跳/%s] 查询计划失败: %v", workerID, err)
		return
	}

	for _, entry := range entries {
		resp, err := cityAPI.Heartbeat(workerID)
		if err != nil {
			log.Printf("[心跳/%s] 发送失败: %v", workerID, err)
			database.UpdateHeartbeatStatus(entry.ID, "skipped")
			continue
		}

		if resp.News != "" {
			database.InsertEvent(resp.News)
			log.Printf("[心跳/%s] 收到 news: %s", workerID, resp.News)

			soul, err := database.GetSoul()
			if err == nil && checkUrgency(llmClient, resp.News, soul) {
				log.Printf("[心跳/%s] 紧急唤醒 LLM", workerID)
				wakeupCh <- WakeupSignal{Trigger: "urgent_news", News: resp.News}
			}
		}

		database.UpdateHeartbeatStatus(entry.ID, "done")
	}
}

// ================================================================
//  紧急判断（本地轻量 LLM 调用）
// ================================================================

func checkUrgency(llmClient llm.Client, news string, soul db.Soul) bool {
	prompt := fmt.Sprintf(
		"你是%s（%s）。以下消息对你来说需要立刻停下手头工作去思考吗？只回答 yes 或 no。\n消息：%s",
		soul.Name, soul.Occupation, news,
	)
	resp, err := llmClient.Chat(
		"你是一个判断助手，只回答 yes 或 no。",
		[]llm.Message{{Role: "user", Content: prompt}},
		nil,
	)
	if err != nil {
		log.Printf("[紧急判断] LLM 调用失败: %v", err)
		return false
	}
	return strings.TrimSpace(strings.ToLower(resp.Message.Content)) == "yes"
}
