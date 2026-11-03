import { useEffect, useState } from 'react'
import { accountsApi, type ProviderAccount } from '../../api/accounts'
import { agentsApi, type ModelInfo } from '../../api/agents'
import { useLanguage } from '../../context/LanguageContext'

// Keep draft selection in the parent: discovery failures must not rewrite it.
export function AccountModelPicker({ account, provider, value, onChange, disabled = false }: {
  account?: ProviderAccount
  provider?: string
  value: string
  onChange: (model: string) => void
  disabled?: boolean
}) {
  const { t } = useLanguage()
  const [models, setModels] = useState<ModelInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)
  const [attempt, setAttempt] = useState(0)
  const connected = account ? account.credentials_present : !!provider
  const accountID = account?.id
  useEffect(() => {
    let current = true
    setModels([]); setFailed(false)
    if (!connected) { setLoading(false); return }
    setLoading(true)
    const request = accountID ? accountsApi.models(accountID) : agentsApi.listModels(provider)
    void request.then(result => { if (current) setModels(result) })
      .catch(() => { if (current) setFailed(true) })
      .finally(() => { if (current) setLoading(false) })
    return () => { current = false }
  }, [accountID, provider, connected, attempt])
  const missing = value && !models.some(model => model.id === value)
  return <div className="space-y-2">
    <label htmlFor="brain-model" className="text-sm font-medium">{t('Preferred model')}</label>
    <select id="brain-model" className="field-control min-h-11" value={value}
      disabled={disabled || !connected || loading || failed}
      onChange={event => onChange(event.target.value)}>
      <option value="">{t('Auto')}</option>
      {missing && <option value={value}>{value} · {t('Saved selection')}</option>}
      {models.map(model => <option key={model.id} value={model.id}>{model.display_name || model.name || model.id}</option>)}
    </select>
    {!connected ? <p className="text-xs leading-5 text-muted-foreground">{t('Connect this account before choosing a model.')}</p>
      : loading ? <p role="status" className="text-xs leading-5 text-muted-foreground">{t('Loading models…')}</p>
      : failed ? <div className="text-xs leading-5 text-muted-foreground"><p role="status">{t('Could not load models. Your saved selection is preserved.')}</p><button type="button" className="min-h-11 underline" onClick={() => setAttempt(old => old + 1)}>{t('Try again')}</button></div>
      : <p className="text-xs leading-5 text-muted-foreground">{t('Auto lets the provider choose. Model availability depends on this connection.')}</p>}
  </div>
}
