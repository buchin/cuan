import { useEffect, useId, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { CheckCircle2, Clock3, CheckCheck, AlertCircle, Terminal } from 'lucide-react'
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
  const [open, setOpen] = useState(false)
  if (!tasks.length && !modes) return null
  const ordered = [...new Map(tasks.map(task => [task.id, task])).values()]
    .sort((a, b) => (Date.parse(a.created_at) || 0) - (Date.parse(b.created_at) || 0) || a.id.localeCompare(b.id))
  const runningTasks = ordered.filter(running)
  const defaultExpandedIds = runningTasks.length ? runningTasks.map(task => task.id) : ordered.slice(-1).map(task => task.id)
  return <section className="composer-tasks bg-transparent" aria-label={t('Conversation tasks')} onKeyDown={event => {
    if (event.key === 'Escape' && open) {
      event.stopPropagation()
      setOpen(false)
      trigger.current?.focus()
    }
  }}>
    <div className={cn(
      "flex flex-wrap items-end justify-between gap-x-2 bg-transparent",
      !modes && !open && "justify-end pb-1"
    )}>
    {modes}
    {tasks.length > 0 && <button ref={trigger} type="button" aria-expanded={open} aria-controls={open ? id : undefined}
      className={cn(
        "composer-mode-tab ml-auto inline-flex shrink-0 items-center gap-1.5 text-xs font-medium bg-transparent hover:text-foreground cursor-pointer transition-colors",
        modes || open ? "min-h-11 px-2" : "min-h-7 px-2.5 py-1 rounded-full border border-border/40 bg-muted/20 hover:bg-muted/50 text-muted-foreground"
      )}
      onClick={() => setOpen(value => !value)}>
      <span>{t('Tasks')}</span>
      <span className="composer-tasks-count tabular-nums bg-muted px-1.5 text-[11px]">{ordered.length}</span>
      {ordered.some(taskExecutionActive) && <StatusDot status="working" size="xs" label={t('In progress')} />}
    </button>}
    </div>
    {tasks.length > 0 && <div hidden={!open} id={id} className="composer-tasks-panel overscroll-contain overflow-y-auto rounded-lg bg-muted/30">
      <AccordionTimeline label={t('Conversation tasks')} defaultExpandedIds={defaultExpandedIds} className="px-3" items={ordered.map(task => {
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
          </>}
      })} />
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
