import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { Bell, X } from 'lucide-react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useLanguage } from '../../context/LanguageContext'
import { useNotifications } from '../../context/NotificationsContext'
import { NotificationRow } from './NotificationsView'

export function NotificationsPopover() {
  const { items, loading, error, refresh, markRead } = useNotifications()
  const { t, language } = useLanguage()
  const location = useLocation(), navigate = useNavigate(), id = useId()
  const panel = useRef<HTMLDivElement>(null), trigger = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false), [busy, setBusy] = useState(false)
  const unread = items.filter(item => !item.read && !item.resolved).length
  const relative = useMemo(() => new Intl.RelativeTimeFormat(language, {numeric:'auto', style:'short'}), [language])
  const absolute = useMemo(() => new Intl.DateTimeFormat(language, {dateStyle:'medium',timeStyle:'short'}), [language])
  const close = () => panel.current?.hidePopover?.()
  useEffect(() => {
    const element = panel.current
    if (!element) return
    const toggle = () => {
      const visible = element.matches(':popover-open')
      setOpen(visible)
      if (visible) element.querySelector<HTMLElement>('h2')?.focus()
      else if (element.contains(document.activeElement)) trigger.current?.focus()
    }
    element.addEventListener('toggle', toggle)
    return () => element.removeEventListener('toggle', toggle)
  }, [])
  useEffect(() => { panel.current?.hidePopover?.() }, [location.pathname, location.search])
  const timeLabel = (value: string) => {
    const date = Date.parse(value), elapsed = Math.max(0, Date.now()-date)
    if (!Number.isFinite(date)) return ''
    if (elapsed < 60000) return t('Just now')
    if (elapsed < 3600000) return relative.format(-Math.floor(elapsed/60000), 'minute')
    if (elapsed < 86400000) return relative.format(-Math.floor(elapsed/3600000), 'hour')
    return relative.format(-Math.floor(elapsed/86400000), 'day')
  }
  const returnState = { notificationReturnTo: location.pathname+location.search+location.hash }
  return <>
    <button ref={trigger} type="button" className="icon-action relative" aria-label={t('Notifications')} aria-haspopup="dialog" aria-expanded={open} aria-controls={id} onClick={() => {
      if (window.matchMedia('(max-width: 767px)').matches || !panel.current?.showPopover) { navigate('/notifications', {state:returnState}); return }
      if (!open) void refresh()
      panel.current.togglePopover()
    }}>
      <Bell className="h-5 w-5" />
      {unread > 0 && <span aria-hidden="true" className="absolute right-0 top-0 min-w-4 rounded-full bg-primary px-1 text-[10px] leading-4 text-primary-foreground">{unread > 99 ? '99+' : unread}</span>}
    </button>
    <div {...{popover:'auto'}} ref={panel} id={id} hidden={typeof HTMLElement === 'undefined' || !('showPopover' in HTMLElement.prototype)} role="dialog" aria-modal="false" aria-labelledby={id+'-title'} className="fixed inset-x-0 bottom-0 top-auto m-0 max-h-[80dvh] w-full max-w-none overflow-hidden rounded-t-2xl border border-border bg-popover p-0 text-popover-foreground shadow-xl md:inset-auto md:right-4 md:top-[4.5rem] md:w-96 md:rounded-xl">
      <header className="flex items-center gap-2 border-b px-3 py-2">
        <h2 id={id+'-title'} tabIndex={-1} className="min-w-0 flex-1 text-sm font-semibold outline-none">{t('Notifications')}</h2>
        <button className="min-h-11 rounded-md px-2 text-xs font-medium text-primary hover:bg-accent disabled:opacity-50" disabled={busy || unread === 0} onClick={async () => {
          if (busy) return
          setBusy(true)
          try { await markRead() } catch { /* Context retains error and unread state for retry. */ } finally { setBusy(false) }
        }}>{t(busy ? 'Saving…' : 'Mark all as read')}</button>
        <button className="icon-action" aria-label={t('Close notifications')} onClick={close}><X className="h-4 w-4" /></button>
      </header>
      <div className="max-h-[55dvh] overflow-y-auto overscroll-contain px-2">
        {error && <p role="alert" className="p-3 text-xs text-destructive">{t(error)} <button className="min-h-11 underline" onClick={() => void refresh()}>{t('Retry')}</button></p>}
        {loading && !items.length ? <p role="status" className="p-4 text-sm">{t('Loading…')}</p> : !items.length && !error ? <p className="p-6 text-center text-sm text-muted-foreground">{t('You’re all caught up')}</p> : <ul className="divide-y divide-border/60">{items.slice(0,6).map(item => <NotificationRow key={item.id} item={item} timeLabel={timeLabel(item.created_at)} absoluteLabel={Number.isFinite(Date.parse(item.created_at)) ? absolute.format(new Date(item.created_at)) : undefined} />)}</ul>}
      </div>
      <footer className="border-t p-2"><Link className="flex min-h-11 items-center justify-center rounded-md text-sm font-medium hover:bg-accent" to="/notifications" state={returnState} onClick={close}>{t('View all notifications')}</Link></footer>
    </div>
  </>
}
