import { useEffect, useRef, useState } from 'react'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { type ChatContact } from '../../api/chat'
import { Link } from 'react-router-dom'
import { apiFetch } from '../../api/client'
import { Bot, User, Search } from 'lucide-react'
import { Input } from '../../components/ui/forms'
import { getContactInitials } from './contact-initials'

export function NewMessageDialog({
  open,
  onClose,
  contacts,
  loading,
  error,
  onRetry,
  onSelect,
}: {
  open: boolean
  onClose: () => void
  contacts: ChatContact[]
  loading: boolean
  error: string
  onRetry: () => void
  onSelect: (contact: ChatContact) => void
}) {
  const [query, setQuery] = useState('')
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')
  const request = useRef<{ agent: string; id: string } | null>(null)
  const seen = new Set<string>()
  const recipients = contacts
    .filter((c) => {
      if (c.type !== 'agent') return true
      const id = c.agent_id || c.id.replace(/^contact_/, '')
      if (seen.has(id)) return false
      seen.add(id)
      return true
    })
    .map((c) =>
      c.type === 'agent' ? { ...c, name: c.agent_name || c.name } : c,
    )
  async function choose(c: ChatContact) {
    if (creating) return
    if (c.type !== 'agent') {
      onClose()
      onSelect(c)
      return
    }
    const agent = c.agent_id || c.id.replace(/^contact_/, '')
    if (request.current?.agent !== agent)
      request.current = { agent, id: crypto.randomUUID() }
    setCreating(true)
    setCreateError('')
    try {
      const topic = await apiFetch<ChatContact>('/api/v1/chat/conversations', {
        method: 'POST',
        body: JSON.stringify({
          agent_id: agent,
          request_id: request.current.id,
        }),
      })
      request.current = null
      onClose()
      onSelect(topic)
    } catch (e) {
      setCreateError(
        e instanceof Error
          ? e.message
          : 'Could not create the conversation. Try again.',
      )
    } finally {
      setCreating(false)
    }
  }
  useEffect(() => {
    if (open) setQuery('')
  }, [open])
  const filtered = recipients.filter((c) =>
    [c.name, c.product]
      .join(' ')
      .toLowerCase()
      .includes(query.trim().toLowerCase()),
  )
  return (
    <ActionDialog
      open={open}
      title="New message"
      onClose={() => {
        if (!creating) onClose()
      }}
    >
      <p className="mb-4 text-sm text-muted-foreground">
        Choose an agent to start a separate topic, or continue a customer chat.
        Opening a conversation does not send a message or start work.
      </p>
      {createError && (
        <p role="alert" className="mb-3 text-sm text-destructive">
          {createError}
        </p>
      )}
      {creating && <p role="status">Opening conversation…</p>}
      <Input
        className="mb-3"
        aria-label="Search recipients"
        placeholder="Search agents or customers"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        prefixIcon={<Search className="h-4 w-4" />}
      />
      {loading ? (
        <p role="status" className="py-4 text-sm">
          Loading recipients…
        </p>
      ) : error ? (
        <div role="alert">
          <p>{error}</p>
          <button className="action-secondary mt-3" onClick={onRetry}>
            Try again
          </button>
        </div>
      ) : filtered.length === 0 ? (
        <div className="space-y-3 py-3 text-sm">
          <p>
            {contacts.length
              ? 'No recipients found.'
              : 'No recipients yet. Add an agent or connect live chat first.'}
          </p>
          {query ? (
            <button className="action-secondary" onClick={() => setQuery('')}>
              Clear search
            </button>
          ) : (
            <div className="flex flex-wrap gap-2">
              <Link
                className="action-secondary"
                to="/settings/agents"
                onClick={onClose}
              >
                Manage agents
              </Link>
              <Link
                className="action-secondary"
                to="/settings/integrations/live-chat"
                onClick={onClose}
              >
                Connect live chat
              </Link>
            </div>
          )}
        </div>
      ) : (
        <div className="max-h-[50dvh] overflow-y-auto divide-y rounded-lg border">
          {filtered.map((c) => (
            <button
              key={c.id}
              className="flex min-h-16 w-full items-center gap-3 p-3 text-left hover:bg-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
              disabled={creating}
              onClick={() => void choose(c)}
            >
              <span
                aria-hidden="true"
                data-avatar="true"
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-secondary text-xs font-semibold text-foreground overflow-hidden avatar select-none"
              >
                {c.avatar && c.avatar !== '👤' ? (
                  c.avatar.startsWith('http') || c.avatar.startsWith('/') ? (
                    <img src={c.avatar} alt="" className="h-full w-full object-cover rounded-full" />
                  ) : (
                    <span>{c.avatar}</span>
                  )
                ) : c.type === 'agent' ? (
                  <Bot className="h-4 w-4 text-primary" />
                ) : getContactInitials(c.name) ? (
                  <span className="tracking-wide select-none">{getContactInitials(c.name)}</span>
                ) : (
                  <User className="h-4 w-4 text-muted-foreground" />
                )}
              </span>
              <span className="min-w-0">
                <span className="block truncate text-base font-medium">
                  {c.name}
                </span>
                <span className="block truncate text-sm text-muted-foreground">
                  {c.type === 'agent'
                    ? 'Agent'
                    : c.product || 'Live chat customer'}
                </span>
              </span>
            </button>
          ))}
        </div>
      )}
    </ActionDialog>
  )
}
