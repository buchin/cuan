import { useState } from 'react'
import { CheckCircle2, Loader2, Undo2 } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'
import type { ChatContact } from '../../api/chat'

export function conversationHasUnfinishedTasks(contact: ChatContact) {
  return !!(contact.active_tasks || contact.queued_tasks || contact.review_tasks || contact.failed_tasks ||
    (contact.task_count || 0) > (contact.done_tasks || 0) + (contact.canceled_tasks || 0))
}

export function completedConversation(contact: ChatContact) {
  const total = contact.task_count || 0
  return total > 0 && (contact.done_tasks || 0) > 0 &&
    (contact.done_tasks || 0) + (contact.canceled_tasks || 0) === total &&
    !(contact.active_tasks || contact.queued_tasks || contact.review_tasks || contact.failed_tasks)
}

export async function setConversationResolved(contact: ChatContact, resolved: boolean) {
  const result = await apiFetch<{ id: string; resolved: boolean; resolved_at?: string }>(
    '/api/v1/chat/conversations/' + encodeURIComponent(contact.conversation_id || contact.id) + '/resolution',
    { method: 'PATCH', body: JSON.stringify({ resolved, observed_at: contact.resolution_observed_at || contact.last_message_at || contact.updated_at }) },
  )
  window.dispatchEvent(new CustomEvent('cuan-conversation-resolution', { detail: result }))
  return result
}

export function ConversationResolutionAction({ contact, compact = false, onChanged }: { contact: ChatContact; compact?: boolean; onChanged?: () => void }) {
  const { t } = useLanguage()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const blocked = !contact.resolved && conversationHasUnfinishedTasks(contact)
  const label = contact.resolved ? 'Reopen conversation' : 'Resolve conversation'
  return <div className="relative">
    <button type="button" className={compact ? 'icon-action text-muted-foreground' : 'inline-flex min-h-11 items-center gap-2 text-xs font-medium text-muted-foreground hover:text-foreground'}
      disabled={busy || blocked} aria-label={t(label)} title={t(blocked ? 'Finish or cancel the remaining tasks before resolving this conversation' : label)}
      onClick={async () => { setBusy(true); setError(''); try { await setConversationResolved(contact, !contact.resolved); onChanged?.() } catch (err) { setError(err instanceof Error ? err.message : t('Could not save conversation status')) } finally { setBusy(false) } }}>
      {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : contact.resolved ? <Undo2 className="h-4 w-4" /> : <CheckCircle2 className="h-4 w-4" />}
      {!compact && t(label)}
    </button>
    {error && <p role="alert" className="max-w-xs text-xs text-destructive">{error}</p>}
  </div>
}
