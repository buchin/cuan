import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  CheckCircle2,
  ChevronRight,
  CircleAlert,
  Clock3,
  LoaderCircle,
  Square,
  RotateCcw,
} from 'lucide-react'
import { InboxRequest } from '../../api/chat'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import { inboxRecoveryControls } from './inbox-recovery'
import { StatusDot } from '../../components/ui/StatusDot'
import { AccordionTimeline } from '../../components/ui/AccordionTimeline'
import { ChatMarkdown } from './ChatMarkdown'

export function evaluateInboxQuotaStatus(q: {
  steps?: { seq: number; kind: string; text: string; at: string }[]
  recovery_action?: string
  safe_error?: string
}) {
  const recovery = q.recovery_action || 'check_status'

  // Find the last step indicating provider cooldown or usage limit
  const lastCooldownIdx = q.steps
    ? q.steps.reduce(
        (last, step, idx) =>
          step.kind === 'provider_cooldown' || /usage limit|quota/i.test(step.text.split('\n')[0]) ? idx : last,
        -1,
      )
    : -1

  // Historical cooldown is superseded if subsequent failover or provider attempts were executed
  const hasSubsequentFailover =
    lastCooldownIdx !== -1 &&
    (q.steps?.slice(lastCooldownIdx + 1).some((step) =>
      [
        'account_failover',
        'model_fallback',
        'model_selected',
        'model_auto',
        'provider',
        'runtime_provider_started',
        'runtime_provider_finished',
        'runtime_tool_started',
        'output',
        'result_validation',
        'delegation',
      ].includes(step.kind),
    ) ?? false)

  const hasActiveQuotaCooldown = lastCooldownIdx !== -1 && !hasSubsequentFailover
  const hasQuotaCooldown = hasActiveQuotaCooldown
  const isExplicitQuotaError = /usage limit|quota|no allowed account\/model route/i.test(q.safe_error || '')
  const isTechnicalNonQuotaError = /timed out|timeout|connection|network|refused|syntax|parse|5\d{2}|4\d{2}/i.test(q.safe_error || '')
  const accessBlocked =
    isExplicitQuotaError ||
    ((recovery === 'check_usage' || hasActiveQuotaCooldown) && !isTechnicalNonQuotaError && !hasSubsequentFailover)

  return {
    lastCooldownIdx,
    hasSubsequentFailover,
    hasActiveQuotaCooldown,
    hasQuotaCooldown,
    isExplicitQuotaError,
    isTechnicalNonQuotaError,
    accessBlocked,
  }
}

export function TypingDots({
  className = '',
  size = 'md',
  'aria-hidden': ariaHidden = true,
}: {
  className?: string
  size?: 'sm' | 'md'
  'aria-hidden'?: boolean | 'true' | 'false'
}) {
  const cleanClassName = className.replace(/\banimate-spin\b/g, '').trim()
  const dotSize = size === 'sm' || cleanClassName.includes('h-3.5') ? 'h-1 w-1' : 'h-1.5 w-1.5'
  const gap = size === 'sm' || cleanClassName.includes('h-3.5') ? 'gap-0.5' : 'gap-1'
  return (
    <span
      data-testid="inbox-activity-typing-dots"
      aria-hidden={ariaHidden}
      className={`inline-flex items-center ${gap} h-4 shrink-0 text-muted-foreground ${cleanClassName}`}
    >
      <span
        className={`${dotSize} rounded-full bg-current typing-dot-1 animate-pulse motion-reduce:animate-none`}
        style={{ animationDuration: '1.4s', animationDelay: '0ms' }}
      />
      <span
        className={`${dotSize} rounded-full bg-current typing-dot-2 animate-pulse motion-reduce:animate-none`}
        style={{ animationDuration: '1.4s', animationDelay: '200ms' }}
      />
      <span
        className={`${dotSize} rounded-full bg-current typing-dot-3 animate-pulse motion-reduce:animate-none`}
        style={{ animationDuration: '1.4s', animationDelay: '400ms' }}
      />
    </span>
  )
}

