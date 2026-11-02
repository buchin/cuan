import { useState, useMemo, useEffect, memo } from 'react'
import { Link, useNavigate } from 'react-router-dom'
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
  Terminal,
  RefreshCw,
  Send,
} from 'lucide-react'
import type { Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import { taskRef, agentDisplayName, relativeTime, taskTerminalUrl, taskDetailUrl } from '../tasks/task-ui'
import { ChatMarkdown } from './ChatMarkdown'
import { parseTaskReviewEnvelope } from './task-envelope'

export interface TaskCompletionCardProps {
  task: Task
  conversationId?: string
  onContinueChat?: (suggestedText: string) => void
  onUseDraft?: (draftText: string) => void
  onViewDetails?: (task: Task, tab?: 'details' | 'terminal') => void
  onOpenTerminal?: (task: Task) => void
  onRetry?: (task: Task) => Promise<void> | void
  className?: string
  isCustomerRoom?: boolean
}

export const TaskCompletionCard = memo(function TaskCompletionCard({
  task,
  conversationId,
  onContinueChat,
  onUseDraft,
  onViewDetails,
  onOpenTerminal,
  onRetry,
  className,
  isCustomerRoom = false,
}: TaskCompletionCardProps) {
  const { t } = useLanguage()
  const navigate = useNavigate()

  const [currentTask, setCurrentTask] = useState<Task>(task)
  const [retrying, setRetrying] = useState(false)

  useEffect(() => {
    setCurrentTask(task)
  }, [task])

  const isDone = (currentTask.status || '').toLowerCase().trim() === 'done'

  // Parse dual-channel output envelope & fallback
  const envelopeResult = useMemo(() => {
    return parseTaskReviewEnvelope(currentTask)
  }, [currentTask])

  // Parse DoD items for metadata count and SlideDrawer handoff
  const dodItems = useMemo(() => {
    if (!currentTask.dod_checklist || !Array.isArray(currentTask.dod_checklist)) return []
    return currentTask.dod_checklist.map((item) => {
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
  }, [currentTask.dod_checklist])

  const agentName = agentDisplayName(
    currentTask.latest_attempt?.agent_name || currentTask.assignee || 'cuan-runner'
  )

  const taskDetailPath = `/tasks/${encodeURIComponent(currentTask.id)}${
    conversationId ? `?returnTo=${encodeURIComponent(`/inbox/${conversationId}`)}` : ''
  }`

  const handleContinueChat = () => {
    const ref = taskRef(currentTask.id)
    const suggested = isDone
      ? t(`Terima kasih. Terkait tugas ${ref} (${currentTask.title}), tolong bantu `)
      : t(`Saya ingin mereviu hasil tugas ${ref} (${currentTask.title}): `)
    if (onContinueChat) {
      onContinueChat(suggested)
    }
  }

  const handleUseDraft = () => {
    const draftText = envelopeResult.draft
    if (!draftText) return
    if (onUseDraft) {
      onUseDraft(draftText)
    } else if (onContinueChat) {
      onContinueChat(draftText)
    }
  }

  const handleOpenTerminal = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (onOpenTerminal) {
      onOpenTerminal(currentTask)
    } else if (onViewDetails) {
      onViewDetails(currentTask, 'terminal')
    } else {
      navigate(taskTerminalUrl(currentTask))
    }
  }

  const handleRetry = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (retrying) return
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
    } catch {
      // ignore retry error
    } finally {
      setRetrying(false)
    }
  }

  const handleViewDetails = (tab: 'details' | 'terminal' = 'details') => {
    if (onViewDetails) {
      onViewDetails(currentTask, tab)
    } else {
      navigate(taskDetailUrl(currentTask))
    }
  }

  return (
    <div
      data-testid="task-completion-card"
      role="region"
      aria-label={isDone ? t('Kartu Tugas Selesai') : t('Kartu Tugas Siap Reviu')}
      className={cn(
        'w-full max-w-xl rounded-xl border border-border/40 bg-card/40 dark:bg-card/20 p-3.5 sm:p-4 text-card-foreground transition-all shadow-xs space-y-3',
        isDone
          ? 'border-l-2 border-l-emerald-500/70 border-emerald-500/20'
          : 'border-l-2 border-l-purple-500/70 border-purple-500/20',
        className
      )}
    >
      {/* Top Header: Calm Status, TaskRef, Live Terminal & Retry Buttons */}
      <div className="flex items-center justify-between gap-2 flex-wrap border-b border-border/40 pb-2.5">
        <div className="flex items-center gap-2 min-w-0">
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
              <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
            ) : (
              <ClipboardCheck className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400 shrink-0" />
            )}
            <span>{isDone ? t('Tugas Selesai') : t('Siap Direviu')}</span>
          </span>

          <span className="text-xs font-mono font-medium text-muted-foreground">
            {taskRef(currentTask.id)}
          </span>

          <div className="hidden sm:flex items-center gap-1 text-[11px] text-muted-foreground font-mono">
            <Clock className="h-3 w-3 shrink-0" />
            <span>{relativeTime(currentTask.updated_at || currentTask.created_at)}</span>
          </div>
        </div>

        {/* Actionable Controls: Live Terminal & Retry */}
        <div className="flex items-center gap-1.5 shrink-0">
          <button
            type="button"
            onClick={handleOpenTerminal}
            data-testid="task-card-terminal-btn"
            className="min-h-[32px] px-2.5 py-1 text-xs font-medium rounded-lg inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors cursor-pointer border border-border/40"
            title={t('Live Terminal')}
            aria-label={t('Live Terminal')}
          >
            <Terminal className="h-3.5 w-3.5 text-emerald-500 shrink-0" />
            <span className="hidden xs:inline sm:inline">{t('Live Terminal')}</span>
          </button>

          <button
            type="button"
            disabled={retrying}
            onClick={handleRetry}
            data-testid="task-card-retry-btn"
            className="min-h-[32px] px-2.5 py-1 text-xs font-medium rounded-lg inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors cursor-pointer border border-border/40 disabled:opacity-50"
            title={t('Coba Lagi')}
            aria-label={t('Coba Lagi')}
          >
            <RefreshCw className={cn('h-3.5 w-3.5 shrink-0', retrying && 'animate-spin text-primary')} />
            <span className="hidden xs:inline sm:inline">{retrying ? t('Retrying...') : t('Coba Lagi')}</span>
          </button>
        </div>
      </div>

      {/* Main Body: Title & Business Summary */}
      <div className="space-y-2">
        <h4
          data-testid="completion-card-title"
          className="text-sm sm:text-base font-semibold text-foreground leading-snug break-words"
        >
          {currentTask.title}
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
            <ChatMarkdown content={envelopeResult.summary} />
          </div>
        </div>

        {/* 1-Click Actionable Visitor Reply Draft if customer room and draft exists */}
        {isCustomerRoom && envelopeResult.draft && (
          <div
            data-testid="task-card-draft-section"
            className="border-t border-border/30 pt-2.5 space-y-2"
          >
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-1.5 font-semibold text-xs text-primary">
                <Sparkles className="h-3.5 w-3.5 text-primary shrink-0" />
                <span>{t('Draf Balasan Pengunjung')}</span>
              </div>
              <button
                type="button"
                onClick={handleUseDraft}
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

        {/* Secondary Technical Details link: parks raw DoD checklist to SlideDrawer */}
        <div className="pt-2 flex flex-wrap items-center justify-between gap-2 border-t border-border/30 text-xs">
          <button
            type="button"
            onClick={() => handleViewDetails('details')}
            data-testid="task-card-diagnostic-link"
            className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors group cursor-pointer"
          >
            <span>{t('Detail Diagnostik & Status Deploy →')}</span>
          </button>

          {/* Accessibility & test requirement compatibility: keep DoD criteria count */}
          {dodItems.length > 0 && (
            <span className="text-[11px] font-mono text-muted-foreground/80">
              {t('Kriteria Selesai (DoD)')}: {dodItems.filter((i) => i.completed).length}/{dodItems.length} {t('Terpenuhi')}
            </span>
          )}
        </div>

        {/* Meta Row: Branch / PR / Worker Info */}
        <div className="flex flex-wrap items-center gap-2 pt-1 text-[11px] text-muted-foreground">
          <span className="inline-flex items-center gap-1 rounded-md bg-muted/60 px-2 py-0.5 font-medium">
            <Bot className="h-3 w-3 text-primary" />
            <span>{agentName}</span>
          </span>

          {currentTask.branch_name && (
            <span className="inline-flex items-center gap-1 rounded-md bg-muted/60 px-2 py-0.5 font-mono text-[10px]">
              <GitBranch className="h-3 w-3 text-muted-foreground" />
              <span>{currentTask.branch_name}</span>
            </span>
          )}

          {currentTask.pr_url && (
            <a
              href={currentTask.pr_url}
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
      <div className="flex flex-wrap items-center justify-end gap-2 border-t border-border/40 pt-2.5">
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
            onClick={() => handleViewDetails('details')}
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
})
