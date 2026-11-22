import { TaskDuration } from './TaskDuration'
import { ListRowContent } from '../../components/layout/ListControls'
import { useState, useEffect, useRef } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import {
  Folder,
  Terminal,
  CheckCircle2,
  Clock3,
  Eye,
  X,
  AlertCircle,
  Play,
  Rocket,
  RefreshCw,
  FileText,
  Trash2,
  MoreHorizontal,
  Loader2,
} from 'lucide-react'
import type { Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { agentsApi } from '../../api/agents'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { StatusDot, type StatusDotVariant } from '../../components/ui/StatusDot'
import './task-workspace.css'
import {
  workStatus,
  workDisplayStatus,
  taskAgentPending,
  taskExecutionActive,
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

function getTaskDotVariant(status: string): StatusDotVariant {
  switch (status) {
    case 'Queued':
    case 'Waiting to start':
      return 'warning'
    case 'Needs attention':
      return 'exception'
    case 'Ready for review':
      return 'neutral'
    case 'Completed':
    case 'Answered':
      return 'healthy'
    case 'Cancelled':
      return 'neutral'
    case 'Not running':
    case 'Not started':
    default:
      return 'neutral'
  }
}

export interface TaskItemCardProps {
  task: Task
  returnTo?: string
  showProject?: boolean
  managedUpdates?: boolean
  showActions?: boolean
  variant?: 'card' | 'row'
  className?: string
  queuePosition?: number
  onClick?: (task: Task) => void
  onNavigate?: () => void
  onRetry?: (task: Task) => void | Promise<void>
  onDelete?: (task: Task) => void | Promise<void>
}

export function TaskItemCard({
  task,
  returnTo,
  showProject = true,
  showActions = true,
  managedUpdates = false,
  variant = 'card',
  className,
  queuePosition,
  onClick,
  onNavigate,
  onRetry,
  onDelete,
}: TaskItemCardProps) {
  const { t } = useLanguage()
  const location = useLocation()
  const navigate = useNavigate()
  const currentReturnTo = returnTo ?? location.pathname

  const [currentTask, setCurrentTask] = useState<Task>(task)
  useEffect(() => {
    setCurrentTask(task)
  }, [task])

  const status = workStatus(currentTask)
  const displayStatus = workDisplayStatus(currentTask)
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
    if ((!isRunning && !taskAgentPending(currentTask)) || managedUpdates) return
    let active = true
    const poll = async () => {
      if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
      try {
        const updated = await tasksApi.get(currentTask.id)
        if (active && updated) {
          setCurrentTask((prev) => {
            if (
              prev.latest_progress !== updated.latest_progress ||
              prev.review_state !== updated.review_state ||
              prev.review_action !== updated.review_action ||
              prev.review_finding !== updated.review_finding ||
              prev.launch?.state !== updated.launch?.state ||
              prev.latest_attempt?.cancel_requested !== updated.latest_attempt?.cancel_requested ||
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
    }
    const interval = setInterval(() => void poll(), 3000)
    const onResume = () => void poll()
    document.addEventListener('visibilitychange', onResume)
    return () => {
      active = false
      clearInterval(interval)
      document.removeEventListener('visibilitychange', onResume)
    }
  }, [isRunning, currentTask.id, currentTask.review_state, managedUpdates])
  const agent =
    currentTask.latest_attempt?.agent_name || currentTask.assignee_id || currentTask.assignee
  const statusBadgeClass = workStatusBadgeClass(status)
  const [applying, setApplying] = useState(false)
  const [applied, setApplied] = useState(
    currentTask.status === 'done' && (currentTask.complete_note?.includes('diterapkan') || !!currentTask.pr_url)
  )
  const [retrying, setRetrying] = useState(false)
  const [retryError, setRetryError] = useState('')
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!menuOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [menuOpen])

  const handleOpenDeleteModal = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDeleteError('')
    setDeleteModalOpen(true)
  }

  const handleConfirmDelete = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (deleting) return
    setDeleting(true)
    setDeleteError('')
    try {
      if (onDelete) {
        await onDelete(currentTask)
      } else {
        await tasksApi.delete(currentTask.id)
      }
      window.dispatchEvent(
        new CustomEvent('cuan:task-deleted', { detail: { id: currentTask.id } })
      )
      setDeleteModalOpen(false)
      onNavigate?.()
    } catch (err: any) {
      setDeleteError(err?.message || t('Failed to cancel task'))
    } finally {
      setDeleting(false)
    }
  }

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
    const dotVariant = taskExecutionActive(currentTask) ? 'working' : isQueued ? 'queued' : getTaskDotVariant(status)

    return (
      <div
        role="button"
        tabIndex={0}
        onClick={(e) => {
          e.preventDefault()
          if (onClick) {
            onClick(currentTask)
          } else {
            navigate(detailUrl, { state: { taskReturnTo: currentReturnTo } })
          }
          onNavigate?.()
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            if (onClick) {
              onClick(currentTask)
            } else {
              navigate(detailUrl, { state: { taskReturnTo: currentReturnTo } })
            }
            onNavigate?.()
          }
        }}
        className={cn(
          'group flex min-h-[48px] items-center gap-3 border-b border-border/40 px-2.5 py-3 transition-colors hover:bg-muted/40 active:bg-muted/60 cursor-pointer select-none',
          className
        )}
      >
        <ListRowContent
          leading={<span className="flex h-5 w-5 items-center justify-center">{status === 'Completed' ? <CheckCircle2 className="h-3.5 w-3.5 text-[color:var(--status-success)]" aria-label={t(displayStatus)} /> : <StatusDot status={dotVariant} size="sm" aria-label={t(displayStatus)} title={t(displayStatus)} />}</span>}
          title={<span className="flex min-w-0 items-center gap-1.5"><span className="truncate">{currentTask.title}</span>{showProject&&currentTask.project_name&&<span className="hidden shrink-0 text-xs font-normal text-muted-foreground sm:inline">· {currentTask.project_name}</span>}</span>}
          description={isWorking&&liveActivity?liveActivity:isQueued?<>{queuePosition? t('Antrean')+' #'+queuePosition+' · ':''}{currentTask.latest_progress?cleanTerminalText(currentTask.latest_progress):t('Menunggu giliran eksekusi runner...')}</>:undefined}
          trailing={<TaskDuration task={currentTask} compact fallback={<time dateTime={currentTask.updated_at} title={new Date(currentTask.updated_at).toLocaleString()}>{t(relativeTime(currentTask.updated_at))}</time>} />}
        />
      </div>
    )
  }

  return (
    <div
      onClick={onClick ? () => onClick(currentTask) : undefined}
      className={cn(
        'group rounded-xl border border-border/40 bg-card/60 p-4 text-card-foreground shadow-2xs transition-all hover:border-border/60 hover:bg-card/80 space-y-3',
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
            {taskExecutionActive(currentTask) ? (
              <StatusDot status="working" size="sm" label={t(displayStatus)} />
            ) : isQueued ? (
              <Clock3 className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            ) : status === 'Ready for review' ? (
              <Eye className="h-3.5 w-3.5 shrink-0 text-purple-600 dark:text-purple-400" />
            ) : status === 'Completed' || status === 'Answered' ? (
              <CheckCircle2 className="h-3 w-3 shrink-0 text-[color:var(--status-success)]" />
            ) : status === 'Cancelled' ? (
              <X className="h-3.5 w-3.5 shrink-0 text-zinc-500 dark:text-zinc-400" />
            ) : status === 'Needs attention' ? (
              <AlertCircle className="h-3.5 w-3.5 shrink-0 text-destructive" />
            ) : (
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-muted-foreground/60 shrink-0" />
            )}
            {isQueued && queuePosition ? `${t(displayStatus)} #${queuePosition}` : t(displayStatus)}
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

        <div className="flex items-center gap-1.5 shrink-0">
          <TaskDuration task={currentTask} compact fallback={
            <time dateTime={currentTask.updated_at} title={new Date(currentTask.updated_at).toLocaleString()} className="text-xs text-muted-foreground whitespace-nowrap">
              {t(relativeTime(currentTask.updated_at))}
            </time>
          } />
          {status === 'Not started' && (
            <div className="relative">
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation()
                  e.preventDefault()
                  setMenuOpen((prev) => !prev)
                }}
                className="h-7 w-7 min-h-7 min-w-7 inline-flex items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/80 transition-colors cursor-pointer"
                aria-label={t('Task options menu')}
                title={t('Task options menu')}
              >
                <MoreHorizontal className="h-4 w-4" />
              </button>
              {menuOpen && (
                <div
                  ref={menuRef}
                  onClick={(e) => e.stopPropagation()}
                  className="absolute right-0 top-full mt-1 z-30 min-w-44 rounded-xl border border-border/40 bg-popover/95 backdrop-blur-md p-1 text-popover-foreground shadow-lg animate-in fade-in-50 zoom-in-95"
                >
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation()
                      setMenuOpen(false)
                      handleOpenDeleteModal(e)
                    }}
                    className="flex min-h-10 w-full items-center gap-2 px-3 py-1.5 text-left text-xs text-destructive hover:bg-destructive/10 rounded-md transition-colors cursor-pointer"
                  >
                    <Trash2 className="h-3.5 w-3.5 shrink-0" />
                    <span>{t('Cancel / Delete task')}</span>
                  </button>
                  <Link
                    to={detailUrl}
                    state={{ taskReturnTo: currentReturnTo }}
                    onClick={(e) => {
                      e.stopPropagation()
                      setMenuOpen(false)
                      onNavigate?.()
                    }}
                    className="flex min-h-10 w-full items-center gap-2 px-3 py-1.5 text-left text-xs hover:bg-muted rounded-md transition-colors"
                  >
                    <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <span>{t('Edit draft')}</span>
                  </Link>
                </div>
              )}
            </div>
          )}
        </div>
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
            <StatusDot status={taskAgentPending(currentTask) ? 'working' : 'neutral'} size="sm" label={t(displayStatus)} />
            <span className="line-clamp-2 break-words text-[11px] font-medium leading-tight">
              {liveActivity}
            </span>
          </div>
        ) : isQueued ? (
          <div className="relative z-10 flex items-center gap-2 rounded-lg bg-amber-500/10 border border-amber-500/25 px-2.5 py-1.5 text-xs text-amber-700 dark:text-amber-300 animate-in fade-in duration-200">
            <StatusDot status={taskAgentPending(currentTask) ? 'working' : 'warning'} size="sm" label={t(displayStatus)} />
            <span className="line-clamp-2 break-words text-[11px] font-medium leading-tight">
              {queuePosition ? `${t('Antrean')} #${queuePosition} · ` : ''}
              {currentTask.latest_progress
                ? cleanTerminalText(currentTask.latest_progress)
                : t('Menunggu giliran eksekusi runner...')}
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
          {/* Left Actions: Detail & Cancel draft */}
          <div className="flex items-center gap-2">
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

            {status === 'Not started' && (
              <button
                type="button"
                onClick={handleOpenDeleteModal}
                className="action-secondary min-h-11 min-w-[44px] inline-flex items-center gap-1.5 px-3 text-xs font-medium rounded-lg justify-center text-muted-foreground hover:text-destructive hover:bg-destructive/10 border border-transparent hover:border-destructive/20 transition-colors focus-visible:outline-2 focus-visible:outline-ring cursor-pointer"
                aria-label={`${t('Cancel draft')} ${currentTask.title}`}
                title={t('Cancel draft')}
              >
                <Trash2 className="h-3.5 w-3.5 shrink-0 text-destructive/80" />
                <span>{t('Cancel draft')}</span>
              </button>
            )}
          </div>

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
            {status === 'Needs attention' && currentTask.source_room_id ? null : status === 'Needs attention' ? (
              <button
                type="button"
                disabled={retrying}
                onClick={handleRetry}
                className="action-primary min-h-11 min-w-[44px] inline-flex items-center gap-1.5 px-4 text-xs font-medium rounded-lg justify-center shadow-sm focus-visible:outline-2 focus-visible:outline-ring"
              >
                <RefreshCw className={cn('h-3.5 w-3.5 shrink-0', retrying && 'animate-spin')} />
                <span>{retrying ? t('Retrying...') : t('Retry')}</span>
              </button>
            ) : isWorking ? (
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
            ) : isQueued ? null : (
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

      {/* Cancel/Delete Draft Confirmation Dialog */}
      <ActionDialog
        open={deleteModalOpen}
        title={t('Cancel draft task?')}
        onClose={() => {
          if (!deleting) setDeleteModalOpen(false)
        }}
        busy={deleting}
      >
        <div className="space-y-4 text-sm" onClick={(e) => e.stopPropagation()}>
          <p className="text-muted-foreground leading-relaxed">
            {t(
              'This draft task has not been started. Remove it from your task list?'
            )}
          </p>

          <div className="rounded-lg bg-muted/50 p-2.5 text-xs">
            <span className="font-semibold text-foreground line-clamp-1">
              {currentTask.title}
            </span>
            {summary && (
              <span className="text-muted-foreground line-clamp-1 mt-0.5">
                {summary}
              </span>
            )}
          </div>

          {deleteError && (
            <p className="text-xs font-medium text-destructive">{deleteError}</p>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              className="action-secondary min-h-11 min-w-[44px] px-3.5 text-xs font-medium rounded-lg"
              disabled={deleting}
              onClick={(e) => {
                e.stopPropagation()
                setDeleteModalOpen(false)
              }}
            >
              {t('Keep draft')}
            </button>
            <button
              type="button"
              className="action-primary bg-destructive text-destructive-foreground hover:bg-destructive/90 min-h-11 min-w-[44px] px-4 text-xs font-medium rounded-lg inline-flex items-center gap-1.5"
              disabled={deleting}
              onClick={handleConfirmDelete}
            >
              {deleting ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin shrink-0" />
                  <span>{t('Deleting...')}</span>
                </>
              ) : (
                <>
                  <Trash2 className="h-3.5 w-3.5 shrink-0" />
                  <span>{t('Cancel task')}</span>
                </>
              )}
            </button>
          </div>
        </div>
      </ActionDialog>
    </div>
  )
}
