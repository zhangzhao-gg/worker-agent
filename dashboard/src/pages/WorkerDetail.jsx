import { useState, useEffect, useRef, useMemo, useCallback } from 'react'
import styles from './WorkerDetail.module.css'

const TABS = ['narrative', 'memory', 'event', 'schedule', 'wakeup', 'reasoning']
const TAB_LABELS = { narrative: '对外叙事', memory: '私人记忆', event: '城市事件', schedule: '心跳计划', wakeup: '唤醒计划', reasoning: '推理日志' }

export default function WorkerDetail({ name, onBack }) {
  const [tab, setTab] = useState('narrative')
  const [worker, setWorker] = useState(null)
  const [wakeupReason, setWakeupReason] = useState('')
  const [msg, setMsg] = useState('')
  const [editing, setEditing] = useState(false)
  const [editForm, setEditForm] = useState({})
  const [adminCode, setAdminCode] = useState('')
  const [adminVerified, setAdminVerified] = useState(false)
  const [codeInput, setCodeInput] = useState('')

  useEffect(() => {
    fetch(`/api/workers/${name}`).then(r => r.json()).then(setWorker).catch(() => {})
    const timer = setInterval(() => {
      fetch(`/api/workers/${name}`).then(r => r.json()).then(setWorker).catch(() => {})
    }, 15000)
    return () => clearInterval(timer)
  }, [name])

  async function verifyCode() {
    try {
      const resp = await fetch('/api/auth/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: codeInput })
      })
      const data = await resp.json()
      if (data.ok) {
        setAdminCode(codeInput)
        setAdminVerified(true)
        setMsg('验证通过')
      } else {
        setMsg('验证码错误')
      }
    } catch { setMsg('验证失败') }
    setTimeout(() => setMsg(''), 3000)
  }

  async function doWakeup() {
    const reason = wakeupReason || '手动唤醒'
    try {
      await fetch(`/api/workers/${name}/wakeup`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reason })
      })
      setMsg('唤醒已安排')
      setWakeupReason('')
      setTab('wakeup')
    } catch (e) { setMsg('失败: ' + e.message) }
    setTimeout(() => setMsg(''), 3000)
  }

  function startEdit() {
    const s = worker?.soul || {}
    setEditForm({
      occupation: s.occupation || '',
      background: s.background || '',
      personality: s.personality || '',
      speech_style: s.speech_style || '',
      values_desc: s.values_desc || '',
      family: s.family || ''
    })
    setEditing(true)
  }

  async function saveEdit() {
    try {
      const resp = await fetch(`/api/workers/${name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'x-admin-code': adminCode },
        body: JSON.stringify(editForm)
      })
      if (resp.ok) {
        setEditing(false)
        setMsg('Dossier updated')
        fetch(`/api/workers/${name}`).then(r => r.json()).then(setWorker)
      } else {
        setMsg('失败: ' + await resp.text())
      }
    } catch (e) { setMsg('失败: ' + e.message) }
    setTimeout(() => setMsg(''), 3000)
  }

  async function doReset() {
    if (!confirm('确定要重置？将清除该工人除人设以外的所有数据（叙事、记忆、事件、心跳、唤醒、推理日志）。')) return
    try {
      const resp = await fetch(`/api/workers/${name}/reset`, {
        method: 'POST',
        headers: { 'x-admin-code': adminCode }
      })
      if (resp.ok) {
        location.reload()
      } else {
        setMsg('失败: ' + await resp.text())
        setTimeout(() => setMsg(''), 3000)
      }
    } catch (e) {
      setMsg('失败: ' + e.message)
      setTimeout(() => setMsg(''), 3000)
    }
  }

  if (!worker) return <div className={styles.loading}>Loading...</div>

  const soul = worker.soul || {}

  return (
    <div className={styles.container}>
      <nav className={styles.nav}>
        <div className={styles.logo}>WORKER ARCHIVE</div>
        <div className={styles.navLinks}>
          <a onClick={onBack}>CATALOGUE</a>
          <span className={styles.navActive}>ENTRIES</span>
        </div>
      </nav>

      <main className={styles.main}>
        <header className={styles.header}>
          <div>
            <span className={styles.label}>WORKER DOSSIER · {worker.status}</span>
            <h1 className={styles.title}>Dossier: {soul.name || name}</h1>
          </div>
          <div className={styles.headerActions}>
            <input
              value={wakeupReason}
              onChange={e => setWakeupReason(e.target.value)}
              placeholder="唤醒原因..."
              className={styles.headerInput}
              onKeyDown={e => e.key === 'Enter' && doWakeup()}
            />
            <button onClick={doWakeup} className={styles.backBtn}>唤醒</button>
            {adminVerified && <button className={styles.resetBtn} onClick={doReset}>重置</button>}
            <button className={styles.backBtn} onClick={onBack}>&larr; 返回</button>
          </div>
        </header>

        <div className={styles.layout}>
          {/* 左栏 */}
          <aside className={styles.sidebar}>
            <div className={styles.avatarSection}>
              <div className={styles.avatarLg}>
                {soul.avatar ? <img src={soul.avatar} className={styles.avatarImg} /> : (soul.name || name)[0]?.toUpperCase()}
              </div>
              <h2 className={styles.soulName}>{soul.name}</h2>
              <div className={styles.occupation}>{soul.occupation}</div>
              <div className={styles.statusBar}>Status: {worker.status}</div>
            </div>

            <div className={styles.section}>
              <h3 className={styles.sectionTitle}>VITAL METRICS</h3>
              <Meter label="心情" value={soul.mood} />
              <Meter label="希望" value={soul.hope} />
              <Meter label="不满" value={soul.grievance} />
            </div>

            <div className={styles.section}>
              <div className={styles.sectionHeader}>
                <h3 className={styles.sectionTitle}>人物档案</h3>
                {!editing && adminVerified && <button className={styles.btnSmall} onClick={startEdit}>编辑</button>}
              </div>
              {editing ? (
                <div className={styles.editForm}>
                  <EditField label="OCCUPATION" value={editForm.occupation} onChange={v => setEditForm(f => ({ ...f, occupation: v }))} />
                  <EditField label="背景故事" value={editForm.background} onChange={v => setEditForm(f => ({ ...f, background: v }))} textarea />
                  <EditField label="PERSONALITY" value={editForm.personality} onChange={v => setEditForm(f => ({ ...f, personality: v }))} textarea />
                  <EditField label="SPEECH STYLE" value={editForm.speech_style} onChange={v => setEditForm(f => ({ ...f, speech_style: v }))} textarea />
                  <EditField label="VALUES" value={editForm.values_desc} onChange={v => setEditForm(f => ({ ...f, values_desc: v }))} textarea />
                  <EditField label="FAMILY" value={editForm.family} onChange={v => setEditForm(f => ({ ...f, family: v }))} textarea />
                  <div className={styles.editActions}>
                    <button className={styles.btn} onClick={saveEdit}>保存</button>
                    <button className={styles.btnMuted} onClick={() => setEditing(false)}>取消</button>
                  </div>
                </div>
              ) : (
                <>
                  {soul.occupation && <Field label="OCCUPATION" value={soul.occupation} />}
                  {soul.background && <Field label="背景故事" value={soul.background} />}
                  {soul.personality && <Field label="PERSONALITY" value={soul.personality} />}
                  {soul.speech_style && <Field label="SPEECH STYLE" value={soul.speech_style} />}
                  {soul.values_desc && <Field label="VALUES" value={soul.values_desc} />}
                  {soul.family && <Field label="FAMILY" value={soul.family} />}
                </>
              )}
            </div>


            <div className={styles.section}>
              <h3 className={styles.sectionTitle}>管理员验证</h3>
              {adminVerified ? (
                <div className={styles.adminOk}>VERIFIED</div>
              ) : (
                <div className={styles.wakeupRow}>
                  <input
                    type="password"
                    value={codeInput}
                    onChange={e => setCodeInput(e.target.value)}
                    placeholder="安全码..."
                    className={styles.input}
                    onKeyDown={e => e.key === 'Enter' && verifyCode()}
                  />
                  <button onClick={verifyCode} className={styles.btn}>VERIFY</button>
                </div>
              )}
              {msg && <div className={styles.msg}>{msg}</div>}
            </div>
          </aside>

          {/* 右栏 */}
          <div className={styles.content}>
            <div className={styles.tabs}>
              {TABS.map(t => (
                <button key={t} className={`${styles.tab} ${tab === t ? styles.tabActive : ''}`} onClick={() => setTab(t)}>
                  {TAB_LABELS[t]}
                </button>
              ))}
            </div>
            <TabContent name={name} tab={tab} />
          </div>
        </div>
      </main>
    </div>
  )
}

// ================================================================
//  标签页内容（独立组件，各自管理轮询）
// ================================================================

function TabContent({ name, tab }) {
  if (tab === 'reasoning') return <ReasoningTab name={name} />
  if (tab === 'memory') return <MemoryTab name={name} />
  return <GenericTab name={name} tab={tab} />
}

function MemoryTab({ name }) {
  const [data, setData] = useState([])
  const [revealed, setRevealed] = useState(new Set())

  useEffect(() => {
    const load = () => fetch(`/api/workers/${name}/memories`).then(r => r.json()).then(setData).catch(() => {})
    load()
    const timer = setInterval(load, 15000)
    return () => clearInterval(timer)
  }, [name])

  if (!data.length) return <p className={styles.empty}>暂无数据</p>

  const toggle = (id) => {
    setRevealed(prev => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  return (
    <div className={styles.parchmentWrap}>
      <div className={styles.logList}>
        {data.map((item, i) => (
          <article key={item.id} className={styles.logItem} style={{ animationDelay: `${i * 40}ms` }} onClick={() => toggle(item.id)}>
            <div className={styles.logMeta}>
              <span>{fmtTime(item.timestamp)}</span>
            </div>
            <p className={`${styles.logContent} ${revealed.has(item.id) ? '' : styles.blurred}`}>{item.content}</p>
          </article>
        ))}
      </div>
    </div>
  )
}

function GenericTab({ name, tab }) {
  const [data, setData] = useState([])
  const endpointMap = { narrative: 'narratives', memory: 'memories', event: 'events', schedule: 'heartbeats', wakeup: 'wakeups' }

  useEffect(() => {
    const load = () => fetch(`/api/workers/${name}/${endpointMap[tab]}`).then(r => r.json()).then(setData).catch(() => {})
    load()
    const timer = setInterval(load, 15000)
    return () => clearInterval(timer)
  }, [name, tab])

  if (!data.length) return <p className={styles.empty}>暂无数据</p>

  if (tab === 'schedule') return <ScheduleTable data={data} />
  if (tab === 'wakeup') return <WakeupTable data={data} />

  return (
    <div className={styles.parchmentWrap}>
      <div className={styles.logList}>
        {data.map((item, i) => (
          <article key={item.id} className={styles.logItem} style={{ animationDelay: `${i * 40}ms` }}>
            <div className={styles.logMeta}>
              <span>{fmtTime(item.timestamp || item.datetime)}</span>
              {item.type && <span className={styles.badge}>{item.type}</span>}
              {item.processed !== undefined && (
                <span className={item.processed ? styles.badgeDone : styles.badgeWarn}>
                  {item.processed ? '已处理' : '未处理'}
                </span>
              )}
            </div>
            <p className={styles.logContent}>{item.content}</p>
          </article>
        ))}
      </div>
    </div>
  )
}

// ================================================================
//  Reasoning Tab — 按 session 分组，左列表右详情
// ================================================================

function ReasoningTab({ name }) {
  const [logs, setLogs] = useState([])
  const [selectedSession, setSelectedSession] = useState(null)
  const detailRef = useRef(null)

  useEffect(() => {
    const load = () => fetch(`/api/workers/${name}/reasoning?limit=500`)
      .then(r => r.json())
      .then(rows => setLogs(rows.sort((a, b) => a.id - b.id)))
      .catch(() => {})
    load()
    const timer = setInterval(load, 5000)
    return () => clearInterval(timer)
  }, [name])

  const sessions = useMemo(() => {
    const map = new Map()
    for (const log of logs) {
      if (!map.has(log.session_id)) {
        map.set(log.session_id, { id: log.session_id, startTime: log.timestamp, rounds: log.round, logs: [] })
      }
      const s = map.get(log.session_id)
      s.logs.push(log)
      if (log.round > s.rounds) s.rounds = log.round
    }
    return [...map.values()].reverse()
  }, [logs])

  // 自动选中最新 session
  useEffect(() => {
    if (!selectedSession && sessions.length) setSelectedSession(sessions[0].id)
  }, [sessions, selectedSession])

  const activeLogs = useMemo(() => {
    const s = sessions.find(s => s.id === selectedSession)
    return s ? s.logs : []
  }, [sessions, selectedSession])

  function copySession(logs) {
    const text = logs.map(l => `[R${l.round}] [${l.type}] ${l.timestamp}\n${l.content}`).join('\n\n---\n\n')
    navigator.clipboard.writeText(text)
  }

  return (
    <div className={styles.reasoningContainer}>
      {/* 左侧 session 列表 */}
      <div className={styles.sessionList}>
        <div className={styles.sessionListHeader}>推理会话 ({sessions.length})</div>
        <div className={styles.sessionListScroll}>
          {sessions.map(s => (
            <div
              key={s.id}
              className={`${styles.sessionItem} ${selectedSession === s.id ? styles.sessionActive : ''}`}
              onClick={() => setSelectedSession(s.id)}
            >
              <div className={styles.sessionTime}>{fmtTime(s.startTime)}</div>
              <div className={styles.sessionMeta}>
                <span>{s.rounds} 轮</span>
                <span>{s.logs.length} 条</span>
              </div>
              <div className={styles.sessionId}>{s.id.slice(0, 8)}...</div>
            </div>
          ))}
          {!sessions.length && <p className={styles.empty}>暂无推理记录</p>}
        </div>
      </div>

      {/* 右侧详情 */}
      <div className={styles.sessionDetail}>
        {activeLogs.length ? (
          <>
            <div className={styles.sessionDetailHeader}>
              <button className={styles.copyBtn} onClick={() => copySession(activeLogs)}>复制全部</button>
            </div>
            <div ref={detailRef} className={styles.sessionDetailScroll}>
              {activeLogs.map(log => (
                <div key={log.id} className={`${styles.reasoningItem} ${styles[`type_${log.type}`] || ''}`}>
                  <div className={styles.logMeta}>
                    <span>{fmtTime(log.timestamp)}</span>
                    <span className={styles.badge}>R{log.round}</span>
                    <span className={`${styles.badge} ${styles[`badge_${log.type}`] || ''}`}>{log.type}</span>
                  </div>
                  <pre className={styles.reasoningContent}>{log.content}</pre>
                </div>
              ))}
            </div>
          </>
        ) : (
          <p className={styles.empty}>选择一个 session 查看推理链路</p>
        )}
      </div>
    </div>
  )
}

// ================================================================
//  子组件
// ================================================================

function ScheduleTable({ data }) {
  const PAGE_SIZE = 30
  const [page, setPage] = useState(0)
  const totalPages = Math.ceil(data.length / PAGE_SIZE)
  const slice = data.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)

  return (
    <div className={styles.parchmentWrap}>
      <table className={styles.table}>
        <thead><tr><th>日期</th><th>时间</th><th>任务</th><th>状态</th></tr></thead>
        <tbody>
          {slice.map((r, i) => (
            <tr key={r.id} style={{ animationDelay: `${i * 30}ms` }}>
              <td>{r.date}</td><td>{r.time}</td><td>{r.task}</td>
              <td><span className={`${styles.badge} ${r.status === 'done' ? styles.badgeDone : styles.badgeWarn}`}>{r.status}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
      {totalPages > 1 && (
        <Pagination page={page} totalPages={totalPages} onChange={setPage} />
      )}
    </div>
  )
}

function WakeupTable({ data }) {
  const PAGE_SIZE = 20
  const [page, setPage] = useState(0)
  const totalPages = Math.ceil(data.length / PAGE_SIZE)
  const slice = data.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)

  return (
    <div className={styles.parchmentWrap}>
      <table className={styles.table}>
        <thead><tr><th>时间</th><th>原因</th><th>状态</th></tr></thead>
        <tbody>
          {slice.map((r, i) => (
            <tr key={r.id} style={{ animationDelay: `${i * 30}ms` }}>
              <td>{fmtTime(r.datetime)}</td>
              <td><span className={styles.reasonCell} title={r.reason}>{r.reason}</span></td>
              <td><span className={`${styles.badge} ${r.status === 'done' ? styles.badgeDone : styles.badgeWarn}`}>{r.status}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
      {totalPages > 1 && (
        <Pagination page={page} totalPages={totalPages} onChange={setPage} />
      )}
    </div>
  )
}

function Pagination({ page, totalPages, onChange }) {
  return (
    <div className={styles.pagination}>
      <button className={styles.pageBtn} disabled={page === 0} onClick={() => onChange(page - 1)}>&laquo; 上一页</button>
      <span className={styles.pageInfo}>{page + 1} / {totalPages}</span>
      <button className={styles.pageBtn} disabled={page >= totalPages - 1} onClick={() => onChange(page + 1)}>下一页 &raquo;</button>
    </div>
  )
}

function Meter({ label, value }) {
  return (
    <div className={styles.meter}>
      <div className={styles.meterLabel}><span>{label}</span><span>{value}</span></div>
      <div className={styles.meterBar}><div className={styles.meterFill} style={{ width: `${Math.min(100, Math.max(0, value))}%` }} /></div>
    </div>
  )
}

function Field({ label, value }) {
  return (
    <div className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <p>{value}</p>
    </div>
  )
}

function EditField({ label, value, onChange, textarea }) {
  return (
    <div className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      {textarea ? (
        <textarea className={styles.textarea} value={value} onChange={e => onChange(e.target.value)} rows={3} />
      ) : (
        <input className={styles.input} value={value} onChange={e => onChange(e.target.value)} />
      )}
    </div>
  )
}

function fmtTime(s) {
  if (!s) return ''
  try {
    const d = new Date(s)
    return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' }) + ', ' + d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
  } catch { return s }
}
