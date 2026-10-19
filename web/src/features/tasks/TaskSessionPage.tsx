import { useCallback, useEffect, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { ArrowLeft, TerminalSquare, FileText } from 'lucide-react'
import { tasksApi, type LiveTaskSession } from '../../api/tasks'
import type { TaskDetail } from '../../api/types'
import { TaskTerminal } from './TaskTerminal'
import { ChatMarkdown } from '../inbox/ChatMarkdown'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { attemptLabel, taskRef } from './task-ui'
import { cn } from '../../lib/utils'

export function TaskSessionPage({
  taskId,
  sessionId,
}: {
  taskId: string
  sessionId: string
}) {
  const location = useLocation(),
    navigate = useNavigate()
  const [detail, setDetail] = useState<TaskDetail | null>(null),
    [live, setLive] = useState<LiveTaskSession | null>(null),
    [error, setError] = useState(''),
    [runtimeError, setRuntimeError] = useState(''),
    [tab, setTab] = useState<'terminal' | 'report'>('terminal'),
    [cancel, setCancel] = useState(false),
    [busy, setBusy] = useState(false)
  const refresh = useCallback(async () => {
    try {
      setDetail(await tasksApi.get(taskId))
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load this session.')
    }
    try {
      const runtime = await tasksApi.runtime(taskId)
      setLive(runtime.live_session || null)
      setRuntimeError('')
    } catch {
      setLive(null)
      setRuntimeError(
        'Live session status is unavailable. Saved output remains available.',
      )
    }
  }, [taskId])
  useEffect(() => {
    void refresh()
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') void refresh()
    }, 4000)
    return () => clearInterval(timer)
  }, [refresh])
  const attempt = detail?.attempts.find((a) => a.id === sessionId)
  const suffix = location.search,
    state = location.state
  const taskURL = `/tasks/${encodeURIComponent(taskId)}${suffix}`
  const reports =
    detail?.activities.filter(
      (a) => a.attempt_id === sessionId && a.activity_type === 'agent_report',
    ) || []
  if (!detail)
    return (
      <div role={error ? 'alert' : 'status'} className="p-4 text-sm">
        {error || 'Loading session…'}
        {error && (
          <button
            className="action-secondary ml-3"
            onClick={() => void refresh()}
          >
            Try again
          </button>
        )}
        <Link className="ml-3 underline" to={taskURL} state={state}>
          Task details
        </Link>
      </div>
    )
  if (!attempt)
    return (
      <div className="p-4 text-sm">
        <p role="alert">Session not found for this task.</p>
        <Link
          className="mt-3 inline-block underline"
          to={taskURL}
          state={state}
        >
          Task details
        </Link>
      </div>
    )
  const active = ['running', 'queued'].includes(attempt.status)
  return (
    <section
      aria-label="Task session"
      className="flex h-full min-h-[680px] flex-col gap-3"
    >
      <header className="shrink-0 space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Link
            to={taskURL}
            state={state}
            className="inline-flex min-h-11 items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" />
            {taskRef(taskId)} · Task details
          </Link>
          <div className="flex min-w-0 items-center gap-2">
            <label>
              <span className="sr-only">Session</span>
              <select
                aria-label="Select session"
                className="field-control max-w-[220px] text-sm"
                value={sessionId}
                onChange={(e) =>
                  navigate(
                    `/tasks/${encodeURIComponent(taskId)}/sessions/${encodeURIComponent(e.target.value)}${suffix}`,
                    { state },
                  )
                }
              >
                {detail.attempts.map((a) => (
                  <option key={a.id} value={a.id}>
                    Session {a.attempt_number} · {attemptLabel(a)}
                  </option>
                ))}
              </select>
            </label>
            {active && (
              <button
                className="action-secondary text-destructive"
                onClick={() => setCancel(true)}
              >
                Cancel session
              </button>
            )}
          </div>
        </div>
        <h2 className="break-words text-base font-semibold sm:text-lg">
          {detail.title}
        </h2>
        <p className="text-xs text-muted-foreground">
          {attemptLabel(attempt)}
          {attempt.exit_code !== undefined &&
            ` · Exit ${attempt.exit_code}`} ·{' '}
          {new Date(attempt.started_at || attempt.created_at).toLocaleString(
            'en-US',
          )}
        </p>
        {attempt.error_message && (
          <p className="break-words text-sm text-destructive">
            {attempt.error_message}
          </p>
        )}
      </header>
      {(error || runtimeError) && (
        <p role="alert" className="shrink-0 text-sm text-destructive">
          {error || runtimeError}
        </p>
      )}
      <nav
        aria-label="Session panels"
        className="flex shrink-0 gap-1 border-b lg:hidden"
      >
        {(
          [
            { id: 'terminal', label: 'Terminal', icon: TerminalSquare },
            { id: 'report', label: 'Agent report', icon: FileText },
          ] as const
        ).map((item) => (
          <button
            key={item.id}
            className={cn(
              'inline-flex min-h-11 items-center gap-2 border-b-2 px-3 text-sm',
              tab === item.id
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground',
            )}
            aria-pressed={tab === item.id}
            onClick={() => setTab(item.id)}
          >
            <item.icon className="h-4 w-4" />
            {item.label}
          </button>
        ))}
      </nav>
      <div className="flex min-h-0 flex-1 gap-4">
        <div
          className={cn(
            'min-h-0 min-w-0 flex-1',
            tab === 'terminal' ? 'block' : 'hidden lg:block',
          )}
        >
          <TaskTerminal
            key={sessionId}
            taskId={taskId}
            attempt={attempt}
            live={live}
            onRefresh={() => void refresh()}
          />
        </div>
        <aside
          aria-label="Agent report"
          className={cn(
            'min-h-0 min-w-0 overflow-y-auto lg:w-80 lg:shrink-0 lg:border-l lg:pl-4',
            tab === 'report' ? 'block w-full' : 'hidden lg:block',
          )}
        >
          <h3 className="mb-3 text-sm font-semibold">Agent report</h3>
          {reports.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {active
                ? 'Waiting for the agent’s first checkpoint.'
                : 'No structured report was recorded. Check the saved output.'}
            </p>
          ) : (
            <ol className="space-y-4">
              {reports.map((report) => (
                <li key={report.id} className="min-w-0 rounded-lg border p-3">
                  <p className="mb-2 text-xs text-muted-foreground">
                    {report.new_status === 'review'
                      ? 'Result ready'
                      : report.new_status === 'blocked'
                        ? 'Blocker'
                        : 'Progress'}{' '}
                    · {new Date(report.created_at).toLocaleTimeString('en-US')}
                  </p>
                  <ChatMarkdown content={report.message} />
                </li>
              ))}
            </ol>
          )}
        </aside>
      </div>
      <ActionDialog
        open={cancel}
        title="Cancel session?"
        busy={busy}
        onClose={() => setCancel(false)}
      >
        <p className="mb-4 text-sm">
          Stop the active process? Saved output and existing file changes are
          retained.
        </p>
        <div className="flex justify-end gap-2">
          <button
            className="action-secondary"
            disabled={busy}
            onClick={() => setCancel(false)}
          >
            Keep running
          </button>
          <button
            className="action-primary"
            disabled={busy}
            onClick={async () => {
              setBusy(true)
              try {
                await tasksApi.cancel(taskId)
                setCancel(false)
                await refresh()
              } catch (e) {
                setError(
                  e instanceof Error
                    ? e.message
                    : 'Could not cancel the session.',
                )
              } finally {
                setBusy(false)
              }
            }}
          >
            Cancel session
          </button>
        </div>
      </ActionDialog>
    </section>
  )
}
