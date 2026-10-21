import { useLanguage } from '../../context/LanguageContext'
import { AttachmentPicker, useAttachments } from './Attachments'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowRight, LoaderCircle } from 'lucide-react'
import { apiFetch } from '../../api/client'
import { TeamView } from '../team/TeamView'

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
    [error, setError] = useState('')
  const update = (text: string) => {
    setGoal(text)
    try {
      localStorage.setItem('cuan-studio-draft', text)
    } catch {
      /* Submission surfaces unavailable storage. */
    }
  }
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
        JSON.stringify(saved.attachment_ids || []) ===
          JSON.stringify(uploads.items.map((a) => a.id))
          ? saved
          : {
              goal: goal.trim(),
              request_id: crypto.randomUUID(),
              attachment_ids: uploads.items.map((a) => a.id),
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
      const result = await apiFetch<{ conversation_id: string }>(
        '/api/v1/studio/goals',
        { method: 'POST', body: JSON.stringify(request) },
      )
      localStorage.removeItem('cuan-studio-pending')
      localStorage.removeItem('cuan-studio-draft')
      setGoal('')
      uploads.clear()
      onAccepted()
      navigate('/inbox/' + encodeURIComponent(result.conversation_id))
    } catch (e) {
      setError(
        e instanceof Error
          ? t(e.message)
          : t('Could not send your goal. Try again; your request is kept.'),
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <section aria-label={t('Studio')} className="mx-auto w-full max-w-2xl">
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
          <div className="overflow-hidden rounded-2xl border border-input bg-background focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2">
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
            <div className="px-2 pb-2">
              <AttachmentPicker uploads={uploads} disabled={busy} />
            </div>
          </div>
          <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:items-center sm:justify-end">
            <button
              type="submit"
              className="action-primary w-full sm:w-auto"
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
          {firstGoal && installation === 'available' && agentSetup === 'ready' && (
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t('Start with your goal. Cuan will organize the work for you.')}
              {!hasProject && <> <button type="button" className="min-h-11 underline underline-offset-2" onClick={onCreateProject}>{t('Connect a project (optional)')}</button></>}
            </p>
          )}
          {error && (
            <div
              role="alert"
              className="rounded-lg border border-destructive/30 p-3 text-sm"
            >
              <p>{error}</p>
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
      {agentSetup === 'needed' && <TeamView
        connectionOnly
        createRequested={connectRequested}
        onCreateHandled={() => setConnectRequested(false)}
        onAgentConnected={onAgentConnected}
      />}
    </section>
  )
}
