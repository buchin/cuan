import { useEffect, useState, useId } from 'react'
import {
  Sparkles,
  Check,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  Download,
  Key,
  Globe,
  Bot,
  Star,
  Eye,
  EyeOff,
  Cpu,
  Zap,
  ShieldCheck,
  Activity,
  Layers,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import {
  aiProvidersApi,
  getDefaultStoreProvider,
  setDefaultStoreProvider,
  type AIProviderDTO,
  type TestAIProviderResponse,
} from '../../api/aiProviders'

export function AIProvidersView() {
  const { t } = useLanguage()
  const formBaseId = useId()

  const [providers, setProviders] = useState<AIProviderDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [defaultProvider, setDefaultProvider] = useState<string>(getDefaultStoreProvider())

  // Card form states mapped by provider ID
  const [modes, setModes] = useState<Record<string, 'subscription' | 'byok'>>({})
  const [apiKeys, setApiKeys] = useState<Record<string, string>>({})
  const [showKey, setShowKey] = useState<Record<string, boolean>>({})
  const [baseUrls, setBaseUrls] = useState<Record<string, string>>({})
  const [selectedModels, setSelectedModels] = useState<Record<string, string>>({})

  // Feedback states
  const [testingId, setTestingId] = useState<string | null>(null)
  const [savingId, setSavingId] = useState<string | null>(null)
  const [installingBinary, setInstallingBinary] = useState<string | null>(null)
  const [testResults, setTestResults] = useState<Record<string, TestAIProviderResponse | null>>({})
  const [messages, setMessages] = useState<Record<string, { type: 'success' | 'error'; text: string } | null>>({})

  const fetchProviders = async (silent = false) => {
    if (!silent) setLoading(true)
    setError(null)
    try {
      const list = await aiProvidersApi.list()
      setProviders(list)

      // Initialize form fields from loaded catalog
      const initialModes: Record<string, 'subscription' | 'byok'> = {}
      const initialBaseUrls: Record<string, string> = {}
      const initialModels: Record<string, string> = {}

      list.forEach((p) => {
        initialModes[p.id] = p.mode === 'subscription' ? 'subscription' : 'byok'
        if (p.base_url) initialBaseUrls[p.id] = p.base_url
        if (p.default_model) initialModels[p.id] = p.default_model
      })

      setModes((prev) => ({ ...initialModes, ...prev }))
      setBaseUrls((prev) => ({ ...initialBaseUrls, ...prev }))
      setSelectedModels((prev) => ({ ...initialModels, ...prev }))
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('Failed to load AI providers')
      setError(msg)
    } finally {
      if (!silent) setLoading(false)
    }
  }

  useEffect(() => {
    fetchProviders()
    const onDefaultChange = (e: Event) => {
      const customEvent = e as CustomEvent<{ provider: string }>
      if (customEvent.detail?.provider) {
        setDefaultProvider(customEvent.detail.provider)
      }
    }
    window.addEventListener('cuan:default-provider-changed', onDefaultChange)
    return () => window.removeEventListener('cuan:default-provider-changed', onDefaultChange)
  }, [])

  const handleSetDefaultStore = (provider: AIProviderDTO) => {
    setDefaultStoreProvider(provider.id, selectedModels[provider.id] || provider.default_model)
    setDefaultProvider(provider.id)
    setMessages((prev) => ({
      ...prev,
      [provider.id]: {
        type: 'success',
        text: t('Berhasil dijadikan default toko. Seluruh agen otomatis menggunakan {name}.').replace(
          '{name}',
          provider.name
        ),
      },
    }))
    setTimeout(() => {
      setMessages((prev) => ({ ...prev, [provider.id]: null }))
    }, 4000)
  }

  const handleModeChange = (providerId: string, newMode: 'subscription' | 'byok') => {
    setModes((prev) => ({ ...prev, [providerId]: newMode }))
    setTestResults((prev) => ({ ...prev, [providerId]: null }))
    setMessages((prev) => ({ ...prev, [providerId]: null }))
  }

  const handleTestConnection = async (provider: AIProviderDTO) => {
    setTestingId(provider.id)
    setMessages((prev) => ({ ...prev, [provider.id]: null }))
    try {
      const res = await aiProvidersApi.test({
        provider: provider.id,
        api_key: apiKeys[provider.id] || undefined,
        base_url: baseUrls[provider.id] || undefined,
        model: selectedModels[provider.id] || provider.default_model,
      })
      setTestResults((prev) => ({ ...prev, [provider.id]: res }))
      if (res.valid) {
        setMessages((prev) => ({
          ...prev,
          [provider.id]: {
            type: 'success',
            text: t('Koneksi berhasil! Latensi respon: {ms}ms').replace('{ms}', String(res.latency_ms)),
          },
        }))
      } else {
        setMessages((prev) => ({
          ...prev,
          [provider.id]: {
            type: 'error',
            text: res.error_message || t('Koneksi gagal. Periksa kembali API Key atau URL.'),
          },
        }))
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('Gagal melakukan uji koneksi')
      setTestResults((prev) => ({
        ...prev,
        [provider.id]: {
          status: 'error',
          valid: false,
          provider: provider.id,
          latency_ms: 0,
          error_message: msg,
        },
      }))
      setMessages((prev) => ({ ...prev, [provider.id]: { type: 'error', text: msg } }))
    } finally {
      setTestingId(null)
    }
  }

  const handleSaveConfig = async (provider: AIProviderDTO) => {
    setSavingId(provider.id)
    setMessages((prev) => ({ ...prev, [provider.id]: null }))
    try {
      const mode = modes[provider.id] || provider.mode
      const rawKey = apiKeys[provider.id] || ''
      await aiProvidersApi.save({
        provider: provider.id,
        mode,
        api_key: rawKey.trim() ? rawKey.trim() : undefined,
        base_url: baseUrls[provider.id] || '',
        default_model: selectedModels[provider.id] || provider.default_model,
        enabled: true,
      })

      setMessages((prev) => ({
        ...prev,
        [provider.id]: {
          type: 'success',
          text: t('Pengaturan {name} berhasil disimpan dengan aman.').replace('{name}', provider.name),
        },
      }))

      // Clear plain API key from input state once saved to protect user privacy
      setApiKeys((prev) => ({ ...prev, [provider.id]: '' }))

      // Refresh catalog silently to get updated masking and statuses
      await fetchProviders(true)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('Gagal menyimpan konfigurasi')
      setMessages((prev) => ({ ...prev, [provider.id]: { type: 'error', text: msg } }))
    } finally {
      setSavingId(null)
    }
  }

  const handleInstallBinary = async (provider: AIProviderDTO) => {
    const binaryName = provider.binary_status?.name || (provider.id === 'anthropic' ? 'claude' : provider.id === 'openai' ? 'codex' : 'agy')
    setInstallingBinary(binaryName)
    setMessages((prev) => ({ ...prev, [provider.id]: null }))
    try {
      const res = await aiProvidersApi.installBinary(binaryName)
      setMessages((prev) => ({
        ...prev,
        [provider.id]: {
          type: 'success',
          text: res.message || t('Binary CLI {name} berhasil dipasang ke user-space (~/.local/bin).').replace('{name}', binaryName),
        },
      }))
      await fetchProviders(true)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('Gagal memasang binary CLI')
      setMessages((prev) => ({ ...prev, [provider.id]: { type: 'error', text: msg } }))
    } finally {
      setInstallingBinary(null)
    }
  }

  const defaultProviderObj = providers.find((p) => p.id === defaultProvider)

  return (
    <div className="space-y-6">
      {/* Overview & Store Default Hero Banner */}
      <section className="relative overflow-hidden rounded-xl border border-primary/20 bg-gradient-to-r from-primary/10 via-background to-accent/10 p-5 md:p-6 shadow-xs">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="space-y-1.5 max-w-2xl">
            <div className="inline-flex items-center gap-2 rounded-full bg-primary/15 px-3 py-1 text-xs font-semibold text-primary">
              <Zap className="h-3.5 w-3.5" />
              <span>{t('Arsitektur Dual-Mode & Zero-Friction')}</span>
            </div>
            <h3 className="text-xl font-bold tracking-tight text-foreground">
              {t('Penyedia AI & Mesin Cerdas')}
            </h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              {t(
                'Set Sekali, Jalan Semua. Pilih penyedia AI default untuk toko Anda, atau gunakan akun langganan resmi & API Key hemat langsung tanpa perlu membuka terminal.'
              )}
            </p>
          </div>

          <div className="shrink-0 flex flex-col sm:items-end justify-center gap-1.5 bg-card/60 backdrop-blur-xs p-3.5 rounded-lg border border-border">
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
              {t('Default Toko Saat Ini')}
            </span>
            <div className="flex items-center gap-2">
              <Star className="h-4 w-4 fill-amber-500 text-amber-500" />
              <span className="text-base font-bold text-foreground">
                {defaultProviderObj ? defaultProviderObj.name : defaultProvider || 'Google Gemini'}
              </span>
            </div>
            <span className="text-xs text-muted-foreground">
              {t('Semua agen otomatis memakai engine ini')}
            </span>
          </div>
        </div>
      </section>

      {/* Global Error Banner */}
      {error && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive flex items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <AlertCircle className="h-5 w-5 shrink-0" />
            <span>{error}</span>
          </div>
          <button
            type="button"
            onClick={() => fetchProviders()}
            className="action-secondary min-h-11 px-3 text-xs shrink-0"
          >
            {t('Coba Lagi')}
          </button>
        </div>
      )}

      {/* Loading Skeleton */}
      {loading && providers.length === 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
          {[1, 2, 3, 4].map((i) => (
            <div
              key={i}
              className="rounded-xl border border-border bg-card p-5 space-y-4 animate-pulse min-h-[320px]"
            >
              <div className="flex items-center justify-between">
                <div className="h-6 w-36 bg-muted rounded" />
                <div className="h-6 w-20 bg-muted rounded-full" />
              </div>
              <div className="h-10 w-full bg-muted/60 rounded" />
              <div className="space-y-2">
                <div className="h-4 w-28 bg-muted rounded" />
                <div className="h-10 w-full bg-muted rounded" />
              </div>
              <div className="h-11 w-full bg-muted/80 rounded" />
            </div>
          ))}
        </div>
      )}

      {/* Catalog Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
        {providers.map((provider) => {
          const currentMode = modes[provider.id] || (provider.mode === 'subscription' ? 'subscription' : 'byok')
          const supportsSubscription = provider.supported_modes?.includes('subscription')
          const isStoreDefault = defaultProvider.toLowerCase() === provider.id.toLowerCase()
          const testRes = testResults[provider.id]
          const msg = messages[provider.id]
          const isTesting = testingId === provider.id
          const isSaving = savingId === provider.id
          const binName = provider.binary_status?.name || (provider.id === 'anthropic' ? 'claude' : provider.id === 'openai' ? 'codex' : 'agy')
          const isInstalling = installingBinary === binName
          const isBinInstalled = Boolean(provider.binary_status?.installed)

          // Provider Specific Badges & Highlights
          const isDeepSeek = provider.id.toLowerCase() === 'deepseek'
          const isGoogle = provider.id.toLowerCase() === 'google' || provider.id.toLowerCase() === 'gemini'

          return (
            <article
              key={provider.id}
              className={`flex flex-col justify-between rounded-xl border bg-card transition-all shadow-xs ${
                isStoreDefault
                  ? 'border-primary ring-2 ring-primary/20 shadow-md'
                  : 'border-border hover:border-border/80'
              }`}
            >
              {/* Card Header */}
              <div className="p-5 border-b border-border/60 space-y-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                      {isDeepSeek ? (
                        <Zap className="h-6 w-6 text-emerald-500" />
                      ) : isGoogle ? (
                        <Sparkles className="h-6 w-6 text-blue-500" />
                      ) : provider.id === 'anthropic' ? (
                        <Cpu className="h-6 w-6 text-amber-600" />
                      ) : (
                        <Bot className="h-6 w-6 text-primary" />
                      )}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <h4 className="text-base font-bold text-foreground">
                          {provider.name}
                        </h4>
                        {isDeepSeek && (
                          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 px-2.5 py-0.5 text-xs font-semibold">
                            <Sparkles className="h-3 w-3" />
                            {t('Paling Hemat')}
                          </span>
                        )}
                        {isGoogle && (
                          <span className="inline-flex items-center gap-1 rounded-full bg-blue-500/15 text-blue-600 dark:text-blue-400 px-2.5 py-0.5 text-xs font-semibold">
                            {t('Bawaan Cuan')}
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        {isDeepSeek
                          ? t('Model cerdas open-weights dengan efisiensi biaya tertinggi untuk UMKM.')
                          : isGoogle
                            ? t('Mesin bawaan siap pakai terintegrasi harness Google Antigravity.')
                            : provider.id === 'anthropic'
                              ? t('Kecerdasan penalaran mendalam Claude Pro/Team via CLI resmi.')
                              : provider.id === 'openai'
                                ? t('Eksekusi GPT-4o via ChatGPT Plus/Team atau API Key langsung.')
                                : t('Penyedia model AI global dengan fleksibilitas multi-engine.')}
                      </p>
                    </div>
                  </div>

                  {isStoreDefault && (
                    <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-primary/15 text-primary px-2.5 py-1 text-xs font-semibold">
                      <Star className="h-3.5 w-3.5 fill-current" />
                      {t('Default Toko')}
                    </span>
                  )}
                </div>

                {/* Mode Selector Toggle (Langganan Flat vs BYOK) */}
                {supportsSubscription ? (
                  <div className="pt-1">
                    <label className="text-xs font-semibold text-muted-foreground block mb-1.5">
                      {t('Pilihan Mode Eksekusi')}
                    </label>
                    <div className="grid grid-cols-2 p-1 bg-muted/50 rounded-lg gap-1 border border-border/50">
                      <button
                        type="button"
                        onClick={() => handleModeChange(provider.id, 'subscription')}
                        className={`min-h-10 text-xs font-medium rounded-md transition-all flex items-center justify-center gap-1.5 ${
                          currentMode === 'subscription'
                            ? 'bg-card text-foreground font-semibold shadow-xs'
                            : 'text-muted-foreground hover:text-foreground'
                        }`}
                      >
                        <Layers className="h-3.5 w-3.5" />
                        <span>{t('Mode Langganan Flat')}</span>
                      </button>
                      <button
                        type="button"
                        onClick={() => handleModeChange(provider.id, 'byok')}
                        className={`min-h-10 text-xs font-medium rounded-md transition-all flex items-center justify-center gap-1.5 ${
                          currentMode === 'byok'
                            ? 'bg-card text-foreground font-semibold shadow-xs'
                            : 'text-muted-foreground hover:text-foreground'
                        }`}
                      >
                        <Key className="h-3.5 w-3.5" />
                        <span>{t('Mode API Key (BYOK)')}</span>
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="pt-1">
                    <div className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground bg-muted/40 px-3 py-1.5 rounded-md border border-border/40">
                      <Key className="h-3.5 w-3.5 text-primary" />
                      <span>{t('Mode Eksklusif: API Key Hemat (BYOK)')}</span>
                    </div>
                  </div>
                )}
              </div>

              {/* Card Body - Content per Mode */}
              <div className="p-5 space-y-4 flex-1">
                {currentMode === 'subscription' && supportsSubscription ? (
                  /* Mode Langganan Flat Panel */
                  <div className="space-y-4">
                    <div className="rounded-lg border border-border/80 bg-muted/20 p-4 space-y-3">
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-xs font-medium text-foreground">
                          {t('Status Binary Harness')} ({binName})
                        </span>
                        {isBinInstalled ? (
                          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 px-2.5 py-0.5 text-xs font-medium">
                            <CheckCircle2 className="h-3.5 w-3.5" />
                            {t('Terpasang')}
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/15 text-amber-600 dark:text-amber-400 px-2.5 py-0.5 text-xs font-medium">
                            <AlertCircle className="h-3.5 w-3.5" />
                            {t('Belum Terpasang')}
                          </span>
                        )}
                      </div>

                      <p className="text-xs text-muted-foreground">
                        {isBinInstalled
                          ? t('Binary resmi terpasang di user-space: {path}. Siap digunakan tanpa biaya token API.').replace(
                              '{path}',
                              provider.binary_status?.path || '~/.local/bin'
                            )
                          : t('Binary CLI resmi belum terpasang. Klik tombol di bawah untuk memasang otomatis ke ruang pengguna tanpa hak akses root.')}
                      </p>

                      <button
                        type="button"
                        onClick={() => handleInstallBinary(provider)}
                        disabled={isInstalling}
                        className="action-secondary min-h-11 w-full flex items-center justify-center gap-2 text-xs font-medium"
                      >
                        {isInstalling ? (
                          <>
                            <RefreshCw className="h-4 w-4 animate-spin text-primary" />
                            <span>{t('Memasang binary resmi ke user-space...')}</span>
                          </>
                        ) : (
                          <>
                            <Download className="h-4 w-4 text-primary" />
                            <span>
                              {isBinInstalled
                                ? t('Perbarui Binary 1-Klik (~/.local/bin)')
                                : t('Pasang Binary 1-Klik (~/.local/bin)')}
                            </span>
                          </>
                        )}
                      </button>
                    </div>

                    <div className="space-y-1.5">
                      <label
                        htmlFor={`${formBaseId}-sub-model-${provider.id}`}
                        className="text-xs font-medium text-foreground block"
                      >
                        {t('Model Default Langganan')}
                      </label>
                      <select
                        id={`${formBaseId}-sub-model-${provider.id}`}
                        value={selectedModels[provider.id] || provider.default_model || ''}
                        onChange={(e) =>
                          setSelectedModels((prev) => ({ ...prev, [provider.id]: e.target.value }))
                        }
                        className="field-control text-xs min-h-11 w-full"
                      >
                        {provider.models && provider.models.length > 0 ? (
                          provider.models.map((m) => (
                            <option key={m.id} value={m.id}>
                              {m.display_name || m.name || m.id}
                            </option>
                          ))
                        ) : (
                          <option value={provider.default_model || ''}>
                            {provider.default_model || t('Model Bawaan')}
                          </option>
                        )}
                      </select>
                    </div>
                  </div>
                ) : (
                  /* Mode BYOK Panel */
                  <div className="space-y-3.5">
                    {/* API Key Input */}
                    <div className="space-y-1.5">
                      <div className="flex items-center justify-between">
                        <label
                          htmlFor={`${formBaseId}-key-${provider.id}`}
                          className="text-xs font-medium text-foreground"
                        >
                          {t('API Key {name}').replace('{name}', provider.name)}
                        </label>
                        {provider.has_api_key && (
                          <span className="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                            <ShieldCheck className="h-3.5 w-3.5" />
                            {t('Kunci Tersimpan Aman')}
                          </span>
                        )}
                      </div>

                      <div className="relative">
                        <input
                          id={`${formBaseId}-key-${provider.id}`}
                          type={showKey[provider.id] ? 'text' : 'password'}
                          value={apiKeys[provider.id] || ''}
                          onChange={(e) =>
                            setApiKeys((prev) => ({ ...prev, [provider.id]: e.target.value }))
                          }
                          placeholder={
                            provider.has_api_key
                              ? provider.api_key_masked || 'sk-••••••••••••'
                              : isDeepSeek
                                ? 'sk-...'
                                : t('Masukkan API Key...')
                          }
                          className="field-control text-xs min-h-11 pr-10 w-full font-mono"
                          autoComplete="off"
                          spellCheck={false}
                        />
                        <button
                          type="button"
                          onClick={() =>
                            setShowKey((prev) => ({ ...prev, [provider.id]: !prev[provider.id] }))
                          }
                          className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground p-1"
                          aria-label={t(showKey[provider.id] ? 'Sembunyikan Kunci' : 'Lihat Kunci')}
                        >
                          {showKey[provider.id] ? (
                            <EyeOff className="h-4 w-4" />
                          ) : (
                            <Eye className="h-4 w-4" />
                          )}
                        </button>
                      </div>
                      <p className="text-[11px] text-muted-foreground">
                        {t('Kredensial dienkripsi AES-256-GCM. Tidak pernah ditampilkan utuh kembali.')}
                      </p>
                    </div>

                    {/* Model Dropdown */}
                    <div className="space-y-1.5">
                      <label
                        htmlFor={`${formBaseId}-byok-model-${provider.id}`}
                        className="text-xs font-medium text-foreground block"
                      >
                        {t('Model Utama Pilihan')}
                      </label>
                      <select
                        id={`${formBaseId}-byok-model-${provider.id}`}
                        value={selectedModels[provider.id] || provider.default_model || ''}
                        onChange={(e) =>
                          setSelectedModels((prev) => ({ ...prev, [provider.id]: e.target.value }))
                        }
                        className="field-control text-xs min-h-11 w-full"
                      >
                        {provider.models && provider.models.length > 0 ? (
                          provider.models.map((m) => (
                            <option key={m.id} value={m.id}>
                              {m.display_name || m.name || m.id}
                            </option>
                          ))
                        ) : (
                          <option value={provider.default_model || ''}>
                            {provider.default_model || t('Model Bawaan')}
                          </option>
                        )}
                      </select>
                    </div>

                    {/* Optional Base URL (for Ollama, Groq, custom proxy) */}
                    <div className="space-y-1.5">
                      <label
                        htmlFor={`${formBaseId}-url-${provider.id}`}
                        className="text-xs font-medium text-muted-foreground block"
                      >
                        {t('URL Endpoint API Kustom (Opsional)')}
                      </label>
                      <div className="relative">
                        <Globe className="h-3.5 w-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                        <input
                          id={`${formBaseId}-url-${provider.id}`}
                          type="url"
                          value={baseUrls[provider.id] || ''}
                          onChange={(e) =>
                            setBaseUrls((prev) => ({ ...prev, [provider.id]: e.target.value }))
                          }
                          placeholder={
                            provider.id === 'ollama'
                              ? 'http://localhost:11434/v1'
                              : 'https://api.example.com/v1'
                          }
                          className="field-control text-xs min-h-11 pl-9 w-full font-mono"
                        />
                      </div>
                    </div>
                  </div>
                )}

                {/* Visual Latency & Test Feedback Badge */}
                {testRes && (
                  <div
                    className={`rounded-lg p-3 text-xs flex items-center justify-between gap-2 border ${
                      testRes.valid
                        ? testRes.latency_ms < 500
                          ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20'
                          : 'bg-amber-500/10 text-amber-700 dark:text-amber-300 border-amber-500/20'
                        : 'bg-destructive/10 text-destructive border-destructive/20'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <Activity className="h-4 w-4 shrink-0" />
                      <span>
                        {testRes.valid
                          ? t('Koneksi Terverifikasi ({status})').replace(
                              '{status}',
                              testRes.latency_ms < 500 ? t('Sangat Cepat') : t('Stabil')
                            )
                          : t('Koneksi Gagal')}
                      </span>
                    </div>

                    <span
                      className={`font-mono font-bold px-2 py-0.5 rounded text-[11px] ${
                        testRes.valid
                          ? testRes.latency_ms < 500
                            ? 'bg-emerald-500/20 text-emerald-800 dark:text-emerald-200'
                            : 'bg-amber-500/20 text-amber-800 dark:text-amber-200'
                          : 'bg-destructive/20 text-destructive'
                      }`}
                    >
                      {testRes.latency_ms} ms
                    </span>
                  </div>
                )}

                {/* Inline Message */}
                {msg && (
                  <div
                    className={`text-xs p-2.5 rounded-lg flex items-center gap-2 ${
                      msg.type === 'success'
                        ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                        : 'bg-destructive/10 text-destructive'
                    }`}
                  >
                    {msg.type === 'success' ? (
                      <Check className="h-4 w-4 shrink-0" />
                    ) : (
                      <AlertCircle className="h-4 w-4 shrink-0" />
                    )}
                    <span>{msg.text}</span>
                  </div>
                )}
              </div>

              {/* Card Footer Actions */}
              <div className="p-4 bg-muted/20 border-t border-border/60 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-2.5">
                <button
                  type="button"
                  onClick={() => handleSetDefaultStore(provider)}
                  disabled={isStoreDefault}
                  className={`min-h-11 px-3.5 text-xs font-semibold rounded-lg flex items-center justify-center gap-1.5 transition-all ${
                    isStoreDefault
                      ? 'bg-primary/10 text-primary border border-primary/30 cursor-default'
                      : 'action-secondary text-foreground hover:bg-accent'
                  }`}
                >
                  <Star
                    className={`h-4 w-4 ${isStoreDefault ? 'fill-primary text-primary' : 'text-muted-foreground'}`}
                  />
                  <span>
                    {isStoreDefault ? t('Default Toko Aktif') : t('Jadikan Default Toko')}
                  </span>
                </button>

                <div className="flex items-center gap-2 shrink-0">
                  {currentMode === 'byok' && (
                    <button
                      type="button"
                      onClick={() => handleTestConnection(provider)}
                      disabled={isTesting || isSaving}
                      className="action-secondary min-h-11 px-3 text-xs font-medium flex items-center justify-center gap-1.5 flex-1 sm:flex-initial"
                    >
                      {isTesting ? (
                        <>
                          <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                          <span>{t('Menguji...')}</span>
                        </>
                      ) : (
                        <>
                          <Activity className="h-3.5 w-3.5 text-primary" />
                          <span>{t('Uji Koneksi')}</span>
                        </>
                      )}
                    </button>
                  )}

                  <button
                    type="button"
                    onClick={() => handleSaveConfig(provider)}
                    disabled={isSaving}
                    className="action-primary min-h-11 px-4 text-xs font-semibold flex items-center justify-center gap-1.5 flex-1 sm:flex-initial"
                  >
                    {isSaving ? (
                      <>
                        <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                        <span>{t('Menyimpan...')}</span>
                      </>
                    ) : (
                      <>
                        <Check className="h-3.5 w-3.5" />
                        <span>{t('Simpan')}</span>
                      </>
                    )}
                  </button>
                </div>
              </div>
            </article>
          )
        })}
      </div>
    </div>
  )
}
