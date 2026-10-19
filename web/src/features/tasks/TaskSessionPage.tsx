import { useCallback, useEffect, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { ArrowLeft, Info, Square, Send } from 'lucide-react'
import { tasksApi, type LiveTaskSession } from '../../api/tasks'
import type { TaskDetail } from '../../api/types'
import { TaskTerminal } from './TaskTerminal'
import { ChatMarkdown } from '../inbox/ChatMarkdown'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { attemptLabel, taskRef } from './task-ui'

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
    [showDetail, setShowDetail] = useState(false),
    [followup, setFollowup] = useState(''),
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
      className="flex h-full min-h-0 flex-col bg-background"
    >
      <header className="flex shrink-0 items-center gap-2 border-b px-2 py-1.5">
        <Link
          to={taskURL}
          state={state}
          className="icon-action shrink-0"
          aria-label="Task details"
          title="Task details"
        >
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-sm font-semibold" title={detail.title}>
            {detail.title}
          </h2>
          <p className="text-xs text-muted-foreground">
            {attemptLabel(attempt)} · Session {attempt.attempt_number}
          </p>
        </div>
        <button
          className="icon-action shrink-0"
          aria-label="Detail"
          title="Detail"
          onClick={() => setShowDetail(true)}
        >
          <Info className="h-4 w-4" />
        </button>
        {active && (
          <button
            className="icon-action shrink-0 text-destructive"
            aria-label="Cancel session"
            title="Cancel session"
            onClick={() => setCancel(true)}
          >
            <Square className="h-4 w-4" />
          </button>
        )}
      </header>
      {(error || runtimeError) && (
        <p role="alert" className="shrink-0 px-3 text-xs text-destructive">
          {error || runtimeError}
        </p>
      )}
      {attempt.error_message && (
        <button
          className="shrink-0 truncate px-3 py-1 text-left text-xs text-destructive"
          onClick={() => setShowDetail(true)}
          title={attempt.error_message}
        >
          Session needs attention · View detail
        </button>
      )}
      <div className="min-h-0 flex-1 px-2 pb-2">
        <TaskTerminal
          key={sessionId}
          taskId={taskId}
          attempt={attempt}
          live={live}
          onRefresh={() => void refresh()}
        />
      </div>
      {!active &&
        detail.latest_attempt?.id === attempt.id &&
        detail.status !== 'done' &&
        detail.assignee_id && (
          <form
            className="flex shrink-0 items-end gap-2 border-t bg-background p-2"
            onSubmit={(e) => {
              e.preventDefault()
              if (!followup.trim() || busy) return
              const request = {
                request_id: crypto.randomUUID(),
                agent_id: detail.assignee_id!,
                revision: followup.trim(),
              }
              try {
                localStorage.setItem(
                  'cuan-task-launch:' + taskId,
                  JSON.stringify(request),
                )
                navigate(
                  '/tasks/' +
                    encodeURIComponent(taskId) +
                    '/sessions/start' +
                    suffix,
                  {
                    state: {
                      taskReturnTo: state?.taskReturnTo,
                      startRequestId: request.request_id,
                    },
                  },
                )
              } catch {
                setError(
                  'Could not save the session request. Enable browser storage and try again.',
                )
              }
            }}
          >
            <label className="min-w-0 flex-1">
              <span className="sr-only">Follow-up instructions</span>
              <textarea
                aria-label="Follow-up instructions"
                rows={1}
                maxLength={24000}
                className="field-control max-h-32 resize-y text-base md:text-sm"
                placeholder="What should the agent do next?"
                value={followup}
                onChange={(e) => setFollowup(e.target.value)}
              />
            </label>
            <button
              type="submit"
              className="icon-action bg-primary text-primary-foreground"
              disabled={!followup.trim() || busy}
              aria-label="Start follow-up session"
              title="Start follow-up session"
            >
              <Send className="h-4 w-4" />
            </button>
            <span className="sr-only">
              Starts a new session with this task’s context and retained
              history.
            </span>
          </form>
        )}
      <ActionDialog
        open={showDetail}
        title="Detail"
        drawer
        onClose={() => setShowDetail(false)}
      >
        <div className="space-y-4">
          <h3 className="break-words text-sm font-semibold">{detail.title}</h3>
          <Link className="text-sm underline" to={taskURL} state={state}>
            Open task details · {taskRef(taskId)}
          </Link>
          <label className="block text-xs text-muted-foreground">
            Session
            <select
              aria-label="Select session"
              className="field-control mt-1 text-sm"
              value={sessionId}
              onChange={(e) => {
                setShowDetail(false)
                navigate(
                  '/tasks/' +
                    encodeURIComponent(taskId) +
                    '/sessions/' +
                    encodeURIComponent(e.target.value) +
                    suffix,
                  { state },
                )
              }}
            >
              {detail.attempts.map((a) => (
                <option key={a.id} value={a.id}>
                  Session {a.attempt_number} · {attemptLabel(a)}
                </option>
              ))}
            </select>
          </label>
          <p className="text-xs text-muted-foreground">
            {attemptLabel(attempt)}
            {attempt.exit_code !== undefined &&
              ' · Exit ' + attempt.exit_code}{' '}
            ·{' '}
            {new Date(attempt.started_at || attempt.created_at).toLocaleString(
              'en-US',
            )}
          </p>
          {attempt.error_message && (
            <p className="break-words text-sm text-destructive">
              {attempt.error_message}
            </p>
          )}
          <aside aria-label="Agent report">
            <h3 className="mb-3 text-sm font-semibold">Agent report</h3>
            {!reports.length ? (
              <p className="text-sm text-muted-foreground">
                {active
                  ? 'Waiting for the agent’s first checkpoint.'
                  : 'No structured report was recorded. Check the saved output.'}
              </p>
            ) : (
              <ol className="space-y-3">
                {reports.map((r) => (
                  <li key={r.id} className="rounded-lg border p-3">
                    <p className="mb-2 text-xs text-muted-foreground">
                      {r.new_status === 'review'
                        ? 'Result ready'
                        : r.new_status === 'blocked'
                          ? 'Blocker'
                          : 'Progress'}{' '}
                      · {new Date(r.created_at).toLocaleTimeString('en-US')}
                    </p>
                    <ChatMarkdown content={r.message} />
                  </li>
                ))}
              </ol>
            )}
          </aside>
        </div>
      </ActionDialog>
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
