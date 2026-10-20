import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  Search,
  SlidersHorizontal,
  Users,
  MessageSquare,
  RefreshCw,
  ChevronLeft,
  X,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'

type Contact = {
  is_manual?: boolean
  id: string
  name: string
  email: string
  product: string
  conversation_id: string
  tags: string[]
  updated_at: string
}
const source = (c: Contact) => c.product.trim() || 'Live chat'
const dateLabel = (iso: string, language: string) => {
  const d = new Date(iso)
  return Number.isNaN(d.getTime())
    ? '—'
    : d.toLocaleDateString(language, { day: 'numeric', month: 'short' })
}

export function ContactsView({ refreshKey = 0 }: { refreshKey?: number }) {
  const { t, language } = useLanguage()
  const label = (c: Contact) => c.name || c.email || t('Anonymous visitor')
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [contacts, setContacts] = useState<Contact[]>([])
  const [loading, setLoading] = useState(true),
    [error, setError] = useState('')
  const [query, setQuery] = useState(''),
    [tag, setTag] = useState(''),
    [product, setProduct] = useState('')
  const [params, setParams] = useSearchParams()
  const selectedId = params.get('contact') || ''
  const selected = contacts.find((c) => c.id === selectedId)
  const [drafts, setDrafts] = useState<Record<string, string[]>>({})
  const [tagInput, setTagInput] = useState(''),
    [saveError, setSaveError] = useState(''),
    [saved, setSaved] = useState(false),
    [saving, setSaving] = useState(false)
  const detailRef = useRef<HTMLElement>(null),
    lastOpened = useRef<HTMLButtonElement | null>(null)
  const selectedIdRef = useRef(selectedId)
  selectedIdRef.current = selectedId
  const tags = [...new Set(contacts.flatMap((c) => c.tags))].sort()
  const sources = [...new Set(contacts.map(source))].sort()
  const filtered = contacts.filter(
    (c) =>
      (!tag || c.tags.includes(tag)) &&
      (!product || source(c) === product) &&
      [c.name, c.email, c.product, ...c.tags]
        .join(' ')
        .toLowerCase()
        .includes(query.trim().toLowerCase()),
  )
  const filteredActive = !!(query.trim() || tag || product)
  const draft = selected ? drafts[selected.id] || selected.tags : []
  const dirty =
    !!selected &&
    (JSON.stringify(draft) !== JSON.stringify(selected.tags) ||
      !!tagInput.trim())
  const suggestions = tags
    .filter(
      (t) =>
        !draft.some((d) => d.toLowerCase() === t.toLowerCase()) &&
        t.toLowerCase().includes(tagInput.trim().toLowerCase()),
    )
    .slice(0, 8)
  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await apiFetch<{ contacts: Contact[] }>('/api/v1/contacts')
      setContacts(data.contacts)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load contacts')
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => {
    void load()
  }, [refreshKey])
  useEffect(() => {
    setTagInput('')
    setSaveError('')
    setSaved(false)
  }, [selectedId])
  useEffect(() => {
    if (selectedId) detailRef.current?.focus({ preventScroll: true })
    else lastOpened.current?.focus({ preventScroll: true })
  }, [selectedId, loading])
  function select(id: string) {
    const next = new URLSearchParams(params)
    if (id) next.set('contact', id)
    else next.delete('contact')
    setParams(next, { replace: !id })
  }
  function append(value: string, base = draft): string[] | null {
    const text = value.trim()
    if (!text) return base
    if (Array.from(text).length > 40) {
      setSaveError('Tags can contain up to 40 characters.')
      return null
    }
    if (base.some((t) => t.toLowerCase() === text.toLowerCase())) return base
    if (base.length >= 20) {
      setSaveError('Each contact can have up to 20 tags.')
      return null
    }
    return [...base, text]
  }
  function add(value: string) {
    if (!selected) return
    const next = append(value)
    if (!next) return
    setDrafts((old) => ({ ...old, [selected.id]: next }))
    setTagInput('')
    setSaveError('')
    setSaved(false)
  }
  async function save() {
    if (!selected || saving) return
    const next = append(tagInput)
    if (!next) return
    const id = selected.id
    setDrafts((old) => ({ ...old, [id]: next }))
    setTagInput('')
    setSaving(true)
    setSaved(false)
    setSaveError('')
    try {
      const result = await apiFetch<{ tags: string[] }>(
        `/api/v1/contacts/${encodeURIComponent(id)}`,
        { method: 'PATCH', body: JSON.stringify({ tags: next }) },
      )
      setContacts((old) =>
        old.map((c) => (c.id === id ? { ...c, tags: result.tags } : c)),
      )
      setDrafts((old) => ({ ...old, [id]: result.tags }))
      if (selectedIdRef.current === id) setSaved(true)
    } catch (e) {
      if (selectedIdRef.current === id)
        setSaveError(e instanceof Error ? e.message : 'Could not save tags')
    } finally {
      setSaving(false)
    }
  }
  const clear = () => {
    setQuery('')
    setTag('')
    setProduct('')
  }
  return (
    <section
      className="flex h-full min-h-0 min-w-0 overflow-hidden"
      aria-label={t('Contacts')}
    >
      <aside
        aria-label={t('Contact list')}
        className={`min-h-0 min-w-0 w-full flex-col min-[1200px]:w-80 min-[1200px]:shrink-0 min-[1200px]:border-r ${selectedId ? 'hidden min-[1200px]:flex' : 'flex'}`}
      >
        <div className="shrink-0 border-b p-3">
          {contacts.length > 0 && (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <label className="flex min-w-0 flex-1 min-h-11 items-center gap-2 rounded-lg border bg-background px-3 focus-within:ring-2 focus-within:ring-ring">
                  <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <input
                    aria-label={t('Search contacts')}
                    className="min-w-0 flex-1 bg-transparent py-2 text-base outline-none"
                    placeholder={t('Search contacts\u2026')}
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                  />
                </label>
                <button
                  type="button"
                  className="icon-action relative shrink-0"
                  aria-label={t('Filters')}
                  aria-expanded={filtersOpen}
                  aria-controls="contact-filters"
                  onClick={() => setFiltersOpen(!filtersOpen)}
                >
                  <SlidersHorizontal className="h-5 w-5" />
                  {(tag || product) && (
                    <span
                      aria-hidden="true"
                      className="absolute right-1 top-1 h-2 w-2 rounded-full bg-primary"
                    />
                  )}
                </button>
              </div>
              {(tag || product) && !filtersOpen && (
                <button
                  type="button"
                  className="min-h-11 text-xs text-primary underline"
                  onClick={() => setFiltersOpen(true)}
                >
                  {t('Filters applied')}
                </button>
              )}
              {filtersOpen && (
                <div id="contact-filters" className="grid grid-cols-2 gap-2">
                  <select
                    aria-label={t('Filter by tag')}
                    className="field-control min-w-0 text-sm"
                    value={tag}
                    onChange={(e) => setTag(e.target.value)}
                  >
                    <option value="">{t('All tags')}</option>
                    {tags.map((t) => (
                      <option key={t}>{t}</option>
                    ))}
                  </select>
                  <select
                    aria-label={t('Filter by source')}
                    className="field-control min-w-0 text-sm"
                    value={product}
                    onChange={(e) => setProduct(e.target.value)}
                  >
                    <option value="">{t('All sources')}</option>
                    {sources.map((s) => (
                      <option key={s}>{s}</option>
                    ))}
                  </select>
                  {filteredActive && (
                    <button
                      className="action-secondary col-span-2"
                      onClick={clear}
                    >
                      {t('Clear filters')}
                    </button>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-3">
          {loading ? (
            <p role="status" className="py-6 text-sm text-muted-foreground">
              {t('Loading contacts\u2026')}
            </p>
          ) : error ? (
            <div role="alert" className="rounded-lg border p-5">
              <p>{t(error)}</p>
              <button
                className="action-secondary mt-3"
                onClick={() => void load()}
              >
                {t('Try again')}
              </button>
            </div>
          ) : contacts.length === 0 && !selectedId ? (
            <div className="rounded-lg border border-dashed p-6 text-center">
              <Users className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
              <h2 className="text-lg font-semibold">{t('No contacts yet')}</h2>
              <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
                {t(
                  'Live chat visitors appear here automatically. Set up your widget in Settings \u2192 Integrations.',
                )}
              </p>
              <Link
                to="/settings/integrations/live-chat"
                className="action-primary mt-4"
              >
                {t('Connect live chat')}
              </Link>
            </div>
          ) : (
            <div className="min-w-0">
              <div className="mb-1 flex items-center justify-between gap-2">
                <p className="text-xs text-muted-foreground" role="status">
                  {filteredActive
                    ? t('{shown} of {total} contacts')
                        .replace('{shown}', String(filtered.length))
                        .replace('{total}', String(contacts.length))
                    : t(
                        contacts.length === 1
                          ? '{count} contact'
                          : '{count} contacts',
                      ).replace('{count}', String(contacts.length))}
                </p>
                <button
                  type="button"
                  className="icon-action"
                  aria-label={t('Reload contacts')}
                  onClick={() => void load()}
                  disabled={loading || saving}
                >
                  <RefreshCw className="h-4 w-4" />
                </button>
              </div>
              {filtered.length === 0 ? (
                <div className="rounded-lg border p-5 text-center">
                  <p className="font-medium">{t('No matching contacts.')}</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t('Try another search or clear your filters.')}
                  </p>
                  <button className="action-secondary mt-3" onClick={clear}>
                    {t('Clear all filters')}
                  </button>
                </div>
              ) : (
                <div className="overflow-hidden divide-y divide-border/50">
                  {filtered.map((c) => (
                    <button
                      key={c.id}
                      type="button"
                      disabled={saving}
                      onClick={(e) => {
                        lastOpened.current = e.currentTarget
                        select(c.id)
                      }}
                      aria-pressed={selectedId === c.id}
                      className={`flex min-h-20 w-full items-start gap-3 p-3 text-left hover:bg-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2 ${selectedId === c.id ? 'bg-accent' : ''}`}
                    >
                      <span
                        aria-hidden="true"
                        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-secondary text-sm font-semibold"
                      >
                        {Array.from(label(c))
                          .slice(0, 2)
                          .join('')
                          .toUpperCase()}
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-semibold">
                          {label(c)}
                        </span>
                        <span className="mt-0.5 block truncate text-sm text-muted-foreground">
                          {c.email || t('No email provided')}
                        </span>
                        <span className="mt-1 block truncate text-xs text-muted-foreground">
                          {t(source(c))}
                        </span>
                        {c.tags.length > 0 && (
                          <span className="mt-2 flex min-w-0 flex-wrap gap-1">
                            {c.tags.slice(0, 2).map((t) => (
                              <span
                                key={t}
                                className="max-w-28 truncate rounded bg-secondary px-2 py-0.5 text-xs"
                              >
                                {t}
                              </span>
                            ))}
                            {c.tags.length > 2 && (
                              <span className="text-xs text-muted-foreground">
                                +{c.tags.length - 2}
                              </span>
                            )}
                          </span>
                        )}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </aside>
      <div
        className={`min-h-0 min-w-0 flex-1 overflow-y-auto p-4 md:p-6 ${selectedId ? '' : 'hidden min-[1200px]:block'}`}
      >
        {selectedId ? (
          <aside
            ref={detailRef}
            tabIndex={-1}
            aria-label={t('Contact details')}
            className="mx-auto min-w-0 w-full max-w-3xl outline-none"
          >
            <button
              className="action-secondary mb-4 min-[1200px]:hidden"
              disabled={saving}
              onClick={() => select('')}
            >
              <ChevronLeft className="h-4 w-4" />
              {t('Back to contacts')}
            </button>
            {selected ? (
              <>
                <h2 className="break-words text-xl font-semibold">
                  {label(selected)}
                </h2>
                <p className="mt-1 break-all text-sm text-muted-foreground">
                  {selected.email || t('No email provided')}
                </p>
                <p className="mt-2 text-xs text-muted-foreground">
                  {t(source(selected))} · {t('Updated')}{' '}
                  {dateLabel(selected.updated_at, language)}
                </p>
                {selected.email && (
                  <p className="mt-2 text-xs text-muted-foreground">
                    {t('Email is unverified.')}
                  </p>
                )}
                {selected.is_manual ? (
                  <p className="my-5 rounded-lg border p-3 text-sm text-muted-foreground">
                    {t(
                      'This manual contact has no connected messaging channel. Saving a contact does not send a message or email.',
                    )}
                  </p>
                ) : (
                  <Link
                    className="action-primary my-5 w-full sm:w-auto"
                    to={`/inbox/${encodeURIComponent(selected.conversation_id)}`}
                  >
                    <MessageSquare className="h-4 w-4" />
                    {t('Open conversation')}
                  </Link>
                )}
                <form
                  aria-label={t('Manage tags')}
                  className="space-y-3 border-t pt-4"
                  onSubmit={(e) => {
                    e.preventDefault()
                    void save()
                  }}
                >
                  <h3 className="text-base font-semibold">{t('Tags')}</h3>
                  <div
                    aria-label={t('Contact tags')}
                    className="flex flex-wrap gap-2"
                  >
                    {draft.length === 0 ? (
                      <p className="text-sm text-muted-foreground">
                        {t('No tags yet.')}
                      </p>
                    ) : (
                      draft.map((t) => (
                        <button
                          type="button"
                          key={t}
                          disabled={saving}
                          aria-label={
                            language === 'id'
                              ? `Hapus tag ${t}`
                              : `Remove tag ${t}`
                          }
                          className="inline-flex min-h-11 max-w-full items-center gap-2 rounded-full border bg-background px-3 text-sm hover:bg-accent"
                          onClick={() => {
                            setDrafts((old) => ({
                              ...old,
                              [selected!.id]: draft.filter((v) => v !== t),
                            }))
                            setSaved(false)
                            setSaveError('')
                          }}
                        >
                          <span className="min-w-0 break-all">{t}</span>
                          <X className="h-4 w-4 shrink-0" />
                        </button>
                      ))
                    )}
                  </div>
                  <label
                    className="block text-sm font-medium"
                    htmlFor="contact-add-tag"
                  >
                    {t('Add tag')}
                  </label>
                  <div className="flex gap-2">
                    <input
                      id="contact-add-tag"
                      className="field-control flex-1"
                      placeholder={t('For example: New customer')}
                      value={tagInput}
                      disabled={saving}
                      onChange={(e) => {
                        setTagInput(e.target.value)
                        setSaved(false)
                        setSaveError('')
                      }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault()
                          add(tagInput)
                        }
                      }}
                    />
                    <button
                      type="button"
                      className="action-secondary"
                      disabled={saving || !tagInput.trim()}
                      onClick={() => add(tagInput)}
                    >
                      {t('Add')}
                    </button>
                  </div>
                  {suggestions.length > 0 && (
                    <div aria-label={t('Available tags')} className="space-y-1">
                      <p className="text-xs text-muted-foreground">
                        {t('Use an existing tag')}
                      </p>
                      <div className="flex flex-wrap gap-2">
                        {suggestions.map((t) => (
                          <button
                            type="button"
                            key={t}
                            disabled={saving}
                            className="action-secondary max-w-full break-all"
                            onClick={() => add(t)}
                          >
                            {t}
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                  <p className="text-xs text-muted-foreground">
                    {t(
                      '{count}/20 tags · up to 40 characters each. Press Enter to add.',
                    ).replace('{count}', String(draft.length))}
                  </p>
                  {saveError && (
                    <p role="alert" className="text-sm text-destructive">
                      {t(saveError)}
                    </p>
                  )}
                  {dirty && !saving && (
                    <p className="text-sm text-muted-foreground">
                      {t('You have unsaved changes.')}
                    </p>
                  )}
                  <div className="flex flex-wrap gap-2">
                    <button
                      type="submit"
                      className="action-primary"
                      disabled={saving || !dirty}
                    >
                      {saving ? t('Saving…') : t('Save tags')}
                    </button>
                    {dirty && (
                      <button
                        type="button"
                        className="action-secondary"
                        disabled={saving}
                        onClick={() => {
                          setDrafts((old) => ({
                            ...old,
                            [selected!.id]: selected!.tags,
                          }))
                          setTagInput('')
                          setSaveError('')
                          setSaved(false)
                        }}
                      >
                        {t('Discard changes')}
                      </button>
                    )}
                  </div>
                  {saved && (
                    <p role="status" className="text-sm">
                      {t('Tags saved.')}
                    </p>
                  )}
                </form>
              </>
            ) : (
              <>
                <h2 className="text-lg font-semibold">
                  {t('Contact not found')}
                </h2>
                <p className="mt-2 text-sm text-muted-foreground">
                  {t('This contact may no longer be available.')}
                </p>
                <button
                  className="action-secondary mt-3"
                  onClick={() => select('')}
                >
                  {t('View contacts')}
                </button>
              </>
            )}
          </aside>
        ) : (
          <aside className="flex h-full flex-col items-center justify-center p-6 text-center text-sm text-muted-foreground">
            <Users className="mx-auto mb-3 h-8 w-8" />
            <p>
              {t(
                'Select a contact to view their details, tags, and conversation.',
              )}
            </p>
          </aside>
        )}
      </div>
    </section>
  )
}
