import { useState } from 'react'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'
import type { PromptChoices, PromptOption } from '../../api/chat'

export interface PromptChoiceChipsProps {
  prompt: PromptChoices
  disabled?: boolean
  onSelect: (option: PromptOption) => void
  className?: string
}

export function PromptChoiceChips({
  prompt,
  disabled = false,
  onSelect,
  className,
}: PromptChoiceChipsProps) {
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

      <div className="flex flex-wrap items-center gap-2">
        {prompt.options.map((opt) => {
          const isSelected = selectedId === opt.id
          const isPrimary = opt.variant === 'primary' || (!opt.variant && opt === prompt.options[0])

          return (
            <Button
              key={opt.id}
              type="button"
              size="sm"
              variant={isPrimary ? 'default' : 'outline'}
              disabled={disabled || (selectedId !== null && !isSelected)}
              onClick={() => handleChoose(opt)}
              data-testid={`prompt-choice-${opt.id}`}
              className={cn(
                'min-h-11 rounded-xl px-3.5 py-2 text-xs font-medium transition-all duration-150 cursor-pointer',
                isSelected && 'ring-2 ring-primary ring-offset-1 bg-primary text-primary-foreground',
                !isSelected && isPrimary && 'bg-primary/10 text-primary border border-primary/30 hover:bg-primary/20 dark:bg-primary/20 dark:text-primary dark:border-primary/40',
                !isSelected && !isPrimary && 'bg-card/50 text-foreground border border-border/50 hover:bg-muted/40 hover:text-foreground',
                disabled && 'opacity-60 cursor-not-allowed',
              )}
            >
              {isSelected && <span className="mr-1.5 text-xs">✓</span>}
              <span>{opt.label}</span>
            </Button>
          )
        })}
      </div>
    </div>
  )
}
