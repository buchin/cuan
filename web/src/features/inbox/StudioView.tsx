import { useLanguage } from '../../context/LanguageContext'
import { AttachmentPicker, useAttachments } from './Attachments'
import { useLayoutEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowRight, LoaderCircle } from 'lucide-react'
import { apiFetch } from '../../api/client'

export function StudioView({
  onAccepted, agentSetup, firstGoal, hasProject,
  onConnectAgent, onCreateProject, onRetrySetup,
}: {
  onAccepted: () => void
  agentSetup: 'loading' | 'needed' | 'ready' | 'error'
  firstGoal: boolean
  hasProject: boolean
  onConnectAgent: () => void
  onCreateProject: () => void
  onRetrySetup: () => void
}) {
  const { t } = useLanguage()

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
    if (busy || uploads.busy) return
    if (agentSetup === 'needed') {
      onConnectAgent()
      return
    }
    if (agentSetup === 'error') {
      onRetrySetup()
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
        {agentSetup === 'needed' && (
          <aside aria-label={t('Get started')} className="mt-4 rounded-xl border bg-card p-4 text-sm">
            <h3 className="font-medium">{t('Connect your AI to get started')}</h3>
            <p className="mt-1 leading-relaxed text-muted-foreground">{t('Connect an AI account so Cuan can work on your goals. Your draft will stay here.')}</p>
            <button type="button" className="action-secondary mt-3" onClick={onConnectAgent}>{t('Connect your AI')}</button>
          </aside>
        )}
        {agentSetup === 'error' && (
          <div role="alert" className="mt-4 rounded-xl border p-4 text-sm">
            <p>{t('Could not check your AI connections.')}</p>
            <button type="button" className="action-secondary mt-3" onClick={onRetrySetup}>{t('Try again')}</button>
          </div>
        )}
        {agentSetup === 'ready' && firstGoal && !hasProject && (
          <aside aria-label={t('Your first project')} className="mt-4 rounded-xl border bg-card p-4 text-sm">
            <h3 className="font-medium">{t('Start with your first goal')}</h3>
            <p className="mt-1 leading-relaxed text-muted-foreground">{t('Cuan can organize the project for you. Already have one? Connect it below, or start with your goal.')}</p>
            <button type="button" className="action-secondary mt-3" onClick={onCreateProject}>{t('Connect a project (optional)')}</button>
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
          <div className="overflow-hidden rounded-2xl border border-input bg-background focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2">
            <textarea
              id="studio-goal"
              ref={goalInput}
              className="block min-h-36 max-h-80 md:min-h-44 w-full resize-none border-0 bg-transparent p-4 text-base leading-relaxed outline-none placeholder:text-muted-foreground disabled:opacity-50"
              placeholder={t('Describe what you want to achieve…')}
              value={goal}
              maxLength={6000}
              disabled={busy}
              onChange={(e) => update(e.target.value)}
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
                busy || uploads.busy || agentSetup !== 'ready' || (!goal.trim() && !uploads.items.length)
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
              {busy ? t('Getting started…') : agentSetup === 'loading' ? t('Checking connection…') : t('Get it done')}
            </button>
          </div>
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
    </section>
  )
}
