import { useState, useEffect, useRef, useMemo, useCallback } from 'react'
import styles from './WorkerDetail.module.css'

const TABS = ['narrative', 'memory', 'event', 'schedule', 'wakeup', 'reasoning']
const TAB_LABELS = { narrative: '日记', memory: '内心OS', event: '见闻', schedule: '日程', wakeup: '思考计划', reasoning: '思维链' }

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
  const [showDetail, setShowDetail] = useState(false)

  useEffect(() => {
    fetch(`/api/workers/${name}`).then(r => r.json()).then(setWorker).catch(() => {})
    const timer = setInterval(() => {
      fetch(`/api/workers/${name}`).then(r => r.json()).then(setWorker).catch(() => {})
    }, 15000)
    return () => clearInterval(timer)
  }, [name])

  if (!worker) return <div className={styles.loading}>Loading...</div>

  if (!showDetail) {
    return (
      <VisitorView
        name={name}
        worker={worker}
        onBack={onBack}
        onShowDetail={() => setShowDetail(true)}
        codeInput={codeInput}
        setCodeInput={setCodeInput}
        onVerify={async () => {
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
              setShowDetail(true)
              setMsg('验证通过')
            } else {
              setMsg('验证码错误')
            }
          } catch { setMsg('验证失败') }
          setTimeout(() => setMsg(''), 3000)
        }}
        msg={msg}
        setMsg={setMsg}
      />
    )
  }

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
            <button className={styles.backBtn} onClick={() => setShowDetail(false)}>简要视图</button>
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
//  访客视图 — 一屏角色状态卡
// ================================================================

