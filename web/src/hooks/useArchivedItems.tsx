import { useCallback, useEffect, useState } from 'react'
import { apiFetch } from '../api/client'
import { useLanguage } from '../context/LanguageContext'

type Items = Record<string, { archived_at?: string; read_at?: string; unread?: boolean }>
export async function updateItemPreference(kind: 'conversation' | 'contact', id: string, patch: { archived?: boolean; read?: boolean; seen_at?: string }) {
  const data = await apiFetch<{ items: Items }>('/api/v1/item-preferences?kind=' + kind, { method: 'PATCH', body: JSON.stringify({ id, ...patch }) })
  window.dispatchEvent(new CustomEvent('cuan-item-preferences', { detail: { kind, items: data.items } }))
  return data.items
}
export function useArchivedItems(kind: 'conversation' | 'contact') {
  const [items, setItems] = useState<Items>({})
  const [loaded, setLoaded] = useState(false), [error, setError] = useState('')
  const [notice, setNotice] = useState<{ id: string; archived: boolean }>()
  const url = '/api/v1/item-preferences?kind=' + kind
  const refresh = useCallback(async () => {
    try {
      const data = await apiFetch<{ items: Items }>(url)
      setItems(data.items); setLoaded(true); setError('')
    } catch { setError('Could not load archived items. Try again.') }
  }, [url])
  useEffect(() => {
    void refresh()
    const resume = () => { if (document.visibilityState === 'visible') void refresh() }
    window.addEventListener('focus', resume)
    document.addEventListener('visibilitychange', resume)
    return () => { window.removeEventListener('focus', resume); document.removeEventListener('visibilitychange', resume) }
  }, [refresh])
  useEffect(() => {
    const changed = (event: Event) => {
      const data = (event as CustomEvent).detail
      if (data.kind === kind) { setItems(data.items); setLoaded(true); setError('') }
    }
    window.addEventListener('cuan-item-preferences', changed)
    return () => window.removeEventListener('cuan-item-preferences', changed)
  }, [kind])
  const update = async (id: string, archived: boolean, undo = false) => {
    const items = await updateItemPreference(kind, id, { archived })
    setItems(items); setNotice(undo ? undefined : { id, archived })
  }
  return {
    loaded, error, refresh, notice,
    archive: update,
    markRead: async (id: string, read: boolean, seen_at?: string) => { await updateItemPreference(kind, id, { read, seen_at }) },
    unreadCount: (id: string, fallback: number, latestMessage?: string) => {
      const value = items[id]
      if (value?.unread) return Math.max(1, fallback)
      const read = Date.parse(value?.read_at || '')
      const latest = Date.parse(latestMessage || '')
      if (Number.isFinite(read) && Number.isFinite(latest) && latest <= read) return 0
      return fallback
    },
    undo: async () => { if (notice) await update(notice.id, !notice.archived, true) },
    dismiss: () => setNotice(undefined),
    isArchived: (id: string, latestMessage?: string) => {
      const at = Date.parse(items[id]?.archived_at || '')
      return Number.isFinite(at) && (!latestMessage || !Number.isFinite(Date.parse(latestMessage)) || Date.parse(latestMessage) <= at)
    },
  }
}
export function ArchiveNotice({ state }: { state: ReturnType<typeof useArchivedItems> }) {
  const { t } = useLanguage()
  const [busy, setBusy] = useState(false), [error, setError] = useState('')
  if (!state.notice) return null
  return <div role="status" className="flex flex-wrap items-center justify-between gap-2 border-t bg-background px-3 py-2 text-xs">
    <span>{t(error || (state.notice.archived ? 'Archived' : 'Restored'))}</span>
    <span className="flex items-center gap-2">
      <button type="button" className="min-h-11 px-2 font-medium underline" disabled={busy} onClick={async () => {
        setBusy(true);setError('')
        try { await state.undo() } catch { setError('Could not undo. Try again.') } finally { setBusy(false) }
      }}>{t('Undo')}</button>
      <button type="button" className="min-h-11 px-2 text-muted-foreground" onClick={state.dismiss}>{t('Dismiss')}</button>
    </span>
  </div>
}
