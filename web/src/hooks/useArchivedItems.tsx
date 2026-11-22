import { useCallback, useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { apiFetch } from '../api/client'
import { useLanguage } from '../context/LanguageContext'

type Items = Record<string, { archived_at?: string; read_at?: string; unread?: boolean }>
export async function updateItemPreference(kind: 'conversation' | 'contact', id: string, patch: { archived?: boolean; read?: boolean; seen_at?: string }) {
  const data = await apiFetch<{ items: Items }>('/api/v1/item-preferences?kind=' + kind, { method: 'PATCH', body: JSON.stringify({ id, ...patch }) })
  window.dispatchEvent(new CustomEvent('cuan-item-preferences', { detail: { kind, items: data.items } }))
  return data.items
}
export async function markAllItemPreferencesRead(kind: 'conversation' | 'contact', ids?: string[]) {
  const body = ids && ids.length > 0 ? { ids, read: true } : { all: true, read: true }
  const data = await apiFetch<{ items: Items }>('/api/v1/item-preferences?kind=' + kind, { method: 'PATCH', body: JSON.stringify(body) })
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
      setItems(data?.items || {}); setLoaded(true); setError('')
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
  const update = useCallback(async (id: string, archived: boolean, undo = false) => {
    const items = await updateItemPreference(kind, id, { archived })
    setItems(items)
    setNotice(undo ? undefined : { id, archived })
  }, [kind])

  const dismiss = useCallback(() => setNotice(undefined), [])

  const markRead = useCallback(async (id: string, read: boolean, seen_at?: string) => {
    await updateItemPreference(kind, id, { read, seen_at })
  }, [kind])

  const markAllRead = useCallback(async (ids?: string[]) => {
    const updated = await markAllItemPreferencesRead(kind, ids)
    setItems(updated)
    return updated
  }, [kind])

  const unreadCount = useCallback((id: string, fallback: number, latestMessage?: string) => {
    const value = items?.[id]
    if (value?.unread) return Math.max(1, fallback)
    const read = Date.parse(value?.read_at || '')
    const latest = Date.parse(latestMessage || '')
    if (Number.isFinite(read) && Number.isFinite(latest) && latest <= read) return 0
    return fallback
  }, [items])

  const undo = useCallback(async () => {
    if (notice) await update(notice.id, !notice.archived, true)
  }, [notice, update])

  const isArchived = useCallback((id: string, latestMessage?: string) => {
    const at = Date.parse(items?.[id]?.archived_at || '')
    return Number.isFinite(at) && (!latestMessage || !Number.isFinite(Date.parse(latestMessage)) || Date.parse(latestMessage) <= at)
  }, [items])

  return useMemo(() => ({
    loaded,
    error,
    refresh,
    notice,
    archive: update,
    markRead,
    markAllRead: markAllRead,
    unreadCount,
    undo,
    dismiss,
    isArchived,
  }), [loaded, error, refresh, notice, update, markRead, markAllRead, unreadCount, undo, dismiss, isArchived])
}
export function ArchiveNotice({ state }: { state: ReturnType<typeof useArchivedItems> }) {
  const { t } = useLanguage()
  const [busy, setBusy] = useState(false), [error, setError] = useState('')
  const [hovered, setHovered] = useState(false), [focused, setFocused] = useState(false)
  useEffect(() => { setError('') }, [state.notice])
  useEffect(() => {
    if (!state.notice || busy || error || hovered || focused) return
    const timer = window.setTimeout(state.dismiss, 8000)
    return () => window.clearTimeout(timer)
  }, [state.notice, state.dismiss, busy, error, hovered, focused])
  if (!state.notice) return null
  return createPortal(<div data-archive-notice role="status" aria-live="polite" aria-atomic="true"
    onPointerEnter={event => { if (event.pointerType === 'mouse') setHovered(true) }}
    onPointerLeave={event => { if (event.pointerType === 'mouse') setHovered(false) }}
    onFocusCapture={() => setFocused(true)}
    onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false) }}
    className="fixed inset-x-4 bottom-[calc(80px+env(safe-area-inset-bottom))] z-50 mx-auto flex max-w-sm items-center gap-2 rounded-lg border bg-popover px-3 py-1 text-sm text-popover-foreground shadow-lg md:bottom-4 md:left-auto md:right-4 md:mx-0 md:w-80">
    <span className="flex-1">{t(error || (state.notice.archived ? 'Archived' : 'Restored'))}</span>
    <button type="button" className="min-h-11 px-2 font-medium underline" disabled={busy} onClick={async () => {
      setBusy(true); setError('')
      try { await state.undo() } catch { setError('Could not undo. Try again.') } finally { setBusy(false) }
    }}>{t('Undo')}</button>
    <button type="button" className="icon-action text-muted-foreground" aria-label={t('Dismiss')} onClick={state.dismiss}><X className="h-4 w-4" aria-hidden="true" /></button>
  </div>, document.body)
}
