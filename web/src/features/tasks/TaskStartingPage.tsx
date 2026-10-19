import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { LoaderCircle } from 'lucide-react'
import { tasksApi, type RunTaskRequest } from '../../api/tasks'
import { ApiError } from '../../api/client'

export function TaskStartingPage({ taskId }: { taskId: string }) {
  const location = useLocation(),
    navigate = useNavigate()
  const key = 'cuan-task-launch:' + taskId,
    resultKey = 'cuan-task-launch-result:' + taskId
  const [request, setRequest] = useState<RunTaskRequest | null>(() => {
    try {
      const p = JSON.parse(localStorage.getItem(key) || 'null')
      return p &&
        typeof p.request_id === 'string' &&
        typeof p.agent_id === 'string'
        ? p
        : null
    } catch {
      return null
    }
  })
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [title, setTitle] = useState('')
  const mounted = useRef(false),
    started = useRef(false),
    working = useRef(false),
    state = location.state
  const taskURL = `/tasks/${encodeURIComponent(taskId)}${location.search}`
  const openSession = useCallback(
    (id: string) =>
      navigate(
        `/tasks/${encodeURIComponent(taskId)}/sessions/${encodeURIComponent(id)}${location.search}`,
        { replace: true, state: { taskReturnTo: state?.taskReturnTo } },
      ),
    [navigate, taskId, location.search, state?.taskReturnTo],
  )
  useEffect(() => {
    mounted.current = true
    void tasksApi
      .get(taskId)
      .then((t) => {
        if (mounted.current) setTitle(t.title)
      })
      .catch(() => {})
    return () => {
      mounted.current = false
    }
  }, [taskId])
  const launch = useCallback(async () => {
    if (!request || working.current) return
    working.current = true
    setBusy(true)
    setError('')
    try {
      const result = await tasksApi.runAgent(taskId, request)
      if (!result.attempt) throw new Error('The session was not confirmed.')
      try {
        localStorage.setItem(
          resultKey,
          JSON.stringify({
            request_id: request.request_id,
            attempt_id: result.attempt.id,
          }),
        )
        localStorage.removeItem(key)
      } catch {}
      if (mounted.current) openSession(result.attempt.id)
    } catch (e) {
      if (!mounted.current) return
      if (e instanceof ApiError && e.status >= 400 && e.status < 500) {
        try {
          localStorage.removeItem(key)
        } catch {}
        setRequest(null)
        setError('Could not start the agent: ' + e.message)
      } else
        setError(
          'Session start was not confirmed. Check the same request to continue.',
        )
    } finally {
      working.current = false
      if (mounted.current) setBusy(false)
    }
  }, [request, taskId, resultKey, key, openSession])
  useEffect(() => {
    if (started.current) return
    if (request && state?.startRequestId === request.request_id) {
      started.current = true
      navigate(location.pathname + location.search, {
        replace: true,
        state: {
          taskReturnTo: state?.taskReturnTo,
          launchRequestId: request.request_id,
        },
      })
      void launch()
    } else if (!request && state?.launchRequestId) {
      try {
        const result = JSON.parse(localStorage.getItem(resultKey) || 'null')
        if (
          result?.request_id === state.launchRequestId &&
          typeof result.attempt_id === 'string'
        ) {
          started.current = true
          openSession(result.attempt_id)
        }
      } catch {}
    }
  }, [
    request,
    state,
    launch,
    navigate,
    location.pathname,
    location.search,
    resultKey,
    openSession,
  ])
  return (
    <section
      aria-label="Task session"
      className="flex h-full min-h-80 items-center justify-center"
    >
      <div className="max-w-md space-y-4 text-center">
        {busy && (
          <LoaderCircle className="mx-auto h-6 w-6 animate-spin text-primary" />
        )}
        <h2 className="text-lg font-semibold">
          {busy
            ? 'Starting session…'
            : error
              ? 'Could not start session'
              : request
                ? 'Continue session'
                : 'Session unavailable'}
        </h2>
        {title && (
          <p className="break-words text-sm text-muted-foreground">{title}</p>
        )}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-center gap-2">
          {request && !busy && (
            <button className="action-primary" onClick={() => void launch()}>
              Check session
            </button>
          )}
          <Link
            className="action-secondary"
            to={taskURL}
            state={{ taskReturnTo: state?.taskReturnTo }}
          >
            Task details
          </Link>
        </div>
      </div>
    </section>
  )
}
