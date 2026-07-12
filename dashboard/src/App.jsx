/**
 * [INPUT]: 依赖 React hooks、WorkerList 与 WorkerDetail 页面组件
 * [OUTPUT]: 对外提供 App 根组件，维护路径路由并在列表页/详情页之间切换
 * [POS]: dashboard/src 的应用入口组件，被 main.jsx 挂载，是 Dashboard 前端路由根
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { useState, useEffect } from 'react'
import WorkerList from './pages/WorkerList'
import WorkerDetail from './pages/WorkerDetail'

export default function App() {
  const [route, setRoute] = useState(parseRoute())

  useEffect(() => {
    const onPop = () => setRoute(parseRoute())
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  function navigate(path) {
    history.pushState(null, '', path)
    setRoute(parseRoute(path))
  }

  if (route.worker) {
    return <WorkerDetail name={route.worker} onBack={() => navigate('/')} />
  }
  return <WorkerList onSelect={name => navigate(`/worker/${name}`)} />
}

function parseRoute(path) {
  const p = path || location.pathname
  const m = p.match(/^\/worker\/(.+)$/)
  return { worker: m ? m[1] : null }
}
