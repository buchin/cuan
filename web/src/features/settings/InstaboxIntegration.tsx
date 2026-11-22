import { useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { apiFetch } from '../../api/client'
import { useLanguage } from '../../context/LanguageContext'
import {
  CheckCircle2,
  AlertTriangle,
  RefreshCw,
  Eye,
  EyeOff,
  ArrowLeft,
  Server,
} from 'lucide-react'

export interface InstaboxSettingsDTO {
  has_api_key: boolean
  masked_api_key: string
  endpoint_url: string
  enabled: boolean
  last_validated_at?: string
  validation_status: 'untested' | 'valid' | 'invalid'
}

export function InstaboxIntegration() {
  const { t } = useLanguage()
  const { search } = useLocation()

  const [settings, setSettings] = useState<InstaboxSettingsDTO | null>(null)
  const [endpointInput, setEndpointInput] = useState('')
  const [apiKeyInput, setApiKeyInput] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [enabled, setEnabled] = useState(false)

  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [validating, setValidating] = useState(false)

  const [feedback, setFeedback] = useState<{
    type: 'success' | 'error' | 'info'
    message: string
  } | null>(null)

  const loadSettings = async () => {
    try {
      setLoading(true)
      const res = await apiFetch<{ status: string; settings: InstaboxSettingsDTO }>(
        '/api/v1/settings/instabox'
      )
      if (res && res.settings) {
        setSettings(res.settings)
        setEndpointInput(res.settings.endpoint_url || '')
        setEnabled(res.settings.enabled)
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setFeedback({
        type: 'error',
        message: t('Gagal memuat pengaturan Instabox: ') + msg,
      })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadSettings()
  }, [])

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setFeedback(null)

    try {
      const payload: {
        endpoint_url: string
        api_key?: string
        enabled: boolean
      } = {
        endpoint_url: endpointInput.trim(),
        enabled,
      }

      if (apiKeyInput.trim() !== '') {
        payload.api_key = apiKeyInput.trim()
      }

      const res = await apiFetch<{ status: string; message: string; settings: InstaboxSettingsDTO }>(
        '/api/v1/settings/instabox',
        {
          method: 'POST',
          body: JSON.stringify(payload),
        }
      )

      if (res && res.settings) {
        setSettings(res.settings)
        setEndpointInput(res.settings.endpoint_url || '')
        setEnabled(res.settings.enabled)
        setApiKeyInput('')
      }

      setFeedback({
        type: 'success',
        message: t('Pengaturan Instabox berhasil disimpan.'),
      })
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setFeedback({
        type: 'error',
        message: t('Gagal menyimpan: ') + msg,
      })
    } finally {
      setSaving(false)
    }
  }

  const handleValidate = async () => {
    setValidating(true)
    setFeedback(null)

    try {
      const res = await apiFetch<{
        status: string
        valid: boolean
        message: string
        http_status?: number
      }>('/api/v1/settings/instabox/validate', {
        method: 'POST',
        body: JSON.stringify({
          endpoint_url: endpointInput.trim(),
          api_key: apiKeyInput.trim() || undefined,
        }),
      })

      if (res && res.valid) {
        setFeedback({
          type: 'success',
          message: t('Koneksi berhasil! ') + (res.message || ''),
        })
        void loadSettings()
      } else {
        setFeedback({
          type: 'error',
          message: t('Pengujian koneksi gagal: ') + (res?.message || t('Endpoint tidak merespons.')),
        })
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setFeedback({
        type: 'error',
        message: t('Uji koneksi gagal: ') + msg,
      })
    } finally {
      setValidating(false)
    }
  }

  if (loading) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        <RefreshCw className="mr-2 h-4 w-4 animate-spin text-muted-foreground/60" />
        {t('Memuat pengaturan Instabox…')}
      </div>
    )
  }

  const status = settings?.validation_status || 'untested'
  const isConnected = settings?.enabled && status === 'valid'

  return (
    <div className="min-w-0 max-w-2xl space-y-6" aria-label="Instabox integration settings">
      <nav>
        <Link
          to={'/settings/integrations' + search}
          className="inline-flex items-center text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          {t('Kembali ke Integrasi')}
        </Link>
      </nav>

      <header className="space-y-1">
        <div className="flex items-center gap-2.5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-muted text-foreground">
            <Server className="h-4 w-4" />
          </div>
          <h1 className="text-lg font-semibold tracking-tight text-foreground">
            Instabox / Remote Browser
          </h1>
        </div>
        <p className="text-sm text-muted-foreground leading-relaxed">
          {t(
            'Alihkan eksekusi headless browser, tangkapan layar Playwright, dan inspeksi visual ke sandbox cloud terisolasi agar menghemat memori dan CPU server.'
          )}
        </p>
      </header>

      {/* Status banner */}
      <div className="flex items-center justify-between rounded-lg bg-muted/30 px-3.5 py-2.5 text-xs text-muted-foreground">
        <div className="flex items-center gap-2">
          <span
            className={`h-2 w-2 rounded-full ${
              isConnected
                ? 'bg-emerald-500'
                : status === 'invalid'
                ? 'bg-rose-500'
                : 'bg-amber-400'
            }`}
          />
          <span>
            {isConnected
              ? t('Terhubung dan aktif')
              : status === 'invalid'
              ? t('Koneksi terputus / tidak valid')
              : t('Belum diuji')}
          </span>
        </div>
        {settings?.endpoint_url && (
          <span className="font-mono text-[11px] text-muted-foreground/70 truncate max-w-[200px]">
            {settings.endpoint_url}
          </span>
        )}
      </div>

      {feedback && (
        <div
          role="status"
          className={`flex items-start gap-2.5 rounded-lg p-3 text-xs leading-relaxed ${
            feedback.type === 'success'
              ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
              : feedback.type === 'error'
              ? 'bg-rose-500/10 text-rose-700 dark:text-rose-400'
              : 'bg-muted text-muted-foreground'
          }`}
        >
          {feedback.type === 'success' ? (
            <CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          ) : (
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          )}
          <span>{feedback.message}</span>
        </div>
      )}

      <form onSubmit={handleSave} className="space-y-5">
        <div className="space-y-4 rounded-xl border border-border/40 p-4 bg-card/40">
          <div className="space-y-1.5">
            <label
              htmlFor="instabox-endpoint"
              className="text-xs font-medium text-foreground flex items-center justify-between"
            >
              <span>{t('Endpoint URL')}</span>
              <span className="text-[11px] font-normal text-muted-foreground">
                (WebSocket / HTTP CDP URL)
              </span>
            </label>
            <input
              id="instabox-endpoint"
              type="text"
              value={endpointInput}
              onChange={(e) => setEndpointInput(e.target.value)}
              placeholder="https://instabox.example.com atau wss://..."
              className="w-full rounded-md border border-border/50 bg-background px-3 py-2 text-xs font-mono placeholder:text-muted-foreground/50 focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <p className="text-[11px] text-muted-foreground leading-normal">
              {t('Alamat server browser remote (misal Instabox, Browserless, atau endpoint Chrome CDP).')}
            </p>
          </div>

          <div className="space-y-1.5">
            <label
              htmlFor="instabox-key"
              className="text-xs font-medium text-foreground flex items-center justify-between"
            >
              <span>{t('API Key / Token Rahasia')}</span>
              {settings?.has_api_key && (
                <span className="text-[11px] font-normal text-emerald-600 dark:text-emerald-400">
                  {t('Tersimpan: ')} {settings.masked_api_key}
                </span>
              )}
            </label>
            <div className="relative">
              <input
                id="instabox-key"
                type={showKey ? 'text' : 'password'}
                value={apiKeyInput}
                onChange={(e) => setApiKeyInput(e.target.value)}
                placeholder={
                  settings?.has_api_key
                    ? t('Masukkan kunci baru untuk mengganti…')
                    : 'Bearer token atau sandi'
                }
                className="w-full rounded-md border border-border/50 bg-background px-3 py-2 pr-9 text-xs font-mono placeholder:text-muted-foreground/50 focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
              />
              <button
                type="button"
                onClick={() => setShowKey(!showKey)}
                aria-label={showKey ? t('Sembunyikan sandi') : t('Tampilkan sandi')}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                {showKey ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              </button>
            </div>
            <p className="text-[11px] text-muted-foreground leading-normal">
              {t('Otentikasi jika Instabox diamankan dengan token akses.')}
            </p>
          </div>

          <div className="pt-2 border-t border-border/30 flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-foreground">
                {t('Aktifkan Browser Remote')}
              </p>
              <p className="text-[11px] text-muted-foreground">
                {t('Gunakan endpoint ini untuk pemeriksaan inspeksi visual Playwright.')}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="sr-only peer"
              />
              <div className="w-9 h-5 bg-muted peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-primary" />
            </label>
          </div>
        </div>

        <div className="flex items-center gap-2 pt-1">
          <button
            type="submit"
            disabled={saving}
            className="action-primary text-xs flex items-center gap-1.5"
          >
            {saving && <RefreshCw className="h-3 w-3 animate-spin" />}
            {t('Simpan Pengaturan')}
          </button>

          <button
            type="button"
            onClick={handleValidate}
            disabled={validating || !endpointInput.trim()}
            className="action-secondary text-xs flex items-center gap-1.5"
          >
            {validating && <RefreshCw className="h-3 w-3 animate-spin" />}
            {t('Uji Koneksi')}
          </button>
        </div>
      </form>
    </div>
  )
}
