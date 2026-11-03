import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { ListRowContent } from '../../components/layout/ListControls'
import { ItemActions } from '../../components/ui/ItemActions'
import { useCallback, useEffect, useState } from 'react'
import { Bot, Plus, RefreshCw } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useAuth } from '../../context/AuthContext'
import { aiProvidersApi, type AIProviderDTO } from '../../api/aiProviders'
import { accountsApi, type ProviderAccount } from '../../api/accounts'
import { agentsApi, type Agent } from '../../api/agents'
import { AddBrainModal } from './AddBrainModal'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import { GroupedSection } from '../../components/layout/GroupedSection'

const ASSISTANT_NAME_KEY = 'cuan.assistant_name'

export function getStoredAssistantName(): string {
  try {
    return localStorage.getItem(ASSISTANT_NAME_KEY) || 'Cuan'
  } catch {
    return 'Cuan'
  }
}

export function setStoredAssistantName(name: string): void {
  try {
    const clean = name.trim() || 'Cuan'
    localStorage.setItem(ASSISTANT_NAME_KEY, clean)
    window.dispatchEvent(
      new CustomEvent('cuan:assistant-name-changed', { detail: { name: clean } })
    )
  } catch {
    // Ignore storage restrictions
  }
}


type Brain = { id: string; label: string; provider: string; state: string; account?: ProviderAccount; byok?: AIProviderDTO }

