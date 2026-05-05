import { useState, useEffect } from 'react'
import styles from './WorkerList.module.css'

export default function WorkerList({ onSelect }) {
  const [workers, setWorkers] = useState([])

  useEffect(() => {
    const load = () => fetch('/api/workers').then(r => r.json()).then(setWorkers).catch(() => {})
    load()
    const timer = setInterval(load, 10000)
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
