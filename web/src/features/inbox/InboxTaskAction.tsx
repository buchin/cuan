import { CardActions } from './CardActions'
import { SurfaceHeader } from '../../components/layout/SurfaceHeader'
import { InternalVisibility } from '../../components/ui/InternalVisibility'
import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch } from '../../api/client'
import { InboxTaskDraft } from '../../api/chat'
import { tasksApi } from '../../api/tasks'
import { projectsApi } from '../../api/projects'
import { Project, Task } from '../../api/types'
import { FileText, Terminal, Folder, FolderGit2, Clock3, Check, AlertCircle, Circle, X } from 'lucide-react'
import { StatusDot } from '../../components/ui/StatusDot'
import { workStatus, workDisplayStatus, taskDetailUrl, taskTerminalUrl } from '../tasks/task-ui'
import { taskAttention } from '../tasks/task-attention'


export function InboxTaskAction({
  conversationId,
  messageId,
  draft,
  linkedTask,
  onViewTask,
  onOpenTerminal,
  proposal,
  isCustomerRoom = false,
}: {
  conversationId: string
  messageId: string
  draft?: InboxTaskDraft
  linkedTask?: Task
  onContinueChat?: (suggestedText: string) => void
  proposal?: { description: string; accepted?: boolean; disabled?: boolean; onApprove: () => void; onOther: () => void }
  onViewTask?: (task: Task) => void
  onOpenTerminal?: (task: Task) => void
  onUseDraft?: (draftText: string) => void
  onRetry?: (task: Task) => Promise<void> | void
  isCustomerRoom?: boolean
}) {
  const { t } = useLanguage()

  const [projects, setProjects] = useState<Project[]>([])
  const [projectId, setProjectId] = useState(draft?.project_id || '')
  const [title, setTitle] = useState(draft?.title || '')
  const [description, setDescription] = useState(draft?.description || '')
  const [loading, setLoading] = useState(!!draft)
  const [busy, setBusy] = useState(false)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState('')
  const [created, setCreated] = useState<Task | null>(null)
  const [retry, setRetry] = useState(0)
  const existing = linkedTask || created
  const attemptId = existing?.latest_attempt?.id || ''
  const working = existing?.latest_attempt?.status === 'running' && workStatus(existing) === 'Working'
  const [terminalAttempt, setTerminalAttempt] = useState('')
  useEffect(() => {
    if (!working || !existing?.id || !attemptId) { setTerminalAttempt(''); return }
    let active = true
    const check = () => tasksApi.runtime(existing.id).then(runtime => {
      if (active) setTerminalAttempt(runtime.live_session?.attempt_id === attemptId ? attemptId : '')
    }).catch(() => { if (active) setTerminalAttempt('') })
    void check()
    const timer = setInterval(check, 5000)
    return () => { active = false; clearInterval(timer) }
  }, [existing?.id, attemptId, working])
  const hasLiveTerminal = Boolean(attemptId) && working && terminalAttempt === attemptId

  useEffect(() => {
    if (!draft || existing) return
    let live = true
    setLoading(true)
    setReady(false)
    setError('')
    Promise.all([
      projectsApi.list(),
      apiFetch<Task>(`/api/v1/tasks/msg_${messageId}`).catch((e) => {
        if (e.status === 404) return null
        throw e
      }),
    ])
      .then(([data, task]) => {
        if (!live) return
        const active = data.filter((p) => !p.is_archived)
        setProjects(active)
        setProjectId((current) =>
          active.some((p) => p.id === current)
            ? current
            : active.length === 1
              ? active[0].id
              : '',
        )
        setCreated(task)
        setReady(true)
      })
      .catch(() => {
        if (live) setError('Could not load projects or tasks. Try again.')
      })
      .finally(() => {
        if (live) setLoading(false)
      })
    return () => {
      live = false
    }
  }, [draft, existing, messageId, retry])

  if (!draft && !existing) return null
  if (existing) {
    const attention = taskAttention(existing)
    const state = workDisplayStatus(existing)
    const rawUpdate = existing.latest_progress || (state === 'Completed' ? existing.summary : undefined)
    const titlePrefix = existing.title.trim()
    const update = titlePrefix && rawUpdate?.trim().toLocaleLowerCase().startsWith(titlePrefix.toLocaleLowerCase()) && (!rawUpdate.trim().slice(titlePrefix.length) || /^[\s:,.–—-]/.test(rawUpdate.trim().slice(titlePrefix.length)))
      ? rawUpdate.trim().slice(titlePrefix.length).replace(/^[\s:,.–—-]+/, '')
      : rawUpdate
    const project = existing.project_name || existing.project_names?.join(', ')
    const ProjectIcon = /(?:https?:\/\/github\.com\/|git@github\.com:)/i.test(existing.repo_url || '') ? FolderGit2 : Folder
    const StateIcon = state === 'Completed' ? Check : (state === 'Queued' || state === 'Waiting for checks') ? Clock3 : state === 'Cancelled' ? X : state === 'Needs attention' ? AlertCircle : Circle

    return <section data-card="task-card" data-task-id={existing.id} aria-label={t('Task') + ': ' + existing.title} className="inbox-attachment mt-2.5 space-y-3 rounded-xl bg-muted/30 px-4 pt-3">
      <SurfaceHeader variant="card" className="-mx-4 -mt-3 rounded-t-xl bg-muted/50"
        title={<span className="inline-flex min-w-0 max-w-full items-center gap-1.5">{isCustomerRoom && <InternalVisibility className="-ml-3 -my-2" />}<ProjectIcon aria-hidden className="h-3.5 w-3.5 shrink-0" /><span className="truncate">{project || t('Task')}</span></span>}
        actions={<span data-task-status={state} className="inline-flex shrink-0 items-center gap-1.5 py-1">{(state === 'Working' || state === 'Checking result') ? <StatusDot status="working" size="xs" pulse aria-hidden /> : <StateIcon aria-hidden className={`h-3 w-3 ${state === 'Completed' ? 'text-[color:var(--status-success)]' : ''}`} />}{t(state)}</span>}
      />
      <p className="text-sm font-medium leading-5 line-clamp-2">{existing.title}</p>
      {attention ? <p className="break-words text-sm leading-5 text-muted-foreground"><span className="text-foreground">{t(attention.reason)}</span></p> : update && <p data-task-latest-update className="line-clamp-2 break-words text-xs leading-5 text-muted-foreground">{update}</p>}
      {attention && proposal && <p data-task-recommendation className="whitespace-pre-wrap break-words text-sm leading-5">{proposal.description}</p>}
      <CardActions className="-mx-4 px-4">
        {onViewTask ? <button type="button" data-task-detail className="mr-auto inline-flex min-h-11 items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground" onClick={() => onViewTask(existing)}><FileText aria-hidden className="h-4 w-4" />{t('Detail')}</button>
          : <Link data-task-detail className="mr-auto inline-flex min-h-11 items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground" to={taskDetailUrl(existing)}><FileText aria-hidden className="h-4 w-4" />{t('Detail')}</Link>}
        {attention && proposal && <>
          <button type="button" data-task-other disabled={proposal.disabled || proposal.accepted} className="inline-flex min-h-11 items-center gap-1.5 text-xs" onClick={proposal.onOther}>{t('Others')}</button>
          <button type="button" data-task-approve data-primary-action disabled={proposal.disabled || proposal.accepted} className="inline-flex min-h-11 items-center gap-1.5 text-xs font-medium disabled:opacity-50" onClick={proposal.onApprove}><Check aria-hidden className="h-4 w-4" />{t(proposal.accepted ? 'Approved' : 'Approve')}</button>
        </>}
        {hasLiveTerminal && (onOpenTerminal ? <button type="button" className="inline-flex min-h-11 items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground" onClick={() => onOpenTerminal(existing)}><Terminal aria-hidden className="h-4 w-4" />{t('Terminal')}</button>
          : <Link className="inline-flex min-h-11 items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground" to={taskTerminalUrl(existing)}><Terminal aria-hidden className="h-4 w-4" />{t('Terminal')}</Link>)}
      </CardActions>
    </section>
  }
  return (
    <form
      aria-label="Draft task"
      className="inbox-attachment mt-3 space-y-3 rounded-lg border border-border bg-card p-3 text-sm text-card-foreground"
      onSubmit={async (e) => {
        e.preventDefault()
        if (busy || loading || !ready) return
        setBusy(true)
        setError('')
        try {
          const data = await apiFetch<{ task: Task }>(
            `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/messages/${messageId}/task`,
            {
              method: 'POST',
              body: JSON.stringify({
                project_id: projectId,
                title: title.trim(),
                description: description.trim(),
              }),
            },
          )
          setCreated(data.task)
        } catch (e: any) {
          setError(
            e.message ||
              'Could not save the task. Try again; this will not create a duplicate.',
          )
        } finally {
          setBusy(false)
        }
      }}
    >
      <div>
        <p className="font-medium">Draft task</p>
        <p className="mt-1 text-muted-foreground">
          Review and edit before creating the task. It has not been saved or
          started.
        </p>
      </div>
      <label className="block">
        Task title
        <input
          className="field-control mt-1"
          maxLength={200}
          required
          disabled={busy}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </label>
      <label className="block">
        Description
        <textarea
          aria-label="Description"
          className="field-control mt-1 min-h-36 resize-y"
          maxLength={6000}
          required
          disabled={busy}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </label>
      <label className="block">
        Project
        <select
          aria-label="Project"
          className="field-control mt-1"
          required
          disabled={loading || busy || !ready}
          value={projectId}
          onChange={(e) => setProjectId(e.target.value)}
        >
          <option value="">Select a project</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </label>
      {loading && <p role="status">Loading projects and task status…</p>}
      {!loading && projects.length === 0 && !error && (
        <p>
          Add a project from{' '}
          <Link className="underline" to="/projects">
            {t('Projects')}
          </Link>
          .
        </p>
      )}
      {error && (
        <div role="alert">
          <p className="text-destructive">{error}</p>
          <button
            type="button"
            className="action-secondary mt-1"
            disabled={busy || loading}
            onClick={() => setRetry((n) => n + 1)}
          >
            Try again
          </button>
        </div>
      )}
      <button
        className="action-primary min-h-11 disabled:opacity-50"
        disabled={
          busy ||
          loading ||
          !ready ||
          !projectId ||
          !title.trim() ||
          !description.trim()
        }
      >
        {busy ? 'Creating…' : 'Create task'}
      </button>
      <p className="text-muted-foreground">
        The task will be added to the backlog. Creating it does not start an
        agent.
      </p>
    </form>
  )
}
