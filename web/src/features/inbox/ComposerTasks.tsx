import { useEffect, useId, useRef, useState } from 'react'
import { ChevronDown, ListTodo, Terminal } from 'lucide-react'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { StatusDot } from '../../components/ui/StatusDot'
import { workStatus, taskLiveActivityText, taskSummaryText } from '../tasks/task-ui'
import { tasksApi } from '../../api/tasks'
import { CardActions } from './CardActions'

function running(task: Task) {
  const status = workStatus(task)
  return status === 'Working' || (!['Completed', 'Cancelled'].includes(status) && task.review_state === 'running')
}

// Key this component by conversation so disclosure choices never leak between rooms.
export function ComposerTasks({ tasks, onDetails, onTerminal }: {
  tasks: Task[]
  onDetails: (task: Task) => void
  onTerminal: (task: Task) => void
}) {
  const { t } = useLanguage()
  const id = useId()
  const trigger = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [choices, setChoices] = useState<Record<string, boolean>>({})
  if (!tasks.length) return null
  const ordered = [...new Map(tasks.map(task => [task.id, task])).values()]
    .sort((a, b) => Number(running(b)) - Number(running(a)))
  const active = ordered.some(running)
  return <section className="composer-tasks" aria-label={t('Conversation tasks')} onKeyDown={event => {
    if (event.key === 'Escape' && open) {
      event.stopPropagation()
      setOpen(false)
      trigger.current?.focus()
    }
  }}>
    <button ref={trigger} type="button" aria-expanded={open} aria-controls={open ? id : undefined}
      className="inline-flex min-h-11 items-center gap-2 px-2 text-xs font-medium text-muted-foreground hover:text-foreground"
      onClick={() => setOpen(value => !value)}>
      <ListTodo className="h-3.5 w-3.5" aria-hidden="true" />
      <span>{t('Tasks')}</span>
      <span className="composer-tasks-count tabular-nums bg-muted px-1.5 text-[11px]">{ordered.length}</span>
      {active && <StatusDot status="working" size="xs" label={t('Working')} />}
      <ChevronDown className={`h-3.5 w-3.5 transition-transform ${open ? 'rotate-180' : ''}`} aria-hidden="true" />
    </button>
    {open && <div id={id} className="composer-tasks-panel overscroll-contain overflow-y-auto rounded-lg bg-muted/30">
      {ordered.map(task => {
        const expanded = choices[task.id] ?? running(task)
        const work = workStatus(task)
        const status = ['Completed', 'Cancelled'].includes(work) ? work : task.review_state === 'running' ? 'Checking result' : task.review_state === 'queued' ? 'Queued' : work
        const bodyID = `${id}-${task.id}`
        const progress = work === 'Needs attention' ? task.review_finding || '' : taskLiveActivityText(task, '') || taskSummaryText(task)
        return <article key={task.id} className="border-b border-border/40 last:border-0">
          <button type="button" aria-expanded={expanded} aria-controls={expanded ? bodyID : undefined}
            onClick={() => setChoices(value => ({ ...value, [task.id]: !expanded }))}
            className="flex min-h-11 w-full items-center gap-2 px-3 py-2 text-left text-sm">
            <StatusDot status={running(task) ? 'working' : status === 'Completed' ? 'healthy' : status === 'Needs attention' ? 'warning' : 'neutral'} size="xs" label={t(status)} />
            <span className="min-w-0 flex-1 sm:flex sm:items-center sm:gap-3">
              <span className={`min-w-0 flex-1 font-medium ${expanded ? 'line-clamp-2 sm:line-clamp-1' : 'block truncate'}`}>{task.title}</span>
              <span className="block shrink-0 text-[11px] font-normal text-muted-foreground">{t(status)}</span>
            </span>
            <ChevronDown className={`h-3.5 w-3.5 shrink-0 text-muted-foreground ${expanded ? 'rotate-180' : ''}`} aria-hidden="true" />
          </button>
          {expanded && <div id={bodyID}>
            {progress && <p className="line-clamp-3 px-3 pb-2 text-sm leading-relaxed text-muted-foreground">{progress}</p>}
            <CardActions className="px-3 text-xs">
              <button type="button" className="mr-auto" onClick={() => onDetails(task)}>{t('Details')}</button>
              {workStatus(task) === 'Working' && <TaskTerminalAction task={task} onTerminal={onTerminal} />}
            </CardActions>
          </div>}
        </article>
      })}
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
