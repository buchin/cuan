import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Folder,
  Terminal,
  Copy,
  Check,
  RefreshCw,
  FileText,
  AlertCircle,
  Clock,
  Send,
  Sparkles,
} from 'lucide-react'
import type { Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import {
  workStatus,
  workStatusBadgeClass,
  taskTargetUrl,
  taskDetailUrl,
  taskTerminalUrl,
  taskSummaryText,
  taskLiveActivityText,
  taskRef,
  relativeTime,
  agentDisplayName,
} from '../tasks/task-ui'
import { isTaskActive, formatDuration } from './task-progress'
import { TaskCompletionCard } from './TaskCompletionCard'
import { parseTaskReviewEnvelope } from './task-envelope'

export interface TaskCardProps {
  taskId: string
  initialTask?: Task
  onNavigate?: () => void
  onClick?: (task: Task, tab?: 'details' | 'terminal') => void
  className?: string
  returnTo?: string
  onContinueChat?: (suggestedText: string) => void
  onUseDraft?: (draftText: string) => void
  onOpenTerminal?: (task: Task) => void
  onRetry?: (task: Task) => Promise<void> | void
  disableCompletionCard?: boolean
  isCustomerRoom?: boolean
}

export function TaskCard({
  taskId,
  initialTask,
  onNavigate,
  onClick,
  className,
  returnTo,
  onContinueChat,
  onUseDraft,
  onOpenTerminal,
  onRetry,
  disableCompletionCard = false,
  isCustomerRoom = false,
}: TaskCardProps) {
  const { t } = useLanguage()
  const navigate = useNavigate()

  const [currentTask, setCurrentTask] = useState<Task | null>(initialTask || null)
  const [loading, setLoading] = useState(!initialTask)
  const [error, setError] = useState(false)
  const [copied, setCopied] = useState(false)
  const [retrying, setRetrying] = useState(false)
  const [elapsedSeconds, setElapsedSeconds] = useState<number>(0)

  useEffect(() => {
    if (initialTask) {
      setCurrentTask(initialTask)
      setLoading(false)
      return
    }
    let active = true
    tasksApi
      .get(taskId)
      .then((data) => {
        if (active && data) {
          setCurrentTask(data)
          setError(false)
        }
      })
      .catch(() => {
        if (active) setError(true)
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [taskId, initialTask])

  // Autonomous polling for active tasks in stream (every 3.5s)
  useEffect(() => {
    if (!currentTask || !isTaskActive(currentTask)) return
    let active = true
    const interval = setInterval(async () => {
      try {
        const updated = await tasksApi.get(taskId)
        if (active && updated) {
          setCurrentTask(updated)
        }
      } catch {
        // ignore polling error
      }
    }, 3500)
    return () => {
      active = false
      clearInterval(interval)
    }
  }, [
    taskId,
    currentTask?.id,
    currentTask?.status,
    currentTask?.latest_attempt?.status,
    currentTask?.launch?.state,
  ])

  // Live timer interval for queued/running tasks
  useEffect(() => {
    if (!currentTask) {
      setElapsedSeconds(0)
      return
    }

    const taskStatus = (currentTask.status || '').toLowerCase().trim()
    const attemptStatus = (currentTask.latest_attempt?.status || '').toLowerCase().trim()
    const launchState = (currentTask.launch?.state || '').toLowerCase().trim()
    const isRunningOrQueued =
      taskStatus === 'doing' ||
      taskStatus === 'queued' ||
      attemptStatus === 'running' ||
      attemptStatus === 'queued' ||
      launchState === 'started' ||
      launchState === 'claimed' ||
      launchState === 'queued' ||
      isTaskActive(currentTask)

    if (!isRunningOrQueued) {
      setElapsedSeconds(0)
      return
    }

    const startTimestamp =
      currentTask.latest_attempt?.started_at ||
      currentTask.latest_attempt?.created_at ||
      currentTask.updated_at ||
      currentTask.created_at

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
    currentTask?.id,
    currentTask?.status,
    currentTask?.latest_attempt?.status,
    currentTask?.latest_attempt?.started_at,
    currentTask?.latest_attempt?.created_at,
    currentTask?.updated_at,
    currentTask?.created_at,
  ])

  const handleCardClick = (_e: React.MouseEvent) => {
    if (onClick && currentTask) {
      onClick(currentTask, 'details')
    } else {
      const url = currentTask ? taskTargetUrl(currentTask) : `/tasks/${encodeURIComponent(taskId)}`
      navigate(url, returnTo ? { state: { taskReturnTo: returnTo } } : undefined)
    }
    onNavigate?.()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      if (onClick && currentTask) {
        onClick(currentTask, 'details')
      } else {
        const url = currentTask ? taskTargetUrl(currentTask) : `/tasks/${encodeURIComponent(taskId)}`
        navigate(url, returnTo ? { state: { taskReturnTo: returnTo } } : undefined)
      }
      onNavigate?.()
    }
  }

  const handleCopyLink = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    try {
      const origin = typeof window !== 'undefined' ? window.location.origin : ''
      await navigator.clipboard.writeText(`${origin}/tasks/${taskId}`)
    } catch {
      // ignore
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleActionClick = (e?: React.MouseEvent) => {
    if (e) {
      e.preventDefault()
      e.stopPropagation()
    }
    if (!currentTask) {
      navigate(`/tasks/${encodeURIComponent(taskId)}`)
      onNavigate?.()
      return
    }
    const status = workStatus(currentTask)
    let url = taskTargetUrl(currentTask)
    if (status === 'Ready for review' || status === 'Completed' || status === 'Answered') {
      url = taskDetailUrl(currentTask)
    } else if (status === 'Working' || status === 'Queued' || (status as string) === 'Waiting to start') {
      url = taskTerminalUrl(currentTask)
    }
    if (onOpenTerminal && (status === 'Working' || status === 'Queued')) {
      onOpenTerminal(currentTask)
    } else if (onClick) {
      onClick(currentTask, 'details')
    } else {
      navigate(url, returnTo ? { state: { taskReturnTo: returnTo } } : undefined)
    }
    onNavigate?.()
  }

  const handleOpenTerminal = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!currentTask) return
    if (onOpenTerminal) {
      onOpenTerminal(currentTask)
    } else if (onClick) {
      onClick(currentTask, 'terminal')
    } else {
      navigate(taskTerminalUrl(currentTask), returnTo ? { state: { taskReturnTo: returnTo } } : undefined)
    }
    onNavigate?.()
  }

  const handleRetry = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (retrying || !currentTask) return
    setRetrying(true)
    try {
      if (onRetry) {
        await onRetry(currentTask)
      } else {
        const agentId =
          currentTask.latest_attempt?.agent_name ||
          currentTask.assignee_id ||
          currentTask.assignee ||
          'agy'
        await tasksApi.runAgent(currentTask.id, {
          request_id: crypto.randomUUID(),
          agent_id: agentId,
        })
        const updated = await tasksApi.get(currentTask.id)
        if (updated) setCurrentTask(updated)
      }
      onNavigate?.()
    } catch {
      // ignore
    } finally {
      setRetrying(false)
    }
  }

  const handleUseDraft = (e: React.MouseEvent, draftText: string) => {
    e.preventDefault()
    e.stopPropagation()
    if (!draftText) return
    if (onUseDraft) {
      onUseDraft(draftText)
    } else if (onContinueChat) {
      onContinueChat(draftText)
    }
  }

  const handleViewDetails = (task: Task, tab: 'details' | 'terminal' = 'details') => {
    if (onClick) {
      onClick(task, tab)
    } else {
      const url = taskDetailUrl(task)
      navigate(url, returnTo ? { state: { taskReturnTo: returnTo } } : undefined)
    }
    onNavigate?.()
  }

  if (loading) {
    return (
      <div
        data-card="task-card"
        className={cn(
          'w-full max-w-full rounded-xl border border-border/40 bg-card/40 p-3.5 sm:p-4 text-card-foreground animate-pulse min-h-[44px]',
          className
        )}
      >
        <div className="flex items-center gap-2 mb-2">
          <div className="h-4 w-20 bg-muted rounded-full" />
          <div className="h-4 w-24 bg-muted rounded" />
        </div>
        <div className="h-5 w-3/4 bg-muted rounded mb-3" />
        <div className="h-4 w-1/2 bg-muted rounded" />
      </div>
    )
  }

  if (error || !currentTask) {
    return (
      <div
        data-card="task-card"
        onClick={handleCardClick}
        onKeyDown={handleKeyDown}
        role="button"
        tabIndex={0}
        className={cn(
          'w-full max-w-full rounded-xl border border-border/40 bg-card/40 p-3.5 sm:p-4 text-card-foreground transition-all hover:border-border/70 cursor-pointer min-h-[44px]',
          className
        )}
      >
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
            <span>{taskRef(taskId)}</span>
          </div>
          <button
            type="button"
            onClick={handleCopyLink}
            aria-label={t('Copy link')}
            className="min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors cursor-pointer"
          >
            {copied ? <Check className="h-3.5 w-3.5 text-emerald-600" /> : <Copy className="h-3.5 w-3.5" />}
            <span>{copied ? t('Copied') : t('Copy link')}</span>
          </button>
        </div>
        <p className="mt-1 text-sm font-medium text-foreground">{t('Task')} {taskRef(taskId)}</p>
      </div>
    )
  }

  const rawStatus = (currentTask.status || '').toLowerCase().trim()
  const status = workStatus(currentTask)
  const isDone = rawStatus === 'done' || status === 'Completed'
  const isReview = rawStatus === 'review' || status === 'Ready for review' || status === 'Answered'

  // Seamless transition to TaskCompletionCard for completed or review tasks
  if ((isDone || isReview) && !disableCompletionCard) {
    const convId = returnTo?.includes('/inbox/')
      ? returnTo.split('/inbox/')[1]?.split(/[?#]/)[0]
      : undefined

    return (
      <div
        data-card="task-card"
        className={cn(
          'w-full max-w-full transition-all duration-300 ease-out animate-in fade-in',
          className
        )}
      >
        <TaskCompletionCard
          task={currentTask}
          conversationId={convId}
          onContinueChat={onContinueChat}
          onUseDraft={onUseDraft}
          onViewDetails={handleViewDetails}
          onOpenTerminal={onOpenTerminal}
          onRetry={onRetry}
          isCustomerRoom={isCustomerRoom}
        />
      </div>
    )
  }

  const isWorking = status === 'Working'
  const isQueued =
    status === 'Queued' ||
    (status as string) === 'Waiting to start' ||
    rawStatus === 'queued'
  const isNeedsAttention = status === 'Needs attention'
  const statusBadge = workStatusBadgeClass(status)
  const agent =
    currentTask.latest_attempt?.agent_name ||
    currentTask.assignee_id ||
    currentTask.assignee

  const envelopeResult = parseTaskReviewEnvelope(currentTask)
  const summary = envelopeResult.summary || taskSummaryText(currentTask)
  const liveActivity = taskLiveActivityText(currentTask, t('Runner aktif...'))
  const queuedActivity =
    currentTask.latest_progress || t('Menunggu antrean worker…')

  return (
    <div
      data-card="task-card"
      onClick={handleCardClick}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
      className={cn(
        'group relative w-full max-w-full min-w-0 rounded-xl border border-border/40 bg-card/40 p-3.5 sm:p-4 text-card-foreground transition-all hover:border-border/70 cursor-pointer select-none space-y-3 min-h-[44px]',
        className
      )}
    >
      {/* Header: Status, Live Timer, Project, TaskRef, and Timestamp */}
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/40 pb-2.5">
        <div className="flex flex-wrap items-center gap-2 min-w-0">
          <span
            className={cn(
              'inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold transition-colors',
              statusBadge
            )}
          >
            {isWorking ? (
              <span className="relative flex h-2 w-2 shrink-0 items-center justify-center">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-500 opacity-75" />
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-blue-600 dark:bg-blue-400" />
              </span>
            ) : isQueued ? (
              <span className="relative flex h-2 w-2 shrink-0 items-center justify-center">
                <span className="animate-pulse absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-amber-500 dark:bg-amber-400" />
              </span>
            ) : isNeedsAttention ? (
              <AlertCircle className="h-3 w-3 shrink-0 text-destructive" />
            ) : (
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-muted-foreground/60 shrink-0" />
            )}
            <span>{t(status)}</span>
          </span>

          {/* Running/Queued Live Timer with duration */}
          {(isWorking || isQueued) && (
            <span
              data-testid="task-card-live-timer"
              className="inline-flex items-center gap-1 text-[11px] font-mono font-medium text-muted-foreground bg-muted/60 px-2 py-0.5 rounded-md"
              title={t('Durasi pengerjaan')}
            >
              <Clock className="h-3 w-3 shrink-0 text-primary animate-pulse" />
              <span>{formatDuration(elapsedSeconds)}</span>
            </span>
          )}

          {currentTask.project_name && (
            <span className="inline-flex items-center gap-1 text-xs text-muted-foreground font-medium truncate max-w-[150px]">
              <Folder className="h-3.5 w-3.5 shrink-0" />
              <span className="truncate">{currentTask.project_name}</span>
            </span>
          )}

          <span className="font-mono text-xs text-muted-foreground">
            {taskRef(currentTask.id)}
          </span>
        </div>
      </div>

      {/* Title & Description */}
      <div className="space-y-1">
        <h4 className="text-sm font-semibold text-foreground group-hover:text-primary transition-colors break-words leading-snug line-clamp-2">
          {currentTask.title}
        </h4>
        {isWorking ? (
          <p className="text-xs text-blue-600 dark:text-blue-400 font-medium line-clamp-1 break-words">
            {liveActivity}
          </p>
        ) : isQueued ? (
          <p className="text-xs text-amber-600 dark:text-amber-400 font-medium line-clamp-1 break-words">
            {queuedActivity}
          </p>
        ) : summary ? (
          <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed break-words">
            {summary}
          </p>
        ) : null}
      </div>

      {/* Visitor Reply Draft Section if customer room and draft exists */}
      {isCustomerRoom && envelopeResult.draft && (
        <div
          data-testid="task-card-draft-section"
          onClick={(e) => e.stopPropagation()}
          className="border-t border-border/30 pt-2.5 space-y-2 cursor-default"
        >
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1.5 font-semibold text-xs text-primary">
              <Sparkles className="h-3.5 w-3.5 text-primary shrink-0" />
              <span>{t('Draf Balasan Pengunjung')}</span>
            </div>
            <button
              type="button"
              onClick={(e) => handleUseDraft(e, envelopeResult.draft)}
              data-testid="task-card-use-draft-btn"
              className="min-h-7 px-2.5 py-1 text-xs font-medium rounded-lg bg-primary/10 hover:bg-primary/20 text-primary border border-primary/20 inline-flex items-center gap-1.5 transition-colors cursor-pointer shadow-none"
              title={t('Gunakan Draf di Input')}
            >
              <Send className="h-3 w-3 shrink-0" />
              <span>{t('Gunakan Draf di Input')}</span>
            </button>
          </div>
          <p className="text-xs text-foreground/90 italic border-l-2 border-primary/40 pl-3 py-1 whitespace-pre-wrap leading-relaxed break-words">
            "{envelopeResult.draft}"
          </p>
        </div>
      )}

      {/* Secondary Technical Details link: parks details to SlideDrawer */}
      <div className="pt-2 flex flex-wrap items-center justify-between gap-2 border-t border-border/30 text-xs">
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            handleViewDetails(currentTask, 'details')
          }}
          data-testid="task-card-diagnostic-link"
          className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors group cursor-pointer"
        >
          <span>{t('Detail Diagnostik & Status Deploy →')}</span>
        </button>

        <time
          dateTime={currentTask.updated_at}
          title={new Date(currentTask.updated_at).toLocaleString()}
          className="text-[11px] text-muted-foreground whitespace-nowrap shrink-0"
        >
          {t(relativeTime(currentTask.updated_at))}
        </time>
      </div>

      {/* Assignee snippet */}
      {agent && (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
          <span className="text-muted-foreground/70">{t('Assignee')}:</span>
          <span className="font-medium text-foreground">{agentDisplayName(agent)}</span>
        </div>
      )}

      {/* Footer: Quick Action Buttons */}
      <div className="flex flex-wrap items-center justify-between gap-2 pt-2 border-t border-border/40">
        <button
          type="button"
          onClick={handleCopyLink}
          aria-label={t('Copy link')}
          className="min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors cursor-pointer"
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
          ) : (
            <Copy className="h-3.5 w-3.5 shrink-0" />
          )}
          <span>{copied ? t('Copied') : t('Copy link')}</span>
        </button>

        <div className="flex items-center gap-2">
          {isNeedsAttention && (
            <button
              type="button"
              disabled={retrying}
              onClick={handleRetry}
              data-testid="task-card-retry-btn"
              aria-label={t('Retry')}
              className="min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 text-destructive hover:bg-destructive/10 transition-colors cursor-pointer"
            >
              <RefreshCw className={cn('h-3.5 w-3.5 shrink-0', retrying && 'animate-spin')} />
              <span>{retrying ? t('Retrying...') : t('Retry')}</span>
            </button>
          )}

          <button
            type="button"
            onClick={isWorking ? handleOpenTerminal : handleActionClick}
            data-testid="task-card-terminal-btn"
            aria-label={`${t('Open task')}: ${currentTask.title}`}
            className="min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors cursor-pointer"
          >
            {isWorking ? (
              <>
                <Terminal className="h-3.5 w-3.5 shrink-0 text-emerald-500" />
                <span>{t('Live Terminal')}</span>
              </>
            ) : isQueued ? (
              <>
                <FileText className="h-3.5 w-3.5 shrink-0" />
                <span>{t('Open task')}</span>
              </>
            ) : (
              <>
                <FileText className="h-3.5 w-3.5 shrink-0" />
                <span>{t('Open task')}</span>
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  )
}