export function AgentSettingsView({ showHeading = true }: { showHeading?: boolean }) {
  const { t } = useLanguage()
  const { isOwner } = useAuth()
  const [primary, setPrimary] = useState<Agent | null>(null)
  const [name, setName] = useState(getStoredAssistantName)
  const [baselineName, setBaselineName] = useState('')
  const [accounts, setAccounts] = useState<ProviderAccount[]>([])
  const [providers, setProviders] = useState<AIProviderDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [addOpen, setAddOpen] = useState(false)
  const [selected, setSelected] = useState<Brain | null>(null)
  const [label, setLabel] = useState('')
  const [model, setModel] = useState('')
  const [drawerError, setDrawerError] = useState('')
  const [drawerNotice, setDrawerNotice] = useState('')
  const [checking, setChecking] = useState(false)
  const [disconnectOpen, setDisconnectOpen] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [accountList, providerList, agentList] = await Promise.all([accountsApi.list(), aiProvidersApi.list(), agentsApi.list()])
      setAccounts(accountList || [])
      setProviders(providerList || [])
      const assistant = agentList.find(a => a.id === 'agent_cuan') || null
      setPrimary(assistant)
      if (assistant) {
        const saved = assistant.name.replace(/^@/, '')
        setName(saved); setBaselineName(saved); setStoredAssistantName(saved)
      }
    } catch { setError(t('Could not load assistant settings. Try again.')) }
    finally { setLoading(false) }
  }, [t])
  useEffect(() => { void load() }, [load])
  const brains: Brain[] = [
    ...accounts.filter(account => !account.legacy).map(account => ({ id: account.id, label: account.label, provider: account.provider === 'agy' ? 'Google · Antigravity' : account.provider, state: account.state, account })),
    ...providers.filter(provider => provider.has_api_key).map(byok => ({ id: 'byok_' + byok.id, label: byok.name, provider: byok.id, state: 'configured', byok })),
  ]
  const verified = brains.filter(brain => brain.state === 'verified').length
  const cooldown = brains.filter(brain => brain.state === 'quota_limited').length
  const statusText = (state: string) => t(({ verified: 'Connected', quota_limited: 'Waiting for capacity', login_required: 'Sign-in required', unavailable: 'Connection needs attention', unknown: 'Not checked', configured: 'Configured' } as Record<string, string>)[state] || 'Not checked')
  const openBrain = (brain: Brain) => {
    setSelected(brain); setLabel(brain.label)
    setModel(brain.account?.model_id || brain.byok?.default_model || '')
    setDrawerError(''); setDrawerNotice('')
  }
  const saveName = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!primary || busy || !name.trim() || !isOwner) return
    setBusy(true); setError(''); setNotice('')
    try {
      const clean = name.trim().replace(/^@/, '')
      const saved = await agentsApi.patch(primary.id, { name: '@' + clean })
      setPrimary(saved); setName(clean); setBaselineName(clean); setStoredAssistantName(clean)
      setNotice(t('Changes saved.'))
    } catch { setError(t('Could not save the assistant name. Your draft is preserved.')) }
    finally { setBusy(false) }
  }
  const saveBrain = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!selected || busy || !isOwner) return
    setBusy(true); setDrawerError(''); setDrawerNotice('')
    try {
      if (selected.account) {
        const saved = await accountsApi.update(selected.account.id, { label, model_id: model })
        setAccounts(old => old.map(account => account.id === saved.id ? saved : account))
        setSelected({ ...selected, label: saved.label, account: saved })
        setLabel(saved.label); setModel(saved.model_id || '')
      } else if (selected.byok) {
        await aiProvidersApi.save({ provider: selected.byok.id, id: selected.byok.id, default_model: model })
        setProviders(old => old.map(provider => provider.id === selected.byok?.id ? { ...provider, default_model: model } : provider))
        setSelected({ ...selected, byok: { ...selected.byok, default_model: model } })
      }
      setDrawerNotice(t('Changes saved.'))
    } catch { setDrawerError(t('Could not save brain settings. Your draft is preserved.')) }
    finally { setBusy(false) }
  }
  const checkBrain = async () => {
    if (!selected || checking) return
    setChecking(true); setDrawerError(''); setDrawerNotice('')
    try {
      if (selected.account) {
        const saved = await accountsApi.verify(selected.account.id)
        setAccounts(old => old.map(account => account.id === saved.id ? saved : account))
        setSelected({ ...selected, state: saved.state, account: saved })
        setDrawerNotice(statusText(saved.state))
      } else if (selected.byok) {
        const result = await aiProvidersApi.test({ provider: selected.byok.id, id: selected.byok.id, model: selected.byok.default_model })
        setDrawerNotice(result.valid ? t('Connection verified.') : t('Connection needs attention'))
      }
    } catch { setDrawerError(t('Could not check this connection. Try again.')) }
    finally { setChecking(false) }
  }
  const disconnect = async () => {
    if (!selected || !isOwner || busy) return
    setBusy(true); setDrawerError('')
    try {
      if (selected.account) {
        await accountsApi.delete(selected.account.id)
        setAccounts(old => old.filter(account => account.id !== selected.account?.id))
      } else if (selected.byok) {
        await aiProvidersApi.save({ provider: selected.byok.id, id: selected.byok.id, clear_api_key: true, enabled: false })
        setProviders(old => old.filter(provider => provider.id !== selected.byok?.id))
      }
      setDisconnectOpen(false); setSelected(null); setNotice(t('Connection removed. Your saved work is preserved.'))
    } catch (cause) { setDrawerError(t(cause instanceof Error ? cause.message : 'Could not disconnect this brain. Try again.')) }
    finally { setBusy(false) }
  }
  const dirty = selected ? label !== selected.label || model !== (selected.account?.model_id || selected.byok?.default_model || '') : false
  const closeDrawer = () => {
    if (busy || checking) return
    if (dirty && !confirm(t('Discard unsaved changes?'))) return
    setSelected(null)
  }
  return <div className="mx-auto w-full max-w-2xl space-y-8 pb-8">
    {showHeading && <div className="space-y-2"><h2 className="text-lg font-semibold">{t('Assistant & AI brains')}</h2><p className="text-sm leading-6 text-muted-foreground">{t('One assistant for your conversations and work. AI capacity is managed automatically.')}</p></div>}
    {error && <p role="alert" className="text-sm text-destructive">{error} <button className="min-h-11 underline" onClick={() => void load()}>{t('Try again')}</button></p>}
    {notice && <p role="status" className="text-sm text-muted-foreground">{notice}</p>}
    <section className="space-y-4" aria-label={t('Assistant identity')}>
      <div className="flex items-center gap-3"><Bot className="h-5 w-5 text-muted-foreground" /><div><h3 className="text-base font-semibold">{primary?.name || name}</h3><p className="mt-1 text-sm text-muted-foreground">{loading ? t('Loading…') : `${verified} ${t('connected')} · ${cooldown} ${t('waiting for capacity')}`}</p></div></div>
      <form onSubmit={saveName} className="space-y-2">
        <label htmlFor="assistant-name-input" className="block text-sm font-medium">{t('Assistant name')}</label>
        <div className="flex flex-col gap-2 sm:flex-row"><input id="assistant-name-input" className="field-control min-w-0 flex-1" maxLength={100} required value={name} disabled={busy || loading || !isOwner} onChange={event => setName(event.target.value)} /><button type="submit" className="action-secondary" disabled={busy || loading || !primary || !isOwner || !name.trim() || name === baselineName}>{t(busy ? 'Saving…' : 'Save changes')}</button></div>
        <p className="text-xs leading-5 text-muted-foreground">{t('Used in new replies and conversations across your workspace.')}</p>
      </form>
    </section>
    <section className="space-y-3" aria-label={t('AI brains')}>
      <div className="flex items-center justify-between gap-3"><h3 className="text-base font-semibold">{t('AI brains')}</h3>{isOwner && <button className="action-secondary inline-flex items-center gap-1.5" onClick={() => setAddOpen(true)}><Plus className="h-4 w-4" />{t('Add brain')}</button>}</div>
      <p className="text-sm leading-6 text-muted-foreground">{t('Cuan uses available capacity automatically. Open a connection to edit its name, model, or connection details.')}</p>
      {loading ? <p role="status" className="py-5 text-sm text-muted-foreground">{t('Loading…')}</p> : brains.length === 0 ? <p className="py-5 text-sm text-muted-foreground">{t('Connect an AI brain to get started.')}</p> : <GroupedSection inset={false} containerClassName="!bg-transparent">
        {brains.map(brain => <ItemActions key={brain.id} label={brain.label} actions={[{ label: 'Edit brain', run: () => openBrain(brain) }, ...(isOwner ? [{ label: 'Disconnect brain', run: () => { openBrain(brain); setDisconnectOpen(true) } }] : [])]}>
          <button className="flex min-h-[76px] w-full items-center gap-3 py-3 text-left hover:bg-muted/20 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring" onClick={() => openBrain(brain)} aria-label={t('Edit brain') + ': ' + brain.label}>
            <ListRowContent leading={<span className="flex h-9 w-9 items-center justify-center bg-muted/50 text-xs text-muted-foreground">{brain.provider.slice(0,1).toUpperCase()}</span>} title={brain.label}
              description={<><span className="block whitespace-normal break-all">{brain.account ? (brain.account.email || t('Email identity unavailable')) : brain.provider}</span><span className="mt-1 block whitespace-normal">{brain.provider} · {statusText(brain.state)}</span></>} />
          </button>
        </ItemActions>)}

      </GroupedSection>}
    </section>
    <SlideDrawer isOpen={!!selected} onClose={closeDrawer} title={t('Brain settings')} description={selected?.label} size="md">
      {selected && <div className="space-y-6">
        <form onSubmit={saveBrain} className="space-y-5">
          {selected.account ? (<div className="space-y-2"><label htmlFor="brain-label" className="text-sm font-medium">{t('Display name')}</label><input id="brain-label" className="field-control" maxLength={100} required value={label} disabled={busy || !isOwner || !selected.account || selected.account.legacy} onChange={event => setLabel(event.target.value)} /></div>) : <p className="text-sm font-medium">{selected.label}</p>}
          <div className="space-y-2"><label htmlFor="brain-model" className="text-sm font-medium">{t('Preferred model')}</label><input id="brain-model" className="field-control" maxLength={200} value={model} placeholder="Auto" disabled={busy || !isOwner || selected.account?.legacy} onChange={event => setModel(event.target.value)} /><p className="text-xs leading-5 text-muted-foreground">{t('Leave empty for Auto. Model availability depends on this connection.')}</p></div>
          {drawerError && <p role="alert" className="text-sm text-destructive">{drawerError}</p>}{drawerNotice && <p role="status" className="text-sm text-muted-foreground">{drawerNotice}</p>}
          {isOwner && !selected.account?.legacy && <button className="action-primary" disabled={busy || !dirty || !label.trim()}>{t(busy ? 'Saving…' : 'Save changes')}</button>}
        </form>
        <div className="space-y-3 border-t border-border/40 pt-5"><h3 className="text-sm font-medium">{t('Connection')}</h3><p className="text-sm text-muted-foreground">{statusText(selected.state)}</p>{selected.account && <div className="space-y-1 text-sm"><p className="break-all">{selected.account.email || t('Email identity unavailable')}</p><p className="text-xs leading-5 text-muted-foreground">{t('Subscription account. Check the connection to refresh its email identity.')}</p></div>}{selected.account?.message && <p className="break-words text-xs leading-6 text-muted-foreground">{selected.account.message}</p>}{selected.account?.quota_retry_at && <p className="text-xs text-muted-foreground">{t('Next capacity check')}: {new Date(selected.account.quota_retry_at).toLocaleString()}</p>}{selected.byok?.api_key_masked && <p className="text-xs font-mono text-muted-foreground">{selected.byok.api_key_masked}</p>}
          <button type="button" className="action-secondary inline-flex items-center gap-2" disabled={checking || busy} onClick={() => void checkBrain()}><RefreshCw className={`h-4 w-4 ${checking ? 'animate-spin' : ''}`} />{t(checking ? 'Checking…' : 'Check connection')}</button>
        </div>
        {isOwner && <div className="border-t border-border/40 pt-5"><button type="button" className="min-h-11 text-sm text-muted-foreground hover:text-destructive" disabled={busy || checking} onClick={() => setDisconnectOpen(true)}>{t('Disconnect brain')}</button></div>}
      </div>}
    </SlideDrawer>
    <ActionDialog open={disconnectOpen} onClose={() => { if (!busy) setDisconnectOpen(false) }} title={t('Disconnect brain')} busy={busy}>
      <div className="space-y-4"><p className="text-sm leading-6 text-muted-foreground">{t('Disconnect this AI brain? Saved conversations and work will be preserved.')}</p><p className="break-all text-sm font-medium">{selected?.account?.email || selected?.label}</p>{drawerError && <p role="alert" className="text-sm text-destructive">{drawerError}</p>}<div className="flex justify-end gap-2"><button type="button" className="action-secondary" disabled={busy} onClick={() => setDisconnectOpen(false)}>{t('Cancel')}</button><button type="button" className="action-primary" disabled={busy} onClick={() => void disconnect()}>{t(busy ? 'Saving…' : 'Disconnect brain')}</button></div></div>
    </ActionDialog>
    <AddBrainModal open={addOpen} onClose={() => setAddOpen(false)} onBrainAdded={load} existingProviders={providers} />
  </div>
}
