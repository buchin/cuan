import { useEffect, useState } from 'react'
import { apiFetch } from '../../api/client'
import { useLanguage } from '../../context/LanguageContext'
import { usePWA } from '../../context/PWAContext'
interface Preferences {
  approval: boolean
  blocker: boolean
  completed: boolean
  customer: boolean
  quiet: boolean
  start_hour: number
  end_hour: number
  timezone: string
}
export function NotificationSettings() {
  const { t } = useLanguage(),
    { installed } = usePWA(),
    [prefs, setPrefs] = useState<Preferences | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [notice, setNotice] = useState(''),
    [connected, setConnected] = useState(false),
    [savedPrefs, setSavedPrefs] = useState('')
  const dirty = !!prefs && JSON.stringify(prefs) !== savedPrefs
  useEffect(() => { if (dirty) setNotice('') }, [dirty, prefs])
  const supported =
    'Notification' in window &&
    'PushManager' in window &&
    'serviceWorker' in navigator
  const ios =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  async function load() {
    try {
      const data = await apiFetch<{
        preferences: Preferences
        devices: number
      }>('/api/v1/notifications/preferences')
      const initial = {
        ...data.preferences,
        timezone:
          data.preferences.timezone === 'UTC' && !data.preferences.quiet
            ? Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
            : data.preferences.timezone,
      }
      setPrefs(initial)
      setSavedPrefs(JSON.stringify(initial))
      if (supported) {
        const registration = await navigator.serviceWorker.getRegistration()
        const sub = await registration?.pushManager.getSubscription()
        setConnected(!!sub && data.devices > 0)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load preferences.')
    }
  }
  useEffect(() => {
    void load()
  }, [])
  async function togglePush() {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      if (!connected) {
        const permission = await Notification.requestPermission()
        if (permission !== 'granted')
          throw new Error(
            'Notifications were not enabled. You can change this in your browser settings.',
          )
      }
      const registration = await Promise.race([
        navigator.serviceWorker.ready,
        new Promise<never>((_, reject) =>
          setTimeout(
            () =>
              reject(
                new Error('App service is not ready. Reload and try again.'),
              ),
            8000,
          ),
        ),
      ])
      let sub = await registration.pushManager.getSubscription()
      if (connected) {
        if (sub) {
          try {
            await apiFetch('/api/v1/notifications/push', {
              method: 'DELETE',
              body: JSON.stringify(sub.toJSON()),
            })
          } finally {
            await sub.unsubscribe()
          }
        }
        setConnected(false)
        setNotice('Push notifications disabled on this device.')
      } else {
        const key = await apiFetch<{ public_key: string }>(
          '/api/v1/notifications/push',
        )
        const raw = atob(
          key.public_key.replace(/-/g, '+').replace(/_/g, '/') +
            '='.repeat((4 - (key.public_key.length % 4)) % 4),
        )
        const bytes = Uint8Array.from(raw, (c) => c.charCodeAt(0))
        if (!sub)
          sub = await registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: bytes,
          })
        await apiFetch('/api/v1/notifications/push', {
          method: 'POST',
          body: JSON.stringify(sub.toJSON()),
        })
        setConnected(true)
        setNotice('Push notifications enabled on this device.')
      }
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : 'Could not update push notifications. Try again.',
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="max-w-2xl space-y-5">
      {error && (
        <div role="alert" className="text-sm text-destructive">
          {t(error)}{' '}
          {!prefs && (
            <button className="underline" onClick={() => void load()}>
              {t('Retry')}
            </button>
          )}
        </div>
      )}
      {notice && (
        <p role="status" className="text-sm">
          {t(notice)}
        </p>
      )}
      <section className="space-y-3 rounded-lg border border-border bg-card p-5">
        <h2 className="text-lg font-semibold">{t('Push notifications')}</h2>
        <p className="text-sm text-muted-foreground">
          {t(
            'Get important updates while Cuan is closed. Message contents and project names stay off your lock screen.',
          )}
        </p>
        {ios && !installed ? (
          <p className="text-sm">
            {t(
              'Install Cuan on your home screen, open the installed app, then enable notifications here.',
            )}
          </p>
        ) : supported ? (
          <button
            type="button"
            className="action-primary"
            disabled={busy}
            onClick={() => void togglePush()}
          >
            {t(connected ? 'Disable on this device' : 'Enable on this device')}
          </button>
        ) : (
          <p className="text-sm">
            {t(
              'This browser does not support push notifications. Updates remain available inside Cuan.',
            )}
          </p>
        )}
      </section>
      {prefs ? (
        <form
          className="space-y-4 rounded-lg border border-border bg-card p-5"
          onSubmit={async (e) => {
            e.preventDefault()
            if (!dirty || busy) return
            setBusy(true)
            setError('')
            setNotice('')
            try {
              await apiFetch('/api/v1/notifications/preferences', {
                method: 'PATCH',
                body: JSON.stringify(prefs),
              })
              setSavedPrefs(JSON.stringify(prefs))
              setNotice('Notification preferences saved.')
            } catch (e) {
              setError(
                e instanceof Error
                  ? e.message
                  : 'Could not save notification preferences.',
              )
            } finally {
              setBusy(false)
            }
          }}
        >
          <h2 className="text-lg font-semibold">{t('Push updates')}</h2>
          <p className="text-sm text-muted-foreground">
            {t(
              'Choose which updates reach your devices. All updates remain in the notification center.',
            )}
          </p>
          {(['approval', 'blocker', 'completed', 'customer'] as const).map(
            (kind) => (
              <label
                key={kind}
                className="flex min-h-11 items-center gap-3 text-sm"
              >
                <input
                  type="checkbox"
                  checked={prefs[kind]}
                  disabled={busy}
                  onChange={(e) =>
                    setPrefs({ ...prefs, [kind]: e.target.checked })
                  }
                />
                {t(
                  {
                    approval: 'Needs approval',
                    blocker: 'Needs attention',
                    completed: 'Completed',
                    customer: 'Customer messages',
                  }[kind],
                )}
              </label>
            ),
          )}
          <label className="flex min-h-11 items-center gap-3 text-sm">
            <input
              type="checkbox"
              checked={prefs.quiet}
              disabled={busy}
              onChange={(e) => setPrefs({ ...prefs, quiet: e.target.checked })}
            />
            {t('Quiet hours')}
          </label>
          {prefs.quiet && (
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                {(['start_hour', 'end_hour'] as const).map((key) => (
                  <label key={key} className="space-y-1 text-sm">
                    <span>{t(key === 'start_hour' ? 'From' : 'Until')}</span>
                    <select
                      className="field-control"
                      value={prefs[key]}
                      disabled={busy}
                      onChange={(e) =>
                        setPrefs({ ...prefs, [key]: Number(e.target.value) })
                      }
                    >
                      {Array.from({ length: 24 }, (_, i) => (
                        <option key={i} value={i}>
                          {String(i).padStart(2, '0')}:00
                        </option>
                      ))}
                    </select>
                  </label>
                ))}
              </div>
              <label className="block space-y-1 text-sm">
                <span>{t('Timezone')}</span>
                <input
                  className="field-control"
                  value={prefs.timezone}
                  disabled={busy}
                  required
                  placeholder="Asia/Jakarta"
                  onChange={(e) =>
                    setPrefs({ ...prefs, timezone: e.target.value })
                  }
                />
              </label>
            </div>
          )}
          <p role="status" className="text-xs text-muted-foreground">{t(busy ? 'Saving…' : dirty ? 'Unsaved changes' : 'All changes saved')}</p>
          <button className="action-primary" disabled={busy || !dirty}>
            {t(busy ? 'Saving…' : 'Save preferences')}
          </button>
        </form>
      ) : (
        !error && <p role="status">{t('Loading…')}</p>
      )}
    </div>
  )
}
