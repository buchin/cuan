import { InstallNudge } from '../settings/InstallNudge'
import { useLanguage } from '../../context/LanguageContext'
import { AttachmentPicker, useAttachments } from './Attachments'
import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowRight, LoaderCircle } from 'lucide-react'
import { apiFetch } from '../../api/client'

export function StudioView({ onAccepted }: { onAccepted: () => void }) {
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
    if (busy || uploads.busy || (!goal.trim() && !uploads.items.length)) return
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
    <section
      aria-label={t('Studio')}
      className="mx-auto flex min-h-full w-full max-w-2xl flex-col justify-start py-6 md:justify-center md:py-12"
    >
      <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
        {t('What would you like to get done today?')}
      </h2>
      <p className="mt-3 max-w-lg text-sm leading-relaxed text-muted-foreground">
        {t(
          'Tell Cuan the outcome. We’ll organize the work and keep you updated.',
        )}
      </p>
      <form
        className="mt-6 space-y-3"
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
            className="block min-h-44 w-full resize-none border-0 bg-transparent p-4 text-base leading-relaxed outline-none placeholder:text-muted-foreground disabled:opacity-50"
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
        <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="order-2 w-full text-center text-xs text-muted-foreground sm:order-none sm:w-auto sm:text-left">
            {t('We’ll ask if anything needs clarification.')}
          </p>
          <button
            type="submit"
            className="action-primary w-full sm:w-auto"
            disabled={
              busy || uploads.busy || (!goal.trim() && !uploads.items.length)
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
            {busy ? t('Getting started…') : t('Get it done')}
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
      <InstallNudge />
    </section>
  )
}
