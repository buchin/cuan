import { useState } from 'react'
import { useLanguage } from '../../context/LanguageContext'
import { Button } from '../../components/ui/button'
import { ChatMarkdown } from './ChatMarkdown'
import { cn } from '../../lib/utils'
import {
  ClipboardCheck,
  CheckCircle2,
  Clock3,
  Folder,
  Pencil,
  Play,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'
import type { ApprovalPlan } from '../../api/chat'

export type { ApprovalPlan }

export interface ApprovalCardProps {
  plan: ApprovalPlan
  accepted?: boolean
  deferred?: boolean
  disabled?: boolean
  onApprove?: () => void
  onChangePlan?: () => void
  onDefer?: () => void
  className?: string
}

export function FormattedTitle({ title }: { title: string }) {
  if (!title) return null
  const parts = title.split(/(`[^`]+`)/g)
  return (
    <h4 className="text-sm font-semibold tracking-tight text-foreground break-words leading-snug">
      {parts.map((part, index) => {
        if (part.startsWith('`') && part.endsWith('`') && part.length > 2) {
          return (
            <code
              key={index}
              className="mx-0.5 rounded px-1.5 py-0.5 font-mono text-[11px] sm:text-xs bg-muted border border-border/60 text-foreground font-normal break-all align-baseline"
            >
              {part.slice(1, -1)}
            </code>
          )
        }
        return <span key={index}>{part}</span>
      })}
    </h4>
  )
}

export function ApprovalCard({
  plan,
  accepted = false,
  deferred = false,
  disabled = false,
  onApprove,
  onChangePlan,
  onDefer,
  className,
}: ApprovalCardProps) {
  const { t } = useLanguage()
  const [planExpanded, setPlanExpanded] = useState(false)
  const isLongPlan = !!plan.description && (plan.description.length > 250 || plan.description.split('\n').filter(Boolean).length > 4)

  return (
    <div
      role="region"
      aria-label={`${t('Needs approval')}: ${plan.title}`}
      data-testid="approval-card"
      data-status={accepted ? 'accepted' : deferred ? 'deferred' : 'pending'}
      className={cn(
        'approval-card mt-3 border-0 border-l-2 pl-3.5 sm:pl-4 py-0.5 space-y-3 bg-transparent shadow-none transition-all duration-200 min-w-0 max-w-full overflow-hidden',
        accepted
          ? 'border-l-2 border-l-emerald-500'
          : deferred
            ? 'border-l-2 border-l-muted-foreground'
            : 'border-l-2 border-l-primary',
        className,
      )}
    >
      {/* Header Badge & Project Tag */}
      <div className="flex items-center justify-between gap-2 border-b border-border/30 pb-2">
        <div className="flex items-center gap-1.5 min-w-0">
          {accepted ? (
            <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
          ) : deferred ? (
            <Clock3 className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
          ) : (
            <ClipboardCheck className="h-3.5 w-3.5 text-primary shrink-0" />
          )}
          <span
            className={cn(
              'text-xs font-medium tracking-normal truncate',
              accepted
                ? 'text-emerald-600 dark:text-emerald-400'
                : deferred
                  ? 'text-muted-foreground'
                  : 'text-primary font-semibold',
            )}
          >
            {t(
              accepted
                ? 'Approved'
                : deferred
                  ? 'Saved for later'
                  : 'Needs approval',
            )}
          </span>
        </div>

        {plan.project_name && (
          <span
            className="inline-flex items-center gap-1 text-[10px] font-mono font-medium px-2 py-0.5 rounded-full bg-secondary text-secondary-foreground shrink-0 max-w-[160px] truncate"
            title={plan.project_name}
          >
            <Folder className="h-2.5 w-2.5 opacity-70 shrink-0" />
            <span className="truncate">{plan.project_name}</span>
          </span>
        )}
      </div>

      {/* Plan Title */}
      <div className="approval-card-title">
        <FormattedTitle title={plan.title} />
      </div>

      {/* Plan Markdown Description (Bullet points, bold, inline code, code blocks) */}
      {plan.description && (
        <div className="approval-card-description min-w-0 max-w-full text-foreground/90">
          <div
            className={cn(
              'relative transition-all duration-200',
              !planExpanded && isLongPlan && 'max-h-[140px] overflow-hidden',
            )}
          >
            <ChatMarkdown content={plan.description} className="text-sm" />
            {!planExpanded && isLongPlan && (
              <div className="absolute inset-x-0 bottom-0 h-10 bg-gradient-to-t from-[var(--wa-incoming,transparent)] via-[var(--wa-incoming,transparent)]/60 to-transparent pointer-events-none" />
            )}
          </div>
          {isLongPlan && (
            <button
              type="button"
              onClick={() => setPlanExpanded(!planExpanded)}
              aria-expanded={planExpanded}
              data-testid="approval-card-toggle-plan"
              className="mt-1.5 inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline cursor-pointer"
            >
              <span>{planExpanded ? t('Hide plan details') : t('Show full plan')}</span>
              {planExpanded ? (
                <ChevronUp className="h-3 w-3" />
              ) : (
                <ChevronDown className="h-3 w-3" />
              )}
            </button>
          )}
        </div>
      )}

      {/* Action Buttons (Touch target min-h-11, mobile responsive wrap) */}
      {!accepted && (
        <div className="flex flex-wrap items-center gap-2 pt-2 border-t border-border/30">
          <Button
            size="sm"
            className="min-h-11 text-xs gap-1.5 font-semibold px-3.5 py-2 cursor-pointer shadow-xs bg-primary text-primary-foreground"
            disabled={disabled}
            onClick={onApprove}
          >
            <Play className="h-3.5 w-3.5 fill-current" />
            <span>{t('Approve and start')}</span>
          </Button>

          {onChangePlan && (
            <Button
              size="sm"
              variant="outline"
              className="min-h-11 font-medium text-xs px-3 py-2 gap-1.5 cursor-pointer"
              disabled={disabled}
              onClick={onChangePlan}
            >
              <Pencil className="h-3.5 w-3.5 opacity-70" />
              <span>{t('Change plan')}</span>
            </Button>
          )}
          {onDefer && (
            <Button
              size="sm"
              variant="ghost"
              className="min-h-11 font-medium text-xs px-3 py-2 gap-1.5 cursor-pointer text-muted-foreground hover:text-foreground"
              disabled={deferred || disabled}
              onClick={onDefer}
            >
              <Clock3 className="h-3.5 w-3.5 opacity-70" />
              <span>{t(deferred ? 'Saved for later' : 'Not now')}</span>
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
