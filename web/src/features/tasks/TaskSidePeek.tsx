import { GitHubPrerequisite } from './GitHubPrerequisite'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams, useNavigate, useLocation } from 'react-router-dom'
import {
  X,
  Maximize2,
  Minimize2,
  MoreHorizontal,
  Play,
  Pencil,
  Copy,
  Trash2,
  RefreshCw,
  ExternalLink,
  Rocket,
  CheckCircle2,
  FolderOpen,
} from 'lucide-react'
import { AttachmentFile } from '../inbox/Attachments'
import { useLanguage } from '../../context/LanguageContext'
import type {
  Task,
  TaskDetail,
  TaskStatus,
  UpdateTaskRequest,
} from '../../api/types'
import { agentsApi, type Agent } from '../../api/agents'
import { ApiError } from '../../api/client'
import {
  tasksApi,
  type LiveTaskSession,
  type RunTaskRequest,
} from '../../api/tasks'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { ChatMarkdown } from '../inbox/ChatMarkdown'
import { trapDialogFocus } from '../../lib/dialog'
import { cn } from '../../lib/utils'
import { TaskSessions } from './TaskSessions'
import {
  TASK_STATUSES,
  activeAttempt,
  statusLabel,
  taskRef,
  executionLabel,
  relativeTime,
} from './task-ui'
import './task-workspace.css'
interface Props {
  launching?: boolean
  initialError?: string
  task: Task | null
  projects: import('../../api/types').Project[]
  isOpen: boolean
  onClose: () => void
  onUpdate: (id: string, updates: UpdateTaskRequest) => Promise<void>
  onDelete: (id: string) => Promise<void>
  onRunAgent: (id: string) => Promise<void>
}
interface Draft {
  title: string
  description: string
  summary: string
  complete_note: string
  criteria: string
  priority: Task['priority']
  assignee: string
  project: string
  version: string
}
const drafts = new Map<string, Draft>()
function makeDraft(t: Task): Draft {
  return {
    title: t.title,
    description: t.description || t.body || t.prompt || '',
    summary: t.summary || '',
    complete_note: t.complete_note || '',
    criteria: (t.dod_checklist || [])
      .map((i) => (typeof i === 'string' ? i : i.text))
      .join('\n'),
    priority: t.priority || 'normal',
    assignee: t.assignee_id || t.assignee || '',
    project: t.project_id || t.project_ids?.[0] || '',
    version: t.updated_at,
  }
}
function pendingKey(id: string) {
  return `cuan-task-launch:${id}`
}
function readPending(id: string): RunTaskRequest | null {
  try {
    const p = JSON.parse(localStorage.getItem(pendingKey(id)) || 'null')
    return p &&
      typeof p.request_id === 'string' &&
      typeof p.agent_id === 'string'
      ? p
      : null
  } catch {
    return null
  }
}
export function TaskSidePeek(props: Props) {
  if (!props.isOpen || !props.task) return null
  return <TaskDetailView key={props.task.id} {...props} task={props.task} />
}
function TaskDetailView({
  task: initial,
  initialError,
  launching = false,
  projects,
  onClose,
  onUpdate,
  onDelete,
  onRunAgent,
}: Props & { task: Task }) {
  const { t } = useLanguage()
  const dialog = useRef<HTMLDialogElement>(null),
    [params, setParams] = useSearchParams(),
    full = params.get('view') === 'full'
  const [detail, setDetail] = useState<TaskDetail>({
      ...initial,
      attempts: [],
      activities: [],
    }),
    [loaded, setLoaded] = useState(false),
    [loadError, setLoadError] = useState(''),
    [error, setError] = useState(initialError || ''),
    [notice, setNotice] = useState(''),
    [agents, setAgents] = useState<Agent[]>([]),
    [live, setLive] = useState<LiveTaskSession | null>(null),
    [runtimeError, setRuntimeError] = useState(''),
    [applying, setApplying] = useState(false)
  const [tab, setTab] = useState<'overview' | 'sessions' | 'activity'>(
      'overview',
    ),
    [editing, setEditing] = useState(drafts.has(initial.id)),
    [draft, setDraft] = useState<Draft>(
      () => drafts.get(initial.id) || makeDraft(initial),
    ),
    [busy, setBusy] = useState(false),
    [menu, setMenu] = useState(false),
    [modal, setModal] = useState<
      'run' | 'approve' | 'delete' | 'cancel' | null
    >(null),
    [agentId, setAgentId] = useState(
      initial.assignee_id || initial.assignee || '',
    ),
    [feedback, setFeedback] = useState(''),
    [approvalNote, setApprovalNote] = useState(''),
    [comment, setComment] = useState(''),
    [pending, setPending] = useState<RunTaskRequest | null>(() =>
      readPending(initial.id),
    )
  useEffect(() => {
    if (initialError) setError(initialError)
  }, [initialError])
  const navigate = useNavigate(),
    location = useLocation()
  const openSession = (id: string) => {
    if (!id) return
    const next = new URLSearchParams(location.search)
    next.delete('view')
    next.delete('retry')
    navigate(
      `/tasks/${encodeURIComponent(initial.id)}/sessions/${encodeURIComponent(id)}${next.toString() ? '?' + next.toString() : ''}`,
      { state: location.state },
    )
  }
  useEffect(() => {
    if (!launching) setPending(readPending(initial.id))
  }, [launching, initial.id])
  const busyRef = useRef(false),
    mounted = useRef(true)
  const running = activeAttempt(detail.latest_attempt),
    checklist = (detail.dod_checklist || []).map((i) =>
      typeof i === 'string' ? { text: i, completed: false } : i,
    )
  const dirty =
    editing && JSON.stringify(draft) !== JSON.stringify(makeDraft(detail))
  const selectedProject = projects.find(
    (p) => p.id === detail.project_id || detail.project_ids?.includes(p.id),
  )
  const reload = useCallback(async () => {
    try {
      const next = await tasksApi.get(initial.id)
      if (!mounted.current) return
      setDetail(next)
      setLoaded(true)
      setLoadError('')
    } catch (e) {
      if (mounted.current)
        setLoadError(
          e instanceof Error ? e.message : 'Could not load this task.',
        )
    }
    try {
      const state = await tasksApi.runtime(initial.id)
      if (mounted.current) {
        setLive(state.live_session || null)
        setRuntimeError('')
      }
    } catch {
      if (mounted.current) {
        setLive(null)
        setRuntimeError(
          'Live session status is unavailable. Saved attempts remain available.',
        )
      }
    }
  }, [initial.id])

  const handleApplyToWebsite = async () => {
    if (applying || !detail) return
    setApplying(true)
    try {
      await tasksApi.apply(detail.id)
      setNotice(t('Changes successfully applied!'))
      await reload()
      if (onUpdate) {
        await onUpdate(detail.id, { status: 'done' })
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Could not apply changes.'))
    } finally {
      setApplying(false)
    }
  }
  useEffect(() => {
    mounted.current = true
    void reload()
    let active = true
    agentsApi
      .list()
      .then((list) => {
        if (active) setAgents(list)
      })
      .catch(() => {})
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible' && !busyRef.current)
        void reload()
    }, 4000)
    return () => {
      mounted.current = false
      active = false
      clearInterval(timer)
    }
  }, [reload])
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    dialog.current?.showModal()
    return () => {
      dialog.current?.close()
      if (previous?.isConnected) previous.focus()
    }
  }, [])
  useEffect(() => {
    if (editing) drafts.set(initial.id, draft)
  }, [editing, draft, initial.id])
  useEffect(() => {
    if (!dirty) return
    const before = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', before)
    return () => window.removeEventListener('beforeunload', before)
  }, [dirty])
  const close = () => {
    if (busy) return
    if (dirty && !window.confirm('Discard unsaved task changes?')) return
    drafts.delete(initial.id)
    onClose()
  }
  const working = async (action: () => Promise<void>) => {
    if (busyRef.current) return
    busyRef.current = true
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await action()
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : 'The action could not be completed. Try again.',
      )
    } finally {
      busyRef.current = false
      if (mounted.current) setBusy(false)
    }
  }
  const update = async (patch: UpdateTaskRequest) => {
    await onUpdate(initial.id, patch)
    await reload()
  }
  const startEdit = () => {
    setDraft(drafts.get(initial.id) || makeDraft(detail))
    setEditing(true)
    setMenu(false)
    setError('')
  }
  const save = () =>
    void working(async () => {
      const text = draft.title.trim()
      if (!text || text.length > 200)
        throw new Error('Enter a task title of up to 200 characters.')
      const items = draft.criteria
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean)
        .map((text) => ({
          text,
          completed: checklist.find((i) => i.text === text)?.completed || false,
        }))
      await update({
        title: text,
        description: draft.description,
        summary: draft.summary,
        complete_note: draft.complete_note,
        priority: draft.priority,
        assignee_id: draft.assignee,
        project_ids: draft.project ? [draft.project] : [],
        dod_checklist: items,
        expected_updated_at: draft.version,
      })
      drafts.delete(initial.id)
      setEditing(false)
      setNotice('Task changes saved.')
    })
  const changeStatus = (status: TaskStatus) =>
    void working(async () => {
      await update({ status, expected_updated_at: detail.updated_at })
      setNotice(`Moved to ${statusLabel(status)}.`)
    })
  const launch = async (payload: RunTaskRequest) => {
    // Persist the request before sending. Unknown outcomes are checked with the
    // same ID, including after reload or a fast process exit.
    try {
      localStorage.setItem(pendingKey(initial.id), JSON.stringify(payload))
    } catch {
      throw new Error(
        'Browser storage is unavailable. Enable it before starting a session so an uncertain request can be recovered safely.',
      )
    }
    setPending(payload)
    setModal(null)
    try {
      const result = await tasksApi.runAgent(initial.id, payload)
      localStorage.removeItem(pendingKey(initial.id))
      setPending(null)
      void onRunAgent(initial.id)
      if (result.attempt) {
        openSession(result.attempt.id)
        return
      }
      setTab('sessions')
      await reload()
      setNotice(
        result.replayed
          ? 'Showing the recorded result of the same launch request.'
          : 'Session request accepted.',
      )
    } catch (e) {
      if (e instanceof ApiError && e.status >= 400 && e.status < 500) {
        localStorage.removeItem(pendingKey(initial.id))
        setPending(null)
        await reload()
      } else await reload()
      throw e
    }
  }
  const autoRetryStarted = useRef(false)
  useEffect(() => {
    if (
      autoRetryStarted.current ||
      !loaded ||
      !params.get('retry') ||
      launching ||
      busyRef.current ||
      pending ||
      running ||
      !agentId ||
      detail.status === 'done'
    )
      return
    autoRetryStarted.current = true
    void working(() =>
      launch({
        request_id: crypto.randomUUID(),
        agent_id: agentId,
      }),
    )
  }, [agentId, detail.status, launch, loaded, launching, params, pending, running])
  const openRun = () => {
    setAgentId(detail.assignee_id || detail.assignee || '')
    setFeedback('')
    setModal('run')
    setError('')
    setMenu(false)
  }
  const runLabel = running
    ? 'Open session'
    : detail.status === 'review'
      ? 'Request changes'
      : detail.attempts.length
        ? 'Retry'
        : 'Start'
  const taskAction = () => {
    if (running) {
      openSession(detail.latest_attempt?.id || '')
    } else openRun()
  }
  const authorName = (id: string) => agents.find((a) => a.id === id)?.name || id
  const field = (key: keyof Draft, value: string) =>
    setDraft((d) => ({ ...d, [key]: value }))
  const emptyProse = (text: string | undefined, empty: string) => (
    <div className="task-prose min-w-0">
      {text ? (
        <ChatMarkdown content={text} />
      ) : (
        <p className="text-sm text-muted-foreground">{empty}</p>
      )}
    </div>
  )
  return (
    <dialog
      ref={dialog}
      aria-modal="true"
      aria-label="Task detail"
      onKeyDown={(e) => {
        trapDialogFocus(e)
        if (e.key === 'Escape') setMenu(false)
      }}
      onCancel={(e) => {
        e.preventDefault()
        close()
      }}
      className={cn(
        'task-workspace fixed inset-y-0 left-auto right-0 m-0 flex h-dvh max-h-none max-w-none flex-col border-l bg-card p-0 text-card-foreground shadow-2xl backdrop:bg-black/40',
        full ? 'w-full' : 'w-full sm:w-[min(760px,94vw)]',
      )}
    >
      <GitHubPrerequisite task={detail} />
      <header className="shrink-0 border-b px-4 pb-3 pt-2 sm:px-6">
        <div className="flex min-h-11 items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
            <span className="shrink-0 font-mono">{taskRef(detail.id)}</span>
            {selectedProject && (
              <span className="truncate">· {selectedProject.name}</span>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <button
              className="icon-action hidden sm:inline-flex"
              aria-label={full ? 'Collapse task view' : 'Expand task view'}
              onClick={() =>
                setParams(
                  (old) => {
                    const next = new URLSearchParams(old)
                    if (full) next.set('view', 'detail')
                    else next.set('view', 'full')
                    return next
                  },
                  { replace: true, state: location.state },
                )
              }
            >
              {full ? (
                <Minimize2 className="h-4 w-4" />
              ) : (
                <Maximize2 className="h-4 w-4" />
              )}
            </button>
            <div className="relative">
              <button
                className="icon-action"
                aria-label="Task actions"
                aria-expanded={menu}
                onClick={() => setMenu((v) => !v)}
              >
                <MoreHorizontal className="h-4 w-4" />
              </button>
              {menu && (
                <div className="absolute right-0 top-full z-20 min-w-48 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg">
                  <button
                    className="flex min-h-11 w-full items-center gap-2 px-3 text-left text-sm"
                    onClick={async () => {
                      setMenu(false)
                      try {
                        await navigator.clipboard.writeText(
                          window.location.href,
                        )
                        setNotice('Task link copied.')
                      } catch {
                        setError(
                          'Could not copy the task link. Copy it from your address bar.',
                        )
                      }
                    }}
                  >
                    <Copy className="h-4 w-4" />
                    Copy link
                  </button>
                  <button
                    className="flex min-h-11 w-full items-center gap-2 px-3 text-left text-sm text-destructive disabled:opacity-50"
                    disabled={running || busy}
                    onClick={() => {
                      setMenu(false)
                      setError('')
                      setModal('delete')
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                    Delete task
                  </button>
                </div>
              )}
            </div>
            <button
              autoFocus
              className="icon-action"
              aria-label="Close task details"
              onClick={close}
              disabled={busy}
            >
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>
        <h2 className="break-words text-lg font-semibold leading-snug sm:text-xl">
          {detail.title}
        </h2>
        <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
          <span>{statusLabel(detail.status)}</span>
          {['running', 'queued'].includes(
            detail.latest_attempt?.status || '',
          ) && (
            <span>
              {['running', 'queued'].includes(
                detail.latest_attempt?.status || '',
              )
                ? executionLabel(detail)
                : ''}
            </span>
          )}
          <span>Updated {relativeTime(detail.updated_at)}</span>
        </div>
      </header>
      <nav
        aria-label="Task sections"
        className="flex shrink-0 gap-1 border-b px-3 sm:px-5"
      >
        {(
          [
            { id: 'overview', label: 'Overview' },
            {
              id: 'sessions',
              label: `Sessions${detail.attempts.length ? ' (' + detail.attempts.length + ')' : ''}`,
            },
            { id: 'activity', label: 'Activity' },
          ] as const
        ).map((item) => (
          <button
            key={item.id}
            aria-pressed={tab === item.id}
            disabled={editing}
            onClick={() => setTab(item.id)}
            className={cn(
              'min-h-11 border-b-2 px-3 text-sm',
              tab === item.id
                ? 'border-primary font-medium text-primary'
                : 'border-transparent text-muted-foreground',
            )}
          >
            {item.label}
          </button>
        ))}
      </nav>
      {(error || loadError) && (
        <div
          role="alert"
          className="shrink-0 border-b px-4 py-2 text-sm text-destructive"
        >
          {error || loadError}
          {loadError && (
            <button
              className="ml-2 min-h-8 underline"
              onClick={() => void reload()}
            >
              Reload task
            </button>
          )}
        </div>
      )}
      {notice && (
        <p
          role="status"
          className="shrink-0 border-b px-4 py-2 text-xs text-muted-foreground"
        >
          {notice}
        </p>
      )}
      {launching && (
        <p role="status" className="shrink-0 border-b px-4 py-3 text-sm">
          Starting session…
        </p>
      )}
      {pending && !launching && (
        <div className="shrink-0 border-b bg-muted/40 px-4 py-3 text-sm">
          <p>
            A launch request is not confirmed. Check this same request before
            starting another session.
          </p>
          <button
            className="action-secondary mt-2"
            disabled={busy}
            onClick={() => void working(() => launch(pending))}
          >
            Check launch request
          </button>
        </div>
      )}
      <main
        className={cn('min-h-0 min-w-0 flex-1 p-4 sm:p-6', 'overflow-y-auto')}
      >
        {!loaded && (
          <p role="status" className="mb-3 text-sm text-muted-foreground">
            Loading task details…
          </p>
        )}
        {tab === 'overview' && (
          <div className="space-y-6">
            {editing ? (
              <form
                id="task-edit-form"
                className="space-y-4"
                onSubmit={(e) => {
                  e.preventDefault()
                  save()
                }}
              >
                <label className="grid gap-1 text-sm font-medium">
                  Title
                  <textarea
                    aria-label="Task title"
                    className="field-control resize-y"
                    rows={2}
                    maxLength={200}
                    required
                    value={draft.title}
                    onChange={(e) => field('title', e.target.value)}
                    disabled={busy}
                  />
                </label>
                <label className="grid gap-1 text-sm font-medium">
                  Human instructions
                  <textarea
                    aria-label="Human instructions"
                    className="field-control min-h-40 resize-y font-normal"
                    rows={6}
                    maxLength={24000}
                    value={draft.description}
                    onChange={(e) => field('description', e.target.value)}
                    disabled={busy}
                  />
                </label>
                <p className="text-xs text-muted-foreground">
                  Markdown is supported. Instructions are kept separate from
                  work summaries and results.
                </p>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="grid gap-1 text-sm">
                    Priority
                    <select
                      className="field-control"
                      value={draft.priority}
                      onChange={(e) => field('priority', e.target.value)}
                      disabled={busy}
                    >
                      {['urgent', 'high', 'normal', 'low'].map((p) => (
                        <option key={p} value={p}>
                          {p[0].toUpperCase() + p.slice(1)}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="grid gap-1 text-sm">
                    Assignee
                    <select
                      className="field-control"
                      value={draft.assignee}
                      onChange={(e) => field('assignee', e.target.value)}
                      disabled={busy}
                    >
                      <option value="">Unassigned</option>
                      {draft.assignee &&
                        !agents.some((a) => a.id === draft.assignee) && (
                          <option value={draft.assignee}>
                            {draft.assignee} (unavailable)
                          </option>
                        )}
                      {agents.map((a) => (
                        <option
                          key={a.id}
                          value={a.id}
                          disabled={a.enabled === false}
                        >
                          {a.name}
                          {a.enabled === false ? ' (disabled)' : ''}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="grid gap-1 text-sm sm:col-span-2">
                    Project
                    <select
                      className="field-control"
                      value={draft.project}
                      onChange={(e) => field('project', e.target.value)}
                      disabled={busy}
                    >
                      <option value="">No project</option>
                      {projects
                        .filter((p) => !p.is_archived || p.id === draft.project)
                        .map((p) => (
                          <option
                            key={p.id}
                            value={p.id}
                            disabled={p.is_archived}
                          >
                            {p.name}
                            {p.is_archived ? ' (archived)' : ''}
                          </option>
                        ))}
                    </select>
                  </label>
                </div>
                <label className="grid gap-1 text-sm font-medium">
                  Acceptance criteria
                  <textarea
                    aria-label="Acceptance criteria"
                    className="field-control font-normal"
                    rows={4}
                    value={draft.criteria}
                    onChange={(e) => field('criteria', e.target.value)}
                    disabled={busy}
                  />
                  <span className="text-xs font-normal text-muted-foreground">
                    One criterion per line, up to50. Changing its text clears
                    its verification.
                  </span>
                </label>
                <label className="grid gap-1 text-sm font-medium">
                  Work summary
                  <textarea
                    aria-label="Work summary"
                    className="field-control font-normal"
                    rows={4}
                    maxLength={12000}
                    value={draft.summary}
                    onChange={(e) => field('summary', e.target.value)}
                    disabled={busy}
                  />
                </label>
                <label className="grid gap-1 text-sm font-medium">
                  Completion notes & verification
                  <textarea
                    aria-label="Completion notes"
                    className="field-control font-normal"
                    rows={4}
                    maxLength={16000}
                    value={draft.complete_note}
                    onChange={(e) => field('complete_note', e.target.value)}
                    disabled={busy}
                  />
                </label>
              </form>
            ) : (
              <>
                <section className="grid grid-cols-2 gap-3 rounded-lg border bg-muted/20 p-3 text-sm">
                  <label className="grid gap-1 text-xs text-muted-foreground">
                    Status
                    <select
                      aria-label="Task status"
                      className="field-control text-sm"
                      disabled={busy || running || !loaded}
                      value={detail.status}
                      onChange={(e) =>
                        changeStatus(e.target.value as TaskStatus)
                      }
                    >
                      {TASK_STATUSES.filter(
                        (s) =>
                          s.id === detail.status ||
                          ['backlog', 'todo', 'blocked'].includes(s.id) ||
                          (s.id === 'review' && detail.attempts.length === 0),
                      ).map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <div>
                    <p className="text-xs text-muted-foreground">Priority</p>
                    <p className="mt-2 capitalize">{detail.priority}</p>
                  </div>
                  <div className="col-span-2">
                    <p className="text-xs text-muted-foreground">Assignee</p>
                    <p className="mt-1 break-words">
                      {authorName(
                        detail.assignee_id || detail.assignee || '',
                      ) || 'Unassigned'}
                    </p>
                  </div>
                  {detail.pr_url && /^https?:\/\//i.test(detail.pr_url) && (
                    <div className="col-span-2 mt-2 rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-3.5 space-y-2.5">
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2">
                          <Rocket className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                          <span className="text-xs font-semibold text-foreground">
                            {detail.status === 'done' || detail.complete_note?.includes('diterapkan')
                              ? t('Changes applied')
                              : t('Changes ready to apply')}
                          </span>
                        </div>
                        {detail.status === 'done' || detail.complete_note?.includes('diterapkan') ? (
                          <span className="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                            <CheckCircle2 className="h-3.5 w-3.5" />
                            {t('Changes applied')}
                          </span>
                        ) : null}
                      </div>

                      {detail.status !== 'done' && !detail.complete_note?.includes('diterapkan') ? (
                        <>
                          <p className="text-xs text-muted-foreground leading-relaxed">
                            {t('Agent work has been verified and is ready to be applied to your project.')}
                          </p>
                          <button
                            type="button"
                            disabled={busy || applying}
                            onClick={handleApplyToWebsite}
                            className="action-primary w-full text-xs py-2 inline-flex items-center justify-center gap-2 bg-gradient-to-r from-emerald-600 to-teal-600 text-white hover:from-emerald-700 hover:to-teal-700 shadow-sm transition-all"
                          >
                            <Rocket className="h-3.5 w-3.5" />
                            <span>{applying ? t('Applying...') : t('Apply changes')}</span>
                          </button>
                        </>
                      ) : null}

                      <div className="pt-1 border-t border-border/40">
                        <a
                          href={detail.pr_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground hover:text-foreground hover:underline"
                        >
                          <ExternalLink className="h-3 w-3" />
                          <span>{t('View technical details on GitHub')}</span>
                        </a>
                      </div>
                    </div>
                  )}
                </section>
                <section>
                  <h3 className="mb-2 text-sm font-semibold">
                    Human instructions
                  </h3>
                  {emptyProse(
                    detail.description,
                    'No instructions yet. Edit this task to describe the work.',
                  )}
                </section>
                <section>
                  <h3 className="mb-2 text-sm font-semibold">
                    Agent checklist
                  </h3>
                  {checklist.length ? (
                    <div className="space-y-1">
                      {checklist.map((item, index) => (
                        <label
                          key={item.id || index}
                          className="flex min-h-11 cursor-pointer items-start gap-3 rounded-md py-2 text-sm"
                        >
                          <input
                            type="checkbox"
                            aria-label={`Verify: ${item.text}`}
                            checked={!!item.completed}
                            disabled={busy || detail.status === 'done'}
                            onChange={() =>
                              void working(async () => {
                                const next = checklist.map((c, i) =>
                                  i === index
                                    ? { ...c, completed: !c.completed }
                                    : c,
                                )
                                await update({
                                  dod_checklist: next,
                                  expected_updated_at: detail.updated_at,
                                })
                              })
                            }
                            className="mt-1 h-4 w-4 shrink-0"
                          />
                          <span
                            className={cn(
                              'min-w-0 break-words',
                              item.completed && 'text-muted-foreground',
                            )}
                          >
                            {item.text}
                          </span>
                        </label>
                      ))}
                      <p className="text-xs text-muted-foreground">
                        {checklist.filter((i) => i.completed).length} of{' '}
                        {checklist.length} reported by agent
                      </p>
                    </div>
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      No acceptance criteria defined yet.
                    </p>
                  )}
                </section>
                <section>
                  <h3 className="mb-2 text-sm font-semibold">Work summary</h3>
                  {emptyProse(detail.summary, 'No work summary recorded yet.')}
                </section>
                <section>
                  <h3 className="mb-2 text-sm font-semibold">
                    Completion notes & verification
                  </h3>
                  {emptyProse(
                    detail.complete_note,
                    'No completion notes recorded yet. Review the session output and record what changed and how it was verified before approval.',
                  )}
                </section>
                {detail.attachments && detail.attachments.length > 0 && (
                  <section aria-label={t('Files from this work session')}>
                    <h3 className="mb-2 text-sm font-semibold flex items-center gap-1.5">
                      <FolderOpen className="h-4 w-4 text-primary" />
                      <span>{t('Files from this work session')}</span>
                    </h3>
                    <div className="flex flex-wrap gap-2">
                      {detail.attachments.map((a) => (
                        <div
                          key={a.id}
                          className="rounded-lg border border-border bg-card p-2 text-xs"
                        >
                          <AttachmentFile attachment={a} />
                        </div>
                      ))}
                    </div>
                  </section>
                )}
                {(detail.diff_summary || detail.test_summary) && (
                  <details className="rounded-lg border p-3 text-sm">
                    <summary className="cursor-pointer">
                      Additional execution evidence
                    </summary>
                    {emptyProse(detail.diff_summary, '')}
                    {emptyProse(detail.test_summary, '')}
                  </details>
                )}
                {detail.source_room_id && (
                  <a
                    href={`/inbox/${encodeURIComponent(detail.source_room_id)}`}
                    className="action-secondary text-sm"
                  >
                    Open source conversation
                  </a>
                )}
              </>
            )}
          </div>
        )}
        {tab === 'sessions' && (
          <div className="flex h-full min-h-0 flex-col gap-2">
            {runtimeError && (
              <p role="status" className="text-xs text-muted-foreground">
                {runtimeError}
              </p>
            )}
            <div className="min-h-0 flex-1">
              <TaskSessions
                attempts={detail.attempts}
                agents={agents}
                live={live}
                onOpen={openSession}
                onCancel={() => {
                  setError('')
                  setModal('cancel')
                }}
                busy={busy}
              />
            </div>
          </div>
        )}
        {tab === 'activity' && (
          <div className="space-y-5">
            <section aria-label="Task timeline" className="space-y-3">
              {detail.activities.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No activity recorded yet.
                </p>
              ) : (
                [...detail.activities]
                  .sort(
                    (a, b) =>
                      Date.parse(b.created_at) - Date.parse(a.created_at),
                  )
                  .map((item) => (
                    <article key={item.id} className="rounded-lg border p-3">
                      <header className="mb-2 flex flex-wrap justify-between gap-2 text-xs text-muted-foreground">
                        <span>
                          {authorName(item.author)} ·{' '}
                          {item.activity_type === 'comment'
                            ? 'Comment'
                            : item.activity_type.replace(/_/g, ' ')}
                        </span>
                        <time
                          title={new Date(item.created_at).toLocaleString(
                            'en-US',
                          )}
                        >
                          {relativeTime(item.created_at)}
                        </time>
                      </header>
                      <div className="task-prose">
                        <ChatMarkdown content={item.message} />
                      </div>
                      {item.attempt_id && (
                        <button
                          className="mt-2 min-h-9 text-xs text-primary underline"
                          onClick={() => {
                            openSession(item.attempt_id || '')
                          }}
                        >
                          Open session
                        </button>
                      )}
                    </article>
                  ))
              )}
            </section>
            <form
              className="space-y-2"
              onSubmit={(e) => {
                e.preventDefault()
                void working(async () => {
                  await tasksApi.comment(initial.id, comment.trim())
                  setComment('')
                  await reload()
                })
              }}
            >
              <label htmlFor="task-comment" className="text-sm font-medium">
                Add a comment
              </label>
              <textarea
                id="task-comment"
                className="field-control"
                rows={3}
                maxLength={6000}
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                disabled={busy}
              />
              <button
                className="action-primary"
                disabled={busy || !comment.trim()}
              >
                {busy ? 'Saving…' : 'Send comment'}
              </button>
            </form>
          </div>
        )}
      </main>
      <footer className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-t bg-card px-4 py-3 pb-[max(12px,env(safe-area-inset-bottom))] sm:px-6">
        {editing ? (
          <>
            <button
              className="action-secondary"
              disabled={busy}
              onClick={() => {
                if (dirty && !window.confirm('Discard unsaved task changes?'))
                  return
                drafts.delete(initial.id)
                setEditing(false)
              }}
            >
              Cancel edit
            </button>
            <button
              className="action-primary"
              type="submit"
              form="task-edit-form"
              disabled={busy || !loaded}
            >
              {busy ? 'Saving…' : 'Save changes'}
            </button>
          </>
        ) : (
          <>
            <button
              className="action-secondary"
              disabled={busy || running || !loaded}
              onClick={startEdit}
            >
              <Pencil className="h-4 w-4" />
              Edit
            </button>
            <div className="flex flex-wrap gap-2">
              {detail.status === 'done' ? (
                <button
                  className="action-secondary"
                  disabled={busy}
                  onClick={() => changeStatus('todo')}
                >
                  Reopen task
                </button>
              ) : (
                <>
                  {detail.status === 'review' && !running && (
                    <button
                      className="action-secondary"
                      disabled={busy || !loaded}
                      onClick={() => {
                        setApprovalNote(detail.complete_note || '')
                        setError('')
                        setModal('approve')
                      }}
                    >
                      Approve result
                    </button>
                  )}
                  <button
                    className="action-primary"
                    disabled={busy || !loaded || !!pending || launching}
                    onClick={taskAction}
                  >
                    {busy ? (
                      <RefreshCw className="h-4 w-4 animate-spin" />
                    ) : (
                      !running && <Play className="h-4 w-4" />
                    )}
                    {runLabel}
                  </button>
                </>
              )}
            </div>
          </>
        )}
      </footer>
      <ActionDialog
        open={modal === 'run'}
        title={
          detail.status === 'review'
            ? 'Request changes'
            : detail.attempts.length
              ? 'Retry task'
              : 'Start task'
        }
        onClose={() => setModal(null)}
        busy={busy}
      >
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            if (!agentId) return
            void working(() =>
              launch({
                request_id: crypto.randomUUID(),
                agent_id: agentId,
                revision: feedback.trim(),
              }),
            )
          }}
        >
          <p className="text-sm text-muted-foreground">
            Start a new session using the saved instructions and the selected
            agent. Earlier sessions and output are retained.
          </p>
          <label className="grid gap-1 text-sm">
            Agent
            <select
              aria-label="Execution agent"
              className="field-control"
              required
              value={agentId}
              onChange={(e) => setAgentId(e.target.value)}
              disabled={busy}
            >
              <option value="">Choose an agent</option>
              {agents
                .filter((a) => a.enabled !== false && a.status !== 'inactive')
                .map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
            </select>
          </label>
          {agents.every(
            (a) => a.enabled === false || a.status === 'inactive',
          ) && (
            <p className="text-sm">
              Add an agent in Settings → Agents before starting.
            </p>
          )}
          <label className="grid gap-1 text-sm">
            {detail.status === 'review'
              ? 'Required changes'
              : 'Additional instructions (optional)'}
            <textarea
              aria-label="Revision feedback"
              className="field-control"
              rows={4}
              maxLength={6000}
              required={detail.status === 'review'}
              value={feedback}
              onChange={(e) => setFeedback(e.target.value)}
              placeholder={
                detail.status === 'review'
                  ? 'Describe the gap, expected fix, and how to verify it.'
                  : 'Add context for this session without replacing the human instructions.'
              }
            />
          </label>
          <button
            className="action-primary w-full"
            disabled={
              busy ||
              !agentId ||
              (detail.status === 'review' && !feedback.trim())
            }
          >
            Start session
          </button>
        </form>
      </ActionDialog>
      <ActionDialog
        open={modal === 'approve'}
        title="Approve task result"
        onClose={() => setModal(null)}
        busy={busy}
      >
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            void working(async () => {
              await update({
                status: 'done',
                complete_note: approvalNote.trim(),
                expected_updated_at: detail.updated_at,
              })
              setModal(null)
              setNotice('Task result approved.')
            })
          }}
        >
          <p className="text-sm text-muted-foreground">
            Check that the result answers the original request, stays within
            scope, and uses the correct project workspace. Passing tests alone
            does not establish acceptance.
          </p>
          <div className="rounded-lg border bg-muted/30 p-3">
            <p className="mb-1 text-xs font-medium text-muted-foreground">
              Original request
            </p>
            <p className="max-h-32 overflow-y-auto whitespace-pre-wrap break-words text-sm">
              {detail.description || detail.title}
            </p>
          </div>
          {checklist.some((i) => !i.completed) && (
            <p className="text-sm text-destructive">
              Verify all acceptance criteria in Overview before approving.
            </p>
          )}
          <label className="grid gap-1 text-sm">
            Completion notes & verification
            <textarea
              className="field-control"
              aria-label="Approval evidence"
              rows={6}
              maxLength={16000}
              required
              value={approvalNote}
              onChange={(e) => setApprovalNote(e.target.value)}
              placeholder="How the result meets the original request, where outputs belong, checks performed, and limitations."
            />
          </label>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <button
            className="action-primary w-full"
            disabled={
              busy ||
              !approvalNote.trim() ||
              checklist.some((i) => !i.completed)
            }
          >
            Approve and mark Done
          </button>
        </form>
      </ActionDialog>
      <ActionDialog
        open={modal === 'delete'}
        title="Delete task?"
        onClose={() => setModal(null)}
        busy={busy}
      >
        <p className="mb-4 break-words text-sm">
          Delete {taskRef(detail.id)} {detail.title}? Its comments, attempts,
          and saved output will also be removed. This cannot be undone.
        </p>
        {error && (
          <p role="alert" className="mb-3 text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <button
            className="action-secondary"
            disabled={busy}
            onClick={() => setModal(null)}
          >
            Keep task
          </button>
          <button
            className="action-primary bg-destructive text-destructive-foreground hover:bg-destructive/90"
            disabled={busy || running}
            onClick={() =>
              void working(async () => {
                await onDelete(initial.id)
                drafts.delete(initial.id)
              })
            }
          >
            Delete task
          </button>
        </div>
      </ActionDialog>
      <ActionDialog
        open={modal === 'cancel'}
        title="Cancel session?"
        onClose={() => setModal(null)}
        busy={busy}
      >
        <p className="mb-4 text-sm">
          Stop the active agent process? Existing output is retained. Changes
          already made by the agent are not rolled back.
        </p>
        {error && (
          <p role="alert" className="mb-3 text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <button
            className="action-secondary"
            disabled={busy}
            onClick={() => setModal(null)}
          >
            Keep running
          </button>
          <button
            className="action-primary"
            disabled={busy}
            onClick={() =>
              void working(async () => {
                await tasksApi.cancel(initial.id)
                setModal(null)
                await reload()
                await onRunAgent(initial.id)
              })
            }
          >
            Cancel session
          </button>
        </div>
      </ActionDialog>
    </dialog>
  )
}
