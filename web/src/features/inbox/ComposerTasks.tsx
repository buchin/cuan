import { useEffect, useId, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { CheckCircle2, Clock3, Eye, AlertCircle, ChevronDown, Terminal } from 'lucide-react'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { StatusDot } from '../../components/ui/StatusDot'
import { SlideDrawer } from '../../components/ui/SlideDrawer'
import { CollapsibleChatText } from './CollapsibleChatText'
import { workStatus, workDisplayStatus, taskExecutionActive, taskConversationSummary, TASK_STATUS_THEMES, type TaskStateKey } from '../tasks/task-ui'
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
  const badgeStatus = getComposerTaskBadgeStatus(latestTask)

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
          "hover:bg-muted/30 active:bg-muted/50 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
        )}
        onClick={() => setOpen(value => !value)}>
        {latestTask && (
          <div className="flex items-center gap-2 min-w-0 flex-1">
            <TaskStatusIcon task={latestTask} size="sm" />
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
    {tasks.length > 0 && (
      <SlideDrawer
        isOpen={open}
        onClose={() => {
          setOpen(false)
          trigger.current?.focus()
        }}
        bodyRef={panelRef}
        title={t('Conversation tasks')}
        badge={<span className={cn(
          "composer-tasks-count tabular-nums px-1.5 py-0.5 text-[11px] font-medium rounded-full",
          TASK_STATUS_THEMES[badgeStatus]?.counterBadgeClass || "bg-muted/80 text-muted-foreground border border-border/30"
        )}>{ordered.length}</span>}
        data-testid="composer-tasks-drawer"
        size="md"
      >
        <div id={id} className="composer-tasks-panel">
          <ol className="relative min-w-0 space-y-6 pb-2.5" data-timeline="linear-progress">
            {ordered.map((task, index) => {
              const status = workDisplayStatus(task)
              const progress = taskConversationSummary(task)
              return (
                <li
                  key={task.id}
                  data-timeline-item={task.id}
                  data-work-event={task.id}
                  className="relative min-w-0 pl-7"
                >
                  {index < ordered.length - 1 && (
                    <span
                      data-timeline-connector
                      aria-hidden
                      className="absolute left-[7.5px] top-[22px] -bottom-[24px] w-px bg-muted-foreground/[0.18] dark:bg-muted-foreground/25 pointer-events-none"
                    />
                  )}
                  <span className="absolute left-0 top-0.5 flex h-4 w-4 items-center justify-center bg-card z-1">
                    <TaskStatusIcon task={task} size="md" />
                  </span>
                  <div className="min-w-0 space-y-1.5">
                    <div className="flex items-start justify-between gap-3">
                      <span className="font-medium text-sm text-foreground leading-snug break-words">
                        {task.title}
                      </span>
                      <span className="shrink-0 text-[11px] font-normal text-muted-foreground pt-0.5">
                        {t(status)}
                      </span>
                    </div>
                    <TaskDuration task={task} className="text-xs text-muted-foreground block" />
                    {progress && (
                      <div className="task-timeline-content text-xs sm:text-sm text-muted-foreground break-words pt-0.5">
                        <CollapsibleChatText
                          content={progress}
                          maxLines={6}
                          hideTaskCard
                          markdownClassName="task-timeline-markdown"
                        />
                      </div>
                    )}
                    <CardActions className="px-0 pt-1 text-xs">
                      <button
                        type="button"
                        className="mr-auto"
                        onClick={() => {
                          setOpen(false)
                          onDetails(task)
                        }}
                      >
                        {t('Details')}
                      </button>
                      {workStatus(task) === 'Working' && (
                        <TaskTerminalAction
                          task={task}
                          onTerminal={t => {
                            setOpen(false)
                            onTerminal(t)
                          }}
                        />
                      )}
                    </CardActions>
                  </div>
                </li>
              )
            })}
          </ol>
        </div>
      </SlideDrawer>
    )}
  </section>
}

function getComposerTaskBadgeStatus(task?: Task): TaskStateKey {
  if (!task) return 'neutral'
  const work = workStatus(task)
  const status = workDisplayStatus(task)
  if (status === 'Completed' || work === 'Answered') return 'done'
  if (taskExecutionActive(task)) return 'working'
  if (work === 'Queued') return 'queued'
  if (work === 'Ready for review') return 'review'
  if (work === 'Needs attention') return 'attention'
  return 'neutral'
}

function TaskStatusIcon({ task, size = 'md' }: { task: Task; size?: 'sm' | 'md' }) {
  const { t } = useLanguage()
  const work = workStatus(task)
  const status = workDisplayStatus(task)
  const sizeClass = size === 'sm' ? 'h-3.5 w-3.5' : 'h-4 w-4'
  if (status === 'Completed' || work === 'Answered') {
    return <CheckCircle2 className="h-4 w-4 shrink-0 text-[color:var(--status-success)]" aria-label={t(status)} />
  }
  if (taskExecutionActive(task)) {
    return <StatusDot status="working" size="xs" label={t(status)} />
  }
  if (work === 'Queued') {
    return <Clock3 className={cn(sizeClass, 'shrink-0 text-muted-foreground')} aria-label={t(status)} />
  }
  if (work === 'Ready for review') {
    return <Eye className={cn(sizeClass, 'shrink-0', TASK_STATUS_THEMES.review.textColor)} aria-label={t(status)} />
  }
  if (work === 'Needs attention') {
    return <AlertCircle className={cn(sizeClass, 'shrink-0 text-destructive')} aria-label={t(status)} />
  }
  return <StatusDot status="neutral" size="xs" label={t(status)} />
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
