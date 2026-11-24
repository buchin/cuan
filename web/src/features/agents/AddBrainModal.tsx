import React, { useState, useEffect } from 'react'
import {
  ExternalLink,
  Eye,
  EyeOff,
  RefreshCw,
  Sparkles,
  Terminal,
  Check,
  Bot,
  Key,
  ChevronDown,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { aiProvidersApi, type AIProviderDTO, type AIProviderModelInfo } from '../../api/aiProviders'
import { accountsApi } from '../../api/accounts'
import { agentsApi } from '../../api/agents'
import { FormField, Input } from '../../components/ui/forms'
import { BrandLogo } from '../../components/ui/BrandLogo'
import { SlideDrawer } from '../../components/layout/SlideDrawer'

interface AddBrainModalProps {
  open: boolean
  onClose: () => void
  onBrainAdded: () => void
  existingProviders: AIProviderDTO[]
}

type ProviderHarness = 'claude' | 'grok' | 'codex' | 'google' | 'commandcode' | 'opencode'

// Provider engine compatibility mapping ('openai', 'anthropic', 'google', 'deepseek', 'groq', 'ollama')
export type SupportedProviderEngine = 'openai' | 'anthropic' | 'google' | 'deepseek' | 'groq' | 'ollama' | 'opencode' | 'claude' | 'codex' | 'grok'

interface ProviderOption {
  id: string
  harness: ProviderHarness
  name: string
  defaultHandle: string
  defaultModel: string
  keyPlaceholder: string
  keyHelpUrl: string
  defaultBaseUrl?: string
}

const PROVIDER_OPTIONS: ProviderOption[] = [
  {
    id: 'claude',
    harness: 'claude',
    name: 'Claude Code',
    defaultHandle: '@claude',
    defaultModel: 'claude-sonnet-5-5',
    keyPlaceholder: 'sk-ant-api03-...',
    keyHelpUrl: 'https://console.anthropic.com/settings/keys',
  },
  {
    id: 'grok',
    harness: 'grok',
    name: 'xAI (Grok)',
    defaultHandle: '@grok',
    defaultModel: 'grok-4.7',
    keyPlaceholder: 'xai-...',
    keyHelpUrl: 'https://console.x.ai',
  },
  {
    id: 'codex',
    harness: 'codex',
    name: 'OpenAI (Codex)',
    defaultHandle: '@codex',
    defaultModel: 'gpt-6.1-sol',
    keyPlaceholder: 'sk-proj-...',
    keyHelpUrl: 'https://platform.openai.com/api-keys',
  },
  {
    id: 'google',
    harness: 'google',
    name: 'Google (Antigravity)',
    defaultHandle: '@cuan',
    defaultModel: 'gemini-4-neon',
    keyPlaceholder: 'AIzaSy...',
    keyHelpUrl: 'https://aistudio.google.com/app/apikey',
  },
  {
    id: 'commandcode',
    harness: 'commandcode',
    name: 'CommandCode',
    defaultHandle: '@cc',
    defaultModel: 'claude-sonnet-5-5',
    keyPlaceholder: 'sk-...',
    keyHelpUrl: 'https://commandcode.ai/docs/byok',
  },
  {
    id: 'opencode',
    harness: 'opencode',
    name: 'OpenCode',
    defaultHandle: '@opencode',
    defaultModel: 'deepseek/deepseek-r1',
    keyPlaceholder: 'sk-or-v1-...',
    keyHelpUrl: 'https://openrouter.ai/keys',
    defaultBaseUrl: 'https://openrouter.ai/api/v1',
  },
]

export function AddBrainModal({
  open,
  onClose,
  onBrainAdded,
  existingProviders: _existingProviders,
}: AddBrainModalProps) {
  const { t } = useLanguage()

  // 1. Selected harness
  const [selectedId, setSelectedId] = useState<string>('claude')
  // 2. Connection method: 'subscription' (Official Login) vs 'byok' (API Key)
  const [authMethod, setAuthMethod] = useState<'subscription' | 'byok'>('subscription')

  // Progressive disclosure: Advanced technical settings collapsed by default
  const [showAdvanced, setShowAdvanced] = useState(false)

  // Subscription states
  const [cliLabel, setCliLabel] = useState('Claude Code')
  const [cliHandle, setCliHandle] = useState('@claude')
  const [cliAuthMethod, setCliAuthMethod] = useState<'browser' | 'token'>('browser')
  const [cliToken, setCliToken] = useState('')
  const [cliShowToken, setCliShowToken] = useState(false)
  const [cliStep, setCliStep] = useState<'init' | 'browser'>('init')
  const [cliSessionId, setCliSessionId] = useState('')
  const [_cliAuthUrl, setCliAuthUrl] = useState('')

  // Google OAuth states
  const [googleLabel, setGoogleLabel] = useState('Google Akun')
  const [googleAccountId, setGoogleAccountId] = useState('')
  const [googleSessionId, setGoogleSessionId] = useState('')
  const [_googleAuthUrl, setGoogleAuthUrl] = useState('')
  const [googleCode, setGoogleCode] = useState('')
  const [googleStep, setGoogleStep] = useState<'init' | 'code'>('init')

  // BYOK states
  const [apiKey, setApiKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [baseUrl, setBaseUrl] = useState('')
  const [customModel, setCustomModel] = useState('claude-sonnet-5-5')
  const [contextLimit, setContextLimit] = useState('128000')
  const [customAgentName, setCustomAgentName] = useState('Claude Code')
  const [customHandle, setCustomHandle] = useState('@claude')

  // Dynamic models discovery (auto-polled)
  const [discoveredModels, setDiscoveredModels] = useState<AIProviderModelInfo[]>([])
  const [fetchingModels, setFetchingModels] = useState(false)
  const [modelMode, setModelMode] = useState<'select' | 'manual'>('select')

  // Status & feedback
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const activeProvider =
    PROVIDER_OPTIONS.find((p) => p.id === selectedId) || PROVIDER_OPTIONS[0]

  // Poll models from provider endpoint
  const handleFetchLiveModels = async (silent = false) => {
    if (fetchingModels) return
    setFetchingModels(true)
    setError('')
    try {
      const res = await aiProvidersApi.fetchModels({
        provider: activeProvider.id,
        api_key: apiKey.trim() || undefined,
        base_url: baseUrl.trim() || undefined,
        refresh: true,
      })
      if (res.models && res.models.length > 0) {
        setDiscoveredModels(res.models)
        setModelMode('select')
        // Automatically select the default / top model
        const def = res.models.find((m) => m.is_default) || res.models[0]
        if (def) {
          setCustomModel(def.id)
          if (def.input_token_limit && def.input_token_limit > 0) {
            setContextLimit(String(def.input_token_limit))
          }
        }
        if (!silent) {
          setSuccess(t('Koneksi terverifikasi dan daftar model berhasil dimuat!'))
          setTimeout(() => setSuccess(''), 3000)
        }
      } else if (!silent) {
        setError(t('Endpoint terhubung namun tidak mengembalikan daftar model. Menggunakan daftar bawaan catalog.'))
      }
    } catch {
      if (!silent) {
        setError(
          t(
            'Tidak dapat memverifikasi atau mengambil model dari endpoint. Periksa API key atau koneksi internet.'
          )
        )
      }
    } finally {
      setFetchingModels(false)
    }
  }

  // Synchronize defaults whenever selected provider changes
  useEffect(() => {
    setError('')
    setSuccess('')
    setCliStep('init')
    setGoogleStep('init')
    setCliToken('')
    setApiKey('')
    setBaseUrl(activeProvider.defaultBaseUrl || '')
    setCustomModel(activeProvider.defaultModel)
    setShowAdvanced(false)
    setModelMode('select')

    setCliLabel(activeProvider.name)
    setCliHandle(activeProvider.defaultHandle)
    setCustomAgentName(activeProvider.name)
    setCustomHandle(activeProvider.defaultHandle)

    // Default to BYOK for OpenCode
    if (activeProvider.harness === 'opencode') {
      setAuthMethod('byok')
    }

    // Auto-probe provider models in background to pre-populate dropdown
    void aiProvidersApi
      .fetchModels({
        provider: activeProvider.id,
        refresh: false,
      })
      .then((res) => {
        if (res.models && res.models.length > 0) {
          setDiscoveredModels(res.models)
          const def = res.models.find((m) => m.is_default) || res.models[0]
          if (def) {
            setCustomModel(def.id)
            if (def.input_token_limit && def.input_token_limit > 0) {
              setContextLimit(String(def.input_token_limit))
            }
          }
        }
      })
      .catch(() => {})
  }, [selectedId, activeProvider])

  // Reset when dialog opens
  useEffect(() => {
    if (open) {
      setError('')
      setSuccess('')
      setBusy(false)
      setCliStep('init')
      setGoogleStep('init')
    }
  }, [open])

  // --- Handlers: Google OAuth ---
  const handleStartGoogleAuth = async () => {
    setBusy(true)
    setError('')
    try {
      const cleanLabel = googleLabel.trim() || 'Google Akun'
      const acc = await accountsApi.create(cleanLabel)
      setGoogleAccountId(acc.id)
      const session = await accountsApi.login(acc.id)
      if (!session.auth_url || !session.session_id) {
        throw new Error(t('Gagal mendapatkan tautan otorisasi Google. Coba lagi.'))
      }
      setGoogleSessionId(session.session_id)
      setGoogleAuthUrl(session.auth_url)
      setGoogleStep('code')
      window.open(session.auth_url, '_blank', 'noopener,noreferrer')
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal memulai otorisasi Google.'))
    } finally {
      setBusy(false)
    }
  }

  const handleVerifyGoogleCode = async () => {
    if (!googleCode.trim()) {
      setError(t('Masukkan kode otorisasi dari Google.'))
      return
    }
    setBusy(true)
    setError('')
    try {
      try {
        await accountsApi.code(googleAccountId, googleSessionId, googleCode.trim(), true)
      } catch (err) {
        if (err instanceof Error && /setup is (not complete|incomplete)/i.test(err.message)) {
          await accountsApi.finishSetup(googleAccountId)
        } else {
          throw err
        }
      }
      setSuccess(t('Google Antigravity berhasil terhubung ke Brain Cluster!'))
      setTimeout(() => {
        onBrainAdded()
        onClose()
      }, 1000)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal memverifikasi kode otorisasi Google.'))
    } finally {
      setBusy(false)
    }
  }

  // --- Handlers: CLI Official Subscription ---
  const handleStartCliAuth = async () => {
    setBusy(true)
    setError('')
    const harness = activeProvider.harness
    const name = cliLabel.trim() || activeProvider.name
    const handle = cliHandle.trim() || activeProvider.defaultHandle

    try {
      if (cliAuthMethod === 'token' || cliToken.trim()) {
        await agentsApi.connectCLI({
          name,
          handle,
          provider: harness,
          auth_type: 'subscription',
          session_token: cliToken.trim(),
        })
        setSuccess(t(`${name} berhasil terhubung ke Brain Cluster!`))
        setTimeout(() => {
          onBrainAdded()
          onClose()
        }, 1000)
        return
      }

      // Browser login flow
      const session = await agentsApi.authInit(harness)
      if (!session.session_id) throw new Error(t(`Sesi login ${name} tidak tersedia.`))
      setCliSessionId(session.session_id)
      setCliAuthUrl(session.auth_url || '')
      setCliStep('browser')
      if (session.auth_url) {
        window.open(session.auth_url, '_blank', 'noopener,noreferrer')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t(`Gagal memulai login ${activeProvider.name}.`))
    } finally {
      setBusy(false)
    }
  }

  const handleFinishCliBrowser = async () => {
    setBusy(true)
    setError('')
    const harness = activeProvider.harness
    const name = cliLabel.trim() || activeProvider.name
    const handle = cliHandle.trim() || activeProvider.defaultHandle

    try {
      const res = await agentsApi.authCode(cliSessionId, 'browser-completed')
      if (!res.authenticated && !res.verified) {
        throw new Error(res.message || t(`Login ${name} belum selesai di browser.`))
      }
      await agentsApi.connectCLI({
        name,
        handle,
        provider: harness,
        auth_type: 'subscription',
      })
      setSuccess(t(`${name} berhasil terhubung ke Brain Cluster!`))
      setTimeout(() => {
        onBrainAdded()
        onClose()
      }, 1000)
    } catch (err) {
      setError(err instanceof Error ? err.message : t(`Gagal memverifikasi login ${activeProvider.name}.`))
    } finally {
      setBusy(false)
    }
  }

  // --- Handlers: BYOK Save ---
  const handleSaveBYOK = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!apiKey.trim() && activeProvider.id !== 'ollama') {
      setError(t('Kunci API tidak boleh kosong.'))
      return
    }

    setBusy(true)
    setError('')
    try {
      const parsedLimit = parseInt(contextLimit, 10) || 128000
      const provId = activeProvider.id === 'google' ? 'google' : activeProvider.id

      // 1. Save BYOK config to AIProviderStore
      const saveRes = await aiProvidersApi.save({
        provider: provId,
        id: provId,
        mode: 'byok',
        api_key: apiKey.trim(),
        base_url: baseUrl.trim() || undefined,
        default_model: customModel.trim() || activeProvider.defaultModel || undefined,
        enabled: true,
      })

      if (saveRes.status !== 'success') {
        throw new Error(t('Gagal menyimpan konfigurasi kunci API.'))
      }

      // 2. Connect CLI agent harness directly with this custom API key & endpoint
      const harness =
        activeProvider.harness === 'google'
          ? 'agy'
          : activeProvider.harness

      const agentName = customAgentName.trim() || activeProvider.name
      const agentHandle = customHandle.trim() || activeProvider.defaultHandle

      await agentsApi.connectCLI({
        name: agentName,
        handle: agentHandle,
        provider: harness,
        auth_type: 'byok',
        custom_api_key: apiKey.trim(),
        base_url: baseUrl.trim() || undefined,
        custom_model: customModel.trim() || activeProvider.defaultModel || undefined,
        context_limit: parsedLimit,
      })

      // 3. Optional quick health test of the endpoint
      try {
        const testRes = await aiProvidersApi.test({
          provider: provId,
          id: provId,
          api_key: apiKey.trim(),
          base_url: baseUrl.trim() || undefined,
          model: customModel.trim() || activeProvider.defaultModel || undefined,
        })
        if (!testRes.valid) {
          setSuccess(
            t('Otak tersimpan. Catatan verifikasi endpoint: ') +
              (testRes.error_message || t('Koneksi belum valid.'))
          )
        } else {
          setSuccess(t(`${activeProvider.name} berhasil terhubung ke Brain Cluster!`))
        }
      } catch {
        setSuccess(t(`${activeProvider.name} berhasil terhubung ke Brain Cluster!`))
      }

      setTimeout(() => {
        onBrainAdded()
        onClose()
      }, 1000)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal menghubungkan otak AI.'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <SlideDrawer
      isOpen={open}
      onClose={() => {
        if (!busy) onClose()
      }}
      title={
        <div className="flex items-center gap-2.5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-muted text-foreground">
            <Bot className="h-4 w-4" />
          </div>
          <span>{t('Tambah Otak AI ke Cluster')}</span>
        </div>
      }
      description={t('Pilih harness coding, tentukan metode koneksi, dan pilih model aktif.')}
      size="lg"
      data-testid="add-brain-drawer"
    >
      <div className="space-y-5 pt-1 pb-6" data-testid="add-brain-content">
        {/* Status alerts */}
        {error && (
          <div role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive">
            {error}
          </div>
        )}
        {success && (
          <div role="status" className="rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-3 text-xs text-emerald-600 dark:text-emerald-400">
            {success}
          </div>
        )}

        {/* 1. Pilih Harness Coding (Zero Visual Noise - Hanya Logo + Nama) */}
        <section className="space-y-2">
          <label className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
            {t('1. Pilih Harness Coding')}
          </label>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
            {PROVIDER_OPTIONS.map((provider) => {
              const isSelected = provider.id === selectedId
              return (
                <button
                  key={provider.id}
                  type="button"
                  onClick={() => {
                    setSelectedId(provider.id)
                  }}
                  className={`w-full min-w-0 flex items-center gap-2.5 rounded-xl border px-3 py-2 text-left transition-all h-12 ${
                    isSelected
                      ? 'border-2 border-foreground dark:border-primary bg-background shadow-xs ring-1 ring-foreground/10 dark:ring-primary/20'
                      : 'border-border/40 bg-muted/15 text-muted-foreground hover:bg-muted/35 hover:border-border/80 hover:text-foreground'
                  }`}
                >
                  <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-background shadow-xs border border-border/30">
                    <BrandLogo id={provider.id} className="h-4 w-4" />
                  </span>
                  <span className="truncate font-semibold text-xs text-foreground">
                    {provider.name}
                  </span>
                </button>
              )
            })}
          </div>
        </section>

        {/* 2. Metode Koneksi */}
        <section className="space-y-2">
          <label className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
            {t('2. Metode Koneksi')}
          </label>
          <div className="grid grid-cols-2 gap-1 rounded-xl bg-muted/30 p-1 border border-border/40 text-xs">
            <button
              type="button"
              className={`flex items-center justify-center gap-1.5 min-h-10 rounded-lg transition-all ${
                authMethod === 'subscription'
                  ? 'bg-background text-foreground shadow-xs font-semibold'
                  : 'text-muted-foreground hover:text-foreground font-medium'
              }`}
              onClick={() => setAuthMethod('subscription')}
            >
              <Sparkles className="h-3.5 w-3.5 text-amber-500 shrink-0" />
              <span>
                {activeProvider.harness === 'google'
                  ? t('Akun Google Resmi')
                  : t('Langganan Resmi')}
              </span>
              <span className="rounded-full px-1.5 py-0.2 text-[10px] font-medium bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                {t('Hemat')}
              </span>
            </button>
            <button
              type="button"
              className={`flex items-center justify-center gap-1.5 min-h-10 rounded-lg transition-all ${
                authMethod === 'byok'
                  ? 'bg-background text-foreground shadow-xs font-semibold'
                  : 'text-muted-foreground hover:text-foreground font-medium'
              }`}
              onClick={() => setAuthMethod('byok')}
            >
              <Key className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <span>
                {activeProvider.harness === 'google'
                  ? t('Gemini API Key')
                  : t('Kunci API (BYOK)')}
              </span>
            </button>
          </div>
        </section>

        {/* 3. Autentikasi & Model Selection */}
        {authMethod === 'subscription' ? (
          /* --- Flow A: Subscription / Official CLI Login --- */
          <div className="space-y-4 pt-1">
            <label className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase block">
              {t('3. Hubungkan Akun')}
            </label>

            {activeProvider.harness === 'google' ? (
              /* Google OAuth Flow */
              googleStep === 'init' ? (
                <div className="space-y-3 rounded-xl border border-border/50 bg-muted/15 p-4">
                  <p className="text-xs text-muted-foreground leading-relaxed">
                    {t('Masuk dengan Akun Google resmi Anda untuk mengaktifkan Antigravity.')}
                  </p>
                  <FormField label={t('Label Akun Google')} htmlFor="google-label">
                    <Input
                      id="google-label"
                      value={googleLabel}
                      disabled={busy}
                      onChange={(e) => setGoogleLabel(e.target.value)}
                      className="h-11 rounded-xl"
                    />
                  </FormField>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => void handleStartGoogleAuth()}
                    className="action-primary flex w-full items-center justify-center gap-2 h-11 rounded-xl font-medium"
                  >
                    <ExternalLink className="h-4 w-4" />
                    <span>{busy ? t('Mempersiapkan Otorisasi…') : t('Buka Otorisasi Google di Browser')}</span>
                  </button>
                </div>
              ) : (
                <div className="space-y-3 rounded-xl border border-border/60 bg-muted/20 p-4">
                  <p className="text-xs text-muted-foreground">
                    {t('Tempel kode otorisasi dari jendela Google yang baru saja terbuka:')}
                  </p>
                  <FormField label={t('Kode Otorisasi')} htmlFor="google-code">
                    <Input
                      id="google-code"
                      placeholder="4/0A..."
                      value={googleCode}
                      disabled={busy}
                      onChange={(e) => setGoogleCode(e.target.value)}
                      className="h-11 rounded-xl font-mono text-xs"
                    />
                  </FormField>
                  <div className="flex gap-2 pt-1">
                    <button
                      type="button"
                      className="action-secondary h-11 rounded-xl"
                      disabled={busy}
                      onClick={() => setGoogleStep('init')}
                    >
                      {t('Ulangi')}
                    </button>
                    <button
                      type="button"
                      className="action-primary flex-1 h-11 rounded-xl font-medium"
                      disabled={busy || !googleCode.trim()}
                      onClick={() => void handleVerifyGoogleCode()}
                    >
                      {busy ? t('Memverifikasi…') : t('Verifikasi & Hubungkan')}
                    </button>
                  </div>
                </div>
              )
            ) : (
              /* CLI Harness Official Login Flow */
              cliAuthMethod === 'browser' ? (
                cliStep === 'init' ? (
                  <div className="space-y-3 rounded-xl border border-border/50 bg-muted/15 p-4">
                    <p className="text-xs text-muted-foreground leading-relaxed">
                      {t(`Hubungkan sesi langganan resmi ${activeProvider.name} langsung lewat browser Anda.`)}
                    </p>
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => void handleStartCliAuth()}
                      className="action-primary flex w-full items-center justify-center gap-2 h-11 rounded-xl font-medium"
                    >
                      <ExternalLink className="h-4 w-4" />
                      <span>{busy ? t('Mempersiapkan Login…') : `${t('Masuk via Browser')} ${activeProvider.name}`}</span>
                    </button>
                    <div className="pt-1 text-center">
                      <button
                        type="button"
                        onClick={() => setCliAuthMethod('token')}
                        className="text-[11px] text-muted-foreground hover:text-foreground underline"
                      >
                        {t('Atau gunakan token sesi manual')}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-3 rounded-xl border border-border/60 bg-muted/20 p-4">
                    <p className="text-xs text-muted-foreground">
                      {t('Selesaikan login di tab browser yang terbuka, lalu konfirmasi di bawah:')}
                    </p>
                    <div className="flex gap-2">
                      <button
                        type="button"
                        className="action-secondary h-11 rounded-xl"
                        disabled={busy}
                        onClick={() => setCliStep('init')}
                      >
                        {t('Batal')}
                      </button>
                      <button
                        type="button"
                        className="action-primary flex-1 h-11 rounded-xl font-medium"
                        disabled={busy}
                        onClick={() => void handleFinishCliBrowser()}
                      >
                        {busy ? t('Memeriksa Status…') : t('Saya Sudah Selesai Login')}
                      </button>
                    </div>
                  </div>
                )
              ) : (
                <div className="space-y-3 rounded-xl border border-border/50 bg-muted/15 p-4">
                  <FormField
                    label={t('Token Sesi CLI')}
                    htmlFor="cli-token"
                    helperText={t('Tempel token sesi kredensial resmi dari terminal Anda.')}
                  >
                    <div className="relative">
                      <Input
                        id="cli-token"
                        type={cliShowToken ? 'text' : 'password'}
                        placeholder="eyJhbGciOi..."
                        value={cliToken}
                        disabled={busy}
                        onChange={(e) => setCliToken(e.target.value)}
                        className="h-11 rounded-xl font-mono text-xs pr-10"
                      />
                      <button
                        type="button"
                        className="absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground hover:text-foreground"
                        onClick={() => setCliShowToken(!cliShowToken)}
                      >
                        {cliShowToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                      </button>
                    </div>
                  </FormField>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      className="action-secondary h-11 rounded-xl"
                      disabled={busy}
                      onClick={() => setCliAuthMethod('browser')}
                    >
                      {t('Batal')}
                    </button>
                    <button
                      type="button"
                      disabled={busy || !cliToken.trim()}
                      onClick={() => void handleStartCliAuth()}
                      className="action-primary flex-1 flex items-center justify-center gap-2 h-11 rounded-xl font-medium"
                    >
                      <Terminal className="h-4 w-4" />
                      <span>{busy ? t('Menghubungkan…') : t('Hubungkan dengan Token')}</span>
                    </button>
                  </div>
                </div>
              )
            )}
          </div>
        ) : (
          /* --- Flow B: BYOK / Kunci API --- */
          <form onSubmit={handleSaveBYOK} className="space-y-4 pt-1">
            <label className="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase block">
              {t('3. Kunci API & Model')}
            </label>

            {/* Kunci API */}
            <FormField
              label={t('Kunci API (API Key)')}
              accessory={
                activeProvider.keyHelpUrl ? (
                  <a
                    href={activeProvider.keyHelpUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-[11px] text-primary hover:underline font-normal"
                  >
                    <span>{t('Dapatkan Kunci')}</span>
                    <ExternalLink className="h-3 w-3" />
                  </a>
                ) : undefined
              }
              htmlFor="custom-api-key"
              helperText={t('Kunci API disimpan aman dan hanya digunakan saat eksekusi tugas.')}
            >
              <div className="relative">
                <Input
                  id="custom-api-key"
                  type={showKey ? 'text' : 'password'}
                  placeholder={activeProvider.keyPlaceholder}
                  value={apiKey}
                  disabled={busy}
                  onChange={(e) => setApiKey(e.target.value)}
                  className="h-11 rounded-xl font-mono text-xs pr-10"
                />
                <button
                  type="button"
                  className="absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground hover:text-foreground"
                  onClick={() => setShowKey(!showKey)}
                >
                  {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </FormField>

            {/* Base URL (Tampil langsung untuk OpenCode, atau jika diatur) */}
            {(activeProvider.harness === 'opencode' || baseUrl) && (
              <FormField
                label={t('Base URL Endpoint')}
                htmlFor="custom-base-url"
                helperText={t('Endpoint penyedia model AI (OpenRouter, DeepSeek, Ollama, proxy, dll).')}
              >
                <Input
                  id="custom-base-url"
                  type="url"
                  placeholder={activeProvider.defaultBaseUrl || 'https://api.openai.com/v1'}
                  value={baseUrl}
                  disabled={busy}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  className="h-11 rounded-xl font-mono text-xs"
                />
              </FormField>
            )}

            {/* 4. Model Aktif (Polling Live - Default terpilih otomatis) */}
            <FormField
              label={t('Model Aktif')}
              accessory={
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    disabled={fetchingModels}
                    onClick={() => void handleFetchLiveModels()}
                    className="inline-flex items-center gap-1 text-[11px] text-primary hover:underline font-normal disabled:opacity-50"
                  >
                    <RefreshCw className={`h-3 w-3 ${fetchingModels ? 'animate-spin' : ''}`} />
                    <span>{fetchingModels ? t('Memeriksa…') : t('Ambil Model Terbaru')}</span>
                  </button>
                  {discoveredModels.length > 0 && (
                    <button
                      type="button"
                      onClick={() => setModelMode(modelMode === 'select' ? 'manual' : 'select')}
                      className="text-[11px] text-muted-foreground hover:text-foreground"
                    >
                      {modelMode === 'select' ? t('Ketik manual') : t('Pilih daftar')}
                    </button>
                  )}
                </div>
              }
              htmlFor="custom-model"
              helperText={t('Model default terpilih otomatis. Anda tidak perlu mengetik nama model.')}
            >
              {modelMode === 'select' && discoveredModels.length > 0 ? (
                <select
                  id="custom-model"
                  className="field-control h-11 w-full rounded-xl text-xs font-medium"
                  value={customModel}
                  disabled={busy}
                  onChange={(e) => {
                    const sel = e.target.value
                    setCustomModel(sel)
                    const found = discoveredModels.find((m) => m.id === sel)
                    if (found && found.input_token_limit && found.input_token_limit > 0) {
                      setContextLimit(String(found.input_token_limit))
                    }
                  }}
                >
                  {discoveredModels.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.display_name || m.name || m.id}{' '}
                      {m.input_token_limit ? `(${Math.round(m.input_token_limit / 1000)}k context)` : ''}
                      {m.is_default ? ' ★' : ''}
                    </option>
                  ))}
                </select>
              ) : (
                <Input
                  id="custom-model"
                  placeholder={activeProvider.defaultModel}
                  value={customModel}
                  disabled={busy}
                  onChange={(e) => setCustomModel(e.target.value)}
                  className="h-11 rounded-xl font-mono text-xs"
                />
              )}
            </FormField>

            {/* Pengaturan Lanjutan (Opsional - Terlipat rapi) */}
            <div className="pt-1">
              <button
                type="button"
                onClick={() => setShowAdvanced(!showAdvanced)}
                className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors font-medium py-1"
              >
                <ChevronDown className={`h-3.5 w-3.5 transition-transform duration-200 ${showAdvanced ? 'rotate-180' : ''}`} />
                <span>{t('Pengaturan Lanjutan (Opsional)')}</span>
              </button>

              {showAdvanced && (
                <div className="space-y-3 pt-3 border-t border-border/30 mt-2">
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <FormField label={t('Nama Asisten')} htmlFor="custom-agent-name">
                      <Input
                        id="custom-agent-name"
                        value={customAgentName}
                        disabled={busy}
                        onChange={(e) => setCustomAgentName(e.target.value)}
                        className="h-11 rounded-xl"
                      />
                    </FormField>
                    <FormField label={t('Handle / Mention')} htmlFor="custom-handle">
                      <Input
                        id="custom-handle"
                        value={customHandle}
                        disabled={busy}
                        onChange={(e) => setCustomHandle(e.target.value)}
                        className="h-11 rounded-xl font-mono"
                      />
                    </FormField>
                  </div>

                  <FormField label={t('Batas Konteks (Token)')} htmlFor="custom-context-limit">
                    <Input
                      id="custom-context-limit"
                      type="number"
                      placeholder="128000"
                      value={contextLimit}
                      disabled={busy}
                      onChange={(e) => setContextLimit(e.target.value)}
                      className="h-11 rounded-xl font-mono"
                    />
                  </FormField>

                  {activeProvider.harness !== 'opencode' && !baseUrl && (
                    <FormField label={t('Custom Base URL')} htmlFor="adv-base-url">
                      <Input
                        id="adv-base-url"
                        type="url"
                        placeholder="https://api.openai.com/v1"
                        value={baseUrl}
                        disabled={busy}
                        onChange={(e) => setBaseUrl(e.target.value)}
                        className="h-11 rounded-xl font-mono text-xs"
                      />
                    </FormField>
                  )}
                </div>
              )}
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              disabled={busy || !apiKey.trim()}
              className="action-primary flex w-full items-center justify-center gap-2 h-11 rounded-xl font-medium mt-4"
            >
              {busy ? (
                <>
                  <RefreshCw className="h-4 w-4 animate-spin" />
                  <span>{t('Menyimpan & Menghubungkan…')}</span>
                </>
              ) : (
                <>
                  <Check className="h-4 w-4" />
                  <span>{t('Simpan Otak AI')}</span>
                </>
              )}
            </button>
          </form>
        )}
      </div>
    </SlideDrawer>
  )
}
