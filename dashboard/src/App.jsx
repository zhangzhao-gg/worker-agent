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
