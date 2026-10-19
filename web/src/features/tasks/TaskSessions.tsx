import { useEffect, useState } from 'react'
import type { TaskAttempt } from '../../api/types'
import type { Agent } from '../../api/agents'
import type { LiveTaskSession } from '../../api/tasks'
import { TaskTerminal } from './TaskTerminal'
import { attemptLabel, relativeTime } from './task-ui'
export function TaskSessions({
  taskId,
  attempts,
  agents,
  live,
  preferred,
  onRefresh,
  onCancel,
  busy,
}: {
  taskId: string
  attempts: TaskAttempt[]
  agents: Agent[]
  live: LiveTaskSession | null
  preferred: string
  onRefresh: () => void
  onCancel: () => void
  busy: boolean
}) {
  const [selectedId, setSelectedId] = useState(preferred)
  useEffect(() => {
    if (preferred) setSelectedId(preferred)
  }, [preferred])
  const sorted = [...attempts].sort(
    (a, b) => b.attempt_number - a.attempt_number,
  )
  const selected = sorted.find((a) => a.id === selectedId) || sorted[0]
  if (!selected)
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-6 text-center">
        <h3 className="font-medium">No sessions yet</h3>
        <p className="max-w-md text-sm text-muted-foreground">
          Start this task with an agent when the instructions are ready. Opening
          this tab does not start a process.
        </p>
      </div>
    )
  const agent =
    agents.find((a) => a.id === selected.agent_name)?.name ||
    selected.agent_name
  const active = ['queued', 'running'].includes(selected.status)
  return (
    <div className="flex h-full min-h-0 min-w-0 flex-col gap-3">
      <div className="flex shrink-0 flex-wrap items-center gap-2">
        <label className="min-w-0 flex-1">
          <span className="sr-only">Session</span>
          <select
            aria-label="Select session"
            className="field-control"
            value={selected.id}
            onChange={(e) => setSelectedId(e.target.value)}
          >
            {sorted.map((a) => (
              <option key={a.id} value={a.id}>
                Session {a.attempt_number} · {attemptLabel(a)} ·{' '}
                {relativeTime(a.started_at || a.created_at)}
              </option>
            ))}
          </select>
        </label>
        {active && (
          <button
            className="action-secondary text-destructive"
            disabled={busy}
            onClick={onCancel}
          >
            Cancel session
          </button>
        )}
      </div>
      <div className="shrink-0 text-xs text-muted-foreground">
        <p>
          {agent} ·{' '}
          <time
            title={new Date(
              selected.started_at || selected.created_at,
            ).toLocaleString('en-US')}
          >
            {relativeTime(selected.started_at || selected.created_at)}
          </time>
          {selected.exit_code !== undefined && ` · Exit ${selected.exit_code}`}
          {selected.finished_at &&
            ` · Finished ${new Date(selected.finished_at).toLocaleString('en-US')}`}
        </p>
        {selected.error_message && (
          <p className="mt-1 break-words text-destructive">
            {selected.error_message}
          </p>
        )}
      </div>
      <div className="min-h-0 flex-1">
        <TaskTerminal
          key={selected.id}
          taskId={taskId}
          attempt={selected}
          live={live}
          onRefresh={onRefresh}
        />
      </div>
    </div>
  )
}
