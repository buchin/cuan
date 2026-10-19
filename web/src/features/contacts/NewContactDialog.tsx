import { useEffect, useState } from 'react'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { apiFetch } from '../../api/client'

export function NewContactDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: (contact: { id: string }) => void
}) {
  const [name, setName] = useState(''),
    [email, setEmail] = useState(''),
    [busy, setBusy] = useState(false),
    [error, setError] = useState('')
  const [requestId, setRequestId] = useState(() => crypto.randomUUID())
  useEffect(() => {
    if (open) {
      setName('')
      setEmail('')
      setError('')
      setRequestId(crypto.randomUUID())
    }
  }, [open])
  return (
    <ActionDialog open={open} title="New contact" onClose={onClose} busy={busy}>
      <p className="mb-4 text-sm text-muted-foreground">
        Add a manual contact to your CRM. This does not send a message or email
        or connect a messaging channel.
      </p>
      <form
        className="space-y-4"
        onSubmit={async (e) => {
          e.preventDefault()
          if (busy || !name.trim()) return
          setBusy(true)
          setError('')
          try {
            const data = await apiFetch<{ contact: { id: string } }>(
              '/api/v1/contacts',
              {
                method: 'POST',
                body: JSON.stringify({
                  request_id: requestId,
                  name: name.trim(),
                  email: email.trim(),
                }),
              },
            )
            onCreated(data.contact)
          } catch (e) {
            setError(
              e instanceof Error
                ? e.message
                : 'Could not save this contact. Try again.',
            )
          } finally {
            setBusy(false)
          }
        }}
      >
        <div>
          <label htmlFor="new-contact-name" className="text-sm font-medium">
            Name
          </label>
          <input
            id="new-contact-name"
            className="field-control mt-1"
            required
            maxLength={120}
            autoComplete="name"
            value={name}
            disabled={busy}
            onChange={(e) => setName(e.target.value)}
          />
        </div>
        <div>
          <label htmlFor="new-contact-email" className="text-sm font-medium">
            Email (optional)
          </label>
          <input
            id="new-contact-email"
            type="email"
            className="field-control mt-1"
            maxLength={254}
            autoComplete="email"
            value={email}
            disabled={busy}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <button
            type="button"
            className="action-secondary"
            disabled={busy}
            onClick={onClose}
          >
            Cancel
          </button>
          <button className="action-primary" disabled={busy || !name.trim()}>
            {busy ? 'Saving…' : 'Save contact'}
          </button>
        </div>
      </form>
    </ActionDialog>
  )
}
