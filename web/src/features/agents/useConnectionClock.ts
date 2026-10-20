import { useEffect, useState } from 'react'
import type { Agent } from '../../api/agents'

// Status changes at verification expiry, without ticking the application tree.
export function useConnectionClock(agents: Agent[]) {
  const [now, setNow] = useState(Date.now())
  const deadlines = agents.filter(agent => agent.connection_state === 'verified')
    .flatMap(agent => {
      const verified = Date.parse(agent.verified_at || '')
      return Number.isFinite(verified) ? [verified - 60_000, verified + 300_000] : []
    }).join(',')
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>
    const update = () => {
      const current = Date.now()
      setNow(current)
      const next = deadlines.split(',').map(Number).filter(value => value > current).sort((a,b) => a-b)[0]
      if (next) timer = setTimeout(update, Math.min(next - current + 1, 2_147_483_647))
    }
    update()
    return () => clearTimeout(timer)
  }, [deadlines])
  return now
}
