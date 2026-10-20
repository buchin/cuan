import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Bell, CheckCheck, MoreHorizontal, Settings } from 'lucide-react'
import { useNotifications } from '../../context/NotificationsContext'
import { useLanguage } from '../../context/LanguageContext'
export function NotificationsView() {
  const { items, loading, error, refresh, markRead } = useNotifications(),
    { t, language } = useLanguage(),
    navigate = useNavigate()
  const location = useLocation()
  const filter =
    new URLSearchParams(location.search).get('filter') === 'unread'
      ? 'unread'
      : 'all'
  const [busy, setBusy] = useState(false),
    [actionError, setActionError] = useState('')
  const visible = items.filter((x) => filter === 'all' || !x.read)
  return (
    <section className="mx-auto max-w-3xl space-y-4">
      {(error || actionError) && (
        <div role="alert" className="text-sm text-destructive">
          {t(error || actionError)}{' '}
          <button className="underline" onClick={() => void refresh()}>
            {t('Retry')}
          </button>
        </div>
      )}
      {loading && !items.length ? (
        <p role="status" className="text-sm">
          {t('Loading…')}
        </p>
      ) : !visible.length ? (
        <div className="rounded-lg border border-border p-8 text-center">
          <Bell className="mx-auto mb-3 h-7 w-7 text-muted-foreground" />
          <p className="font-medium">{t('You’re all caught up')}</p>
          <p className="mt-2 text-sm text-muted-foreground">
            {t(
              'Approvals, blockers, completed work and new customer messages appear here.',
            )}
          </p>
        </div>
      ) : (
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border">
          {visible.map((item) => (
            <li key={item.id}>
              <button
                className="flex min-h-20 w-full gap-3 p-4 text-left hover:bg-accent"
                disabled={busy}
                onClick={async () => {
                  setBusy(true)
                  setActionError('')
                  try {
                    await markRead([item.id])
                    navigate(
                      /^\/(inbox|projects|tasks)(\/|$)/.test(item.href)
                        ? item.href
                        : '/studio',
                    )
                  } catch {
                    setActionError(
                      'Could not mark notifications as read. Try again.',
                    )
                  } finally {
                    setBusy(false)
                  }
                }}
              >
                <span
                  aria-label={item.read ? t('Read') : t('Unread')}
                  className={
                    'mt-2 h-2 w-2 shrink-0 rounded-full ' +
                    (item.read ? 'bg-transparent' : 'bg-primary')
                  }
                />
                <span className="min-w-0">
                  <span className="block text-xs text-muted-foreground">
                    {t(
                      {
                        approval: 'Needs approval',
                        blocker: 'Needs attention',
                        completed: 'Completed',
                        customer: 'Customer message',
                      }[item.kind],
                    )}
                  </span>
                  <span
                    className={
                      'mt-1 block break-words text-sm ' +
                      (!item.read ? 'font-semibold' : '')
                    }
                  >
                    {t(item.title)}
                  </span>
                  <time
                    className="mt-1 block text-xs text-muted-foreground"
                    dateTime={item.created_at}
                  >
                    {new Date(item.created_at).toLocaleString(language)}
                  </time>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export function NotificationsNavigation() {
  const { items } = useNotifications(),
    { t } = useLanguage(),
    location = useLocation()
  const unread = items.filter((item) => !item.read).length,
    filter =
      new URLSearchParams(location.search).get('filter') === 'unread'
        ? 'unread'
        : 'all'
  return (
    <nav
      aria-label={t('Notification filters')}
      className="flex shrink-0 gap-5 border-b border-border bg-background px-4 md:px-6"
    >
      {[
        { id: 'all', label: 'All', to: '/notifications' },
        { id: 'unread', label: 'Unread', to: '/notifications?filter=unread' },
      ].map((item) => (
        <Link
          key={item.id}
          to={item.to}
          aria-current={filter === item.id ? 'page' : undefined}
          className={
            'inline-flex min-h-11 items-center gap-2 border-b-2 px-1 text-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ' +
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
    </nav>
  )
}
export function NotificationsActions() {
  const { items, markRead } = useNotifications(),
    { t } = useLanguage(),
    navigate = useNavigate()
  const [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState('')
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
      className="relative"
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
          setError('')
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
            disabled={busy || !items.some((item) => !item.read)}
            className="flex min-h-11 w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-accent focus:bg-accent focus:outline-none disabled:opacity-50"
            onClick={async () => {
              if (busy) return
              setBusy(true)
              setError('')
              try {
                await markRead()
                close()
              } catch {
                setError('Could not mark notifications as read. Try again.')
              } finally {
                setBusy(false)
              }
            }}
          >
            <CheckCheck className="h-4 w-4 shrink-0" />
            {t('Mark all as read')}
          </button>
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
          {error && (
            <p role="alert" className="px-3 py-2 text-xs text-destructive">
              {t(error)}
            </p>
          )}
        </div>
      )}
    </div>
  )
}
