import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  Search,
  Users,
  MessageSquare,
  RefreshCw,
  ChevronLeft,
  X,
} from 'lucide-react'
import { apiFetch } from '../../api/client'

type Contact = {
  id: string
  name: string
  email: string
  product: string
  conversation_id: string
  tags: string[]
  updated_at: string
}
const label = (c: Contact) => c.name || c.email || 'Pengunjung anonim'
const source = (c: Contact) => c.product.trim() || 'Live chat'
const dateLabel = (iso: string) => {
  const d = new Date(iso)
  return Number.isNaN(d.getTime())
    ? '—'
    : d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })
}

export function ContactsView() {
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
      setError(e instanceof Error ? e.message : 'Kontak gagal dimuat')
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => {
    void load()
  }, [])
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
      setSaveError('Tag maksimal 40 karakter.')
      return null
    }
    if (base.some((t) => t.toLowerCase() === text.toLowerCase())) return base
    if (base.length >= 20) {
      setSaveError('Maksimal 20 tag per kontak.')
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
        setSaveError(e instanceof Error ? e.message : 'Tag gagal disimpan')
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
    <section className="page-content" aria-label="Contacts">
      {selectedId && <h1 className="sr-only min-[1200px]:hidden">Contacts</h1>}
      <header
        className={`mb-5 items-start justify-between gap-3 ${selectedId ? 'hidden min-[1200px]:flex' : 'flex'}`}
      >
        <div>
          <h1 className="page-title">Contacts</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Pelanggan dan riwayat percakapan dalam satu tempat.
          </p>
        </div>
        <button
          className="icon-action"
          aria-label="Muat ulang kontak"
          onClick={() => void load()}
          disabled={loading || saving}
        >
          <RefreshCw className="h-5 w-5" />
        </button>
      </header>
      {contacts.length > 0 && (
        <div
          className={`mb-4 space-y-3 ${selectedId ? 'hidden min-[1200px]:block' : ''}`}
        >
          <label className="flex min-h-11 items-center gap-2 rounded-lg border bg-background px-3 focus-within:ring-2 focus-within:ring-ring">
            <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
            <input
              aria-label="Cari kontak"
              className="min-w-0 flex-1 bg-transparent py-2 text-base outline-none"
              placeholder="Cari nama, email, atau tag"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </label>
          <div className="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap">
            <select
              aria-label="Filter tag"
              className="field-control sm:w-auto sm:max-w-xs"
              value={tag}
              onChange={(e) => setTag(e.target.value)}
            >
              <option value="">Semua tag</option>
              {tags.map((t) => (
                <option key={t}>{t}</option>
              ))}
            </select>
            <select
              aria-label="Filter sumber"
              className="field-control sm:w-auto sm:max-w-xs"
              value={product}
              onChange={(e) => setProduct(e.target.value)}
            >
              <option value="">Semua sumber</option>
              {sources.map((s) => (
                <option key={s}>{s}</option>
              ))}
            </select>
            {filteredActive && (
              <button className="action-secondary col-span-2" onClick={clear}>
                Hapus filter
              </button>
            )}
          </div>
        </div>
      )}
      {loading ? (
        <p role="status" className="py-6 text-sm text-muted-foreground">
          Memuat kontak…
        </p>
      ) : error ? (
        <div role="alert" className="rounded-lg border p-5">
          <p>{error}</p>
          <button className="action-secondary mt-3" onClick={() => void load()}>
            Coba lagi
          </button>
        </div>
      ) : contacts.length === 0 && !selectedId ? (
        <div className="rounded-lg border border-dashed p-6 text-center">
          <Users className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
          <h2 className="text-lg font-semibold">Belum ada kontak</h2>
          <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
            Pengunjung live chat otomatis masuk di sini. Pasang widget melalui
            Settings → Integrations.
          </p>
          <Link
            to="/settings/integrations/live-chat"
            className="action-primary mt-4"
          >
            Hubungkan live chat
          </Link>
        </div>
      ) : (
        <div className="grid gap-5 min-[1200px]:grid-cols-[minmax(0,1fr)_360px]">
          <div
            className={`min-w-0 ${selectedId ? 'hidden min-[1200px]:block' : ''}`}
            aria-label="Daftar kontak"
          >
            <p className="mb-2 text-sm text-muted-foreground">
              {filteredActive
                ? `${filtered.length} dari ${contacts.length}`
                : contacts.length}{' '}
              kontak
            </p>
            {filtered.length === 0 ? (
              <div className="rounded-lg border p-5 text-center">
                <p className="font-medium">Tidak ada kontak yang cocok.</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  Coba kata lain atau hapus filter.
                </p>
                <button className="action-secondary mt-3" onClick={clear}>
                  Hapus semua filter
                </button>
              </div>
            ) : (
              <div className="overflow-hidden rounded-lg border divide-y divide-border/50">
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
                    className={`flex min-h-24 w-full items-start gap-3 p-3 text-left hover:bg-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2 ${selectedId === c.id ? 'bg-accent' : ''}`}
                  >
                    <span
                      aria-hidden="true"
                      className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-secondary text-sm font-semibold"
                    >
                      {Array.from(label(c)).slice(0, 2).join('').toUpperCase()}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-base font-semibold">
                        {label(c)}
                      </span>
                      <span className="mt-0.5 block truncate text-sm text-muted-foreground">
                        {c.email || 'Email belum tersedia'}
                      </span>
                      <span className="mt-1 block truncate text-xs text-muted-foreground">
                        {source(c)} · Diperbarui {dateLabel(c.updated_at)}
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
          {selectedId ? (
            <aside
              ref={detailRef}
              tabIndex={-1}
              aria-label="Detail kontak"
              className="min-w-0 self-start rounded-lg border bg-card p-4 outline-none sm:p-5"
            >
              <button
                className="action-secondary mb-4 min-[1200px]:hidden"
                disabled={saving}
                onClick={() => select('')}
              >
                <ChevronLeft className="h-4 w-4" />
                Kembali ke daftar kontak
              </button>
              {selected ? (
                <>
                  <h2 className="break-words text-xl font-semibold">
                    {label(selected)}
                  </h2>
                  <p className="mt-1 break-all text-sm text-muted-foreground">
                    {selected.email || 'Email belum tersedia'}
                  </p>
                  <p className="mt-2 text-xs text-muted-foreground">
                    {source(selected)} · Diperbarui{' '}
                    {dateLabel(selected.updated_at)}
                  </p>
                  {selected.email && (
                    <p className="mt-2 text-xs text-muted-foreground">
                      Email dari widget, belum terverifikasi.
                    </p>
                  )}
                  <Link
                    className="action-primary my-5 w-full"
                    to={`/inbox/${encodeURIComponent(selected.conversation_id)}`}
                  >
                    <MessageSquare className="h-4 w-4" />
                    Buka percakapan
                  </Link>
                  <form
                    aria-label="Kelola tag"
                    className="space-y-3 border-t pt-4"
                    onSubmit={(e) => {
                      e.preventDefault()
                      void save()
                    }}
                  >
                    <h3 className="text-base font-semibold">Tag</h3>
                    <div
                      aria-label="Tag kontak"
                      className="flex flex-wrap gap-2"
                    >
                      {draft.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                          Belum ada tag.
                        </p>
                      ) : (
                        draft.map((t) => (
                          <button
                            type="button"
                            key={t}
                            disabled={saving}
                            aria-label={`Hapus tag ${t}`}
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
                      Tambah tag
                    </label>
                    <div className="flex gap-2">
                      <input
                        id="contact-add-tag"
                        className="field-control flex-1"
                        placeholder="Misalnya: Pelanggan baru"
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
                        Tambah
                      </button>
                    </div>
                    {suggestions.length > 0 && (
                      <div aria-label="Tag yang tersedia" className="space-y-1">
                        <p className="text-xs text-muted-foreground">
                          Gunakan tag yang sudah ada
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
                      {draft.length}/20 tag · maksimal 40 karakter per tag.
                      Enter untuk menambahkan.
                    </p>
                    {saveError && (
                      <p role="alert" className="text-sm text-destructive">
                        {saveError}
                      </p>
                    )}
                    {dirty && !saving && (
                      <p className="text-sm text-muted-foreground">
                        Perubahan belum disimpan.
                      </p>
                    )}
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="submit"
                        className="action-primary"
                        disabled={saving || !dirty}
                      >
                        {saving ? 'Menyimpan…' : 'Simpan tag'}
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
                          Batalkan perubahan
                        </button>
                      )}
                    </div>
                    {saved && (
                      <p role="status" className="text-sm">
                        Tag tersimpan.
                      </p>
                    )}
                  </form>
                </>
              ) : (
                <>
                  <h2 className="text-lg font-semibold">
                    Kontak tidak ditemukan
                  </h2>
                  <p className="mt-2 text-sm text-muted-foreground">
                    Kontak ini mungkin tidak lagi tersedia.
                  </p>
                  <button
                    className="action-secondary mt-3"
                    onClick={() => select('')}
                  >
                    Buka daftar kontak
                  </button>
                </>
              )}
            </aside>
          ) : (
            <aside className="hidden min-[1200px]:block self-start rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
              <Users className="mx-auto mb-3 h-8 w-8" />
              <p>Pilih kontak untuk melihat detail, tag, dan percakapannya.</p>
            </aside>
          )}
        </div>
      )}
    </section>
  )
}
