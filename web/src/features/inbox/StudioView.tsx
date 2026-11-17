import { useLanguage } from '../../context/LanguageContext'
import { AttachmentPicker, AttachmentPreviews, useAttachments } from './Attachments'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { ArrowRight, LoaderCircle } from 'lucide-react'
import { apiFetch } from '../../api/client'
import { TeamView } from '../team/TeamView'
import { agentsApi, type Agent, isModelVisionCapable } from '../../api/agents'
import { projectsApi } from '../../api/projects'
import type { Project } from '../../api/types'
import { StudioProjectSelector } from './StudioProjectSelector'

export function StudioView({
  onAccepted,
  onConversationCreated,
  agentSetup,
  onAgentConnected,
  onCreateProject,
  onRetrySetup,
  checkRuntime = true,
  autoFocus = false,
  projects: initialProjects,
}: {
  onAccepted: () => void
  onConversationCreated?: (conversationId: string) => void
  agentSetup: 'loading' | 'needed' | 'ready' | 'error'
  firstGoal: boolean
  hasProject: boolean
  onAgentConnected: () => void
  onCreateProject: () => void
  checkRuntime?: boolean
  onRetrySetup: () => void
  autoFocus?: boolean
  projects?: Project[]
}) {
  const { t } = useLanguage()
  const [connectRequested, setConnectRequested] = useState(false)
  const [addAgentModalOpen, setAddAgentModalOpen] = useState(false)
  const [addAgentTrigger, setAddAgentTrigger] = useState(false)

  const [installation, setInstallation] = useState<'checking' | 'available' | 'incomplete' | 'unsupported' | 'error'>('checking')
  const [checkVersion, setCheckVersion] = useState(0)
  useEffect(() => {
    let active = true
    if (!checkRuntime) { setInstallation('available'); return }
    setInstallation('checking')
    apiFetch<{ runtime_prerequisites?: { cli_available: boolean; sandbox_available: boolean; installation_supported: boolean } }>('/api/v1/agents/auth-status?harness=agy')
      .then((result) => {
        if (!active) return
        const runtime = result.runtime_prerequisites
        setInstallation(!runtime ? 'error' : !runtime.installation_supported ? 'unsupported' :
          runtime.cli_available && runtime.sandbox_available ? 'available' : 'incomplete')
      })
      .catch(() => { if (active) setInstallation('error') })
    return () => { active = false }
  }, [checkVersion, checkRuntime])
  const retrySetup = () => {
    setCheckVersion((version) => version + 1)
    onRetrySetup()
  }
  const navigate = useNavigate()
  const uploads = useAttachments('cuan-studio-attachments')
  const [goal, setGoal] = useState(() => {
    try {
      return localStorage.getItem('cuan-studio-draft') || ''
    } catch {
      return ''
    }
  })
  const goalInput = useRef<HTMLTextAreaElement>(null)
  useLayoutEffect(() => {
    const resize = () => {
      const input = goalInput.current
      if (!input) return
      input.style.height = '0px'
      input.style.height = `${Math.min(input.scrollHeight, 320)}px`
    }
    resize()
    window.addEventListener('resize', resize)
    return () => window.removeEventListener('resize', resize)
  }, [goal])
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [retryCount, setRetryCount] = useState(0)
  const [agents, setAgents] = useState<Agent[]>([])
  const [searchParams] = useSearchParams()
  const selectedAgentId = agents.find((a) => a.id === 'agent_cuan')?.id || ''

  const [projects, setProjects] = useState<Project[]>(initialProjects || [])
  useEffect(() => {
    if (initialProjects && initialProjects.length > 0) {
      setProjects(initialProjects)
      return
    }
    let active = true
    projectsApi.list()
      .then((list) => { if (active && Array.isArray(list)) setProjects(list) })
      .catch(() => {})
    return () => { active = false }
  }, [initialProjects])

  const [selectedProjectIds, setSelectedProjectIds] = useState<string[]>(() => {
    const projectParam = searchParams.get('project')
    if (projectParam) return [projectParam]
    try {
      const stored = localStorage.getItem('cuan-studio-last-projects')
      if (stored) {
        const parsed = JSON.parse(stored)
        if (Array.isArray(parsed) && parsed.length > 0) return parsed
      }
    } catch {}
    return []
  })

  // When projects load, default to first project if nothing selected or stored project no longer exists
  useEffect(() => {
    if (projects.length > 0) {
      setSelectedProjectIds((prev) => {
        if (prev.length > 0 && prev.some((id) => projects.some((p) => p.id === id))) {
          return prev
        }
        const defaultId = projects[0].id
        try {
          localStorage.setItem('cuan-studio-last-projects', JSON.stringify([defaultId]))
        } catch {}
        return [defaultId]
      })
    }
  }, [projects])

  const handleToggleProject = (id: string) => {
    setSelectedProjectIds((prev) => {
      let next: string[]
      if (prev.includes(id)) {
        next = prev.filter((p) => p !== id)
        if (next.length === 0 && projects.length > 0) {
          next = [id]
        }
      } else {
        next = [...prev, id]
      }
      try {
        localStorage.setItem('cuan-studio-last-projects', JSON.stringify(next))
      } catch {}
      return next
    })
  }

  const selectedProjectId = selectedProjectIds[0] || ''

  useEffect(() => {
    const projectParam = searchParams.get('project')
    if (projectParam) {
      setTimeout(() => {
        goalInput.current?.focus()
      }, 60)
    } else if (autoFocus) {
      setTimeout(() => {
        goalInput.current?.focus()
      }, 60)
    }
  }, [searchParams, autoFocus])

  const selectedAgent = agents.find((a) => a.id === selectedAgentId)
  const supportsVision = selectedAgent?.id === 'agent_cuan' ? true : selectedAgent
    ? isModelVisionCapable(selectedAgent.model_id, selectedAgent.provider || selectedAgent.harness)
    : true

  useEffect(() => {
    if (!selectedAgent || selectedAgent.id === 'agent_cuan') return
    const vision = isModelVisionCapable(selectedAgent.model_id, selectedAgent.provider || selectedAgent.harness)
    if (!vision && uploads.hasImages()) {
      uploads.removeImages()
      setError(
        t('Model ini tidak mendukung input gambar (vision). Lampiran gambar telah dibatalkan.')
      )
    }
  }, [selectedAgentId, selectedAgent])

  const update = (text: string) => {
    setGoal(text)
    try {
      localStorage.setItem('cuan-studio-draft', text)
    } catch {
      /* Submission surfaces unavailable storage. */
    }
  }

  useEffect(() => {
    let active = true
    agentsApi.list()
      .then((list) => { if (active) setAgents(list) })
      .catch(() => { if (active) setAgents([]) })
    return () => { active = false }
  }, [])

  async function submit() {
    if (busy || uploads.busy || installation !== 'available') return
    if (agentSetup === 'needed') {
      setConnectRequested(true)
      return
    }
    if (agentSetup === 'error') {
      retrySetup()
      return
    }
    if (agentSetup === 'loading' || (!goal.trim() && !uploads.items.length)) return
    setBusy(true)
    setError('')
    try {
      let saved = null
      try {
        saved = JSON.parse(
          localStorage.getItem('cuan-studio-pending') || 'null',
        )
      } catch {
        /* An invalid pending draft cannot authorize a replay. */
      }
      const request =
        saved?.goal === goal.trim() &&
        (saved?.agent_id || '') === selectedAgentId &&
        (saved?.project_id || '') === selectedProjectId &&
        JSON.stringify(saved?.project_ids || []) === JSON.stringify(selectedProjectIds) &&
        JSON.stringify(saved.attachment_ids || []) ===
          JSON.stringify(uploads.items.map((a) => a.id))
          ? saved
          : {
              goal: goal.trim(),
              request_id: crypto.randomUUID(),
              attachment_ids: uploads.items.map((a) => a.id),
              ...(selectedAgentId ? { agent_id: selectedAgentId } : {}),
              ...(selectedProjectId ? { project_id: selectedProjectId } : {}),
              ...(selectedProjectIds.length > 0 ? { project_ids: selectedProjectIds } : {}),
            }
      try {
        localStorage.setItem('cuan-studio-pending', JSON.stringify(request))
      } catch {
        throw new Error(
          t(
            'Enable browser storage so Cuan can safely retry this request. Your goal is kept on this page.',
          ),
        )
      }
      const result = await apiFetch<{ conversation_id: string; retry_count?: number }>(
        '/api/v1/studio/goals',
        { method: 'POST', body: JSON.stringify(request) },
      )
      if (result.retry_count != null) {
        setRetryCount(result.retry_count)
      }
      localStorage.removeItem('cuan-studio-pending')
      localStorage.removeItem('cuan-studio-draft')
      setGoal('')
      uploads.clear()
      onAccepted()
      if (onConversationCreated) {
        onConversationCreated(result.conversation_id)
      } else {
        navigate('/inbox/' + encodeURIComponent(result.conversation_id))
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      if (msg.includes('provider_quota_exhausted') || msg.includes('quota')) {
        setError(
          t('AI capacity is temporarily full. Your message is saved.'),
        )
      } else {
        setError(
          t(msg)
        )
      }
    } finally {
      setBusy(false)
    }
  }
  return (
    <section aria-label={t('New work')} className="mx-auto w-full max-w-[75ch]">
      <div
        data-studio-goal-panel
        className="flex min-h-[calc(100dvh-180px)] flex-col justify-center py-6 md:min-h-[calc(100dvh-150px)] md:py-2"
      >
        <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          {t('What would you like to get done today?')}
        </h2>
        {installation === 'incomplete' || installation === 'unsupported' ? (
          <aside aria-label={t('Server setup')} className="mt-4 rounded-lg border p-3 text-sm">
            <p className="font-medium">{t('Finish setting up Cuan')}</p>
            <p className="mt-1 text-muted-foreground">{t(installation === 'unsupported'
              ? 'This installation does not yet support AI execution. Your goal is saved.'
              : 'The AI runtime is not installed yet. Your goal is saved.')}</p>
            <details className="mt-2">
              <summary className="cursor-pointer text-xs text-muted-foreground">{t('For the server administrator')}</summary>
              <p className="mt-2 text-xs text-muted-foreground">{t('Complete the AI runtime setup under the account that runs Cuan, then check again.')}</p>
              <a className="inline-flex min-h-11 items-center underline" href="https://github.com/buchin/cuan/blob/sprint26-agents/docs/guides/onboarding.md#connect-an-agent-and-verify-readiness" target="_blank" rel="noreferrer">{t('Installation guide')}</a>
            </details>
            <button type="button" className="action-secondary mt-2" onClick={retrySetup}>{t('Check again')}</button>
          </aside>
        ) : installation === 'error' || agentSetup === 'error' ? (
          <div role="alert" className="mt-4 rounded-lg border p-3 text-sm">
            <p>{t('Could not check your setup. Your goal is kept.')}</p>
            <button type="button" className="action-secondary mt-2" onClick={retrySetup}>{t('Try again')}</button>
          </div>
        ) : agentSetup === 'needed' && installation === 'available' ? (
          <p className="mt-4 text-sm text-muted-foreground">{t('Write your goal, then connect your AI to get started. Your draft will stay here.')}</p>
        ) : null}
        <form
          className="mt-4 space-y-3 md:mt-6"
          onPaste={(e) => {
            if (!busy) uploads.onPaste(e)
          }}
          onDrop={(e) => {
            e.preventDefault()
            if (!busy) uploads.onDrop(e)
          }}
          onDragOver={uploads.onDragOver}
          onSubmit={(e) => {
            e.preventDefault()
            void submit()
          }}
        >
          <label htmlFor="studio-goal" className="sr-only">
            {t('Your goal')}
          </label>
          <div className="relative rounded-2xl border border-input bg-background focus-within:border-muted-foreground/60 focus-within:ring-1 focus-within:ring-muted-foreground/30">
            <textarea
              data-primary-composer
              id="studio-goal"
              ref={goalInput}
              rows={3}
              className="block min-h-[104px] max-h-80 w-full resize-none border-0 bg-transparent p-4 text-base leading-6 outline-none placeholder:text-muted-foreground disabled:opacity-50"
              placeholder={t('Write whatever is on your mind…')}
              value={goal}
              maxLength={6000}
              disabled={busy}
              onChange={(e) => update(e.target.value)}
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
            {/* Attachment previews inside the composer box above toolbar */}
            {(uploads.items.length > 0 || uploads.busy || uploads.error) && (
              <div className="px-3 pb-2.5 pt-1.5 border-t border-border/40">
                <AttachmentPreviews uploads={uploads} disabled={busy} />
              </div>
            )}
            <div className="flex items-center justify-between gap-2 p-2 sm:px-3 sm:pb-3 border-t border-border/40">
              <div className="flex items-center gap-1.5 sm:gap-2 min-w-0 flex-1">
                <AttachmentPicker
                  uploads={uploads}
                  disabled={busy}
                  disableImages={!supportsVision}
                  compact={true}
                />
                {projects.length > 0 && (
                  <StudioProjectSelector
                    projects={projects}
                    selectedProjectIds={selectedProjectIds}
                    onToggleProject={handleToggleProject}
                    onConnectProjectClick={onCreateProject}
                    disabled={busy}
                  />
                )}
              </div>
              <button
                type="submit"
                className="action-primary shrink-0"
                disabled={
                  busy || uploads.busy || installation !== 'available' || agentSetup === 'loading' || agentSetup === 'error' || (agentSetup === 'ready' && !goal.trim() && !uploads.items.length)
                }
              >
                {busy ? (
                  <LoaderCircle
                    className="h-4 w-4 animate-spin"
                    aria-hidden="true"
                  />
                ) : (
                  <ArrowRight className="h-4 w-4" aria-hidden="true" />
                )}
                {busy ? t('Getting started…') : installation === 'checking' || agentSetup === 'loading' ? t('Checking setup…') : agentSetup === 'needed' ? t('Connect Google') : t('Send')}
              </button>
            </div>
          </div>
          {!supportsVision && (
            <p className="mt-1 text-xs text-muted-foreground">
              ℹ {t('Model ini tidak mendukung input gambar (vision). Lampiran gambar dibatalkan; gunakan teks atau dokumen.')}
            </p>
          )}
          {error && (
            <div
              role="alert"
              className="rounded-lg border border-destructive/30 p-3 text-sm"
            >
              <p>{error}</p>
              {retryCount > 0 && (
                <p className="mt-1 text-xs text-muted-foreground">
                  {t('Auto-retry attempted ' + retryCount + ' time(s) with available agents.')}
                </p>
              )}
              <Link
                className="mt-2 inline-flex min-h-11 items-center underline"
                to="/settings/agents"
              >
                {t('Manage agent connections')}
              </Link>
            </div>
          )}
        </form>
      </div>
      {(agentSetup === 'needed' || addAgentModalOpen) && (
        <TeamView
          dialogOnly={agentSetup !== 'needed'}
          connectionOnly={agentSetup === 'needed'}
          createRequested={connectRequested || addAgentTrigger}
          onCreateHandled={() => {
            setConnectRequested(false)
            setAddAgentTrigger(false)
          }}
          onClose={() => {
            setAddAgentModalOpen(false)
          }}
          onAgentConnected={() => {
            setAddAgentModalOpen(false)
            onAgentConnected()
            agentsApi.list().then(setAgents).catch(() => {})
          }}
          onAgentAdded={() => {
            setAddAgentModalOpen(false)
            agentsApi.list().then(setAgents).catch(() => {})
          }}
        />
      )}
    </section>
  )
}
