import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useState } from 'react'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { apiFetch } from '../../api/client'
import { FormField, Input } from '../../components/ui/forms'

export function NewContactDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: (contact: { id: string }) => void
}) {
  const { t } = useLanguage()
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
    <ActionDialog
      open={open}
      title={t('New contact')}
      onClose={onClose}
      busy={busy}
    >
      <p className="mb-4 text-sm text-muted-foreground">
        {t(
          'Add a manual contact to your CRM. This does not send a message or email or connect a messaging channel.',
        )}
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
        <FormField label={t('Name')} htmlFor="new-contact-name" required>
          <Input
            id="new-contact-name"
            required
            maxLength={120}
            autoComplete="name"
            value={name}
            disabled={busy}
            onChange={(e) => setName(e.target.value)}
          />
        </FormField>
        <FormField label={t('Email (optional)')} htmlFor="new-contact-email">
          <Input
            id="new-contact-email"
            type="email"
            maxLength={254}
            autoComplete="email"
            value={email}
            disabled={busy}
            onChange={(e) => setEmail(e.target.value)}
          />
        </FormField>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {t(error)}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <button
            type="button"
            className="action-secondary"
            disabled={busy}
            onClick={onClose}
          >
            {t('Cancel')}
          </button>
          <button className="action-primary" disabled={busy || !name.trim()}>
            {busy ? t('Saving…') : t('Save contact')}
          </button>
        </div>
      </form>
    </ActionDialog>
  )
}
