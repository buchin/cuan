import { useState, useRef, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Sparkles, Brain, X, ArrowUpRight } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { type GroundingTaste } from '../../api/taste'
import { cn } from '../../lib/utils'

export interface TasteGroundingChipsProps {
  tastes?: GroundingTaste[]
  maxVisible?: number
  className?: string
  messageId?: string
  returnTo?: string
}

export function TasteGroundingChips({
  tastes,
  maxVisible = 2,
  className,
  returnTo = '/chat',
}: TasteGroundingChipsProps) {
  const { t } = useLanguage()
  const [isOpen, setIsOpen] = useState(false)
  const [selectedTasteId, setSelectedTasteId] = useState<string | null>(null)
  const popoverRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // Close popover when clicking outside or pressing Escape
  useEffect(() => {
    if (!isOpen) return

    const handleClickOutside = (e: MouseEvent) => {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node) &&
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false)
      }
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen])

  if (!tastes || tastes.length === 0) {
    return null
  }

  // Guardrail anti-clutter: Show at most maxVisible (default 2) chips
  const visibleTastes = tastes.slice(0, maxVisible)
  const remainingCount = tastes.length - maxVisible

  return (
    <div
      ref={containerRef}
      className={cn('relative inline-flex flex-wrap items-center gap-1.5', className)}
      data-testid="taste-grounding-container"
      onClick={(e) => e.stopPropagation()}
    >
      {/* Primary Grounding Chips (Max 2) */}
      {visibleTastes.map((taste, idx) => {
        const topicName = taste.topic || t('Preferensi')
        return (
          <button
            key={taste.id || `taste-${idx}`}
            type="button"
            data-testid="taste-grounding-chip"
            data-topic={taste.topic}
            data-category={taste.category}
            onClick={(e) => {
              e.stopPropagation()
              setSelectedTasteId(taste.id)
              setIsOpen((prev) => (selectedTasteId === taste.id && prev ? false : true))
            }}
            title={taste.summary || taste.preview || taste.content || topicName}
            className={cn(
              'inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-medium transition-all cursor-pointer shadow-2xs max-w-[220px]',
              taste.category === 'technical'
                ? 'bg-cyan-500/10 text-cyan-900 dark:text-cyan-200 border border-cyan-500/25 hover:bg-cyan-500/20'
                : 'bg-amber-500/10 text-amber-900 dark:text-amber-200 border border-amber-500/25 hover:bg-amber-500/20'
            )}
          >
            <Sparkles
              className={cn(
                'h-3 w-3 shrink-0',
                taste.category === 'technical'
                  ? 'text-cyan-600 dark:text-cyan-400'
                  : 'text-amber-600 dark:text-amber-400'
              )}
              aria-hidden="true"
            />
            <span className="truncate">
              {t('Memori')}: {topicName}
            </span>
          </button>
        )
      })}

      {/* Clutter Guardrail Counter Badge: +N lainnya */}
      {remainingCount > 0 && (
        <button
          type="button"
          data-testid="taste-grounding-counter-badge"
          onClick={(e) => {
            e.stopPropagation()
            setSelectedTasteId(null)
            setIsOpen((prev) => !prev)
          }}
          aria-label={`${remainingCount} ${t('preferensi memori lainnya')}`}
          title={`${remainingCount} ${t('preferensi memori lainnya')}`}
          className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-semibold bg-muted/80 hover:bg-muted text-muted-foreground hover:text-foreground border border-border/60 transition-colors cursor-pointer shadow-2xs"
        >
          <span>+{remainingCount} {t('lainnya')}</span>
        </button>
      )}

      {/* 1-Click Context Popover */}
      {isOpen && (
        <div
          ref={popoverRef}
          role="dialog"
          aria-modal="true"
          aria-label={t('Detail Preferensi Aktif')}
          data-testid="taste-grounding-popover"
          className="absolute left-0 bottom-full mb-2 z-50 w-80 max-w-[calc(100vw-36px)] sm:w-96 rounded-xl border border-border bg-popover p-3.5 text-popover-foreground shadow-xl animate-in fade-in zoom-in-95 duration-150"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Header */}
          <div className="flex items-center justify-between gap-2 border-b border-border/50 pb-2">
            <div className="flex items-center gap-1.5 min-w-0">
              <Sparkles className="h-4 w-4 text-amber-500 shrink-0" aria-hidden="true" />
              <h3 className="font-semibold text-xs text-foreground truncate">
                {t('Landasan Memori (Taste Grounding)')}
              </h3>
            </div>
            <button
              type="button"
              data-testid="close-grounding-popover"
              onClick={(e) => {
                e.stopPropagation()
                setIsOpen(false)
              }}
              className="icon-action text-muted-foreground hover:text-foreground h-6 w-6 rounded-md flex items-center justify-center transition-colors"
              aria-label={t('Tutup')}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>

          <p className="mt-1.5 text-[11px] text-muted-foreground">
            {t('Preferensi aktif yang memandu respon asisten AI')}:
          </p>

          {/* List of active grounding preferences */}
          <div className="mt-2.5 max-h-56 overflow-y-auto space-y-2 pr-0.5">
            {tastes.map((taste, idx) => {
              const isSelected = selectedTasteId === taste.id
              const topicName = taste.topic || t('Preferensi')
              const snippet = taste.summary || taste.preview || taste.content || ''
              const isBrand = taste.category === 'brand'

              return (
                <div
                  key={taste.id || `popover-taste-${idx}`}
                  data-testid={`grounding-taste-item-${taste.id || idx}`}
                  className={cn(
                    'p-2.5 rounded-lg border transition-all text-xs space-y-1',
                    isSelected
                      ? 'bg-accent/40 border-primary/40 ring-1 ring-primary/30'
                      : 'bg-muted/30 border-border/50 hover:bg-muted/50'
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-semibold text-foreground truncate text-xs">
                      {topicName}
                    </span>
                    <span
                      data-testid="grounding-taste-category-badge"
                      className={cn(
                        'px-1.5 py-0.5 rounded text-[10px] font-semibold tracking-wider uppercase font-mono shrink-0',
                        isBrand
                          ? 'bg-purple-500/15 text-purple-700 dark:text-purple-300 border border-purple-500/30'
                          : 'bg-cyan-500/15 text-cyan-700 dark:text-cyan-300 border border-cyan-500/30'
                      )}
                    >
                      {isBrand ? t('Brand Taste') : t('Technical Taste')}
                    </span>
                  </div>
                  {snippet && (
                    <p className="text-[11px] text-muted-foreground leading-relaxed italic bg-background/50 p-1.5 rounded border border-border/30 break-words">
                      "{snippet}"
                    </p>
                  )}
                </div>
              )
            })}
          </div>

          {/* Footer & 1-Click Navigation to /settings/memory */}
          <div className="mt-3 pt-2.5 border-t border-border/50 flex items-center justify-between gap-2">
            <span className="text-[11px] text-muted-foreground truncate">
              {t('Sesuaikan memori agen?')}
            </span>
            <Link
              to={returnTo ? `/settings/memory?returnTo=${encodeURIComponent(returnTo)}` : '/settings/memory'}
              data-testid="taste-grounding-settings-link"
              onClick={() => setIsOpen(false)}
              className="inline-flex min-h-8 items-center gap-1.5 px-2.5 py-1 rounded-lg bg-primary text-primary-foreground text-xs font-semibold hover:bg-primary/90 transition-colors shrink-0 shadow-2xs"
            >
              <Brain className="h-3.5 w-3.5" aria-hidden="true" />
              <span>{t('Kelola Memori')}</span>
              <ArrowUpRight className="h-3 w-3" aria-hidden="true" />
            </Link>
          </div>
        </div>
      )}
    </div>
  )
}
