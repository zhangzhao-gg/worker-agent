/**
 * [INPUT]: 依赖 better-sqlite3 直读 data/*.db, express 提供 HTTP
 * [OUTPUT]: REST API 供前端消费，/api/workers + /api/workers/:name/*
 * [POS]: dashboard 后端唯一入口，纯只读，不侵入 Worker Agent
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import express from 'express'
import cors from 'cors'
import Database from 'better-sqlite3'
import { readdirSync, existsSync } from 'fs'
import { join, resolve } from 'path'

const DATA_DIR = resolve(process.env.DATA_DIR || '../data')
const PORT = process.env.PORT || 3001
const WORKER_API = process.env.WORKER_API || 'http://localhost:8080'

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
    .filter(f => f.endsWith('.db'))
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
      const soul = db.prepare('SELECT name, occupation, mood, hope, grievance FROM soul WHERE id = 1').get()
      const pending = db.prepare('SELECT COUNT(*) as c FROM wakeup_schedule WHERE status = ?').get('pending')
      return {
        name,
        occupation: soul?.occupation || '',
        mood: soul?.mood || 0,
        hope: soul?.hope || 0,
        grievance: soul?.grievance || 0,
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
  const limit = Math.min(parseInt(req.query.limit) || 50, 200)
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

// 代理写操作到 Worker API
app.post('/api/workers/:name/wakeup', async (req, res) => {
  try {
    const resp = await fetch(`${WORKER_API}/api/workers/${encodeURIComponent(req.params.name)}/wakeup`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req.body)
    })
    const data = await resp.json()
    res.status(resp.status).json(data)
  } catch (e) { res.status(502).json({ error: e.message }) }
})

app.put('/api/workers/:name', async (req, res) => {
  try {
    const resp = await fetch(`${WORKER_API}/api/workers/${encodeURIComponent(req.params.name)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req.body)
    })
    const data = await resp.json()
    res.status(resp.status).json(data)
  } catch (e) { res.status(502).json({ error: e.message }) }
})

app.get('/api/config', (req, res) => {
  res.json({ workerAPI: WORKER_API })
})

app.listen(PORT, () => {
  console.log(`[dashboard] API: http://localhost:${PORT}  data: ${DATA_DIR}`)
})
