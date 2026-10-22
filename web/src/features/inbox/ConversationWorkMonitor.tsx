import { useLanguage } from '../../context/LanguageContext'
import { Link, useLocation } from 'react-router-dom'
import {
  CheckSquare,
  Terminal,
  EyeOff,
  Play,
  CheckCircle2,
  Plus,
  RefreshCw,
  Folder,
} from 'lucide-react'
import type { Task } from '../../api/types'
import type { ChatContact } from '../../api/chat'
import { cn } from '../../lib/utils'

export function ConversationWorkMonitor({
  tasks,
  activeContact: _activeContact,
  onRefresh,
  onNewTask,
  onBackToChat,
  loading = false,
}: {
  tasks: Task[]
  activeContact: ChatContact
  onRefresh: () => void
  onNewTask: () => void
  onBackToChat: () => void
  loading?: boolean
}) {
  const { t } = useLanguage()
  const location = useLocation()

  const activeCount = tasks.filter(
    (t) =>
      ['doing', 'review'].includes(t.status) ||
      ['running', 'queued'].includes(t.latest_attempt?.status || '')
  ).length
  const doneCount = tasks.filter((t) => t.status === 'done').length

  return (
    <div className="flex h-full min-h-0 flex-1 flex-col overflow-y-auto bg-background p-3 sm:p-5 space-y-4">
      {/* Top Banner */}
      <div className="rounded-xl border border-border bg-card p-4 shadow-sm text-card-foreground">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <h3 className="text-base font-semibold text-foreground">
                {t('Work Monitor')}
              </h3>
              <span className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground border border-border">
                <EyeOff className="h-3 w-3" />
                {t('Internal · Invisible to visitor')}
              </span>
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              {t(
                'Tasks and escalated work for this conversation are handled internally by developers or AI agents without displaying bubbles to the visitor.'
              )}
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <button
              type="button"
              onClick={onRefresh}
              title={t('Refresh tasks')}
              className="action-secondary text-xs px-2.5 py-1.5 inline-flex items-center gap-1"
            >
              <RefreshCw className={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
              <span>{t('Refresh')}</span>
            </button>
            <button
              type="button"
              onClick={onNewTask}
              className="action-primary text-xs px-3 py-1.5 inline-flex items-center gap-1"
            >
              <Plus className="h-3.5 w-3.5" />
              <span>{t('New task')}</span>
            </button>
          </div>
        </div>

        {/* Stats Row */}
        <div className="mt-3 flex flex-wrap items-center gap-2 pt-3 border-t border-border/60 text-xs">
          <span className="rounded-md bg-muted/60 px-2.5 py-1 font-medium text-muted-foreground">
            {t('Total')}: <strong className="text-foreground">{tasks.length}</strong>
          </span>
          <span className="rounded-md bg-blue-500/10 px-2.5 py-1 font-medium text-blue-600 dark:text-blue-400">
            {t('Active')}: <strong>{activeCount}</strong>
          </span>
          <span className="rounded-md bg-emerald-500/10 px-2.5 py-1 font-medium text-emerald-600 dark:text-emerald-400">
            {t('Completed')}: <strong>{doneCount}</strong>
          </span>
        </div>
      </div>

      {/* Task List or Empty State */}
      {tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-8 text-center text-muted-foreground space-y-3 my-auto">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <CheckSquare className="h-6 w-6 text-muted-foreground" />
          </div>
          <div className="max-w-sm space-y-1">
            <h4 className="text-sm font-semibold text-foreground">
              {t('No tasks connected to this conversation')}
            </h4>
            <p className="text-xs text-muted-foreground">
              {t(
                'When a user asks for bug fixes or features, escalate their message into a task. It will appear here for dev execution without showing bubbles to the customer.'
              )}
            </p>
          </div>
          <div className="flex items-center gap-2 pt-2">
            <button
              type="button"
              onClick={onNewTask}
              className="action-primary text-xs px-3 py-1.5 inline-flex items-center gap-1"
            >
              <Plus className="h-3.5 w-3.5" />
              <span>{t('Create task for this chat')}</span>
            </button>
            <button
              type="button"
              onClick={onBackToChat}
              className="action-secondary text-xs px-3 py-1.5 inline-flex items-center gap-1"
            >
              <span>{t('Back to chat')}</span>
            </button>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {tasks.map((task) => {
            const isRunning =
              task.latest_attempt &&
              ['running', 'queued'].includes(task.latest_attempt.status)
            const targetUrl = task.project_id
              ? `/projects/${encodeURIComponent(task.project_id)}/tasks/${encodeURIComponent(task.id)}`
              : `/tasks/${task.id}`

            return (
              <div
                key={task.id}
                className="rounded-xl border border-border bg-card p-4 text-card-foreground shadow-sm transition-all hover:border-border/80 space-y-3"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span
                        className={cn(
                          'inline-flex items-center gap-1 text-[11px] font-semibold px-2 py-0.5 rounded-full border',
                          task.status === 'done'
                            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                            : task.status === 'doing'
                              ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
                              : task.status === 'review'
                                ? 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20'
                                : task.status === 'blocked'
                                  ? 'bg-destructive/10 text-destructive border-destructive/20'
                                  : 'bg-muted text-muted-foreground border-border'
                        )}
                      >
                        {isRunning && (
                          <span className="h-1.5 w-1.5 rounded-full bg-blue-500 animate-ping" />
                        )}
                        {task.status === 'doing'
                          ? t('Work in progress')
                          : task.status === 'done'
                            ? t('Completed')
                            : task.status === 'blocked'
                              ? t('Needs attention')
                              : task.status === 'review'
                                ? t('Result ready')
                                : t('Ready')}
                      </span>
                      {task.project_name && (
                        <span className="inline-flex items-center gap-1 text-xs text-muted-foreground font-medium">
                          <Folder className="h-3.5 w-3.5" />
                          {task.project_name}
                        </span>
                      )}
                    </div>
                    <h4 className="text-sm font-semibold text-foreground break-words pt-1">
                      {task.title}
                    </h4>
                    {task.summary && (
                      <p className="text-xs text-muted-foreground line-clamp-2">
                        {task.summary}
                      </p>
                    )}
                  </div>
                </div>

                {/* Footer with actions */}
                <div className="flex flex-wrap items-center justify-between gap-2 pt-2 border-t border-border/50 text-xs">
                  <div className="text-[11px] text-muted-foreground">
                    {task.latest_attempt?.agent_name && (
                      <span>
                        {t('Agent')}:{' '}
                        <strong>{task.latest_attempt.agent_name}</strong>
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2 ml-auto">
                    <Link
                      to={`${targetUrl}?view=detail`}
                      state={{ taskReturnTo: location.pathname }}
                      className="action-secondary text-xs px-2.5 py-1 inline-flex items-center gap-1"
                    >
                      {t('Detail')}
                    </Link>
                    <Link
                      to={targetUrl}
                      state={{ taskReturnTo: location.pathname }}
                      className="action-primary text-xs px-3 py-1 inline-flex items-center gap-1.5"
                    >
                      {isRunning ? (
                        <>
                          <Terminal className="h-3.5 w-3.5" />
                          <span>{t('Live Terminal')}</span>
                        </>
                      ) : task.status === 'review' || task.status === 'done' ? (
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
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
