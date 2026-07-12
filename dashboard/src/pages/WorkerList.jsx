/**
 * [INPUT]: 依赖 React hooks、Dashboard Worker/公告 API、WorkerList.module.css
 * [OUTPUT]: 对外提供 WorkerList 页面组件，以报纸头版展示今日城市公告与全部工人入口
 * [POS]: dashboard/src/pages 的首页列表页，被 App.jsx 在根路径渲染
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { useState, useEffect } from 'react'
import styles from './WorkerList.module.css'

export default function WorkerList({ onSelect }) {
  const [workers, setWorkers] = useState([])
  const [announcements, setAnnouncements] = useState([])
  const today = new Date()
  const lead = workers[0] || null
  const others = workers.slice(1)
  const announcement = announcements[0] || '暂无公告'

  useEffect(() => {
    const load = () => fetch('/api/workers').then(r => r.json()).then(setWorkers).catch(() => {})
    load()
    const timer = setInterval(load, 10000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => {
    const load = () => fetch('/api/announcements')
      .then(r => r.json())
      .then(data => setAnnouncements(data.announcements || []))
      .catch(() => setAnnouncements([]))
    load()
    const timer = setInterval(load, 60000)
    return () => clearInterval(timer)
  }, [])

  return (
    <div className={styles.container}>
      <main className={styles.main}>
        <header className={styles.masthead}>
          <div className={styles.editionLine}>
            <span>Price: One Coal Chit</span>
            <span>Vol. I — No. {String(dayOfYear(today)).padStart(3, '0')}</span>
            <span>{formatDateline(today)}</span>
          </div>
          <div className={styles.rule} />
          <h1 className={styles.paperTitle}>The Daily Bulletin</h1>
          <div className={styles.rule} />
          <nav className={styles.sections} aria-label="Bulletin sections">
            <span>City Orders</span>
            <span>Industrial Ledger</span>
            <span>Worker Dispatch</span>
            <span>Public Notices</span>
          </nav>
        </header>

        <section className={styles.frontPage}>
          <div className={styles.leadStory}>
            <span className={styles.kicker}>Latest Order From The Generator</span>
            <h2 className={styles.headline}>{announcement}</h2>
            {lead ? (
              <div className={styles.leadGrid}>
                <div>
                  <p className={styles.dropcap}>
                    {lead.name} 今日登记在册，职业为{lead.occupation || '新伦敦居民'}。
                    档案员记录其心情指数为 {clampMetric(lead.mood)}，希望指数为 {clampMetric(lead.hope)}，
                    不满指数为 {clampMetric(lead.grievance)}。点击姓名档案可进入完整记录。
                  </p>
                  <button className={styles.readMore} onClick={() => onSelect(lead.name)}>
                    Open Dossier
                  </button>
                </div>
                <figure className={styles.leadPortrait}>
                  {lead.avatar ? (
                    <img src={lead.avatar} alt={lead.name} className={styles.woodcut} />
                  ) : (
                    <div className={styles.initialPortrait}>{lead.name[0]?.toUpperCase()}</div>
                  )}
                  <figcaption>
                    {lead.name}<br />
                    <span>档案状态：{statusText(lead.status)}</span>
                  </figcaption>
                </figure>
              </div>
            ) : (
              <p className={styles.empty}>暂无工人。请先启动 Worker Agent。</p>
            )}
          </div>

          <aside className={styles.statistics}>
            <h3>状态统计</h3>
            {lead ? (
              <>
                <Metric label="心情指数" value={lead.mood} />
                <Metric label="希望指数" value={lead.hope} />
                <Metric label="不满指数" value={lead.grievance} />
              </>
            ) : (
              <p className={styles.muted}>No active registry entries.</p>
            )}
            <div className={styles.editorialNote}>
              <span>Editorial Note:</span>
              城市公告对所有 agent 保持一致；个人见闻仍由各自心跳偶发记录。
            </div>
          </aside>
        </section>

        <section className={styles.dispatchGrid}>
          {others.map((w, i) => (
            <article key={w.name} className={styles.dispatch} onClick={() => onSelect(w.name)}>
              <span className={styles.entryNo}>Entry No. {entryNumber(w.name, i)}</span>
              <h4>{w.name}</h4>
              {w.avatar && (
                <div className={i % 2 === 0 ? styles.thumbRight : styles.thumbLeft}>
                  <img src={w.avatar} alt={w.name} className={styles.woodcut} />
                </div>
              )}
              <p>
                {w.name} 被记录为{w.occupation || '新伦敦居民'}。
                当前档案状态为{statusText(w.status)}，希望与不满在寒冷中继续拉扯。
              </p>
              <div className={styles.dispatchStatus}>状态：{statusText(w.status)}</div>
              <div className={styles.miniMetrics}>
                <span>心情 {clampMetric(w.mood)}</span>
                <span>希望 {clampMetric(w.hope)}</span>
                <span>不满 {clampMetric(w.grievance)}</span>
              </div>
            </article>
          ))}
        </section>
      </main>
      <footer className={styles.footer}>
        <span>Printed by the Worker Archive Press</span>
        <span>New London Registry</span>
        <span>Page 1 of {Math.max(workers.length, 1)}</span>
      </footer>
    </div>
  )
}

function Metric({ label, value }) {
  const v = clampValue(value)
  return (
    <div className={styles.metric}>
      <span>{label}</span>
      <strong>{v}%</strong>
    </div>
  )
}

function clampValue(value) {
  return Math.min(100, Math.max(0, Number(value) || 0))
}

function clampMetric(value) {
  return `${clampValue(value)}%`
}

function statusText(status) {
  if (status === 'running') return '观察中'
  if (status === 'thinking' || status === 'reasoning') return '思考中'
  return '休眠中'
}

function dayOfYear(date) {
  const start = new Date(date.getFullYear(), 0, 0)
  return Math.floor((date - start) / 86400000)
}

function formatDateline(date) {
  return new Intl.DateTimeFormat('en-US', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  }).format(date)
}

function entryNumber(name, index) {
  const sum = [...name].reduce((n, ch) => n + ch.charCodeAt(0), 0)
  return String((sum + index * 97) % 10000).padStart(4, '0')
}
