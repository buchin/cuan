import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowRight, LoaderCircle } from 'lucide-react'
import { apiFetch } from '../../api/client'

export function StudioView({ onAccepted }: { onAccepted: () => void }) {
  const navigate = useNavigate()
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
    if (busy || !goal.trim()) return
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
        saved?.goal === goal.trim()
          ? saved
          : { goal: goal.trim(), request_id: crypto.randomUUID() }
      try {
        localStorage.setItem('cuan-studio-pending', JSON.stringify(request))
      } catch {
        throw new Error(
          'Enable browser storage so Cuan can safely retry this request. Your goal is kept on this page.',
        )
      }
      const result = await apiFetch<{ conversation_id: string }>(
        '/api/v1/studio/goals',
        { method: 'POST', body: JSON.stringify(request) },
      )
      localStorage.removeItem('cuan-studio-pending')
      localStorage.removeItem('cuan-studio-draft')
      setGoal('')
      onAccepted()
      navigate('/inbox/' + encodeURIComponent(result.conversation_id))
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : 'Could not send your goal. Try again; your request is kept.',
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <section
      aria-label="Studio"
      className="mx-auto flex min-h-full w-full max-w-2xl flex-col justify-start py-6 md:justify-center md:py-12"
    >
      <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
        What would you like to get done today?
      </h2>
      <p className="mt-3 max-w-lg text-sm leading-relaxed text-muted-foreground">
        Tell Cuan the outcome. We’ll organize the work and keep you updated.
      </p>
      <form
        className="mt-6 space-y-3"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <label htmlFor="studio-goal" className="sr-only">
          Your goal
        </label>
        <div className="overflow-hidden rounded-2xl border border-input bg-background focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2">
          <textarea
            id="studio-goal"
            className="block min-h-44 w-full resize-none border-0 bg-transparent p-4 text-base leading-relaxed outline-none placeholder:text-muted-foreground disabled:opacity-50"
            placeholder="Describe what you want to achieve…"
            value={goal}
            maxLength={6000}
            disabled={busy}
            onChange={(e) => update(e.target.value)}
          />
        </div>
        <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="order-2 w-full text-center text-xs text-muted-foreground sm:order-none sm:w-auto sm:text-left">
            We’ll ask if anything needs clarification.
          </p>
          <button
            type="submit"
            className="action-primary w-full sm:w-auto"
            disabled={busy || !goal.trim()}
          >
            {busy ? (
              <LoaderCircle
                className="h-4 w-4 animate-spin"
                aria-hidden="true"
              />
            ) : (
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            )}
            {busy ? 'Getting started…' : 'Get it done'}
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
              Manage agent connections
            </Link>
          </div>
        )}
      </form>
    </section>
  )
}
