import { GitHubPrerequisite } from './GitHubPrerequisite'
import { workspacePath } from './workspacePath'
import { useCallback, useEffect, useRef, useState, lazy, Suspense } from 'react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import {
  ArrowLeft,
  Info,
  Square,
  Send,
  Bot,
  CheckCircle2,
  Copy,
  Check,
  Terminal,
  MessageSquare,
  Rocket,
  FolderOpen,
  AlertCircle,
  Ban,
  RefreshCw,
  Pin,
  PinOff,
  ExternalLink,
  GitBranch,
} from 'lucide-react'
import { tasksApi, type LiveTaskSession } from '../../api/tasks'
import type { TaskDetail } from '../../api/types'
import { agentsApi } from '../../api/agents'
import { projectsApi } from '../../api/projects'
import { TaskReadyPage } from './TaskReadyPage'
const TaskTerminal = lazy(() =>
  import('./TaskTerminal').then((m) => ({
    default: m.TaskTerminal,
  })),
)
import { ChatMarkdown } from '../inbox/ChatMarkdown'
import { AttachmentFile } from '../inbox/Attachments'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { attemptLabel, taskRef, workStatus, relativeTime, workStatusBadgeClass } from './task-ui'
import { cn } from '../../lib/utils'
import { useLanguage } from '../../context/LanguageContext'

