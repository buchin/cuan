import { useCallback, useEffect, useState } from 'react'
import {
  Bot,
  Plus,
  RefreshCw,
  Trash2,
  AlertCircle,
  Eye,
  Layers,
  Sparkles,
  Key,
  Shield,
  ShieldCheck,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useAuth } from '../../context/AuthContext'
import {
  aiProvidersApi,
  type AIProviderDTO,
  type TestAIProviderResponse,
} from '../../api/aiProviders'
import { accountsApi, type ProviderAccount } from '../../api/accounts'
import { agentsApi, type Agent } from '../../api/agents'
import { AddBrainModal } from './AddBrainModal'
import { HairlineList } from '../../components/layout/HairlineList'

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

export interface BrainClusterNode {
  id: string
  kind: 'subscription' | 'byok'
  provider: string
  name: string
  label: string
  email?: string
  maskedKey?: string
  model: string
  state: 'verified' | 'quota_limited' | 'login_required' | 'unavailable' | 'unknown'
  message?: string
  supportsVision: boolean
  allowFailover?: boolean
  account?: ProviderAccount
  byokProvider?: AIProviderDTO
}

export function AgentSettingsView({ showHeading = true }: { showHeading?: boolean }) {
  const { t } = useLanguage()
  const { isOwner } = useAuth()

  // Assistant Identity state
  const [assistantName, setAssistantName] = useState<string>(getStoredAssistantName)
  const [nameSaving, setNameSaving] = useState(false)
  const [nameSavedNotice, setNameSavedNotice] = useState(false)
  const [primaryAgent, setPrimaryAgent] = useState<Agent | null>(null)

  // Brain Cluster state
  const [accounts, setAccounts] = useState<ProviderAccount[]>([])
  const [providers, setProviders] = useState<AIProviderDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [isAddModalOpen, setIsAddModalOpen] = useState(false)

  // Testing & Action feedback
  const [testingId, setTestingId] = useState<string | null>(null)
  const [testResults, setTestResults] = useState<Record<string, TestAIProviderResponse | null>>({})
  const [removingId, setRemovingId] = useState<string | null>(null)
  const [togglingFailoverId, setTogglingFailoverId] = useState<string | null>(null)

  const loadData = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [accList, provList, agentList] = await Promise.allSettled([
        accountsApi.list(),
        aiProvidersApi.list(),
        agentsApi.list(),
      ])

      if (accList.status === 'fulfilled') {
        setAccounts(accList.value || [])
      }

      if (provList.status === 'fulfilled') {
        setProviders(provList.value || [])
      }

      if (agentList.status === 'fulfilled') {
        const foundPrimary =
          agentList.value.find((a) => a.id === 'agent_cuan' || a.name === '@cuan') ||
          agentList.value[0] ||
          null
        setPrimaryAgent(foundPrimary)
        if (foundPrimary && foundPrimary.name) {
          const rawName = foundPrimary.name.replace(/^@/, '')
          if (rawName && !localStorage.getItem(ASSISTANT_NAME_KEY)) {
            setAssistantName(rawName)
            setStoredAssistantName(rawName)
          }
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal memuat status cluster otak AI.'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleSaveName = async () => {
    const clean = assistantName.trim() || 'Cuan'
    setAssistantName(clean)
    setStoredAssistantName(clean)
    setNameSaving(true)
    try {
      if (primaryAgent) {
        await agentsApi.updateModelConfig(primaryAgent.id, {
          role: t('Asisten Bisnis & Orkestrator Utama'),
        })
      }
      setNameSavedNotice(true)
      setTimeout(() => setNameSavedNotice(false), 2000)
    } catch {
      // Fallback
    } finally {
      setNameSaving(false)
    }
  }

  // Unified list of all connected AI Brains
  const brainNodes: BrainClusterNode[] = []

  // 1. Add subscription accounts (Antigravity Google accounts, CommandCode, etc.)
  for (const acc of accounts) {
    if (acc.credentials_present || acc.state === 'verified' || acc.state === 'quota_limited') {
      const isGoogle = acc.provider === 'agy' || acc.provider === 'google'
      const isCC = acc.provider === 'commandcode'
      brainNodes.push({
        id: `account_${acc.id}`,
        kind: 'subscription',
        provider: isGoogle ? 'google' : isCC ? 'commandcode' : acc.provider,
        name: isGoogle ? 'Google (Antigravity)' : isCC ? 'CommandCode' : acc.label,
        label: acc.label,
        email: acc.email,
        model: acc.model_id || (isGoogle ? 'gemini-1.5-pro' : 'claude-3-7-sonnet'),
        state: acc.state,
        message: acc.message,
        supportsVision: true,
        allowFailover: acc.allow_failover,
        account: acc,
      })
    }
  }

  // 2. Add BYOK providers with configured API keys
  for (const p of providers) {
    if (p.has_api_key) {
      brainNodes.push({
        id: `byok_${p.id}`,
        kind: 'byok',
        provider: p.id,
        name: p.name,
        label: `${p.name} (BYOK)`,
        maskedKey: p.api_key_masked,
        model: p.default_model || p.models?.[0]?.id || 'default',
        state: 'verified',
        supportsVision: p.models?.some((m) => m.supports_vision) ?? false,
        byokProvider: p,
      })
    }
  }

  // Pending sign-ins (accounts that need Google authorization)
  const pendingAccounts = accounts.filter(
    (a) => !a.credentials_present && a.state === 'login_required'
  )

  const readyBrains = brainNodes.filter((b) => b.state === 'verified')
  const cooldownBrains = brainNodes.filter((b) => b.state === 'quota_limited')
  const isClusterActive = readyBrains.length > 0 || cooldownBrains.length > 0

  // Test connection on subscription account or latency on BYOK provider
  const handleTestBrain = async (node: BrainClusterNode) => {
    setTestingId(node.id)
    try {
      if (node.kind === 'subscription' && node.account) {
        const updated = await accountsApi.verify(node.account.id)
        setAccounts((old) => old.map((a) => (a.id === updated.id ? updated : a)))
        setNotice(
          t(`Koneksi ${node.email || node.label} terverifikasi: `) + (updated.message || 'Ready')
        )
      } else if (node.kind === 'byok' && node.byokProvider) {
        const res = await aiProvidersApi.test({
          provider: node.byokProvider.id,
          id: node.byokProvider.id,
          model: node.byokProvider.default_model,
        })
        setTestResults((prev) => ({ ...prev, [node.id]: res }))
      }
    } catch (err) {
      if (node.kind === 'byok') {
        setTestResults((prev) => ({
          ...prev,
          [node.id]: {
            status: 'error',
            valid: false,
            provider: node.provider,
            latency_ms: 0,
            error_message: err instanceof Error ? err.message : String(err),
          },
        }))
      } else {
        setError(err instanceof Error ? err.message : t('Gagal memverifikasi koneksi akun.'))
      }
    } finally {
      setTestingId(null)
    }
  }

  // Toggle backup / failover on an account
  const handleToggleFailover = async (node: BrainClusterNode) => {
    if (!node.account) return
    const target = node.account
    setTogglingFailoverId(node.id)
    try {
      const updated = await accountsApi.failover(target.id, !target.allow_failover)
      setAccounts((old) => old.map((a) => (a.id === updated.id ? updated : a)))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal mengubah pengaturan cadangan.'))
    } finally {
      setTogglingFailoverId(null)
    }
  }

  // Disconnect / Delete brain
  const handleRemoveBrain = async (node: BrainClusterNode) => {
    if (
      !window.confirm(
        t(
          `Putuskan akun AI ${node.email || node.label || node.name} dari cluster? Otak ini tidak akan melayani tugas lagi.`
        )
      )
    ) {
      return
    }

    setRemovingId(node.id)
    try {
      if (node.kind === 'subscription' && node.account) {
        await accountsApi.delete(node.account.id)
        setAccounts((old) => old.filter((a) => a.id !== node.account?.id))
        setNotice(t('Akun langganan berhasil diputuskan dari cluster.'))
      } else if (node.kind === 'byok' && node.byokProvider) {
        await aiProvidersApi.save({
          provider: node.byokProvider.id,
          id: node.byokProvider.id,
          api_key: '',
          enabled: false,
        })
        setProviders((old) =>
          old.map((p) =>
            p.id === node.byokProvider?.id
              ? { ...p, has_api_key: false, api_key_masked: undefined, status: 'unconfigured' }
              : p
          )
        )
        setNotice(t('Kunci API berhasil dihapus dari cluster.'))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal memutuskan otak AI.'))
    } finally {
      setRemovingId(null)
    }
  }

  // Remove a pending login attempt
  const handleRemovePendingAccount = async (account: ProviderAccount) => {
    try {
      await accountsApi.delete(account.id)
      setAccounts((old) => old.filter((a) => a.id !== account.id))
    } catch {
      // Fallback
    }
  }

  return (
    <div className="w-full space-y-6 pb-8">
      {/* Heading */}
      {showHeading && (
        <div className="pb-2 border-b border-border/40">
          <h2 className="text-xl font-bold tracking-tight text-foreground flex items-center gap-2">
            <Bot className="h-5 w-5 text-primary" />
            <span>{t('Asisten AI & Kumpulan Otak')}</span>
          </h2>
          <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
            {t('Atur nama asisten dan hubungkan akun-akun AI yang menjadi otak komputasi di balik layar.')}
          </p>
        </div>
      )}

      {notice && (
        <div
          role="status"
          className="rounded-xl border border-primary/20 bg-primary/10 p-3.5 text-xs text-primary flex items-center justify-between"
        >
          <span>{notice}</span>
          <button type="button" onClick={() => setNotice('')} className="underline text-xs ml-2">
            {t('Tutup')}
          </button>
        </div>
      )}

      {error && (
        <div
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/10 p-3.5 text-xs text-destructive flex items-center justify-between"
        >
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
          <button
            type="button"
            onClick={loadData}
            className="underline font-medium hover:opacity-80"
          >
            {t('Coba lagi')}
          </button>
        </div>
      )}

      {/* 1. Assistant Identity Card */}
      <div className="rounded-2xl border border-border/60 bg-card/60 p-5 sm:p-6 space-y-4 shadow-xs">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-3.5">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary border border-primary/20">
              <Bot className="h-6 w-6" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-bold text-base text-foreground">{assistantName}</h3>
                <span
                  className={`inline-flex items-center gap-1 px-2.5 py-0.5 text-[10px] font-semibold rounded-full border ${
                    isClusterActive
                      ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30'
                      : 'bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30'
                  }`}
                >
                  <span
                    className={`h-1.5 w-1.5 rounded-full ${
                      isClusterActive ? 'bg-emerald-500' : 'bg-amber-500'
                    }`}
                  />
                  <span>
                    {isClusterActive
                      ? cooldownBrains.length > 0
                        ? t(
                            `Siap Melayani (${readyBrains.length} Siap · ${cooldownBrains.length} Cooldown)`
                          )
                        : t(`Siap Melayani (${readyBrains.length} Otak Terhubung)`)
                      : t('Belum Ada Otak Terhubung')}
                  </span>
                </span>
              </div>
              <p className="text-xs text-muted-foreground mt-0.5">
                {t('Satu asisten cerdas untuk semua percakapan, tugas, dan proyek bisnis Anda.')}
              </p>
            </div>
          </div>
        </div>

        {/* Name input */}
        <div className="pt-2 border-t border-border/40 max-w-md">
          <label
            htmlFor="assistant-name-input"
            className="block text-xs font-semibold text-foreground mb-1.5"
          >
            {t('Nama Asisten')}
          </label>
          <div className="flex items-center gap-2">
            <input
              id="assistant-name-input"
              type="text"
              value={assistantName}
              onChange={(e) => setAssistantName(e.target.value)}
              onBlur={handleSaveName}
              placeholder="Cuan"
              className="field-control flex-1 text-sm rounded-xl font-medium"
            />
            <button
              type="button"
              onClick={handleSaveName}
              disabled={nameSaving}
              className="action-secondary min-h-11 px-3.5 text-xs rounded-xl"
            >
              {nameSaving ? (
                <RefreshCw className="h-3.5 w-3.5 animate-spin" />
              ) : nameSavedNotice ? (
                <span className="text-emerald-500 font-semibold">{t('Tersimpan!')}</span>
              ) : (
                t('Simpan')
              )}
            </button>
          </div>
          <p className="text-[11px] text-muted-foreground mt-1">
            {t('Nama ini akan tampil di seluruh pesan, instruksi, dan ruang obrolan Anda.')}
          </p>
        </div>
      </div>

      {/* 2. Brain Cluster Header & List */}
      <div className="space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <h3 className="text-base font-bold tracking-tight text-foreground flex items-center gap-2">
              <Layers className="h-4 w-4 text-primary" />
              <span>{t('Kumpulan Otak AI (Brain Cluster)')}</span>
            </h3>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {t(
                'Akun-akun AI yang terhubung melayani request layaknya server cluster. Cuan otomatis membagi beban tugas (load balancing), beralih akun saat kuota habis (auto-failover), dan mengarahkan gambar ke otak berkemampuan Vision.'
              )}
            </p>
          </div>

          {isOwner && (
            <button
              type="button"
              onClick={() => setIsAddModalOpen(true)}
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-xs hover:bg-primary/90 transition-colors shrink-0"
            >
              <Plus className="h-4 w-4" />
              <span>{t('Tambah Otak AI')}</span>
            </button>
          )}
        </div>

        {/* Nodes List */}
        {loading ? (
          <div className="p-8 text-center rounded-2xl border border-border/40 bg-card/40 space-y-2">
            <RefreshCw className="h-5 w-5 animate-spin mx-auto text-muted-foreground" />
            <p className="text-xs text-muted-foreground">{t('Memuat daftar otak AI...')}</p>
          </div>
        ) : brainNodes.length === 0 ? (
          <div className="p-8 text-center rounded-2xl border border-dashed border-border bg-card/20 space-y-3">
            <div className="flex h-12 w-12 mx-auto items-center justify-center rounded-2xl bg-muted text-muted-foreground">
              <Bot className="h-6 w-6" />
            </div>
            <div>
              <h4 className="font-semibold text-sm text-foreground">
                {t('Belum ada otak AI yang terhubung')}
              </h4>
              <p className="text-xs text-muted-foreground mt-1 max-w-sm mx-auto">
                {t(
                  'Hubungkan akun langganan Google AI Pro, CommandCode, atau kunci API pribadi Anda agar asisten dapat mulai bekerja.'
                )}
              </p>
            </div>
            {isOwner && (
              <button
                type="button"
                onClick={() => setIsAddModalOpen(true)}
                className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2 text-xs font-semibold text-primary-foreground shadow-xs hover:bg-primary/90 transition-colors"
              >
                <Plus className="h-4 w-4" />
                <span>{t('Hubungkan Otak Pertama')}</span>
              </button>
            )}
          </div>
        ) : (
          <div className="rounded-2xl border border-border/60 bg-card/60 shadow-xs overflow-hidden">
            <HairlineList>
              {brainNodes.map((brain) => {
                const isTesting = testingId === brain.id
                const isRemoving = removingId === brain.id
                const isTogglingFailover = togglingFailoverId === brain.id
                const testRes = testResults[brain.id]

                const isVerified = brain.state === 'verified'
                const isCooldown = brain.state === 'quota_limited'

                return (
                  <div
                    key={brain.id}
                    className="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 transition-colors hover:bg-muted/20"
                  >
                    <div className="flex items-start gap-3.5 min-w-0">
                      {/* Avatar */}
                      <div
                        className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl font-bold text-xs ${
                          brain.provider === 'google'
                            ? 'bg-blue-500/10 text-blue-500 border border-blue-500/20'
                            : brain.provider === 'commandcode'
                            ? 'bg-amber-500/10 text-amber-500 border border-amber-500/20'
                            : brain.provider === 'anthropic'
                            ? 'bg-orange-500/10 text-orange-500 border border-orange-500/20'
                            : brain.provider === 'openai'
                            ? 'bg-emerald-500/10 text-emerald-500 border border-emerald-500/20'
                            : 'bg-primary/10 text-primary border border-primary/20'
                        }`}
                      >
                        {brain.provider === 'google'
                          ? 'G'
                          : brain.provider === 'commandcode'
                          ? 'CC'
                          : brain.name.slice(0, 2).toUpperCase()}
                      </div>

                      {/* Info & Badges */}
                      <div className="min-w-0 space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-semibold text-sm text-foreground truncate">
                            {brain.email
                              ? `${brain.name} · ${brain.email}`
                              : brain.label || brain.name}
                          </span>

                          {/* Status Badge */}
                          <span
                            className={`inline-flex items-center px-2 py-0.5 text-[10px] font-semibold rounded-full border ${
                              isVerified
                                ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30'
                                : isCooldown
                                ? 'bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30'
                                : 'bg-destructive/15 text-destructive border-destructive/30'
                            }`}
                          >
                            {isVerified
                              ? t('Siap Melayani')
                              : isCooldown
                              ? t('Kuota Cooldown')
                              : t('Perlu Cek')}
                          </span>

                          {/* Type Badge */}
                          <span className="inline-flex items-center px-2 py-0.5 text-[10px] rounded-full bg-muted text-muted-foreground font-medium">
                            {brain.kind === 'subscription' ? (
                              <span className="flex items-center gap-1">
                                <Sparkles className="h-3 w-3 text-amber-500" />
                                <span>{t('Langganan')}</span>
                              </span>
                            ) : (
                              <span className="flex items-center gap-1">
                                <Key className="h-3 w-3 text-primary" />
                                <span>{t('Kunci API')}</span>
                              </span>
                            )}
                          </span>

                          {/* Vision Badge */}
                          {brain.supportsVision && (
                            <span className="inline-flex items-center gap-1 px-2 py-0.5 text-[10px] rounded-full bg-blue-500/10 text-blue-500 border border-blue-500/20 font-medium">
                              <Eye className="h-3 w-3" />
                              <span>Vision Ready</span>
                            </span>
                          )}

                          {/* Model name */}
                          {brain.model && (
                            <span className="text-[10px] font-mono text-muted-foreground bg-muted/40 px-2 py-0.5 rounded-md">
                              {brain.model}
                            </span>
                          )}

                          {/* Failover / Backup Badge */}
                          {brain.allowFailover && (
                            <span className="inline-flex items-center gap-1 px-2 py-0.5 text-[10px] rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 font-medium">
                              <ShieldCheck className="h-3 w-3" />
                              <span>{t('Cadangan Aktif')}</span>
                            </span>
                          )}
                        </div>

                        {/* Cooldown or Error message */}
                        {brain.message && (
                          <p
                            className={`text-[11px] leading-relaxed ${
                              isCooldown
                                ? 'text-amber-600 dark:text-amber-400'
                                : 'text-muted-foreground'
                            }`}
                          >
                            {brain.message}
                          </p>
                        )}

                        {/* Masked Key & Latency */}
                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground pt-0.5">
                          {brain.maskedKey && (
                            <span className="font-mono text-[11px]">{brain.maskedKey}</span>
                          )}
                          {testRes && (
                            <span
                              className={`text-[11px] font-medium ${
                                testRes.valid
                                  ? 'text-emerald-600 dark:text-emerald-400'
                                  : 'text-destructive'
                              }`}
                            >
                              {testRes.valid
                                ? `✓ Latency: ${testRes.latency_ms ?? 0}ms`
                                : `✕ ${testRes.error_message || t('Uji koneksi gagal')}`}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Node Actions */}
                    <div className="flex flex-wrap items-center gap-2 self-start sm:self-center shrink-0">
                      {/* Failover toggle for subscription accounts */}
                      {brain.kind === 'subscription' && brain.account && isOwner && (
                        <button
                          type="button"
                          disabled={isTogglingFailover}
                          onClick={() => handleToggleFailover(brain)}
                          title={t('Jadikan akun cadangan otomatis saat kuota utama habis')}
                          className={`inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs font-medium transition-colors ${
                            brain.allowFailover
                              ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'border-border/60 text-muted-foreground hover:bg-muted'
                          }`}
                        >
                          <Shield className="h-3.5 w-3.5" />
                          <span>
                            {brain.allowFailover ? t('Cadangan (Aktif)') : t('+ Jadikan Cadangan')}
                          </span>
                        </button>
                      )}

                      {/* Test Connection / Latency */}
                      <button
                        type="button"
                        disabled={isTesting}
                        onClick={() => handleTestBrain(brain)}
                        className="inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 rounded-lg border border-border/60 text-xs font-medium text-foreground hover:bg-muted transition-colors disabled:opacity-50"
                      >
                        <RefreshCw className={`h-3.5 w-3.5 ${isTesting ? 'animate-spin' : ''}`} />
                        <span>{isTesting ? t('Menguji…') : t('Tes Koneksi')}</span>
                      </button>

                      {/* Remove Brain */}
                      {isOwner && (
                        <button
                          type="button"
                          disabled={isRemoving}
                          onClick={() => handleRemoveBrain(brain)}
                          className="inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                          <span>{isRemoving ? t('Memutuskan…') : t('Putuskan')}</span>
                        </button>
                      )}
                    </div>
                  </div>
                )
              })}
            </HairlineList>
          </div>
        )}

        {/* 3. Pending Sign-Ins (if any) */}
        {pendingAccounts.length > 0 && (
          <div className="pt-2">
            <details className="group rounded-2xl border border-amber-500/30 bg-amber-500/5 p-4 text-xs space-y-3">
              <summary className="cursor-pointer font-semibold text-amber-600 dark:text-amber-400 flex items-center justify-between">
                <span>
                  {t('Otorisasi Belum Selesai')} ({pendingAccounts.length})
                </span>
                <span className="text-[11px] font-normal text-muted-foreground group-open:hidden">
                  {t('Lihat detail')}
                </span>
              </summary>
              <div className="space-y-2 pt-2 border-t border-amber-500/20">
                {pendingAccounts.map((acc) => (
                  <div
                    key={acc.id}
                    className="flex items-center justify-between p-2 rounded-xl bg-card/60 border border-border/40"
                  >
                    <div>
                      <span className="font-semibold text-foreground">{acc.label}</span>
                      <p className="text-[11px] text-muted-foreground mt-0.5">
                        {acc.message || t('Menunggu otorisasi Google')}
                      </p>
                    </div>
                    {isOwner && (
                      <button
                        type="button"
                        onClick={() => handleRemovePendingAccount(acc)}
                        className="text-destructive hover:underline text-xs px-2 py-1"
                      >
                        {t('Hapus Draf')}
                      </button>
                    )}
                  </div>
                ))}
              </div>
            </details>
          </div>
        )}
      </div>

      {/* Add Brain Modal */}
      <AddBrainModal
        open={isAddModalOpen}
        onClose={() => setIsAddModalOpen(false)}
        onBrainAdded={loadData}
        existingProviders={providers}
      />
    </div>
  )
}
