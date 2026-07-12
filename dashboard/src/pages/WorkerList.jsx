/**
 * [INPUT]: 依赖 React hooks、Dashboard Worker/公告 API、WorkerList.module.css
 * [OUTPUT]: 对外提供 WorkerList 页面组件，展示今日城市公告与全部工人入口
 * [POS]: dashboard/src/pages 的首页列表页，被 App.jsx 在根路径渲染
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { useState, useEffect } from 'react'
import styles from './WorkerList.module.css'

export default function WorkerList({ onSelect }) {
  const [workers, setWorkers] = useState([])
  const [announcements, setAnnouncements] = useState([])

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
      <nav className={styles.nav}>
        <div className={styles.logo}>WORKER ARCHIVE</div>
        <span className={styles.label}>NEW LONDON REGISTRY</span>
      </nav>
      <main className={styles.main}>
        <h1 className={styles.title}>Worker Registry</h1>
        <section className={styles.announcementBoard}>
          <span className={styles.announcementLabel}>今日公告</span>
          {announcements.length ? (
            announcements.map((text, i) => (
              <p key={i} className={styles.announcementText}>{text}</p>
            ))
          ) : (
            <p className={styles.announcementEmpty}>暂无公告</p>
          )}
        </section>
        <div className={styles.grid}>
          {workers.map(w => (
            <div key={w.name} className={styles.card} onClick={() => onSelect(w.name)}>
              <div className={styles.avatar}>
                {w.avatar ? <img src={w.avatar} className={styles.avatarImg} /> : w.name[0]?.toUpperCase()}
              </div>
              <div className={styles.info}>
                <div className={styles.name}>{w.name}</div>
                <div className={styles.occupation}>{w.occupation}</div>
                <div className={styles.status} data-status={w.status}>{w.status}</div>
              </div>
              <div className={styles.meters}>
                <Meter label="心情" value={w.mood} />
                <Meter label="希望" value={w.hope} />
                <Meter label="不满" value={w.grievance} />
              </div>
            </div>
          ))}
          {workers.length === 0 && <p className={styles.empty}>暂无工人。请先启动 Worker Agent。</p>}
        </div>
      </main>
    </div>
  )
}

function Meter({ label, value }) {
  return (
    <div className={styles.meter}>
      <span>{label}</span>
      <div className={styles.bar}>
        <div className={styles.fill} style={{ width: `${Math.min(100, Math.max(0, value))}%` }} />
      </div>
      <span>{value}</span>
    </div>
  )
}
