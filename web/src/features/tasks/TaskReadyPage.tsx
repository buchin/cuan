import { GitHubPrerequisite } from './GitHubPrerequisite'
import { tasksApi } from '../../api/tasks'
import { workspacePath } from './workspacePath'
import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { ArrowLeft, Info, Play } from 'lucide-react'
import { agentsApi, type Agent } from '../../api/agents'
import type { TaskDetail } from '../../api/types'
import { ChatMarkdown } from '../inbox/ChatMarkdown'

export function TaskReadyPage({ task }: { task: TaskDetail }) {
  const location = useLocation(),
    navigate = useNavigate()
  const [agents, setAgents] = useState<Agent[]>([]),
    [agent, setAgent] = useState(task.assignee_id || ''),
    [loading, setLoading] = useState(true),
    [revision, setRevision] = useState(0),
    [agentError, setAgentError] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState('')
  const [cancelled, setCancelled] = useState(false)
  const waiting = task.launch?.state === 'queued' || task.launch?.state === 'claimed'
  const starting = useRef(false)
  const basePath = workspacePath(task.id, location.pathname)
  const search = new URLSearchParams(location.search)
  search.delete('view')
  const suffix = search.toString() ? '?' + search.toString() : ''
  const detailSearch = new URLSearchParams(search)
  detailSearch.set('view', 'detail')
  const back =
    typeof location.state?.taskReturnTo === 'string' &&
    /^\/(?:tasks|projects\/[^/?#]+(?:\/tasks)?|inbox\/[^/?#]+)(?:\?[^#]*)?$/.test(
      location.state.taskReturnTo,
    )
      ? location.state.taskReturnTo
      : basePath.startsWith('/projects/')
        ? basePath.split('/tasks/')[0] + '/tasks'
        : '/tasks' + suffix
  useEffect(() => {
    let alive = true
    if (task.status === 'done' || waiting) {
      setLoading(false)
      return
    }
    setLoading(true)
    setAgentError(false)
    setError('')
    agentsApi
      .list()
      .then((list) => {
        if (!alive) return
        const eligible = list.filter(
          (a) => a.enabled !== false && a.status !== 'inactive',
        )
        setAgents(eligible)
        setAgent((value) =>
          eligible.some((a) => a.id === value)
            ? value
            : (
                eligible.find((a) => a.connection_state === 'verified') ||
                eligible[0]
              )?.id || '',
        )
      })
      .catch(() => {
        if (alive) {
          setAgentError(true)
          setError(
            'Could not load agents. Try again or configure an agent in Settings.',
          )
        }
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [revision, task.status, waiting])
  return (
    <section
      aria-label="Task workspace"
      className="flex h-full min-h-0 flex-col"
    >
      <GitHubPrerequisite task={task} />
      <header className="flex shrink-0 items-center gap-2 border-b p-2">
        <Link
          to={back}
          aria-label={
            back.startsWith('/inbox/')
              ? 'Back to conversation'
              : 'Back to tasks'
          }
          className="icon-action"
        >
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <h2 className="min-w-0 flex-1 truncate text-sm font-semibold">
          {task.title}
        </h2>
        <Link
          aria-label="Detail"
          title="Detail"
          className="icon-action"
          to={
            '/tasks/' +
            encodeURIComponent(task.id) +
            '?' +
            detailSearch.toString()
          }
          state={{
            ...location.state,
            workspaceReturnTo: location.pathname + location.search,
          }}
        >
          <Info className="h-4 w-4" />
        </Link>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-3xl space-y-6 p-4 sm:p-6">
          <section aria-label="Original request">
            <h3 className="mb-2 text-xs font-medium text-muted-foreground">
              Your request
            </h3>
            <ChatMarkdown content={task.description || task.title} />
          </section>
          <p className="text-sm text-muted-foreground">
            {waiting ? (cancelled ? 'The waiting work was cancelled.' : 'Waiting for an available AI session. Cuan will start this work automatically.') : task.launch?.state === 'stopped' ? (task.launch.safe_error || 'The pending launch stopped. Check its details before continuing.') : task.launch?.state === 'cancelled' ? 'The waiting work was cancelled.' : task.status === 'done'
              ? 'This task is complete. Reopen it in Detail to continue.'
              : 'This task has not started yet.'}
          </p>
          {waiting && <button className="action-secondary" disabled={busy || cancelled} onClick={async () => {
            setBusy(true);setError('')
            try {await tasksApi.cancel(task.id);setCancelled(true)} catch(e) {setError(e instanceof Error ? e.message : 'Could not cancel waiting work.')} finally {setBusy(false)}
          }}>{cancelled ? 'Cancelled' : 'Cancel waiting work'}</button>}
          {task.status !== 'done' && !waiting && (
            <form
              className="flex flex-wrap items-center gap-2"
              onSubmit={(e) => {
                e.preventDefault()
                if (starting.current || busy || !agent) return
                starting.current = true
                setBusy(true)
                setError('')
                try {
                  const request = {
                    request_id: crypto.randomUUID(),
                    agent_id: agent,
                  }
                  localStorage.setItem(
                    'cuan-task-launch:' + task.id,
                    JSON.stringify(request),
                  )
                  navigate(basePath + '/sessions/start' + suffix, {
                    state: {
                      taskReturnTo: back,
                      startRequestId: request.request_id,
                    },
                  })
                } catch {
                  starting.current = false
                  setBusy(false)
                  setError(
                    'Could not save the start request. Enable browser storage and try again.',
                  )
                }
              }}
            >
              <label className="min-w-0">
                <span className="sr-only">Agent</span>
                <select
                  aria-label="Agent"
                  className="field-control max-w-64 text-sm"
                  value={agent}
                  disabled={loading || busy}
                  onChange={(e) => setAgent(e.target.value)}
                >
                  {!agent && <option value="">Choose an agent</option>}
                  {agents.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </select>
              </label>
              <button
                className="action-primary"
                disabled={loading || busy || !agent}
              >
                <Play className="h-4 w-4" />
                {busy ? 'Starting…' : 'Start'}
              </button>
            </form>
          )}
          {task.status !== 'done' && !waiting && !loading && !agents.length && (
            <Link
              to="/settings/agents"
              className="inline-block text-sm underline"
            >
              Configure an agent
            </Link>
          )}
          {agentError && (
            <button
              className="action-secondary"
              onClick={() => setRevision((value) => value + 1)}
            >
              Try again
            </button>
          )}
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
        </div>
      </div>
    </section>
  )
}
