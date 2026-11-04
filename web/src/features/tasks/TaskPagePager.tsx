import { ChevronLeft, ChevronRight } from 'lucide-react'
import { IconButton } from '../../components/ui/IconButton'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'

export function TaskPagePager({ page, pageSize, total, onPage, className }: { page: number; pageSize: number; total: number; onPage: (page: number) => void; className?: string }) {
  const { t } = useLanguage()
  if (total <= pageSize) return null
  const last = Math.max(1, Math.ceil(total / pageSize))
  return <div className={cn('flex min-h-11 items-center justify-between gap-3 text-xs text-muted-foreground', className)}>
    <span className="tabular-nums">{(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} / {total}</span>
    <div className="flex items-center gap-1">
      <IconButton label={t('Previous page')} disabled={page <= 1} onClick={() => onPage(page - 1)}><ChevronLeft /></IconButton>
      <IconButton label={t('Next page')} disabled={page >= last} onClick={() => onPage(page + 1)}><ChevronRight /></IconButton>
    </div>
  </div>
}
