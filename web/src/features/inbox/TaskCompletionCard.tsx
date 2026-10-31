import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import {
  CheckCircle2,
  ClipboardCheck,
  ExternalLink,
  MessageSquare,
  GitBranch,
  Bot,
  Clock,
  Sparkles,
  ChevronRight,
  Check,
} from 'lucide-react'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import { taskRef, agentDisplayName, relativeTime } from '../tasks/task-ui'
import { ChatMarkdown } from './ChatMarkdown'

export interface TaskCompletionCardProps {
  task: Task
  conversationId?: string
  onContinueChat?: (suggestedText: string) => void
  onViewDetails?: (task: Task) => void
  className?: string
}

export function TaskCompletionCard({
  task,
  conversationId,
  onContinueChat,
  onViewDetails,
  className,
}: TaskCompletionCardProps) {
  const { t } = useLanguage()

  const isDone = (task.status || '').toLowerCase().trim() === 'done'

  // Extract business-friendly summary (avoiding raw technical stack traces)
  const businessSummary = useMemo(() => {
    const raw = (task.summary || task.complete_note || task.description || '').trim()
    if (!raw) {
      return isDone
        ? t('Instruksi tugas berhasil diselesaikan dengan baik.')
        : t('Tugas telah selesai diproses dan siap untuk direviu.')
    }
    // Clean up if starts with redundant title
    return raw
  }, [task.summary, task.complete_note, task.description, isDone, t])

  // Parse DoD items
  const dodItems = useMemo(() => {
    if (!task.dod_checklist || !Array.isArray(task.dod_checklist)) return []
    return task.dod_checklist.map((item) => {
      if (typeof item === 'string') {
        const checked = item.startsWith('[x]') || item.startsWith('[X]') || item.startsWith('✓')
        const text = item.replace(/^(\[[ xX]\]|✓)\s*/, '').trim()
        return { text, completed: checked }
      }
      return {
        text: item.text,
        completed: !!item.completed,
      }
    })
  }, [task.dod_checklist])

  const agentName = agentDisplayName(
    task.latest_attempt?.agent_name || task.assignee || 'cuan-runner'
  )

  const taskDetailPath = `/tasks/${encodeURIComponent(task.id)}${
    conversationId ? `?returnTo=${encodeURIComponent(`/inbox/${conversationId}`)}` : ''
  }`

  const handleContinueChat = () => {
    const ref = taskRef(task.id)
    const suggested = isDone
      ? t(`Terima kasih. Terkait tugas ${ref} (${task.title}), tolong bantu `)
      : t(`Saya ingin mereviu hasil tugas ${ref} (${task.title}): `)
    if (onContinueChat) {
      onContinueChat(suggested)
    }
  }

  return (
    <div
      data-testid="task-completion-card"
      role="region"
      aria-label={isDone ? t('Kartu Tugas Selesai') : t('Kartu Tugas Siap Reviu')}
      className={cn(
        'w-full max-w-xl rounded-2xl border p-4 sm:p-5 shadow-sm transition-all text-card-foreground',
        isDone
          ? 'border-emerald-500/30 bg-emerald-500/5 dark:bg-emerald-950/20'
          : 'border-purple-500/30 bg-purple-500/5 dark:bg-purple-950/20',
        className
      )}
    >
      {/* Top Header Badge & Timestamp */}
      <div className="flex items-center justify-between gap-3 flex-wrap border-b border-border/60 pb-3">
        <div className="flex items-center gap-2">
          <span
            data-testid="completion-card-status"
            className={cn(
              'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-bold tracking-wide uppercase border',
              isDone
                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30'
                : 'bg-purple-500/15 text-purple-700 dark:text-purple-300 border-purple-500/30'
            )}
          >
            {isDone ? (
              <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
            ) : (
              <ClipboardCheck className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400" />
            )}
            <span>{isDone ? t('Tugas Selesai') : t('Siap Direviu')}</span>
          </span>

          <span className="text-xs font-mono font-semibold text-muted-foreground">
            {taskRef(task.id)}
          </span>
        </div>

        <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <Clock className="h-3 w-3 shrink-0" />
          <span>{relativeTime(task.updated_at || task.created_at)}</span>
        </div>
      </div>

      {/* Main Body: Title & Business Summary */}
      <div className="mt-3 space-y-2.5">
        <h4
          data-testid="completion-card-title"
          className="text-sm sm:text-base font-semibold text-foreground leading-snug"
        >
          {task.title}
        </h4>

        <div
          data-testid="completion-card-summary"
          className="rounded-xl bg-card border border-border/80 p-3 sm:p-3.5 text-xs sm:text-sm text-foreground/90 leading-relaxed shadow-xs"
        >
          <div className="text-xs font-medium text-muted-foreground mb-1.5 flex items-center gap-1">
            <Sparkles className="h-3 w-3 text-primary" />
            <span>{t('Ringkasan Hasil')}</span>
          </div>
          <div className="prose prose-xs dark:prose-invert max-w-none break-words">
            <ChatMarkdown content={businessSummary} />
          </div>
        </div>

        {/* Criteria / Definition of Done Checklist if available */}
        {dodItems.length > 0 && (
          <div className="mt-2 rounded-xl bg-background/60 border border-border/60 p-3 text-xs space-y-1.5">
            <div className="flex items-center justify-between text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">
              <span>{t('Kriteria Selesai (DoD)')}</span>
              <span className="text-emerald-600 dark:text-emerald-400">
                {dodItems.filter((i) => i.completed).length}/{dodItems.length} {t('Terpenuhi')}
              </span>
            </div>
            <ul className="space-y-1 pt-1">
              {dodItems.map((item, idx) => (
                <li key={idx} className="flex items-start gap-2 text-foreground/80">
                  <span
                    className={cn(
                      'mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full text-[9px]',
                      item.completed
                        ? 'bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 font-bold'
                        : 'bg-muted text-muted-foreground'
                    )}
                  >
                    {item.completed ? <Check className="h-2.5 w-2.5" /> : '·'}
                  </span>
                  <span className={cn('break-words', item.completed && 'font-medium text-foreground')}>
                    {item.text}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Meta Row: Branch / PR / Worker Info */}
        <div className="flex flex-wrap items-center gap-2 pt-1 text-[11px] text-muted-foreground">
          <span className="inline-flex items-center gap-1 rounded-md bg-muted/60 px-2 py-0.5 font-medium">
            <Bot className="h-3 w-3 text-primary" />
            <span>{agentName}</span>
          </span>

          {task.branch_name && (
            <span className="inline-flex items-center gap-1 rounded-md bg-muted/60 px-2 py-0.5 font-mono text-[10px]">
              <GitBranch className="h-3 w-3 text-muted-foreground" />
              <span>{task.branch_name}</span>
            </span>
          )}

          {task.pr_url && (
            <a
              href={task.pr_url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-primary hover:underline font-medium"
            >
              <span>Pull Request</span>
              <ExternalLink className="h-3 w-3" />
            </a>
          )}
        </div>
      </div>

      {/* Actionable Quick Action Buttons */}
      <div className="mt-4 flex flex-wrap items-center justify-end gap-2 border-t border-border/60 pt-3">
        <button
          type="button"
          data-testid="completion-card-chat-btn"
          onClick={handleContinueChat}
          className="action-secondary min-h-9 px-3 py-1.5 text-xs inline-flex items-center gap-1.5 rounded-lg cursor-pointer"
        >
          <MessageSquare className="h-3.5 w-3.5" />
          <span>{t('Lanjutkan Diskusi')}</span>
        </button>

        {onViewDetails ? (
          <button
            type="button"
            data-testid="completion-card-action-btn"
            onClick={() => onViewDetails(task)}
            className="action-primary min-h-9 px-3.5 py-1.5 text-xs inline-flex items-center gap-1.5 rounded-lg cursor-pointer"
          >
            <span>{t('Buka Detail Tugas')}</span>
            <ChevronRight className="h-3.5 w-3.5" />
          </button>
        ) : (
          <Link
            to={taskDetailPath}
            data-testid="completion-card-action-btn"
            className="action-primary min-h-9 px-3.5 py-1.5 text-xs inline-flex items-center gap-1.5 rounded-lg"
          >
            <span>{t('Buka Detail Tugas')}</span>
            <ChevronRight className="h-3.5 w-3.5" />
          </Link>
        )}
      </div>
    </div>
  )
}
