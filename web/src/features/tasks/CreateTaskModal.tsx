import { useState, useRef, useEffect } from 'react'
import { X, AlertCircle } from 'lucide-react'
import type { Project, CreateTaskRequest } from '../../api/types'
import { agentsApi, type Agent } from '../../api/agents'
import { trapDialogFocus } from '../../lib/dialog'

interface Props {
  isOpen: boolean
  projects: Project[]
  activeProjectId?: string
  onClose: () => void
  onSubmit: (data: CreateTaskRequest, startAgentId?: string) => Promise<void>
}
interface Draft {
  prompt: string
  project: string
  agent: string
  requestId: string
}
function readDraft(key: string, project: string): Draft {
  try {
    const d = JSON.parse(sessionStorage.getItem(key) || 'null')
    if (
      d &&
      typeof d.prompt === 'string' &&
      typeof d.project === 'string' &&
      typeof d.agent === 'string' &&
      /^[0-9a-f-]{36}$/i.test(d.requestId)
    )
      return d
  } catch {}
  let agent = ''
  try {
    agent = localStorage.getItem('cuan-new-task-agent') || ''
  } catch {}
  return { prompt: '', project, agent, requestId: crypto.randomUUID() }
}

export function CreateTaskModal({
  isOpen,
  projects,
  activeProjectId,
  onClose,
  onSubmit,
}: Props) {
  const key = 'cuan-new-task:' + (activeProjectId || 'global')
  const [draft, setDraft] = useState<Draft>(() =>
      readDraft(key, activeProjectId || ''),
    ),
    [loading, setLoading] = useState(false),
    [error, setError] = useState(''),
    [agents, setAgents] = useState<Agent[]>([]),
    [agentsError, setAgentsError] = useState(false),
    [agentsLoading, setAgentsLoading] = useState(true)
  const dialog = useRef<HTMLDialogElement>(null),
    input = useRef<HTMLTextAreaElement>(null),
    submitting = useRef(false),
    mounted = useRef(false)
  useEffect(() => {
    if (!isOpen) return
    mounted.current = true
    setLoading(false)
    submitting.current = false
    let preferredProject = activeProjectId || ''
    try {
      if (!preferredProject) {
        const p = localStorage.getItem('cuan-new-task-project')
        if (
          projects.some((project) => project.id === p && !project.is_archived)
        )
          preferredProject = p || ''
      }
    } catch {}
    setDraft(readDraft(key, preferredProject))
    setError('')
    setAgentsLoading(true)
    let alive = true
    agentsApi
      .list()
      .then((list) => {
        if (alive) {
          setAgents(list)
          setAgentsError(false)
          setAgentsLoading(false)
          let preference: string | null = null
          try {
            preference = localStorage.getItem('cuan-new-task-agent')
          } catch {}
          setDraft((d) => {
            if (
              d.agent &&
              list.some(
                (a) =>
                  a.id === d.agent &&
                  a.enabled !== false &&
                  a.status !== 'inactive',
              )
            )
              return d
            if (preference === '') return { ...d, agent: '' }
            const available = list.filter(
              (a) => a.enabled !== false && a.status !== 'inactive',
            )
            const selected =
              available.find((a) => a.id === preference) ||
              available.find((a) => a.connection_state === 'verified') ||
              available[0]
            const next = { ...d, agent: selected?.id || '' }
            try {
              sessionStorage.setItem(key, JSON.stringify(next))
            } catch {}
            return next
          })
        }
      })
      .catch(() => {
        if (alive) {
          setAgents([])
          setAgentsError(true)
          setAgentsLoading(false)
          setDraft((d) => ({ ...d, agent: '' }))
        }
      })
    const previous = document.activeElement as HTMLElement | null
    dialog.current?.showModal()
    input.current?.focus()
    return () => {
      mounted.current = false
      alive = false
      dialog.current?.close()
      if (previous?.isConnected) previous.focus()
    }
  }, [isOpen, key, activeProjectId])
  const change = (patch: Partial<Draft>) =>
    setDraft((d) => {
      const next = { ...d, ...patch }
      try {
        if (patch.agent !== undefined)
          localStorage.setItem('cuan-new-task-agent', patch.agent)
        if (patch.project !== undefined)
          localStorage.setItem('cuan-new-task-project', patch.project)
        sessionStorage.setItem(key, JSON.stringify(next))
      } catch {}
      return next
    })
  const close = () => {
    if (loading) return
    if (draft.prompt.trim() && !window.confirm('Discard this task draft?'))
      return
    try {
      sessionStorage.removeItem(key)
    } catch {}
    onClose()
  }
  if (!isOpen) return null
  const eligible = agents.filter(
    (a) => a.enabled !== false && a.status !== 'inactive',
  )
  return (
    <dialog
      ref={dialog}
      aria-labelledby="create-task-heading"
      onKeyDown={trapDialogFocus}
      onCancel={(e) => {
        e.preventDefault()
        close()
      }}
      className="m-auto max-h-[90dvh] w-[calc(100%_-_24px)] max-w-xl overflow-y-auto rounded-2xl border border-border/40 bg-card/95 backdrop-blur-md p-4 text-card-foreground shadow-2xl backdrop:bg-black/50 sm:p-6"
    >
      <header className="mb-4 flex items-center justify-between gap-3">
        <h2 id="create-task-heading" className="text-lg font-semibold">
          New task
        </h2>
        <button
          type="button"
          aria-label="Close task form"
          className="icon-action -mr-1"
          onClick={close}
          disabled={loading}
        >
          <X className="h-5 w-5" />
        </button>
      </header>
      <label className="mb-4 flex items-center gap-2">
        <span className="text-xs text-muted-foreground">Project</span>
        <select
          aria-label="Project"
          className="min-h-11 min-w-0 max-w-[70%] rounded-lg border border-border/40 bg-muted/20 px-3 text-sm hover:bg-muted/30 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring"
          value={draft.project}
          onChange={(e) => change({ project: e.target.value })}
          disabled={loading}
        >
          <option value="">No project</option>
          {draft.project && !projects.some((p) => p.id === draft.project) && (
            <option value={draft.project}>Project unavailable</option>
          )}
          {projects
            .filter((p) => !p.is_archived)
            .map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
        </select>
      </label>
      {error && (
        <p
          role="alert"
          className="mb-3 flex items-start gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          {error}
        </p>
      )}
      <form
        onSubmit={async (e) => {
          e.preventDefault()
          if (submitting.current || !draft.prompt.trim()) return
          if (
            draft.project &&
            !projects.some((p) => p.id === draft.project && !p.is_archived)
          ) {
            setError('Choose an available project or No project.')
            return
          }
          if (draft.agent && !eligible.some((a) => a.id === draft.agent)) {
            setError('Choose an available agent or No agent.')
            return
          }
          submitting.current = true
          setLoading(true)
          setError('')
          try {
            await onSubmit(
              {
                title: '',
                description: draft.prompt.trim(),
                project_ids: draft.project ? [draft.project] : undefined,
                assignee_id: draft.agent,
                status: 'backlog',
                request_id: draft.requestId,
              },
              draft.agent || undefined,
            )
            try {
              sessionStorage.removeItem(key)
            } catch {}
            onClose()
          } catch (e) {
            if (mounted.current)
              setError(
                e instanceof Error
                  ? e.message
                  : 'Could not create task. Your draft is retained; try again.',
              )
          } finally {
            submitting.current = false
            if (mounted.current) setLoading(false)
          }
        }}
      >
        <div className="overflow-hidden rounded-xl border border-border/40 bg-muted/20 focus-within:border-border/60 focus-within:bg-muted/30 transition-colors focus-within:ring-2 focus-within:ring-ring">
          <textarea
            ref={input}
            aria-label="Task instructions"
            aria-describedby={agentsError ? 'new-task-help' : undefined}
            placeholder="Describe your task…"
            className="block min-h-32 w-full resize-y border-0 bg-transparent p-4 text-base leading-relaxed outline-none focus:ring-0 sm:text-[15px]"
            data-primary-composer
            rows={4}
            required
            maxLength={24000}
            value={draft.prompt}
            disabled={loading}
            onChange={(e) => change({ prompt: e.target.value })}
            onKeyDown={(e) => {
              if (
                !e.nativeEvent.isComposing &&
                e.key === 'Enter' &&
                (e.ctrlKey || e.metaKey)
              ) {
                e.preventDefault()
                e.currentTarget.form?.requestSubmit()
              }
            }}
          />
          <div className="flex items-center justify-between gap-2 px-3 pb-3">
            <label className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
              <span className="sr-only">Agent</span>
              <select
                aria-label="Agent"
                className="min-h-11 min-w-0 max-w-[150px] rounded-lg border border-border/40 bg-card/60 px-2.5 text-xs sm:max-w-[240px] hover:bg-card transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={draft.agent}
                onChange={(e) => change({ agent: e.target.value })}
                disabled={loading || agentsLoading}
              >
                <option value="">No agent</option>
                {draft.agent && !eligible.some((a) => a.id === draft.agent) && (
                  <option value={draft.agent}>Agent unavailable</option>
                )}
                {eligible.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </label>
            <button
              type="submit"
              aria-label="Create task"
              className="action-primary shrink-0"
              disabled={loading || agentsLoading || !draft.prompt.trim()}
            >
              {loading ? 'Creating…' : 'Create'}
            </button>
          </div>
        </div>
        {agentsError && (
          <p
            id="new-task-help"
            role="status"
            className="mt-3 text-xs text-muted-foreground"
          >
            Agents unavailable. You can still save this task.
          </p>
        )}
      </form>
    </dialog>
  )
}
