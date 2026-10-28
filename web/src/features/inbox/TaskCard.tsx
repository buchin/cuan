import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Folder,
  Terminal,
  CheckCircle2,
  Copy,
  Check,
  RefreshCw,
  FileText,
  AlertCircle,
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

export interface TaskCardProps {
  taskId: string
  initialTask?: Task
  onNavigate?: () => void
  onClick?: (task: Task) => void
  className?: string
  returnTo?: string
}

export function TaskCard({
  taskId,
  initialTask,
  onNavigate,
  onClick,
  className,
  returnTo,
}: TaskCardProps) {
  const { t } = useLanguage()
  const navigate = useNavigate()

  const [currentTask, setCurrentTask] = useState<Task | null>(initialTask || null)
  const [loading, setLoading] = useState(!initialTask)
  const [error, setError] = useState(false)
  const [copied, setCopied] = useState(false)
  const [retrying, setRetrying] = useState(false)

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

  const handleCardClick = (_e: React.MouseEvent) => {
    if (onClick && currentTask) {
      onClick(currentTask)
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
        onClick(currentTask)
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

  const handleActionClick = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
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
    navigate(url, returnTo ? { state: { taskReturnTo: returnTo } } : undefined)
    onNavigate?.()
  }

  const handleRetry = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (retrying || !currentTask) return
    setRetrying(true)
    try {
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
      onNavigate?.()
    } catch {
      // ignore
    } finally {
      setRetrying(false)
    }
  }

  if (loading) {
    return (
      <div
        data-card="task-card"
        className={cn(
          'w-full max-w-full rounded-xl border border-border/80 bg-card p-3.5 sm:p-4 text-card-foreground shadow-xs animate-pulse min-h-[44px]',
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
          'w-full max-w-full rounded-xl border border-border/80 bg-card p-3.5 sm:p-4 text-card-foreground shadow-xs transition-all hover:border-primary/50 cursor-pointer min-h-[44px]',
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
            className="action-secondary min-h-[44px] min-w-[44px] px-3 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 cursor-pointer"
          >
            {copied ? <Check className="h-3.5 w-3.5 text-emerald-600" /> : <Copy className="h-3.5 w-3.5" />}
            <span>{copied ? t('Copied') : t('Copy link')}</span>
          </button>
        </div>
        <p className="mt-1 text-sm font-medium text-foreground">{t('Task')} {taskRef(taskId)}</p>
      </div>
    )
  }

  const status = workStatus(currentTask)
  const isWorking = status === 'Working'
  const isQueued = status === 'Queued' || (status as string) === 'Waiting to start' || (currentTask.status as string) === 'queued'
  const isNeedsAttention = status === 'Needs attention'
  const statusBadge = workStatusBadgeClass(status)
  const agent = currentTask.latest_attempt?.agent_name || currentTask.assignee_id || currentTask.assignee
  const summary = taskSummaryText(currentTask)
  const liveActivity = taskLiveActivityText(currentTask, t('Runner aktif...'))

  return (
    <div
      data-card="task-card"
      onClick={handleCardClick}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
      className={cn(
        'group relative w-full max-w-full min-w-0 rounded-xl border border-border/80 bg-card p-3.5 sm:p-4 text-card-foreground shadow-xs transition-all hover:border-primary/50 hover:shadow-md cursor-pointer select-none space-y-2.5 min-h-[44px]',
        className
      )}
    >
      {/* Header: Status, Project, TaskRef, and Timestamp */}
      <div className="flex flex-wrap items-center justify-between gap-2">
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
            ) : status === 'Ready for review' ? (
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-purple-600 dark:bg-purple-400 shrink-0" />
            ) : status === 'Completed' || status === 'Answered' ? (
              <CheckCircle2 className="h-3 w-3 shrink-0 text-emerald-600 dark:text-emerald-400" />
            ) : status === 'Needs attention' ? (
              <AlertCircle className="h-3 w-3 shrink-0 text-destructive" />
            ) : (
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-muted-foreground/60 shrink-0" />
            )}
            <span>{t(status)}</span>
          </span>

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

        <time
          dateTime={currentTask.updated_at}
          title={new Date(currentTask.updated_at).toLocaleString()}
          className="text-xs text-muted-foreground whitespace-nowrap shrink-0"
        >
          {t(relativeTime(currentTask.updated_at))}
        </time>
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
        ) : summary ? (
          <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed break-words">
            {summary}
          </p>
        ) : null}
      </div>

      {/* Assignee snippet */}
      {agent && (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
          <span className="text-muted-foreground/70">{t('Assignee')}:</span>
          <span className="font-medium text-foreground">{agentDisplayName(agent)}</span>
        </div>
      )}

      {/* Footer: Quick Action Buttons (Tombol Aksi Cepat) */}
      <div className="flex flex-wrap items-center justify-between gap-2 pt-2 border-t border-border/50">
        <button
          type="button"
          onClick={handleCopyLink}
          aria-label={t('Copy link')}
          className="action-secondary min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 transition-colors cursor-pointer"
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
              aria-label={t('Retry')}
              className="action-primary min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 shadow-xs cursor-pointer"
            >
              <RefreshCw className={cn('h-3.5 w-3.5 shrink-0', retrying && 'animate-spin')} />
              <span>{retrying ? t('Retrying...') : t('Retry')}</span>
            </button>
          )}

          <button
            type="button"
            onClick={handleActionClick}
            aria-label={`${t('Open task')}: ${currentTask.title}`}
            className="action-primary min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 shadow-xs cursor-pointer"
          >
            {isWorking ? (
              <>
                <Terminal className="h-3.5 w-3.5 shrink-0" />
                <span>{t('Live Terminal')}</span>
              </>
            ) : status === 'Ready for review' || status === 'Completed' || status === 'Answered' ? (
              <>
                <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
                <span>{t('View result')}</span>
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
