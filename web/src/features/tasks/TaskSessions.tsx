import type { TaskAttempt } from '../../api/types'
import type { Agent } from '../../api/agents'
import type { LiveTaskSession } from '../../api/tasks'
import { useLanguage } from '../../context/LanguageContext'
import { attemptLabel, relativeTime } from './task-ui'

function getSessionDuration(a: TaskAttempt, idLang: boolean): string | null {
  if (!a.started_at) return null
  const start = Date.parse(a.started_at)
  const end = a.finished_at ? Date.parse(a.finished_at) : (a.status === 'running' ? Date.now() : NaN)
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return null
  const mins = Math.floor((end - start) / 60000)
  if (mins < 1) return idLang ? '<1 menit' : '<1 min'
  if (mins < 60) return `${mins} ${idLang ? 'menit' : 'min'}`
  const hrs = Math.floor(mins / 60)
  const rem = mins % 60
  return `${hrs} ${idLang ? 'jam' : 'hr'}${rem ? ` ${rem} ${idLang ? 'menit' : 'min'}` : ''}`
}

export function TaskSessions({
  attempts,
  agents,
  live,
  onOpen,
  onCancel,
  busy,
}: {
  attempts: TaskAttempt[]
  agents: Agent[]
  live: LiveTaskSession | null
  onOpen: (id: string) => void
  onCancel: () => void
  busy: boolean
}) {
  const { language } = useLanguage()
  if (!attempts.length)
    return (
      <div className="rounded-lg border border-dashed p-6 text-center">
        <h3 className="text-sm font-medium">No sessions yet</h3>
        <p className="mt-2 text-sm text-muted-foreground">
          Start this task with an agent when you’re ready.
        </p>
      </div>
    )
  return (
    <ol className="space-y-2">
      {[...attempts]
        .sort((a, b) => b.attempt_number - a.attempt_number)
        .map((a) => (
          <li key={a.id} className="rounded-lg border p-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <button
                className="min-h-11 text-left text-sm font-medium text-primary hover:underline"
                onClick={() => onOpen(a.id)}
              >
                Session {a.attempt_number} · {attemptLabel(a)}
              </button>
              {live?.attempt_id === a.id && (
                <span className="text-xs text-muted-foreground">Live</span>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {agents.find((agent) => agent.id === 'agent_cuan')?.name || '@cuan'}{' '}
              · {relativeTime(a.started_at || a.created_at)}
              {getSessionDuration(a, language === 'id') && ` · ${getSessionDuration(a, language === 'id')}`}
              {a.status === 'failed' && ' · Failed'}
            </p>
            {a.error_message && (
              <p className="mt-2 break-words text-xs text-destructive">
                {a.error_message}
              </p>
            )}
            <div className="mt-2 flex flex-wrap gap-2">
              <button className="action-secondary" onClick={() => onOpen(a.id)}>
                Open session
              </button>
              {['running', 'queued'].includes(a.status) && (
                <button
                  className="action-secondary text-destructive"
                  disabled={busy}
                  onClick={onCancel}
                >
                  Cancel session
                </button>
              )}
            </div>
          </li>
        ))}
    </ol>
  )
}
