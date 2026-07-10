/**
 * [INPUT]: 依赖 better-sqlite3 读写 data/*.db, express 提供 HTTP
 * [OUTPUT]: REST API 供前端消费，读取数据 + 编辑人设 + 重置 + 手动唤醒 + 代理实时聊天状态/关闭
 * [POS]: dashboard 后端唯一入口，直连 SQLite，不依赖 Worker 进程
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import express from 'express'
import cors from 'cors'
import Database from 'better-sqlite3'
import { readdirSync, existsSync } from 'fs'
import { join, resolve } from 'path'

const DATA_DIR = resolve(process.env.DATA_DIR || '../data')
const PORT = process.env.PORT || 3001
const ADMIN_CODE = process.env.ADMIN_CODE || ''

function requireAdmin(req, res, next) {
  if (!ADMIN_CODE) return next()
  if (req.headers['x-admin-code'] === ADMIN_CODE) return next()
  res.status(403).json({ error: 'invalid admin code' })
}

const app = express()
app.use(cors())
app.use(express.json())

// ================================================================
//  DB 连接池（只读）
// ================================================================

const conns = new Map()

function getDB(name) {
  if (conns.has(name)) return conns.get(name)
  const dbPath = join(DATA_DIR, `${name}.db`)
  if (!existsSync(dbPath)) return null
  const db = new Database(dbPath, { readonly: true, fileMustExist: true })
  db.pragma('journal_mode = WAL')
  conns.set(name, db)
  return db
}

function scanWorkers() {
  if (!existsSync(DATA_DIR)) return []
  return readdirSync(DATA_DIR)
    .filter(f => f.endsWith('.db') && !f.startsWith('_'))
    .map(f => f.replace('.db', ''))
}

// ================================================================
//  API 路由
// ================================================================

app.get('/api/workers', (req, res) => {
  const names = scanWorkers()
  const list = names.map(name => {
    const db = getDB(name)
    if (!db) return null
    try {
      const soul = db.prepare('SELECT name, occupation, mood, hope, grievance, avatar FROM soul WHERE id = 1').get()
      const pending = db.prepare('SELECT COUNT(*) as c FROM wakeup_schedule WHERE status = ?').get('pending')
      return {
        name,
        occupation: soul?.occupation || '',
        mood: soul?.mood || 0,
        hope: soul?.hope || 0,
        grievance: soul?.grievance || 0,
        avatar: soul?.avatar || '',
        status: pending?.c > 0 ? 'running' : 'stopped'
      }
    } catch { return null }
  }).filter(Boolean)
  res.json(list)
})

app.get('/api/workers/:name', (req, res) => {
  const db = getDB(req.params.name)
  if (!db) return res.status(404).json({ error: 'not found' })
  try {
    const soul = db.prepare('SELECT * FROM soul WHERE id = 1').get()
    const pending = db.prepare('SELECT COUNT(*) as c FROM wakeup_schedule WHERE status = ?').get('pending')
    res.json({ soul, status: pending?.c > 0 ? 'running' : 'stopped' })
  } catch (e) { res.status(500).json({ error: e.message }) }
})

app.get('/api/workers/:name/narratives', (req, res) => {
  const db = getDB(req.params.name)
  if (!db) return res.status(404).json({ error: 'not found' })
  const limit = Math.min(parseInt(req.query.limit) || 50, 200)
  res.json(db.prepare('SELECT * FROM narratives ORDER BY id DESC LIMIT ?').all(limit))
})

app.get('/api/workers/:name/memories', (req, res) => {
  const db = getDB(req.params.name)
  if (!db) return res.status(404).json({ error: 'not found' })
  const limit = Math.min(parseInt(req.query.limit) || 50, 200)
  res.json(db.prepare('SELECT * FROM memories ORDER BY id DESC LIMIT ?').all(limit))
})

app.get('/api/workers/:name/events', (req, res) => {
  const db = getDB(req.params.name)
  if (!db) return res.status(404).json({ error: 'not found' })
  const limit = Math.min(parseInt(req.query.limit) || 50, 200)
  res.json(db.prepare('SELECT * FROM events ORDER BY id DESC LIMIT ?').all(limit))
})

app.get('/api/workers/:name/heartbeats', (req, res) => {
  const db = getDB(req.params.name)
  if (!db) return res.status(404).json({ error: 'not found' })
  const limit = Math.min(parseInt(req.query.limit) || 200, 500)
  res.json(db.prepare('SELECT * FROM heartbeat_schedule ORDER BY date DESC, time DESC LIMIT ?').all(limit))
})

app.get('/api/workers/:name/wakeups', (req, res) => {
  const db = getDB(req.params.name)
  if (!db) return res.status(404).json({ error: 'not found' })
  const limit = Math.min(parseInt(req.query.limit) || 50, 200)
  res.json(db.prepare('SELECT * FROM wakeup_schedule ORDER BY id DESC LIMIT ?').all(limit))
})

app.get('/api/workers/:name/reasoning', (req, res) => {
  const db = getDB(req.params.name)
  if (!db) return res.status(404).json({ error: 'not found' })
  const limit = Math.min(parseInt(req.query.limit) || 100, 500)
  const afterId = parseInt(req.query.after) || 0
  const rows = db.prepare(
    'SELECT * FROM reasoning_logs WHERE id > ? ORDER BY id DESC LIMIT ?'
  ).all(afterId, limit)
  res.json(rows)
})

app.post('/api/workers/:name/wakeup', (req, res) => {
  const dbPath = join(DATA_DIR, `${req.params.name}.db`)
  if (!existsSync(dbPath)) return res.status(404).json({ error: 'not found' })
  const reason = req.body.reason || '手动唤醒'
  const datetime = new Date(Date.now() + 3000).toISOString()
  try {
    if (conns.has(req.params.name)) {
      conns.get(req.params.name).close()
      conns.delete(req.params.name)
    }
    const wdb = new Database(dbPath)
    wdb.prepare('INSERT INTO wakeup_schedule (datetime, reason) VALUES (?, ?)').run(datetime, reason)
    wdb.close()
    res.json({ status: 'wakeup_scheduled', reason })
  } catch (e) { res.status(500).json({ error: e.message }) }
})

// ================================================================
//  对话代理 → Go worker server :8080
// ================================================================

const WORKER_API = process.env.WORKER_API || 'http://127.0.0.1:8080'

app.get('/api/workers/:name/live-status', async (req, res) => {
  try {
    const resp = await fetch(`${WORKER_API}/api/workers/${req.params.name}`)
    if (!resp.ok) return res.status(resp.status).json({ error: 'worker not found' })
    const data = await resp.json()
    res.json({ reasoning: data.reasoning || false, chatting_with: data.chatting_with || '' })
  } catch (e) {
    res.status(502).json({ error: 'worker service unavailable' })
  }
})

app.post('/api/workers/:name/chat', async (req, res) => {
  const { content, conversation_id } = req.body
  if (!content) return res.status(400).json({ error: 'content required' })
  try {
    const resp = await fetch(`${WORKER_API}/api/workers/${req.params.name}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content, conversation_id: conversation_id || '' }),
    })
    const data = await resp.json()
    res.status(resp.status).json(data)
  } catch (e) {
    res.status(502).json({ error: '无法连接 Worker 服务: ' + e.message })
  }
})

app.post('/api/workers/:name/chat/close', async (req, res) => {
  const { conversation_id } = req.body
  if (!conversation_id) return res.status(400).json({ error: 'conversation_id required' })
  try {
    const resp = await fetch(`${WORKER_API}/api/workers/${req.params.name}/chat/close`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ conversation_id }),
    })
    const data = await resp.json()
    res.status(resp.status).json(data)
  } catch (e) {
    res.status(502).json({ error: '无法连接 Worker 服务: ' + e.message })
  }
})

app.put('/api/workers/:name', requireAdmin, (req, res) => {
  const dbPath = join(DATA_DIR, `${req.params.name}.db`)
  if (!existsSync(dbPath)) return res.status(404).json({ error: 'not found' })

  const allowed = ['occupation', 'background', 'personality', 'speech_style', 'values_desc', 'family', 'avatar']
  const updates = req.body
  const fields = Object.keys(updates).filter(k => allowed.includes(k))
  if (!fields.length) return res.status(400).json({ error: 'no valid fields' })

  try {
    // 关闭只读连接
    if (conns.has(req.params.name)) {
      conns.get(req.params.name).close()
      conns.delete(req.params.name)
    }
    const wdb = new Database(dbPath)
    const stmt = fields.map(f => `${f} = ?`).join(', ')
    const values = fields.map(f => updates[f])
    wdb.prepare(`UPDATE soul SET ${stmt} WHERE id = 1`).run(...values)
    wdb.close()
    res.json({ status: 'updated' })
  } catch (e) { res.status(500).json({ error: e.message }) }
})

app.post('/api/workers/:name/reset', requireAdmin, (req, res) => {
  const dbPath = join(DATA_DIR, `${req.params.name}.db`)
  if (!existsSync(dbPath)) return res.status(404).json({ error: 'not found' })
  try {
    // 关闭只读连接
    if (conns.has(req.params.name)) {
      conns.get(req.params.name).close()
      conns.delete(req.params.name)
    }
    // 写模式打开，清除 soul 以外的表
    const wdb = new Database(dbPath)
    wdb.exec(`
      DELETE FROM narratives;
      DELETE FROM memories;
      DELETE FROM events;
      DELETE FROM heartbeat_schedule;
      DELETE FROM wakeup_schedule;
      DELETE FROM reasoning_logs;
    `)
    wdb.close()
    res.json({ status: 'reset' })
  } catch (e) { res.status(500).json({ error: e.message }) }
})

app.post('/api/auth/verify', (req, res) => {
  if (!ADMIN_CODE) return res.json({ ok: true })
  res.json({ ok: req.body.code === ADMIN_CODE })
})

app.listen(PORT, () => {
  console.log(`[dashboard] API: http://localhost:${PORT}  data: ${DATA_DIR}`)
})
