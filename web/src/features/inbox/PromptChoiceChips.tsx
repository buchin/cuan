import { useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { Send } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'
import type { PromptChoices, PromptOption } from '../../api/chat'
import { useLanguage } from '../../context/LanguageContext'
import { Composer, ComposerSendButton } from '../../components/ui/Composer'
import { ComposerInput, resizeComposerInput } from '../../components/ui/ComposerInput'

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
  const [otherOpen, setOtherOpen] = useState(false)
  const [otherText, setOtherText] = useState('')
  const otherInput = useRef<HTMLTextAreaElement>(null)

  if (!prompt || !prompt.options || prompt.options.length === 0) {
    return null
  }

  const handleChoose = (opt: PromptOption) => {
    if (disabled || selectedId) return
    setOtherOpen(false)
    setSelectedId(opt.id)
    onSelect(opt)
  }
  const openOther = () => {
    if (selectedId) return
    // Commit the input before focusing, within the trusted tap event. Deferring
    // focus to an animation frame can leave the iOS software keyboard closed.
    flushSync(() => setOtherOpen(true))
    onOther?.()
    otherInput.current?.focus()
  }
  const sendOther = () => {
    const label = otherText.trim()
    if (!label) return
    handleChoose({ id: 'custom', label })
  }

  return (
    <div
      role="group"
      aria-label="Pilihan tanggapan cepat"
      data-testid="prompt-choice-chips"
      className={cn('prompt-choice-container space-y-2 bg-transparent border-0 shadow-none', className)}
    >
      {prompt.question && (
        <p className="text-xs font-medium text-muted-foreground/90 pl-0.5 tracking-tight">
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
              variant="ghost"
              disabled={disabled || (selectedId !== null && !isSelected)}
              onClick={() => handleChoose(opt)}
              data-testid={`prompt-choice-${opt.id}`}
              data-primary-action={isSelected || (!otherOpen && !selectedId && isPrimary) ? true : undefined}
              aria-pressed={isSelected}
              className={cn(
                'prompt-choice-option min-h-11 rounded-xl px-3.5 py-2 text-xs font-medium max-w-full text-left whitespace-normal break-words leading-snug cursor-pointer transition-all duration-150 shadow-none',
                'bg-transparent hover:bg-muted/20 border border-border/40 hover:border-border/70 text-foreground/90 hover:text-foreground',
                isPrimary && !isSelected && 'border-primary/30 hover:border-primary/60 text-foreground hover:bg-primary/5',
                isSelected && 'border-primary/60 bg-primary/10 text-primary font-semibold',
                disabled && 'opacity-60 cursor-not-allowed pointer-events-none',
              )}
            >
              {isSelected && <span className="mr-1.5 text-xs">✓</span>}
              <span>{opt.label}</span>
            </Button>
          )
        })}
        {prompt.allow_other && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={selectedId !== null}
            onClick={openOther}
            aria-expanded={otherOpen}
            aria-pressed={otherOpen || selectedId === 'custom'}
            data-primary-action={otherOpen || selectedId === 'custom' ? true : undefined}
            data-testid="prompt-choice-other"
            className={cn(
              'prompt-choice-option min-h-11 rounded-xl px-3.5 py-2 text-xs font-medium max-w-full text-left whitespace-normal break-words leading-snug cursor-pointer transition-all duration-150 shadow-none',
              'bg-transparent hover:bg-muted/20 border border-border/30 hover:border-border/60 text-muted-foreground hover:text-foreground',
              (otherOpen || selectedId === 'custom') && 'border-primary/60 bg-primary/10 text-primary font-semibold',
            )}
          >
            {selectedId === 'custom' && <span aria-hidden className="mr-1.5">✓</span>}
            {t('Others')}
          </Button>
        )}
      </div>
      {otherOpen && (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            sendOther()
          }}
        >
          <Composer className="mt-2 bg-transparent border border-border/40 shadow-none">
            <ComposerInput
              ref={otherInput}
              data-choice-other-input
              rows={1}
              aria-label={t('Your answer')}
              placeholder={t('Type another option…')}
              value={otherText}
              onChange={(event) => {
                setOtherText(event.target.value)
                resizeComposerInput(event.target)
              }}
              onKeyDown={(event) => {
                if (event.key === 'Escape') {
                  event.preventDefault()
                  setOtherOpen(false)
                }
                if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
                  event.preventDefault()
                  sendOther()
                }
              }}
            />
            <ComposerSendButton
              type="submit"
              data-testid="prompt-choice-other-send"
              aria-label={t('Send answer')}
              disabled={disabled || !otherText.trim()}
            >
              <Send aria-hidden className="h-4 w-4" />
            </ComposerSendButton>
          </Composer>
        </form>
      )}
    </div>
  )
}
