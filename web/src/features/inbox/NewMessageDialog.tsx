import { useEffect, useState } from 'react'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { type ChatContact } from '../../api/chat'
import { Link } from 'react-router-dom'

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
  useEffect(() => {
    if (open) setQuery('')
  }, [open])
  const filtered = contacts.filter((c) =>
    [c.name, c.product]
      .join(' ')
      .toLowerCase()
      .includes(query.trim().toLowerCase()),
  )
  return (
    <ActionDialog open={open} title="New message" onClose={onClose}>
      <p className="mb-4 text-sm text-muted-foreground">
        Choose an agent or live chat customer. Opening a conversation does not
        send a message.
      </p>
      <input
        className="field-control mb-3"
        aria-label="Search recipients"
        placeholder="Search agents or customers"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
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
              onClick={() => {
                onClose()
                onSelect(c)
              }}
            >
              <span
                aria-hidden="true"
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-secondary"
              >
                {c.avatar || c.name.slice(0, 2).toUpperCase()}
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
