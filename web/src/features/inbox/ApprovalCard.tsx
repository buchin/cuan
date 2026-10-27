import { useLanguage } from '../../context/LanguageContext'
import { Button } from '../../components/ui/button'
import { ChatMarkdown } from './ChatMarkdown'
import { cn } from '../../lib/utils'
import {
  ClipboardCheck,
  CheckCircle2,
  CheckCheck,
  Clock3,
  Folder,
  Pencil,
  Play,
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

  return (
    <div
      role="region"
      aria-label={`${t('Needs approval')}: ${plan.title}`}
      data-testid="approval-card"
      data-status={accepted ? 'accepted' : deferred ? 'deferred' : 'pending'}
      className={cn(
        'approval-card mt-3 rounded-xl border p-3.5 sm:p-4 space-y-3 shadow-xs transition-all duration-200 min-w-0 max-w-full overflow-hidden',
        accepted
          ? 'border-emerald-500/30 bg-emerald-500/[0.04] dark:bg-emerald-500/[0.08] border-l-4 border-l-emerald-500 dark:border-l-emerald-400'
          : deferred
            ? 'border-border/60 bg-muted/20 border-l-4 border-l-muted-foreground/60'
            : 'border-primary/25 bg-background/80 dark:bg-card/70 border-l-4 border-l-primary shadow-xs',
        className,
      )}
    >
      {/* Header Badge & Project Tag */}
      <div className="flex items-center justify-between gap-2 border-b border-border/50 pb-2">
        <div className="flex items-center gap-1.5 min-w-0">
          {accepted ? (
            <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
          ) : deferred ? (
            <Clock3 className="h-4 w-4 text-muted-foreground shrink-0" />
          ) : (
            <ClipboardCheck className="h-4 w-4 text-primary shrink-0" />
          )}
          <span
            className={cn(
              'text-xs font-semibold tracking-wide uppercase truncate',
              accepted
                ? 'text-emerald-700 dark:text-emerald-300'
                : deferred
                  ? 'text-muted-foreground'
                  : 'text-primary',
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
          <ChatMarkdown content={plan.description} className="text-sm" />
        </div>
      )}

      {/* Action Buttons (Touch target min-h-11, mobile responsive wrap) */}
      <div className="flex flex-wrap items-center gap-2 pt-1 border-t border-border/40">
        <Button
          size="sm"
          className={cn(
            'min-h-11 font-semibold text-xs px-3.5 py-2 gap-1.5 cursor-pointer shadow-xs',
            accepted &&
              'bg-emerald-600 dark:bg-emerald-500 hover:bg-emerald-600 dark:hover:bg-emerald-500 opacity-90 text-white cursor-default',
          )}
          disabled={accepted || disabled}
          onClick={onApprove}
        >
          {accepted ? (
            <CheckCheck className="h-3.5 w-3.5" />
          ) : (
            <Play className="h-3.5 w-3.5 fill-current" />
          )}
          <span>{t(accepted ? 'Approved' : 'Approve and start')}</span>
        </Button>

        {!accepted && (
          <>
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
          </>
        )}
      </div>
    </div>
  )
}
