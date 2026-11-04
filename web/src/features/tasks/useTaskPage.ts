import { useCallback, useEffect, useState } from 'react'
import { tasksApi, type TaskPage, type TaskPageOptions } from '../../api/tasks'

export function useTaskPage(options: TaskPageOptions) {
  const [search, setSearch] = useState(options.search || '')
  useEffect(() => {
    const timer = setTimeout(() => setSearch(options.search || ''), 200)
    return () => clearTimeout(timer)
  }, [options.search])
  const searchPending = (options.search || '') !== search
  const key = JSON.stringify({ ...options, search })
  const scope = JSON.stringify([options.project_id || '', !!options.exclude_terminals])
  const [scopeState, setScopeState] = useState<{ key: string; counts: Record<string, number>; total: number }>({ key: '', counts: {}, total: 0 })
  const [revision, setRevision] = useState(0)
  const [state, setState] = useState<{ key: string; data: TaskPage | null; error: string }>({ key: '', data: null, error: '' })
  const refresh = useCallback(() => setRevision(value => value + 1), [])
  useEffect(() => {
    if (searchPending) return
    let stopped = false, busy = false
    let request: AbortController | null = null
    let timer: ReturnType<typeof setTimeout>
    let changeTimer: ReturnType<typeof setTimeout>
    const filters: TaskPageOptions = JSON.parse(key)
    const load = async () => {
      if (stopped || busy || document.hidden) return
      busy = true
      request = new AbortController()
      let timedOut = false
      const deadline = setTimeout(() => { timedOut = true; request?.abort() }, 15000)
      let active = false
      try {
        const data = await tasksApi.page(filters, request.signal)
        if (stopped) return
        active = (data.counts?.active || 0) > 0
        setScopeState(previous => previous.key === scope && previous.total === data.scope_total && JSON.stringify(previous.counts) === JSON.stringify(data.counts) ? previous : { key: scope, counts: data.counts, total: data.scope_total })
        setState(previous => previous.key === key && !previous.error && JSON.stringify(previous.data) === JSON.stringify(data) ? previous : { key, data, error: '' })
      } catch (error) {
        if (!stopped && (!request.signal.aborted || timedOut)) setState(previous => ({ ...previous, key, data: previous.key === key ? previous.data : null, error: 'Could not load tasks. Check your connection and try again.' }))
      } finally {
        clearTimeout(deadline)
        busy = false
        if (!stopped) { clearTimeout(timer); timer = setTimeout(() => void load(), active ? 5000 : 20000) }
      }
    }
    void load()
    const resume = () => { if (!document.hidden) void load() }
    const changed = () => { clearTimeout(changeTimer); changeTimer = setTimeout(resume, 0) }
    window.addEventListener('focus', resume)
    document.addEventListener('visibilitychange', resume)
    window.addEventListener('cuan:tasks-changed', changed)
    return () => { stopped = true; request?.abort(); clearTimeout(timer); clearTimeout(changeTimer); window.removeEventListener('focus', resume); document.removeEventListener('visibilitychange', resume); window.removeEventListener('cuan:tasks-changed', changed) }
  }, [key, revision, searchPending, scope])
  return { counts: scopeState.key === scope ? scopeState.counts : {}, scopeTotal: scopeState.key === scope ? scopeState.total : 0, data: state.key === key ? state.data : null, loading: state.key !== key || (!state.data && !state.error), error: state.key === key ? state.error : '', refresh }
}
