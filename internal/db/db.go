/**
 * [INPUT]: 依赖 database/sql, github.com/mattn/go-sqlite3
 * [OUTPUT]: 对外提供 Database struct 及全部 CRUD 方法
 * [POS]: internal/db 的唯一成员，数据层核心，被心跳协程/唤醒协程/推理引擎共同消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ================================================================
//  核心结构体
// ================================================================

type Database struct {
	db *sql.DB
}

// ================================================================
//  模型定义
// ================================================================

type Soul struct {
	Name        string
	Occupation  string
	Background  string
	Personality string
	SpeechStyle string
	ValuesDesc  string
	Family      string
	Avatar      string
	Mood        int
	Hope        int
	Grievance   int
}

type SoulUpdate struct {
	Field string
	Value int
}

type HeartbeatEntry struct {
	ID     int64
	Time   string
	Date   string
	Task   string
	Status string
}

type WakeupEntry struct {
	ID       int64
	Datetime string
	Reason   string
	Status   string
}

type Event struct {
	ID        int64
	Timestamp string
	Content   string
	Processed int
}

type Memory struct {
	ID        int64
	Timestamp string
	Content   string
	Type      string
}

// ================================================================
//  构造 & 建表
// ================================================================

func New(dbPath string) (*Database, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	conn.Exec("PRAGMA journal_mode=WAL")

	d := &Database{db: conn}
	if err := d.createTables(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("建表: %w", err)
	}
	return d, nil
}

// NewReadOnly 只读打开已有数据库，不建表，供 dashboard 独立进程使用
func NewReadOnly(dbPath string) (*Database, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("只读打开数据库: %w", err)
	}
	return &Database{db: conn}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) createTables() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS soul (
		id           INTEGER PRIMARY KEY CHECK (id = 1),
		name         TEXT NOT NULL,
		occupation   TEXT NOT NULL,
		background   TEXT,
		personality  TEXT,
		speech_style TEXT,
		values_desc  TEXT,
		family       TEXT,
		avatar       TEXT DEFAULT '',
		mood         INTEGER DEFAULT 50,
		hope         INTEGER DEFAULT 50,
		grievance    INTEGER DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS heartbeat_schedule (
		id     INTEGER PRIMARY KEY AUTOINCREMENT,
		time   TEXT NOT NULL,
		date   TEXT NOT NULL,
		task   TEXT NOT NULL,
		status TEXT DEFAULT 'pending'
	);
	CREATE TABLE IF NOT EXISTS wakeup_schedule (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		datetime TEXT NOT NULL,
		reason   TEXT,
		status   TEXT DEFAULT 'pending'
	);
	CREATE TABLE IF NOT EXISTS events (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		content   TEXT NOT NULL,
		processed INTEGER DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS memories (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		content   TEXT NOT NULL,
		type      TEXT DEFAULT 'memory'
	);
	CREATE TABLE IF NOT EXISTS narratives (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		content   TEXT NOT NULL
	);`
	_, err := d.db.Exec(ddl)
	return err
}

// ================================================================
//  Soul
// ================================================================

func (d *Database) GetSoul() (Soul, error) {
	var s Soul
	err := d.db.QueryRow(`SELECT name, occupation, background, personality,
		speech_style, values_desc, family, avatar, mood, hope, grievance
		FROM soul WHERE id = 1`).Scan(
		&s.Name, &s.Occupation, &s.Background, &s.Personality,
		&s.SpeechStyle, &s.ValuesDesc, &s.Family, &s.Avatar,
		&s.Mood, &s.Hope, &s.Grievance,
	)
	return s, err
}

func (d *Database) UpdateSoul(updates []SoulUpdate) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	allowed := map[string]bool{"mood": true, "hope": true, "grievance": true}
	for _, u := range updates {
		if !allowed[u.Field] {
			return fmt.Errorf("禁止修改字段: %s", u.Field)
		}
		_, err := tx.Exec(fmt.Sprintf("UPDATE soul SET %s = ? WHERE id = 1", u.Field), u.Value)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *Database) InitSoul(s Soul) error {
	_, err := d.db.Exec(`INSERT OR REPLACE INTO soul
		(id, name, occupation, background, personality, speech_style, values_desc, family, avatar, mood, hope, grievance)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Name, s.Occupation, s.Background, s.Personality,
		s.SpeechStyle, s.ValuesDesc, s.Family, s.Avatar,
		s.Mood, s.Hope, s.Grievance,
	)
	return err
}

// ================================================================
//  Heartbeat Schedule
// ================================================================

func (d *Database) GetPendingHeartbeats(now string) ([]HeartbeatEntry, error) {
	rows, err := d.db.Query(
		`SELECT id, time, date, task, status FROM heartbeat_schedule
		 WHERE status = 'pending' AND date || 'T' || time <= ?
		 ORDER BY date, time`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []HeartbeatEntry
	for rows.Next() {
		var e HeartbeatEntry
		if err := rows.Scan(&e.ID, &e.Time, &e.Date, &e.Task, &e.Status); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *Database) InsertHeartbeats(entries []HeartbeatEntry) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO heartbeat_schedule (time, date, task) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.Time, e.Date, e.Task); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *Database) UpdateHeartbeatStatus(id int64, status string) error {
	_, err := d.db.Exec("UPDATE heartbeat_schedule SET status = ? WHERE id = ?", status, id)
	return err
}

// ================================================================
//  Wakeup Schedule
// ================================================================

func (d *Database) GetPendingWakeups(now string) ([]WakeupEntry, error) {
	rows, err := d.db.Query(
		`SELECT id, datetime, reason, status FROM wakeup_schedule
		 WHERE status = 'pending' AND datetime <= ?
		 ORDER BY datetime`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []WakeupEntry
	for rows.Next() {
		var e WakeupEntry
		if err := rows.Scan(&e.ID, &e.Datetime, &e.Reason, &e.Status); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *Database) InsertWakeup(datetime string, reason string) error {
	t, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		t, err = time.ParseInLocation("2006-01-02T15:04:05", datetime, time.Local)
		if err != nil {
			return fmt.Errorf("时间格式错误: %w", err)
		}
		datetime = t.Format(time.RFC3339)
	}
	if t.Before(time.Now()) {
		return fmt.Errorf("拒绝过去的时间: %s", datetime)
	}
	_, err = d.db.Exec("INSERT INTO wakeup_schedule (datetime, reason) VALUES (?, ?)", datetime, reason)
	return err
}

func (d *Database) GetRecentWakeups(n int) ([]WakeupEntry, error) {
	rows, err := d.db.Query(
		`SELECT id, datetime, reason, status FROM wakeup_schedule ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []WakeupEntry
	for rows.Next() {
		var e WakeupEntry
		if err := rows.Scan(&e.ID, &e.Datetime, &e.Reason, &e.Status); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *Database) GetWakeupRange(from, to string) ([]WakeupEntry, error) {
	rows, err := d.db.Query(
		`SELECT id, datetime, reason, status FROM wakeup_schedule
		 WHERE datetime BETWEEN ? AND ?
		 ORDER BY datetime`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []WakeupEntry
	for rows.Next() {
		var e WakeupEntry
		if err := rows.Scan(&e.ID, &e.Datetime, &e.Reason, &e.Status); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *Database) MarkWakeupDone(id int64) error {
	_, err := d.db.Exec("UPDATE wakeup_schedule SET status = 'done' WHERE id = ?", id)
	return err
}

func (d *Database) CancelWakeup(id int64) error {
	_, err := d.db.Exec("UPDATE wakeup_schedule SET status = 'cancelled' WHERE id = ? AND status = 'pending'", id)
	return err
}

func (d *Database) HasPendingWakeups() (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM wakeup_schedule WHERE status = 'pending'").Scan(&count)
	return count > 0, err
}

// ================================================================
//  Events
// ================================================================

func (d *Database) InsertEvent(content string) error {
	_, err := d.db.Exec("INSERT INTO events (timestamp, content) VALUES (?, ?)",
		time.Now().Format(time.RFC3339), content)
	return err
}

func (d *Database) GetUnprocessedEvents() ([]Event, error) {
	return d.queryEvents("SELECT id, timestamp, content, processed FROM events WHERE processed = 0 ORDER BY id")
}

func (d *Database) GetRecentEvents(n int) ([]Event, error) {
	return d.queryEvents("SELECT id, timestamp, content, processed FROM events ORDER BY id DESC LIMIT ?", n)
}

func (d *Database) MarkEventsProcessed() error {
	_, err := d.db.Exec("UPDATE events SET processed = 1 WHERE processed = 0")
	return err
}

func (d *Database) queryEvents(query string, args ...any) ([]Event, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Content, &e.Processed); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ================================================================
//  Memories
// ================================================================

func (d *Database) InsertMemory(content string, memType string) error {
	_, err := d.db.Exec("INSERT INTO memories (timestamp, content, type) VALUES (?, ?, ?)",
		time.Now().Format(time.RFC3339), content, memType)
	return err
}

func (d *Database) GetRecentMemories(n int) ([]Memory, error) {
	rows, err := d.db.Query("SELECT id, timestamp, content, type FROM memories ORDER BY id DESC LIMIT ?", n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Timestamp, &m.Content, &m.Type); err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

func (d *Database) GetLatestSummary() (string, error) {
	var content string
	err := d.db.QueryRow(
		"SELECT content FROM memories WHERE type = 'summary' ORDER BY id DESC LIMIT 1",
	).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

// ================================================================
//  Narratives
// ================================================================

type Narrative struct {
	ID        int64
	Timestamp string
	Content   string
}

func (d *Database) InsertNarrative(content string) error {
	_, err := d.db.Exec("INSERT INTO narratives (timestamp, content) VALUES (?, ?)",
		time.Now().Format(time.RFC3339), content)
	return err
}

func (d *Database) GetRecentNarratives(n int) ([]Narrative, error) {
	rows, err := d.db.Query("SELECT id, timestamp, content FROM narratives ORDER BY id DESC LIMIT ?", n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var narratives []Narrative
	for rows.Next() {
		var n Narrative
		if err := rows.Scan(&n.ID, &n.Timestamp, &n.Content); err != nil {
			return nil, err
		}
		narratives = append(narratives, n)
	}
	return narratives, rows.Err()
}

func (d *Database) GetRecentHeartbeats(n int) ([]HeartbeatEntry, error) {
	rows, err := d.db.Query(
		`SELECT id, time, date, task, status FROM heartbeat_schedule ORDER BY date DESC, time DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []HeartbeatEntry
	for rows.Next() {
		var e HeartbeatEntry
		if err := rows.Scan(&e.ID, &e.Time, &e.Date, &e.Task, &e.Status); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
