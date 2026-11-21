import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { InstallNudge } from '../settings/InstallNudge'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Bell, CheckCheck, MoreHorizontal, Settings, ArrowUpRight, CircleAlert, CircleCheck, ClipboardCheck, MessageCircle } from 'lucide-react'
import type { AppNotification } from '../../context/NotificationsContext'
import { useNotifications } from '../../context/NotificationsContext'
import { useLanguage } from '../../context/LanguageContext'
export function NotificationsView() {
  const { items, loading, error, refresh } = useNotifications(),
    { t, language } = useLanguage()
  const location = useLocation()
  const filter = new URLSearchParams(location.search).get('filter') === 'unread' ? 'unread' : 'all'
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    const update = () => setNow(Date.now())
    const timer = window.setInterval(update, 60000)
    document.addEventListener('visibilitychange', update)
    return () => { window.clearInterval(timer); document.removeEventListener('visibilitychange', update) }
  }, [])
  const relative = useMemo(() => new Intl.RelativeTimeFormat(language, {numeric:'auto', style:'short'}), [language])
  const absolute = useMemo(() => new Intl.DateTimeFormat(language, {dateStyle:'medium',timeStyle:'short'}), [language])
  const visible = items.filter((x) => filter === 'all' || (!x.read && !x.resolved))
  const today = new Date(now); today.setHours(0,0,0,0)
  const yesterday = new Date(today); yesterday.setDate(yesterday.getDate()-1)
  const groups = ['Today','Yesterday','Earlier'].map(label => ({label, items: visible.filter(item => {
    const date = Date.parse(item.created_at)
    return label === (date >= today.getTime() ? 'Today' : date >= yesterday.getTime() ? 'Yesterday' : 'Earlier')
  })})).filter(group => group.items.length)
  const timeLabel = (value: string) => {
    const date = Date.parse(value)
    if (!Number.isFinite(date)) return ''
    const elapsed = Math.max(0,now-date)
    if (elapsed < 60000) return t('Just now')
    if (elapsed < 3600000) return relative.format(-Math.floor(elapsed/60000),'minute')
    if (elapsed < 86400000) return relative.format(-Math.floor(elapsed/3600000),'hour')
    if (elapsed < 7*86400000) return relative.format(-Math.floor(elapsed/86400000),'day')
    return new Intl.DateTimeFormat(language,{month:'short',day:'numeric',...(new Date(date).getFullYear()!==new Date(now).getFullYear()?{year:'numeric' as const}:{})}).format(date)
  }
  return (
    <section aria-label={t('Notifications')} className="mx-auto max-w-3xl space-y-5">
      <InstallNudge />
      {error && <div role="alert" className="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">{t(error)}{' '}<button className="underline" onClick={() => void refresh()}>{t('Retry')}</button></div>}
      {loading && !items.length ? <p role="status" className="text-sm">{t('Loading…')}</p> : !visible.length ? (
        !error && <div className="py-12 text-center"><Bell className="mx-auto mb-3 h-7 w-7 text-muted-foreground" /><p className="font-medium">{t('You’re all caught up')}</p><p className="mx-auto mt-2 max-w-sm text-sm text-muted-foreground">{t('Approvals, blockers, completed work and new customer messages appear here.')}</p></div>
      ) : groups.map(group => (
        <section key={group.label} aria-label={t(group.label)}>
          <h2 className="mb-2 px-2 text-xs font-medium text-muted-foreground">{t(group.label)}</h2>
          <ul className="divide-y divide-border/60">
            {group.items.map(item => <NotificationRow key={item.id} item={item} timeLabel={timeLabel(item.created_at)} absoluteLabel={Number.isFinite(Date.parse(item.created_at)) ? absolute.format(new Date(item.created_at)) : undefined} />)}
          </ul>
        </section>
      ))}
    </section>
  )
}


