import { useState, useEffect } from 'react'
import { apiFetch } from '../../api/client'
import { useLanguage } from '../../context/LanguageContext'
import { Mail, ShieldCheck, CheckCircle2, AlertTriangle, Send, RefreshCw, Eye, EyeOff } from 'lucide-react'
import { FormField, Input, Select, FormGrid } from '../../components/ui/forms'

export interface ResendSettingsDTO {
  has_webhook_secret?: boolean
  has_api_key: boolean
  masked_api_key: string
  from_email: string
  from_name: string
  notification_email: string
  idle_threshold_minutes: number
  enabled: boolean
  last_validated_at?: string
  validation_status: 'untested' | 'valid' | 'invalid'
}

export function ResendSettingsCard() {
  const { t } = useLanguage()

  const [settings, setSettings] = useState<ResendSettingsDTO | null>(null)
  const [apiKeyInput, setApiKeyInput] = useState('')
  const [webhookSecret, setWebhookSecret] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [fromName, setFromName] = useState('')
  const [fromEmail, setFromEmail] = useState('')
  const [notificationEmail, setNotificationEmail] = useState('')
  const [idleMinutes, setIdleMinutes] = useState(5)
  const [enabled, setEnabled] = useState(false)

  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [validating, setValidating] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testEmailRecipient, setTestEmailRecipient] = useState('')

  const [feedback, setFeedback] = useState<{
    type: 'success' | 'error' | 'info'
    message: string
  } | null>(null)

  const loadSettings = async () => {
    try {
      setLoading(true)
      const res = await apiFetch<{ status: string; settings: ResendSettingsDTO }>(
        '/api/v1/settings/resend'
      )
      if (res && res.settings) {
        setSettings(res.settings)
        setFromName(res.settings.from_name || 'Cuan Support')
        setFromEmail(res.settings.from_email || 'onboarding@resend.dev')
        setNotificationEmail(res.settings.notification_email || '')
        setIdleMinutes(res.settings.idle_threshold_minutes || 5)
        setEnabled(res.settings.enabled)
        if (res.settings.notification_email) {
          setTestEmailRecipient(res.settings.notification_email)
        }
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Gagal memuat pengaturan Resend.'
      setFeedback({ type: 'error', message: msg })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadSettings()
  }, [])

  const handleSave = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    setSaving(true)
    setFeedback(null)

    try {
      const payload: Record<string, any> = {
        from_name: fromName.trim(),
        from_email: fromEmail.trim(),
        notification_email: notificationEmail.trim(),
        idle_threshold_minutes: Number(idleMinutes),
        enabled,
      }
      if (apiKeyInput.trim()) {
        payload.api_key = apiKeyInput.trim()
      }
      if (webhookSecret.trim()) payload.webhook_secret = webhookSecret.trim()

      const res = await apiFetch<{ status: string; message: string; settings: ResendSettingsDTO }>(
        '/api/v1/settings/resend',
        {
          method: 'POST',
          body: JSON.stringify(payload),
        }
      )

      setSettings(res.settings)
      setApiKeyInput('')
      setWebhookSecret('')
      setFeedback({
        type: 'success',
        message: t('Pengaturan Resend berhasil disimpan dengan aman dalam vault terenkripsi!'),
      })
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Gagal menyimpan pengaturan.'
      setFeedback({ type: 'error', message: msg })
    } finally {
      setSaving(false)
    }
  }

  const handleValidateKey = async () => {
    setValidating(true)
    setFeedback(null)

    try {
      const payload: Record<string, any> = {}
      if (apiKeyInput.trim()) {
        payload.api_key = apiKeyInput.trim()
      }

      const res = await apiFetch<{
        status: string
        valid: boolean
        message?: string
        error?: string
        validation_status: 'valid' | 'invalid'
      }>('/api/v1/settings/resend/validate', {
        method: 'POST',
        body: JSON.stringify(payload),
      })

      if (res.valid) {
        setFeedback({
          type: 'success',
          message: t('✅ API Key Resend Valid! Koneksi ke API Resend berhasil diverifikasi.'),
        })
      } else {
        setFeedback({
          type: 'error',
          message: t('❌ API Key Resend Tidak Valid: ') + (res.error || 'Autentikasi ditolak.'),
        })
      }
      void loadSettings()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Gagal memvalidasi API Key.'
      setFeedback({ type: 'error', message: msg })
    } finally {
      setValidating(false)
    }
  }

  const handleSendTestEmail = async () => {
    const target = testEmailRecipient.trim() || notificationEmail.trim()
    if (!target) {
      setFeedback({
        type: 'error',
        message: 'Masukkan email tujuan pengujian.',
      })
      return
    }

    setTesting(true)
    setFeedback(null)

    try {
      const res = await apiFetch<{
        status: string
        message: string
        resend_email_id?: string
      }>('/api/v1/settings/resend/test', {
        method: 'POST',
        body: JSON.stringify({ recipient_email: target }),
      })

      setFeedback({
        type: 'success',
        message: `${res.message} (ID Resend: ${res.resend_email_id || 'ok'})`,
      })
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Gagal mengirim email pengujian.'
      setFeedback({ type: 'error', message: msg })
    } finally {
      setTesting(false)
    }
  }

  if (loading) {
    return (
      <section className="rounded-xl border border-border/40 bg-card/50 p-5" aria-label="Resend Email Settings">
        <p className="text-sm text-muted-foreground">{t('Memuat konfigurasi Resend…')}</p>
      </section>
    )
  }

  return (
    <section className="rounded-xl border border-border/40 bg-card/50 p-5 space-y-5" aria-label="Resend Email Settings">
      <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-border/50 pb-4">
        <div className="flex items-center gap-3">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Mail className="h-5 w-5" aria-hidden />
          </span>
          <div>
            <h2 className="text-base font-semibold flex items-center gap-2">
              <span>{t('Integrasi Email Resend & Transkrip Chat Idle')}</span>
              {settings?.enabled ? (
                <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-xs font-medium text-emerald-600 dark:text-emerald-400">
                  <CheckCircle2 className="h-3 w-3" />
                  {t('Aktif')}
                </span>
              ) : (
                <span className="inline-flex items-center rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium text-muted-foreground">
                  {t('Nonaktif')}
                </span>
              )}
            </h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t('Kirim ringkasan & notifikasi transkrip email otomatis saat pesan Live Chat pelanggan idle belum terbaca (>5-10 menit).')}
            </p>
          </div>
        </div>

        {/* Validation Status Badge */}
        {settings?.has_api_key && (
          <div className="flex items-center gap-1.5 self-start sm:self-auto">
            {settings.validation_status === 'valid' ? (
              <span className="inline-flex items-center gap-1 text-xs text-emerald-600 font-medium">
                <ShieldCheck className="h-4 w-4" />
                {t('Koneksi Valid')}
              </span>
            ) : settings.validation_status === 'invalid' ? (
              <span className="inline-flex items-center gap-1 text-xs text-destructive font-medium">
                <AlertTriangle className="h-4 w-4" />
                {t('Key Tidak Valid')}
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 text-xs text-amber-600 font-medium">
                <RefreshCw className="h-3.5 w-3.5" />
                {t('Belum Divalidasi')}
              </span>
            )}
          </div>
        )}
      </header>

      {/* Action feedback banner */}
      {feedback && (
        <div
          role="status"
          className={`rounded-lg p-3 text-sm flex items-start gap-2.5 ${
            feedback.type === 'success'
              ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200 border border-emerald-200 dark:border-emerald-900'
              : feedback.type === 'error'
              ? 'bg-destructive/10 text-destructive border border-destructive/20'
              : 'bg-muted text-foreground'
          }`}
        >
          {feedback.type === 'success' ? (
            <CheckCircle2 className="h-4 w-4 shrink-0 mt-0.5" />
          ) : (
            <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
          )}
          <span className="flex-1">{feedback.message}</span>
        </div>
      )}

      <form onSubmit={handleSave} className="space-y-4">
        {/* API Key Field */}
        <div className="space-y-1.5">
          <FormField
            label={t('Resend API Key')}
            accessory={
              settings?.has_api_key ? (
                <span className="text-xs text-muted-foreground font-normal">
                  {t('Tersimpan:')} <code className="bg-muted px-1.5 py-0.5 rounded">{settings.masked_api_key}</code>
                </span>
              ) : undefined
            }
          >
            <div className="relative flex items-center">
              <Input
                type={showKey ? 'text' : 'password'}
                mono
                className="pr-10"
                placeholder={settings?.has_api_key ? t('Masukkan API Key baru untuk mengganti…') : 're_1234567890abcdef…'}
                value={apiKeyInput}
                onChange={(e) => setApiKeyInput(e.target.value)}
              />
              <button
                type="button"
                className="absolute right-3 text-muted-foreground hover:text-foreground p-1"
                onClick={() => setShowKey(!showKey)}
                aria-label={showKey ? 'Sembunyikan API key' : 'Tampilkan API key'}
              >
                {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </FormField>
          <div className="flex items-center justify-between text-xs text-muted-foreground pt-1">
            <span>{t('API Key disimpan secara aman menggunakan enkripsi AES-256-GCM pada vault server.')}</span>
            <button
              type="button"
              onClick={handleValidateKey}
              disabled={validating || (!settings?.has_api_key && !apiKeyInput)}
              className="action-secondary text-xs inline-flex items-center gap-1 py-1 px-2.5"
            >
              <ShieldCheck className="h-3 w-3" />
              <span>{validating ? t('Memvalidasi…') : t('Validasi Key')}</span>
            </button>
          </div>
        </div>

        {/* Sender details: From Name & From Email */}
        <details className="text-sm">
          <summary className="cursor-pointer py-3 text-muted-foreground">{t('Email delivery tracking')}</summary>
          <div className="space-y-2 pb-3">
            <FormField label={t('Webhook signing secret')} htmlFor="resend-webhook-secret" helperText={`${t('Resend webhook URL')}: ${typeof window !== 'undefined' ? window.location.origin : ''}/api/webhooks/resend`}>
              <Input
                id="resend-webhook-secret"
                type="password"
                autoComplete="new-password"
                value={webhookSecret}
                onChange={e => setWebhookSecret(e.target.value)}
                placeholder={settings?.has_webhook_secret ? t('Signing secret saved') : 'whsec_…'}
                mono
              />
            </FormField>
          </div>
        </details>

        <FormGrid cols={2}>
          <FormField label={t('Nama Pengirim (From Name)')}>
            <Input
              type="text"
              placeholder="Cuan Support"
              value={fromName}
              onChange={(e) => setFromName(e.target.value)}
            />
          </FormField>
          <FormField
            label={t('Email Pengirim (From Email)')}
            helperText={t('Gunakan domain terverifikasi Anda di Resend, atau default onboarding@resend.dev untuk testing.')}
          >
            <Input
              type="email"
              placeholder="onboarding@resend.dev"
              value={fromEmail}
              onChange={(e) => setFromEmail(e.target.value)}
            />
          </FormField>
        </FormGrid>

        {/* Recipient Notification Email & Idle Threshold */}
        <FormGrid cols={2}>
          <FormField
            label={t('Email Penerima Notifikasi Transkrip')}
            helperText={t('Alamat email staf / pemilik ruang kerja untuk menerima ringkasan pesan chat yang belum terbaca.')}
          >
            <Input
              type="email"
              placeholder="pemilik@workspace.com"
              value={notificationEmail}
              onChange={(e) => setNotificationEmail(e.target.value)}
            />
          </FormField>
          <FormField
            label={t('Ambang Batas Waktu Chat Idle')}
            helperText={t('Email otomatis dikirim jika pelanggan mengirim pesan dan tidak direspons dalam batas waktu ini.')}
          >
            <Select
              value={idleMinutes}
              onChange={(e) => setIdleMinutes(Number(e.target.value))}
            >
              <option value={3}>3 {t('Menit (Testing / Cepat)')}</option>
              <option value={5}>5 {t('Menit (Direkomendasikan)')}</option>
              <option value={10}>10 {t('Menit (Standar Bisnis)')}</option>
              <option value={15}>15 {t('Menit')}</option>
              <option value={30}>30 {t('Menit')}</option>
            </Select>
          </FormField>
        </FormGrid>

        {/* Enabled Toggle */}
        <div className="rounded-lg border border-border/60 bg-muted/20 p-3.5">
          <label className="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              className="h-4 w-4 rounded border-border text-primary focus:ring-primary"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            <div>
              <span className="text-sm font-semibold">{t('Aktifkan Notifikasi Email Otomatis')}</span>
              <p className="text-xs text-muted-foreground">
                {t('Background timer dan cron job Cuan akan aktif memindai percakapan Live Chat dan mengirimkan email via Resend.')}
              </p>
            </div>
          </label>
        </div>

        {/* Save button */}
        <div className="flex items-center justify-end gap-3 pt-2">
          <button
            type="submit"
            disabled={saving}
            className="action-primary min-h-11 inline-flex items-center justify-center px-5 text-sm"
          >
            {saving ? t('Menyimpan…') : t('Simpan Pengaturan Resend')}
          </button>
        </div>
      </form>

      {/* Live Email Testing Section */}
      <div className="border-t border-border/60 pt-4 mt-4 space-y-3">
        <h3 className="text-sm font-semibold flex items-center gap-1.5">
          <Send className="h-4 w-4 text-primary" />
          <span>{t('Uji Coba Pengiriman Email Resend')}</span>
        </h3>
        <p className="text-xs text-muted-foreground">
          {t('Kirimkan satu email uji coba langsung untuk memastikan kredensial Resend dan email pengirim/penerima berfungsi normal.')}
        </p>
        <div className="flex flex-col sm:flex-row gap-2.5 max-w-lg">
          <Input
            type="email"
            className="flex-1"
            placeholder="penerima-uji-coba@domain.com"
            value={testEmailRecipient}
            onChange={(e) => setTestEmailRecipient(e.target.value)}
          />
          <button
            type="button"
            onClick={handleSendTestEmail}
            disabled={testing || !settings?.has_api_key}
            className="action-secondary min-h-10 inline-flex items-center justify-center gap-1.5 px-4 text-xs font-medium"
          >
            <Send className="h-3.5 w-3.5" />
            <span>{testing ? t('Mengirim…') : t('Kirim Uji Coba')}</span>
          </button>
        </div>
      </div>
    </section>
  )
}
