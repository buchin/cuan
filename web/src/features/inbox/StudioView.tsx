import { useLanguage } from '../../context/LanguageContext'
import { AttachmentPicker, AttachmentPreviews, useAttachments } from './Attachments'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { ArrowRight, LoaderCircle } from 'lucide-react'
import { apiFetch } from '../../api/client'
import { TeamView } from '../team/TeamView'
import { agentsApi, type Agent, isModelVisionCapable } from '../../api/agents'
import { projectsApi } from '../../api/projects'
import { type Project } from '../../api/types'
import { StudioExecutionSelector } from './StudioExecutionSelector'

export function StudioView({
  onAccepted, agentSetup, firstGoal, hasProject,
  onAgentConnected, onCreateProject, onRetrySetup, checkRuntime = true,
}: {
  onAccepted: () => void
  agentSetup: 'loading' | 'needed' | 'ready' | 'error'
  firstGoal: boolean
  hasProject: boolean
  onAgentConnected: () => void
  onCreateProject: () => void
  checkRuntime?: boolean
  onRetrySetup: () => void
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
  const [agentsLoading, setAgentsLoading] = useState(true)
  const [selectedAgentId, setSelectedAgentId] = useState('')
  const [projects, setProjects] = useState<Project[]>([])
  const [projectsLoading, setProjectsLoading] = useState(true)
  const [selectedProjectId, setSelectedProjectId] = useState(
    () => searchParams.get('project') || ''
  )

  useEffect(() => {
    const projectParam = searchParams.get('project')
    if (projectParam) {
      setSelectedProjectId(projectParam)
      setTimeout(() => {
        goalInput.current?.focus()
      }, 60)
    }
  }, [searchParams])

  const selectedAgent = agents.find((a) => a.id === selectedAgentId)
  const supportsVision = selectedAgent
    ? isModelVisionCapable(selectedAgent.model_id, selectedAgent.provider || selectedAgent.harness)
    : true

  useEffect(() => {
    if (!selectedAgent) return
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
    setAgentsLoading(true)
    agentsApi.list()
      .then((list) => { if (active) setAgents(list) })
      .catch(() => { if (active) setAgents([]) })
      .finally(() => { if (active) setAgentsLoading(false) })
    return () => { active = false }
  }, [])

  useEffect(() => {
    let active = true
    setProjectsLoading(true)
    projectsApi.list()
      .then((list) => { if (active) setProjects(list) })
      .catch(() => { if (active) setProjects([]) })
      .finally(() => { if (active) setProjectsLoading(false) })
    return () => { active = false }
  }, [hasProject])

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
        saved?.agent_id === selectedAgentId &&
        saved?.project_id === selectedProjectId &&
        JSON.stringify(saved.attachment_ids || []) ===
          JSON.stringify(uploads.items.map((a) => a.id))
          ? saved
          : {
              goal: goal.trim(),
              request_id: crypto.randomUUID(),
              attachment_ids: uploads.items.map((a) => a.id),
              ...(selectedAgentId ? { agent_id: selectedAgentId } : {}),
              ...(selectedProjectId ? { project_id: selectedProjectId } : {}),
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
      navigate('/inbox/' + encodeURIComponent(result.conversation_id))
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      if (msg.includes('provider_quota_exhausted') || msg.includes('quota')) {
        setError(
          t('All available agents reached usage limits. Your message is saved. Try again after the limit resets.'),
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
    <section aria-label={t('Chat')} className="mx-auto w-full max-w-2xl">
      <div
        data-studio-goal-panel
        className="flex min-h-[calc(var(--visible-height,100dvh)-136px-env(safe-area-inset-bottom))] flex-col justify-center py-4 md:min-h-[calc(100dvh-112px)]"
      >
        <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          {t('What would you like to get done today?')}
        </h2>
        <p className="mt-2 max-w-lg text-sm leading-relaxed text-muted-foreground">
          {t('Tell Cuan your goal. We’ll take care of the rest.')}
        </p>
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
          <div className="relative rounded-2xl border border-input bg-background focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2">
            <textarea
              data-primary-composer
              id="studio-goal"
              ref={goalInput}
              rows={3}
              className="block min-h-[104px] max-h-80 w-full resize-none border-0 bg-transparent p-4 text-base leading-6 outline-none placeholder:text-muted-foreground disabled:opacity-50"
              placeholder={t('What would you like done?')}
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
                {agentSetup === 'ready' && !agentsLoading && !projectsLoading && (
                  <StudioExecutionSelector
                    projects={projects.filter((p) => !p.is_archived)}
                    selectedProjectId={selectedProjectId}
                    onSelectProject={setSelectedProjectId}
                    onConnectProjectClick={onCreateProject}
                    agents={agents}
                    selectedAgentId={selectedAgentId}
                    onSelectAgent={setSelectedAgentId}
                    onAddAgentClick={() => {
                      setAddAgentModalOpen(true)
                      setAddAgentTrigger(true)
                    }}
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
                {busy ? t('Getting started…') : installation === 'checking' || agentSetup === 'loading' ? t('Checking setup…') : agentSetup === 'needed' ? t('Connect Google') : t('Get it done')}
              </button>
            </div>
          </div>
          {!supportsVision && (
            <p className="mt-1 text-xs text-muted-foreground">
              ℹ {t('Model ini tidak mendukung input gambar (vision). Lampiran gambar dibatalkan; gunakan teks atau dokumen.')}
            </p>
          )}
          {firstGoal && installation === 'available' && agentSetup === 'ready' && (
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t('Start with your goal. Cuan will organize the work for you.')}
              {!hasProject && <> <button type="button" className="min-h-11 underline underline-offset-2" onClick={onCreateProject}>{t('Create or connect a project (optional)')}</button></>}
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
