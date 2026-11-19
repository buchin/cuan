export { resolvableConversation, conversationHasUnfinishedTasks } from './conversation-resolution-policy'
import { useState } from 'react'
import { CheckCircle2, Loader2, Undo2 } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'
import type { ChatContact } from '../../api/chat'

export async function setConversationResolved(contact: ChatContact, resolved: boolean, options?: { cancelUnfinished?: boolean }) {
  const cancelUnfinished = options?.cancelUnfinished ?? (resolved ? true : undefined)
  const result = await apiFetch<{ id: string; resolved: boolean; resolved_at?: string }>(
    '/api/v1/chat/conversations/' + encodeURIComponent(contact.conversation_id || contact.id) + '/resolution',
    {
      method: 'PATCH',
      body: JSON.stringify({
        resolved,
        observed_at: contact.resolution_observed_at || contact.last_message_at || contact.updated_at || new Date().toISOString(),
        ...(cancelUnfinished !== undefined ? { cancel_unfinished: cancelUnfinished } : {}),
      }),
    },
  )
  window.dispatchEvent(new CustomEvent('cuan-conversation-resolution', { detail: result }))
  return result
}

export function ConversationResolutionAction({ contact, compact = false, onChanged }: { contact: ChatContact; compact?: boolean; onChanged?: () => void }) {
  const { t } = useLanguage()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const label = contact.resolved ? 'Reopen conversation' : 'Resolve conversation'

  const handleToggle = async () => {
    if (busy) return
    if (!contact.resolved && (contact.active_tasks || 0) > 0) {
      const confirmMessage = t('Ada {{count}} tugas yang sedang berjalan di background. Batalkan tugas dan tuntaskan percakapan?')
        .replace('{{count}}', String(contact.active_tasks))
      if (!window.confirm(confirmMessage)) return
    }
    setBusy(true)
    setError('')
    try {
      await setConversationResolved(contact, !contact.resolved, { cancelUnfinished: true })
      onChanged?.()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Could not save conversation status'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="relative">
      <button
        type="button"
        className={compact ? 'icon-action text-muted-foreground' : 'inline-flex min-h-11 items-center gap-2 text-xs font-medium text-muted-foreground hover:text-foreground'}
        disabled={busy}
        aria-label={t(label)}
        title={t(label)}
        onClick={handleToggle}
      >
        {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : contact.resolved ? <Undo2 className="h-4 w-4" /> : <CheckCircle2 className="h-4 w-4" />}
        {!compact && t(label)}
      </button>
      {error && <p role="alert" className="max-w-xs text-xs text-destructive">{error}</p>}
    </div>
  )
}
