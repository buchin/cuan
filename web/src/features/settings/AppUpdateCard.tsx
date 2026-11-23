import { useState, useEffect } from 'react'
import { RefreshCw, CheckCircle2, AlertCircle, ArrowUpCircle, ExternalLink, Clock } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { GroupedSection } from '../../components/layout/GroupedSection'
import { Select } from '../../components/ui/forms'

interface UpdateInfo {
  current_version: string
  current_commit?: string
  latest_version: string
  has_update: boolean
  release_notes?: string
  release_url?: string
  last_checked_at?: string
  auto_update_enabled: boolean
  auto_update_time: string
  last_error?: string
}

export function AppUpdateCard() {
  const { t } = useLanguage()
  const [info, setInfo] = useState<UpdateInfo | null>(null)
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(false)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const fetchUpdateInfo = async () => {
    try {
      setLoading(true)
      const res = await fetch('/api/system/update')
      if (res.ok) {
        const data = await res.json()
        setInfo(data)
      }
    } catch {
      // keep fallback
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchUpdateInfo()
  }, [])

  const handleCheckUpdate = async () => {
    try {
      setChecking(true)
      const res = await fetch('/api/system/update/check', { method: 'POST' })
      if (res.ok) {
        const data = await res.json()
        setInfo(data.update || data)
      }
    } catch {
      // ignore
    } finally {
      setChecking(false)
    }
  }

  const handleToggleAutoUpdate = async (enabled: boolean) => {
    if (!info) return
    try {
      const res = await fetch('/api/system/update/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ auto_update_enabled: enabled }),
      })
      if (res.ok) {
        const updated = await res.json()
        setInfo(updated)
        setSaveSuccess(true)
        setTimeout(() => setSaveSuccess(false), 2000)
      }
    } catch {
      // ignore
    }
  }

  const handleTimeChange = async (timeVal: string) => {
    if (!info) return
    try {
      const res = await fetch('/api/system/update/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ auto_update_time: timeVal }),
      })
      if (res.ok) {
        const updated = await res.json()
        setInfo(updated)
        setSaveSuccess(true)
        setTimeout(() => setSaveSuccess(false), 2000)
      }
    } catch {
      // ignore
    }
  }

  const formattedTime = info?.last_checked_at
    ? new Date(info.last_checked_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : null

  return (
    <GroupedSection
      id="system-update"
      aria-labelledby="system-update-title"
      title={t('App Version & Updates')}
      description={t('Automated non-developer updates and nightly maintenance schedules.')}
    >
      <div className="p-4 sm:p-5 space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <span id="system-update-title" className="text-sm font-semibold text-foreground">
              {t('Version')}
            </span>
            <span className="inline-flex items-center rounded-md bg-secondary/80 px-2 py-0.5 text-xs font-mono text-secondary-foreground border border-border/40">
              v{info?.current_version || '0.1.0'}
              {info?.current_commit ? ` (${info.current_commit.substring(0, 7)})` : ''}
            </span>
          </div>

          <button
            type="button"
            onClick={handleCheckUpdate}
            disabled={checking || loading}
            className="action-secondary inline-flex items-center gap-2 shrink-0"
          >
            <RefreshCw className={`h-4 w-4 ${checking ? 'animate-spin' : ''}`} />
            {checking ? t('Checking...') : t('Check for updates')}
          </button>
        </div>

        {info?.has_update ? (
          <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-foreground">
            <div className="flex items-start gap-3">
              <ArrowUpCircle className="h-5 w-5 text-amber-500 shrink-0 mt-0.5" />
              <div className="space-y-1">
                <div className="font-semibold text-amber-500">
                  {t('A new version is available: v')}{info.latest_version}
                </div>
                <p className="text-xs text-muted-foreground">
                  {info.release_notes || t('Includes performance improvements and bug fixes.')}
                </p>
                {info.release_url && (
                  <a
                    href={info.release_url}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline mt-1"
                  >
                    {t('View release notes')}
                    <ExternalLink className="h-3 w-3" />
                  </a>
                )}
              </div>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/20 border border-border/30 rounded-xl p-3">
            <CheckCircle2 className="h-4 w-4 text-emerald-500 shrink-0" />
            <span>
              {t('Your Cuan application is up to date.')}
              {formattedTime && ` (${t('Last checked:')} ${formattedTime})`}
            </span>
          </div>
        )}

        {info?.last_error && (
          <div className="flex items-center gap-2 text-xs text-amber-500">
            <AlertCircle className="h-3.5 w-3.5 shrink-0" />
            <span>{info.last_error}</span>
          </div>
        )}

        {/* Scheduler & Nightly Auto-Update Preference */}
        <div className="border-t border-border/40 pt-4 mt-2 space-y-3">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <label htmlFor="toggle-auto-update" className="text-sm font-medium cursor-pointer">
                {t('Nightly auto-update (Scheduled)')}
              </label>
              <p className="text-xs text-muted-foreground">
                {t('Automatically check and apply seamless updates during low-activity hours.')}
              </p>
            </div>
            <input
              id="toggle-auto-update"
              type="checkbox"
              checked={info?.auto_update_enabled ?? true}
              onChange={(e) => handleToggleAutoUpdate(e.target.checked)}
              className="h-4 w-4 rounded border-border text-primary focus:ring-primary cursor-pointer"
            />
          </div>

          <div className="flex flex-col items-start gap-2 pt-1 sm:flex-row sm:items-center">
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <Clock className="h-3.5 w-3.5" />
              <span>{t('Execution window:')}</span>
            </div>
            <Select
              value={info?.auto_update_time || '03:00'}
              onChange={(e) => handleTimeChange(e.target.value)}
              disabled={!(info?.auto_update_enabled ?? true)}
              className="max-w-full sm:w-auto"
            >
              <option value="01:00">01:00 WIB (Midnight)</option>
              <option value="02:00">02:00 WIB</option>
              <option value="03:00">03:00 WIB (Recommended)</option>
              <option value="04:00">04:00 WIB</option>
              <option value="05:00">05:00 WIB (Early morning)</option>
            </Select>
            {saveSuccess && (
              <span className="text-xs text-emerald-500 animate-fade-in">
                {t('Saved')}
              </span>
            )}
          </div>
        </div>
      </div>
    </GroupedSection>
  )
}
