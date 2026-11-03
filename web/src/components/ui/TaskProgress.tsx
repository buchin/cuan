import { CheckCircle2 } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'

/** Task counts, never execution-time estimates. Unknown and empty scopes stay quiet. */
export function TaskProgress({ total, done, className }: {
  total?: number
  done?: number
  className?: string
}) {
  const { t, language } = useLanguage()
  if (typeof total !== 'number' || typeof done !== 'number' || !Number.isSafeInteger(total) || !Number.isSafeInteger(done) || total <= 0 || done < 0 || done > total) return null
  const complete = done === total
  const format = new Intl.NumberFormat(language)
  const label = t('{{done}} of {{total}} tasks completed')
    .replace('{{done}}', format.format(done)).replace('{{total}}', format.format(total))
  const fraction = done / total
  const angle = fraction * Math.PI * 2
  const x = 8 + 4.5 * Math.sin(angle)
  const y = 8 - 4.5 * Math.cos(angle)
  return (
    <span role="img" aria-label={label} title={label} data-task-progress={complete ? 'complete' : 'partial'}
      className={cn('inline-flex h-4 w-4 shrink-0 align-middle', complete ? 'text-[color:var(--status-success)]' : done > 0 ? 'text-[color:var(--status-working)]' : 'text-muted-foreground', className)}>
      {complete ? <CheckCircle2 aria-hidden="true" className="h-full w-full" strokeWidth={1.75} /> : (
        <svg aria-hidden="true" viewBox="0 0 16 16" className="h-full w-full">
          <circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" strokeWidth="1.5" opacity="0.65" />
          {done > 0 && <path d={`M 8 8 L 8 3.5 A 4.5 4.5 0 ${fraction > 0.5 ? 1 : 0} 1 ${x} ${y} Z`} fill="currentColor" />}
        </svg>
      )}
    </span>
  )
}
