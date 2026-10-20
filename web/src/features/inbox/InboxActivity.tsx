import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useState } from 'react'
import {
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  Clock3,
  LoaderCircle,
  Square,
} from 'lucide-react'
import { InboxRequest } from '../../api/chat'

export function InboxActivity({
  request: q,
  onCancel,
  bubble = false,
}: {
  request: InboxRequest
  onCancel: () => Promise<void>
  bubble?: boolean
}) {
  const { t, language } = useLanguage()

  const [now, setNow] = useState(Date.now())
  const [expanded, setExpanded] = useState(false)
  const [error, setError] = useState('')
  const [cancelling, setCancelling] = useState(false)
  const [copyStatus, setCopyStatus] = useState('')
  const latestActivity = q.steps
    ?.slice()
    .reverse()
    .find((step) =>
      [
        'context',
        'provider',
        'output',
        'contact_search',
        'runtime_init',
        'runtime_provider_started',
        'runtime_provider_finished',
        'runtime_tool_started',
        'runtime_tool_finished',
        'runtime_tool_failed',
        'runtime_permission_denied',
        'result_validation',
        'persistence',
      ].includes(step.kind),
    )
  const activityText = latestActivity?.text
    ? Array.from(latestActivity.text).slice(0, 140).join('')
    : t('Working…')
  const compact = q.state === 'completed' && !bubble
  const active = ['queued', 'running', 'cancelling'].includes(q.state)
  useEffect(() => {
    if (!active) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [active])
  const start = Date.parse(q.started_at || q.created_at)
  const finished = q.steps
    ?.slice()
    .reverse()
    .find((step) => step.kind === 'provider_result')
  const end = finished ? Date.parse(finished.at) : NaN
  const duration = active ? now - start : end - start
  const seconds =
    Number.isFinite(duration) && duration >= 0
      ? Math.floor(duration / 1000)
      : null
  const label = {
    queued: 'Queued',
    running: 'Running',
    cancelling: 'Cancelling…',
    completed: 'Completed',
    failed: 'Failed',
    interrupted: 'Interrupted',
    cancelled: 'Cancelled',
  }[q.state]
  const processSteps =
    q.steps?.filter(
      (step) =>
        step.kind !== 'provider_result' && step.kind !== 'runtime_session',
    ) || []
  const Icon =
    q.state === 'queued'
      ? Clock3
      : active
        ? LoaderCircle
        : q.state === 'completed'
          ? CheckCircle2
          : CircleAlert
  return (
    <div
      className={
        bubble || compact
          ? 'max-w-full min-w-0 text-xs break-words'
          : 'mt-2 max-w-full min-w-0 text-xs break-words'
      }
      aria-label={t('Response status')}
      data-response-state={q.state}
    >
      {bubble && (
        <div className="flex items-center gap-2 px-2 pt-2 text-sm">
          <Icon
            aria-hidden
            className={
              'h-4 w-4 shrink-0 ' +
              (active && q.state !== 'queued'
                ? 'animate-spin'
                : q.state === 'failed'
                  ? 'text-destructive'
                  : '')
            }
          />
          <span
            role="status"
            className="min-w-0 font-medium break-words"
            aria-live="polite"
          >
            {q.state === 'running'
              ? activityText
              : q.state === 'failed'
                ? 'Could not reply'
                : label}
          </span>
          {seconds !== null && (
            <span className="text-xs tabular-nums text-muted-foreground">
              {seconds} s
            </span>
          )}
        </div>
      )}
      <div className="flex items-start gap-2">
        <details
          className="group/response min-w-0 flex-1"
          onToggle={(event) => setExpanded(event.currentTarget.open)}
        >
          <summary
            className={
              compact
                ? 'inline-flex min-h-11 cursor-pointer list-none items-center gap-1 px-1 text-xs text-muted-foreground hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden'
                : 'flex min-h-11 cursor-pointer list-none items-center gap-2 rounded-md px-2 text-muted-foreground hover:bg-muted/50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden'
            }
          >
            {compact && (
              <>
                <span>{t('Detail')}</span>
                {seconds !== null && (
                  <>
                    <span aria-hidden="true">·</span>
                    <span className="tabular-nums">
                      {seconds} {language === 'id' ? 'dtk' : 's'}
                    </span>
                  </>
                )}
              </>
            )}
            {!bubble && !compact && (
              <Icon
                className={`h-4 w-4 shrink-0 ${active && q.state !== 'queued' ? 'animate-spin' : q.state === 'failed' ? 'text-destructive' : q.state === 'completed' ? 'text-emerald-700 dark:text-emerald-400' : ''}`}
                aria-hidden="true"
              />
            )}
            {!bubble && !compact && (
              <span className="font-medium" role="status">
                {label}
              </span>
            )}
            {!bubble && !compact && seconds !== null && (
              <span className="tabular-nums">
                {seconds} s{!active && ' recorded'}
              </span>
            )}
            {!compact && (
              <>
                <span className={bubble ? 'text-xs' : 'ml-auto text-xs'}>
                  {t('Detail')}
                </span>
                <ChevronDown
                  className="h-4 w-4 shrink-0 transition-transform group-open/response:rotate-180"
                  aria-hidden="true"
                />
              </>
            )}
          </summary>
          {expanded && (
            <div className="mt-1 space-y-3 rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
              <p className="font-medium">
                {q.state === 'completed'
                  ? q.agent_id
                    ? 'Reply saved.'
                    : 'Message sent.'
                  : q.state === 'queued'
                    ? 'Message saved and queued.'
                    : q.state === 'running'
                      ? 'The agent is preparing a reply.'
                      : q.state === 'interrupted'
                        ? "The process was interrupted. The provider's result is unknown."
                        : q.state === 'cancelled'
                          ? 'Request cancelled. Your message is still saved.'
                          : q.state === 'cancelling'
                            ? 'Waiting for cancellation to be confirmed.'
                            : 'The agent could not reply. Your message is still saved.'}
              </p>
              {q.safe_error && (
                <p className="text-muted-foreground">{q.safe_error}</p>
              )}
              {processSteps.length ? (
                <ol
                  aria-label="Response timeline"
                  className="space-y-3 border-l border-border pl-3"
                >
                  {processSteps.map((step) => (
                    <li key={step.seq} className="space-y-1">
                      {step.kind.startsWith('runtime_tool_') && (
                        <span
                          className={`text-xs font-medium ${step.kind === 'runtime_tool_failed' ? 'text-destructive' : 'text-muted-foreground'}`}
                        >
                          Tool
                          {step.kind === 'runtime_tool_failed' ? ' failed' : ''}
                        </span>
                      )}
                      {step.kind.startsWith('runtime_provider_') && (
                        <span
                          className={`text-xs font-medium ${step.kind === 'runtime_provider_failed' ? 'text-destructive' : 'text-muted-foreground'}`}
                        >
                          Provider
                          {step.kind === 'runtime_provider_failed'
                            ? ' failed'
                            : ''}
                        </span>
                      )}
                      <p>{step.text}</p>
                      {Number.isFinite(Date.parse(step.at)) && (
                        <time
                          dateTime={step.at}
                          className="block text-xs tabular-nums text-muted-foreground"
                        >
                          {new Date(step.at).toLocaleTimeString(language)}
                        </time>
                      )}
                    </li>
                  ))}
                </ol>
              ) : (
                <p className="text-muted-foreground">
                  No activity has been recorded yet.
                </p>
              )}
              <details className="border-t border-border pt-1 text-muted-foreground">
                <summary className="min-h-11 cursor-pointer py-3">
                  Technical details
                </summary>
                <dl className="space-y-3 text-xs">
                  <div>
                    <dt className="font-medium">Request ID</dt>
                    <dd className="break-all font-mono">{q.request_id}</dd>
                  </div>
                  <div>
                    <dt className="font-medium">Message ID</dt>
                    <dd className="break-all font-mono">{q.message_id}</dd>
                  </div>
                  <div>
                    <dt className="font-medium">Saved state</dt>
                    <dd>{q.state}</dd>
                  </div>
                  {q.agent_id && (
                    <div>
                      <dt className="font-medium">Agent</dt>
                      <dd className="break-all">{q.agent_id}</dd>
                    </div>
                  )}
                  <div>
                    <dt className="font-medium">Received</dt>
                    <dd>{new Date(q.created_at).toLocaleString(language)}</dd>
                  </div>
                  {q.started_at && (
                    <div>
                      <dt className="font-medium">Started</dt>
                      <dd>{new Date(q.started_at).toLocaleString(language)}</dd>
                    </div>
                  )}
                  {q.steps
                    ?.filter((step) => step.kind === 'runtime_session')
                    .map((step) => (
                      <div key={step.seq}>
                        <dt className="font-medium">Runtime session</dt>
                        <dd className="break-all font-mono">
                          {step.text.replace(
                            /^(?:Runtime session ID: |ID sesi runtime: )/,
                            '',
                          )}
                        </dd>
                      </div>
                    ))}
                  {finished && (
                    <div>
                      <dt className="font-medium">CLI result</dt>
                      <dd>{finished.text}</dd>
                    </div>
                  )}
                </dl>
                {!finished && !active && (
                  <p className="mt-3 text-xs">
                    Provider process metadata is unavailable for this request.
                  </p>
                )}
                <button
                  type="button"
                  className="my-2 min-h-11 text-sm underline"
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(
                        JSON.stringify(
                          {
                            request_id: q.request_id,
                            message_id: q.message_id,
                            agent_id: q.agent_id,
                            state: q.state,
                            created_at: q.created_at,
                            started_at: q.started_at,
                            steps: q.steps,
                            safe_error: q.safe_error,
                          },
                          null,
                          2,
                        ),
                      )
                      setCopyStatus('Details copied.')
                    } catch {
                      setCopyStatus(
                        'Could not copy. Select the request ID above to copy it manually.',
                      )
                    }
                  }}
                >
                  Copy technical details
                </button>
                {copyStatus && (
                  <p role="status" className="text-xs">
                    {copyStatus}
                  </p>
                )}
                <p className="text-xs">
                  Opening details or refreshing does not rerun the request.
                  Details contain recorded events only, without private
                  reasoning or credentials.
                </p>
              </details>
            </div>
          )}
        </details>
        {(q.state === 'queued' || q.state === 'running') && (
          <button
            type="button"
            disabled={cancelling}
            className="inline-flex min-h-11 shrink-0 items-center gap-1 px-2 text-muted-foreground underline disabled:opacity-50"
            onClick={async () => {
              setCancelling(true)
              setError('')
              try {
                await onCancel()
              } catch {
                setError(
                  'Cancellation is not confirmed. Check the status before trying again.',
                )
              } finally {
                setCancelling(false)
              }
            }}
          >
            <Square className="h-3 w-3" />
            {t('Cancel')}
          </button>
        )}
      </div>
      {error && (
        <p role="alert" className="mt-1 text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
