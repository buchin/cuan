import { workspacePath } from './workspacePath'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { ArrowLeft, Info, Square, Send } from 'lucide-react'
import { tasksApi, type LiveTaskSession } from '../../api/tasks'
import type { TaskDetail } from '../../api/types'
import { TaskReadyPage } from './TaskReadyPage'
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
    [terminalControls, setTerminalControls] = useState<HTMLDivElement | null>(
      null,
    ),
    [followup, setFollowup] = useState(''),
    [cancel, setCancel] = useState(false),
    [busy, setBusy] = useState(false),
    [showTerminal, setShowTerminal] = useState(false),
    [accept, setAccept] = useState(false),
    [acceptNote, setAcceptNote] = useState('')
  const following = useRef(false)
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
  const attempt =
    sessionId === 'latest'
      ? detail?.latest_attempt || detail?.attempts[0]
      : detail?.attempts.find((a) => a.id === sessionId)
  const waitingLaunch = detail?.launch?.state === 'queued' || detail?.launch?.state === 'claimed'
  const basePath = workspacePath(taskId, location.pathname)
  const suffix = location.search,
    state = location.state
  const detailParams = new URLSearchParams(suffix)
  detailParams.set('view', 'detail')
  const taskURL =
    '/tasks/' + encodeURIComponent(taskId) + '?' + detailParams.toString()
  const backURL =
    typeof state?.taskReturnTo === 'string' &&
    /^\/(?:tasks|projects\/[^/?#]+(?:\/tasks)?|inbox\/[^/?#]+)(?:\?[^#]*)?$/.test(
      state.taskReturnTo,
    )
      ? state.taskReturnTo
      : basePath.startsWith('/projects/')
        ? basePath.split('/tasks/')[0] + '/tasks'
        : '/tasks' + suffix
  const detailState = {
    ...state,
    workspaceReturnTo: location.pathname + location.search,
  }
  useEffect(() => {
    if (sessionId === 'latest' && attempt && !waitingLaunch)
      navigate(
        basePath + '/sessions/' + encodeURIComponent(attempt.id) + suffix,
        { replace: true, state },
      )
  }, [sessionId, attempt?.id, taskId, suffix, navigate, state, basePath, waitingLaunch])
  useEffect(() => {
    setShowTerminal(false)
    setAccept(false)
    setFollowup('')
    following.current = false
  }, [sessionId])

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
  if ((!attempt || waitingLaunch) && sessionId === 'latest') return <TaskReadyPage task={detail} />
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
  const latest = detail.latest_attempt?.id === attempt.id
  const final = [...reports]
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))
    .find((r) => ['review', 'blocked', 'done'].includes(r.new_status || ''))
  const answer =
    final?.new_status === 'review' &&
    attempt.status === 'succeeded' &&
    detail.activities.some(
      (a) =>
        a.attempt_id === attempt.id &&
        a.activity_type === 'agent_result_kind' &&
        a.created_at === final.created_at &&
        a.new_status === 'answer',
    )
  const result =
    final?.message.match(
      /## Work summary\n([\s\S]*?)(?:\n\n## Verification|$)/,
    )?.[1] ||
    (latest ? detail.summary : '') ||
    ''
  const checklist = (detail.dod_checklist || []).map((c) =>
    typeof c === 'string' ? { text: c, completed: false } : c,
  )
  const canAccept =
    latest &&
    detail.status === 'review' &&
    !active &&
    !answer &&
    attempt.status === 'succeeded' &&
    !!result.trim() &&
    !!detail.complete_note

  return (
    <section
      aria-label="Task session"
      className="flex h-full min-h-0 flex-col bg-background"
    >
      <header className="flex shrink-0 items-center gap-2 border-b px-2 py-1.5">
        <Link
          to={backURL}
          state={state}
          className="icon-action shrink-0"
          aria-label={
            backURL.startsWith('/inbox/')
              ? 'Back to conversation'
              : 'Back to tasks'
          }
          title={
            backURL.startsWith('/inbox/')
              ? 'Back to conversation'
              : 'Back to tasks'
          }
        >
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-sm font-semibold" title={detail.title}>
            {detail.title}
          </h2>
          <p className="truncate text-xs text-muted-foreground">
            {!latest
              ? attemptLabel(attempt)
              : active
                ? attemptLabel(attempt)
                : detail.status === 'done'
                  ? 'Completed'
                  : answer
                    ? 'Answered'
                    : detail.status === 'blocked'
                      ? 'Needs attention'
                      : 'Ready for review'}{' '}
            · <span className="hidden sm:inline">Session </span>
            {attempt.attempt_number}
          </p>
        </div>
        <div ref={setTerminalControls} className="flex shrink-0 items-center" />
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
      {active || showTerminal ? (
        <div className="flex min-h-0 flex-1 flex-col">
          {!active && (
            <button
              className="action-secondary m-2 text-xs"
              onClick={() => setShowTerminal(false)}
            >
              Back to result
            </button>
          )}
          <div className="min-h-0 flex-1">
            <TaskTerminal
              key={sessionId}
              taskId={taskId}
              attempt={attempt}
              live={live}
              onRefresh={() => void refresh()}
              controlsTarget={terminalControls}
            />
          </div>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-3xl space-y-6 p-4 sm:p-6">
            <section aria-label="Original request">
              <h3 className="mb-2 text-xs font-medium text-muted-foreground">
                Your request
              </h3>
              <div className="max-h-40 overflow-y-auto">
                <ChatMarkdown content={detail.description || detail.title} />
              </div>
            </section>
            <section aria-label="Session result">
              <h3 className="mb-2 text-sm font-semibold">
                {answer ? 'Answer' : 'Result'}
              </h3>
              {result ? (
                <ChatMarkdown content={result} />
              ) : (
                <p className="text-sm text-muted-foreground">
                  No result was recorded for this session. Open the terminal or
                  Detail to check what happened.
                </p>
              )}
            </section>
            <div className="flex flex-wrap items-center gap-2">
              {canAccept && (
                <button
                  className="action-primary"
                  onClick={() => {
                    setAcceptNote('')
                    setAccept(true)
                  }}
                >
                  Accept
                </button>
              )}
              {canAccept && detail.assignee_id && (
                <button
                  className="action-secondary"
                  onClick={() =>
                    document
                      .querySelector<HTMLTextAreaElement>(
                        'textarea[aria-label="Follow-up instructions"]',
                      )
                      ?.focus()
                  }
                >
                  Request changes
                </button>
              )}
              {!result && (
                <button
                  className="action-secondary"
                  onClick={() => setShowTerminal(true)}
                >
                  Open terminal
                </button>
              )}
            </div>
            {!latest && (
              <p className="text-xs text-muted-foreground">
                Earlier session · select the latest session in Detail to
                continue.
              </p>
            )}
            {latest && detail.status === 'done' && (
              <p className="text-sm text-muted-foreground">Task completed.</p>
            )}
          </div>
        </div>
      )}
      {!active &&
        detail.latest_attempt?.id === attempt.id &&
        detail.status !== 'done' &&
        detail.assignee_id && (
          <form
            className="flex shrink-0 items-end gap-2 border-t bg-background p-2"
            onSubmit={(e) => {
              e.preventDefault()
              if (!followup.trim() || busy || following.current) return
              following.current = true
              setBusy(true)
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
                navigate(basePath + '/sessions/start' + suffix, {
                  state: {
                    taskReturnTo: state?.taskReturnTo,
                    startRequestId: request.request_id,
                  },
                })
              } catch {
                following.current = false
                setBusy(false)
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
                placeholder={
                  answer
                    ? 'Reply to the agent…'
                    : 'What should change or happen next?'
                }
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
          <Link className="text-sm underline" to={taskURL} state={detailState}>
            Open task details · {taskRef(taskId)}
          </Link>
          <button
            className="action-secondary"
            onClick={() => {
              setShowDetail(false)
              setShowTerminal(true)
            }}
          >
            Open terminal
          </button>
          <label className="block text-xs text-muted-foreground">
            Session
            <select
              aria-label="Select session"
              className="field-control mt-1 text-sm"
              value={sessionId}
              onChange={(e) => {
                setShowDetail(false)
                navigate(
                  basePath +
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
          {latest && checklist.length > 0 && (
            <details className="rounded-lg border p-3">
              <summary className="cursor-pointer text-sm font-medium">
                Agent checklist
              </summary>
              <p className="my-2 text-xs text-muted-foreground">
                Reported by agent · compare with your original request before
                accepting.
              </p>
              {checklist.map((item, index) => (
                <label
                  key={item.id || index}
                  className="flex min-h-11 items-start gap-2 py-2 text-sm"
                >
                  <input
                    type="checkbox"
                    aria-label={'Verify: ' + item.text}
                    checked={item.completed}
                    disabled={busy || active || detail.status === 'done'}
                    onChange={async () => {
                      setBusy(true)
                      try {
                        await tasksApi.update(taskId, {
                          dod_checklist: checklist.map((c, i) =>
                            i === index ? { ...c, completed: !c.completed } : c,
                          ),
                          expected_updated_at: detail.updated_at,
                        })
                        await refresh()
                      } catch (e) {
                        setError(
                          e instanceof Error
                            ? e.message
                            : 'Could not save checklist.',
                        )
                      } finally {
                        setBusy(false)
                      }
                    }}
                    className="mt-1 h-4 w-4 shrink-0"
                  />
                  <span>{item.text}</span>
                </label>
              ))}
            </details>
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
                      {r.new_status === 'done'
                        ? 'Completed'
                        : r.new_status === 'review'
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
        open={accept}
        title="Accept result"
        busy={busy}
        onClose={() => setAccept(false)}
      >
        <form
          className="space-y-4"
          onSubmit={async (e) => {
            e.preventDefault()
            if (
              busy ||
              !acceptNote.trim() ||
              checklist.some((c) => !c.completed)
            )
              return
            setBusy(true)
            try {
              await tasksApi.update(taskId, {
                status: 'done',
                complete_note: acceptNote.trim(),
                expected_updated_at: detail.updated_at,
              })
              setAccept(false)
              await refresh()
            } catch (e) {
              setError(
                e instanceof Error ? e.message : 'Could not accept the result.',
              )
            } finally {
              setBusy(false)
            }
          }}
        >
          <p className="text-sm text-muted-foreground">
            Confirm the result meets your request, stays within scope, and uses
            the correct workspace.
          </p>
          <p className="max-h-32 overflow-y-auto whitespace-pre-wrap rounded-lg border p-3 text-sm">
            {detail.description || detail.title}
          </p>
          {checklist.some((c) => !c.completed) && (
            <p className="text-sm text-destructive">
              Verify the remaining checklist items in Detail before accepting.
            </p>
          )}
          <label className="grid gap-1 text-sm">
            Verification
            <textarea
              aria-label="Acceptance evidence"
              required
              rows={3}
              maxLength={16000}
              className="field-control"
              value={acceptNote}
              onChange={(e) => setAcceptNote(e.target.value)}
              placeholder="How you verified this meets the request."
            />
          </label>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <button
            className="action-primary"
            disabled={
              busy || !acceptNote.trim() || checklist.some((c) => !c.completed)
            }
          >
            Accept result
          </button>
        </form>
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
