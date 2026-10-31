import { useState, useEffect, useMemo } from 'react'
import {
  Clock,
  Loader2,
  Bot,
  ChevronRight,
} from 'lucide-react'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import { taskRef, agentDisplayName } from '../tasks/task-ui'
import { isTaskActive, formatDuration } from './task-progress'

export { isTaskActive, formatDuration }

export interface TaskProgressBannerProps {
  tasks: Task[]
  onViewTasks?: () => void
  onOpenTask?: (task: Task) => void
  className?: string
}

export function TaskProgressBanner({
  tasks,
  onViewTasks,
  onOpenTask,
  className,
}: TaskProgressBannerProps) {
  const { t } = useLanguage()

  const activeTasks = useMemo(() => {
    return (tasks || []).filter(isTaskActive)
  }, [tasks])

  const primaryTask = activeTasks[0]

  // Calculate live elapsed seconds based on attempt started_at or task updated_at / created_at
  const [elapsedSeconds, setElapsedSeconds] = useState<number>(0)

  useEffect(() => {
    if (!primaryTask) {
      setElapsedSeconds(0)
      return
    }

    const startTimestamp =
      primaryTask.latest_attempt?.started_at ||
      primaryTask.latest_attempt?.created_at ||
      primaryTask.updated_at ||
      primaryTask.created_at

    const parseTime = Date.parse(startTimestamp || '')
    const startTime = Number.isFinite(parseTime) ? parseTime : Date.now()

    const updateTimer = () => {
      const diff = Math.max(0, Math.floor((Date.now() - startTime) / 1000))
      setElapsedSeconds(diff)
    }

    updateTimer()
    const timer = setInterval(updateTimer, 1000)

    return () => clearInterval(timer)
  }, [
    primaryTask?.id,
    primaryTask?.latest_attempt?.started_at,
    primaryTask?.latest_attempt?.created_at,
    primaryTask?.updated_at,
    primaryTask?.created_at,
  ])

  if (!primaryTask) return null

  const attemptStatus = (primaryTask.latest_attempt?.status || '').toLowerCase().trim()
  const taskStatus = (primaryTask.status || '').toLowerCase().trim()
  const isQueued =
    taskStatus === 'queued' ||
    attemptStatus === 'queued' ||
    primaryTask.launch?.state === 'queued' ||
    primaryTask.launch?.state === 'claimed'

  const agentName = agentDisplayName(
    primaryTask.latest_attempt?.agent_name || primaryTask.assignee || 'cuan-runner'
  )

  const stageText = primaryTask.latest_progress || (isQueued ? t('Menunggu antrean worker…') : t('Sedang memproses instruksi…'))

  const handleAction = () => {
    if (onOpenTask) {
      onOpenTask(primaryTask)
    } else if (onViewTasks) {
      onViewTasks()
    }
  }

  return (
    <div
      data-testid="floating-task-progress-banner"
      className={cn(
        'pointer-events-none absolute inset-x-0 top-3 z-30 px-3 sm:px-6 transition-all duration-300 ease-out animate-in fade-in slide-in-from-top-2',
        className
      )}
      aria-label={t('Floating Live Progress')}
    >
      <div className="mx-auto flex max-w-xl justify-center">
        <div
          role="status"
          aria-live="polite"
          className={cn(
            'pointer-events-auto flex w-full max-w-lg items-center justify-between gap-3 rounded-full border px-3.5 py-2 shadow-lg backdrop-blur-md transition-all sm:px-4',
            isQueued
              ? 'border-amber-500/30 bg-card/95 text-card-foreground shadow-amber-500/5'
              : 'border-emerald-500/35 bg-card/95 text-card-foreground shadow-emerald-500/10'
          )}
        >
          {/* Left section: Pulse badge & Status */}
          <div className="flex min-w-0 items-center gap-2.5">
            <span className="relative flex h-3 w-3 shrink-0 items-center justify-center">
              <span
                className={cn(
                  'absolute inline-flex h-full w-full rounded-full opacity-75',
                  isQueued ? 'bg-amber-400 animate-ping' : 'bg-emerald-400 animate-ping'
                )}
              />
              <span
                className={cn(
                  'relative inline-flex h-2.5 w-2.5 rounded-full',
                  isQueued ? 'bg-amber-500' : 'bg-emerald-500'
                )}
              />
            </span>

            <div className="flex min-w-0 flex-col">
              <div className="flex items-center gap-1.5 flex-wrap">
                <span
                  data-testid="live-progress-status-badge"
                  className={cn(
                    'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase border',
                    isQueued
                      ? 'bg-amber-500/15 text-amber-700 dark:text-amber-300 border-amber-500/30'
                      : 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30'
                  )}
                >
                  {isQueued ? (
                    <Clock className="h-3 w-3" />
                  ) : (
                    <Loader2 className="h-3 w-3 animate-spin" />
                  )}
                  <span>{isQueued ? t('Dalam Antrean') : t('Sedang Dikerjakan')}</span>
                </span>

                <span
                  data-testid="live-progress-title"
                  className="truncate text-xs font-semibold text-foreground max-w-[160px] sm:max-w-[210px]"
                  title={primaryTask.title}
                >
                  {taskRef(primaryTask.id)} · {primaryTask.title}
                </span>

                {activeTasks.length > 1 && (
                  <span
                    data-testid="live-progress-multi-badge"
                    className="rounded-full bg-muted px-1.5 py-0.2 text-[10px] font-medium text-muted-foreground border border-border/60"
                  >
                    +{activeTasks.length - 1} {t('tugas lainnya')}
                  </span>
                )}
              </div>

              <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
                <span data-testid="live-progress-agent" className="flex items-center gap-1 font-medium">
                  <Bot className="h-3 w-3 text-primary shrink-0" />
                  <span className="truncate max-w-[90px]">{agentName}</span>
                </span>
                <span>·</span>
                <span
                  data-testid="live-progress-timer"
                  className="font-mono text-xs font-semibold text-foreground"
                >
                  {formatDuration(elapsedSeconds)}
                </span>
                {stageText && (
                  <>
                    <span className="hidden sm:inline">·</span>
                    <span
                      data-testid="live-progress-stage"
                      className="hidden sm:inline truncate max-w-[140px] text-muted-foreground/80"
                      title={stageText}
                    >
                      {stageText}
                    </span>
                  </>
                )}
              </div>
            </div>
          </div>

          {/* Right section: Action button */}
          <div className="shrink-0">
            <button
              type="button"
              data-testid="view-task-details-btn"
              onClick={handleAction}
              className="inline-flex min-h-9 min-w-9 sm:min-w-0 items-center justify-center gap-1 rounded-full bg-primary/10 hover:bg-primary/20 text-primary border border-primary/20 px-2.5 sm:px-3 py-1.5 text-xs font-semibold transition-colors cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
              title={t('Lihat Detail Tugas')}
              aria-label={t('Lihat Detail Tugas')}
            >
              <span className="hidden sm:inline">{t('Lihat Detail')}</span>
              <ChevronRight className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
