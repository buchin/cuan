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
        'w-full max-w-xl rounded-xl border border-border/40 bg-card/40 dark:bg-card/20 p-3.5 sm:p-4 text-card-foreground transition-all',
        isDone
          ? 'border-l-2 border-l-emerald-500/70 border-emerald-500/20'
          : 'border-l-2 border-l-purple-500/70 border-purple-500/20',
        className
      )}
    >
      {/* Top Header Badge & Timestamp */}
      <div className="flex items-center justify-between gap-3 flex-wrap border-b border-border/40 pb-2.5">
        <div className="flex items-center gap-2">
          <span
            data-testid="completion-card-status"
            className={cn(
              'inline-flex items-center gap-1.5 text-xs font-medium',
              isDone
                ? 'text-emerald-600 dark:text-emerald-400'
                : 'text-purple-600 dark:text-purple-400'
            )}
          >
            {isDone ? (
              <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
            ) : (
              <ClipboardCheck className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400" />
            )}
            <span>{isDone ? t('Tugas Selesai') : t('Siap Direviu')}</span>
          </span>

          <span className="text-xs font-mono font-medium text-muted-foreground">
            {taskRef(task.id)}
          </span>
        </div>

        <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <Clock className="h-3 w-3 shrink-0" />
          <span>{relativeTime(task.updated_at || task.created_at)}</span>
        </div>
      </div>

      {/* Main Body: Title & Business Summary */}
      <div className="mt-3 space-y-2">
        <h4
          data-testid="completion-card-title"
          className="text-sm sm:text-base font-semibold text-foreground leading-snug"
        >
          {task.title}
        </h4>

        <div
          data-testid="completion-card-summary"
          className="text-xs sm:text-sm text-foreground/90 leading-relaxed space-y-1"
        >
          <div className="text-[11px] font-medium text-muted-foreground/80 flex items-center gap-1">
            <Sparkles className="h-3 w-3 text-muted-foreground/70" />
            <span>{t('Ringkasan Hasil')}</span>
          </div>
          <div className="prose prose-xs dark:prose-invert max-w-none break-words">
            <ChatMarkdown content={businessSummary} />
          </div>
        </div>

        {/* Criteria / Definition of Done Checklist if available */}
        {dodItems.length > 0 && (
          <div className="mt-3 pt-2.5 border-t border-border/30 text-xs space-y-1.5">
            <div className="flex items-center justify-between text-[11px] font-medium text-muted-foreground uppercase tracking-wide">
              <span>{t('Kriteria Selesai (DoD)')}</span>
              <span className="text-[11px] font-mono text-muted-foreground">
                {dodItems.filter((i) => i.completed).length}/{dodItems.length} {t('Terpenuhi')}
              </span>
            </div>
            <ul className="space-y-1.5 pt-0.5">
              {dodItems.map((item, idx) => (
                <li key={idx} className="flex items-start gap-2.5 text-foreground/85 leading-normal">
                  <span
                    className={cn(
                      'mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded text-[10px]',
                      item.completed
                        ? 'text-emerald-600 dark:text-emerald-400 font-bold'
                        : 'text-muted-foreground/50'
                    )}
                  >
                    {item.completed ? '✓' : '·'}
                  </span>
                  <span className={cn('break-words', item.completed && 'font-normal text-foreground')}>
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
      <div className="mt-3.5 flex flex-wrap items-center justify-end gap-2 border-t border-border/40 pt-2.5">
        <button
          type="button"
          data-testid="completion-card-chat-btn"
          onClick={handleContinueChat}
          className="min-h-8 px-2.5 py-1 text-xs font-medium text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 rounded-md hover:bg-muted/40 transition-colors cursor-pointer"
        >
          <MessageSquare className="h-3.5 w-3.5" />
          <span>{t('Lanjutkan Diskusi')}</span>
        </button>

        {onViewDetails ? (
          <button
            type="button"
            data-testid="completion-card-action-btn"
            onClick={() => onViewDetails(task)}
            className="min-h-8 px-2.5 py-1 text-xs font-medium text-muted-foreground hover:text-foreground inline-flex items-center gap-1 rounded-md hover:bg-muted/40 transition-colors cursor-pointer group"
          >
            <span>{t('Buka Detail Tugas')}</span>
            <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/70 group-hover:translate-x-0.5 transition-transform" />
          </button>
        ) : (
          <Link
            to={taskDetailPath}
            data-testid="completion-card-action-btn"
            className="min-h-8 px-2.5 py-1 text-xs font-medium text-muted-foreground hover:text-foreground inline-flex items-center gap-1 rounded-md hover:bg-muted/40 transition-colors group"
          >
            <span>{t('Buka Detail Tugas')}</span>
            <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/70 group-hover:translate-x-0.5 transition-transform" />
          </Link>
        )}
      </div>
    </div>
  )
}