export function TaskSessionPage({
  taskId,
  sessionId,
}: {
  taskId: string
  sessionId: string
}) {
  const { t, language } = useLanguage()
  const location = useLocation(),
    navigate = useNavigate()
  const [detail, setDetail] = useState<TaskDetail | null>(null),
    [live, setLive] = useState<LiveTaskSession | null>(null),
    [error, setError] = useState(''),
    [runtimeError, setRuntimeError] = useState(''),
    [shareState, setShareState] = useState<'idle' | 'copied' | 'failed'>('idle'),
    [resultCopied, setResultCopied] = useState(false),
    [terminalControls, setTerminalControls] = useState<HTMLDivElement | null>(
      null,
    ),
    [terminalConnection, setTerminalConnection] = useState(''),
    [followup, setFollowup] = useState(''),
    [cancel, setCancel] = useState(false),
    [busy, setBusy] = useState(false),
    [pinTerminal, setPinTerminal] = useState(false),
    [showTerminal, setShowTerminal] = useState(false),
    [accept, setAccept] = useState(false),
    [acceptNote, setAcceptNote] = useState(''),
    [applying, setApplying] = useState(false),
    [applied, setApplied] = useState(false),
    [assistantName, setAssistantName] = useState('@cuan')

  useEffect(() => {
    let alive = true
    agentsApi.list().then(list => {
      const assistant = list.find(a => a.id === 'agent_cuan')
      if (alive && assistant) setAssistantName(assistant.name)
    }).catch(() => {})
    return () => { alive = false }
  }, [])

  const handleApply = async () => {
    if (applying || applied || !detail) return
    setApplying(true)
    setError('')
    try {
      await tasksApi.apply(detail.id)
      setApplied(true)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not apply changes to website.')
    } finally {
      setApplying(false)
    }
  }
  const [searchParams, setSearchParams] = useSearchParams()
  const showDetail = searchParams.get('panel') === 'detail'
  const setShowDetail = (open: boolean) => {
    setShareState('idle')
    const next = new URLSearchParams(searchParams)
    if (open) next.set('panel', 'detail')
    else next.delete('panel')
    setSearchParams(next, { replace: true, state: location.state })
  }
  const following = useRef(false)
  const refresh = useCallback(async () => {
    try {
      setDetail(await tasksApi.get(taskId))
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load this session.')
    }
    try {
      const runtime = await tasksApi.runtime(taskId)
      setLive(runtime.live_session || null)
      setRuntimeError('')
    } catch {
      setLive(null)
      setRuntimeError(
        'Live session status is unavailable. Saved output remains available.',
      )
    }
  }, [taskId])
  useEffect(() => {
    void refresh()
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') void refresh()
    }, 4000)
    return () => clearInterval(timer)
  }, [refresh])

  useEffect(() => {
    document.documentElement.classList.add('terminal-active')
    document.body.classList.add('terminal-active')
    return () => {
      document.documentElement.classList.remove('terminal-active')
      document.body.classList.remove('terminal-active')
    }
  }, [])
  const attempt =
    sessionId === 'latest'
      ? detail?.latest_attempt || detail?.attempts[0]
      : detail?.attempts.find((a) => a.id === sessionId)
  const active = Boolean(attempt && ['running', 'queued'].includes(attempt.status))

  const isDedicatedTerminal = Boolean(
    searchParams.get('terminal') === 'open' ||
    (location.state as { openTerminal?: boolean } | null)?.openTerminal ||
    detail?.title?.startsWith('Terminal ·')
  )

  // Auto-close prevention: keep terminal open on task completion when pinned
  const prevActiveRef = useRef(active)
  useEffect(() => {
    if (prevActiveRef.current && !active && pinTerminal) {
      setShowTerminal(true)
    }
    prevActiveRef.current = active
  }, [active, pinTerminal])

  useEffect(() => {
    if (isDedicatedTerminal) {
      setShowTerminal(true)
      setPinTerminal(true)
    }
  }, [isDedicatedTerminal])

  const handleTogglePin = useCallback(() => {
    setPinTerminal((prev) => {
      const next = !prev
      if (next) {
        setShowTerminal(true)
      }
      return next
    })
  }, [])

  const waitingLaunch = detail?.launch?.state === 'queued' || detail?.launch?.state === 'claimed'
  const basePath = workspacePath(taskId, location.pathname)
  const suffix = location.search,
    state = location.state
  const detailParams = new URLSearchParams(suffix)
  detailParams.set('view', 'detail')
  const taskURL =
    '/tasks/' + encodeURIComponent(taskId) + '?' + detailParams.toString()
  const backURL =
    typeof state?.taskReturnTo === 'string' &&
    /^\/(?:tasks|projects\/[^/?#]+(?:\/tasks)?|inbox\/[^/?#]+)(?:\?[^#]*)?$/.test(
      state.taskReturnTo,
    )
      ? state.taskReturnTo
      : basePath.startsWith('/projects/')
        ? basePath.split('/tasks/')[0] + '/tasks'
        : '/tasks' + suffix
  const detailState = {
    ...state,
    workspaceReturnTo: location.pathname + location.search,
  }
  useEffect(() => {
    if (sessionId === 'latest' && attempt && !waitingLaunch)
      navigate(
        basePath + '/sessions/' + encodeURIComponent(attempt.id) + suffix,
        { replace: true, state },
      )
  }, [sessionId, attempt?.id, taskId, suffix, navigate, state, basePath, waitingLaunch])
  useEffect(() => {
    setShowTerminal(pinTerminal || isDedicatedTerminal)
    setAccept(false)
    setFollowup('')
    following.current = false
  }, [sessionId, pinTerminal, isDedicatedTerminal])

  const [upstreamStatus, setUpstreamStatus] = useState<{
    sync_status?: string
    upstream?: string
    branch?: string
    behind_count?: number
    ahead_count?: number
    status_summary?: string
  } | null>(null)
  const [upstreamLoading, setUpstreamLoading] = useState(false)

  const loadUpstreamStatus = useCallback(async (fetchRemote = false) => {
    const projId = detail?.project_id || detail?.project_ids?.[0]
    if (!projId) return
    setUpstreamLoading(true)
    try {
      const res = await projectsApi.getTerminalStatus(projId, fetchRemote)
      if (res?.upstream) {
        setUpstreamStatus(res.upstream)
      }
    } catch {}
    finally {
      setUpstreamLoading(false)
    }
  }, [detail?.project_id, detail?.project_ids])

  useEffect(() => {
    if (isDedicatedTerminal && (detail?.project_id || detail?.project_ids?.[0])) {
      void loadUpstreamStatus(false)
    }
  }, [isDedicatedTerminal, detail?.project_id, detail?.project_ids, loadUpstreamStatus])

  const reports =
    detail?.activities.filter(
      (a) => a.attempt_id === attempt?.id && a.activity_type === 'agent_report',
    ) || []
  if (!detail) {
    return (
      <div className="flex h-full min-h-0 flex-col bg-background">
        <header className="flex h-11 sm:h-12 shrink-0 items-center justify-between border-b bg-background px-2 sm:px-3 gap-1.5 sm:gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-1.5 sm:gap-2">
            <Link
              to={backURL}
              state={state}
              className="icon-action shrink-0"
              aria-label={t('Back')}
              title={t('Back')}
            >
              <ArrowLeft className="h-4 w-4" />
            </Link>
            <h2 className="truncate text-sm sm:text-base font-semibold text-foreground min-w-0">
              {error ? t('Work not found') : t('Loading session…')}
            </h2>
          </div>
          {error && (
            <span className="text-xs font-semibold px-2 py-0.5 rounded-full border border-destructive/20 bg-destructive/10 text-destructive">
              {t('Error')}
            </span>
          )}
        </header>

        <div className="flex flex-1 items-center justify-center p-4">
          <div
            role={error ? 'alert' : 'status'}
            className="flex flex-col items-center justify-center max-w-sm rounded-xl border border-border bg-card p-6 text-center text-card-foreground shadow-sm space-y-3"
          >
            <div className={`flex h-12 w-12 items-center justify-center rounded-full ${error ? 'bg-destructive/10 text-destructive' : 'bg-primary/10 text-primary'}`}>
              {error ? <AlertCircle className="h-6 w-6" /> : <RefreshCw className="h-6 w-6 animate-spin" />}
            </div>
            <div className="space-y-1">
              <h3 className="text-sm font-semibold text-foreground">
                {error ? t('Work or session not found') : t('Loading session…')}
              </h3>
              <p className="text-xs text-muted-foreground">
                {error || t('Connecting to task runner and preparing terminal session...')}
              </p>
            </div>
            {error && (
              <div className="flex flex-wrap items-center justify-center gap-2 pt-2">
                <Link to={backURL} state={state} className="action-primary text-xs px-3 py-1.5">
                  {t('Back')}
                </Link>
                <button
                  type="button"
                  onClick={() => void refresh()}
                  className="action-secondary text-xs px-3 py-1.5 inline-flex items-center gap-1"
                >
                  <RefreshCw className="h-3 w-3" />
                  <span>{t('Try again')}</span>
                </button>
                <Link to="/tasks" className="action-secondary text-xs px-3 py-1.5">
                  {t('All tasks')}
                </Link>
              </div>
            )}
          </div>
        </div>
      </div>
    )
  }
  if ((!attempt || waitingLaunch) && sessionId === 'latest') return <TaskReadyPage task={detail} />
  if (!attempt) {
    const isCompleted = detail.status === 'done'
    return (
      <div className="flex h-full min-h-0 flex-col bg-background">
        <header className="flex h-11 sm:h-12 shrink-0 items-center justify-between border-b bg-background px-2 sm:px-3 gap-1.5 sm:gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-1.5 sm:gap-2">
            <Link
              to={backURL}
              state={state}
              className="icon-action shrink-0"
              aria-label={t('Back')}
              title={t('Back')}
            >
              <ArrowLeft className="h-4 w-4" />
            </Link>
            <h2 className="truncate text-sm sm:text-base font-semibold text-foreground min-w-0" title={detail.title}>
              {detail.title}
            </h2>
          </div>
          <span className={`text-xs font-semibold px-2.5 py-0.5 rounded-full border ${isCompleted ? 'bg-emerald-500/10 text-emerald-600 border-emerald-500/20' : 'bg-muted text-muted-foreground border-border'}`}>
            {t(isCompleted ? 'Completed' : 'Session ended')}
          </span>
        </header>

        <div className="flex flex-1 items-center justify-center p-4">
          <div
            role="status"
            className="flex flex-col items-center justify-center max-w-sm rounded-xl border border-border bg-card p-6 text-center text-card-foreground shadow-sm space-y-3"
          >
            <div className={`flex h-12 w-12 items-center justify-center rounded-full ${isCompleted ? 'bg-emerald-500/10 text-emerald-600' : 'bg-muted text-muted-foreground'}`}>
              {isCompleted ? <CheckCircle2 className="h-6 w-6" /> : <Ban className="h-6 w-6" />}
            </div>
            <div className="space-y-1">
              <h3 className="text-sm font-semibold text-foreground">
                {isCompleted ? t('Terminal session completed') : t('Session not found for this task')}
              </h3>
              <p className="text-xs text-muted-foreground">
                {isCompleted
                  ? t('This task execution has finished. You can view the results, checklist, and details in the task overview.')
                  : t('The requested session is no longer active or was not found. Start a new session or return to details.')}
              </p>
            </div>
            <div className="flex flex-wrap items-center justify-center gap-2 pt-2">
              <Link to={backURL} state={state} className="action-primary text-xs px-3 py-1.5">
                {t('Back')}
              </Link>
              <Link to={taskURL} state={state} className="action-secondary text-xs px-3 py-1.5">
                {t('Task details')}
              </Link>
              <button
                type="button"
                onClick={() => void refresh()}
                className="action-secondary text-xs px-3 py-1.5 inline-flex items-center gap-1"
              >
                <RefreshCw className="h-3 w-3" />
                <span>{t('Refresh')}</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    )
  }
  const latest = detail.latest_attempt?.id === attempt.id
  const final = [...reports]
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))
    .find((r) => ['review', 'blocked', 'done'].includes(r.new_status || ''))
  const answer =
    final?.new_status === 'review' &&
    attempt.status === 'succeeded' &&
    detail.activities.some(
      (a) =>
        a.attempt_id === attempt.id &&
        a.activity_type === 'agent_result_kind' &&
        a.created_at === final.created_at &&
        a.new_status === 'answer',
    )
  const result =
    final?.message.match(
      /## Work summary\n([\s\S]*?)(?:\n\n## Verification|$)/,
    )?.[1] ||
    (latest ? detail.summary : '') ||
    ''
  const checklist = (detail.dod_checklist || []).map((c) =>
    typeof c === 'string' ? { text: c, completed: false } : c,
  )
  const canAccept =
    latest &&
    detail.status === 'review' &&
    !active &&
    !answer &&
    attempt.status === 'succeeded' &&
    !!result.trim() &&
    !!detail.complete_note

  const copyResult = () => {
    const text = result || detail?.summary || detail?.complete_note || ''
    if (text) {
      navigator.clipboard.writeText(text).then(() => {
        setResultCopied(true)
        setTimeout(() => setResultCopied(false), 2000)
      })
    }
  }

  const status = workStatus(detail)
  const isWorking = status === 'Working'
  const isConnected = terminalConnection === 'connected' || terminalConnection === 'streaming'
  const isConnecting = (terminalConnection === 'connecting' || terminalConnection === 'loading') && !isConnected
  const statusLabel = answer
    ? 'Answered'
    : !latest
      ? attemptLabel(attempt)
      : isWorking
        ? (language === 'id' ? 'Berjalan' : 'Live')
        : status

  const statusBadge = (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1 sm:gap-1.5 rounded-full border px-2 sm:px-2.5 py-0.5 text-[11px] sm:text-xs font-semibold shadow-xs transition-colors',
        isWorking && isConnected
          ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/25'
          : workStatusBadgeClass(status)
      )}
      title={terminalConnection ? `${statusLabel} (${terminalConnection})` : statusLabel}
    >
      <span className="relative flex h-1.5 w-1.5 sm:h-2 sm:w-2 shrink-0 items-center justify-center">
        {isWorking && isConnected && (
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
        )}
        <span
          className={cn(
            'relative inline-flex h-1.5 w-1.5 sm:h-2 sm:w-2 rounded-full',
            isConnected
              ? 'bg-emerald-500'
              : isConnecting
                ? 'bg-amber-500 animate-pulse'
                : isWorking
                  ? 'bg-blue-500'
                  : status === 'Completed' || status === 'Answered'
                    ? 'bg-emerald-500'
                    : status === 'Needs attention'
                      ? 'bg-destructive'
                      : 'bg-muted-foreground'
          )}
        />
      </span>
      <span className="truncate max-w-[70px] min-[380px]:max-w-[100px] min-[440px]:max-w-none">{statusLabel}</span>
    </span>
  )

  return (
    <section
      aria-label="Task session"
      className="flex h-full min-h-0 flex-col bg-background overscroll-contain"
    >
      <GitHubPrerequisite task={detail} />
      <header className="flex h-11 sm:h-12 shrink-0 items-center justify-between border-b bg-background px-2 sm:px-3 gap-1.5 sm:gap-2 select-none">
        {/* Navigation & Task Title */}
        <div className="flex min-w-0 flex-1 items-center gap-1 sm:gap-2">
          <Link
            to={backURL}
            state={state}
            className="icon-action shrink-0 -ml-1 text-muted-foreground hover:text-foreground"
            aria-label={
              backURL.startsWith('/inbox/')
                ? 'Back to conversation'
                : 'Back to tasks'
            }
            title={
              backURL.startsWith('/inbox/')
                ? 'Back to conversation'
                : 'Back to tasks'
            }
          >
            <ArrowLeft className="h-4 w-4" />
          </Link>
          <div className="flex min-w-0 flex-1 items-center gap-1.5 sm:gap-2">
            <span className="flex h-5 w-5 sm:h-6 sm:w-6 shrink-0 items-center justify-center rounded bg-muted/60 text-muted-foreground">
              <Terminal className="h-3 w-3 sm:h-3.5 sm:w-3.5 text-foreground/80" />
            </span>
            <div className="flex items-center gap-1.5 min-w-0">
              <h2
                className="truncate text-xs sm:text-sm font-semibold text-foreground min-w-0 tracking-tight"
                title={detail.title}
              >
                {detail.title.startsWith('Terminal ·')
                  ? detail.title.replace(/^Terminal\s*·\s*/, '')
                  : detail.title}
              </h2>
              {detail.branch_name && (
                <span className="hidden xs:inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono bg-muted/80 text-muted-foreground shrink-0 max-w-[120px] truncate border border-border/40">
                  <GitBranch className="h-2.5 w-2.5 shrink-0" />
                  <span className="truncate">{detail.branch_name}</span>
                </span>
              )}
              {(assistantName) && (
                <span
                  className="hidden xs:inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-primary/10 text-primary shrink-0 border border-primary/20 max-w-[110px] truncate"
                  title={`Agent: ${assistantName}`}
                >
                  <Bot className="h-2.5 w-2.5 shrink-0" />
                  <span className="truncate">{assistantName}</span>
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Action and status controls: clean single row, mobile-safe (< 360px) */}
        <div className="flex shrink-0 items-center gap-1 sm:gap-1.5">
          {statusBadge}
          <div ref={setTerminalControls} className="hidden sm:flex shrink-0 items-center gap-1" />
          <button
            type="button"
            className={cn(
              'icon-action shrink-0 transition-colors hidden sm:inline-flex',
              pinTerminal
                ? 'text-emerald-500 dark:text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20'
                : 'text-muted-foreground hover:text-foreground'
            )}
            aria-label={pinTerminal ? t('Terminal pinned (Active)') : t('Pin terminal')}
            title={
              pinTerminal
                ? t('Terminal pinned: do not auto-close when completed')
                : t('Pin terminal: keep open when completed')
            }
            onClick={handleTogglePin}
          >
            {pinTerminal ? (
              <Pin className="h-4 w-4 fill-emerald-500/20 dark:fill-emerald-400/20" />
            ) : (
              <PinOff className="h-4 w-4" />
            )}
          </button>
          {active && (
            <button
              type="button"
              className="icon-action shrink-0 text-rose-500 hover:bg-rose-500/15 hover:text-rose-400 active:bg-rose-600 active:text-white transition-colors"
              aria-label="Cancel session"
              title={t('Cancel session')}
              onClick={() => setCancel(true)}
            >
              <Square className="h-4 w-4 fill-current" />
            </button>
          )}
          <button
            type="button"
            className="icon-action shrink-0 text-muted-foreground hover:text-foreground"
            aria-label="Detail"
            title={t('Detail')}
            onClick={() => setShowDetail(true)}
          >
            <Info className="h-4 w-4" />
          </button>
        </div>
      </header>
      {(error || runtimeError) && (
        <p role="alert" className="shrink-0 px-3 text-xs text-destructive">
          {error || runtimeError}
        </p>
      )}
      {attempt.error_message && (
        <button
          className="shrink-0 truncate px-3 py-1 text-left text-xs text-destructive"
          onClick={() => setShowDetail(true)}
          title={attempt.error_message}
        >
          Session needs attention · View detail
        </button>
      )}
      {active || showTerminal || isDedicatedTerminal ? (
        <div className="flex min-h-0 flex-1 flex-col overscroll-contain">
          {!active && (
            <div className="flex shrink-0 items-center justify-between border-b border-border/60 bg-muted/40 px-3 py-1.5 text-xs text-muted-foreground backdrop-blur-xs">
              <div className="flex items-center gap-2 min-w-0">
                <span className="flex h-2 w-2 shrink-0 rounded-full bg-muted-foreground/50" />
                <span className="truncate font-medium text-foreground">
                  {isDedicatedTerminal
                    ? t('Project terminal · Live with agent')
                    : pinTerminal
                      ? (language === 'id' ? 'Terminal disematkan · Sesi berakhir' : 'Terminal pinned · Session ended')
                      : (language === 'id' ? 'Sesi berakhir' : 'Session ended')}
                  {attempt.status && !isDedicatedTerminal ? ` (${attempt.status})` : ''}
                </span>
                <span className="hidden sm:inline text-muted-foreground/60">·</span>
                <span className="hidden sm:inline text-muted-foreground">{t('Terminal output saved')}</span>
              </div>
              {!isDedicatedTerminal && (
                <button
                  type="button"
                  className="action-secondary shrink-0 text-xs py-1 px-2.5 inline-flex items-center gap-1.5 ml-2"
                  onClick={() => setShowTerminal(false)}
                >
                  <span>{t('View result')}</span>
                  <ArrowLeft className="h-3 w-3 rotate-180" />
                </button>
              )}
            </div>
          )}
          <div className="min-h-0 flex-1 overscroll-contain">
            <Suspense fallback={<div className="h-full w-full bg-black/90 flex items-center justify-center text-xs font-mono text-muted-foreground">Connecting terminal...</div>}>
              <TaskTerminal
                key={sessionId}
                taskId={taskId}
                attempt={attempt}
                live={live}
                onRefresh={() => void refresh()}
                controlsTarget={terminalControls}
                onConnectionChange={setTerminalConnection}
              />
            </Suspense>
          </div>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto">
          <div className="task-result-content max-w-3xl space-y-4 p-4 sm:p-6">
            <section aria-label="Session result">
              <div className="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
                <Bot className="h-4 w-4" aria-hidden="true" />
                <span className="font-medium text-foreground">{assistantName}</span>
                <time dateTime={final?.created_at || attempt.finished_at || attempt.created_at}>{relativeTime(final?.created_at || attempt.finished_at || attempt.created_at)}</time>
              </div>
              {result ? (
                <div className="w-fit max-w-full rounded-xl rounded-tl-sm border bg-muted/20 p-3 sm:p-4"><ChatMarkdown content={result} /></div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  No result was recorded for this session. Open the terminal or
                  Detail to check what happened.
                </p>
              )}
            </section>
            {detail.attachments && detail.attachments.length > 0 && (
              <section aria-label={t('Files from this work session')} className="space-y-2 rounded-xl border border-border bg-card p-4">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                  <FolderOpen className="h-4 w-4 text-primary" />
                  <span>{t('Files from this work session')}</span>
                </h3>
                <div className="flex flex-wrap gap-2 pt-1">
                  {detail.attachments.map((a) => (
                    <div
                      key={a.id}
                      className="rounded-lg border border-border bg-muted/30 p-2 text-xs"
                    >
                      <AttachmentFile attachment={a} />
                    </div>
                  ))}
                </div>
              </section>
            )}
            {(canAccept || !result) && <div className="flex flex-wrap items-center gap-2">
              {detail.pr_url && !applied && canAccept && (
                <button
                  type="button"
                  disabled={busy || applying}
                  onClick={handleApply}
                  className="action-primary bg-gradient-to-r from-emerald-600 to-teal-600 text-white hover:from-emerald-700 hover:to-teal-700 shadow-sm inline-flex items-center gap-1.5"
                >
                  <Rocket className="h-4 w-4" />
                  <span>{applying ? t('Applying...') : t('Apply changes')}</span>
                </button>
              )}
              {canAccept && (
                <button
                  className="action-secondary"
                  onClick={() => {
                    setAcceptNote('')
                    setAccept(true)
                  }}
                >
                  Accept
                </button>
              )}
              {canAccept && detail.assignee_id && (
                <button
                  className="action-secondary"
                  onClick={() =>
                    document
                      .querySelector<HTMLTextAreaElement>(
                        'textarea[aria-label="Follow-up instructions"]',
                      )
                      ?.focus()
                  }
                >
                  Request changes
                </button>
              )}
              {!result && (
                <button
                  className="action-secondary"
                  onClick={() => setShowTerminal(true)}
                >
                  Open terminal
                </button>
              )}
            </div>}
            {!latest && (
              <div className="mt-4 rounded-xl border border-border/80 bg-muted/40 p-4 shadow-xs space-y-3">
                <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
                  <div className="flex items-center gap-2.5">
                    <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-primary shrink-0">
                      <Info className="h-4 w-4" />
                    </div>
                    <div>
                      <h4 className="text-sm font-semibold text-foreground">
                        {t('Viewing earlier session')} ({attemptLabel(attempt)})
                      </h4>
                      <p className="text-xs text-muted-foreground">
                        {t('To run commands or continue working with the agent, switch to the latest session.')}
                      </p>
                    </div>
                  </div>
                  {detail.latest_attempt && (
                    <button
                      type="button"
                      className="action-primary text-xs px-3.5 py-2 shrink-0 inline-flex items-center gap-1.5 ml-auto sm:ml-0"
                      onClick={() => {
                        navigate(
                          basePath + '/sessions/' + encodeURIComponent(detail.latest_attempt!.id) + suffix,
                          { replace: true, state },
                        )
                      }}
                    >
                      <span>{t('Switch to latest session')}</span>
                      <ArrowLeft className="h-3.5 w-3.5 rotate-180" />
                    </button>
                  )}
                </div>
              </div>
            )}
            {latest && detail.status === 'done' && (
              <div className="mt-4 rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-4 text-card-foreground shadow-sm space-y-3">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-emerald-500/20 text-emerald-600 dark:text-emerald-400">
                    <CheckCircle2 className="h-4 w-4" />
                  </div>
                  <div>
                    <h4 className="text-sm font-semibold text-foreground">
                      {t('Task completed')}
                    </h4>
                    <p className="text-xs text-muted-foreground">
                      {t('The result has been accepted and verified.')}
                    </p>
                  </div>
                </div>
                {detail.complete_note && (
                  <div className="rounded-lg border border-border/70 bg-background/60 p-3 text-xs space-y-1">
                    <span className="font-semibold text-foreground">
                      {t('Acceptance note')}:
                    </span>
                    <ChatMarkdown content={detail.complete_note} />
                  </div>
                )}
                <div className="flex flex-wrap items-center gap-2 pt-1">
                  <button
                    type="button"
                    onClick={copyResult}
                    className="action-secondary text-xs px-2.5 py-1 inline-flex items-center gap-1.5"
                  >
                    {resultCopied ? (
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
                  <button
                    type="button"
                    onClick={() => setShowTerminal(true)}
                    className="action-secondary text-xs px-2.5 py-1 inline-flex items-center gap-1.5"
                  >
                    <Terminal className="h-3.5 w-3.5" />
                    <span>{t('Terminal log')}</span>
                  </button>
                  <Link
                    to={backURL}
                    state={state}
                    className="action-primary text-xs px-3 py-1 inline-flex items-center gap-1.5 ml-auto"
                  >
                    {backURL.startsWith('/inbox/') ? (
                      <>
                        <MessageSquare className="h-3.5 w-3.5" />
                        <span>{t('Back to chat')}</span>
                      </>
                    ) : (
                      <span>{t('Back to tasks')}</span>
                    )}
                  </Link>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
      {!active &&
        detail.latest_attempt?.id === attempt.id &&
        detail.status !== 'done' && (
          <form
            className="flex shrink-0 flex-col sm:flex-row items-stretch sm:items-end gap-2 border-t bg-background p-2"
            onSubmit={(e) => {
              e.preventDefault()
              if (!followup.trim() || busy || following.current) return
              following.current = true
              setBusy(true)
              const agentToUse = 'agent_cuan'
              const request = {
                request_id: crypto.randomUUID(),
                agent_id: agentToUse,
                revision: followup.trim(),
              }
              try {
                localStorage.setItem(
                  'cuan-task-launch:' + taskId,
                  JSON.stringify(request),
                )
                navigate(basePath + '/sessions/start' + suffix, {
                  state: {
                    taskReturnTo: state?.taskReturnTo,
                    startRequestId: request.request_id,
                  },
                })
              } catch {
                following.current = false
                setBusy(false)
                setError(
                  'Could not save the session request. Enable browser storage and try again.',
                )
              }
            }}
          >
            <label className="min-w-0 flex-1">
              <span className="sr-only">Follow-up instructions</span>
              <textarea
                aria-label="Follow-up instructions"
                rows={1}
                maxLength={24000}
                className="field-control min-h-11 max-h-32 resize-none text-base leading-5 md:text-sm"
                placeholder={
                  answer
                    ? 'Reply to the agent…'
                    : 'Message the agent…'
                }
                value={followup}
                onChange={(e) => setFollowup(e.target.value)}
              />
            </label>
            <button
              type="submit"
              className="icon-action bg-primary text-primary-foreground shrink-0 self-end"
              disabled={!followup.trim() || busy}
              aria-label="Start follow-up session"
              title="Start follow-up session"
            >
              <Send className="h-4 w-4" />
            </button>
            <span className="sr-only">
              Starts a new session with this task’s context and retained
              history.
            </span>
          </form>
        )}
      <ActionDialog
        open={showDetail}
        title={t('Detail')}
        drawer
        onClose={() => setShowDetail(false)}
      >
        <div className="space-y-4">
          {/* Header Card: Title, Status Badge, and Session Timestamp */}
          <div className="rounded-xl border bg-muted/20 p-3 sm:p-4 space-y-2.5">
            <div className="flex items-start justify-between gap-2">
              <div className="flex items-center gap-2 min-w-0">
                <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground">
                  <Terminal className="h-4 w-4" />
                </span>
                <h3 className="break-words text-sm sm:text-base font-semibold text-foreground">
                  {detail.title}
                </h3>
              </div>
              <span
                className={cn(
                  'inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium',
                  workStatusBadgeClass(status)
                )}
              >
                <span className="h-1.5 w-1.5 rounded-full bg-current" />
                {statusLabel}
              </span>
            </div>

            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground pt-2 border-t border-border/40">
              <span>
                {t('Session')} {attempt.attempt_number}
                {attempt.exit_code !== undefined && ` · Exit ${attempt.exit_code}`}
              </span>
              <span>·</span>
              <span>
                {new Date(attempt.started_at || attempt.created_at).toLocaleString('en-US', {
                  month: 'short',
                  day: 'numeric',
                  year: 'numeric',
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </span>
            </div>
          </div>

          {/* Action Toolbar: Clean secondary buttons */}
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              className="inline-flex h-9 items-center gap-1.5 rounded-lg border bg-background px-3 text-xs font-medium text-foreground hover:bg-muted active:scale-95 transition-all cursor-pointer"
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(window.location.href)
                  setShareState('copied')
                  setTimeout(() => setShareState('idle'), 2500)
                } catch {
                  setShareState('failed')
                }
              }}
            >
              {shareState === 'copied' ? (
                <>
                  <Check className="h-3.5 w-3.5 text-emerald-500" />
                  <span className="text-emerald-500 font-semibold">{t('Link copied')}</span>
                </>
              ) : (
                <>
                  <Copy className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>{t('Copy link')}</span>
                </>
              )}
            </button>

            <Link
              to={taskURL}
              state={detailState}
              className="inline-flex h-9 items-center gap-1.5 rounded-lg border bg-background px-3 text-xs font-medium text-foreground hover:bg-muted active:scale-95 transition-all"
            >
              <ExternalLink className="h-3.5 w-3.5 text-muted-foreground" />
              <span>{t('Open task details')} · {taskRef(taskId)}</span>
            </Link>

            {!showTerminal && (
              <button
                type="button"
                className="inline-flex h-9 items-center gap-1.5 rounded-lg border bg-background px-3 text-xs font-medium text-foreground hover:bg-muted active:scale-95 transition-all cursor-pointer"
                onClick={() => {
                  setShowDetail(false)
                  setShowTerminal(true)
                }}
              >
                <Terminal className="h-3.5 w-3.5 text-muted-foreground" />
                <span>{t('Open terminal')}</span>
              </button>
            )}
          </div>
          {shareState === 'failed' && (
            <p role="status" className="text-xs text-destructive">
              {t('Could not copy the link. Copy it from your browser’s address bar.')}
            </p>
          )}

          {/* Session Switcher (Only shown if there are multiple attempts) */}
          {detail.attempts.length > 1 && (
            <label className="block text-xs text-muted-foreground space-y-1">
              <span className="font-medium">{t('Switch session')}</span>
              <select
                aria-label="Select session"
                className="field-control text-sm"
                value={sessionId}
                onChange={(e) => {
                  navigate(
                    basePath +
                      '/sessions/' +
                      encodeURIComponent(e.target.value) +
                      suffix,
                    { state },
                  )
                }}
              >
                {detail.attempts.map((a) => (
                  <option key={a.id} value={a.id}>
                    Session {a.attempt_number} · {attemptLabel(a)}
                  </option>
                ))}
              </select>
            </label>
          )}

          {attempt.error_message && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive break-words">
              {attempt.error_message}
            </div>
          )}

          {/* Dedicated Terminal Info Card (if terminal task) */}
          {isDedicatedTerminal && (
            <div className="rounded-xl border bg-muted/10 p-3 text-xs text-muted-foreground space-y-2">
              <div className="flex items-center justify-between gap-1.5 font-medium text-foreground">
                <div className="flex items-center gap-1.5">
                  <Info className="h-3.5 w-3.5 text-primary" />
                  <span>{t('Dedicated Project Terminal')}</span>
                </div>
                {upstreamStatus?.upstream && (
                  <button
                    type="button"
                    onClick={() => void loadUpstreamStatus(true)}
                    disabled={upstreamLoading}
                    className="inline-flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                    title={t('Fetch remote upstream status non-destructively')}
                  >
                    <RefreshCw className={cn("h-3 w-3", upstreamLoading && "animate-spin")} />
                    <span>{upstreamLoading ? t('Checking…') : t('Fetch upstream')}</span>
                  </button>
                )}
              </div>
              <p className="leading-relaxed">
                {t('This session provides an interactive PTY shell directly connected to the workspace. Output is streamed live and preserved automatically.')}
              </p>
              {upstreamStatus && (
                <div className="pt-1.5 border-t border-border/40 flex flex-wrap items-center gap-2 text-[11px]">
                  <span className="font-medium text-foreground">
                    {t('Upstream')}:
                  </span>
                  <span className={cn(
                    "px-1.5 py-0.5 rounded text-[10px] font-mono",
                    upstreamStatus.behind_count && upstreamStatus.behind_count > 0
                      ? "bg-amber-500/10 text-amber-600 dark:text-amber-400 font-semibold"
                      : "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                  )}>
                    {upstreamStatus.upstream || 'origin/main'} · {upstreamStatus.sync_status || 'in sync'}
                  </span>
                  {upstreamStatus.branch && (
                    <span className="text-muted-foreground font-mono">
                      ({upstreamStatus.branch})
                    </span>
                  )}
                </div>
              )}
              <div className="pt-1 border-t border-border/40 flex items-center gap-1.5 text-[10px] text-muted-foreground">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 inline-block" />
                <span>{t('Brain Failover: Automatic healthy model rotation on quota limit (429)')}</span>
              </div>
            </div>
          )}

          {latest && checklist.length > 0 && (
            <details className="rounded-lg border p-3">
              <summary className="cursor-pointer text-sm font-medium">
                Agent checklist
              </summary>
              <p className="my-2 text-xs text-muted-foreground">
                Reported by agent · compare with your original request before
                accepting.
              </p>
              {checklist.map((item, index) => (
                <label
                  key={item.id || index}
                  className="flex min-h-11 items-start gap-2 py-2 text-sm"
                >
                  <input
                    type="checkbox"
                    aria-label={'Verify: ' + item.text}
                    checked={item.completed}
                    disabled={busy || active || detail.status === 'done'}
                    onChange={async () => {
                      setBusy(true)
                      try {
                        await tasksApi.update(taskId, {
                          dod_checklist: checklist.map((c, i) =>
                            i === index ? { ...c, completed: !c.completed } : c,
                          ),
                          expected_updated_at: detail.updated_at,
                        })
                        await refresh()
                      } catch (e) {
                        setError(
                          e instanceof Error
                            ? e.message
                            : 'Could not save checklist.',
                        )
                      } finally {
                        setBusy(false)
                      }
                    }}
                    className="mt-1 h-4 w-4 shrink-0"
                  />
                  <span>{item.text}</span>
                </label>
              ))}
            </details>
          )}
            {latest && detail.complete_note && detail.complete_note !== result && <details className="rounded-lg border px-3 py-2">
              <summary className="flex min-h-11 cursor-pointer items-center text-sm font-medium">Verification</summary>
              <div className="pb-2 pt-1"><ChatMarkdown content={detail.complete_note} /></div>
            </details>}
            {(!isDedicatedTerminal || (detail.description && detail.description !== detail.title)) && (
              <details className="rounded-lg border px-3 py-2">
                <summary className="flex min-h-11 cursor-pointer items-center text-sm font-medium">Your request</summary>
                <div className="pb-2 pt-1"><ChatMarkdown content={detail.description || detail.title} /></div>
              </details>
            )}

          {(!isDedicatedTerminal || reports.length > 0) && (
            <aside aria-label="Agent report">
              <h3 className="mb-3 text-sm font-semibold">Agent report</h3>
              {!reports.length ? (
                <p className="text-sm text-muted-foreground">
                  {active
                    ? 'Waiting for the agent’s first checkpoint.'
                    : 'No structured report was recorded. Check the saved output.'}
                </p>
              ) : (
                <ol className="space-y-3">
                  {reports.map((r) => (
                    <li key={r.id} className="rounded-lg border p-3">
                      <p className="mb-2 text-xs text-muted-foreground">
                        {r.new_status === 'done'
                          ? 'Completed'
                          : r.new_status === 'review'
                            ? 'Result ready'
                            : r.new_status === 'blocked'
                              ? 'Blocker'
                              : 'Progress'}{' '}
                        · {new Date(r.created_at).toLocaleTimeString('en-US')}
                      </p>
                      <ChatMarkdown content={r.message} />
                    </li>
                  ))}
                </ol>
              )}
            </aside>
          )}
        </div>
      </ActionDialog>
      <ActionDialog
        open={accept}
        title="Accept result"
        busy={busy}
        onClose={() => setAccept(false)}
      >
        <form
          className="space-y-4"
          onSubmit={async (e) => {
            e.preventDefault()
            if (
              busy ||
              !acceptNote.trim() ||
              checklist.some((c) => !c.completed)
            )
              return
            setBusy(true)
            try {
              await tasksApi.update(taskId, {
                status: 'done',
                complete_note: acceptNote.trim(),
                expected_updated_at: detail.updated_at,
              })
              setAccept(false)
              await refresh()
            } catch (e) {
              setError(
                e instanceof Error ? e.message : 'Could not accept the result.',
              )
            } finally {
              setBusy(false)
            }
          }}
        >
          <p className="text-sm text-muted-foreground">
            Confirm the result meets your request, stays within scope, and uses
            the correct workspace.
          </p>
          <p className="max-h-32 overflow-y-auto whitespace-pre-wrap rounded-lg border p-3 text-sm">
            {detail.description || detail.title}
          </p>
          {checklist.some((c) => !c.completed) && (
            <p className="text-sm text-destructive">
              Verify the remaining checklist items in Detail before accepting.
            </p>
          )}
          <label className="grid gap-1 text-sm">
            Verification
            <textarea
              aria-label="Acceptance evidence"
              required
              rows={3}
              maxLength={16000}
              className="field-control"
              value={acceptNote}
              onChange={(e) => setAcceptNote(e.target.value)}
              placeholder="How you verified this meets the request."
            />
          </label>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <button
            className="action-primary"
            disabled={
              busy || !acceptNote.trim() || checklist.some((c) => !c.completed)
            }
          >
            Accept result
          </button>
        </form>
      </ActionDialog>
      <ActionDialog
        open={cancel}
        title="Cancel session?"
        busy={busy}
        onClose={() => setCancel(false)}
      >
        <p className="mb-4 text-sm">
          Stop the active process? Saved output and existing file changes are
          retained.
        </p>
        <div className="flex justify-end gap-2">
          <button
            className="action-secondary"
            disabled={busy}
            onClick={() => setCancel(false)}
          >
            Keep running
          </button>
          <button
            className="action-primary"
            disabled={busy}
            onClick={async () => {
              setBusy(true)
              try {
                await tasksApi.cancel(taskId)
                setCancel(false)
                await refresh()
              } catch (e) {
                setError(
                  e instanceof Error
                    ? e.message
                    : 'Could not cancel the session.',
                )
              } finally {
                setBusy(false)
              }
            }}
          >
            Cancel session
          </button>
        </div>
      </ActionDialog>
    </section>
  )
}
