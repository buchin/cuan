import { TaskTimeline } from './TaskTimeline'
import { TaskDuration } from './TaskDuration'
import { StatusDot } from '../../components/ui/StatusDot'
import { IconButton } from '../../components/ui/IconButton'
import { GitHubPrerequisite } from './GitHubPrerequisite'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams, useNavigate, useLocation } from 'react-router-dom'
import {
  X,
  Maximize2,
  Minimize2,
  MoreHorizontal,
  Play,
  Terminal,
  Pencil,
  Copy,
  Trash2,
  RefreshCw,
  ExternalLink,
  Rocket,
  CheckCircle2,
  Clock3,
  Eye,
  AlertCircle,
  FolderOpen,
  ChevronDown,
  ShieldCheck,
  Flame,
  Bot,
  User,
  ListChecks,
  FileText,
  Sparkles,
  MessageSquare,
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
  taskExecutionActive,
  taskAgentQueued,
  workStatus,
  workDisplayStatus,
  statusLabel,
  taskRef,
  relativeTime,
  taskStatusBadgeClass,
  taskPriorityBadgeClass,
  parseTaskInstructions,
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
  const { t, language } = useLanguage()
  const id = language === 'id'
  const timelineScroll = useRef<HTMLElement>(null)
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
      'agent_cuan',
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
  const assistantName = agents.find((a) => a.id === 'agent_cuan')?.name || '@cuan'
  const hasReviewEvidence = Boolean(detail.summary?.trim() && detail.complete_note?.trim() &&
    checklist.every((item) => item.completed && 'evidence' in item && String(item.evidence || '').trim()))

  const attemptError = detail.latest_attempt?.error_message || ''
  const timedOut = detail.latest_attempt?.status === 'failed' && (
    (detail.latest_attempt.exit_code === 124 && attemptError.startsWith('Execution timed out after adaptive timeout (')) ||
    (detail.latest_attempt.exit_code === 1 && attemptError.startsWith('Task runner exceeded hard execution timeout (')) ||
    attemptError === 'The execution deadline was reached; saved work is preserved.'
  )
  const blockerText = timedOut
    ? (id ? 'Eksekusi mencapai batas waktu. Hasil belum terverifikasi.' : 'Execution reached its time limit. The result is not verified.')
    : detail.latest_attempt?.status === 'succeeded'
      ? (id ? 'Hasil belum memenuhi kriteria penyelesaian.' : 'The result has not met the completion criteria.')
      : (id ? 'Eksekusi belum berhasil. Hasil sementara tetap tersimpan.' : 'Execution did not succeed. Saved work is preserved.')
  const latestProgress = [...(detail.activities || [])].reverse().find((event) => event.activity_type === 'agent_report' && event.attempt_id === detail.latest_attempt?.id && !['done', 'review'].includes(event.new_status || '') && event.message?.trim())
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
        if (active) {
          setAgents(list)
          const assistant = list.find((a) => a.id === 'agent_cuan' && a.enabled !== false && a.status !== 'inactive')
          if (assistant) setAgentId(assistant.id)
        }
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
    setAgentId('agent_cuan')
    setFeedback('')
    setModal('run')
    setError('')
    setMenu(false)
  }
  const runLabel = running
    ? (detail.latest_attempt?.status === 'queued' ? 'Queued' : 'Open terminal')
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
  const authorName = (_id: string) => assistantName
  const field = (key: keyof Draft, value: string) =>
    setDraft((d) => ({ ...d, [key]: value }))
  function renderChecklistText(text: string) {
    if (!text) return null
    const parts = text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g)
    return parts.map((part, i) => {
      if (part.startsWith('**') && part.endsWith('**') && part.length > 4) {
        return (
          <strong key={i} className="font-semibold text-foreground">
            {part.slice(2, -2)}
          </strong>
        )
      }
      if (part.startsWith('`') && part.endsWith('`') && part.length > 2) {
        return (
          <code
            key={i}
            className="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px] text-primary"
          >
            {part.slice(1, -1)}
          </code>
        )
      }
      return <span key={i}>{part}</span>
    })
  }
  const emptyProse = (text: string | undefined, empty: string) => (
    <div className="task-prose min-w-0">
      {text ? (
        <ChatMarkdown content={text} />
      ) : (
        <p className="rounded-lg border border-dashed border-border/70 bg-muted/10 p-3 text-xs sm:text-sm text-muted-foreground italic">
          {empty}
        </p>
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
      <header className="px-4 pb-4 pt-2 sm:px-6 shrink-0 border-b border-border/40 bg-background">
        <div className="flex w-full items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
            <span className="shrink-0 font-mono text-[11px] font-semibold px-2 py-0.5 rounded-md bg-muted/80 text-foreground border border-border/60">{taskRef(detail.id)}</span>
            {selectedProject && (
              <span className="truncate text-xs text-muted-foreground font-medium">· {selectedProject.name}</span>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <IconButton
              className="hidden sm:inline-flex"
              label={full ? 'Collapse task view' : 'Expand task view'}
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
            </IconButton>
            <div className="relative">
              <IconButton

                label={id ? 'Menu tugas' : 'Task actions'}
                aria-expanded={menu}
                onClick={() => setMenu((v) => !v)}
              >
                <MoreHorizontal className="h-4 w-4" />
              </IconButton>
              {menu && (
                <div className="absolute right-0 top-full z-20 min-w-48 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg">
                  {detail.source_room_id && (
                    <a
                      href={`/inbox/${encodeURIComponent(detail.source_room_id)}`}
                      className="flex min-h-11 w-full items-center gap-2 px-3 text-left text-sm hover:bg-accent rounded-md transition-colors"
                      onClick={() => setMenu(false)}
                    >
                      <MessageSquare className="h-4 w-4" />
                      Open conversation
                    </a>
                  )}
                  <button
                    className="flex min-h-11 w-full items-center gap-2 px-3 text-left text-sm hover:bg-accent rounded-md transition-colors"
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
                    className="flex min-h-11 w-full items-center gap-2 px-3 text-left text-sm text-destructive hover:bg-destructive/10 rounded-md transition-colors disabled:opacity-50"
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
            <IconButton
              autoFocus
              label={id ? 'Tutup detail tugas' : 'Close task details'}
              onClick={close}
              disabled={busy}
            >
              <X />
            </IconButton>
          </div>
        </div>
        <h2 className="mt-2 break-words text-base sm:text-lg font-semibold tracking-tight text-foreground leading-snug">
          {detail.title}
        </h2>
        <div className="mt-2.5 flex flex-wrap items-center gap-2 text-xs">
          <span className={cn("inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold border", taskStatusBadgeClass(detail.status))}>
            {detail.status === 'done' ? (
              <CheckCircle2 className="h-3.5 w-3.5 text-[color:var(--status-success)]" aria-hidden />
            ) : taskExecutionActive(detail) ? (
              <StatusDot status="working" size="xs" aria-hidden />
            ) : taskAgentQueued(detail) ? (
              <Clock3 className="h-3.5 w-3.5 text-muted-foreground" aria-hidden />
            ) : detail.status === 'review' ? (
              <Eye className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400" aria-hidden />
            ) : detail.status === 'blocked' ? (
              <AlertCircle className="h-3.5 w-3.5 text-destructive" aria-hidden />
            ) : null}
            {t(workDisplayStatus(detail))}
          </span>
          <span className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground ml-auto sm:ml-0">
            <TaskDuration task={detail} fallback={<>Updated {relativeTime(detail.updated_at)}</>} />
            {detail.attempts && detail.attempts.length > 1 && (
              <span className="text-[10px] text-muted-foreground/70">
                · {detail.attempts.length} {id ? 'sesi' : 'sessions'}
              </span>
            )}
          </span>
        </div>
      </header>
      <nav
        aria-label="Task sections"
        className="flex shrink-0 gap-1 border-b bg-transparent px-3 sm:px-5"
      >
        {(
          [
            { id: 'overview', label: id ? 'Ringkasan' : 'Overview' },
            {
              id: 'sessions',
              label: (id ? 'Detail teknis' : 'Technical details'),
            },
            { id: 'activity', label: id ? 'Progres' : 'Progress' },
          ] as const
        ).map((item) => (
          <button
            key={item.id}
            aria-pressed={tab === item.id}
            disabled={editing}
            onClick={() => setTab(item.id)}
            className={cn(
              'min-h-11 border-b-2 px-3 text-sm bg-transparent',
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
      <main ref={timelineScroll}
        className={cn('min-h-0 min-w-0 flex-1 p-4 sm:p-6', 'overflow-y-auto')}
      >
        {!loaded && (
          <p role="status" className="mb-3 text-sm text-muted-foreground">
            Loading task details…
          </p>
        )}
        {tab === 'overview' && (
          <div className="space-y-6">
            {!editing && (
              <section aria-label={id ? 'Langkah berikutnya' : 'Next step'} className="space-y-1">
                <h3 className="text-sm font-medium">{detail.review_state ? (detail.review_state === 'running' ? (id ? `${assistantName} sedang memeriksa hasil` : `${assistantName} is checking the result`) : (id ? 'Menunggu pemeriksaan' : 'Waiting for review')) : running
                  ? (id ? `${assistantName} sedang mengerjakan tugas ini` : `${assistantName} is working on this task`)
                  : detail.status === 'done' ? (id ? 'Tugas selesai' : 'Task completed')
                  : detail.status === 'review' ? (id ? 'Perlu ditinjau' : 'Ready for review')
                  : detail.status === 'blocked' ? (timedOut ? (id ? 'Batas waktu tercapai' : 'Time limit reached') : (id ? 'Pekerjaan terhenti' : 'Work is blocked'))
                  : (id ? 'Siap dikerjakan' : 'Ready to start')}</h3>
                <p className="text-sm leading-relaxed text-muted-foreground">{detail.review_state ? '' : running
                  ? (latestProgress?.message || (detail.latest_attempt?.status === 'queued' ? (id ? 'Menunggu giliran agen.' : 'Waiting for an agent.') : ''))
                  : detail.status === 'done' ? (id ? 'Lihat hasil dan bukti penyelesaiannya di bawah.' : 'See the result and verification below.')
                  : detail.status === 'review' ? (id ? (hasReviewEvidence ? 'Periksa ringkasan dan bukti sebelum menyetujui. Jika belum sesuai, minta perbaikan.' : `Bukti penyelesaian belum lengkap. Minta perbaikan agar ${assistantName} melengkapi hasilnya.`) : 'Check the summary and evidence before approving, or request changes.')
                  : detail.status === 'blocked' ? blockerText
                  : (id ? `Periksa instruksi, lalu mulai agar ${assistantName} mengerjakannya.` : `Check the instructions, then start the task with ${assistantName}.`)}</p>
                {detail.status === 'blocked' && <button type="button" className="inline-flex min-h-11 items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground" onClick={() => setTab('sessions')}><FileText aria-hidden="true" className="h-4 w-4" />{id ? 'Lihat kendala' : 'View issue'}</button>}
              </section>
            )}
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
                <section>
                  <details className="group space-y-3">
                    <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between gap-2 text-sm font-medium hover:text-primary"><span>{id ? 'Pengaturan tugas' : 'Task settings'}</span><ChevronDown aria-hidden="true" className="h-4 w-4 text-muted-foreground transition-transform group-open:rotate-180" /></summary>
                  <div className="grid grid-cols-3 gap-3">
                    <div className="pb-3 sm:pb-0 sm:pr-4 flex flex-col justify-between">
                      <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground mb-1.5 flex items-center gap-1.5">
                        <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                        Status
                      </span>
                      <select
                        aria-label="Task status"
                        className="field-control min-h-11 text-xs font-medium rounded-lg border-input bg-background/80 hover:bg-background transition-colors"
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
                    </div>
                    <div className="py-3 sm:py-0 sm:px-4 flex flex-col justify-between">
                      <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground mb-1.5 flex items-center gap-1.5">
                        <Flame className="h-3.5 w-3.5 text-muted-foreground" />
                        Priority
                      </span>
                      <div className="flex items-center min-h-10">
                        <span
                          className={cn(
                            'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold border capitalize',
                            taskPriorityBadgeClass(detail.priority),
                          )}
                        >
                          {detail.priority || 'Normal'}
                        </span>
                      </div>
                    </div>
                    <div className="pt-3 sm:pt-0 sm:pl-4 flex flex-col justify-between">
                      <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground mb-1.5 flex items-center gap-1.5">
                        <User className="h-3.5 w-3.5 text-muted-foreground" />
                        Assignee
                      </span>
                      <div className="flex items-center min-h-10">
                        <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-muted/60 border border-border/50 text-foreground">
                          <Bot className="h-3.5 w-3.5 text-primary" />
                          <span className="truncate max-w-[140px] font-mono">
                            {authorName(
                              detail.assignee_id || detail.assignee || '',
                            ) || 'Unassigned'}
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>
                  </details>
                  {detail.pr_url && /^https?:\/\//i.test(detail.pr_url) && (
                    <div className="mt-3.5 rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-3.5 space-y-2.5">
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

                {detail.summary?.trim() && <section className="space-y-2">
                  <h3 className="text-sm font-medium flex items-center gap-2">
                    <Sparkles className="h-4 w-4 text-primary" />
                    <span>{id ? 'Ringkasan pekerjaan' : 'Work summary'}</span>
                  </h3>
                  {emptyProse(detail.summary, id ? 'Belum ada ringkasan pekerjaan.' : 'No work summary recorded yet.')}
                </section>}
                {detail.complete_note?.trim() && <section className="space-y-2">
                  <h3 className="text-sm font-medium flex items-center gap-2">
                    <ShieldCheck className="h-4 w-4 text-primary" />
                    <span>{id ? 'Hasil & bukti' : 'Result & verification'}</span>
                  </h3>
                  {emptyProse(
                    detail.complete_note,
                    id ? 'Belum ada hasil terverifikasi. Jangan setujui sebelum hasil dan buktinya tersedia.' : 'No verified result yet. Check the result and evidence before approval.',
                  )}
                </section>}

                {(() => {
                  const parsed = parseTaskInstructions(detail.description)
                  return (
                    <details className="space-y-3">
                      <summary className="flex min-h-11 cursor-pointer items-center justify-between gap-2">
                        <h3 className="text-xs sm:text-sm font-bold uppercase tracking-wider text-muted-foreground flex items-center gap-2">
                          <FileText className="h-4 w-4 text-primary" />
                          <span>{id ? 'Instruksi awal' : 'Original instructions'}</span><ChevronDown className="h-4 w-4 text-muted-foreground" />
                        </h3>

                      </summary>
                      {emptyProse(
                        parsed.cleanText || detail.description,
                        'No instructions yet. Edit this task to describe the work.',
                      )}
                      {parsed.isDelegation && parsed.metadata && (
                        <details className="group rounded-xl border border-border/80 bg-muted/20 overflow-hidden text-xs transition-colors">
                          <summary className="flex min-h-11 cursor-pointer select-none items-center justify-between gap-3 px-3.5 py-2.5 hover:bg-muted/40 transition-colors">
                            <div className="flex items-center gap-2.5 min-w-0">
                              <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                                <ShieldCheck className="h-4 w-4 text-primary" />
                              </span>
                              <div className="min-w-0">
                                <div className="flex items-center gap-2 flex-wrap">
                                  <span className="font-semibold text-foreground">
                                    Metadata Delegasi & Scope Digest
                                  </span>
                                  {parsed.metadata.scopeDigest && (
                                    <span className="font-mono text-[10px] rounded bg-muted/80 px-1.5 py-0.5 border border-border/60 text-muted-foreground">
                                      digest: {parsed.metadata.scopeDigest.slice(0, 8)}…
                                    </span>
                                  )}
                                </div>
                                <p className="text-[11px] text-muted-foreground">
                                  Cuan managed delegation · Rincian otorisasi & verifikasi
                                </p>
                              </div>
                            </div>
                            <div className="flex items-center gap-1.5 shrink-0 text-muted-foreground">
                              <span className="text-[11px] hidden sm:inline group-open:hidden">
                                Detail
                              </span>
                              <ChevronDown className="h-4 w-4 transition-transform duration-200 group-open:rotate-180" />
                            </div>
                          </summary>
                          <div className="border-t border-border/60 bg-muted/10 p-3.5 space-y-3 font-sans">
                            {parsed.metadata.scopeDigest && (
                              <div className="space-y-1">
                                <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                                  Approved Scope Digest
                                </span>
                                <div className="flex items-center justify-between gap-2 rounded-lg border bg-background/80 p-2 font-mono text-[11px]">
                                  <span className="truncate">{parsed.metadata.scopeDigest}</span>
                                  <button
                                    type="button"
                                    onClick={async () => {
                                      try {
                                        await navigator.clipboard.writeText(parsed.metadata?.scopeDigest || '')
                                        setNotice('Scope digest copied.')
                                      } catch {
                                        setError('Could not copy digest.')
                                      }
                                    }}
                                    aria-label="Copy scope digest"
                                    className="icon-action h-7 w-7 min-h-7 min-w-7 shrink-0"
                                  >
                                    <Copy className="h-3 w-3" />
                                  </button>
                                </div>
                              </div>
                            )}

                            {parsed.metadata.approvalMessage && (
                              <div className="space-y-1">
                                <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                                  Approval Message ID
                                </span>
                                <div className="flex items-center justify-between gap-2 rounded-lg border bg-background/80 p-2 font-mono text-[11px]">
                                  <span className="truncate">{parsed.metadata.approvalMessage}</span>
                                  <button
                                    type="button"
                                    onClick={async () => {
                                      try {
                                        await navigator.clipboard.writeText(parsed.metadata?.approvalMessage || '')
                                        setNotice('Approval message ID copied.')
                                      } catch {
                                        setError('Could not copy approval message ID.')
                                      }
                                    }}
                                    aria-label="Copy approval message ID"
                                    className="icon-action h-7 w-7 min-h-7 min-w-7 shrink-0"
                                  >
                                    <Copy className="h-3 w-3" />
                                  </button>
                                </div>
                              </div>
                            )}

                            {parsed.metadata.userAuth && (
                              <div className="space-y-1">
                                <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                                  User Authorization
                                </span>
                                <div className="rounded-lg border bg-background/80 p-2 text-xs text-foreground leading-relaxed">
                                  {parsed.metadata.userAuth}
                                </div>
                              </div>
                            )}

                            {parsed.metadata.conversationId && (
                              <div className="space-y-1">
                                <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                                  Source Conversation
                                </span>
                                <div className="flex items-center justify-between gap-2 rounded-lg border bg-background/80 p-2 text-xs">
                                  <span className="font-mono text-[11px] truncate">
                                    {parsed.metadata.conversationId}
                                  </span>
                                  <a
                                    href={`/inbox/${encodeURIComponent(parsed.metadata.conversationId)}`}
                                    className="inline-flex items-center gap-1 text-primary hover:underline text-xs font-medium shrink-0"
                                  >
                                    <span>Buka Chat</span>
                                    <ExternalLink className="h-3 w-3" />
                                  </a>
                                </div>
                              </div>
                            )}
                          </div>
                        </details>
                      )}
                    </details>
                  )
                })()}

                {checklist.length > 0 && (() => {
                  const completedCount = checklist.filter((i) => i.completed).length
                  const totalCount = checklist.length
                  const percent = totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0
                  const isAllDone = totalCount > 0 && completedCount === totalCount

                  return (
                    <section className="border-t border-border/40 pt-5 space-y-3">
                      <div className="flex flex-col gap-2">
                        <div className="flex items-center justify-between gap-2">
                          <div className="flex items-center gap-2 min-w-0">
                            <span
                              className={cn(
                                'flex h-7 w-7 shrink-0 items-center justify-center rounded-lg',
                                isAllDone
                                  ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400'
                                  : 'bg-primary/10 text-primary',
                              )}
                            >
                              <ListChecks className="h-4 w-4" />
                            </span>
                            <h3 className="text-sm font-bold text-foreground">
                              {id ? 'Kriteria selesai' : 'Acceptance criteria'}
                            </h3>
                          </div>

                          <span
                            className={cn(
                              'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold border',
                              isAllDone
                                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30'
                                : 'bg-muted text-muted-foreground border-border/60',
                            )}
                          >
                            {isAllDone && (
                              <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                            )}
                            <span>
                              {completedCount} / {totalCount} ({percent}%)
                            </span>
                          </span>
                        </div>

                        {/* Visual Progress Bar Track */}
                        <div
                          className="h-2 w-full rounded-full bg-muted/80 overflow-hidden"
                          role="progressbar"
                          aria-valuenow={percent}
                          aria-valuemin={0}
                          aria-valuemax={100}
                          aria-label="Agent checklist progress"
                        >
                          <div
                            className={cn(
                              'h-full rounded-full transition-all duration-300 ease-out',
                              isAllDone
                                ? 'bg-emerald-500 dark:bg-emerald-400'
                                : 'bg-primary',
                            )}
                            style={{ width: `${percent}%` }}
                          />
                        </div>

                        <p className="text-xs text-muted-foreground">
                          {isAllDone
                            ? 'Semua kriteria selesai terverifikasi oleh agen.'
                            : `${completedCount} dari ${totalCount} kriteria dilaporkan agen.`}
                        </p>
                      </div>

                      {/* Checklist items with min-44px touch targets and markdown formatting */}
                      <div className="divide-y divide-border/40 pt-1">
                        {checklist.map((item, index) => (
                          <label
                            key={item.id || index}
                            className={cn(
                              'flex min-h-11 cursor-pointer items-start gap-3 rounded-lg px-2.5 py-2.5 transition-colors hover:bg-muted/40',
                              item.completed && 'bg-muted/15',
                            )}
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
                              className="mt-0.5 h-4 w-4 shrink-0 rounded border-input text-primary focus:ring-primary focus:ring-offset-background"
                            />
                            <span
                              className={cn(
                                'min-w-0 flex-1 break-words text-xs sm:text-sm leading-relaxed',
                                item.completed
                                  ? 'text-muted-foreground line-through decoration-muted-foreground/50'
                                  : 'text-foreground font-medium',
                              )}
                            >
                              {renderChecklistText(item.text)}
                              {'evidence' in item && item.evidence && (
                                <span className="mt-1 block text-xs font-normal text-muted-foreground no-underline">{String(item.evidence)}</span>
                              )}
                            </span>
                          </label>
                        ))}
                      </div>
                    </section>
                  )
                })()}

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
                  <details className="group rounded-xl border border-border/80 bg-muted/20 p-3 text-xs sm:text-sm transition-colors">
                    <summary className="flex cursor-pointer select-none items-center justify-between font-medium text-foreground hover:text-primary">
                      <span>Additional execution evidence</span>
                      <ChevronDown className="h-4 w-4 text-muted-foreground transition-transform group-open:rotate-180" />
                    </summary>
                    <div className="mt-3 space-y-3 pt-3 border-t border-border/60">
                      {emptyProse(detail.diff_summary, '')}
                      {emptyProse(detail.test_summary, '')}
                    </div>
                  </details>
                )}
                {detail.source_room_id && (
                  <div className="pt-1">
                    <a
                      href={`/inbox/${encodeURIComponent(detail.source_room_id)}`}
                      className="action-secondary min-h-11 min-w-[44px] inline-flex items-center gap-2 text-xs sm:text-sm"
                    >
                      <MessageSquare className="h-4 w-4" />
                      <span>Open source conversation</span>
                    </a>
                  </div>
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
            <TaskTimeline task={detail} scrollRef={timelineScroll}/>
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
      <footer className="sticky bottom-0 z-30 shrink-0 border-t border-border/80 bg-card/95 backdrop-blur-md px-3.5 sm:px-6 py-3 pb-[max(12px,env(safe-area-inset-bottom))] shadow-[0_-4px_16px_rgba(0,0,0,0.06)] dark:shadow-[0_-4px_16px_rgba(0,0,0,0.25)]">
        {editing ? (
          <div className="flex w-full items-center justify-between gap-3">
            <button
              className="action-secondary min-h-11 min-w-[44px] px-4 text-xs sm:text-sm font-medium"
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
              className="action-primary min-h-11 min-w-[44px] px-5 text-xs sm:text-sm font-medium"
              type="submit"
              form="task-edit-form"
              disabled={busy || !loaded}
            >
              {busy ? 'Saving…' : 'Save changes'}
            </button>
          </div>
        ) : (
          <div className="flex w-full items-center justify-between gap-2.5">
            <button
              className="action-secondary min-h-11 min-w-[44px] px-3.5 sm:px-4 text-xs sm:text-sm font-medium shrink-0"
              disabled={busy || running || !loaded}
              onClick={startEdit}
            >
              <Pencil className="h-4 w-4" />
              <span>Edit</span>
            </button>
            <div className="flex items-center gap-2 shrink-0 sm:shrink">
              {detail.status === 'done' ? (
                <button
                  className="action-secondary min-h-11 min-w-[44px] px-4 text-xs sm:text-sm font-medium"
                  disabled={busy}
                  onClick={() => changeStatus('todo')}
                >
                  Reopen task
                </button>
              ) : (
                <>
                  {detail.status === 'review' && !running && !detail.review_state && (
                    <button
                      className="action-secondary min-h-11 min-w-[44px] px-3.5 sm:px-4 text-xs sm:text-sm font-medium"
                      disabled={busy || !loaded || !hasReviewEvidence}
                      title={!hasReviewEvidence ? (id ? 'Hasil dan bukti belum lengkap' : 'Result and evidence are incomplete') : undefined}
                      onClick={() => {
                        setApprovalNote(detail.complete_note || '')
                        setError('')
                        setModal('approve')
                      }}
                    >
                      {id ? 'Setujui hasil' : 'Approve result'}
                    </button>
                  )}
                  {workStatus(detail) === 'Needs attention' && detail.source_room_id ? null : <button
                    className="action-primary min-h-11 min-w-[44px] px-4 sm:px-5 text-xs sm:text-sm font-medium shadow-sm"
                    disabled={busy || !loaded || !!pending || launching || !!detail.review_state || detail.latest_attempt?.status === 'queued'}
                    onClick={taskAction}
                  >
                    {busy ? (
                      <RefreshCw className="h-4 w-4 animate-spin" />
                    ) : detail.review_state ? (
                      <StatusDot status={detail.review_state === 'running' ? 'working' : 'idle'} size="xs" aria-hidden />
                    ) : (
                      running ? (detail.latest_attempt?.status !== 'queued' && <Terminal aria-hidden="true" className="h-4 w-4" />) : <Play className="h-4 w-4" />
                    )}
                    <span>{detail.review_state ? (detail.review_state === 'running' ? (id ? 'Memeriksa hasil' : 'Checking result') : (id ? 'Menunggu pemeriksaan' : 'Waiting for review')) : id ? (running ? (detail.latest_attempt?.status === 'queued' ? 'Dalam antrean' : 'Buka terminal') : detail.status === 'review' ? 'Minta perbaikan' : detail.attempts.length ? 'Coba lagi' : 'Mulai') : runLabel}</span>
                  </button>}
                </>
              )}
            </div>
          </div>
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
            {id ? `${assistantName} melanjutkan dari instruksi yang tersimpan. Riwayat pekerjaan sebelumnya tetap tersedia.` : `${assistantName} uses the saved instructions. Previous work remains available.`}
          </p>
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
                try {
                  await tasksApi.cancel(initial.id)
                } catch (e) {
                  const message = e instanceof Error ? e.message : ''
                  if (
                    !message.includes('cannot cancel an attempt with status') &&
                    !message.includes('no attempt is available to cancel')
                  ) {
                    throw e
                  }
                }
                setModal(null)
                await reload()
              })
            }
          >
            {busy ? 'Cancelling…' : 'Cancel session'}
          </button>
        </div>
      </ActionDialog>
    </dialog>
  )
}
