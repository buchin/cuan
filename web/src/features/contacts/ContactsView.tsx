import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Search, Users, MessageSquare, RefreshCw } from 'lucide-react'
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
export function ContactsView() {
  const [contacts, setContacts] = useState<Contact[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [tag, setTag] = useState('')
  const [selected, setSelected] = useState<Contact | null>(null)
  const [draft, setDraft] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [saved, setSaved] = useState(false)
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
  const tags = [...new Set(contacts.flatMap((c) => c.tags))].sort()
  const filtered = contacts.filter(
    (c) =>
      (!tag || c.tags.includes(tag)) &&
      [c.name, c.email, c.product, ...c.tags]
        .join(' ')
        .toLowerCase()
        .includes(query.toLowerCase()),
  )
  async function save() {
    if (!selected) return
    setSaving(true)
    setSaved(false)
    setSaveError('')
    try {
      const result = await apiFetch<{ tags: string[] }>(
        `/api/v1/contacts/${selected.id}`,
        {
          method: 'PATCH',
          body: JSON.stringify({
            tags: draft
              .split(',')
              .map((v) => v.trim())
              .filter(Boolean),
          }),
        },
      )
      setContacts((old) =>
        old.map((c) =>
          c.id === selected.id ? { ...c, tags: result.tags } : c,
        ),
      )
      setSelected({ ...selected, tags: result.tags })
      setDraft(result.tags.join(', '))
      setSaved(true)
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Tag gagal disimpan')
    } finally {
      setSaving(false)
    }
  }
  return (
    <section className="page-content" aria-label="Contacts">
      <header className="mb-6 flex items-start justify-between gap-3">
        <div>
          <h1 className="page-title">Contacts</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Pelanggan dari live chat, dengan identitas dan tag dalam satu
            tempat.
          </p>
        </div>
        <button
          className="icon-action shrink-0"
          aria-label="Muat ulang kontak"
          onClick={() => void load()}
          disabled={loading}
        >
          <RefreshCw className="h-5 w-5" />
        </button>
      </header>
      {contacts.length > 0 && (
        <div className="mb-5 flex flex-col gap-3 sm:flex-row">
          <label className="flex min-h-11 min-w-0 flex-1 items-center gap-2 rounded-lg border bg-background px-3">
            <Search className="h-4 w-4 shrink-0" />
            <input
              aria-label="Cari kontak"
              className="min-w-0 flex-1 bg-transparent py-2 text-base outline-none"
              placeholder="Cari nama, email, produk, atau tag"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </label>
          <select
            aria-label="Filter tag"
            className="min-h-11 max-w-full rounded-lg border bg-background px-3 text-base"
            value={tag}
            onChange={(e) => setTag(e.target.value)}
          >
            <option value="">Semua tag</option>
            {tags.map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
        </div>
      )}
      {loading ? (
        <p role="status">Memuat kontak…</p>
      ) : error ? (
        <div role="alert" className="rounded-xl border p-5">
          <p>{error}</p>
          <button
            className="mt-3 min-h-11 underline"
            onClick={() => void load()}
          >
            Coba lagi
          </button>
        </div>
      ) : contacts.length === 0 ? (
        <div className="rounded-xl border border-dashed p-8 text-center">
          <Users className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
          <h2 className="text-lg font-semibold">Belum ada kontak</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Pengunjung yang membuka live chat akan otomatis masuk di sini.
            Tambahkan email melalui konfigurasi widget untuk mengenalinya.
          </p>
          <Link
            to="/settings/integrations/live-chat"
            className="action-primary mt-4"
          >
            Hubungkan live chat
          </Link>
        </div>
      ) : (
        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
          <div>
            <p className="mb-3 text-sm text-muted-foreground">
              {filtered.length} kontak
            </p>
            <div className="space-y-2">
              {filtered.length === 0 && (
                <p className="rounded-xl border p-5">
                  Tidak ada kontak yang cocok. Ubah pencarian atau filter.
                </p>
              )}
              {filtered.map((c) => (
                <button
                  key={c.id}
                  disabled={saving}
                  onClick={() => {
                    setSaved(false)
                    setSelected(c)
                    setDraft(c.tags.join(', '))
                    setSaveError('')
                    requestAnimationFrame(() =>
                      document
                        .getElementById('contact-detail')
                        ?.scrollIntoView({
                          block: 'nearest',
                          behavior: 'smooth',
                        }),
                    )
                  }}
                  aria-pressed={selected?.id === c.id}
                  className={`w-full rounded-xl border p-4 text-left transition-colors hover:bg-accent ${selected?.id === c.id ? 'border-primary bg-accent' : ''}`}
                >
                  <span className="block break-words text-base font-semibold">
                    {c.name || 'Pengunjung anonim'}
                  </span>
                  <span className="mt-1 block break-all text-sm text-muted-foreground">
                    {c.email || 'Email belum tersedia'}
                  </span>
                  <span className="mt-2 block text-xs text-muted-foreground">
                    {c.product || 'Live chat'} ·{' '}
                    {new Date(c.updated_at).toLocaleDateString('id-ID')}
                  </span>
                  <span className="mt-2 flex flex-wrap gap-1">
                    {c.tags.map((t) => (
                      <span
                        key={t}
                        className="break-all rounded-md bg-secondary px-2 py-1 text-xs"
                      >
                        {t}
                      </span>
                    ))}
                  </span>
                </button>
              ))}
            </div>
          </div>
          {selected && (
            <aside
              id="contact-detail"
              className="order-first min-w-0 self-start rounded-xl border bg-card p-5 xl:order-last"
              aria-label="Detail kontak"
            >
              <button
                className="mb-3 min-h-11 text-sm text-primary underline xl:hidden"
                onClick={() => setSelected(null)}
              >
                Kembali ke daftar kontak
              </button>
              <h2 className="break-words text-lg font-semibold">
                {selected.name || 'Pengunjung anonim'}
              </h2>
              <p className="mt-2 break-all text-sm">
                {selected.email || 'Email belum tersedia'}
              </p>
              {selected.email && (
                <p className="mt-1 text-xs text-muted-foreground">
                  Email diklaim oleh widget; belum diverifikasi.
                </p>
              )}
              <Link
                className="my-4 inline-flex min-h-11 items-center gap-2 rounded-lg bg-primary px-4 text-sm text-primary-foreground"
                to={`/inbox/${selected.conversation_id}`}
              >
                <MessageSquare className="h-4 w-4" />
                Buka percakapan
              </Link>
              <label
                className="block text-sm font-medium"
                htmlFor="contact-tags"
              >
                Tag
              </label>
              <textarea
                id="contact-tags"
                className="mt-2 min-h-24 w-full rounded-lg border bg-background p-3 text-base"
                value={draft}
                onChange={(e) => {
                  setDraft(e.target.value)
                  setSaved(false)
                }}
                disabled={saving}
              />
              <p className="mt-1 text-xs text-muted-foreground">
                Pisahkan dengan koma. Maksimal 20 tag, 40 karakter per tag.
              </p>
              {saveError && (
                <p role="alert" className="mt-3 text-sm text-destructive">
                  {saveError}
                </p>
              )}
              <button
                className="mt-3 min-h-11 rounded-lg bg-primary px-4 text-sm text-primary-foreground disabled:opacity-50"
                disabled={saving}
                onClick={() => void save()}
              >
                {saving ? 'Menyimpan…' : 'Simpan tag'}
              </button>
              {saved && (
                <p role="status" className="mt-2 text-sm">
                  Tag tersimpan.
                </p>
              )}
              <p className="mt-4 text-xs text-muted-foreground">
                Tag membantu mengelompokkan kontak. Pengiriman broadcast belum
                tersedia.
              </p>
            </aside>
          )}
        </div>
      )}
    </section>
  )
}
