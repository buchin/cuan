import { useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { Send } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'
import type { PromptChoices, PromptOption } from '../../api/chat'
import { useLanguage } from '../../context/LanguageContext'
import { CardActions } from './CardActions'
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
              data-primary-action={isSelected || (!otherOpen && !selectedId && isPrimary) ? true : undefined}
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
        {prompt.allow_other && <Button type="button" size="sm" variant="ghost" disabled={selectedId !== null} onClick={openOther} aria-expanded={otherOpen} aria-pressed={otherOpen || selectedId === 'custom'} data-primary-action={otherOpen || selectedId === 'custom' ? true : undefined} data-testid="prompt-choice-other" className="min-h-11 rounded-xl px-3 text-xs font-medium text-muted-foreground bg-transparent hover:bg-transparent border-0 shadow-none">{selectedId === 'custom' && <span aria-hidden className="mr-1.5">✓</span>}{t('Others')}</Button>}
      </CardActions>
      {otherOpen && <form onSubmit={event => { event.preventDefault(); sendOther() }}>
        <Composer>
          <ComposerInput ref={otherInput} data-choice-other-input rows={1} aria-label={t('Your answer')} placeholder={t('Type another option…')} value={otherText} onChange={event => { setOtherText(event.target.value); resizeComposerInput(event.target) }} onKeyDown={event => {
            if (event.key === 'Escape') { event.preventDefault(); setOtherOpen(false) }
            if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); sendOther() }
          }} />
          <ComposerSendButton type="submit" data-testid="prompt-choice-other-send" aria-label={t('Send answer')} disabled={disabled || !otherText.trim()}><Send aria-hidden className="h-4 w-4" /></ComposerSendButton>
        </Composer>
      </form>}
    </div>
  )
}
