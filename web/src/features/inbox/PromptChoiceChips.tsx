import { useState } from 'react'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'
import type { PromptChoices, PromptOption } from '../../api/chat'
import { useLanguage } from '../../context/LanguageContext'
import { CardActions } from './CardActions'

export interface PromptChoiceChipsProps {
  prompt: PromptChoices
  disabled?: boolean
  onSelect: (option: PromptOption) => void
  onOther?: () => void
  className?: string
}

export function PromptChoiceChips({
  prompt,
  disabled = false,
  onSelect,
  onOther,
  className,
}: PromptChoiceChipsProps) {
  const { t } = useLanguage()
  const [selectedId, setSelectedId] = useState<string | null>(null)

  if (!prompt || !prompt.options || prompt.options.length === 0) {
    return null
  }

  const handleChoose = (opt: PromptOption) => {
    if (disabled || selectedId) return
    setSelectedId(opt.id)
    onSelect(opt)
  }

  return (
    <div
      role="group"
      aria-label="Pilihan tanggapan cepat"
      data-testid="prompt-choice-chips"
      className={cn('prompt-choice-container mt-2.5 space-y-2', className)}
    >
      {prompt.question && (
        <p className="text-xs font-medium text-muted-foreground/90 pl-0.5">
          {prompt.question}
        </p>
      )}

      <CardActions className="justify-start border-0">
        {prompt.options.map((opt) => {
          const isSelected = selectedId === opt.id
          const isPrimary = opt.variant === 'primary' || (!opt.variant && opt === prompt.options[0])

          return (
            <Button
              key={opt.id}
              type="button"
              size="sm"
              variant="ghost"
              disabled={disabled || (selectedId !== null && !isSelected)}
              onClick={() => handleChoose(opt)}
              data-testid={`prompt-choice-${opt.id}`}
              data-primary-action={isPrimary || isSelected ? true : undefined}
              aria-pressed={isSelected}
              className={cn(
                'min-h-11 rounded-xl px-3 text-xs font-medium bg-transparent hover:bg-transparent border-0 shadow-none cursor-pointer',
                disabled && 'opacity-60 cursor-not-allowed',
              )}
            >
              {isSelected && <span className="mr-1.5 text-xs">✓</span>}
              <span>{opt.label}</span>
            </Button>
          )
        })}
        {prompt.allow_other && onOther && <Button type="button" size="sm" variant="ghost" disabled={disabled} onClick={onOther} data-testid="prompt-choice-other" className="min-h-11 rounded-xl px-3 text-xs font-medium text-muted-foreground bg-transparent hover:bg-transparent border-0 shadow-none">{t('Others')}</Button>}
      </CardActions>
    </div>
  )
}
