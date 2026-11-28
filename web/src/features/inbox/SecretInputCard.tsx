import { memo, useState } from 'react'
import { CheckCircle2, Eye, EyeOff, Lock, Send } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'
import { useLanguage } from '../../context/LanguageContext'
import type { SecretInput } from '../../api/chat'

export interface SecretInputCardProps {
  secretInput: SecretInput
  disabled?: boolean
  onSubmit: (value: string) => Promise<void>
  className?: string
}

export const SecretInputCard = memo(function SecretInputCard({
  secretInput,
  disabled = false,
  onSubmit,
  className,
}: SecretInputCardProps) {
  const { language } = useLanguage()
  const isId = language === 'id'
  const [value, setValue] = useState('')
  const [revealed, setRevealed] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [submittedLocal, setSubmittedLocal] = useState(false)
  const [error, setError] = useState('')

  const isSubmitted = Boolean(secretInput.submitted || submittedLocal)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = value.trim()
    if (!trimmed || disabled || submitting || isSubmitted) return
    setSubmitting(true)
    setError('')
    try {
      await onSubmit(trimmed)
      setValue('')
      setRevealed(false)
      setSubmittedLocal(true)
    } catch (err) {
      const msg =
        err instanceof Error
          ? err.message
          : isId
            ? 'Gagal mengirim kredensial. Silakan coba lagi.'
            : 'Failed to submit credential. Please try again.'
      setError(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      data-testid="secret-input-card"
      data-status={isSubmitted ? 'submitted' : 'pending'}
      className={cn(
        'mt-3 border-l-2 pl-3 py-0.5 space-y-2 min-w-0 max-w-full',
        isSubmitted ? 'border-l-emerald-500' : 'border-l-primary/70',
        className,
      )}
    >
      <div className="flex items-center gap-1.5 min-w-0">
        {isSubmitted ? (
          <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
        ) : (
          <Lock className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
        )}
        <span className="text-xs font-medium text-foreground/90 truncate">
          {secretInput.label}
        </span>
      </div>

      {isSubmitted ? (
        <p
          data-testid="secret-input-submitted"
          className="text-xs text-emerald-600 dark:text-emerald-400 font-medium"
        >
          {isId ? 'Terkirim dan disimpan dengan aman' : 'Sent and stored securely'}
        </p>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-1.5">
          <div className="flex items-center gap-2 min-w-0">
            <div className="relative flex-1 min-w-0">
              <input
                type={revealed ? 'text' : 'password'}
                value={value}
                onChange={(e) => {
                  setValue(e.target.value)
                  if (error) setError('')
                }}
                disabled={disabled || submitting}
                autoComplete="off"
                spellCheck={false}
                data-testid="secret-input-field"
                aria-label={secretInput.label}
                placeholder={
                  secretInput.placeholder ||
                  (isId ? 'Masukkan nilai rahasia…' : 'Enter secret value…')
                }
                className={cn(
                  'w-full min-h-11 rounded-xl border border-border/50 bg-background/80 px-3 pr-10 text-sm text-foreground placeholder:text-muted-foreground/70',
                  'focus:outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary/60 transition-colors',
                  (disabled || submitting) && 'opacity-60 cursor-not-allowed',
                )}
              />
              <button
                type="button"
                onClick={() => setRevealed((prev) => !prev)}
                disabled={disabled || submitting}
                aria-label={
                  revealed
                    ? isId
                      ? 'Sembunyikan nilai'
                      : 'Hide value'
                    : isId
                      ? 'Tampilkan nilai'
                      : 'Show value'
                }
                data-testid="secret-input-toggle"
                className="absolute right-1.5 top-1/2 -translate-y-1/2 inline-flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors cursor-pointer"
              >
                {revealed ? (
                  <EyeOff className="h-4 w-4" aria-hidden />
                ) : (
                  <Eye className="h-4 w-4" aria-hidden />
                )}
              </button>
            </div>
            <Button
              type="submit"
              size="sm"
              disabled={disabled || submitting || !value.trim()}
              data-testid="secret-input-submit"
              className="min-h-11 px-3.5 rounded-xl shrink-0 gap-1.5 cursor-pointer"
            >
              <Send className="h-3.5 w-3.5" aria-hidden />
              <span>
                {submitting
                  ? isId
                    ? 'Mengirim…'
                    : 'Sending…'
                  : secretInput.submit_label || (isId ? 'Kirim' : 'Send')}
              </span>
            </Button>
          </div>
          {error && (
            <p role="alert" className="text-xs text-destructive leading-snug">
              {error}
            </p>
          )}
        </form>
      )}
    </div>
  )
})
