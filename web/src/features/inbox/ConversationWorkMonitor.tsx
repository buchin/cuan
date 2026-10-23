import { useLanguage } from '../../context/LanguageContext'
import {
  CheckSquare,
  EyeOff,
  Plus,
  RefreshCw,
} from 'lucide-react'
import type { Task } from '../../api/types'
import type { ChatContact } from '../../api/chat'
import { cn } from '../../lib/utils'
import { TaskItemCard } from '../tasks/TaskItemCard'

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
                {t('Tasks')}
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
          {tasks.map((task) => (
            <TaskItemCard key={task.id} task={task} />
          ))}
        </div>
      )}
    </div>
  )
}
