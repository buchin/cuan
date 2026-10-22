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
  if (contact.type !== 'agent')
    return (
      <span className="text-xs text-muted-foreground">
        Customer{contact.product ? ` · ${contact.product}` : ''}
      </span>
    )
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
