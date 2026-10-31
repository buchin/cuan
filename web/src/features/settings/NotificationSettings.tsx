import { useEffect, useState } from 'react'
import { apiFetch } from '../../api/client'
import { useLanguage } from '../../context/LanguageContext'
import { usePWA } from '../../context/PWAContext'
import { useAuth } from '../../context/AuthContext'
import { Bell, AlertCircle } from 'lucide-react'
import { ResendSettingsCard } from './ResendSettingsCard'

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
  const { t } = useLanguage()
  const { installed } = usePWA()
  const { role } = useAuth()
  const isMember = role === 'member'

  const [prefs, setPrefs] = useState<Preferences | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [connected, setConnected] = useState(false)
  const [savedPrefs, setSavedPrefs] = useState('')
  const [permission, setPermission] = useState<NotificationPermission>(() => {
    if (typeof window !== 'undefined' && 'Notification' in window) {
      return Notification.permission
    }
    return 'default'
  })

  const dirty = !!prefs && JSON.stringify(prefs) !== savedPrefs
  useEffect(() => {
    if (dirty) setNotice('')
  }, [dirty, prefs])

  const supported =
    typeof window !== 'undefined' &&
    'Notification' in window &&
    'PushManager' in window &&
    'serviceWorker' in navigator

  const ios =
    typeof navigator !== 'undefined' &&
    (/iPad|iPhone|iPod/.test(navigator.userAgent) ||
      (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1))

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
        if ('Notification' in window) {
          setPermission(Notification.permission)
        }
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
        const perm = await Notification.requestPermission()
        setPermission(perm)
        if (perm !== 'granted') {
          throw new Error(
            'Notifications were not enabled. You can change this in your browser settings.',
          )
        }
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
        if (!sub) {
          sub = await registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: bytes,
          })
        }
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
      if (typeof window !== 'undefined' && 'Notification' in window) {
        setPermission(Notification.permission)
      }
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
        <p role="status" className="text-sm text-emerald-600 dark:text-emerald-400">
          {t(notice)}
        </p>
      )}

      {/* Main Push Settings Card */}
      <section className="space-y-4 rounded-lg border border-border bg-card p-5">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Bell className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-foreground">
                {t('Push notifications')}
              </h2>
              <p className="text-xs text-muted-foreground">
                {t('Instant browser alerts & 1-click deep linking')}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span
              className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium border ${
                connected
                  ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                  : 'border-border bg-muted/50 text-muted-foreground'
              }`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  connected ? 'bg-emerald-500' : 'bg-muted-foreground'
                }`}
              />
              {t(connected ? 'Active on this device' : 'Inactive on this device')}
            </span>
            <span className="inline-flex items-center rounded-full border border-primary/20 bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
              {t(isMember ? 'Staff Member' : 'Owner')}
            </span>
          </div>
        </div>

        <p className="text-sm text-muted-foreground">
          {t(
            'Get real-time browser notifications when customers send messages or AI assistants complete tasks. Click any notification to open the conversation directly in Inbox.',
          )}
        </p>

        {/* Browser Permission Guidance Banner if blocked */}
        {supported && permission === 'denied' && (
          <div
            role="alert"
            className="flex items-start gap-3 rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive"
          >
            <AlertCircle className="h-5 w-5 shrink-0 mt-0.5" />
            <div className="space-y-1">
              <p className="font-medium">{t('Notification permission is blocked')}</p>
              <p className="text-xs text-destructive/90 leading-relaxed">
                {t(
                  'Notification permission is blocked in your browser. Open browser site settings to allow notifications to receive customer messages and agent updates.',
                )}
              </p>
            </div>
          </div>
        )}

        {/* Action Button */}
        <div className="pt-1">
          {ios && !installed ? (
            <p className="text-sm text-amber-600 dark:text-amber-400">
              {t(
                'Install Cuan on your home screen, open the installed app, then enable notifications here.',
              )}
            </p>
          ) : supported ? (
            <button
              type="button"
              className={`min-h-11 inline-flex items-center justify-center px-4 text-sm font-medium rounded-md transition-colors ${
                connected
                  ? 'action-secondary'
                  : 'action-primary'
              }`}
              disabled={busy}
              onClick={() => void togglePush()}
            >
              {busy
                ? t('Updating push…')
                : t(connected ? 'Disable on this device' : 'Enable on this device')}
            </button>
          ) : (
            <p className="text-sm text-muted-foreground">
              {t(
                'This browser does not support push notifications. Updates remain available inside Cuan.',
              )}
            </p>
          )}
        </div>
      </section>

      {/* Notification Preferences & Quiet Hours Form */}
      {prefs ? (
        <form
          className="space-y-5 rounded-lg border border-border bg-card p-5"
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
          <div className="border-b border-border/50 pb-3">
            <h2 className="text-lg font-semibold text-foreground">{t('Push updates')}</h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t(
                'Choose which updates reach your devices. All updates remain in the notification center.',
              )}
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 pt-1">
            {(['customer', 'approval', 'blocker', 'completed'] as const).map(
              (kind) => (
                <label
                  key={kind}
                  className="flex min-h-11 items-center gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-sm text-foreground hover:bg-muted/40 transition-colors cursor-pointer select-none"
                >
                  <input
                    type="checkbox"
                    className="h-4 w-4 rounded border-border text-primary focus:ring-primary"
                    checked={prefs[kind]}
                    disabled={busy}
                    onChange={(e) =>
                      setPrefs({ ...prefs, [kind]: e.target.checked })
                    }
                  />
                  <span className="font-medium">
                    {t(
                      {
                        customer: 'Customer messages',
                        approval: 'Needs approval',
                        blocker: 'Needs attention',
                        completed: 'Completed',
                      }[kind],
                    )}
                  </span>
                </label>
              ),
            )}
          </div>

          {/* Personal Quiet Hours Section */}
          <div className="border-t border-border/50 pt-4 space-y-4">
            <div className="flex items-start justify-between gap-3">
              <label className="flex min-h-11 items-center gap-3 text-sm text-foreground cursor-pointer select-none">
                <input
                  type="checkbox"
                  className="h-4 w-4 rounded border-border text-primary focus:ring-primary"
                  checked={prefs.quiet}
                  disabled={busy}
                  onChange={(e) => setPrefs({ ...prefs, quiet: e.target.checked })}
                />
                <div>
                  <span className="font-semibold">{t('Quiet hours')}</span>
                  <p className="text-xs text-muted-foreground">
                    {t(
                      'Push notifications and sounds will be paused automatically during this timeframe according to your device timezone.',
                    )}
                  </p>
                </div>
              </label>
            </div>

            {prefs.quiet && (
              <div className="space-y-3 rounded-lg border border-border/60 bg-muted/20 p-4">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {(['start_hour', 'end_hour'] as const).map((key) => (
                    <label key={key} className="space-y-1.5 text-sm font-medium">
                      <span>{t(key === 'start_hour' ? 'From' : 'Until')}</span>
                      <select
                        className="field-control w-full min-h-11"
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
                <label className="block space-y-1.5 text-sm font-medium">
                  <span>{t('Timezone')}</span>
                  <input
                    className="field-control w-full min-h-11"
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
          </div>

          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 border-t border-border/50 pt-4">
            <p role="status" className="text-xs text-muted-foreground">
              {t(busy ? 'Saving…' : dirty ? 'Unsaved changes' : 'All changes saved')}
            </p>
            <button
              type="submit"
              className="action-primary min-h-11 inline-flex items-center justify-center px-4"
              disabled={busy || !dirty}
            >
              {t(busy ? 'Saving…' : 'Save preferences')}
            </button>
          </div>
        </form>
      ) : (
        !error && <p role="status" className="text-sm text-muted-foreground">{t('Loading…')}</p>
      )}

      {/* Resend Email Integration & Chat Idle Notification */}
      <div className="pt-2">
        <ResendSettingsCard />
      </div>
    </div>
  )
}
