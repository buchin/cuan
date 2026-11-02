import { useCallback, useEffect, useState } from 'react'
import {
  Bot,
  Cpu,
  Plus,
  RefreshCw,
  Trash2,
  AlertCircle,
  Eye,
  Layers,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useAuth } from '../../context/AuthContext'
import {
  aiProvidersApi,
  type AIProviderDTO,
  type TestAIProviderResponse,
} from '../../api/aiProviders'
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

export function AgentSettingsView({ showHeading = true }: { showHeading?: boolean }) {
  const { t } = useLanguage()
  const { isOwner } = useAuth()

  // Assistant Identity state
  const [assistantName, setAssistantName] = useState<string>(getStoredAssistantName)
  const [nameSaving, setNameSaving] = useState(false)
  const [nameSavedNotice, setNameSavedNotice] = useState(false)
  const [primaryAgent, setPrimaryAgent] = useState<Agent | null>(null)

  // Brain Cluster state
  const [providers, setProviders] = useState<AIProviderDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [isAddModalOpen, setIsAddModalOpen] = useState(false)

  // Testing feedback
  const [testingId, setTestingId] = useState<string | null>(null)
  const [testResults, setTestResults] = useState<Record<string, TestAIProviderResponse | null>>({})
  const [removingId, setRemovingId] = useState<string | null>(null)

  const loadData = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [providerList, agentList] = await Promise.allSettled([
        aiProvidersApi.list(),
        agentsApi.list(),
      ])

      if (providerList.status === 'fulfilled') {
        setProviders(providerList.value)
      } else {
        throw new Error(t('Gagal memuat daftar penyedia AI.'))
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
      // Non-fatal
      setNameSavedNotice(true)
      setTimeout(() => setNameSavedNotice(false), 2000)
    } finally {
      setNameSaving(false)
    }
  }

  const handleTestBrain = async (provider: AIProviderDTO) => {
    setTestingId(provider.id)
    try {
      const res = await aiProvidersApi.test({
        provider: provider.id,
        id: provider.id,
        model: provider.default_model,
      })
      setTestResults((prev) => ({ ...prev, [provider.id]: res }))
    } catch (err) {
      setTestResults((prev) => ({
        ...prev,
        [provider.id]: {
          status: 'error',
          valid: false,
          provider: provider.id,
          latency_ms: 0,
          error_message: err instanceof Error ? err.message : String(err),
        },
      }))
    } finally {
      setTestingId(null)
    }
  }

  const handleRemoveBrain = async (provider: AIProviderDTO) => {
    if (
      !window.confirm(
        t(`Putuskan akun AI ${provider.name} dari cluster? Otak ini tidak akan melayani tugas lagi.`)
      )
    ) {
      return
    }

    setRemovingId(provider.id)
    try {
      await aiProvidersApi.save({
        provider: provider.id,
        id: provider.id,
        api_key: '',
        enabled: false,
      })
      setNotice(t(`Akun ${provider.name} berhasil diputuskan.`))
      await loadData()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal memutuskan akun AI.'))
    } finally {
      setRemovingId(null)
    }
  }

  // Active connected brains in the cluster
  const connectedBrains = providers.filter(
    (p) => p.has_api_key || p.status === 'ready' || (p.enabled && p.mode === 'subscription')
  )

  const isClusterActive = connectedBrains.length > 0

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
        <div role="status" className="rounded-xl border border-primary/20 bg-primary/10 p-3.5 text-xs text-primary flex items-center justify-between">
          <span>{notice}</span>
          <button type="button" onClick={() => setNotice('')} className="underline text-xs ml-2">
            {t('Tutup')}
          </button>
        </div>
      )}

      {error && (
        <div role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 p-3.5 text-xs text-destructive flex items-center justify-between">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
          <button type="button" onClick={loadData} className="underline font-medium hover:opacity-80">
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
                  <span className={`h-1.5 w-1.5 rounded-full ${isClusterActive ? 'bg-emerald-500' : 'bg-amber-500'}`} />
                  <span>
                    {isClusterActive
                      ? t(`Siap Melayani (${connectedBrains.length} Otak Terhubung)`)
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
          <label htmlFor="assistant-name-input" className="block text-xs font-semibold text-foreground mb-1.5">
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

      {/* 2. Brain Cluster (Kumpulan Otak AI) */}
      <div className="space-y-3 pt-2">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <h3 className="text-sm font-bold tracking-tight text-foreground flex items-center gap-2">
              <Layers className="h-4 w-4 text-primary" />
              <span>{t('Kumpulan Otak AI (Brain Cluster)')}</span>
            </h3>
            <p className="text-xs text-muted-foreground mt-0.5 max-w-2xl">
              {t(
                'Akun-akun AI yang terhubung melayani request layaknya server cluster. Cuan otomatis membagi beban tugas (load balancing), beralih akun saat kuota habis (auto-failover), dan mengarahkan gambar ke otak berkemampuan Vision.'
              )}
            </p>
          </div>
          {isOwner && (
            <button
              type="button"
              onClick={() => setIsAddModalOpen(true)}
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors shrink-0"
            >
              <Plus className="h-4 w-4" />
              <span>{t('Tambah Otak AI')}</span>
            </button>
          )}
        </div>

        {/* Connected Brains List */}
        {loading ? (
          <div className="flex items-center justify-center p-10 text-muted-foreground gap-2">
            <RefreshCw className="h-4 w-4 animate-spin" />
            <span className="text-xs">{t('Memuat status cluster…')}</span>
          </div>
        ) : connectedBrains.length === 0 ? (
          /* Empty State: No connected brains yet */
          <div className="rounded-2xl border border-dashed border-border/60 p-8 sm:p-12 text-center bg-card/40 space-y-3">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
              <Cpu className="h-6 w-6" />
            </div>
            <h4 className="font-semibold text-foreground text-base">
              {t('Belum Ada Otak AI Terhubung')}
            </h4>
            <p className="text-xs sm:text-sm text-muted-foreground max-w-md mx-auto">
              {t(
                'Hubungkan akun OpenAI, Claude, Google, DeepSeek, Groq, atau Ollama Anda agar asisten siap mengeksekusi tugas.'
              )}
            </p>
            {isOwner && (
              <div className="pt-2">
                <button
                  type="button"
                  onClick={() => setIsAddModalOpen(true)}
                  className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors"
                >
                  <Plus className="h-4 w-4" />
                  <span>{t('Hubungkan Otak Pertama')}</span>
                </button>
              </div>
            )}
          </div>
        ) : (
          /* Connected Brain Nodes */
          <div className="rounded-2xl border border-border/40 bg-card/40 overflow-hidden shadow-xs">
            <HairlineList role="list" aria-label={t('Daftar Node Otak AI')}>
              {connectedBrains.map((brain) => {
                const testRes = testResults[brain.id]
                const isTesting = testingId === brain.id
                const isRemoving = removingId === brain.id

                return (
                  <div
                    role="listitem"
                    key={brain.id}
                    className="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
                  >
                    <div className="flex items-start gap-3.5 min-w-0">
                      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20">
                        <Cpu className="h-5 w-5" />
                      </div>

                      <div className="min-w-0 space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-semibold text-sm text-foreground">
                            {brain.name}
                          </span>
                          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 text-[10px] font-semibold rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30">
                            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                            <span>{t('Siap Melayani')}</span>
                          </span>
                          {/* Capability Badges */}
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 text-[10px] font-medium rounded-full bg-muted text-muted-foreground">
                            <Eye className="h-3 w-3 text-primary" />
                            <span>{t('Vision Ready')}</span>
                          </span>
                          {brain.default_model && (
                            <span className="text-[10px] font-mono text-muted-foreground bg-muted/60 px-2 py-0.5 rounded-full">
                              {brain.default_model}
                            </span>
                          )}
                        </div>

                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                          {brain.api_key_masked && (
                            <span className="font-mono text-[11px]">{brain.api_key_masked}</span>
                          )}
                          {brain.base_url && (
                            <span className="font-mono text-[11px]">{brain.base_url}</span>
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
                    <div className="flex items-center gap-2 self-end sm:self-center shrink-0">
                      <button
                        type="button"
                        disabled={isTesting}
                        onClick={() => handleTestBrain(brain)}
                        className="inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 rounded-lg border border-border/60 text-xs font-medium text-foreground hover:bg-muted transition-colors disabled:opacity-50"
                      >
                        <RefreshCw className={`h-3.5 w-3.5 ${isTesting ? 'animate-spin' : ''}`} />
                        <span>{isTesting ? t('Menguji…') : t('Tes Latency')}</span>
                      </button>

                      {isOwner && (
                        <button
                          type="button"
                          disabled={isRemoving}
                          onClick={() => handleRemoveBrain(brain)}
                          className="inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
                          title={t('Putuskan akun AI')}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                          <span>{t('Putuskan')}</span>
                        </button>
                      )}
                    </div>
                  </div>
                )
              })}
            </HairlineList>
          </div>
        )}
      </div>

      {/* Add Brain Modal */}
      <AddBrainModal
        open={isAddModalOpen}
        onClose={() => setIsAddModalOpen(false)}
        existingProviders={providers}
        onBrainAdded={() => {
          loadData()
          setNotice(t('Otak AI baru berhasil ditambahkan ke cluster!'))
        }}
      />
    </div>
  )
}
