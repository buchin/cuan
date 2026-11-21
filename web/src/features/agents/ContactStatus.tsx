import type { Agent } from '../../api/agents'
import type { ChatContact } from '../../api/chat'
import { agentForContact, connectionStatus } from './connection'

export function ContactStatus({
  contact,
  agents = [],
  now,
}: {
  contact: ChatContact
  agents?: Agent[]
  now?: number
}) {
  if (contact.type !== 'agent') {
    const isOnline = contact.is_online || contact.status === 'online'
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
        <span
          aria-hidden="true"
          className={`h-1.5 w-1.5 shrink-0 rounded-full ${isOnline ? 'bg-emerald-500 animate-pulse task-pulse-subtle' : 'bg-muted-foreground/40'}`}
        />
        <span>{isOnline ? 'Online' : 'Customer'}</span>
        {contact.product ? <span>· {contact.product}</span> : null}
        {isOnline && contact.page_title ? (
          <span className="truncate max-w-[150px] text-[11px] opacity-80">({contact.page_title})</span>
        ) : null}
      </span>
    )
  }
  const status = connectionStatus(agentForContact(contact, agents), now)
  return (
    <span
      className={`inline-flex items-center gap-1.5 text-xs ${status.verified ? 'text-emerald-700 dark:text-emerald-400' : 'text-muted-foreground'}`}
    >
      <span
        aria-hidden="true"
        className={`h-1.5 w-1.5 shrink-0 rounded-full ${status.verified ? 'bg-emerald-500' : 'bg-current'}`}
      />
      {status.label}
    </span>
  )
}
