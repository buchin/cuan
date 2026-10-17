import { useState, useEffect } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import {
  X,
  Bell,
  Send,
  Trash2,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  ShieldCheck,
  Plus,
  Info,
  Check
} from 'lucide-react'
import { AlertChannelDTO } from './types'

interface AlertChannelsModalProps {
  isOpen: boolean
  onClose: () => void
  projectId?: string
}

export function AlertChannelsModal({
  isOpen,
  onClose,
  projectId = 'default',
}: AlertChannelsModalProps) {
  if (!isOpen) return null

  // Channels state
  const [channels, setChannels] = useState<AlertChannelDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  // View state: 'list' | 'create'
  const [activeTab, setActiveTab] = useState<'list' | 'create'>('list')

  // Form state
  const [channelType, setChannelType] = useState<'telegram' | 'discord' | 'webhook'>('telegram')
  const [name, setName] = useState('')
  const [botToken, setBotToken] = useState('')
  const [chatId, setChatId] = useState('')
  const [webhookUrl, setWebhookUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [creating, setCreating] = useState(false)

  // Testing state: map channel ID to status ('testing' | 'success' | 'failed' | null)
  const [testingStatus, setTestingStatus] = useState<Record<string, { status: 'testing' | 'success' | 'failed'; message?: string }>>({})

  // Fetch channels on mount / projectId change
  const fetchChannels = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`/api/projects/${projectId}/alert-channels`)
      if (!res.ok) {
        throw new Error(`Gagal mengambil alert channels (HTTP ${res.status})`)
      }
      const data = await res.json()
      setChannels(data.channels || [])
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan saat memuat alert channels.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      fetchChannels()
    }
  }, [isOpen, projectId])

  // Reset form
  const resetForm = () => {
    setName('')
    setBotToken('')
    setChatId('')
    setWebhookUrl('')
    setSecret('')
    setError(null)
  }

  // Create channel handler
  const handleCreateChannel = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      setError('Nama channel wajib diisi.')
      return
    }

    if (channelType === 'telegram') {
      if (!botToken.trim()) {
        setError('Telegram Bot Token wajib diisi.')
        return
      }
      if (!chatId.trim()) {
        setError('Telegram Chat ID wajib diisi.')
        return
      }
    } else if (channelType === 'discord') {
      if (!webhookUrl.trim() || !webhookUrl.startsWith('http')) {
        setError('Discord Webhook URL tidak valid (harus diawali http/https).')
        return
      }
    } else if (channelType === 'webhook') {
      if (!webhookUrl.trim() || !webhookUrl.startsWith('http')) {
        setError('Webhook URL tidak valid (harus diawali http/https).')
        return
      }
    }

    setCreating(true)
    setError(null)

    try {
      const payload: Record<string, any> = {
        name: name.trim(),
        type: channelType,
        is_active: true,
      }

      if (channelType === 'telegram') {
        payload.bot_token = botToken.trim()
        payload.chat_id = chatId.trim()
      } else if (channelType === 'discord') {
        payload.webhook_url = webhookUrl.trim()
      } else if (channelType === 'webhook') {
        payload.webhook_url = webhookUrl.trim()
        if (secret.trim()) {
          payload.secret = secret.trim()
        }
      }

      const res = await fetch(`/api/projects/${projectId}/alert-channels`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }

      setSuccessMsg(`Channel "${name}" berhasil didaftarkan dan diamankan dengan enkripsi AES-GCM!`)
      setTimeout(() => setSuccessMsg(null), 4000)
      resetForm()
      setActiveTab('list')
      await fetchChannels()
    } catch (err: any) {
      setError(err.message || 'Gagal menyimpan alert channel.')
    } finally {
      setCreating(false)
    }
  }

  // Test Alert 1-klik handler
  const handleTestChannel = async (channel: AlertChannelDTO) => {
    setTestingStatus((prev) => ({
      ...prev,
      [channel.id]: { status: 'testing' },
    }))

    try {
      const res = await fetch(`/api/alert-channels/${channel.id}/test`, {
        method: 'POST',
      })

      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error(data.error || `Gagal mengirim test alert (HTTP ${res.status})`)
      }

      setTestingStatus((prev) => ({
        ...prev,
        [channel.id]: {
          status: 'success',
          message: data.message || 'Pesan uji coba berhasil terkirim!',
        },
      }))

      setTimeout(() => {
        setTestingStatus((prev) => {
          const updated = { ...prev }
          delete updated[channel.id]
          return updated
        })
      }, 5000)
    } catch (err: any) {
      setTestingStatus((prev) => ({
        ...prev,
        [channel.id]: {
          status: 'failed',
          message: err.message || 'Gagal mengirim test alert',
        },
      }))
    }
  }

  // Delete channel handler
  const handleDeleteChannel = async (channelId: string, channelName: string) => {
    if (!window.confirm(`Hapus saluran notifikasi "${channelName}"?`)) {
      return
    }

    try {
      const res = await fetch(`/api/alert-channels/${channelId}`, {
        method: 'DELETE',
      })

      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }

      setSuccessMsg(`Channel "${channelName}" berhasil dihapus.`)
      setTimeout(() => setSuccessMsg(null), 3000)
      setChannels((prev) => prev.filter((c) => c.id !== channelId))
    } catch (err: any) {
      setError(err.message || 'Gagal menghapus alert channel.')
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 animate-in fade-in duration-200">
      <Card className="w-full max-w-2xl bg-white dark:bg-slate-900 shadow-2xl border-slate-200 dark:border-slate-800 flex flex-col max-h-[90vh] overflow-hidden animate-in zoom-in-95 duration-200">
        {/* Modal Header */}
        <div className="p-6 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50/50 dark:bg-slate-900/50">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
              <Bell className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <span>Alert Channels Setup</span>
                <span className="flex items-center gap-1 text-[11px] font-normal text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/60 px-2 py-0.5 rounded-full border border-emerald-200 dark:border-emerald-800">
                  <ShieldCheck className="h-3 w-3" /> AES-GCM Encrypted
                </span>
              </h2>
              <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                Konfigurasi notifikasi darurat insiden outage & degradasi langsung ke Telegram, Discord, atau Webhook.
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Tab Switcher: Daftar Channels vs Tambah Channel */}
        <div className="flex items-center justify-between px-6 py-2.5 bg-slate-50 dark:bg-slate-800/40 border-b border-slate-100 dark:border-slate-800 text-xs">
          <div className="flex items-center gap-1.5">
            <button
              onClick={() => {
                setActiveTab('list')
                setError(null)
              }}
              className={`py-1.5 px-3 rounded-lg font-semibold transition-all ${
                activeTab === 'list'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              Daftar Channel ({channels.length})
            </button>
            <button
              onClick={() => {
                setActiveTab('create')
                setError(null)
              }}
              className={`flex items-center gap-1 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                activeTab === 'create'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              <Plus className="h-3.5 w-3.5 text-emerald-500" />
              <span>Tambah Channel Baru</span>
            </button>
          </div>

          <Button
            variant="ghost"
            size="sm"
            onClick={fetchChannels}
            disabled={loading}
            className="h-7 text-[11px] px-2 text-slate-500"
          >
            <RefreshCw className={`h-3 w-3 mr-1 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>

        {/* Global Feedback Banner */}
        {successMsg && (
          <div className="px-6 py-2.5 bg-emerald-50 dark:bg-emerald-950/40 border-b border-emerald-200 dark:border-emerald-800 text-emerald-800 dark:text-emerald-300 text-xs flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-emerald-600 shrink-0" />
            <span>{successMsg}</span>
          </div>
        )}
        {error && (
          <div className="px-6 py-2.5 bg-red-50 dark:bg-red-950/40 border-b border-red-200 dark:border-red-900 text-red-800 dark:text-red-300 text-xs flex items-center gap-2">
            <AlertCircle className="h-4 w-4 text-red-600 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Modal Body */}
        <div className="p-6 overflow-y-auto space-y-4 flex-1">
          {activeTab === 'list' ? (
            /* ================= Channels List Tab ================= */
            <div className="space-y-3">
              {loading && channels.length === 0 ? (
                <div className="py-12 text-center text-slate-400 space-y-2">
                  <RefreshCw className="h-6 w-6 animate-spin mx-auto text-emerald-500" />
                  <p className="text-xs">Memuat daftar alert channels...</p>
                </div>
              ) : channels.length === 0 ? (
                <div className="py-12 text-center border-2 border-dashed border-slate-200 dark:border-slate-800 rounded-2xl p-6 space-y-3">
                  <div className="p-3 bg-slate-100 dark:bg-slate-800 rounded-full w-fit mx-auto text-slate-400">
                    <Bell className="h-6 w-6" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-sm text-slate-900 dark:text-white">
                      Belum Ada Alert Channel
                    </h3>
                    <p className="text-xs text-slate-500 dark:text-slate-400 max-w-sm mx-auto mt-1">
                      Daftarkan Telegram Bot, Discord Webhook, atau Custom Webhook agar tim Anda seketika menerima notifikasi saat terjadi outage.
                    </p>
                  </div>
                  <Button
                    size="sm"
                    onClick={() => setActiveTab('create')}
                    className="bg-emerald-600 hover:bg-emerald-500 text-white text-xs gap-1.5"
                  >
                    <Plus className="h-3.5 w-3.5" />
                    Tambah Channel Sekarang
                  </Button>
                </div>
              ) : (
                channels.map((ch) => {
                  const testState = testingStatus[ch.id]
                  return (
                    <div
                      key={ch.id}
                      className="p-4 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/30 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:border-slate-300 dark:hover:border-slate-700 transition-colors"
                    >
                      <div className="space-y-1.5 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="font-bold text-sm text-slate-900 dark:text-white truncate">
                            {ch.name}
                          </span>
                          {/* Channel Type Badge */}
                          {ch.type === 'telegram' ? (
                            <Badge className="bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/30 text-[10px] font-mono">
                              Telegram
                            </Badge>
                          ) : ch.type === 'discord' ? (
                            <Badge className="bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border-indigo-500/30 text-[10px] font-mono">
                              Discord
                            </Badge>
                          ) : (
                            <Badge className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30 text-[10px] font-mono">
                              Webhook
                            </Badge>
                          )}

                          {ch.is_active ? (
                            <Badge variant="outline" className="border-emerald-500/40 text-emerald-600 dark:text-emerald-400 text-[10px] gap-1">
                              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" /> Aktif
                            </Badge>
                          ) : (
                            <Badge variant="secondary" className="text-[10px]">
                              Nonaktif
                            </Badge>
                          )}
                        </div>

                        {/* Masked Config Summary */}
                        <div className="text-[11px] font-mono text-slate-500 dark:text-slate-400 flex items-center gap-2 flex-wrap">
                          {ch.masked_config ? (
                            Object.entries(ch.masked_config).map(([k, v]) => (
                              <span key={k} className="bg-slate-200/70 dark:bg-slate-800 px-1.5 py-0.5 rounded text-[10px]">
                                {k}: <span className="text-slate-700 dark:text-slate-300 font-semibold">{v}</span>
                              </span>
                            ))
                          ) : (
                            <span>Credentials: Terenkripsi AES-GCM</span>
                          )}
                          <span className="text-[10px] text-slate-400">
                            • ID: {ch.id.slice(0, 12)}...
                          </span>
                        </div>

                        {/* Test Status Feedback Row */}
                        {testState && (
                          <div
                            className={`text-[11px] p-2 rounded-lg flex items-center gap-1.5 ${
                              testState.status === 'testing'
                                ? 'bg-blue-50 dark:bg-blue-950/40 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-800'
                                : testState.status === 'success'
                                ? 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800'
                                : 'bg-red-50 dark:bg-red-950/40 text-red-700 dark:text-red-300 border border-red-200 dark:border-red-800'
                            }`}
                          >
                            {testState.status === 'testing' ? (
                              <>
                                <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                                <span>Mengirim pesan uji coba ke saluran...</span>
                              </>
                            ) : testState.status === 'success' ? (
                              <>
                                <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
                                <span>{testState.message}</span>
                              </>
                            ) : (
                              <>
                                <AlertCircle className="h-3.5 w-3.5 text-red-600" />
                                <span>{testState.message}</span>
                              </>
                            )}
                          </div>
                        )}
                      </div>

                      {/* Channel Action Buttons: Test Alert & Delete */}
                      <div className="flex items-center gap-2 shrink-0 pt-2 sm:pt-0">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleTestChannel(ch)}
                          disabled={testState?.status === 'testing'}
                          className={`h-8 text-xs font-semibold px-2.5 flex items-center gap-1.5 transition-all ${
                            testState?.status === 'success'
                              ? 'border-emerald-500 text-emerald-600 bg-emerald-50 dark:bg-emerald-950/40'
                              : 'border-slate-300 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800'
                          }`}
                        >
                          {testState?.status === 'testing' ? (
                            <RefreshCw className="h-3 w-3 animate-spin text-emerald-600" />
                          ) : testState?.status === 'success' ? (
                            <Check className="h-3 w-3 text-emerald-600" />
                          ) : (
                            <Send className="h-3 w-3 text-emerald-600" />
                          )}
                          <span>
                            {testState?.status === 'testing'
                              ? 'Testing...'
                              : testState?.status === 'success'
                              ? 'Terkirim!'
                              : 'Test Alert'}
                          </span>
                        </Button>

                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleDeleteChannel(ch.id, ch.name)}
                          className="h-8 w-8 p-0 text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-950/40"
                          title="Hapus Channel"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </div>
                  )
                })
              )}
            </div>
          ) : (
            /* ================= Create Channel Tab ================= */
            <form onSubmit={handleCreateChannel} className="space-y-4">
              {/* Channel Type Selector */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider block">
                  Pilih Saluran Notifikasi
                </label>
                <div className="grid grid-cols-3 gap-2">
                  <button
                    type="button"
                    onClick={() => setChannelType('telegram')}
                    className={`p-3 rounded-xl border text-left flex flex-col justify-between transition-all ${
                      channelType === 'telegram'
                        ? 'border-sky-500 bg-sky-50/50 dark:bg-sky-950/30 text-sky-900 dark:text-sky-200 ring-2 ring-sky-500/20'
                        : 'border-slate-200 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800/40'
                    }`}
                  >
                    <div className="flex items-center justify-between w-full">
                      <span className="font-bold text-xs">Telegram</span>
                      {channelType === 'telegram' && (
                        <CheckCircle2 className="h-3.5 w-3.5 text-sky-500" />
                      )}
                    </div>
                    <span className="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
                      Bot & Group Chat
                    </span>
                  </button>

                  <button
                    type="button"
                    onClick={() => setChannelType('discord')}
                    className={`p-3 rounded-xl border text-left flex flex-col justify-between transition-all ${
                      channelType === 'discord'
                        ? 'border-indigo-500 bg-indigo-50/50 dark:bg-indigo-950/30 text-indigo-900 dark:text-indigo-200 ring-2 ring-indigo-500/20'
                        : 'border-slate-200 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800/40'
                    }`}
                  >
                    <div className="flex items-center justify-between w-full">
                      <span className="font-bold text-xs">Discord</span>
                      {channelType === 'discord' && (
                        <CheckCircle2 className="h-3.5 w-3.5 text-indigo-500" />
                      )}
                    </div>
                    <span className="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
                      Incoming Webhook
                    </span>
                  </button>

                  <button
                    type="button"
                    onClick={() => setChannelType('webhook')}
                    className={`p-3 rounded-xl border text-left flex flex-col justify-between transition-all ${
                      channelType === 'webhook'
                        ? 'border-amber-500 bg-amber-50/50 dark:bg-amber-950/30 text-amber-900 dark:text-amber-200 ring-2 ring-amber-500/20'
                        : 'border-slate-200 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800/40'
                    }`}
                  >
                    <div className="flex items-center justify-between w-full">
                      <span className="font-bold text-xs">Custom Webhook</span>
                      {channelType === 'webhook' && (
                        <CheckCircle2 className="h-3.5 w-3.5 text-amber-500" />
                      )}
                    </div>
                    <span className="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
                      HMAC SHA256 Signed
                    </span>
                  </button>
                </div>
              </div>

              {/* Zero-Friction Guidance Card */}
              <div className="p-3.5 bg-slate-100 dark:bg-slate-800/60 rounded-xl border border-slate-200 dark:border-slate-700/60 text-xs space-y-2">
                <div className="flex items-center gap-1.5 font-bold text-slate-800 dark:text-slate-200">
                  <Info className="h-4 w-4 text-emerald-500 shrink-0" />
                  <span>
                    {channelType === 'telegram'
                      ? 'Panduan Setup Telegram Bot (Zero-Friction)'
                      : channelType === 'discord'
                      ? 'Panduan Setup Discord Webhook (Zero-Friction)'
                      : 'Panduan Integrasi Custom Webhook (HMAC Signature)'}
                  </span>
                </div>

                {channelType === 'telegram' ? (
                  <div className="text-[11px] text-slate-600 dark:text-slate-300 space-y-1.5 leading-relaxed">
                    <p>
                      1. Buka <strong>@BotFather</strong> di Telegram, kirim command <code className="bg-white dark:bg-slate-900 px-1 py-0.5 rounded font-mono">/newbot</code>, beri nama bot, dan salin <strong>API Token</strong>.
                    </p>
                    <p>
                      2. Mulai percakapan dengan bot Anda (tekan <strong>Start</strong>).
                    </p>
                    <p>
                      3. Dapatkan <strong>Chat ID</strong> Anda via <a href="https://t.me/userinfobot" target="_blank" rel="noreferrer" className="text-sky-600 dark:text-sky-400 underline">@userinfobot</a> (format angka positif untuk user, misal <code className="bg-white dark:bg-slate-900 px-1 py-0.5 rounded font-mono">12345678</code>, atau angka negatif untuk group chat, misal <code className="bg-white dark:bg-slate-900 px-1 py-0.5 rounded font-mono">-1001234567890</code>).
                    </p>
                  </div>
                ) : channelType === 'discord' ? (
                  <div className="text-[11px] text-slate-600 dark:text-slate-300 space-y-1.5 leading-relaxed">
                    <p>
                      1. Buka server Discord Anda &rarr; <strong>Server Settings</strong> &rarr; <strong>Integrations</strong> &rarr; <strong>Webhooks</strong>.
                    </p>
                    <p>
                      2. Klik <strong>New Webhook</strong>, pilih channel teks tujuan, lalu klik <strong>Copy Webhook URL</strong>.
                    </p>
                    <p>
                      Format URL:{' '}
                      <code className="bg-white dark:bg-slate-900 px-1 py-0.5 rounded font-mono text-[10px]">
                        https://discord.com/api/webhooks/123456789/abcdef...
                      </code>
                    </p>
                  </div>
                ) : (
                  <div className="text-[11px] text-slate-600 dark:text-slate-300 space-y-1.5 leading-relaxed">
                    <p>
                      Cuan akan mengirimkan HTTP POST JSON saat status deployment berganti menjadi <code className="font-mono text-red-500">firing</code> atau <code className="font-mono text-emerald-500">resolved</code>.
                    </p>
                    <p>
                      Jika Anda mengisi <strong>Secret Key</strong>, setiap request akan ditandatangani dengan header{' '}
                      <code className="bg-white dark:bg-slate-900 px-1 py-0.5 rounded font-mono">
                        X-Cuan-Signature: sha256=&lt;hex&gt;
                      </code>{' '}
                      untuk validasi otentikasi (kompatibel dengan Zapier, Make, n8n).
                    </p>
                  </div>
                )}
              </div>

              {/* Form Input: Channel Name */}
              <div className="space-y-1">
                <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                  Nama Channel <span className="text-red-500">*</span>
                </label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={
                    channelType === 'telegram'
                      ? 'contoh: Telegram DevOps Group'
                      : channelType === 'discord'
                      ? 'contoh: Discord Incident Alerts'
                      : 'contoh: N8N Outage Dispatcher'
                  }
                  required
                  className="h-9 text-xs"
                />
              </div>

              {/* Dynamic Inputs according to channelType */}
              {channelType === 'telegram' ? (
                <>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Telegram Bot Token <span className="text-red-500">*</span>
                    </label>
                    <Input
                      type="password"
                      value={botToken}
                      onChange={(e) => setBotToken(e.target.value)}
                      placeholder="123456789:ABCdefGhI_klm-NoPQRsTuvwxYZ"
                      required
                      className="h-9 text-xs font-mono"
                    />
                    <p className="text-[10px] text-slate-400">
                      Token disimpan terenkripsi dengan AES-GCM 256-bit dan tidak akan pernah diekspos plaintext.
                    </p>
                  </div>

                  <div className="space-y-1">
                    <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Chat ID / Group ID <span className="text-red-500">*</span>
                    </label>
                    <Input
                      value={chatId}
                      onChange={(e) => setChatId(e.target.value)}
                      placeholder="contoh: 987654321 atau -1001234567890"
                      required
                      className="h-9 text-xs font-mono"
                    />
                  </div>
                </>
              ) : channelType === 'discord' ? (
                <div className="space-y-1">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Discord Webhook URL <span className="text-red-500">*</span>
                  </label>
                  <Input
                    type="url"
                    value={webhookUrl}
                    onChange={(e) => setWebhookUrl(e.target.value)}
                    placeholder="https://discord.com/api/webhooks/..."
                    required
                    className="h-9 text-xs font-mono"
                  />
                  <p className="text-[10px] text-slate-400">
                    URL Webhook diamankan dengan enkripsi AES-GCM 256-bit at-rest.
                  </p>
                </div>
              ) : (
                <>
                  <div className="space-y-1">
                    <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Endpoint Webhook URL <span className="text-red-500">*</span>
                    </label>
                    <Input
                      type="url"
                      value={webhookUrl}
                      onChange={(e) => setWebhookUrl(e.target.value)}
                      placeholder="https://api.yourdomain.com/webhooks/cuan"
                      required
                      className="h-9 text-xs font-mono"
                    />
                  </div>

                  <div className="space-y-1">
                    <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Secret Key HMAC (Opsional)
                    </label>
                    <Input
                      type="password"
                      value={secret}
                      onChange={(e) => setSecret(e.target.value)}
                      placeholder="Kunci rahasia untuk tanda tangan HMAC SHA256"
                      className="h-9 text-xs font-mono"
                    />
                    <p className="text-[10px] text-slate-400">
                      Digunakan untuk menghasilkan header <code className="font-mono">X-Cuan-Signature</code>.
                    </p>
                  </div>
                </>
              )}

              {/* Action Buttons */}
              <div className="pt-2 flex items-center justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    resetForm()
                    setActiveTab('list')
                  }}
                  className="text-xs"
                >
                  Batal
                </Button>
                <Button
                  type="submit"
                  size="sm"
                  disabled={creating}
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs flex items-center gap-1.5"
                >
                  {creating ? (
                    <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <ShieldCheck className="h-3.5 w-3.5" />
                  )}
                  <span>{creating ? 'Mendaftarkan...' : 'Simpan Alert Channel'}</span>
                </Button>
              </div>
            </form>
          )}
        </div>
      </Card>
    </div>
  )
}
