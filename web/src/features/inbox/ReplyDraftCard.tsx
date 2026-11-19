import { useState, useEffect } from 'react'
import { CardActions } from './CardActions'
import { FileText, RefreshCw, Mail, Check, AlertCircle } from 'lucide-react'
import { InternalVisibility } from '../../components/ui/InternalVisibility'
import { useLanguage } from '../../context/LanguageContext'
import { CollapsibleChatText } from './CollapsibleChatText'
import { sendCustomerReplyEmail } from '../../api/chat'

export interface ReplyDraftCardProps {
  draft: string
  internal: boolean
  messageId?: string
  conversationId?: string
  defaultRecipient?: string
  emailDelivery?: { status: string; recipient_type?: string }
  onUse: (draft: string) => void
  onRegenerate?: () => void
  onSendEmail?: (recipient: string) => Promise<void> | void
  regenerating?: boolean
  regenerateDisabled?: boolean
}

export function ReplyDraftCard({
  draft,
  internal,
  messageId,
  conversationId,
  defaultRecipient = '',
  emailDelivery,
  onUse,
  onRegenerate,
  onSendEmail,
  regenerating = false,
  regenerateDisabled = false,
}: ReplyDraftCardProps) {
  const { t } = useLanguage()
  const [recipient, setRecipient] = useState(defaultRecipient)
  const [isEditingRecipient, setIsEditingRecipient] = useState(false)
  const [sendingEmail, setSendingEmail] = useState(false)
  const [sendSuccess, setSendSuccess] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)

  useEffect(() => {
    if (defaultRecipient && !recipient) {
      setRecipient(defaultRecipient)
    }
  }, [defaultRecipient])

  const isSent =
    sendSuccess ||
    emailDelivery?.status === 'sent' ||
    emailDelivery?.status === 'delivered'

  const handleSendEmail = async () => {
    const targetRecipient = recipient.trim()
    if (!targetRecipient) {
      setIsEditingRecipient(true)
      setSendError(t('Recipient email is required.'))
      return
    }

    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(targetRecipient)) {
      setIsEditingRecipient(true)
      setSendError(t('Invalid recipient email format.'))
      return
    }

    setSendingEmail(true)
    setSendError(null)
    try {
      if (onSendEmail) {
        await onSendEmail(targetRecipient)
      } else if (conversationId && messageId) {
        await sendCustomerReplyEmail(conversationId, messageId, {
          recipient: targetRecipient,
          draft,
        })
      }
      setSendSuccess(true)
      setIsEditingRecipient(false)
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : t('Failed to send email. Please try again.')
      setSendError(message)
    } finally {
      setSendingEmail(false)
    }
  }

  return (
    <div data-card="reply-draft" className="inbox-attachment mt-2 bg-card/40 text-foreground">
      <div className="flex items-center gap-2 px-3 pt-3 text-xs text-muted-foreground">
        {internal && <InternalVisibility className="-ml-3 -my-2" />}
        <FileText className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{t('Customer reply draft · Not sent yet')}</span>
      </div>

      <div
        data-reply-recipient-row
        className="flex items-center justify-between px-3 py-1.5 text-xs text-muted-foreground border-t border-border/20 bg-muted/20 mt-2"
      >
        <div className="flex items-center gap-1.5 min-w-0 flex-1">
          <span className="shrink-0">{t('To:')}</span>
          {isEditingRecipient ? (
            <input
              type="email"
              data-recipient-input
              value={recipient}
              onChange={(e) => {
                setRecipient(e.target.value)
                if (sendError) setSendError(null)
              }}
              placeholder="nama@email.com"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Enter') setIsEditingRecipient(false)
              }}
              className="min-w-0 flex-1 bg-transparent border-b border-primary/50 px-1 py-0.5 text-xs text-foreground focus:outline-none"
            />
          ) : (
            <span data-recipient-display className="truncate font-medium text-foreground">
              {recipient || (
                <span className="italic text-muted-foreground">{t('No email provided')}</span>
              )}
            </span>
          )}
        </div>
        {!isSent && (
          <button
            type="button"
            data-edit-recipient-btn
            onClick={() => setIsEditingRecipient(!isEditingRecipient)}
            className="ml-2 shrink-0 text-[11px] text-muted-foreground hover:text-foreground cursor-pointer focus-visible:outline-none"
          >
            {isEditingRecipient ? t('Save') : t('Change')}
          </button>
        )}
      </div>

      <div data-reply-draft-body className="px-3 py-3 text-sm leading-relaxed break-words">
        <CollapsibleChatText content={draft} hideTaskCard />
      </div>

      {sendError && (
        <div
          data-email-error
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-destructive bg-destructive/10 border-t border-destructive/20"
        >
          <AlertCircle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span className="truncate">{sendError}</span>
        </div>
      )}

      <CardActions data-draft-footer className="px-3">
        {onRegenerate && (
          <button
            type="button"
            onClick={onRegenerate}
            disabled={regenerateDisabled || regenerating || sendingEmail}
            aria-busy={regenerating}
            className="inline-flex min-h-11 items-center gap-2 text-xs font-medium text-muted-foreground focus-visible:outline-2 focus-visible:outline-ring cursor-pointer disabled:opacity-70 disabled:cursor-default"
          >
            <RefreshCw
              className={`h-4 w-4${regenerating ? ' animate-spin' : ''}`}
              aria-hidden="true"
            />
            {t(regenerating ? 'Regenerating…' : 'Regenerate')}
          </button>
        )}
        <button
          type="button"
          data-primary-action
          onClick={() => onUse(draft)}
          className="inline-flex min-h-11 items-center gap-2 text-xs font-medium focus-visible:outline-2 focus-visible:outline-ring cursor-pointer"
        >
          <FileText className="h-4 w-4" aria-hidden="true" />
          {t('Review & Use')}
        </button>
        {isSent ? (
          <span
            data-email-sent-badge
            className="inline-flex min-h-11 items-center gap-1.5 text-xs font-medium text-emerald-600 dark:text-emerald-400"
          >
            <Check className="h-4 w-4" aria-hidden="true" />
            {t('Email sent')}
          </span>
        ) : (
          <button
            type="button"
            data-action="send-email"
            data-send-email-action
            onClick={handleSendEmail}
            disabled={sendingEmail}
            aria-busy={sendingEmail}
            className="inline-flex min-h-11 items-center gap-2 text-xs font-medium text-primary hover:text-primary/90 focus-visible:outline-2 focus-visible:outline-ring cursor-pointer disabled:opacity-50 disabled:cursor-default"
          >
            {sendingEmail ? (
              <RefreshCw className="h-4 w-4 animate-spin" aria-hidden="true" />
            ) : (
              <Mail className="h-4 w-4" aria-hidden="true" />
            )}
            {sendingEmail ? t('Sending email…') : t('Send Email')}
          </button>
        )}
      </CardActions>
    </div>
  )
}
