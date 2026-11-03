import { useEffect, useState } from 'react'
import { accountsApi, type AccountUsage, type ProviderAccount } from '../../api/accounts'
import { useLanguage } from '../../context/LanguageContext'

export function AccountUsageDetails({ account }: { account: ProviderAccount }) {
  const { t } = useLanguage()
  const [usage, setUsage] = useState<AccountUsage | null>(null)
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    let current = true
    setUsage(null); setFailed(false)
    if (!account.credentials_present || account.provider !== 'agy') return
    setLoading(true)
    // Raw provider quota can include retired models. Show only installed models.
    void Promise.all([accountsApi.usage(account.id), accountsApi.models(account.id)])
      .then(([result, models]) => {
        if (current) setUsage({ ...result, models: result.models.filter(row => models.some(model => model.id === row.model)) })
      }).catch(() => { if (current) setFailed(true) })
      .finally(() => { if (current) setLoading(false) })
    return () => { current = false }
  }, [account.id, account.credentials_present, account.provider, attempt])
  if (!account.credentials_present || account.provider !== 'agy') return null
  const rows = usage?.models || []
  const unavailable = failed || !usage?.available || !rows.length
  return <section aria-label={t('Usage')} className="space-y-3">
    <div className="flex items-center justify-between gap-3"><h3 className="text-sm font-medium">{t('Usage')}</h3><button type="button" className="min-h-11 text-xs text-muted-foreground underline" disabled={loading} onClick={() => setAttempt(old => old + 1)}>{t(loading ? 'Checking…' : 'Refresh usage')}</button></div>
    {loading ? <p role="status" className="text-xs text-muted-foreground">{t('Checking usage…')}</p>
      : unavailable ? <p role="status" className="text-xs leading-5 text-muted-foreground">{t('Usage is unavailable. Remaining quota is unknown.')}</p>
      : <div className="space-y-4">{rows.map(row => {
        const known = Number.isFinite(row.remaining_percent) && row.remaining_percent >= 0
        const remaining = Math.min(100, Math.max(0, row.remaining_percent))
        const reset = row.reset_at ? new Date(row.reset_at) : null
        return <div key={row.model} className="space-y-1.5">
          <div className="flex items-baseline justify-between gap-3 text-xs"><span className="min-w-0 break-words text-muted-foreground">{row.model}</span><span className="shrink-0 tabular-nums">{known ? `${Math.round(remaining)}% ${t('remaining')}` : t('Unknown')}</span></div>
          {known && <div role="meter" aria-label={row.model + ' ' + t('remaining')} aria-valuemin={0} aria-valuemax={100} aria-valuenow={remaining} className="h-1 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-muted-foreground/50" style={{ width: `${remaining}%` }} /></div>}
          {reset && Number.isFinite(reset.getTime()) && <p className="text-xs leading-5 text-muted-foreground">{t('Resets')}: {reset.toLocaleString()}</p>}
        </div>
      })}</div>}
    {usage?.checked_at && Number.isFinite(new Date(usage.checked_at).getTime()) && <p className="text-xs text-muted-foreground">{t('Last checked')}: {new Date(usage.checked_at).toLocaleString()}</p>}
    <p className="text-xs leading-5 text-muted-foreground">{t('Provider-reported quota. Available quota does not guarantee a successful connection.')}</p>
  </section>
}
