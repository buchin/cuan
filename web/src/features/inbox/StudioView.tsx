import { useLanguage } from '../../context/LanguageContext'
import { AttachmentPicker, AttachmentPreviews, useAttachments } from './Attachments'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { ArrowLeft, ArrowRight, LoaderCircle, Brain, Sparkles, X, RotateCcw } from 'lucide-react'
import { apiFetch } from '../../api/client'
import { TeamView } from '../team/TeamView'
import { agentsApi, type Agent, isModelVisionCapable } from '../../api/agents'
import { projectsApi } from '../../api/projects'
import { type Project } from '../../api/types'
import { tasteApi, type GroundingTaste } from '../../api/taste'
import { StudioProjectSelector } from './StudioProjectSelector'
import { TasteGroundingChips } from './TasteGroundingChips'

export function StudioView({
  onAccepted,
  onConversationCreated,
  agentSetup,
  firstGoal,
  hasProject,
  onAgentConnected,
  onCreateProject,
  onRetrySetup,
  checkRuntime = true,
  autoFocus = false,
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
  const selectedAgentId = agents.find((a) => a.id === 'agent_cuan')?.id || ''
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

  const [pendingTasteCount, setPendingTasteCount] = useState(0)
  const [tasteBannerDismissed, setTasteBannerDismissed] = useState(false)
  const [autoApprovedTaste, setAutoApprovedTaste] = useState<{ id: string; text: string; confidence?: number } | null>(null)
  const [undoingTasteId, setUndoingTasteId] = useState<string | null>(null)
  const [activeGroundingTastes, setActiveGroundingTastes] = useState<GroundingTaste[]>([])

  useEffect(() => {
    let active = true
    tasteApi
      .getCandidates({ status: 'pending' })
      .then((res) => {
        if (active) {
          const count = res.total ?? res.candidates?.length ?? 0
          setPendingTasteCount(count)
        }
      })
      .catch(() => {})

    tasteApi
      .getCandidates({ status: 'accepted' })
      .then((res) => {
        if (active && res.candidates?.length) {
          const autoApp = res.candidates.find((c) => (c.confidence ?? 0) >= 0.8)
          if (autoApp) {
            setAutoApprovedTaste({
              id: autoApp.id,
              text: autoApp.text || autoApp.content || '',
              confidence: autoApp.confidence,
            })
          }
        }
      })
      .catch(() => {})

    tasteApi
      .getActiveGrounding()
      .then((res) => {
        if (!active) return
        const list = res.all_tastes || res.grounding_tastes || []
        if (list.length) setActiveGroundingTastes(list)
      })
      .catch(() => {})

    return () => {
      active = false
    }
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
    <section aria-label={t('Chat')} className="mx-auto w-full max-w-[75ch]">
      <div className="mb-2 sm:mb-4 pt-1">
        <Link
          to="/inbox"
          className="inline-flex min-h-11 items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors rounded-lg px-2.5 py-1.5 bg-muted/30 hover:bg-muted/60"
        >
          <ArrowLeft className="h-4 w-4" />
          <span>{t('Kembali ke Chat')}</span>
        </Link>
      </div>
      <div
        data-studio-goal-panel
        className="flex min-h-[calc(var(--visible-height,100dvh)-180px-env(safe-area-inset-bottom))] flex-col justify-center py-2 md:min-h-[calc(100dvh-150px)]"
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
        {autoApprovedTaste && (
          <aside
            aria-label={t('Preferensi baru otomatis disetujui')}
            data-testid="auto-approve-banner"
            className="mt-4 flex items-center justify-between gap-3 rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-3.5 text-xs text-emerald-900 dark:text-emerald-100"
          >
            <div className="flex items-center gap-2.5 min-w-0">
              <Sparkles className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
              <div className="flex items-center gap-2 min-w-0">
                <span className="font-semibold text-foreground">
                  {t('Preferensi baru otomatis disetujui')}:
                </span>
                <span className="truncate">
                  "{autoApprovedTaste.text}"
                </span>
                {autoApprovedTaste.confidence !== undefined && (
                  <span className="rounded-full bg-emerald-500/20 text-emerald-800 dark:text-emerald-200 border border-emerald-500/40 px-2 py-0.5 text-[10px] font-mono shrink-0">
                    {Math.round(autoApprovedTaste.confidence * 100)}% {t('Keyakinan')}
                  </span>
                )}
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <button
                type="button"
                data-testid="undo-auto-approve-studio-btn"
                onClick={async () => {
                  if (!autoApprovedTaste) return
                  try {
                    setUndoingTasteId(autoApprovedTaste.id)
                    await tasteApi.undoCandidate(autoApprovedTaste.id)
                    setAutoApprovedTaste(null)
                    setPendingTasteCount((prev) => prev + 1)
                  } catch (err) {
                    console.error('Failed to undo auto-approval:', err)
                  } finally {
                    setUndoingTasteId(null)
                  }
                }}
                disabled={undoingTasteId === autoApprovedTaste.id}
                className="action-secondary min-h-11 inline-flex items-center gap-1.5 px-3 text-xs font-semibold text-foreground hover:bg-destructive/10 hover:text-destructive transition-colors"
              >
                <RotateCcw className={`h-3.5 w-3.5 ${undoingTasteId === autoApprovedTaste.id ? 'animate-spin' : ''}`} />
                <span>{t('Batalkan')}</span>
              </button>
              <button
                type="button"
                onClick={() => setAutoApprovedTaste(null)}
                className="icon-action text-emerald-800 dark:text-emerald-200 hover:text-foreground"
                aria-label={t('Dismiss')}
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </aside>
        )}
        {pendingTasteCount > 0 && !tasteBannerDismissed && (
          <aside
            aria-label={t('Suggested Preferences')}
            data-testid="suggested-taste-banner"
            className="mt-4 flex items-center justify-between gap-3 rounded-xl border border-amber-500/30 bg-amber-500/10 p-3.5 text-xs text-amber-900 dark:text-amber-100"
          >
            <div className="flex items-center gap-2.5 min-w-0">
              <Sparkles className="h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
              <span className="truncate">
                {t('Agent suggested new preferences from your interactions.')}
              </span>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <Link
                to="/settings/memory?tab=suggested&returnTo=/chat"
                className="inline-flex min-h-11 items-center gap-1.5 rounded-lg bg-amber-600 px-3 text-xs font-semibold text-white hover:bg-amber-700 transition-colors"
              >
                <span>{t('Review Suggestions')}</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
              <button
                type="button"
                onClick={() => setTasteBannerDismissed(true)}
                className="icon-action text-amber-800 dark:text-amber-200 hover:text-foreground"
                aria-label={t('Dismiss')}
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </aside>
        )}
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
                {selectedAgent && <span className="truncate text-xs text-muted-foreground" aria-label={t('Assistant')}>{selectedAgent.name}</span>}
                {agentSetup === 'ready' && !agentsLoading && !projectsLoading && (
                  <StudioProjectSelector
                    projects={projects.filter((p) => !p.is_archived)}
                    selectedProjectId={selectedProjectId}
                    onSelect={setSelectedProjectId}
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
          <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
            <Link
              to="/settings/memory?returnTo=/chat"
              className="inline-flex min-h-11 items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground underline underline-offset-2 transition-colors"
              title={t('Agent Memory / Taste')}
            >
              <Brain className="h-3.5 w-3.5 text-primary" />
              <span>{t('Agent Memory / Taste')}</span>
              {pendingTasteCount > 0 && (
                <span
                  data-testid="suggested-taste-badge"
                  className="ml-1 inline-flex items-center gap-1 rounded-full bg-amber-500/20 border border-amber-500/30 px-2 py-0.5 text-[10px] font-semibold text-amber-800 dark:text-amber-200 font-mono"
                >
                  <Sparkles className="h-2.5 w-2.5" />
                  {pendingTasteCount} {t('suggested')}
                </span>
              )}
            </Link>
            {activeGroundingTastes.length > 0 && (
              <div data-testid="studio-active-grounding" className="flex items-center gap-1.5">
                <TasteGroundingChips tastes={activeGroundingTastes} returnTo="/chat" />
              </div>
            )}
          </div>
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
