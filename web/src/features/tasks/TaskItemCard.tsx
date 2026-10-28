import { useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  Folder,
  Terminal,
  CheckCircle2,
  Play,
  Rocket,
  RefreshCw,
  FileText,
} from 'lucide-react'
import type { Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { agentsApi } from '../../api/agents'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import './task-workspace.css'
import {
  workStatus,
  workStatusBadgeClass,
  taskTargetUrl,
  taskDetailUrl,
  taskTerminalUrl,
  taskSummaryText,
  taskLiveActivityText,
  cleanTerminalText,
  taskRef,
  relativeTime,
  agentDisplayName,
} from './task-ui'

export interface TaskItemCardProps {
  task: Task
  returnTo?: string
  showProject?: boolean
  showActions?: boolean
  variant?: 'card' | 'row'
  className?: string
  queuePosition?: number
  onClick?: (task: Task) => void
  onNavigate?: () => void
  onRetry?: (task: Task) => void | Promise<void>
}

export function TaskItemCard({
  task,
  returnTo,
  showProject = true,
  showActions = true,
  variant = 'card',
  className,
  queuePosition,
  onClick,
  onNavigate,
  onRetry,
}: TaskItemCardProps) {
  const { t } = useLanguage()
  const location = useLocation()
  const currentReturnTo = returnTo ?? location.pathname

  const [currentTask, setCurrentTask] = useState<Task>(task)
  useEffect(() => {
    setCurrentTask(task)
  }, [task])

  const status = workStatus(currentTask)
  const isWorking = status === 'Working'
  const isQueued =
    !isWorking &&
    (status === 'Queued' ||
      currentTask.launch?.state === 'queued' ||
      currentTask.launch?.state === 'claimed' ||
      currentTask.latest_attempt?.status === 'queued' ||
      (currentTask.status as string) === 'queued' ||
      (currentTask.status as string) === 'pending')
  const isRunning = isWorking || isQueued
  const targetUrl = taskTargetUrl(currentTask)
  const detailUrl = taskDetailUrl(currentTask)
  const terminalUrl = taskTerminalUrl(currentTask)
  const summary = taskSummaryText(currentTask)
  const liveActivity = taskLiveActivityText(currentTask, t('Runner aktif...'))

  useEffect(() => {
    if (!isRunning) return
    let active = true
    const interval = setInterval(async () => {
      try {
        const updated = await tasksApi.get(currentTask.id)
        if (active && updated) {
          setCurrentTask((prev) => {
            if (
              prev.latest_progress !== updated.latest_progress ||
              prev.status !== updated.status ||
              prev.latest_attempt?.status !== updated.latest_attempt?.status
            ) {
              return { ...prev, ...updated }
            }
            return prev
          })
        }
      } catch {
        // ignore
      }
    }, 3000)
    return () => {
      active = false
      clearInterval(interval)
    }
  }, [isRunning, currentTask.id])
  const agent =
    currentTask.latest_attempt?.agent_name || currentTask.assignee_id || currentTask.assignee
  const statusBadgeClass = workStatusBadgeClass(status)
  const [applying, setApplying] = useState(false)
  const [applied, setApplied] = useState(
    currentTask.status === 'done' && (currentTask.complete_note?.includes('diterapkan') || !!currentTask.pr_url)
  )
  const [retrying, setRetrying] = useState(false)
  const [retryError, setRetryError] = useState('')

  const handleApply = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (applying || applied) return
    setApplying(true)
    try {
      await tasksApi.apply(currentTask.id)
      setApplied(true)
      onNavigate?.()
    } catch {
      // ignore
    } finally {
      setApplying(false)
    }
  }

  const handleRetry = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (retrying) return
    setRetrying(true)
    setRetryError('')
    try {
      if (onRetry) {
        await onRetry(currentTask)
      } else {
        let agentId =
          currentTask.latest_attempt?.agent_name ||
          currentTask.assignee_id ||
          currentTask.assignee ||
          ''
        if (!agentId) {
          try {
            const list = await agentsApi.list()
            const eligible = list.find((a) => a.enabled !== false && a.status !== 'inactive')
            if (eligible) agentId = eligible.id
          } catch {}
        }
        if (!agentId) {
          agentId = 'agy'
        }
        const result = await tasksApi.runAgent(currentTask.id, {
          request_id: crypto.randomUUID(),
          agent_id: agentId,
        })
        if (result?.attempt) {
          setCurrentTask((prev) => ({
            ...prev,
            status: 'doing',
            execution_status: 'running',
            latest_attempt: result.attempt,
          }))
        }
      }
      onNavigate?.()
    } catch (err: any) {
      setRetryError(err?.message || 'Failed to retry session')
    } finally {
      setRetrying(false)
    }
  }

  if (variant === 'row') {
    return (
      <div
        onClick={onClick ? () => onClick(currentTask) : undefined}
        className={cn(
          'group flex min-h-16 items-center gap-3 border-b px-2 py-3 transition-colors hover:bg-muted/50 focus-within:bg-muted/30',
          onClick && 'cursor-pointer',
          className
        )}
      >
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <div className="flex min-w-0 items-center gap-2">
              <span className="shrink-0 font-mono text-xs text-muted-foreground">
                {taskRef(currentTask.id)}
              </span>
              <span
                className={cn(
                  'inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold transition-colors',
                  statusBadgeClass
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
                ) : status === 'Cancelled' ? (
                  <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-zinc-500 dark:bg-zinc-400 shrink-0" />
                ) : status === 'Needs attention' ? (
                  <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-destructive shrink-0" />
                ) : (
                  <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-muted-foreground/60 shrink-0" />
                )}
                {isQueued && queuePosition ? `${t('Queued')} #${queuePosition}` : t(status)}
              </span>
              {showProject && currentTask.project_name && (
                <span className="inline-flex items-center gap-1 truncate text-xs font-medium text-muted-foreground">
                  <Folder className="h-3 w-3 shrink-0" />
                  {currentTask.project_name}
                </span>
              )}
            </div>
            <time
              dateTime={currentTask.updated_at}
              title={new Date(currentTask.updated_at).toLocaleString()}
              className="shrink-0 text-xs text-muted-foreground"
            >
              {t(relativeTime(currentTask.updated_at))}
            </time>
          </div>

          <Link
            to={targetUrl}
            state={{ taskReturnTo: currentReturnTo }}
            onClick={
              onClick
                ? (e) => {
                    e.preventDefault()
                    onClick(currentTask)
                  }
                : onNavigate
            }
            className="mt-1 block focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
          >
            <span className="line-clamp-2 break-words text-sm font-medium leading-5 transition-colors group-hover:text-primary">
              {currentTask.title}
            </span>
          </Link>

          {isWorking ? (
            <div className="mt-1 flex items-center gap-1.5 text-xs text-blue-600 dark:text-blue-400">
              <span className="relative flex h-1.5 w-1.5 shrink-0 items-center justify-center">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-500 opacity-75" />
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-blue-600 dark:bg-blue-400" />
              </span>
              <span className="line-clamp-1 truncate text-[11px] font-medium">
                {liveActivity}
              </span>
            </div>
          ) : isQueued ? (
            <div className="mt-1 flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400">
              <span className="relative flex h-1.5 w-1.5 shrink-0 items-center justify-center">
                <span className="animate-pulse absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-amber-500 dark:bg-amber-400" />
              </span>
              <span className="line-clamp-1 truncate text-[11px] font-medium">
                {queuePosition ? `${t('Antrean')} #${queuePosition} · ` : ''}
                {currentTask.latest_progress
                  ? cleanTerminalText(currentTask.latest_progress)
                  : (summary || t('Menunggu giliran eksekusi runner...'))}
              </span>
            </div>
          ) : summary ? (
            <p className="mt-0.5 line-clamp-1 break-words text-xs text-muted-foreground">
              {summary}
            </p>
          ) : null}

          <div className="mt-1 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
            {agent && (
              <span className="font-mono text-[11px]">
                {agentDisplayName(agent)}
              </span>
            )}
            {currentTask.priority && currentTask.priority !== 'normal' && (
              <span
                className={cn(
                  'capitalize font-medium',
                  currentTask.priority === 'urgent' && 'text-destructive',
                  currentTask.priority === 'high' && 'text-amber-600 dark:text-amber-400'
                )}
              >
                {currentTask.priority}
              </span>
            )}
          </div>
          {retryError && (
            <p className="mt-1 text-[11px] font-medium text-destructive">{retryError}</p>
          )}
        </div>

        {showActions && (
          <div className="flex shrink-0 items-center gap-2">
            <Link
              to={detailUrl}
              state={{ taskReturnTo: currentReturnTo }}
              onClick={(e) => {
                e.stopPropagation()
                onNavigate?.()
              }}
              className="action-secondary min-h-11 min-w-[44px] px-3 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5"
              aria-label={`${t('Detail')} for ${currentTask.title}`}
            >
              <FileText className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">{t('Detail')}</span>
            </Link>

            {status === 'Needs attention' ? (
              <button
                type="button"
                disabled={retrying}
                onClick={handleRetry}
                className="action-primary min-h-11 min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5"
              >
                <RefreshCw className={cn('h-3.5 w-3.5', retrying && 'animate-spin')} />
                <span>{retrying ? t('Retrying...') : t('Retry')}</span>
              </button>
            ) : isRunning ? (
              <Link
                to={terminalUrl}
                state={{ taskReturnTo: currentReturnTo }}
                onClick={(e) => {
                  e.stopPropagation()
                  onNavigate?.()
                }}
                className="action-primary min-h-11 min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5"
              >
                <Terminal className="h-3.5 w-3.5" />
                <span>{t('Live Terminal')}</span>
              </Link>
            ) : status === 'Ready for review' ||
              status === 'Completed' ||
              status === 'Answered' ? (
              <Link
                to={detailUrl}
                state={{ taskReturnTo: currentReturnTo }}
                onClick={(e) => {
                  e.stopPropagation()
                  onNavigate?.()
                }}
                className="action-primary min-h-11 min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5"
              >
                <CheckCircle2 className="h-3.5 w-3.5" />
                <span>{t('View result')}</span>
              </Link>
            ) : (
              <Link
                to={terminalUrl}
                state={{ taskReturnTo: currentReturnTo }}
                onClick={(e) => {
                  e.stopPropagation()
                  onNavigate?.()
                }}
                className="action-primary min-h-11 min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5"
              >
                <Play className="h-3.5 w-3.5" />
                <span>{t('Start session')}</span>
              </Link>
            )}
          </div>
        )}
      </div>
    )
  }

  return (
    <div
      onClick={onClick ? () => onClick(currentTask) : undefined}
      className={cn(
        'group rounded-xl border border-border bg-card p-4 text-card-foreground shadow-sm transition-all hover:border-border/80 space-y-3',
        onClick && 'cursor-pointer',
        className
      )}
    >
      {/* Header: Status badge, Project badge, TaskRef, and Timestamp */}
      <div className="flex items-start justify-between gap-2 relative z-10">
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              'inline-flex items-center gap-1.5 text-[11px] font-semibold px-2.5 py-0.5 rounded-full border transition-colors',
              statusBadgeClass
            )}
          >
            {isWorking ? (
              <span className="relative flex h-2 w-2 shrink-0 items-center justify-center">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-500 opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-600 dark:bg-blue-400" />
              </span>
            ) : isQueued ? (
              <span className="relative flex h-2 w-2 shrink-0 items-center justify-center">
                <span className="animate-pulse absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500 dark:bg-amber-400" />
              </span>
            ) : status === 'Ready for review' ? (
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-purple-600 dark:bg-purple-400 shrink-0" />
            ) : status === 'Completed' || status === 'Answered' ? (
              <CheckCircle2 className="h-3 w-3 shrink-0 text-emerald-600 dark:text-emerald-400" />
            ) : status === 'Cancelled' ? (
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-zinc-500 dark:bg-zinc-400 shrink-0" />
            ) : status === 'Needs attention' ? (
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-destructive shrink-0" />
            ) : (
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-muted-foreground/60 shrink-0" />
            )}
            {isQueued && queuePosition ? `${t('Queued')} #${queuePosition}` : t(status)}
          </span>

          {showProject && currentTask.project_name && (
            <span className="inline-flex items-center gap-1 text-xs text-muted-foreground font-medium">
              <Folder className="h-3.5 w-3.5" />
              {currentTask.project_name}
            </span>
          )}

          <span className="font-mono text-xs text-muted-foreground">
            {taskRef(currentTask.id)}
          </span>
          {agent && (
            <span
              className="inline-flex items-center text-[11px] text-muted-foreground/75 font-mono"
              title={`Agent: ${agentDisplayName(agent)}`}
            >
              {agentDisplayName(agent)}
            </span>
          )}
        </div>

        <time
          dateTime={currentTask.updated_at}
          title={new Date(currentTask.updated_at).toLocaleString()}
          className="text-xs text-muted-foreground whitespace-nowrap"
        >
          {t(relativeTime(currentTask.updated_at))}
        </time>
      </div>

      {/* Title & Live Activity / Summary with Vertical Timeline flow */}
      <div className="space-y-2">
        <div className="relative">
          {isWorking ? (
            <div
              aria-hidden="true"
              className="task-timeline-connector absolute left-[14px] -top-3 -bottom-2 w-[2px] -translate-x-1/2 bg-blue-500/35 dark:bg-blue-400/40 rounded-full pointer-events-none"
            />
          ) : isQueued ? (
            <div
              aria-hidden="true"
              className="task-timeline-connector-queued absolute left-[14px] -top-3 -bottom-2 w-[2px] -translate-x-1/2 border-l-2 border-dashed border-amber-500/40 dark:border-amber-400/45 pointer-events-none"
            />
          ) : null}
          <Link
            to={targetUrl}
            state={{ taskReturnTo: currentReturnTo }}
            onClick={
              onClick
                ? (e) => {
                    e.preventDefault()
                    e.stopPropagation()
                    onClick(currentTask)
                  }
                : onNavigate
            }
            className={cn(
              'block group-hover:text-primary transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring',
              (isWorking || isQueued) && 'pl-7'
            )}
          >
            <h4 className="text-sm font-semibold text-foreground break-words leading-snug">
              {currentTask.title}
            </h4>
          </Link>
        </div>

        {isWorking ? (
          <div className="relative z-10 flex items-center gap-2 rounded-lg bg-blue-500/10 border border-blue-500/20 px-2.5 py-1.5 text-xs text-blue-700 dark:text-blue-300 animate-in fade-in duration-200">
            <span className="relative flex h-2 w-2 shrink-0 items-center justify-center">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-500 opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-600 dark:bg-blue-400" />
            </span>
            <span className="line-clamp-2 break-words text-[11px] font-medium leading-tight">
              {liveActivity}
            </span>
          </div>
        ) : isQueued ? (
          <div className="relative z-10 flex items-center gap-2 rounded-lg bg-amber-500/10 border border-amber-500/25 px-2.5 py-1.5 text-xs text-amber-700 dark:text-amber-300 animate-in fade-in duration-200">
            <span className="relative flex h-2 w-2 shrink-0 items-center justify-center">
              <span className="animate-pulse absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500 dark:bg-amber-400" />
            </span>
            <span className="line-clamp-2 break-words text-[11px] font-medium leading-tight">
              {queuePosition ? `${t('Antrean')} #${queuePosition} · ` : ''}
              {currentTask.latest_progress
                ? cleanTerminalText(currentTask.latest_progress)
                : (summary || t('Menunggu giliran eksekusi runner...'))}
            </span>
          </div>
        ) : summary ? (
          <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed">
            {summary}
          </p>
        ) : null}
      </div>

      {/* Footer: Action buttons */}
      {showActions && (
        <div className="flex items-center justify-between gap-2 pt-2.5 border-t border-border/50 text-xs">
          {/* Detail Action: Always positioned on the Left */}
          <Link
            to={detailUrl}
            state={{ taskReturnTo: currentReturnTo }}
            onClick={(e) => {
              e.stopPropagation()
              onNavigate?.()
            }}
            className="action-secondary min-h-11 min-w-[44px] inline-flex items-center gap-1 px-3.5 text-xs font-medium rounded-lg justify-center transition-colors focus-visible:outline-2 focus-visible:outline-ring"
            aria-label={`${t('Detail')} for ${currentTask.title}`}
          >
            <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span>{t('Detail')}</span>
          </Link>

          {/* Primary Action: Always positioned on the Right */}
          <div className="flex items-center gap-2">
            {currentTask.pr_url && !applied && status === 'Ready for review' && (
              <button
                type="button"
                disabled={applying}
                onClick={handleApply}
                className="action-primary min-h-11 min-w-[44px] inline-flex items-center gap-1.5 px-3.5 text-xs font-medium rounded-lg justify-center bg-gradient-to-r from-emerald-600 to-teal-600 text-white hover:from-emerald-700 hover:to-teal-700 shadow-sm transition-all focus-visible:outline-2 focus-visible:outline-ring"
              >
                <Rocket className="h-3.5 w-3.5 shrink-0" />
                <span>{applying ? t('Applying...') : t('Apply changes')}</span>
              </button>
            )}
            {applied && (
              <span className="inline-flex min-h-11 items-center gap-1 text-[11px] font-semibold text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
                <span>{t('Changes applied')}</span>
              </span>
            )}
            {status === 'Needs attention' ? (
              <button
                type="button"
                disabled={retrying}
                onClick={handleRetry}
                className="action-primary min-h-11 min-w-[44px] inline-flex items-center gap-1.5 px-4 text-xs font-medium rounded-lg justify-center shadow-sm focus-visible:outline-2 focus-visible:outline-ring"
              >
                <RefreshCw className={cn('h-3.5 w-3.5 shrink-0', retrying && 'animate-spin')} />
                <span>{retrying ? t('Retrying...') : t('Retry')}</span>
              </button>
            ) : isRunning ? (
              <Link
                to={terminalUrl}
                state={{ taskReturnTo: currentReturnTo }}
                onClick={(e) => {
                  e.stopPropagation()
                  onNavigate?.()
                }}
                className="action-primary min-h-11 min-w-[44px] inline-flex items-center gap-1.5 px-4 text-xs font-medium rounded-lg justify-center shadow-sm focus-visible:outline-2 focus-visible:outline-ring"
              >
                <Terminal className="h-3.5 w-3.5 shrink-0" />
                <span>{t('Live Terminal')}</span>
              </Link>
            ) : status === 'Ready for review' ||
              status === 'Completed' ||
              status === 'Answered' ? (
              <Link
                to={detailUrl}
                state={{ taskReturnTo: currentReturnTo }}
                onClick={(e) => {
                  e.stopPropagation()
                  onNavigate?.()
                }}
                className={cn(
                  'min-h-11 min-w-[44px] inline-flex items-center gap-1.5 px-4 text-xs font-medium rounded-lg justify-center shadow-sm focus-visible:outline-2 focus-visible:outline-ring',
                  currentTask.pr_url && !applied && status === 'Ready for review'
                    ? 'action-secondary'
                    : 'action-primary'
                )}
              >
                <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
                <span>{t('View result')}</span>
              </Link>
            ) : (
              <Link
                to={terminalUrl}
                state={{ taskReturnTo: currentReturnTo }}
                onClick={(e) => {
                  e.stopPropagation()
                  onNavigate?.()
                }}
                className="action-primary min-h-11 min-w-[44px] inline-flex items-center gap-1.5 px-4 text-xs font-medium rounded-lg justify-center shadow-sm focus-visible:outline-2 focus-visible:outline-ring"
              >
                <Play className="h-3.5 w-3.5 shrink-0" />
                <span>{t('Start session')}</span>
              </Link>
            )}
          </div>
        </div>
      )}
      {retryError && (
        <div className="text-[11px] text-destructive pt-1 font-medium break-words">
          {retryError}
        </div>
      )}
    </div>
  )
}
