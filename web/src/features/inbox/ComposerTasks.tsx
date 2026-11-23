import { useEffect, useId, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { CheckCircle2, Clock3, Eye, AlertCircle, ChevronDown, Terminal, ArrowLeft } from 'lucide-react'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { StatusDot } from '../../components/ui/StatusDot'
import { AccordionTimeline } from '../../components/ui/AccordionTimeline'
import { workStatus, workDisplayStatus, taskExecutionActive, taskLiveActivityText, taskSummaryText, TASK_STATUS_THEMES } from '../tasks/task-ui'
import { tasksApi } from '../../api/tasks'
import { CardActions } from './CardActions'
import { TaskDuration } from '../tasks/TaskDuration'
import { cn } from '../../lib/utils'

// Key this component by conversation so disclosure choices never leak between rooms.
export function ComposerTasks({ tasks, onDetails, onTerminal, modes }: {
  tasks: Task[]
  onDetails: (task: Task) => void
  onTerminal: (task: Task) => void
  modes?: ReactNode
}) {
  const { t } = useLanguage()
  const id = useId()
  const trigger = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const prevOpenRef = useRef(false)
  const prevCountRef = useRef(0)
  const [open, setOpen] = useState(false)

  // Reverse-chronological: newest task always rendered at the top
  const ordered = useMemo(
    () =>
      [...new Map(tasks.map(task => [task.id, task])).values()].sort(
        (a, b) =>
          (Date.parse(b.created_at) || 0) - (Date.parse(a.created_at) || 0) ||
          b.id.localeCompare(a.id),
      ),
    [tasks],
  )

  useEffect(() => {
    const wasOpened = open && !prevOpenRef.current
    const taskAdded = open && ordered.length > prevCountRef.current
    prevOpenRef.current = open
    prevCountRef.current = ordered.length

    if (!wasOpened && !taskAdded) return
    const panel = panelRef.current
    if (!panel) return

    let canceled = false
    let frame2: number | null = null

    const scrollToBottom = () => {
      if (canceled || !panelRef.current) return
      const el = panelRef.current
      const prefersReducedMotion = typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
      const behavior = prefersReducedMotion ? 'auto' : 'smooth'
      // Reverse-chronological timeline maintains top alignment for newest task
      if (typeof el.scrollTo === 'function') {
        el.scrollTo({ top: el.scrollHeight ? 0 : 0, behavior })
      } else {
        el.scrollTop = 0
      }
    }

    const raf = typeof requestAnimationFrame === 'function' ? requestAnimationFrame : (cb: FrameRequestCallback) => setTimeout(cb, 16) as unknown as number
    const caf = typeof cancelAnimationFrame === 'function' ? cancelAnimationFrame : (id: number) => clearTimeout(id)

    const frame1 = raf(() => {
      frame2 = raf(scrollToBottom)
    })

    return () => {
      canceled = true
      caf(frame1)
      if (frame2 !== null) caf(frame2)
    }
  }, [open, ordered.length])

  if (!tasks.length && !modes) return null
  const latestTask = ordered[0]
  const defaultExpandedIds = latestTask ? [latestTask.id] : []

  const hasAttention = ordered.some(t => workStatus(t) === 'Needs attention')
  const hasActive = ordered.some(taskExecutionActive)
  const hasReview = !hasActive && ordered.some(t => workStatus(t) === 'Ready for review')
  const isAllDone = ordered.length > 0 && ordered.every(t => ['Completed', 'Answered', 'Cancelled'].includes(workStatus(t))) && ordered.some(t => ['Completed', 'Answered'].includes(workStatus(t)))
  const hasQueued = !hasActive && !hasReview && !isAllDone && ordered.some(t => workStatus(t) === 'Queued')

  const badgeStatus = hasAttention ? 'attention'
    : hasActive ? 'working'
    : hasReview ? 'review'
    : isAllDone ? 'done'
    : hasQueued ? 'queued'
    : 'neutral'

  return <section className="composer-tasks border-b border-border/40 px-3 sm:px-6 py-0.5 transition-colors z-10 shrink-0" aria-label={t('Conversation tasks')} data-testid="task-banner" onKeyDown={event => {
    if (event.key === 'Escape' && open) {
      event.stopPropagation()
      setOpen(false)
      trigger.current?.focus()
    }
  }}>
    <div className="calm-stream-constraint max-w-[75ch] mx-auto w-full flex items-center justify-between gap-3">
      {modes}
      {tasks.length > 0 && <button ref={trigger} type="button" aria-expanded={open} aria-controls={open ? id : undefined}
        aria-label={t('Tasks')}
        data-testid="composer-tasks-badge"
        data-status={badgeStatus}
        className={cn(
          "composer-mode-tab flex-1 flex items-center justify-between gap-2.5 min-w-0 min-h-11 sm:min-h-8 px-2 -mx-2 rounded-md text-left transition-colors cursor-pointer select-none",
          "hover:bg-muted/30 active:bg-muted/50 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
          open && "sr-only"
        )}
        onClick={() => setOpen(value => !value)}>
        {latestTask && (
          <div className="flex items-center gap-2 min-w-0 flex-1">
            {hasAttention ? (
              <AlertCircle className="h-3.5 w-3.5 shrink-0 text-destructive" aria-label={t('Needs attention')} />
            ) : hasActive ? (
              <StatusDot status="working" size="xs" label={t('In progress')} />
            ) : hasReview ? (
              <Eye className={cn("h-3.5 w-3.5 shrink-0", TASK_STATUS_THEMES.review.textColor)} aria-label={t('Ready for review')} />
            ) : isAllDone ? (
              <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-[color:var(--status-success)]" aria-label={t('Completed')} />
            ) : hasQueued ? (
              <Clock3 className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-label={t('Queued')} />
            ) : (
              <StatusDot status="neutral" size="xs" label={t('Status')} />
            )}
            <span className="text-xs font-medium text-foreground line-clamp-2 min-w-0 flex-1 sm:line-clamp-1 leading-snug" title={latestTask.title}>
              {latestTask.title}
            </span>
          </div>
        )}
        <div className="ml-auto inline-flex shrink-0 items-center gap-1.5">
          <span className={cn(
            "composer-tasks-count tabular-nums px-1.5 py-0.5 text-[11px] font-medium rounded-full",
            TASK_STATUS_THEMES[badgeStatus]?.counterBadgeClass || "bg-muted/80 text-muted-foreground border border-border/30"
          )}>{ordered.length}</span>
          <span className="inline-flex min-h-11 min-w-11 sm:min-h-8 sm:min-w-8 items-center justify-center -mr-2 text-muted-foreground">
            <ChevronDown className={cn("h-3.5 w-3.5 transition-transform duration-200 ease-in-out", open && "rotate-180")} />
          </span>
        </div>
      </button>}
    </div>
    {tasks.length > 0 && <div className="calm-stream-constraint max-w-[75ch] mx-auto w-full">
      <div ref={panelRef} hidden={!open} id={id} className="composer-tasks-panel overscroll-contain overflow-y-auto scroll-pb-2 h-[calc(100dvh-120px)] sm:h-auto sm:max-h-[calc(100dvh-160px)] pt-0 pb-1">
        <AccordionTimeline label={t('Conversation tasks')} showTriggerCaret={false} defaultExpandedIds={defaultExpandedIds} className="px-0 pb-2.5 pt-0" items={ordered.map((task, index) => {
          const work = workStatus(task)
          const status = workDisplayStatus(task)
          const progress = work === 'Needs attention' ? task.review_finding || ''
            : ['Completed', 'Answered'].includes(work) ? taskSummaryText(task)
            : taskLiveActivityText(task, '') || taskSummaryText(task)
          const isLatest = index === 0
          return {
            id: task.id,
            indicator: status === 'Completed' ? (
              <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-[color:var(--status-success)]" aria-label={t(status)} />
            ) : taskExecutionActive(task) ? (
              <StatusDot status="working" size="xs" label={t(status)} />
            ) : work === 'Queued' ? (
              <Clock3 className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-label={t(status)} />
            ) : work === 'Ready for review' ? (
              <Eye className={cn("h-3.5 w-3.5 shrink-0", TASK_STATUS_THEMES.review.textColor)} aria-label={t(status)} />
            ) : work === 'Needs attention' ? (
              <AlertCircle className="h-3.5 w-3.5 shrink-0 text-destructive" aria-label={t(status)} />
            ) : (
              <StatusDot status="neutral" size="xs" label={t(status)} />
            ),
            title: isLatest ? (
              <span className="min-w-0 sm:flex sm:items-center sm:gap-3">
                <span className="line-clamp-2 min-w-0 flex-1 font-medium sm:line-clamp-1">{task.title}</span>
                <span className="block shrink-0 text-[11px] font-normal text-muted-foreground">{t(status)}</span>
              </span>
            ) : (
              <span className="line-clamp-2 min-w-0 flex-1 font-medium sm:line-clamp-1">{task.title}</span>
            ),
            pinnedAction: isLatest ? (
              <span className="inline-flex shrink-0 items-center gap-1.5">
                <span className={cn(
                  "composer-tasks-count tabular-nums px-1.5 py-0.5 text-[11px] font-medium rounded-full",
                  TASK_STATUS_THEMES[badgeStatus]?.counterBadgeClass || "bg-muted/80 text-muted-foreground border border-border/30"
                )}>{ordered.length}</span>
                <span
                  role="button"
                  tabIndex={0}
                  aria-label={t('Collapse tasks')}
                  data-testid="composer-tasks-collapse"
                  onClick={e => {
                    e.stopPropagation()
                    setOpen(false)
                    trigger.current?.focus()
                  }}
                  onKeyDown={e => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.stopPropagation()
                      e.preventDefault()
                      setOpen(false)
                      trigger.current?.focus()
                    }
                  }}
                  className="inline-flex min-h-11 min-w-11 sm:min-h-8 sm:min-w-8 items-center justify-center -mr-2 rounded hover:bg-muted/40 text-muted-foreground cursor-pointer focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                >
                  <ChevronDown className="h-3.5 w-3.5 rotate-180 transition-transform duration-200 ease-in-out" />
                </span>
              </span>
            ) : undefined,
            meta: !isLatest ? (
              <span className="block shrink-0 text-[11px] font-normal text-muted-foreground">{t(status)}</span>
            ) : undefined,
            content: <>
              <TaskDuration task={task} className="mb-2 block" />
              {progress && <p className="line-clamp-3 pb-2 text-sm leading-relaxed text-muted-foreground">{progress}</p>}
              <CardActions className="px-0 text-xs">
                <button type="button" className="mr-auto" onClick={() => onDetails(task)}>{t('Details')}</button>
                {workStatus(task) === 'Working' && <TaskTerminalAction task={task} onTerminal={onTerminal} />}
              </CardActions>
            </>
          }
        })} />
        <div className="pt-4 pb-8 flex items-center justify-center border-t border-border/20 mt-4">
          <button
            type="button"
            data-testid="composer-tasks-back"
            aria-label={t('Back to conversation')}
            onClick={() => {
              setOpen(false)
              trigger.current?.focus()
            }}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-full border border-border/50 bg-muted/40 hover:bg-muted text-xs font-medium text-foreground transition-colors min-h-11 sm:min-h-9 shadow-xs cursor-pointer select-none"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            <span>{t('Back to conversation')}</span>
          </button>
        </div>
      </div>
    </div>}
  </section>
}

function TaskTerminalAction({ task, onTerminal }: { task: Task; onTerminal: (task: Task) => void }) {
  const { t } = useLanguage()
  const attempt = task.latest_attempt?.id
  const [available, setAvailable] = useState('')
  useEffect(() => {
    if (!attempt) return
    let live = true
    const check = () => {
      if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
      tasksApi.runtime(task.id).then(runtime => {
        if (live) setAvailable(runtime.live_session?.attempt_id === attempt ? attempt : '')
      }).catch(() => { if (live) setAvailable('') })
    }
    void check()
    const timer = setInterval(check, 5000)
    const onResume = () => void check()
    document.addEventListener('visibilitychange', onResume)
    return () => { live = false; clearInterval(timer); document.removeEventListener('visibilitychange', onResume) }
  }, [task.id, attempt])
  if (!attempt || available !== attempt) return null
  return <button type="button" className="inline-flex items-center gap-1.5" onClick={() => onTerminal(task)}><Terminal className="h-3.5 w-3.5" />{t('Terminal')}</button>
}