function VisitorView({ name, worker, onBack, onShowDetail, codeInput, setCodeInput, onVerify, msg, setMsg }) {
  const soul = worker.soul || {}
  const [heartbeats, setHeartbeats] = useState([])
  const [narratives, setNarratives] = useState([])
  const [events, setEvents] = useState([])
  const [wakeups, setWakeups] = useState([])
  const [wakeupReason, setWakeupReason] = useState('')
  const [wakeupMsg, setWakeupMsg] = useState('')
  const [showLockModal, setShowLockModal] = useState(false)

  const [tick, setTick] = useState(Date.now())

  useEffect(() => {
    const load = () => {
      fetch(`/api/workers/${name}/heartbeats`).then(r => r.json()).then(setHeartbeats).catch(() => {})
      fetch(`/api/workers/${name}/narratives`).then(r => r.json()).then(setNarratives).catch(() => {})
      fetch(`/api/workers/${name}/events`).then(r => r.json()).then(setEvents).catch(() => {})
      fetch(`/api/workers/${name}/wakeups`).then(r => r.json()).then(setWakeups).catch(() => {})
      setTick(Date.now())
    }
    load()
    const timer = setInterval(load, 15000)
    return () => clearInterval(timer)
  }, [name])

  // ------ 计算当前状态 ------
  const now = tick
  const twentyMinAgo = now - 20 * 60 * 1000

  const currentActivity = useMemo(() => {
    const recent = heartbeats.find(h => {
      const t = new Date(h.date + 'T' + h.time).getTime()
      return t >= twentyMinAgo && t <= now
    })
    return recent ? recent.task : null
  }, [heartbeats, tick])

  const nextPlan = useMemo(() => {
    const future = heartbeats.filter(h => {
      const t = new Date(h.date + 'T' + h.time).getTime()
      return t > now
    }).sort((a, b) => {
      const ta = new Date(a.date + 'T' + a.time).getTime()
      const tb = new Date(b.date + 'T' + b.time).getTime()
      return ta - tb
    })
    if (!future.length) return null
    const next = future[0]
    const nextTime = new Date(next.date + 'T' + next.time).getTime()
    const diffMin = Math.round((nextTime - now) / 60000)
    if (diffMin > 180) {
      // 超过3小时，找明天最早的
      const earliest = future[0]
      return { type: 'rest', time: earliest.time, task: earliest.task }
    }
    return { type: 'upcoming', minutes: diffMin, task: next.task }
  }, [heartbeats, tick])

  const nextWakeup = useMemo(() => {
    const pending = wakeups.filter(w => w.status === 'pending')
    if (!pending.length) return null
    pending.sort((a, b) => new Date(a.datetime) - new Date(b.datetime))
    const future = pending.find(w => new Date(w.datetime) > new Date())
    return future || pending[0]
  }, [wakeups])

  // ------ 状态动画类型 ------
  const statusType = useMemo(() => {
    if (worker.status === 'thinking' || worker.status === 'reasoning') return 'thinking'
    if (currentActivity) return 'active'
    return 'resting'
  }, [worker.status, currentActivity])

  // ------ 氛围：色温 + 时间段 ------
  const ambience = useMemo(() => {
    const mood = Math.min(100, Math.max(0, soul.mood || 50))
    const hour = new Date().getHours()
    let timeOfDay = 'day'
    if (hour >= 5 && hour < 8) timeOfDay = 'dawn'
    else if (hour >= 8 && hour < 17) timeOfDay = 'day'
    else if (hour >= 17 && hour < 20) timeOfDay = 'dusk'
    else timeOfDay = 'night'

    const warmth = mood / 100
    // mood 低 → 冷灰偏暗；mood 高 → 暖黄明亮
    const surfaceH = Math.round(30 + warmth * 10)
    const surfaceS = Math.round(5 + warmth * 25)
    const surfaceL = Math.round(85 + warmth * 8)
    const surface = `hsl(${surfaceH}, ${surfaceS}%, ${surfaceL}%)`

    const paperH = Math.round(33 + warmth * 8)
    const paperS = Math.round(15 + warmth * 30)
    const paperL = Math.round(65 + warmth * 12)
    const paper = `hsl(${paperH}, ${paperS}%, ${paperL}%)`

    const shadowAlpha = 0.12 + (1 - warmth) * 0.12
    const shadow = `rgba(26, 20, 16, ${shadowAlpha.toFixed(2)})`

    return { surface, paper, shadow, timeOfDay }
  }, [soul.mood])

  const [isThinking, setIsThinking] = useState(false)
  const [thinkingStarted, setThinkingStarted] = useState(false)
  const [thinkingSteps, setThinkingSteps] = useState([])
  const [thinkingResult, setThinkingResult] = useState(null)
  const [manualWakeup, setManualWakeup] = useState(false)
  const [shaking, setShaking] = useState(false)
  const baselineIdRef = useRef(0)
  const [detailModal, setDetailModal] = useState(null)

  const TOOL_LABELS = {
    get_city_temperature: '正在感知天气...',
    get_food_status: '正在了解食物供应...',
    get_city_announcements: '正在查看公告...',
    get_my_work_assignment: '正在确认工作安排...',
    get_recent_events: '正在回忆最近发生的事...',
    get_memories: '正在回忆过去的想法...',
    write_heartbeat_schedule: '正在安排今天的日程...',
    update_heartbeat_schedule: '正在调整日程...',
    schedule_wakeup: '正在安排下次思考时间...',
    write_narrative: '正在写日记...',
    write_memory: '正在记录内心想法...',
    update_soul: '正在调整情绪状态...',
    cancel_wakeup: '正在取消计划...',
    TodoWrite: '正在整理思路...',
  }

  function extractToolName(content) {
    if (!content) return null
    const m = content.match(/"name"\s*:\s*"([^"]+)"/) || content.match(/^(\w+)\(/)
    return m ? m[1] : null
  }

  // 持续轮询 reasoning，自动检测是否在思考
  useEffect(() => {
    const poll = setInterval(() => {
      fetch(`/api/workers/${name}/reasoning?limit=100`)
        .then(r => r.json())
        .then(rows => {
          if (!rows.length) return
          const sorted = rows.sort((a, b) => a.id - b.id)

          let targetLogs
          if (manualWakeup) {
            targetLogs = sorted.filter(r => r.id > baselineIdRef.current)
            if (!targetLogs.length) return
          } else {
            const lastLog = sorted[sorted.length - 1]
            targetLogs = sorted.filter(r => r.session_id === lastLog.session_id)
          }

          if (!targetLogs.length) return
          const lastEntry = targetLogs[targetLogs.length - 1]
          const finished = lastEntry.type === 'finish'

          if (!finished) {
            setIsThinking(true)
            setThinkingStarted(true)
            // 提取人话步骤
            const steps = []
            for (const log of targetLogs) {
              if (log.type === 'tool_call') {
                const toolName = extractToolName(log.content)
                const label = TOOL_LABELS[toolName] || `正在执行 ${toolName}...`
                if (!steps.length || steps[steps.length - 1] !== label) steps.push(label)
              }
            }
            if (!steps.length) steps.push('正在思考...')
            setThinkingSteps(steps.slice(-3))
            setThinkingResult(null)
          } else {
            // 思考完成 — 提取结果
            if (isThinking || manualWakeup) {
              const result = { narrative: null, memory: null, moodChange: null, nextWakeup: null }
              for (const log of targetLogs) {
                if (log.type === 'tool_call') {
                  const toolName = extractToolName(log.content)
                  if (toolName === 'write_narrative') {
                    const m = log.content.match(/"text"\s*:\s*"([^"]*(?:\\.[^"]*)*)"/)
                    if (m) result.narrative = m[1].replace(/\\n/g, '\n').replace(/\\"/g, '"')
                  }
                  if (toolName === 'write_memory') {
                    const m = log.content.match(/"text"\s*:\s*"([^"]*(?:\\.[^"]*)*)"/)
                    if (m) result.memory = m[1].replace(/\\n/g, '\n').replace(/\\"/g, '"')
                  }
                  if (toolName === 'update_soul') result.moodChange = true
                  if (toolName === 'schedule_wakeup') {
                    const m = log.content.match(/"reason"\s*:\s*"([^"]*)"/)
                    if (m) result.nextWakeup = m[1]
                  }
                }
              }
              setThinkingResult(result)
              setIsThinking(false)
              setManualWakeup(false)
              // 刷新主数据
              fetch(`/api/workers/${name}/narratives`).then(r => r.json()).then(setNarratives).catch(() => {})
              fetch(`/api/workers/${name}/wakeups`).then(r => r.json()).then(setWakeups).catch(() => {})
            }
          }
        })
        .catch(() => {})
    }, 2000)
    return () => clearInterval(poll)
  }, [name, manualWakeup, isThinking])

  async function doVisitorWakeup() {
    const reason = wakeupReason || '访客唤醒'
    try {
      const rows = await fetch(`/api/workers/${name}/reasoning?limit=1`).then(r => r.json())
      baselineIdRef.current = rows.length ? Math.max(...rows.map(r => r.id)) : 0

      await fetch(`/api/workers/${name}/wakeup`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reason })
      })
      setWakeupReason('')
      setManualWakeup(true)
      setIsThinking(true)
      setThinkingStarted(false)
      setThinkingSteps([])
      setThinkingResult(null)
      setShaking(true)
      setTimeout(() => setShaking(false), 500)
    } catch (e) { setWakeupMsg('失败: ' + e.message); setTimeout(() => setWakeupMsg(''), 3000) }
  }

  return (
    <div
      className={styles.visitorContainer}
      data-time={ambience.timeOfDay}
      style={{
        '--amb-surface': ambience.surface,
        '--amb-paper': ambience.paper,
        '--amb-shadow': ambience.shadow,
      }}
    >
      {/* 窗户光效 */}
      <div className={styles.windowLight} data-time={ambience.timeOfDay} />

      {/* 隐蔽管理员入口 */}
      <button className={styles.lockIcon} onClick={() => setShowLockModal(true)} title="管理员">
        &#x1f512;
      </button>

      {/* 密码浮层 */}
      {showLockModal && (
        <div className={styles.lockOverlay} onClick={() => setShowLockModal(false)}>
          <div className={styles.lockModal} onClick={e => e.stopPropagation()}>
            <div className={styles.lockModalTitle}>ADMIN VERIFY</div>
            <div className={styles.wakeupRow}>
              <input
                type="password"
                value={codeInput}
                onChange={e => setCodeInput(e.target.value)}
                placeholder="安全码..."
                className={styles.input}
                onKeyDown={e => e.key === 'Enter' && onVerify()}
                autoFocus
              />
              <button onClick={onVerify} className={styles.btn}>VERIFY</button>
            </div>
            {msg && <div className={styles.msg}>{msg}</div>}
          </div>
        </div>
      )}

      {/* 返回按钮 */}
      <button className={styles.visitorBack} onClick={onBack}>&larr; 返回</button>

      {/* 桌面 */}
      <div className={`${styles.desk} ${isThinking ? styles.deskDimmed : ''} ${shaking ? styles.deskShake : ''}`}>
        {/* 身份纸片 */}
        <div className={`${styles.paper} ${styles.identityPaper}`} style={{ transform: 'rotate(-3deg) translate(-6px, 4px)' }}>
          <div className={styles.visitorAvatar}>
            {soul.avatar ? <img src={soul.avatar} className={styles.avatarImg} /> : (soul.name || name)[0]?.toUpperCase()}
          </div>
          <div className={styles.identityInfo}>
            <h1 className={styles.visitorName}>{soul.name || name}</h1>
            <div className={styles.visitorOccupation}>{soul.occupation}</div>
            <div className={styles.visitorMoodNarrative}>{moodNarrative(soul)}</div>
          </div>
        </div>

        {/* 状态纸片 */}
        <div className={`${styles.paper} ${styles.statusPaper}`} style={{ transform: 'rotate(2.5deg) translate(8px, 5px)' }}>
          <div className={styles.visitorStatus}>
            <span className={`${styles.candle} ${styles[`candle_${statusType}`]}`}>
              {statusType === 'thinking' ? '✎' : '🕯'}
            </span>
            <span className={styles.statusText}>
              {currentActivity ? currentActivity : '休息中'}
            </span>
            {statusType === 'active' && <span className={styles.ellipsis} />}
          </div>
          {nextPlan && (
            <div className={styles.visitorNext}>
              {nextPlan.type === 'upcoming'
                ? `${nextPlan.minutes} 分钟后 — ${nextPlan.task}`
                : `休息中，明日 ${nextPlan.time} 起床`
              }
            </div>
          )}
          {nextWakeup && (
            <div className={styles.visitorWakeupInfo}>
              <span className={styles.visitorInfoLabel}>下次思考</span>
              <span>{fmtTime(nextWakeup.datetime)} — {nextWakeup.reason}</span>
            </div>
          )}
        </div>

        {/* 日记纸片 */}
        <div className={`${styles.paper} ${styles.diaryPaper}`} style={{ transform: 'rotate(2deg) translate(4px, -6px)' }} onClick={() => narratives.length && setDetailModal({ title: '日记', content: narratives[0].content })}>
          <span className={styles.paperLabel}>日记</span>
          {narratives.length ? (
            <>
              <div className={styles.diaryContent}>{narratives[0].content}</div>
              <div className={styles.diaryDate}>{fmtTime(narratives[0].timestamp)}</div>
            </>
          ) : (
            <div className={styles.empty}>还没有写过日记</div>
          )}
        </div>

        {/* 见闻纸片 */}
        <div className={`${styles.paper} ${styles.eventsPaper}`} style={{ transform: 'rotate(-3.5deg) translate(-8px, 5px)' }}>
          <span className={styles.paperLabel}>最近见闻</span>
          {events.length ? (
            events.slice(0, 3).map((ev, i) => (
              <div key={ev.id || i} className={styles.eventItem} onClick={() => setDetailModal({ title: '见闻', content: ev.content })}>
                {ev.content.length > 60 ? ev.content.slice(0, 60) + '...' : ev.content}
              </div>
            ))
          ) : (
            <div className={styles.empty}>暂无见闻</div>
          )}
        </div>

        {/* 唤醒区 */}
        <div className={`${styles.paper} ${styles.wakeupPaper}`} style={{ transform: 'rotate(-2deg) translate(-5px, -3px)' }}>
          <span className={styles.paperLabel}>轻敲桌面唤醒 TA</span>
          <div className={styles.visitorWakeupAction}>
            <input
              value={wakeupReason}
              onChange={e => setWakeupReason(e.target.value)}
              placeholder="说点什么..."
              className={styles.visitorWakeupInput}
              onKeyDown={e => e.key === 'Enter' && doVisitorWakeup()}
              disabled={isThinking}
            />
            <button onClick={doVisitorWakeup} className={styles.btn} disabled={isThinking}>
              {isThinking ? '思考中' : '唤醒'}
            </button>
          </div>
          {wakeupMsg && <div className={styles.msg}>{wakeupMsg}</div>}
        </div>

      </div>

      {/* 思考面板 — 浮于桌面之上 */}
      {(isThinking || thinkingResult) && (
        <div className={styles.thinkingPanel}>
          {isThinking ? (
            <>
              <div className={styles.thinkingHeader}>
                <span className={`${styles.statusDot} ${styles.dot_thinking}`} />
                {thinkingStarted ? (
                  <span className={styles.waveText}>
                    {'正在思考'.split('').map((ch, i) => (
                      <span key={i} style={{ animationDelay: `${i * 0.15}s` }}>{ch}</span>
                    ))}
                  </span>
                ) : <span>等待唤醒...</span>}
              </div>
              <div className={styles.thinkingSteps}>
                {thinkingSteps.map((step, i) => (
                  <div key={i} className={`${styles.thinkingStep} ${i === thinkingSteps.length - 1 ? styles.stepActive : styles.stepDone}`}>
                    {step}
                  </div>
                ))}
                <div className={styles.thinkingCursor} />
              </div>
            </>
          ) : thinkingResult && (
            <>
              <div className={styles.thinkingHeader}>
                <span className={styles.statusDot} style={{ background: '#22c55e' }} />
                <span>思考完成</span>
              </div>
              <div className={styles.thinkingResultBody}>
                {thinkingResult.narrative && (
                  <div className={styles.resultItem}>
                    <span className={styles.resultLabel}>写了日记</span>
                    <p className={styles.resultText}>{thinkingResult.narrative}</p>
                  </div>
                )}
                {thinkingResult.memory && (
                  <div className={styles.resultItem}>
                    <span className={styles.resultLabel}>内心想法</span>
                    <p className={styles.resultText}>{thinkingResult.memory}</p>
                  </div>
                )}
                {thinkingResult.moodChange && <div className={styles.resultMeta}>情绪有所变化</div>}
                {thinkingResult.nextWakeup && <div className={styles.resultMeta}>安排了下次思考：{thinkingResult.nextWakeup}</div>}
              </div>
            </>
          )}
        </div>
      )}

      {/* 详情模态框 */}
      {detailModal && (
        <div className={styles.lockOverlay} onClick={() => setDetailModal(null)}>
          <div className={styles.detailModal} onClick={e => e.stopPropagation()}>
            <div className={styles.lockModalTitle}>{detailModal.title}</div>
            <div className={styles.detailContent}>{detailModal.content}</div>
            <button className={styles.btn} onClick={() => setDetailModal(null)}>关闭</button>
          </div>
        </div>
      )}

      {/* 了解更多 */}
      <button className={styles.visitorMoreBtn} onClick={onShowDetail}>
        了解更多关于 {soul.name || name} 的信息 &rarr;
      </button>
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

function Meter({ label, value, color }) {
  return (
    <div className={styles.meter}>
      <div className={styles.meterLabel}><span>{label}</span><span>{value}</span></div>
      <div className={styles.meterBar}><div className={styles.meterFill} style={{ width: `${Math.min(100, Math.max(0, value))}%`, background: color || 'var(--accent)' }} /></div>
    </div>
  )
}

function moodNarrative(soul) {
  const pick = (v, opts) => { const n = Math.min(100, Math.max(0, v || 0)); return opts[n <= 20 ? 0 : n <= 40 ? 1 : n <= 60 ? 2 : n <= 80 ? 3 : 4] }
  const mood = pick(soul.mood, ['心情很差', '有些低落', '心情平平', '心情还不错', '心情很好'])
  const hope = pick(soul.hope, ['对未来感到绝望', '有些迷茫', '对未来没什么想法', '对未来有所期待', '对未来充满希望'])
  const griev = pick(soul.grievance, ['', '，偶尔会有点不满', '，对现状有些不满', '，对现状相当愤怒', '，对现状极度愤怒'])
  return `现在${mood}，${hope}${griev}。`
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
