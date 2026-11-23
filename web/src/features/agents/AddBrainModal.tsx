import React, { useState } from 'react'
import {
  X,
  Bot,
  Key,
  Globe,
  Check,
  AlertCircle,
  Eye,
  EyeOff,
  ExternalLink,
  RefreshCw,
  Sparkles,
  Shield,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { aiProvidersApi, type AIProviderDTO } from '../../api/aiProviders'
import { accountsApi } from '../../api/accounts'
import { agentsApi } from '../../api/agents'
import { FormField, Input } from '../../components/ui/forms'

interface AddBrainModalProps {
  open: boolean
  onClose: () => void
  onBrainAdded: () => void
  existingProviders: AIProviderDTO[]
}

const BYOK_OPTIONS = [
  {
    id: 'openai',
    name: 'OpenAI',
    modelsHint: 'GPT-4o, GPT-4o-mini',
    badge: 'Teks & Gambar',
    keyPlaceholder: 'sk-proj-...',
    keyHelpUrl: 'https://platform.openai.com/api-keys',
  },
  {
    id: 'anthropic',
    name: 'Anthropic (Claude)',
    modelsHint: 'Claude 3.7 Sonnet, Claude 3.5 Haiku',
    badge: 'Penalaran Mendalam & Gambar',
    keyPlaceholder: 'sk-ant-...',
    keyHelpUrl: 'https://console.anthropic.com/settings/keys',
  },
  {
    id: 'google',
    name: 'Google (Gemini API)',
    modelsHint: 'Gemini 2.5 Pro, Gemini 2.0 Flash',
    badge: 'Konteks Luas & Gambar',
    keyPlaceholder: 'AIzaSy...',
    keyHelpUrl: 'https://aistudio.google.com/app/apikey',
  },
  {
    id: 'deepseek',
    name: 'DeepSeek',
    modelsHint: 'DeepSeek-Chat, DeepSeek-R1',
    badge: 'Hemat Biaya & Penalaran',
    keyPlaceholder: 'sk-...',
    keyHelpUrl: 'https://platform.deepseek.com/api_keys',
  },
  {
    id: 'groq',
    name: 'Groq',
    modelsHint: 'Llama 3.3 70B, Mixtral',
    badge: 'Kecepatan Tinggi (Ultra-Fast)',
    keyPlaceholder: 'gsk_...',
    keyHelpUrl: 'https://console.groq.com/keys',
  },
  {
    id: 'openrouter',
    name: 'OpenRouter',
    modelsHint: 'DeepSeek, Claude, Llama via satu akun',
    badge: 'Agregator Model Fleksibel',
    keyPlaceholder: 'sk-or-...',
    keyHelpUrl: 'https://openrouter.ai/keys',
  },
  {
    id: 'grok',
    name: 'xAI (Grok)',
    modelsHint: 'Grok 2 Latest',
    badge: 'Pengetahuan Terkini',
    keyPlaceholder: 'xai-...',
    keyHelpUrl: 'https://console.x.ai',
  },
  {
    id: 'ollama',
    name: 'Ollama (Lokal / Privat)',
    modelsHint: 'Llama 3, Mistral, Qwen (Offline)',
    badge: '100% Privat & Tanpa Kuota',
    keyPlaceholder: 'Opsional jika ada bearer auth',
    isLocal: true,
  },
]

export function AddBrainModal({
  open,
  onClose,
  onBrainAdded,
  existingProviders,
}: AddBrainModalProps) {
  const { t } = useLanguage()

  // Tab mode: 'subscription' (Zero cost / existing subscription) vs 'byok' (API key)
  const [tabMode, setTabMode] = useState<'subscription' | 'byok'>('subscription')
  const [subProvider, setSubProvider] = useState<'google' | 'commandcode'>('google')

  // Google OAuth flow states
  const [googleLabel, setGoogleLabel] = useState('Google Akun Baru')
  const [googleStep, setGoogleStep] = useState<'init' | 'code'>('init')
  const [googleAuthUrl, setGoogleAuthUrl] = useState('')
  const [googleSessionId, setGoogleSessionId] = useState('')
  const [googleAccountId, setGoogleAccountId] = useState('')
  const [googleCode, setGoogleCode] = useState('')

  // CommandCode subscription states
  const [ccLabel] = useState('CommandCode')
  const [ccStep, setCcStep] = useState<'init' | 'browser'>('init')
  const [ccSessionId, setCcSessionId] = useState('')
  const [ccAuthUrl, setCcAuthUrl] = useState('')
  const [ccToken, setCcToken] = useState('')

  // BYOK flow states
  const [selectedProvider, setSelectedProvider] = useState<string>('deepseek')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [showKey, setShowKey] = useState(false)

  // Status & feedback
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  if (!open) return null

  const selectedDef = BYOK_OPTIONS.find((p) => p.id === selectedProvider) || BYOK_OPTIONS[0]
  const isAlreadyConfigured = existingProviders.some(
    (p) => p.id === selectedProvider && p.has_api_key
  )

  // Handle Google OAuth initialization
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

  // Handle Google OAuth code submission
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
      setSuccess(t('Otak Google (Antigravity) berhasil terhubung ke Brain Cluster!'))
      setTimeout(() => {
        onBrainAdded()
        onClose()
      }, 1200)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal memverifikasi kode otorisasi Google.'))
    } finally {
      setBusy(false)
    }
  }

  // Handle CommandCode initialization
  const handleStartCommandCode = async () => {
    setBusy(true)
    setError('')
    try {
      if (ccToken.trim()) {
        // Direct subscription key connection
        await agentsApi.connectCLI({
          name: ccLabel.trim() || 'CommandCode',
          handle: '@cc',
          provider: 'commandcode',
          auth_type: 'subscription',
          session_token: ccToken.trim(),
        })
        setSuccess(t('Akun CommandCode berhasil terhubung ke Brain Cluster!'))
        setTimeout(() => {
          onBrainAdded()
          onClose()
        }, 1200)
        return
      }

      // Browser login flow
      const session = await agentsApi.authInit('commandcode')
      if (!session.session_id) throw new Error(t('Sesi login CommandCode tidak tersedia.'))
      setCcSessionId(session.session_id)
      setCcAuthUrl(session.auth_url || '')
      setCcStep('browser')
      if (session.auth_url) {
        window.open(session.auth_url, '_blank', 'noopener,noreferrer')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal memulai login CommandCode.'))
    } finally {
      setBusy(false)
    }
  }

  const handleFinishCommandCodeBrowser = async () => {
    setBusy(true)
    setError('')
    try {
      const res = await agentsApi.authCode(ccSessionId, 'browser-completed')
      if (!res.authenticated && !res.verified) {
        throw new Error(res.message || t('Login CommandCode belum selesai.'))
      }
      setSuccess(t('Akun CommandCode berhasil terhubung ke Brain Cluster!'))
      setTimeout(() => {
        onBrainAdded()
        onClose()
      }, 1200)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal memverifikasi login CommandCode.'))
    } finally {
      setBusy(false)
    }
  }

  // Handle BYOK save
  const handleSaveBYOK = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!apiKey.trim() && selectedProvider !== 'ollama') {
      setError(t('Kunci API tidak boleh kosong.'))
      return
    }

    setBusy(true)
    setError('')
    try {
      const saveRes = await aiProvidersApi.save({
        provider: selectedProvider,
        id: selectedProvider,
        mode: 'byok',
        api_key: apiKey.trim(),
        base_url: baseUrl.trim() || undefined,
        enabled: true,
      })

      if (saveRes.status !== 'success') {
        throw new Error(t('Gagal menyimpan konfigurasi akun AI.'))
      }

      const testRes = await aiProvidersApi.test({
        provider: selectedProvider,
        id: selectedProvider,
        api_key: apiKey.trim(),
        base_url: baseUrl.trim() || undefined,
      })

      if (!testRes.valid) {
        setSuccess(
          t('Akun tersimpan, namun peringatan tes: ') +
            (testRes.error_message || t('Koneksi tidak valid.'))
        )
      } else {
        setSuccess(t('Otak AI berhasil terhubung ke Brain Cluster!'))
      }

      setTimeout(() => {
        onBrainAdded()
        onClose()
      }, 1200)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Gagal menghubungkan otak AI.'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/80 backdrop-blur-sm overflow-y-auto"
    >
      <div className="relative w-full max-w-xl rounded-2xl border border-border/80 bg-card p-6 shadow-2xl space-y-5 my-8">
        {/* Header */}
        <div className="flex items-center justify-between pb-3 border-b border-border/40">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Bot className="h-5 w-5" />
            </div>
            <div>
              <h3 className="font-bold text-base text-foreground">
                {t('Tambah Otak AI ke Cluster')}
              </h3>
              <p className="text-xs text-muted-foreground">
                {t('Hubungkan akun AI melalui langganan resmi atau kunci API pribadi.')}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Tab Switcher */}
        <div className="grid grid-cols-2 p-1 bg-muted/60 rounded-xl border border-border/40 text-xs font-semibold">
          <button
            type="button"
            onClick={() => {
              setTabMode('subscription')
              setError('')
              setSuccess('')
            }}
            className={`py-2 px-3 rounded-lg flex items-center justify-center gap-1.5 transition-all ${
              tabMode === 'subscription'
                ? 'bg-card text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <Sparkles className="h-3.5 w-3.5 text-amber-500" />
            <span>{t('Langganan Resmi')}</span>
            <span className="text-[10px] px-1.5 py-0.2 rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 font-normal">
              {t('Hemat')}
            </span>
          </button>
          <button
            type="button"
            onClick={() => {
              setTabMode('byok')
              setError('')
              setSuccess('')
            }}
            className={`py-2 px-3 rounded-lg flex items-center justify-center gap-1.5 transition-all ${
              tabMode === 'byok'
                ? 'bg-card text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <Key className="h-3.5 w-3.5 text-primary" />
            <span>{t('Kunci API (BYOK)')}</span>
          </button>
        </div>

        {error && (
          <div
            role="alert"
            className="rounded-xl border border-destructive/30 bg-destructive/10 p-3.5 text-xs text-destructive flex items-center gap-2"
          >
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {success && (
          <div
            role="status"
            className="rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-3.5 text-xs text-emerald-600 dark:text-emerald-400 flex items-center gap-2 font-medium"
          >
            <Check className="h-4 w-4 shrink-0" />
            <span>{success}</span>
          </div>
        )}

        {/* TAB 1: SUBSCRIPTION (LANGGANAN RESMI GOOGLE / COMMANDCODE) */}
        {tabMode === 'subscription' && (
          <div className="space-y-4">
            <div className="p-3 bg-muted/30 border border-border/40 rounded-xl text-xs text-muted-foreground space-y-1">
              <p className="font-medium text-foreground flex items-center gap-1.5">
                <Shield className="h-3.5 w-3.5 text-primary" />
                <span>{t('Gunakan Kuota Langganan yang Sudah Anda Miliki')}</span>
              </p>
              <p>
                {t(
                  'Sambungkan akun Google AI Pro/Ultra atau CommandCode. Tidak memerlukan saldo API key terpisah, kuota di-refresh otomatis oleh penyedia.'
                )}
              </p>
            </div>

            {/* Sub-provider pills */}
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => {
                  setSubProvider('google')
                  setGoogleStep('init')
                  setError('')
                }}
                className={`flex-1 p-3 rounded-xl border text-left transition-all ${
                  subProvider === 'google'
                    ? 'border-primary bg-primary/5 text-foreground'
                    : 'border-border/60 hover:bg-muted/40 text-muted-foreground'
                }`}
              >
                <div className="font-semibold text-xs text-foreground flex items-center gap-1.5">
                  <span className="font-bold text-blue-500">G</span>
                  <span>Google (Antigravity)</span>
                </div>
                <div className="text-[11px] text-muted-foreground mt-0.5">
                  Gemini 2.5 Pro, 2.0 Flash, Vision
                </div>
              </button>

              <button
                type="button"
                onClick={() => {
                  setSubProvider('commandcode')
                  setCcStep('init')
                  setError('')
                }}
                className={`flex-1 p-3 rounded-xl border text-left transition-all ${
                  subProvider === 'commandcode'
                    ? 'border-primary bg-primary/5 text-foreground'
                    : 'border-border/60 hover:bg-muted/40 text-muted-foreground'
                }`}
              >
                <div className="font-semibold text-xs text-foreground flex items-center gap-1.5">
                  <span className="font-bold text-amber-500">CC</span>
                  <span>CommandCode</span>
                </div>
                <div className="text-[11px] text-muted-foreground mt-0.5">
                  Claude 3.7 Sonnet, Hybrid Reasoning
                </div>
              </button>
            </div>

            {/* Google OAuth View */}
            {subProvider === 'google' && (
              <div className="space-y-4 pt-2">
                {googleStep === 'init' ? (
                  <div className="space-y-3">
                    <FormField label={t('Label / Nama Otak AI')}>
                      <Input
                        type="text"
                        value={googleLabel}
                        onChange={(e) => setGoogleLabel(e.target.value)}
                        placeholder="Misal: Google Akun Kerja"
                      />
                    </FormField>

                    <button
                      type="button"
                      disabled={busy}
                      onClick={handleStartGoogleAuth}
                      className="w-full inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-xs hover:bg-primary/90 transition-colors disabled:opacity-50"
                    >
                      {busy ? (
                        <RefreshCw className="h-4 w-4 animate-spin" />
                      ) : (
                        <ExternalLink className="h-4 w-4" />
                      )}
                      <span>{t('Mulai Otorisasi Masuk dengan Google')}</span>
                    </button>
                  </div>
                ) : (
                  <div className="space-y-3.5 bg-muted/20 p-4 rounded-xl border border-border/50">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-semibold text-foreground">{t('Langkah 2: Salin Kode Otorisasi')}</span>
                      {googleAuthUrl && (
                        <a
                          href={googleAuthUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex items-center gap-1 text-primary hover:underline font-medium"
                        >
                          <span>{t('Buka Link Google')}</span>
                          <ExternalLink className="h-3 w-3" />
                        </a>
                      )}
                    </div>
                    <p className="text-[11px] text-muted-foreground leading-relaxed">
                      {t(
                        'Setujui akses di tab Google yang terbuka, lalu salin kode otorisasi dan tempel di bawah ini.'
                      )}
                    </p>

                    <FormField label={t('Kode Otorisasi dari Google')}>
                      <Input
                        type="text"
                        value={googleCode}
                        onChange={(e) => setGoogleCode(e.target.value)}
                        placeholder="4/0A..."
                        mono
                      />
                    </FormField>

                    <div className="flex items-center gap-2 pt-1">
                      <button
                        type="button"
                        onClick={() => setGoogleStep('init')}
                        className="action-secondary min-h-11 px-3 text-xs rounded-xl"
                      >
                        {t('Kembali')}
                      </button>
                      <button
                        type="button"
                        disabled={busy || !googleCode.trim()}
                        onClick={handleVerifyGoogleCode}
                        className="flex-1 inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-xs hover:bg-primary/90 transition-colors disabled:opacity-50"
                      >
                        {busy ? (
                          <RefreshCw className="h-4 w-4 animate-spin" />
                        ) : (
                          <Check className="h-4 w-4" />
                        )}
                        <span>{t('Konfirmasi & Hubungkan Otak Google')}</span>
                      </button>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* CommandCode View */}
            {subProvider === 'commandcode' && (
              <div className="space-y-4 pt-2">
                <div className="space-y-3">
                  <FormField label={t('Token Sesi Langganan (Opsional jika via browser)')}>
                    <Input
                      type="password"
                      value={ccToken}
                      onChange={(e) => setCcToken(e.target.value)}
                      placeholder="Masukkan session token jika ada..."
                      mono
                    />
                  </FormField>

                  {ccStep === 'init' ? (
                    <button
                      type="button"
                      disabled={busy}
                      onClick={handleStartCommandCode}
                      className="w-full inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-xs hover:bg-primary/90 transition-colors disabled:opacity-50"
                    >
                      {busy ? (
                        <RefreshCw className="h-4 w-4 animate-spin" />
                      ) : (
                        <ExternalLink className="h-4 w-4" />
                      )}
                      <span>
                        {ccToken.trim()
                          ? t('Hubungkan dengan Token Sesi')
                          : t('Masuk dengan Browser CommandCode')}
                      </span>
                    </button>
                  ) : (
                    <div className="space-y-3 bg-muted/20 p-4 rounded-xl border border-border/50">
                      <div className="flex items-center justify-between text-xs">
                        <p className="text-xs text-foreground font-medium">
                          {t('Selesaikan login di jendela browser CommandCode yang terbuka.')}
                        </p>
                        {ccAuthUrl && (
                          <a
                            href={ccAuthUrl}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex items-center gap-1 text-primary hover:underline font-medium shrink-0 ml-2"
                          >
                            <span>{t('Buka Link')}</span>
                            <ExternalLink className="h-3 w-3" />
                          </a>
                        )}
                      </div>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={handleFinishCommandCodeBrowser}
                        className="w-full inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-xs hover:bg-primary/90 transition-colors disabled:opacity-50"
                      >
                        {busy ? (
                          <RefreshCw className="h-4 w-4 animate-spin" />
                        ) : (
                          <Check className="h-4 w-4" />
                        )}
                        <span>{t('Saya Sudah Selesai Login di Browser')}</span>
                      </button>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}

        {/* TAB 2: BYOK (KUNCI API PRIBADI) */}
        {tabMode === 'byok' && (
          <form onSubmit={handleSaveBYOK} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-foreground mb-1.5">
                {t('Pilih Penyedia AI')}
              </label>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                {BYOK_OPTIONS.map((opt) => (
                  <button
                    key={opt.id}
                    type="button"
                    onClick={() => {
                      setSelectedProvider(opt.id)
                      setApiKey('')
                      setError('')
                    }}
                    className={`p-2.5 rounded-xl border text-left transition-all ${
                      selectedProvider === opt.id
                        ? 'border-primary bg-primary/10 text-primary shadow-xs font-semibold'
                        : 'border-border/60 hover:bg-muted/40 text-muted-foreground'
                    }`}
                  >
                    <div className="text-xs truncate">{opt.name}</div>
                    <div className="text-[10px] opacity-75 truncate">{opt.badge}</div>
                  </button>
                ))}
              </div>
            </div>

            {/* Provider info box */}
            <div className="rounded-xl border border-border/50 bg-muted/30 p-3 space-y-1">
              <div className="flex items-center justify-between text-xs">
                <span className="font-semibold text-foreground">{selectedDef.name}</span>
                {selectedDef.keyHelpUrl && (
                  <a
                    href={selectedDef.keyHelpUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-[11px] text-primary hover:underline"
                  >
                    <span>{t('Ambil Kunci API')}</span>
                    <ExternalLink className="h-3 w-3" />
                  </a>
                )}
              </div>
              <p className="text-[11px] text-muted-foreground">
                {t('Model didukung')}: {selectedDef.modelsHint}
              </p>
              {isAlreadyConfigured && (
                <div className="text-[11px] text-amber-500 font-medium pt-1">
                  {t('Penyedia ini sudah aktif. Memasukkan kunci baru akan memperbarui kredensial.')}
                </div>
              )}
            </div>

            {/* API Key field */}
            <FormField label={t('Kunci API (API Key)')} htmlFor="brain-api-key">
              <div className="relative">
                <Input
                  id="brain-api-key"
                  type={showKey ? 'text' : 'password'}
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder={selectedDef.keyPlaceholder}
                  className="pr-10"
                  mono
                  autoComplete="off"
                />
                <button
                  type="button"
                  onClick={() => setShowKey(!showKey)}
                  className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground p-1"
                >
                  {showKey ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                </button>
              </div>
            </FormField>

            {/* Base URL (Optional for Ollama / OpenRouter proxy) */}
            <FormField label={t('URL Basis / Custom Endpoint (Opsional)')} htmlFor="brain-base-url">
              <Input
                id="brain-base-url"
                type="url"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder={selectedDef.isLocal ? 'http://localhost:11434' : 'https://...'}
                prefixIcon={<Globe className="h-4 w-4" />}
                mono
              />
            </FormField>

            <div className="flex items-center justify-end gap-2 pt-2 border-t border-border/40">
              <button
                type="button"
                onClick={onClose}
                className="action-secondary min-h-11 px-4 text-xs rounded-xl"
              >
                {t('Batal')}
              </button>
              <button
                type="submit"
                disabled={busy}
                className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-5 py-2.5 text-xs font-semibold text-primary-foreground shadow-xs hover:bg-primary/90 transition-colors disabled:opacity-50"
              >
                {busy ? (
                  <RefreshCw className="h-4 w-4 animate-spin" />
                ) : (
                  <Check className="h-4 w-4" />
                )}
                <span>{t('Hubungkan Kunci API')}</span>
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
