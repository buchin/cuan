import { useEffect, useId, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { CheckCircle2, Clock3, CheckCheck, AlertCircle, ChevronDown, Terminal } from 'lucide-react'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { StatusDot } from '../../components/ui/StatusDot'
import { AccordionTimeline } from '../../components/ui/AccordionTimeline'
import { workStatus, workDisplayStatus, taskExecutionActive, taskLiveActivityText, taskSummaryText } from '../tasks/task-ui'
import { tasksApi } from '../../api/tasks'
import { CardActions } from './CardActions'
import { TaskDuration } from '../tasks/TaskDuration'
import { cn } from '../../lib/utils'

function running(task: Task) {
  const status = workStatus(task)
  return status === 'Working' || (!['Completed', 'Cancelled'].includes(status) && task.review_state === 'running')
}

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

  const ordered = [...new Map(tasks.map(task => [task.id, task])).values()]
    .sort((a, b) => (Date.parse(a.created_at) || 0) - (Date.parse(b.created_at) || 0) || a.id.localeCompare(b.id))

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
      if (typeof el.scrollTo === 'function') {
        el.scrollTo({ top: el.scrollHeight, behavior })
      } else {
        el.scrollTop = el.scrollHeight
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
  const runningTasks = ordered.filter(running)
  const defaultExpandedIds = runningTasks.length ? runningTasks.map(task => task.id) : ordered.slice(-1).map(task => task.id)

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

  const latestTask = ordered[ordered.length - 1]

  return <section className="composer-tasks border-b border-border/40 bg-muted/20 backdrop-blur-xs px-3 sm:px-6 py-2 transition-colors z-10 shrink-0" aria-label={t('Conversation tasks')} data-testid="task-banner" onKeyDown={event => {
    if (event.key === 'Escape' && open) {
      event.stopPropagation()
      setOpen(false)
      trigger.current?.focus()
    }
  }}>
    <div className="calm-stream-constraint max-w-[75ch] mx-auto w-full flex items-center justify-between gap-3">
      {modes}
      {latestTask && (
        <div className="flex items-center gap-2 min-w-0 flex-1">
          {hasAttention ? (
            <AlertCircle className="h-3.5 w-3.5 shrink-0 text-destructive" aria-label={t('Needs attention')} />
          ) : hasActive ? (
            <StatusDot status="working" size="xs" label={t('In progress')} />
          ) : hasReview ? (
            <CheckCheck className="h-3.5 w-3.5 shrink-0 text-purple-600 dark:text-purple-400" aria-label={t('Ready for review')} />
          ) : isAllDone ? (
            <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-[color:var(--status-success)]" aria-label={t('Completed')} />
          ) : hasQueued ? (
            <Clock3 className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-label={t('Queued')} />
          ) : (
            <StatusDot status="neutral" size="xs" label={t('Status')} />
          )}
          <span className="text-xs font-medium text-foreground truncate" title={latestTask.title}>
            {latestTask.title}
          </span>
        </div>
      )}
      {tasks.length > 0 && <button ref={trigger} type="button" aria-expanded={open} aria-controls={open ? id : undefined}
        data-testid="composer-tasks-badge"
        data-status={badgeStatus}
        className={cn(
          "composer-mode-tab ml-auto inline-flex shrink-0 items-center gap-1.5 text-xs font-medium cursor-pointer transition-colors min-h-11 sm:min-h-8 px-2.5 py-1 rounded-full border",
          badgeStatus === 'working' && "border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-300 hover:bg-blue-500/15",
          badgeStatus === 'review' && "border-purple-500/30 bg-purple-500/10 text-purple-700 dark:text-purple-300 hover:bg-purple-500/15",
          badgeStatus === 'done' && "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 hover:bg-emerald-500/15",
          badgeStatus === 'attention' && "border-destructive/30 bg-destructive/10 text-destructive hover:bg-destructive/15",
          (badgeStatus === 'queued' || badgeStatus === 'neutral') && "border-border/40 bg-muted/20 hover:bg-muted/50 text-muted-foreground"
        )}
        onClick={() => setOpen(value => !value)}>
        <span>{t('Tasks')}</span>
        <span className={cn(
          "composer-tasks-count tabular-nums px-1.5 text-[11px] rounded",
          badgeStatus === 'done' ? "bg-emerald-500/20 text-emerald-800 dark:text-emerald-300" :
          badgeStatus === 'review' ? "bg-purple-500/20 text-purple-800 dark:text-purple-300" :
          badgeStatus === 'working' ? "bg-blue-500/20 text-blue-800 dark:text-blue-300" :
          badgeStatus === 'attention' ? "bg-destructive/20 text-destructive" :
          "bg-muted text-muted-foreground"
        )}>{ordered.length}</span>
        <ChevronDown className={cn("h-3.5 w-3.5 transition-transform duration-200", open && "rotate-180")} />
      </button>}
    </div>
    {tasks.length > 0 && <div className="calm-stream-constraint max-w-[75ch] mx-auto w-full">
      <div ref={panelRef} hidden={!open} id={id} className="composer-tasks-panel overscroll-contain overflow-y-auto rounded-lg bg-muted/30 scroll-pb-2 max-h-[40vh] mt-2">
        <AccordionTimeline label={t('Conversation tasks')} defaultExpandedIds={defaultExpandedIds} className="px-3 pb-2.5 pt-1" items={ordered.map(task => {
          const work = workStatus(task)
          const status = workDisplayStatus(task)
          const progress = work === 'Needs attention' ? task.review_finding || ''
            : ['Completed', 'Answered'].includes(work) ? taskSummaryText(task)
            : taskLiveActivityText(task, '') || taskSummaryText(task)
          return {id: task.id,
            indicator: status === 'Completed' ? (
              <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-[color:var(--status-success)]" aria-label={t(status)} />
            ) : taskExecutionActive(task) ? (
              <StatusDot status="working" size="xs" label={t(status)} />
            ) : work === 'Queued' ? (
              <Clock3 className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-label={t(status)} />
            ) : work === 'Ready for review' ? (
              <CheckCheck className="h-3.5 w-3.5 shrink-0 text-purple-600 dark:text-purple-400" aria-label={t(status)} />
            ) : work === 'Needs attention' ? (
              <AlertCircle className="h-3.5 w-3.5 shrink-0 text-destructive" aria-label={t(status)} />
            ) : (
              <StatusDot status="neutral" size="xs" label={t(status)} />
            ),
            title: <span className="min-w-0 sm:flex sm:items-center sm:gap-3">
                <span className="line-clamp-2 min-w-0 flex-1 font-medium sm:line-clamp-1">{task.title}</span>
                <span className="block shrink-0 text-[11px] font-normal text-muted-foreground">{t(status)}</span>
              </span>,
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
    const check = () => tasksApi.runtime(task.id).then(runtime => {
      if (live) setAvailable(runtime.live_session?.attempt_id === attempt ? attempt : '')
    }).catch(() => { if (live) setAvailable('') })
    void check()
    const timer = setInterval(check, 5000)
    return () => { live = false; clearInterval(timer) }
  }, [task.id, attempt])
  if (!attempt || available !== attempt) return null
  return <button type="button" className="inline-flex items-center gap-1.5" onClick={() => onTerminal(task)}><Terminal className="h-3.5 w-3.5" />{t('Live terminal')}</button>
}
