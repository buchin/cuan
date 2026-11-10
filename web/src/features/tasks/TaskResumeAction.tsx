import '../inbox/card-actions.css'
import { useEffect, useState } from 'react'
import { Loader2, Play } from 'lucide-react'
import type { Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { useLanguage } from '../../context/LanguageContext'
import { workDisplayStatus } from './task-ui'

export function TaskResumeAction({ task, onResumed, className = '' }: { task: Task; onResumed?: () => void | Promise<void>; className?: string }) {
  const { t } = useLanguage()
  const [plan, setPlan] = useState<{ available: boolean; attempt_id?: string }>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const paused = workDisplayStatus(task) === 'Work paused' && !!task.source_room_id
  const attempt = task.latest_attempt?.id
  useEffect(() => {
    setPlan(undefined); setError('')
    if (!paused || !attempt) return
    let active = true
    void tasksApi.resumePlan(task.id).then(value => { if (active) setPlan(value) }).catch(() => { if (active) setError(t('Could not check whether this work can continue')) })
    return () => { active = false }
  }, [task.id, task.updated_at, paused, attempt, t])
  if (!paused || !attempt || ((!plan?.available || plan.attempt_id !== attempt) && !error)) return null
  return <div data-card-actions className="inline-flex min-w-0 flex-wrap items-center gap-x-2">
    <button type="button" data-primary-action className={'inline-flex min-h-11 items-center justify-center gap-1.5 px-3 text-xs font-medium ' + className}
      disabled={busy} aria-label={t('Resume work')} onClick={async event => {
        event.stopPropagation()
        if (busy) return
        setBusy(true); setError('')
        try {
          const current = plan || await tasksApi.resumePlan(task.id)
          if (!current.available || current.attempt_id !== attempt) throw new Error(t('The work has changed. Check its latest status.'))
          await tasksApi.resume(task.id, attempt)
          window.dispatchEvent(new Event('cuan:tasks-changed'))
          await onResumed?.()
          setPlan(await tasksApi.resumePlan(task.id))
        } catch (err) { setError(err instanceof Error ? err.message : t('Could not continue this work')) }
        finally { setBusy(false) }
      }}>
      {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" /> : <Play className="h-3.5 w-3.5" />}
      {t(busy ? 'Resuming…' : 'Resume')}
    </button>
    {error && <p role="alert" className="basis-full max-w-xs text-xs leading-relaxed text-destructive">{error}</p>}
  </div>
}