export const INBOX_ACTIVITY_VERBS = [
  'Exploring',
  'Analyzing',
  'Synthesizing',
  'Reviewing',
  'Connecting',
  'Checking',
  'Structuring',
  'Curating',
  'Drafting',
  'Organizing',
  'Refining',
  'Preparing',
  'Inspecting',
  'Formulating',
  'Evaluating',
  'Processing',
  'Researching',
  'Polishing',
  'Verifying',
  'Brainstorming',
  'Conceptualizing',
  'Envisioning',
  'Strategizing',
  'Deliberating',
  'Investigating',
  'Discovering',
  'Gathering',
  'Sifting',
  'Surveying',
  'Examining',
  'Browsing',
  'Uncovering',
  'Categorizing',
  'Arranging',
  'Mapping',
  'Outlining',
  'Sorting',
  'Cataloging',
  'Coordinating',
  'Systematizing',
  'Composing',
  'Writing',
  'Assembling',
  'Creating',
  'Crafting',
  'Designing',
  'Generating',
  'Calculating',
  'Computing',
  'Estimating',
  'Forecasting',
  'Budgeting',
  'Assessing',
  'Auditing',
  'Quantifying',
  'Measuring',
  'Editing',
  'Enhancing',
  'Streamlining',
  'Perfecting',
  'Tailoring',
  'Fine-tuning',
  'Clarifying',
  'Harmonizing',
  'Planning',
  'Scheduling',
  'Aligning',
  'Optimizing',
  'Prioritizing',
  'Consolidating',
  'Orchestrating',
  'Validating',
  'Balancing',
  'Finalizing',
  'Summarizing',
  'Distilling',
  'Reconciling',
  'Cross-checking',
  'Imagining',
  'Thinking',
] as const

export const INBOX_ACTIVITY_CONTEXT_OBJECTS = [
  'workspace',
  'tasks',
  'context',
  'conversation',
  'project details',
  'inbox activity',
  'workflow',
  'requirements',
  'resources',
  'brief',
  'milestones',
  'timeline',
  'action items',
  'deliverables',
  'notes',
  'guidelines',
] as const

export function formatActivityPhrase(verb: string, contextObject: string): string {
  return `${verb} ${contextObject}…`
}

export const INBOX_ACTIVITY_ROTATING_PHRASES: string[] = (() => {
  const phrases: string[] = []
  // Generate multi-cycle variety (160 distinct dynamic pairs) so the assistant never feels repetitive
  for (let cycle = 0; cycle < 2; cycle++) {
    for (let i = 0; i < INBOX_ACTIVITY_VERBS.length; i++) {
      const verb = INBOX_ACTIVITY_VERBS[i]
      const objIndex = (i + cycle * 5) % INBOX_ACTIVITY_CONTEXT_OBJECTS.length
      phrases.push(formatActivityPhrase(verb, INBOX_ACTIVITY_CONTEXT_OBJECTS[objIndex]))
    }
  }
  return phrases
})()

export const INBOX_ACTIVITY_ROTATING_WORDS = INBOX_ACTIVITY_ROTATING_PHRASES

export const ACTIVITY_ROTATION_INTERVAL_MS = 3500

