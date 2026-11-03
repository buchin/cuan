import { BackAction } from '../../components/layout/DetailLayout'
import { ListSearch, ListRowContent } from '../../components/layout/ListControls'
import { IconButton } from '../../components/ui/IconButton'
import { ListCreateAction } from '../../components/layout/ListCreateAction'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useArchivedItems, ArchiveNotice } from '../../hooks/useArchivedItems'
import { ContactMemoryPanel } from './ContactMemoryPanel'
import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  Archive,
  SlidersHorizontal,
  Users,
  MessageSquare,
  X,
  FileText,
  Tag,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'

type Contact = {
  search_note?: { id: string; snippet: string }
  is_manual?: boolean
  id: string
  name: string
  email: string
  product: string
  conversation_id: string
  tags: string[]
  updated_at: string
}
const source = (c: Contact) => c.is_manual ? 'Added manually' : c.product.trim() || 'Live chat'
const dateLabel = (iso: string, language: string) => {
  const d = new Date(iso)
  return Number.isNaN(d.getTime())
    ? '—'
    : d.toLocaleDateString(language, { day: 'numeric', month: 'short' })
}

export function ContactsView({ refreshKey = 0, onNewContact }: { refreshKey?: number; onNewContact?: () => void }) {
  const { t, language } = useLanguage()
  const archive = useArchivedItems('contact')
  const [showArchived, setShowArchived] = useState(false)
  const label = (c: Contact) => c.name || c.email || t('Anonymous visitor')
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [contacts, setContacts] = useState<Contact[]>([])
  const [loading, setLoading] = useState(true),
    [error, setError] = useState('')
  const [query, setQuery] = useState(
      () => new URLSearchParams(window.location.search).get('q') || '',
    ),
    [tag, setTag] = useState(''),
    [product, setProduct] = useState('')
  const [params, setParams] = useSearchParams()
  const selectedId = params.get('contact') || ''
  const [searchResults, setSearchResults] = useState<{
    key: string
    contacts: Contact[]
    more: boolean
  }>({ key: '', contacts: [], more: false })
  const [searchOffset, setSearchOffset] = useState(0)
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState('')
  const [searchRevision, setSearchRevision] = useState(0)
  const searchKey = JSON.stringify([query.trim(), tag, product])
  const selected =
    contacts.find((c) => c.id === selectedId) ||
    searchResults.contacts.find((c) => c.id === selectedId)
  useEffect(() => {
    setSearchOffset(0)
  }, [query, tag, product])
  useEffect(() => {
    const term = query.trim()
    const controller = new AbortController()
    if (!term) {
      setSearching(false)
      setSearchError('')
      return
    }
    setSearching(true)
    setSearchError('')
    const timer = setTimeout(() => {
      const params = new URLSearchParams({
        q: term,
        offset: String(searchOffset),
        tag,
        source: product,
      })
      apiFetch<{ contacts: Contact[]; has_more: boolean }>(
        '/api/v1/contacts/search?' + params,
        { signal: controller.signal },
      )
        .then((data) => {
          if (!controller.signal.aborted)
            setSearchResults((old) => ({
              key: searchKey,
              contacts:
                searchOffset && old.key === searchKey
                  ? [
                      ...old.contacts,
                      ...data.contacts.filter(
                        (c) =>
                          !old.contacts.some(
                            (existing) => existing.id === c.id,
                          ),
                      ),
                    ]
                  : data.contacts,
              more: data.has_more,
            }))
        })
        .catch(() => {
          if (!controller.signal.aborted)
            setSearchError('Could not search contacts and notes. Try again.')
        })
        .finally(() => {
          if (!controller.signal.aborted) setSearching(false)
        })
    }, 250)
    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [query, tag, product, searchOffset, searchKey, searchRevision, refreshKey])
  const [drafts, setDrafts] = useState<Record<string, string[]>>({})
  const [tagInput, setTagInput] = useState(''),
    [saveError, setSaveError] = useState(''),
    [saved, setSaved] = useState(false),
    [saving, setSaving] = useState(false)
  const noteDirty = useRef(false)
  const detailRef = useRef<HTMLElement>(null),
    lastOpened = useRef<HTMLButtonElement | null>(null)
  const selectedIdRef = useRef(selectedId)
  selectedIdRef.current = selectedId
  const tags = [...new Set(contacts.flatMap((c) => c.tags))].sort()
  const sources = [...new Set(contacts.map(source))].sort()
  const filtered = (
    query.trim()
      ? searchResults.key === searchKey
        ? searchResults.contacts
        : []
      : contacts
  ).filter(
    (c) =>
      archive.isArchived(c.id) === showArchived && (!tag || c.tags.includes(tag)) && (!product || source(c) === product),
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
  async function load(silent = false) {
    if (!silent) { setLoading(true); setError('') }
    try {
      const data = await apiFetch<{ contacts: Contact[] }>('/api/v1/contacts')
      setContacts(data.contacts)
    } catch (e) {
      if (!silent) setError(e instanceof Error ? e.message : 'Could not load contacts')
    } finally {
      if (!silent) setLoading(false)
    }
  }
  useEffect(() => {
    void load()
  }, [refreshKey])
  useEffect(() => {
    const refresh = () => { if (document.visibilityState === 'visible' && !saving && !noteDirty.current) void load(true) }
    const timer = setInterval(refresh, 30000)
    window.addEventListener('focus', refresh)
    return () => { clearInterval(timer); window.removeEventListener('focus', refresh) }
  }, [saving])
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
    if (
      id !== selectedId &&
      noteDirty.current &&
      !confirm(t('Discard this unsaved note?'))
    )
      return
    noteDirty.current = false
    const next = new URLSearchParams(params)
    if (id) next.set('contact', id)
    else next.delete('contact')
    const match = filtered.find((c) => c.id === id)?.search_note
    if (id && query.trim() && match) {
      next.set('note_query', query.trim())
      next.set('note', match.id)
    } else {
      next.delete('note_query')
      next.delete('note')
    }
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
    void save(next)
  }
  async function save(tagsToSave?: string[]) {
    if (!selected || saving) return
    const next = tagsToSave ?? append(tagInput)
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
      {/* Left List: Clean Scanning Canvas without Tag Badge Clutter */}
      <aside
        aria-label={t('Contact list')}
        className={`min-h-0 min-w-0 w-full flex-col min-[1200px]:w-80 min-[1200px]:shrink-0 min-[1200px]:border-r border-border/40 ${selectedId ? 'hidden min-[1200px]:flex' : 'flex'}`}
      >
        <div className="shrink-0 border-b border-border/40 p-4">
          <ListCreateAction label="New contact" kind="contacts" onClick={onNewContact} />
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <ListSearch trailing={loading ? undefined : <span role="status" aria-label={t('Contacts')}>{filtered.length}</span>} aria-label={t('Search contacts and notes')} placeholder={t('Search contacts…')} value={query} onChange={event=>setQuery(event.target.value)} />
              <IconButton
                type="button"
                className="relative"
                label={t('Filters')}
                aria-expanded={filtersOpen}
                aria-controls="contact-filters"
                onClick={() => setFiltersOpen(!filtersOpen)}
              >
                <SlidersHorizontal className="h-4 w-4" />
                {(tag || product) && (
                  <span
                    aria-hidden="true"
                    className="absolute right-1 top-1 h-2 w-2 rounded-full bg-primary"
                  />
                )}
              </IconButton>
              <IconButton
                type="button"
                active={showArchived}
                label={t(showArchived ? 'Active contacts' : 'Archived contacts')}
                title={t(showArchived ? 'Active contacts' : 'Archived contacts')}
                aria-pressed={showArchived}
                onClick={() => setShowArchived((value) => !value)}
              >
                <Archive className="h-4 w-4" aria-hidden="true" />
              </IconButton>
              <ItemActions persistent label="Contacts" actions={[{ label: 'Reload contacts', disabled: loading || saving, run: async () => { await load(); setSearchOffset(0); setSearchRevision(value => value + 1) } }]}><span className="sr-only">{t('Contact list actions')}</span></ItemActions>
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
              <div id="contact-filters" className="grid grid-cols-2 gap-2 pt-1">
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
                    className="action-secondary col-span-2 text-xs"
                    onClick={clear}
                  >
                    {t('Clear filters')}
                  </button>
                )}
              </div>
            )}
          </div>
        </div>

        {archive.error && (
          <div role="alert" className="px-3 py-2 text-xs text-destructive">
            {t(archive.error)}{' '}
            <button className="min-h-11 underline" onClick={() => void archive.refresh()}>
              {t('Try again')}
            </button>
          </div>
        )}

        <div className="min-h-0 flex-1 overflow-y-auto px-1 py-1">
          {loading ? (
            <p role="status" className="py-6 text-center text-sm text-muted-foreground">
              {t('Loading contacts\u2026')}
            </p>
          ) : error ? (
            <div role="alert" className="rounded-lg border border-destructive/30 p-4 text-center">
              <p className="text-xs text-destructive">{t(error)}</p>
              <button
                className="action-secondary mt-2 text-xs"
                onClick={() => {
                  void load()
                  setSearchOffset(0)
                  setSearchRevision((value) => value + 1)
                }}
              >
                {t('Try again')}
              </button>
            </div>
          ) : contacts.length === 0 && !selectedId ? (
            <div className="rounded-xl border border-dashed border-border/60 p-6 text-center bg-muted/10">
              <Users className="mx-auto mb-3 h-8 w-8 text-muted-foreground/60" />
              <h2 className="text-base font-semibold text-foreground">{t('No contacts yet')}</h2>
              <p className="mx-auto mt-1.5 max-w-xs text-xs text-muted-foreground">
                {t('Live chat visitors appear here automatically. Set up your widget in Settings \u2192 Integrations.')}
              </p>
              <Link to="/settings/integrations/live-chat" className="action-primary mt-4 inline-flex text-xs">
                {t('Connect live chat')}
              </Link>
            </div>
          ) : (
            <div className="min-w-0">
              {query.trim() && searching && (
                <p role="status" className="py-2 text-center text-xs text-muted-foreground">
                  {t('Searching…')}
                </p>
              )}

              {searchError && (
                <div role="alert" className="space-y-2 p-3 text-xs text-destructive">
                  <p>{t(searchError)}</p>
                  <button
                    className="action-secondary text-xs"
                    onClick={() => setSearchRevision((value) => value + 1)}
                  >
                    {t('Try again')}
                  </button>
                </div>
              )}

              {!searching && !searchError && filtered.length === 0 ? (
                <div className="rounded-lg border border-border/40 p-4 text-center my-3 bg-muted/10">
                  <p className="text-xs font-medium text-foreground">{t('No matching contacts.')}</p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t('Try another search or clear your filters.')}
                  </p>
                  <button className="action-secondary mt-2.5 text-xs" onClick={clear}>
                    {t('Clear all filters')}
                  </button>
                </div>
              ) : (
                <div className="divide-y divide-border/30">
                  {filtered.map((c) => (
                    <ItemActions
                      key={c.id}
                      label={label(c)}
                      actions={[
                        { label: 'Open contact', run: () => select(c.id) },
                        {
                          label: 'Copy link',
                          run: () => copyItemLink('/contacts?contact=' + encodeURIComponent(c.id)),
                        },
                        {
                          label: showArchived ? 'Restore contact' : 'Archive contact',
                          disabled: !archive.loaded,
                          run: () => archive.archive(c.id, !showArchived),
                        },
                      ]}
                    >
                      {/* Clean row without stacked tag badge clutter */}
                      <button
                        key={c.id}
                        type="button"
                        disabled={saving}
                        onClick={(e) => {
                          lastOpened.current = e.currentTarget
                          select(c.id)
                        }}
                        aria-pressed={selectedId === c.id}
                        className={`contact-list-row flex min-h-[4.5rem] w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-muted/30 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${
                          selectedId === c.id ? 'bg-muted/50 font-medium' : ''
                        }`}
                      >
                        <ListRowContent leading={<span aria-hidden="true" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-foreground/80">{Array.from(label(c)).slice(0,2).join('').toUpperCase()}</span>} title={label(c)} description={c.email || t(source(c))}>
                          {c.search_note && query.trim() && <span className="mt-1 block text-xs text-muted-foreground"><span className="font-medium">{t('Note')}:</span> {c.search_note.snippet}</span>}
                        </ListRowContent>
                      </button>
                    </ItemActions>
                  ))}
                </div>
              )}

              {query.trim() && searchResults.key === searchKey && searchResults.more && (
                <button
                  className="action-secondary my-3 w-full text-xs"
                  disabled={searching}
                  onClick={() => setSearchOffset((value) => value + 50)}
                >
                  {t('Load more contacts')}
                </button>
              )}
            </div>
          )}
        </div>
        <ArchiveNotice state={archive} />
      </aside>

      {/* Right Detail: Serene Bear Note Pad Canvas (65-80ch Typographic Constraint, Zero Nested Cards) */}
      <div
        className={`min-h-0 min-w-0 flex-1 overflow-y-auto p-4 md:p-6 bg-background ${
          selectedId ? '' : 'hidden min-[1200px]:block'
        }`}
      >
        {selectedId ? (
          <aside
            ref={detailRef}
            tabIndex={-1}
            aria-label={t('Contact details')}
            className="mx-auto min-w-0 w-full max-w-[75ch] outline-none space-y-6"
          >
            <BackAction className="min-[1200px]:hidden" disabled={saving} onClick={() => select('')}>{t('Back to contacts')}</BackAction>

            {selected ? (
              <article className="space-y-5">
                {/* Bear Note Header: Serene Title & Understated Metadata */}
                <header className="border-b border-border/40 pb-5">
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0 flex-1 space-y-1.5">
                      <h2 className="break-words text-lg sm:text-xl font-semibold tracking-tight text-foreground font-sans">
                        {label(selected)}
                      </h2>
                      {selected.email && (
                        <p className="break-all text-sm text-muted-foreground">
                          {selected.email}
                        </p>
                      )}
                      <div className="flex flex-wrap items-center gap-2 pt-1">
                        <span
                          className="inline-flex items-center rounded-full bg-muted/60 px-2.5 py-0.5 text-xs text-muted-foreground border border-border/40"
                          title={t('Source')}
                        >
                          {t(source(selected))}
                        </span>
                        <span className="text-xs text-muted-foreground/60">·</span>
                        <span className="text-xs text-muted-foreground">
                          {t('Updated')} {dateLabel(selected.updated_at, language)}
                        </span>
                        {!selected.is_manual && (
                          <>
                            <span className="text-xs text-muted-foreground/60">·</span>
                            <Link
                              className="text-xs text-primary hover:underline inline-flex items-center gap-1"
                              to={`/inbox/${encodeURIComponent(selected.conversation_id)}`}
                            >
                              <MessageSquare className="h-3 w-3" />
                              <span>{t('Open conversation')}</span>
                            </Link>
                          </>
                        )}
                      </div>
                    </div>

                    <ItemActions
                      label={t('Contact details')}
                      actions={[
                        {
                          label: 'Copy link',
                          run: () =>
                            copyItemLink(
                              '/contacts?contact=' + encodeURIComponent(selected.id),
                            ),
                        },
                        {
                          label: archive.isArchived(selected.id)
                            ? 'Restore contact'
                            : 'Archive contact',
                          disabled: !archive.loaded,
                          run: () =>
                            archive.archive(
                              selected.id,
                              !archive.isArchived(selected.id),
                            ),
                        },
                      ]}
                    >
                      <span />
                    </ItemActions>
                  </div>

                  <details className="mt-3 text-xs text-muted-foreground">
                    <summary className="cursor-pointer text-muted-foreground hover:text-foreground inline-flex items-center gap-1 py-1">
                      {t('More details')}
                    </summary>
                    <div className="space-y-1 pt-2 pb-1 text-xs text-muted-foreground/80 pl-2 border-l border-border/40">
                      {selected.email && <p>{t('Email is unverified.')}</p>}
                      {selected.is_manual && <p>{t('No messaging channel connected.')}</p>}
                    </div>
                  </details>
                </header>

                {/* Bear Note Content: Contact Memory & Freeform Notes */}
                <div className="space-y-6">
                  <ContactMemoryPanel
                    key={selected.id + ':' + (params.get('note_query') || '')}
                    contactName={label(selected)}
                    contactId={selected.id}
                    initialQuery={params.get('note_query') || ''}
                    matchedNoteId={params.get('note') || ''}
                    onChanged={() => {
                      setSearchOffset(0)
                      setSearchRevision((value) => value + 1)
                    }}
                    onDirtyChange={(dirty) => {
                      noteDirty.current = dirty
                    }}
                  />

                  {/* Tags Section: Bear-style Hashtags at Bottom */}
                  <section
                    aria-label={t('Contact tags')}
                    className="border-t border-border/40 pt-4 space-y-3"
                  >
                    <div className="flex items-center justify-between">
                      <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                        <Tag className="h-3 w-3" />
                        <span>{t('Tags')}</span>
                      </h3>
                    </div>

                    <div className="flex flex-wrap items-center gap-1.5">
                      {draft.map((tItem) => (
                        <button
                          key={tItem}
                          type="button"
                          disabled={saving}
                          aria-label={t('Remove tag') + ' ' + tItem}
                          className="inline-flex min-h-[32px] max-w-full items-center gap-1.5 rounded-full bg-muted/40 border border-border/40 px-2.5 py-1 text-xs text-foreground hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-colors"
                          onClick={() => {
                            const next = draft.filter((value) => value !== tItem)
                            setDrafts((old) => ({ ...old, [selected.id]: next }))
                            void save(next)
                          }}
                        >
                          <span className="font-mono text-muted-foreground">#</span>
                          <span className="break-all">{tItem}</span>
                          <X className="h-3 w-3 shrink-0 text-muted-foreground" aria-hidden="true" />
                        </button>
                      ))}
                    </div>

                    <details className="mt-1">
                      <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1 py-1">
                        + {t('Add tag')}
                      </summary>
                      <form
                        className="max-w-md space-y-2 mt-2"
                        onSubmit={(event) => {
                          event.preventDefault()
                          add(tagInput)
                        }}
                      >
                        <div className="flex gap-2">
                          <input
                            id="contact-add-tag"
                            aria-label={t('Add tag')}
                            className="field-control min-w-0 flex-1 text-xs"
                            placeholder={t('For example: New customer')}
                            value={tagInput}
                            disabled={saving}
                            onChange={(event) => setTagInput(event.target.value)}
                          />
                          <button
                            type="submit"
                            className="action-secondary text-xs"
                            disabled={saving || !tagInput.trim()}
                          >
                            {t('Add')}
                          </button>
                        </div>
                        {!!suggestions.length && (
                          <div className="flex flex-wrap gap-1.5 pt-1">
                            {suggestions.map((sugTag) => (
                              <button
                                type="button"
                                key={sugTag}
                                disabled={saving}
                                className="action-secondary text-[11px] py-0.5 px-2 font-mono"
                                onClick={() => add(sugTag)}
                              >
                                #{sugTag}
                              </button>
                            ))}
                          </div>
                        )}
                      </form>
                    </details>

                    {saving && (
                      <p role="status" className="text-xs text-muted-foreground">
                        {t('Saving…')}
                      </p>
                    )}
                    {saved && !dirty && (
                      <p role="status" className="text-xs text-emerald-600 dark:text-emerald-400">
                        {t('Tags saved.')}
                      </p>
                    )}
                    {saveError && (
                      <div role="alert" className="text-xs text-destructive">
                        {t(saveError)}{' '}
                        <button
                          className="min-h-11 px-2 underline"
                          disabled={saving}
                          onClick={() => void save(draft)}
                        >
                          {t('Try again')}
                        </button>
                      </div>
                    )}
                  </section>
                </div>
              </article>
            ) : (
              <div className="py-12 text-center">
                <h2 className="text-base font-semibold">{t('Contact not found')}</h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t('This contact may no longer be available.')}
                </p>
                <button className="action-secondary mt-3 text-xs" onClick={() => select('')}>
                  {t('View contacts')}
                </button>
              </div>
            )}
          </aside>
        ) : (
          <aside className="flex h-full flex-col items-center justify-center p-6 text-center text-sm text-muted-foreground max-w-[75ch] mx-auto">
            <div className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-muted/30 text-muted-foreground/60 mb-3">
              <FileText className="h-6 w-6" />
            </div>
            <p className="text-xs text-muted-foreground">
              {t('Select a contact to view their details, tags, and conversation.')}
            </p>
          </aside>
        )}
      </div>
    </section>
  )
}
