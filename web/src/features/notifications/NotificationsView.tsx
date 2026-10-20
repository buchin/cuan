import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Bell, CheckCheck } from 'lucide-react'
import { useNotifications } from '../../context/NotificationsContext'
import { useLanguage } from '../../context/LanguageContext'
export function NotificationsView() {
  const { items, loading, error, refresh, markRead } = useNotifications(),
    { t, language } = useLanguage(),
    navigate = useNavigate()
  const [filter, setFilter] = useState<'all' | 'unread'>('all'),
    [busy, setBusy] = useState(false),
    [actionError, setActionError] = useState('')
  const visible = items.filter((x) => filter === 'all' || !x.read)
  return (
    <section className="mx-auto max-w-3xl space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          {t('Updates that need your attention.')}
        </p>
        <Link className="action-secondary" to="/settings/notifications">
          {t('Notification settings')}
        </Link>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <button
          className="action-secondary"
          aria-pressed={filter === 'all'}
          onClick={() => setFilter('all')}
        >
          {t('All')}
        </button>
        <button
          className="action-secondary"
          aria-pressed={filter === 'unread'}
          onClick={() => setFilter('unread')}
        >
          {t('Unread')}
        </button>
        <button
          className="action-secondary ml-auto"
          disabled={busy || !items.some((x) => !x.read)}
          onClick={async () => {
            setBusy(true)
            setActionError('')
            try {
              await markRead()
            } catch {
              setActionError('Could not mark notifications as read. Try again.')
            } finally {
              setBusy(false)
            }
          }}
        >
          <CheckCheck className="h-4 w-4" />
          {t('Mark all as read')}
        </button>
      </div>
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
