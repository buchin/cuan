import { Link, useLocation } from 'react-router-dom'
import {
  Folder,
  Terminal,
  CheckCircle2,
  Play,
} from 'lucide-react'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import {
  workStatus,
  workStatusBadgeClass,
  isTaskRunning,
  taskTargetUrl,
  taskSummaryText,
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
  onClick?: (task: Task) => void
}

export function TaskItemCard({
  task,
  returnTo,
  showProject = true,
  showActions = true,
  variant = 'card',
  className,
  onClick,
}: TaskItemCardProps) {
  const { t } = useLanguage()
  const location = useLocation()
  const currentReturnTo = returnTo ?? location.pathname

  const status = workStatus(task)
  const isRunning = isTaskRunning(task)
  const targetUrl = taskTargetUrl(task)
  const summary = taskSummaryText(task)
  const agent =
    task.latest_attempt?.agent_name || task.assignee_id || task.assignee
  const statusBadgeClass = workStatusBadgeClass(status)

  if (variant === 'row') {
    return (
      <div
        className={cn(
          'group flex min-h-16 items-center gap-3 border-b px-2 py-3 transition-colors hover:bg-muted/50 focus-within:bg-muted/30',
          className
        )}
      >
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <div className="flex min-w-0 items-center gap-2">
              <span className="shrink-0 font-mono text-xs text-muted-foreground">
                {taskRef(task.id)}
              </span>
              <span
                className={cn(
                  'inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-semibold',
                  statusBadgeClass
                )}
              >
                {isRunning && (
                  <span className="h-1.5 w-1.5 rounded-full bg-blue-500 animate-ping" />
                )}
                {t(status)}
              </span>
              {showProject && task.project_name && (
                <span className="inline-flex items-center gap-1 truncate text-xs font-medium text-muted-foreground">
                  <Folder className="h-3 w-3 shrink-0" />
                  {task.project_name}
                </span>
              )}
            </div>
            <time
              dateTime={task.updated_at}
              title={new Date(task.updated_at).toLocaleString()}
              className="shrink-0 text-xs text-muted-foreground"
            >
              {t(relativeTime(task.updated_at))}
            </time>
          </div>

          <Link
            to={targetUrl}
            state={{ taskReturnTo: currentReturnTo }}
            onClick={
              onClick
                ? (e) => {
                    e.preventDefault()
                    onClick(task)
                  }
                : undefined
            }
            className="mt-1 block focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
          >
            <span className="line-clamp-2 break-words text-sm font-medium leading-5 transition-colors group-hover:text-primary">
              {task.title}
            </span>
          </Link>

          {summary && (
            <p className="mt-0.5 line-clamp-1 break-words text-xs text-muted-foreground">
              {summary}
            </p>
          )}

          <div className="mt-1 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
            {agent && (
              <span>
                {t('Agent')}: {agentDisplayName(agent)}
              </span>
            )}
            {task.priority && task.priority !== 'normal' && (
              <span
                className={cn(
                  'capitalize font-medium',
                  task.priority === 'urgent' && 'text-destructive',
                  task.priority === 'high' && 'text-amber-600 dark:text-amber-400'
                )}
              >
                {task.priority}
              </span>
            )}
          </div>
        </div>

        {showActions && (
          <div className="flex shrink-0 items-center gap-1.5 sm:flex">
            <Link
              to={targetUrl}
              state={{ taskReturnTo: currentReturnTo }}
              onClick={
                onClick
                  ? (e) => {
                      e.preventDefault()
                      onClick(task)
                    }
                  : undefined
              }
              className="action-primary inline-flex items-center gap-1 px-2.5 py-1 text-xs"
            >
              {isRunning ? (
                <>
                  <Terminal className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">{t('Live Terminal')}</span>
                </>
              ) : status === 'Ready for review' ||
                status === 'Completed' ||
                status === 'Answered' ? (
                <>
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">{t('View result')}</span>
                </>
              ) : (
                <>
                  <Play className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">{t('Start session')}</span>
                </>
              )}
            </Link>
          </div>
        )}
      </div>
    )
  }

  return (
    <div
      onClick={onClick ? () => onClick(task) : undefined}
      className={cn(
        'group rounded-xl border border-border bg-card p-4 text-card-foreground shadow-sm transition-all hover:border-border/80 space-y-3',
        onClick && 'cursor-pointer',
        className
      )}
    >
      {/* Header: Status badge, Project badge, TaskRef, and Timestamp */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              'inline-flex items-center gap-1.5 text-[11px] font-semibold px-2 py-0.5 rounded-full border',
              statusBadgeClass
            )}
          >
            {isRunning && (
              <span className="h-1.5 w-1.5 rounded-full bg-blue-500 animate-ping" />
            )}
            {t(status)}
          </span>

          {showProject && task.project_name && (
            <span className="inline-flex items-center gap-1 text-xs text-muted-foreground font-medium">
              <Folder className="h-3.5 w-3.5" />
              {task.project_name}
            </span>
          )}

          <span className="font-mono text-xs text-muted-foreground">
            {taskRef(task.id)}
          </span>
        </div>

        <time
          dateTime={task.updated_at}
          title={new Date(task.updated_at).toLocaleString()}
          className="text-xs text-muted-foreground whitespace-nowrap"
        >
          {t(relativeTime(task.updated_at))}
        </time>
      </div>

      {/* Title & Summary */}
      <div className="space-y-1">
        <Link
          to={targetUrl}
          state={{ taskReturnTo: currentReturnTo }}
          onClick={
            onClick
              ? (e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  onClick(task)
                }
              : undefined
          }
          className="block group-hover:text-primary transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
        >
          <h4 className="text-sm font-semibold text-foreground break-words leading-snug">
            {task.title}
          </h4>
        </Link>
        {summary && (
          <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed">
            {summary}
          </p>
        )}
      </div>

      {/* Footer: Agent info & Action buttons */}
      <div className="flex flex-wrap items-center justify-between gap-2 pt-2 border-t border-border/50 text-xs">
        <div className="text-[11px] text-muted-foreground">
          {agent && (
            <span>
              {t('Agent')}: <strong>{agentDisplayName(agent)}</strong>
            </span>
          )}
        </div>

        {showActions && (
          <div className="flex items-center gap-2 ml-auto">
            <Link
              to={`${targetUrl}?view=detail`}
              state={{ taskReturnTo: currentReturnTo }}
              onClick={(e) => e.stopPropagation()}
              className="action-secondary text-xs px-2.5 py-1 inline-flex items-center gap-1"
            >
              {t('Detail')}
            </Link>
            <Link
              to={targetUrl}
              state={{ taskReturnTo: currentReturnTo }}
              onClick={
                onClick
                  ? (e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      onClick(task)
                    }
                  : undefined
              }
              className="action-primary text-xs px-3 py-1 inline-flex items-center gap-1.5"
            >
              {isRunning ? (
                <>
                  <Terminal className="h-3.5 w-3.5" />
                  <span>{t('Live Terminal')}</span>
                </>
              ) : status === 'Ready for review' ||
                status === 'Completed' ||
                status === 'Answered' ? (
                <>
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  <span>{t('View result')}</span>
                </>
              ) : (
                <>
                  <Play className="h-3.5 w-3.5" />
                  <span>{t('Start session')}</span>
                </>
              )}
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}
