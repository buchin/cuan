import { GitHubPrerequisite } from './GitHubPrerequisite'
import { tasksApi } from '../../api/tasks'
import { workspacePath } from './workspacePath'
import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import {
  ArrowLeft,
  Info,
  Play,
  CheckCircle2,
  Copy,
  Check,
  Terminal,
  MessageSquare,
  CheckSquare,
  Rocket,
} from 'lucide-react'
import { agentsApi, type Agent } from '../../api/agents'
import type { TaskDetail } from '../../api/types'
import { ChatMarkdown } from '../inbox/ChatMarkdown'
import { ReloadButton } from '../../components/ui/ReloadButton'
import { useLanguage } from '../../context/LanguageContext'
import { relativeTime, agentDisplayName } from './task-ui'

export function TaskReadyPage({ task }: { task: TaskDetail }) {
  const { t } = useLanguage()
  const location = useLocation(),
    navigate = useNavigate()
  const [agents, setAgents] = useState<Agent[]>([]),
    [agent, setAgent] = useState(task.assignee_id || ''),
    [loading, setLoading] = useState(true),
    [revision, setRevision] = useState(0),
    [agentError, setAgentError] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [copied, setCopied] = useState(false),
    [applying, setApplying] = useState(false),
    [applied, setApplied] = useState(
      task.status === 'done' && (task.complete_note?.includes('diterapkan') || !!task.pr_url),
    )
  const [cancelled, setCancelled] = useState(false)

  const handleApply = async () => {
    if (applying || applied) return
    setApplying(true)
    setError('')
    try {
      await tasksApi.apply(task.id)
      setApplied(true)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not apply changes to website.')
    } finally {
      setApplying(false)
    }
  }

  const copyResult = () => {
    const text = task.summary || task.complete_note || ''
    if (text) {
      navigator.clipboard.writeText(text).then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      })
    }
  }
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
        <ReloadButton
          className="icon-action shrink-0"
          iconClassName="h-4 w-4"
          title={t('Reload app')}
        />
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
        <div className="max-w-3xl space-y-6 p-4 sm:p-6">
          {task.status === 'done' ? (
            <div className="space-y-4">
              {/* Success Banner */}
              <div className="rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-4 sm:p-5 text-card-foreground shadow-sm">
                <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-emerald-500/20 text-emerald-600 dark:text-emerald-400">
                      <CheckCircle2 className="h-6 w-6" />
                    </div>
                    <div>
                      <h3 className="text-base font-semibold text-foreground">
                        {t('Work completed')}
                      </h3>
                      <p className="text-xs text-muted-foreground">
                        {task.latest_attempt?.agent_name ? (
                          <>
                            {t('Completed by')}{' '}
                            <strong className="text-foreground">
                              {agentDisplayName(task.latest_attempt.agent_name)}
                            </strong>
                          </>
                        ) : (
                          t('This task has been resolved and approved.')
                        )}
                      </p>
                    </div>
                  </div>
                  <time
                    dateTime={task.updated_at}
                    title={new Date(task.updated_at).toLocaleString()}
                    className="text-xs text-muted-foreground whitespace-nowrap self-start sm:self-center"
                  >
                    {t(relativeTime(task.updated_at))}
                  </time>
                </div>
              </div>

              {/* Deliverable / Outcome */}
              {task.summary && (
                <section
                  aria-label="Recorded task result"
                  className="rounded-xl border border-border bg-card p-4 sm:p-5 text-card-foreground shadow-sm space-y-3"
                >
                  <div className="flex items-center justify-between gap-2 border-b border-border/50 pb-2.5">
                    <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                      {t('Deliverable / Outcome')}
                    </span>
                    <button
                      type="button"
                      onClick={copyResult}
                      className="action-secondary text-xs px-2.5 py-1 inline-flex items-center gap-1.5"
                    >
                      {copied ? (
                        <>
                          <Check className="h-3.5 w-3.5 text-emerald-600" />
                          <span>{t('Copied!')}</span>
                        </>
                      ) : (
                        <>
                          <Copy className="h-3.5 w-3.5" />
                          <span>{t('Copy result')}</span>
                        </>
                      )}
                    </button>
                  </div>
                  <div className="text-sm leading-relaxed text-foreground">
                    <ChatMarkdown content={task.summary} />
                  </div>
                </section>
              )}

              {/* Verification & Testing */}
              {task.complete_note && (
                <div className="rounded-xl border border-border/80 bg-muted/30 p-4 space-y-2">
                  <div className="flex items-center gap-2 text-xs font-semibold text-foreground">
                    <CheckSquare className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                    <span>{t('Verification & Testing')}</span>
                  </div>
                  <div className="text-xs text-muted-foreground">
                    <ChatMarkdown content={task.complete_note} />
                  </div>
                </div>
              )}

              {/* Action Toolbar */}
              <div className="flex flex-wrap items-center gap-2 pt-2">
                {task.pr_url && !applied && (
                  <button
                    type="button"
                    disabled={busy || applying}
                    onClick={handleApply}
                    className="action-primary text-xs px-3.5 py-2 inline-flex items-center gap-1.5 bg-gradient-to-r from-emerald-600 to-teal-600 text-white hover:from-emerald-700 hover:to-teal-700 shadow-sm transition-all"
                  >
                    <Rocket className="h-3.5 w-3.5" />
                    <span>{applying ? t('Applying...') : t('Apply changes')}</span>
                  </button>
                )}
                {applied && (
                  <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs font-semibold border border-emerald-500/20">
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    <span>{t('Changes applied')}</span>
                  </span>
                )}
                {back.startsWith('/inbox/') ? (
                  <Link
                    to={back}
                    className="action-primary text-xs px-3.5 py-2 inline-flex items-center gap-1.5 shadow-sm"
                  >
                    <MessageSquare className="h-3.5 w-3.5" />
                    <span>{t('Back to chat')}</span>
                  </Link>
                ) : (
                  <Link
                    to={back}
                    className="action-primary text-xs px-3.5 py-2 inline-flex items-center gap-1.5 shadow-sm"
                  >
                    <span>{t('Back to tasks')}</span>
                  </Link>
                )}
                {task.latest_attempt?.id && (
                  <Link
                    to={`${basePath}/sessions/latest${suffix}`}
                    className="action-secondary text-xs px-3 py-2 inline-flex items-center gap-1.5"
                  >
                    <Terminal className="h-3.5 w-3.5" />
                    <span>{t('View terminal log')}</span>
                  </Link>
                )}
                <Link
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
                  className="action-secondary text-xs px-3 py-2 inline-flex items-center gap-1.5"
                >
                  <Info className="h-3.5 w-3.5" />
                  <span>{t('Detail / Reopen')}</span>
                </Link>
              </div>
            </div>
          ) : (
            <>
              <section aria-label="Original request">
                <h3 className="mb-2 text-xs font-medium text-muted-foreground">
                  Your request
                </h3>
                <ChatMarkdown content={task.description || task.title} />
              </section>
              <p className="text-sm text-muted-foreground">
                {waiting
                  ? cancelled
                    ? 'The waiting work was cancelled.'
                    : 'Waiting for an available AI session. Cuan will start this work automatically.'
                  : task.launch?.state === 'stopped'
                    ? task.launch.safe_error ||
                      'The pending launch stopped. Check its details before continuing.'
                    : task.launch?.state === 'cancelled'
                      ? 'The waiting work was cancelled.'
                      : 'This task has not started yet.'}
              </p>
            </>
          )}
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