export function InboxActivity({
  request: q,
  onCancel,
  onRetry,
  onCheckStatus,
  previousAttempts = [],
  bubble = false,
  streamingText,
}: {
  request: InboxRequest
  onCancel: () => Promise<void>
  onRetry?: () => Promise<void>
  onCheckStatus?: () => Promise<void>
  previousAttempts?: InboxRequest[]
  bubble?: boolean
  streamingText?: string
}) {
  const { t, language } = useLanguage()

  const [displayedText, setDisplayedText] = useState(streamingText || '')
  const targetTextRef = useRef(streamingText || '')
  targetTextRef.current = streamingText || ''
  const animFrameRef = useRef<number | null>(null)

  useEffect(() => {
    if (!streamingText) {
      setDisplayedText('')
      if (animFrameRef.current) {
        cancelAnimationFrame(animFrameRef.current)
        animFrameRef.current = null
      }
      return
    }

    const step = () => {
      const target = targetTextRef.current
      setDisplayedText((prev) => {
        if (prev.length >= target.length) {
          animFrameRef.current = null
          return target
        }
        const gap = target.length - prev.length
        const charsToAdd = Math.max(1, Math.min(gap, Math.ceil(gap / 6)))
        return target.slice(0, prev.length + charsToAdd)
      })
      if (targetTextRef.current.length > displayedText.length) {
        animFrameRef.current = requestAnimationFrame(step)
      } else {
        animFrameRef.current = null
      }
    }

    if (!animFrameRef.current) {
      animFrameRef.current = requestAnimationFrame(step)
    }

    return () => {
      if (animFrameRef.current) {
        cancelAnimationFrame(animFrameRef.current)
        animFrameRef.current = null
      }
    }
  }, [streamingText])

  const [now, setNow] = useState(Date.now())
  const [expanded, setExpanded] = useState(false)
  const timelineBody = useRef<HTMLDivElement>(null)
  const followTimeline = useRef(true)
  const previousTimelineTop = useRef(0)
  const wasTimelineActive = useRef(false)
  const [timelinePaused, setTimelinePaused] = useState(false)
  const [error, setError] = useState('')
  const [cancelling, setCancelling] = useState(false)
  const [copyStatus, setCopyStatus] = useState('')
  const [recovering, setRecovering] = useState(false)
  const [recoveryStatus, setRecoveryStatus] = useState('')
  const [retryUnavailable, setRetryUnavailable] = useState(false)
  const failed = ['failed', 'interrupted', 'cancelled'].includes(q.state)
  const recovery = q.recovery_action || 'check_status'
  const autoPending = recovery === 'recovering'
  const isToolBudget = recovery === 'continue_analysis' || /tool budget/i.test(q.safe_error || '') || q.steps?.some((step) => step.kind === 'runtime_tool_budget_exceeded' || /tool budget|tool limit/i.test(step.text.split('\n')[0]))

  const { accessBlocked } = evaluateInboxQuotaStatus(q)
  const { canRetry: retryPermitted, waitingForCapacity: capacityPending } = inboxRecoveryControls(q.state, q.recovery_action, accessBlocked)
  const canRetry = retryPermitted && !retryUnavailable
  const waitingForCapacity = capacityPending && !retryUnavailable
  const recover = async (action: () => Promise<void>) => {
    if (recovering) return
    setRecovering(true)
    setError('')
    setRecoveryStatus('')
    try { await action(); if (action === onCheckStatus) setRecoveryStatus(t('Status is up to date.')) }
    catch (err: any) {
      const msg = err?.message || ''
      if (/cannot be safely repeated/i.test(msg)) {
        setRetryUnavailable(true)
        setExpanded(true)
      } else {
        setError(t('Could not confirm recovery. Check status before trying again.'))
      }
    }
    finally { setRecovering(false) }
  }

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
        'runtime_tool_budget_exceeded',
        'result_validation',
        'delegation',
        'persistence',
      ].includes(step.kind),
    )

  const hasSpecificStep = Boolean(latestActivity?.text?.trim())
  const hasStreaming = Boolean(streamingText && streamingText.length > 0)
  const shouldRotate = q.state === 'running' && !autoPending && !hasSpecificStep && !hasStreaming

  const [rotatingIndex, setRotatingIndex] = useState(0)
  const [fading, setFading] = useState(false)

  // Reset index when request_id changes
  useEffect(() => {
    setRotatingIndex(0)
    setFading(false)
  }, [q.request_id])

  // Periodic word rotation with clean lifecycle management
  useEffect(() => {
    if (!shouldRotate) {
      setFading(false)
      return
    }

    let fadeTimer: ReturnType<typeof setTimeout> | undefined

    const rotationTimer = setInterval(() => {
      setFading(true)
      fadeTimer = setTimeout(() => {
        setRotatingIndex((prev) => (prev + 1) % INBOX_ACTIVITY_ROTATING_WORDS.length)
        setFading(false)
      }, 250)
    }, ACTIVITY_ROTATION_INTERVAL_MS)

    return () => {
      clearInterval(rotationTimer)
      if (fadeTimer) clearTimeout(fadeTimer)
    }
  }, [shouldRotate, q.request_id])

  const rotatingWord = INBOX_ACTIVITY_ROTATING_WORDS[rotatingIndex % INBOX_ACTIVITY_ROTATING_WORDS.length]
  const activityText = hasSpecificStep
    ? Array.from(latestActivity!.text.split('\n')[0]).slice(0, 140).join('')
    : rotatingWord
  const autoRecovery = q.steps?.some((step) => step.kind === 'retry_auto')
  const active = ['queued', 'running', 'cancelling'].includes(q.state)
  const lastStepSeq = q.steps?.at(-1)?.seq
  useEffect(() => {
    followTimeline.current = true
    previousTimelineTop.current = 0
    setTimelinePaused(false)
  }, [expanded, q.request_id])
  useEffect(() => {
    const previouslyActive = wasTimelineActive.current
    wasTimelineActive.current = active
    if (!expanded || (!active && !previouslyActive) || !followTimeline.current) return
    const frame = requestAnimationFrame(() => {
      const body = timelineBody.current
      if (body) body.scrollTop = body.scrollHeight
    })
    return () => cancelAnimationFrame(frame)
  }, [expanded, active, lastStepSeq])
  useEffect(() => {
    if (!active) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [active])
  const compact = q.state === 'completed' && !bubble
  if (compact && !q.steps?.some(step => step.kind.startsWith('runtime_tool_') && step.text.includes('\n\n'))) {
    return null
  }
  const start = Date.parse(q.started_at || q.created_at)
  const finished = q.steps
    ?.slice()
    .reverse()
    .find((step) => step.kind === 'provider_result')
  const terminal = q.steps?.slice().reverse().find((step) => step.kind === 'finished')
  // A provider_result can describe only an earlier stage. Never show that
  // duration as the total of an interrupted multi-stage reply.
  const end = terminal
    ? Date.parse(terminal.at)
    : q.state === 'completed' && finished ? Date.parse(finished.at) : NaN
  const duration = active ? now - start : end - start
  const seconds =
    Number.isFinite(duration) && duration >= 0
      ? Math.floor(duration / 1000)
      : null
  const formatQueueReason = (reason?: string) => {
    if (!reason) return ''
    if (reason === 'waiting_verification') {
      return language === 'id' ? 'Menunggu verifikasi tugas selesai…' : 'Waiting for task verification…'
    }
    if (reason === 'waiting_previous_message') {
      return language === 'id' ? 'Menunggu respon pesan sebelumnya…' : 'Waiting for previous message…'
    }
    if (reason === 'waiting_capacity') {
      return language === 'id' ? 'Menunggu kapasitas AI tersedia…' : 'Waiting for AI capacity…'
    }
    if (reason.startsWith('queue_position:')) {
      const pos = reason.split(':')[1]
      return language === 'id' ? `Antrean pesan ke-${pos}…` : `Position ${pos} in queue…`
    }
    return reason
  }
  const queueLabel = formatQueueReason(q.queue_reason)
  const label = {
    queued: queueLabel || (q.safe_error?.startsWith('AI capacity is temporarily full.') ? 'Waiting for capacity' : 'Queued'),
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
        step.kind !== 'provider_result' && step.kind !== 'runtime_session' && step.kind !== 'recovery_policy' && step.kind !== 'response_input',
    ) || []
  const isRunning = active && q.state !== 'queued' && !autoPending
  const Icon =
    (autoPending || waitingForCapacity) ? Clock3 : q.state === 'queued'
      ? Clock3
      : active
        ? TypingDots
        : q.state === 'completed'
          ? CheckCircle2
          : CircleAlert
  return (
    <div
      data-bubble-tail={bubble ? 'incoming' : undefined}
      className={
        bubble
          ? 'wa-message-bubble wa-incoming relative min-w-[min(100%,272px)] rounded-2xl text-xs break-words'
          : compact
            ? 'max-w-full min-w-0 text-xs break-words'
            : 'mt-2 max-w-full min-w-0 text-xs break-words'
      }
      aria-label={t('Response status')}
      data-response-state={q.state}
    >
      {bubble && (displayedText || streamingText) ? (
        <div className="wa-plain-text break-words font-sans mb-1.5">
          <ChatMarkdown content={displayedText || streamingText || ''} />
          <span
            className="inline-block w-1.5 h-3.5 ml-1 align-baseline bg-foreground/60 animate-pulse rounded-xs"
            aria-hidden="true"
          />
        </div>
      ) : null}
      {bubble && !(displayedText || streamingText) && (
        <div className="flex items-center gap-2 pt-0.5 pb-1 text-sm">
          {isRunning ? (
            <TypingDots />
          ) : (
            <Icon
              aria-hidden
              className={
                'h-4 w-4 shrink-0 ' +
                (autoPending
                  ? 'animate-spin'
                  : q.state === 'failed' && !waitingForCapacity
                    ? 'text-destructive'
                    : '')
              }
            />
          )}
          <span
            role="status"
            className="min-w-0 flex-1 font-medium break-words"
            aria-live="polite"
          >
            {autoPending || (q.state === 'queued' && autoRecovery)
              ? t('Recovering reply…')
              : q.state === 'running'
              ? (
                <span
                  data-testid="inbox-activity-text"
                  className={`inline-block transition-opacity duration-300 ease-in-out motion-reduce:transition-none ${
                    fading ? 'opacity-0' : 'opacity-100'
                  }`}
                >
                  {activityText}
                </span>
              )
              : q.state === 'queued' && queueLabel
              ? queueLabel
              : q.state === 'failed'
                ? (waitingForCapacity ? t('Waiting for AI capacity') : t('Could not reply'))
                : q.state === 'interrupted' ? t('Reply interrupted') : t(label)}
          </span>
          {!autoPending && seconds !== null && (
            <span className="shrink-0 whitespace-nowrap text-xs tabular-nums text-muted-foreground">
              {seconds}s
            </span>
          )}
        </div>
      )}
      <div className={bubble ? 'flex min-w-0 items-center justify-between gap-2' : 'flex min-w-0 flex-col gap-1 sm:flex-row sm:items-center sm:justify-between sm:gap-2'}>
        <div className="min-w-0 flex-1">
          <button type="button" aria-expanded={expanded} onClick={() => setExpanded(true)}
            className={
              compact
                ? 'inline-flex min-h-11 cursor-pointer list-none items-center gap-1 px-1 text-xs text-muted-foreground hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden'
                : bubble
                  ? '-ml-1.5 inline-flex min-h-10 cursor-pointer list-none items-center gap-1.5 rounded-md px-1.5 text-muted-foreground hover:bg-muted/50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden'
                  : 'flex min-h-11 w-full cursor-pointer list-none items-center gap-2 rounded-md px-2 text-muted-foreground hover:bg-muted/50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden'
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
              isRunning ? (
                <TypingDots />
              ) : (
                <Icon
                  className={`h-4 w-4 shrink-0 ${q.state === 'failed' && !waitingForCapacity ? 'text-destructive' : q.state === 'completed' ? 'text-emerald-700 dark:text-emerald-400' : ''}`}
                  aria-hidden="true"
                />
              )
            )}
            {!bubble && !compact && (
              <span className="min-w-0 text-left font-medium" role="status">
                {label}
              </span>
            )}
            {!bubble && !compact && seconds !== null && (
              <span className="shrink-0 whitespace-nowrap tabular-nums">
                {seconds}s{!active && ' recorded'}
              </span>
            )}
            {bubble && (displayedText || streamingText) && (
              isRunning ? (
                <TypingDots size="sm" />
              ) : (
                <Icon
                  className="h-3.5 w-3.5 shrink-0"
                  aria-hidden="true"
                />
              )
            )}
            {!compact && (
              <>
                <span className={bubble ? 'text-xs' : 'ml-auto shrink-0 whitespace-nowrap text-xs'}>
                  {t('Detail')}
                </span>
                {bubble && (displayedText || streamingText) && seconds !== null && (
                  <span className="tabular-nums text-xs">
                    · {seconds}s
                  </span>
                )}
                <ChevronRight
                  className="h-4 w-4 shrink-0"
                  aria-hidden="true"
                />
              </>
            )}
          </button>
          <SlideDrawer isOpen={expanded} onClose={() => setExpanded(false)} title={t('Response details')} description={t(label)} size="md"
            contentClassName="[overflow-anchor:none]"
            bodyRef={timelineBody}
            onBodyScroll={(event) => {
              const body = event.currentTarget
              const movedUp = body.scrollTop < previousTimelineTop.current - 1
              previousTimelineTop.current = body.scrollTop
              const atBottom = body.scrollHeight - body.clientHeight - body.scrollTop <= 2
              if (atBottom) followTimeline.current = true
              else if (movedUp) followTimeline.current = false
              setTimelinePaused(!followTimeline.current)
            }}
            footer={active && timelinePaused ? <button type="button" className="min-h-11 text-xs font-medium" onClick={() => {
              followTimeline.current = true
              setTimelinePaused(false)
              const body = timelineBody.current
              if (body) body.scrollTop = body.scrollHeight
            }}>{t('Latest activity')}</button> : undefined}
          >
            <div className="w-full min-w-0 space-y-5 text-sm">
              <p className="font-medium">
                {q.state === 'completed'
                  ? q.agent_id
                    ? 'Reply saved.'
                    : 'Message sent.'
                  : q.state === 'queued'
                    ? (queueLabel || (q.safe_error?.startsWith('AI capacity is temporarily full.') ? 'Your message is saved and will continue automatically.' : 'Message saved and queued.'))
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
              {previousAttempts.length > 0 && (
                <details className="text-xs">
                  <summary className="min-h-11 cursor-pointer py-3">{t('Previous attempts')} ({previousAttempts.length})</summary>
                  <ol className="space-y-3">
                    {previousAttempts.map((attempt) => <li key={attempt.request_id}>
                      <p>{new Date(attempt.created_at).toLocaleString(language)} · {t(attempt.state === 'failed' ? 'Failed' : attempt.state === 'interrupted' ? 'Interrupted' : attempt.state)}</p>
                      {attempt.safe_error && <p className="text-muted-foreground">{attempt.safe_error}</p>}
                    </li>)}
                  </ol>
                </details>
              )}
              {q.safe_error && !(q.state === 'queued' && q.safe_error.startsWith('AI capacity is temporarily full.')) && (
                <p className="text-muted-foreground">{q.safe_error}</p>
              )}
              {processSteps.length ? (
                <AccordionTimeline key={q.request_id} label="Response timeline" followLatest={!timelinePaused} items={processSteps.map((step, index) => {
                  const headline = step.text.split('\n')[0]
                  const detail = step.text.slice(headline.length).trim()
                  return {
                    id: String(step.seq),
                    title: <span className={step.kind.endsWith('_failed') ? 'text-destructive' : undefined}>{headline}</span>,
                    meta: Number.isFinite(Date.parse(step.at)) ? <time dateTime={step.at}>{new Date(step.at).toLocaleTimeString(language,{hour:'2-digit',minute:'2-digit'})}</time> : undefined,
                    indicator: <StatusDot aria-hidden size="md" status={active && index === processSteps.length - 1 ? 'working' : 'neutral'} />,
                    content: detail ? step.kind.startsWith('runtime_tool_') ? <pre data-tool-detail className="max-h-80 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed">{detail}</pre> : <p className="whitespace-pre-wrap break-words">{detail}</p> : undefined,
                  }
                })} />
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
                  {q.steps?.filter(step => step.kind === 'response_input').map(step => (
                    <div key={step.seq}>
                      <dt className="font-medium">Provider input</dt>
                      <dd className="break-words tabular-nums">{step.text}</dd>
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
          </SlideDrawer>
        </div>
        {(q.state === 'queued' || q.state === 'running') && (
          <button
            type="button"
            disabled={cancelling}
            className={
              bubble
                ? '-mr-1.5 inline-flex min-h-10 shrink-0 items-center gap-1 rounded-md px-1.5 text-muted-foreground hover:bg-muted/50 hover:text-foreground disabled:opacity-50'
                : 'inline-flex min-h-11 shrink-0 items-center gap-1 self-end px-2 text-muted-foreground hover:text-foreground disabled:opacity-50 sm:self-auto'
            }
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
            {t(cancelling ? 'Cancelling…' : q.state === 'queued' ? 'Cancel queued reply' : 'Stop reply')}
          </button>
        )}
      </div>
      {failed && (
        <div className={bubble ? 'pt-1 pb-1' : 'px-2 pb-2'}>
          <p className="mb-1 max-w-prose text-muted-foreground">
            {t(
              recovery === 'recovering'
                ? 'Your message is saved.'
                : waitingForCapacity
                  ? 'AI capacity was unavailable. Your message is saved.'
                  : isToolBudget
                    ? 'The tool budget was reached for this turn. Your message is saved.'
                    : recovery === 'check_status' || retryUnavailable
                      ? 'Your message is saved.'
                    : recovery === 'check_agent'
                      ? 'Check the agent connection before trying again.'
                      : q.safe_error
                        ? q.safe_error
                        : recovery === 'try_again'
                          ? 'Your message is saved. You can try again.'
                          : 'Your reply stopped. Your message is saved.'
            )}
          </p>
          {!waitingForCapacity && canRetry && isToolBudget && onRetry ? (
            <div className="flex flex-wrap items-center gap-3">
              <button
                type="button"
                disabled={recovering}
                className="inline-flex min-h-11 items-center gap-1.5 text-xs font-medium text-foreground underline underline-offset-4 disabled:opacity-50"
                onClick={() => void recover(onRetry)}
              >
                {recovering ? <LoaderCircle aria-hidden className="h-3.5 w-3.5 animate-spin" /> : <RotateCcw aria-hidden className="h-3.5 w-3.5" />}
                {t(recovering ? 'Continuing analysis…' : 'Continue analysis')}
              </button>
            </div>
          ) : !waitingForCapacity && canRetry && recovery === 'try_again' && onRetry ? (
            <button
              type="button"
              disabled={recovering}
              className="inline-flex min-h-11 items-center gap-1.5 text-xs font-medium text-foreground underline underline-offset-4 disabled:opacity-50"
              onClick={() => void recover(onRetry)}
            >
              {recovering ? <LoaderCircle aria-hidden className="h-3.5 w-3.5 animate-spin" /> : <RotateCcw aria-hidden className="h-3.5 w-3.5" />}
              {t(recovering ? 'Trying again…' : 'Try again')}
            </button>
          ) : !waitingForCapacity && recovery === 'check_agent' ? (
            <Link to="/settings/agents" className="inline-flex min-h-11 items-center text-xs font-medium underline underline-offset-4">
              {t('Check agent')}
            </Link>
          ) : !waitingForCapacity && !autoPending && (onCheckStatus || (canRetry && onRetry)) ? (
            <div className="flex flex-wrap items-center gap-3">
              {onCheckStatus && !canRetry && (
                <button
                  type="button"
                  disabled={recovering}
                  className="inline-flex min-h-11 items-center text-xs font-medium underline underline-offset-4 disabled:opacity-50"
                  onClick={() => void recover(onCheckStatus)}
                >
                  {t(recovering ? 'Checking…' : 'Check status')}
                </button>
              )}
              {onRetry && canRetry && (
                <button
                  type="button"
                  disabled={recovering}
                  className="inline-flex min-h-11 items-center gap-1.5 text-xs font-medium text-foreground underline underline-offset-4 disabled:opacity-50"
                  onClick={() => void recover(onRetry)}
                >
                  {recovering ? <LoaderCircle aria-hidden className="h-3.5 w-3.5 animate-spin" /> : <RotateCcw aria-hidden className="h-3.5 w-3.5" />}
                  {t(recovering ? 'Trying again…' : 'Try again')}
                </button>
              )}
            </div>
          ) : null}
          {waitingForCapacity && canRetry && onRetry && (
            <button
              type="button"
              disabled={recovering}
              className="mt-1 inline-flex min-h-11 items-center gap-1.5 text-xs text-muted-foreground underline underline-offset-4 hover:text-foreground disabled:opacity-50"
              onClick={() => void recover(onRetry)}
            >
              {recovering ? <LoaderCircle aria-hidden className="h-3.5 w-3.5 animate-spin" /> : <RotateCcw aria-hidden className="h-3.5 w-3.5" />}
              {t(recovering ? 'Trying again…' : 'Try again')}
            </button>
          )}
          {recoveryStatus && <p role="status" className="text-xs text-muted-foreground">{recoveryStatus}</p>}
        </div>
      )}
      {error && (
        <p role="alert" className="mt-1 text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
