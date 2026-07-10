/**
 * [INPUT]: 依赖 database/sql, github.com/mattn/go-sqlite3
 * [OUTPUT]: 对外提供 MessageRouter struct，ConversationSignal，Message，ReplyPayload 类型及 CloseConversation 释放对话等待
 * [POS]: internal/msgrouter 的唯一成员，agent 间多轮同步对话的核心路由器
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package msgrouter

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ================================================================
//  常量
// ================================================================

const (
	SendTimeout      = 120 * time.Second
	RecvTimeout      = 60 * time.Second
	MaxRoundsPerConv = 30
	closedMessage    = "\x00closed"
)

var ErrConversationClosed = errors.New("对话已被对方关闭")

// ================================================================
//  类型定义
// ================================================================

type ReplyPayload struct {
	Content        string
	Ended          bool
	ConversationID string
}

type MessageRouter struct {
	db        *sql.DB
	waiters   map[string]chan ReplyPayload
	receivers map[string]chan string
	mu        sync.Mutex
	wakeupFn  func(workerName string, signal ConversationSignal) bool
}

type ConversationSignal struct {
	ConversationID string
	SenderName     string
	Content        string
}

type Message struct {
	ID             int64
	ConversationID string
	Sender         string
	Receiver       string
	Content        string
	CreatedAt      string
}

// ================================================================
//  构造
// ================================================================

func New(dbPath string) (*MessageRouter, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开消息数据库: %w", err)
	}
	conn.Exec("PRAGMA journal_mode=WAL")

	r := &MessageRouter{
		db:        conn,
		waiters:   make(map[string]chan ReplyPayload),
		receivers: make(map[string]chan string),
	}
	if err := r.createTables(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("建表: %w", err)
	}
	return r, nil
}

func (r *MessageRouter) SetWakeupFn(fn func(string, ConversationSignal) bool) {
	r.wakeupFn = fn
}

func (r *MessageRouter) Close() error {
	return r.db.Close()
}

func (r *MessageRouter) createTables() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS messages (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT NOT NULL,
		sender          TEXT NOT NULL,
		receiver        TEXT NOT NULL,
		content         TEXT NOT NULL,
		created_at      TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_msg_conv ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_msg_receiver ON messages(receiver, created_at);`
	_, err := r.db.Exec(ddl)
	return err
}

// ================================================================
//  Sender 侧
// ================================================================

// SendAndWait 发起或续接对话。convID 为空则创建新对话。
func (r *MessageRouter) SendAndWait(from, to, content, convID string, timeout time.Duration) (ReplyPayload, error) {
	if from == to {
		return ReplyPayload{}, fmt.Errorf("不能和自己对话")
	}

	isNew := convID == ""
	if isNew {
		convID = fmt.Sprintf("%s_%s_%d", from, to, time.Now().UnixNano())
	}

	rounds := r.countRounds(convID)
	if rounds >= MaxRoundsPerConv {
		return ReplyPayload{}, fmt.Errorf("对话已达 %d 轮上限", MaxRoundsPerConv)
	}

	if err := r.insertMessage(convID, from, to, content); err != nil {
		return ReplyPayload{}, err
	}

	// 注册 sender 等待 channel
	replyCh := make(chan ReplyPayload, 1)
	r.mu.Lock()
	r.waiters[convID] = replyCh
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.waiters, convID)
		r.mu.Unlock()
	}()

	// 续接时直接 signal receiver 的等待 channel
	if !isNew {
		r.mu.Lock()
		recvCh, ok := r.receivers[convID]
		r.mu.Unlock()
		if ok {
			select {
			case recvCh <- content:
			default:
			}
		}
	} else {
		// 新对话：唤醒 receiver
		if r.wakeupFn == nil {
			return ReplyPayload{}, fmt.Errorf("消息路由未初始化")
		}
		signal := ConversationSignal{
			ConversationID: convID,
			SenderName:     from,
			Content:        content,
		}
		if !r.wakeupFn(to, signal) {
			return ReplyPayload{}, fmt.Errorf("对方 %s 不在线或忙碌", to)
		}
	}

	log.Printf("[msgrouter] %s → %s: 等待回复 (conv=%s, round=%d)", from, to, convID, rounds+1)

	select {
	case reply := <-replyCh:
		reply.ConversationID = convID
		log.Printf("[msgrouter] %s ← %s: 收到回复 (conv=%s)", from, to, convID)
		return reply, nil
	case <-time.After(timeout):
		log.Printf("[msgrouter] %s → %s: 等待超时 (conv=%s)", from, to, convID)
		return ReplyPayload{}, fmt.Errorf("等待 %s 回复超时（%v）", to, timeout)
	}
}

// ================================================================
//  Receiver 侧
// ================================================================

// Reply receiver 回复 sender。ended=true 表示结束对话。
func (r *MessageRouter) Reply(convID, from, content string, ended bool) error {
	var receiver string
	err := r.db.QueryRow(
		"SELECT sender FROM messages WHERE conversation_id = ? ORDER BY id ASC LIMIT 1",
		convID,
	).Scan(&receiver)
	if err != nil {
		return fmt.Errorf("找不到对话 %s: %w", convID, err)
	}

	if err := r.insertMessage(convID, from, receiver, content); err != nil {
		return err
	}

	r.mu.Lock()
	ch, ok := r.waiters[convID]
	r.mu.Unlock()
	if ok {
		select {
		case ch <- ReplyPayload{Content: content, Ended: ended, ConversationID: convID}:
		default:
		}
	}
	return nil
}

// WaitNextMessage receiver 回复后阻塞等待 sender 的下一条消息。超时返回错误。
func (r *MessageRouter) WaitNextMessage(convID string, timeout time.Duration) (string, error) {
	recvCh := make(chan string, 1)
	r.mu.Lock()
	r.receivers[convID] = recvCh
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.receivers, convID)
		r.mu.Unlock()
	}()

	select {
	case msg := <-recvCh:
		if msg == closedMessage {
			return "", ErrConversationClosed
		}
		return msg, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("等待对方继续对话超时")
	}
}

func (r *MessageRouter) CloseConversation(convID string) bool {
	r.mu.Lock()
	recvCh, ok := r.receivers[convID]
	r.mu.Unlock()
	if !ok {
		return false
	}

	select {
	case recvCh <- closedMessage:
		return true
	default:
		return false
	}
}

// ================================================================
//  查询
// ================================================================

func (r *MessageRouter) GetRecentMessages(workerName string, n int) ([]Message, error) {
	rows, err := r.db.Query(
		`SELECT id, conversation_id, sender, receiver, content, created_at
		 FROM messages WHERE sender = ? OR receiver = ?
		 ORDER BY id DESC LIMIT ?`, workerName, workerName, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Sender, &m.Receiver, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// ================================================================
//  辅助
// ================================================================

func (r *MessageRouter) insertMessage(convID, sender, receiver, content string) error {
	_, err := r.db.Exec(
		"INSERT INTO messages (conversation_id, sender, receiver, content, created_at) VALUES (?, ?, ?, ?, ?)",
		convID, sender, receiver, content, time.Now().Format(time.RFC3339),
	)
	return err
}

func (r *MessageRouter) countRounds(convID string) int {
	var count int
	r.db.QueryRow("SELECT COUNT(*) FROM messages WHERE conversation_id = ?", convID).Scan(&count)
	return count / 2
}