export function NotificationRow({item, timeLabel, absoluteLabel}: {item: AppNotification; timeLabel: string; absoluteLabel?: string}) {
  const {markRead} = useNotifications(), {t} = useLanguage()
  const navigate = useNavigate()
  const unread = !item.read && !item.resolved
              const Icon = item.resolved || item.kind === 'completed' ? CircleCheck : item.kind === 'approval' ? ClipboardCheck : item.kind === 'customer' ? MessageCircle : CircleAlert
              const status = item.status || ({approval:'Needs approval',blocker:'Needs attention',completed:'Completed',customer:'Customer message'}[item.kind])
              const action = item.action_label || (item.href.startsWith('/inbox/') ? 'Open conversation' : 'View work')
              const href = /^\/(inbox|projects|tasks)(\/|$)/.test(item.href) ? item.href : '/chat'
              const readAction = { label: 'Mark as read', previewLabel: 'Read', disabled: !unread, run: () => markRead([item.id]) }
              return <li key={item.id}>
                <ItemActions label={t(item.title)} swipeAction={readAction} actions={[
                  { label: action, run: () => { navigate(href); if (!item.read) void markRead([item.id]).catch(() => {}) } },
                  readAction,
                  { label: 'Copy link', run: () => copyItemLink(href) },
                ]}>
                <Link to={href} onClick={() => { if (!item.read) void markRead([item.id]).catch(() => {}) }} className="group flex w-full gap-3 rounded-lg px-2 py-3 text-left transition-colors hover:bg-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring">
                  <span aria-hidden="true" className={'mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full '+(unread ? (item.kind === 'blocker' ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400' : item.kind === 'completed' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-primary/10 text-primary') : 'bg-muted text-muted-foreground')}><Icon className="h-4 w-4" /></span>
                  <span className="min-w-0 flex-1">
                    <span className="flex min-h-4 items-center gap-2 text-xs text-muted-foreground">
                      <span className="min-w-0 flex-1 truncate">{item.context && item.context !== item.title ? item.context : t(item.kind === 'customer' ? 'Live chat' : item.href.startsWith('/inbox/') ? 'Conversation' : 'Project work')}</span>
                      <time className="shrink-0 whitespace-nowrap tabular-nums" dateTime={item.created_at} title={absoluteLabel}>{timeLabel}</time>
                      <span aria-label={item.resolved ? t('No action needed') : item.read ? t('Read') : t('Unread')} className={'h-1.5 w-1.5 shrink-0 rounded-full '+(unread ? 'bg-primary' : 'bg-transparent')} />
                    </span>
                    <span title={item.title} className={'mt-1 line-clamp-2 break-words text-sm leading-5 '+(unread ? 'font-semibold' : 'font-normal')}>{t(item.title)}</span>
                    {item.summary && <span className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{t(item.summary)}</span>}
                    <span className="mt-1.5 flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs">
                      <span className={item.resolved ? 'text-muted-foreground' : item.kind === 'blocker' ? 'text-amber-700 dark:text-amber-400' : item.kind === 'completed' ? 'text-emerald-600 dark:text-emerald-400' : item.kind === 'approval' ? 'text-primary' : 'text-muted-foreground'}>{t(status)}</span>
                      <span className="inline-flex items-center gap-1 text-muted-foreground group-hover:text-foreground">{t(action)}<ArrowUpRight className="h-3 w-3" aria-hidden="true" /></span>
                    </span>
                  </span>
                </Link>
                </ItemActions>
              </li>
}

