import React, { useState } from 'react'
import {
  X,
  Cpu,
  Key,
  Globe,
  Check,
  AlertCircle,
  Eye,
  EyeOff,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { aiProvidersApi, type AIProviderDTO } from '../../api/aiProviders'

interface AddBrainModalProps {
  open: boolean
  onClose: () => void
  onBrainAdded: () => void
  existingProviders: AIProviderDTO[]
}

const PROVIDER_OPTIONS = [
  {
    id: 'openai',
    name: 'OpenAI',
    modelsHint: 'GPT-4o, GPT-4o-mini',
    badge: 'Teks & Gambar',
    badgeColor: 'text-emerald-500 bg-emerald-500/10 border-emerald-500/20',
    keyPlaceholder: 'sk-proj-...',
    keyHelpUrl: 'https://platform.openai.com/api-keys',
  },
  {
    id: 'anthropic',
    name: 'Anthropic (Claude)',
    modelsHint: 'Claude 3.7 Sonnet, Claude 3.5 Haiku',
    badge: 'Penalaran Mendalam & Gambar',
    badgeColor: 'text-amber-500 bg-amber-500/10 border-amber-500/20',
    keyPlaceholder: 'sk-ant-...',
    keyHelpUrl: 'https://console.anthropic.com/settings/keys',
  },
  {
    id: 'google',
    name: 'Google (Gemini)',
    modelsHint: 'Gemini 2.5 Pro, Gemini 2.0 Flash',
    badge: 'Konteks Luas & Gambar',
    badgeColor: 'text-blue-500 bg-blue-500/10 border-blue-500/20',
    keyPlaceholder: 'AIzaSy...',
    keyHelpUrl: 'https://aistudio.google.com/app/apikey',
  },
  {
    id: 'deepseek',
    name: 'DeepSeek',
    modelsHint: 'DeepSeek-Chat, DeepSeek-R1',
    badge: 'Hemat Biaya & Penalaran',
    badgeColor: 'text-indigo-500 bg-indigo-500/10 border-indigo-500/20',
    keyPlaceholder: 'sk-...',
    keyHelpUrl: 'https://platform.deepseek.com/api_keys',
  },
  {
    id: 'groq',
    name: 'Groq',
    modelsHint: 'Llama 3.3 70B, Mixtral',
    badge: 'Kecepatan Tinggi (Ultra-Fast)',
    badgeColor: 'text-orange-500 bg-orange-500/10 border-orange-500/20',
    keyPlaceholder: 'gsk_...',
    keyHelpUrl: 'https://console.groq.com/keys',
  },
  {
    id: 'openrouter',
    name: 'OpenRouter',
    modelsHint: 'DeepSeek, Claude, Llama via satu akun',
    badge: 'Agregator Model Fleksibel',
    badgeColor: 'text-purple-500 bg-purple-500/10 border-purple-500/20',
    keyPlaceholder: 'sk-or-...',
    keyHelpUrl: 'https://openrouter.ai/keys',
  },
  {
    id: 'grok',
    name: 'xAI (Grok)',
    modelsHint: 'Grok 2 Latest',
    badge: 'Pengetahuan Terkini',
    badgeColor: 'text-teal-500 bg-teal-500/10 border-teal-500/20',
    keyPlaceholder: 'xai-...',
    keyHelpUrl: 'https://console.x.ai',
  },
  {
    id: 'ollama',
    name: 'Ollama (Lokal / Privat)',
    modelsHint: 'Llama 3, Mistral, Qwen (Offline)',
    badge: '100% Privat & Tanpa Kuota',
    badgeColor: 'text-rose-500 bg-rose-500/10 border-rose-500/20',
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

  const [selectedProvider, setSelectedProvider] = useState<string>('openai')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  if (!open) return null

  const selectedDef = PROVIDER_OPTIONS.find((p) => p.id === selectedProvider) || PROVIDER_OPTIONS[0]
  const isAlreadyConfigured = existingProviders.some(
    (p) => p.id === selectedProvider && p.has_api_key
  )

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (busy) return
    setError('')
    setSuccess('')

    if (!selectedDef.isLocal && !apiKey.trim() && !isAlreadyConfigured) {
      setError(t('Masukkan API Key yang valid untuk penyedia ini.'))
      return
    }

    setBusy(true)
    try {
      // 1. Simpan konfigurasi
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

      // 2. Test koneksi singkat
      const testRes = await aiProvidersApi.test({
        provider: selectedProvider,
        id: selectedProvider,
        api_key: apiKey.trim(),
        base_url: baseUrl.trim() || undefined,
      })

      if (!testRes.valid) {
        setSuccess(
          t('Akun tersimpan, namun peringatan tes: ') + (testRes.error_message || t('Koneksi tidak valid.'))
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
      aria-label={t('Hubungkan Otak AI Baru')}
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/80 backdrop-blur-sm animate-in fade-in-0 duration-200"
    >
      <div className="relative w-full max-w-lg rounded-2xl border border-border bg-card p-6 shadow-xl space-y-5 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between pb-3 border-b border-border/40">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Cpu className="h-5 w-5" />
            </div>
            <div>
              <h3 className="font-bold text-base text-foreground">{t('Tambah Otak AI')}</h3>
              <p className="text-xs text-muted-foreground">
                {t('Hubungkan akun AI ke cluster server komputasi Cuan.')}
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

        {error && (
          <div role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {success && (
          <div role="status" className="rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-3 text-xs text-emerald-600 dark:text-emerald-400 flex items-center gap-2">
            <Check className="h-4 w-4 shrink-0" />
            <span>{success}</span>
          </div>
        )}

        <form onSubmit={handleSave} className="space-y-4">
          {/* Provider Selector Grid */}
          <div className="space-y-2">
            <label className="text-xs font-semibold text-foreground">
              {t('Pilih Penyedia AI (Provider)')}
            </label>
            <div className="grid grid-cols-2 gap-2 max-h-48 overflow-y-auto p-1">
              {PROVIDER_OPTIONS.map((opt) => {
                const isSelected = selectedProvider === opt.id
                const isConfigured = existingProviders.some(
                  (p) => p.id === opt.id && p.has_api_key
                )
                return (
                  <button
                    key={opt.id}
                    type="button"
                    onClick={() => {
                      setSelectedProvider(opt.id)
                      setApiKey('')
                      setError('')
                      setSuccess('')
                      if (opt.id === 'ollama') {
                        setBaseUrl('http://localhost:11434')
                      } else {
                        setBaseUrl('')
                      }
                    }}
                    className={`flex flex-col items-start p-3 rounded-xl border text-left transition-all ${
                      isSelected
                        ? 'border-primary bg-primary/5 ring-1 ring-primary'
                        : 'border-border/60 hover:bg-muted/40'
                    }`}
                  >
                    <div className="flex items-center justify-between w-full">
                      <span className="font-semibold text-xs text-foreground">{opt.name}</span>
                      {isConfigured && (
                        <span className="text-[9px] px-1.5 py-0.2 rounded-full bg-emerald-500/15 text-emerald-600 font-mono">
                          {t('Terhubung')}
                        </span>
                      )}
                    </div>
                    <span className="text-[10px] text-muted-foreground mt-0.5 line-clamp-1">
                      {opt.modelsHint}
                    </span>
                    <span
                      className={`mt-2 inline-flex items-center px-1.5 py-0.5 text-[9px] font-medium rounded-full border ${opt.badgeColor}`}
                    >
                      {opt.badge}
                    </span>
                  </button>
                )
              })}
            </div>
          </div>

          {/* API Key Input */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label htmlFor="brain-api-key" className="text-xs font-semibold text-foreground flex items-center gap-1.5">
                <Key className="h-3.5 w-3.5 text-muted-foreground" />
                <span>{selectedDef.isLocal ? t('Kredensial / Token (Opsional)') : t('Kunci API (API Key)')}</span>
              </label>
              {selectedDef.keyHelpUrl && (
                <a
                  href={selectedDef.keyHelpUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[11px] text-primary hover:underline"
                >
                  {t('Dapatkan API Key')} ↗
                </a>
              )}
            </div>

            <div className="relative">
              <input
                id="brain-api-key"
                type={showKey ? 'text' : 'password'}
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={
                  isAlreadyConfigured
                    ? t('(Kunci terkonfigurasi. Kosongkan untuk mempertahankan)')
                    : selectedDef.keyPlaceholder
                }
                className="field-control w-full pr-10 text-xs sm:text-sm font-mono rounded-xl"
              />
              <button
                type="button"
                onClick={() => setShowKey(!showKey)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                aria-label={showKey ? t('Sembunyikan') : t('Tampilkan')}
              >
                {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
            <p className="text-[11px] text-muted-foreground">
              {t('Kunci API dienkripsi secara aman dan hanya digunakan untuk memproses tugas Anda.')}
            </p>
          </div>

          {/* Optional Base URL for Ollama / Custom endpoints */}
          {(selectedProvider === 'ollama' || selectedProvider === 'openrouter') && (
            <div className="space-y-1.5">
              <label htmlFor="brain-base-url" className="text-xs font-semibold text-foreground flex items-center gap-1.5">
                <Globe className="h-3.5 w-3.5 text-muted-foreground" />
                <span>{t('Alamat Server / Base URL')}</span>
              </label>
              <input
                id="brain-base-url"
                type="url"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder={selectedProvider === 'ollama' ? 'http://localhost:11434' : 'https://openrouter.ai/api/v1'}
                className="field-control w-full text-xs font-mono rounded-xl"
              />
            </div>
          )}

          {/* Dialog Action Buttons */}
          <div className="flex items-center justify-end gap-2 pt-3 border-t border-border/40">
            <button
              type="button"
              onClick={onClose}
              disabled={busy}
              className="action-secondary min-h-11 px-4 text-xs rounded-xl"
            >
              {t('Batal')}
            </button>
            <button
              type="submit"
              disabled={busy}
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-5 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              <Cpu className="h-4 w-4" />
              <span>{busy ? t('Menghubungkan…') : t('Simpan & Aktifkan Otak')}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
