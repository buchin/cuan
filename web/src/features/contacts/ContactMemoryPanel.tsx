import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Plus, Search, Pencil, Trash2, X } from 'lucide-react'
import { apiFetch } from '../../api/client'
import { useLanguage } from '../../context/LanguageContext'

type Entry = {
  id: string
  kind: 'note' | 'fact' | 'birthday'
  body: string
  recorded_by: string
  source_room_id: string
  source_message_id: string
  created_at: string
  updated_at: string
}
export function ContactMemoryPanel({
  contactId,
  onDirtyChange,
}: {
  contactId: string
  onDirtyChange?: (dirty: boolean) => void
}) {
  const { t, language } = useLanguage()
  const [items, setItems] = useState<Entry[]>([])
  const [query, setQuery] = useState('')
  const [search, setSearch] = useState('')
  const [offset, setOffset] = useState(0)
  const [more, setMore] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [revision, setRevision] = useState(0)
  const [editing, setEditing] = useState<string | null>(null)
  const [body, setBody] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [deleting, setDeleting] = useState<string | null>(null)
  const [notice, setNotice] = useState('')
  const requestId = useRef(crypto.randomUUID())
  useEffect(() => {
    onDirtyChange?.(editing !== null && !!body.trim())
    return () => onDirtyChange?.(false)
  }, [editing, body, onDirtyChange])
  const busy = saving
  const base = `/api/v1/contacts/${encodeURIComponent(contactId)}/memory`
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(query.trim())
      setOffset(0)
    }, 250)
    return () => clearTimeout(timer)
  }, [query])
  useEffect(() => {
    let stale = false
    setLoading(true)
    setError('')
    apiFetch<{ items: Entry[]; has_more: boolean }>(
      `${base}?q=${encodeURIComponent(search)}&offset=${offset}`,
    )
      .then((data) => {
        if (!stale) {
          setItems((old) =>
            offset
              ? [
                  ...old,
                  ...data.items.filter(
                    (item) => !old.some((existing) => existing.id === item.id),
                  ),
                ]
              : data.items,
          )
          setMore(data.has_more)
        }
      })
      .catch(() => {
        if (!stale) setError('Could not load notes. Try again.')
      })
      .finally(() => {
        if (!stale) setLoading(false)
      })
    return () => {
      stale = true
    }
  }, [base, search, offset, revision])
  function edit(entry?: Entry) {
    if (editing && body.trim() && !confirm(t('Discard this unsaved note?')))
      return
    setEditing(entry?.id || 'new')
    setBody(
      entry?.kind === 'birthday'
        ? `${t('Birthday')}: ${entry.body}`
        : entry?.body || '',
    )
    setSaveError('')
    setNotice('')
    setDeleting(null)
    requestId.current = crypto.randomUUID()
  }
  function refresh() {
    setOffset(0)
    setRevision((value) => value + 1)
  }
  async function save() {
    if (saving || !body.trim()) return
    setSaving(true)
    setSaveError('')
    try {
      await apiFetch(editing === 'new' ? base : `${base}/${editing}`, {
        method: editing === 'new' ? 'POST' : 'PATCH',
        body: JSON.stringify({
          ...(editing === 'new' ? { request_id: requestId.current } : {}),
          kind: 'note',
          body: body.trim(),
        }),
      })
      setEditing(null)
      setBody('')
      setNotice('Saved')
      refresh()
    } catch (e) {
      setSaveError(
        e instanceof Error ? e.message : 'Could not save note. Try again.',
      )
    } finally {
      setSaving(false)
    }
  }
  async function remove(id: string) {
    setSaving(true)
    setSaveError('')
    try {
      await apiFetch(`${base}/${id}`, { method: 'DELETE' })
      setDeleting(null)
      setNotice('Note deleted')
      refresh()
    } catch {
      setSaveError('Could not delete note. Try again.')
    } finally {
      setSaving(false)
    }
  }
  const typeLabel = (value: Entry['kind']) =>
    t(value === 'note' ? 'Note' : value === 'birthday' ? 'Birthday' : 'Fact')
  const notes = items.filter((item) => item.kind === 'note')
  const facts = items.filter((item) => item.kind !== 'note')
  function cards(entries: Entry[]) {
    return entries.map((entry) => (
      <article key={entry.id} className="rounded-lg border p-3 space-y-2">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 text-xs text-muted-foreground">
            <span>
              {typeLabel(entry.kind)} ·{' '}
              {new Date(entry.created_at).toLocaleDateString(language, {
                day: 'numeric',
                month: 'short',
                year: 'numeric',
              })}
            </span>
            <span className="block mt-1">
              {entry.recorded_by.startsWith('Agent:')
                ? t('Saved by agent')
                : t('You')}
              {entry.updated_at !== entry.created_at ? ` · ${t('Edited')}` : ''}
            </span>
          </div>
          <div className="flex shrink-0">
            <button
              type="button"
              className="icon-action"
              aria-label={t('Edit note')}
              disabled={busy}
              onClick={() => edit(entry)}
            >
              <Pencil className="h-4 w-4" />
            </button>
            <button
              type="button"
              className="icon-action"
              aria-label={t('Delete note')}
              disabled={busy}
              onClick={() => {
                setDeleting(entry.id)
                setSaveError('')
              }}
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </div>
        </div>
        <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
          {entry.body}
        </p>
        {entry.source_room_id && (
          <Link
            className="inline-flex min-h-11 items-center text-xs text-muted-foreground underline underline-offset-4"
            to={`/inbox/${encodeURIComponent(entry.source_room_id)}`}
          >
            {t('Source conversation')}
          </Link>
        )}
        {deleting === entry.id && (
          <div
            role="group"
            aria-label={t('Confirm deletion')}
            className="space-y-2 border-t pt-3"
          >
            <p className="text-sm">
              {t(
                'Delete this note? Cuan will no longer use it for recall. This cannot be undone.',
              )}
            </p>
            <div className="flex gap-2">
              <button
                className="action-secondary text-destructive"
                disabled={busy}
                onClick={() => void remove(entry.id)}
              >
                {t('Delete note')}
              </button>
              <button
                className="action-secondary"
                disabled={busy}
                onClick={() => setDeleting(null)}
              >
                {t('Cancel')}
              </button>
            </div>
          </div>
        )}
      </article>
    ))
  }
  return (
    <section
      aria-label={t('Contact memory')}
      className="space-y-4 border-t pt-5 mt-5"
    >
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold">{t('Notes & about')}</h3>
        <button
          type="button"
          className="action-secondary"
          disabled={busy}
          onClick={() => edit()}
        >
          <Plus className="h-4 w-4" />
          {t('Add note')}
        </button>
      </div>
      {editing && (
        <form
          className="rounded-lg border p-3 space-y-3"
          onSubmit={(event) => {
            event.preventDefault()
            void save()
          }}
        >
          <div className="flex items-center justify-between gap-2">
            <label className="text-xs font-medium" htmlFor="memory-body">
              {t('What would you like to remember?')}
            </label>
            <button
              type="button"
              className="icon-action"
              aria-label={t('Close editor')}
              disabled={busy}
              onClick={() => {
                if (!body.trim() || confirm(t('Discard this unsaved note?')))
                  setEditing(null)
              }}
            >
              <X className="h-4 w-4" />
            </button>
          </div>
          <textarea
            id="memory-body"
            autoFocus
            required
            maxLength={6000}
            rows={4}
            disabled={busy}
            className="w-full resize-y rounded-md border bg-background p-3 text-base sm:text-sm"
            value={body}
            onChange={(event) => setBody(event.target.value)}
            placeholder={t(
              'Meeting notes, interests, preferences, or a follow-up…',
            )}
          />
          <button
            className="action-primary"
            type="submit"
            disabled={busy || !body.trim()}
          >
            {saving ? t('Saving…') : t('Save')}
          </button>
        </form>
      )}
      {saveError && (
        <p role="alert" className="text-sm text-destructive">
          {t(saveError)}
        </p>
      )}
      {notice && (
        <p role="status" className="text-xs text-muted-foreground">
          {t(notice)}
        </p>
      )}
      <label className="flex min-h-11 items-center gap-2 rounded-lg border px-3">
        <Search className="h-4 w-4 text-muted-foreground" />
        <input
          aria-label={t('Search notes')}
          className="min-w-0 w-full bg-transparent text-base outline-none sm:text-sm"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t('Search notes…')}
        />
      </label>
      {error ? (
        <div role="alert" className="text-sm">
          <p>{t(error)}</p>
          <button
            className="action-secondary mt-2"
            onClick={() => setRevision((value) => value + 1)}
          >
            {t('Try again')}
          </button>
        </div>
      ) : (
        <>
          {loading && (
            <p role="status" className="text-xs text-muted-foreground">
              {t('Loading notes…')}
            </p>
          )}
          {!loading && !items.length && (
            <p className="py-4 text-sm text-muted-foreground">
              {search
                ? t('No matching notes.')
                : t(
                    'Add a note here, or ask Cuan to remember something about this person in Inbox.',
                  )}
            </p>
          )}
          {!!facts.length && (
            <section aria-label={t('About')} className="space-y-2">
              <h4 className="text-xs font-medium text-muted-foreground">
                {t('About')}
              </h4>
              {cards(facts)}
            </section>
          )}
          {!!notes.length && (
            <section aria-label={t('Notes')} className="space-y-2">
              <h4 className="text-xs font-medium text-muted-foreground">
                {t('Notes')}
              </h4>
              {cards(notes)}
            </section>
          )}
          {more && (
            <button
              className="action-secondary"
              disabled={loading}
              onClick={() => setOffset((value) => value + 50)}
            >
              {t('Load older notes')}
            </button>
          )}
        </>
      )}
    </section>
  )
}