export function NotificationsNavigation() {
  const { items } = useNotifications(),
    { t } = useLanguage(),
    location = useLocation()
  const unread = items.filter((item) => !item.read && !item.resolved).length,
    filter =
      new URLSearchParams(location.search).get('filter') === 'unread'
        ? 'unread'
        : 'all'
  return (
    <nav
      aria-label={t('Notification filters')}
      className="flex shrink-0 items-center gap-4 border-b border-border bg-transparent px-4 md:px-6"
    >
      {[
        { id: 'all', label: 'All', to: '/notifications' },
        { id: 'unread', label: 'Unread', to: '/notifications?filter=unread' },
      ].map((item) => (
        <Link
          key={item.id}
          to={item.to}
          state={location.state}
          replace
          aria-current={filter === item.id ? 'page' : undefined}
          className={
            'inline-flex min-h-11 items-center gap-2 border-b-2 px-1 text-sm bg-transparent focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ' +
            (filter === item.id
              ? 'border-primary font-semibold text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground')
          }
        >
          {t(item.label)}
          {item.id === 'unread' && (
            <span className="rounded-full bg-muted px-1.5 text-xs tabular-nums">
              {unread}
            </span>
          )}
        </Link>
      ))}
      <div className="ml-auto shrink-0"><MarkAllReadButton /></div>
    </nav>
  )
}
function MarkAllReadButton() {
  const { items, markRead } = useNotifications(), { t } = useLanguage()
  const [busy, setBusy] = useState(false)
  return (
    <button
      type="button"
      aria-label={t('Mark all as read')}
      title={t('Mark all as read')}
      aria-busy={busy}
      disabled={busy || !items.some(item => !item.read && !item.resolved)}
      className="inline-flex min-h-11 min-w-11 items-center justify-center gap-2 rounded-md px-2 text-xs font-medium text-primary hover:bg-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring disabled:text-muted-foreground disabled:opacity-50"
      onClick={async () => {
        if (busy) return
        setBusy(true)
        try { await markRead() } catch { /* Shared context displays the error; unread state is retained. */ }
        finally { setBusy(false) }
      }}
    >
      <CheckCheck className="h-5 w-5 shrink-0 md:h-4 md:w-4" aria-hidden="true" />
      <span className="hidden md:inline">{t(busy ? 'Saving…' : 'Mark all as read')}</span>
    </button>
  )
}

export function NotificationsActions() {
  const { t } = useLanguage(),
    navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null),
    trigger = useRef<HTMLButtonElement>(null)
  function close() {
    setOpen(false)
    trigger.current?.focus()
  }
  useEffect(() => {
    if (!open) return
    root.current
      ?.querySelector<HTMLButtonElement>('[role="menuitem"]:not(:disabled)')
      ?.focus()
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [open])
  return (
    <div
      ref={root}
      className="relative flex items-center gap-1"
      onKeyDown={(event) => {
        if (event.key === 'Escape') {
          event.preventDefault()
          event.stopPropagation()
          close()
        }
        if (event.key === 'Tab' && open) {
          setOpen(false)
          trigger.current?.focus()
        }
        if (
          open &&
          ['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)
        ) {
          event.preventDefault()
          const buttons = Array.from(
            root.current?.querySelectorAll<HTMLButtonElement>(
              '[role="menuitem"]:not(:disabled)',
            ) || [],
          )
          const current = buttons.indexOf(
            document.activeElement as HTMLButtonElement,
          )
          let index =
            event.key === 'Home'
              ? 0
              : event.key === 'End'
                ? buttons.length - 1
                : (current +
                    (event.key === 'ArrowDown' ? 1 : -1) +
                    buttons.length) %
                  buttons.length
          buttons[index]?.focus()
        }
      }}
    >
      <button
        type="button"
        ref={trigger}
        className="icon-action"
        aria-label={t('Notification actions')}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => {
          setOpen((value) => !value)
        }}
      >
        <MoreHorizontal className="h-5 w-5" />
      </button>
      {open && (
        <div
          role="menu"
          aria-label={t('Notification actions')}
          className="absolute right-0 top-full z-40 mt-2 w-64 max-w-[calc(100vw-2rem)] rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-lg"
        >
          <button
            type="button"
            role="menuitem"
            tabIndex={-1}
            className="flex min-h-11 w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-accent focus:bg-accent focus:outline-none"
            onClick={() => {
              close()
              navigate('/settings/notifications')
            }}
          >
            <Settings className="h-4 w-4 shrink-0" />
            {t('Notification settings')}
          </button>

        </div>
      )}
    </div>
  )
}
