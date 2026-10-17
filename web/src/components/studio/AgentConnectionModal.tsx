import { useState, useEffect, FormEvent } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  KeyRound,
  ShieldCheck,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  ExternalLink,
  Trash2,
  Eye,
  EyeOff,
  Sparkles,
  Zap,
  Lock,
  Cpu,
  X
} from 'lucide-react'
import { AgentConnectionDTO, ConnectionProbeResult } from './types'

interface AgentConnectionModalProps {
  isOpen: boolean
  onClose: () => void
  onConnectionUpdated?: (connection: AgentConnectionDTO | null) => void
}

export function AgentConnectionModal({
  isOpen,
  onClose,
  onConnectionUpdated,
}: AgentConnectionModalProps) {
  const [connection, setConnection] = useState<AgentConnectionDTO | null>(null)
  const [loading, setLoading] = useState(false)
  const [verifying, setVerifying] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)
  const [probeResult, setProbeResult] = useState<ConnectionProbeResult | null>(null)

  // Form states
  const [apiKeyInput, setApiKeyInput] = useState('')
  const [showApiKey, setShowApiKey] = useState(false)
  const [isUpdatingKey, setIsUpdatingKey] = useState(false)

  // Fetch current Google Gemini connection
  const fetchConnection = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/agent-connections?provider=google')
      if (res.ok) {
        const data = await res.json()
        const conns: AgentConnectionDTO[] = data.connections || []
        if (conns.length > 0) {
          setConnection(conns[0])
          onConnectionUpdated?.(conns[0])
        } else {
          setConnection(null)
          onConnectionUpdated?.(null)
        }
      }
    } catch (err: any) {
      console.error('Failed to fetch agent connections:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      fetchConnection()
      setApiKeyInput('')
      setShowApiKey(false)
      setIsUpdatingKey(false)
      setError(null)
      setSuccessMessage(null)
      setProbeResult(null)
    }
  }, [isOpen])

  // Save new connection
  const handleSaveConnection = async (e: FormEvent) => {
    e.preventDefault()
    if (!apiKeyInput.trim()) {
      setError('Google Gemini API Key wajib diisi.')
      return
    }

    setSaving(true)
    setError(null)
    setSuccessMessage(null)

    try {
      const res = await fetch('/api/agent-connections', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider: 'google',
          api_key: apiKeyInput.trim(),
          verify_now: true,
        }),
      })

      const data = await res.json()

      if (!res.ok) {
        const errMsg = data.error || (data.probe && data.probe.error_message) || 'Gagal menyimpan dan memverifikasi koneksi Gemini API'
        if (data.probe) {
          setProbeResult(data.probe)
        }
        throw new Error(errMsg)
      }

      setConnection(data.connection)
      if (data.probe) {
        setProbeResult(data.probe)
      }
      onConnectionUpdated?.(data.connection)

      // Zero credential leakage: clear plaintext key immediately from memory
      setApiKeyInput('')
      setIsUpdatingKey(false)
      setSuccessMessage('Kredensial Google Gemini API berhasil dienkripsi (AES-256-GCM) dan diverifikasi aktif!')
      setTimeout(() => setSuccessMessage(null), 4000)
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan saat memvalidasi API Key.')
    } finally {
      setSaving(false)
    }
  }

  // Verify existing connection
  const handleVerifyExisting = async () => {
    if (!connection) return
    setVerifying(true)
    setError(null)
    setSuccessMessage(null)

    try {
      const res = await fetch(`/api/agent-connections/${connection.id}/verify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      })

      const data = await res.json()
      if (data.probe) {
        setProbeResult(data.probe)
      }

      if (data.verified && data.connection) {
        setConnection(data.connection)
        onConnectionUpdated?.(data.connection)
        setSuccessMessage('Verifikasi kuota dan status model Google Gemini Pro 100% SUKSES!')
        setTimeout(() => setSuccessMessage(null), 4000)
      } else {
        const errMsg = data.error || (data.probe && data.probe.error_message) || 'Verifikasi koneksi gagal.'
        throw new Error(errMsg)
      }
    } catch (err: any) {
      setError(err.message || 'Gagal memverifikasi koneksi Gemini API.')
    } finally {
      setVerifying(false)
    }
  }

  // Disconnect / Delete connection
  const handleDeleteConnection = async () => {
    if (!connection) return
    if (!confirm('Apakah Anda yakin ingin memutuskan dan menghapus koneksi Google Gemini API? Seluruh enkripsi kredensial akan dihapus permanen dari server.')) {
      return
    }

    setLoading(true)
    setError(null)

    try {
      const res = await fetch(`/api/agent-connections/${connection.id}`, {
        method: 'DELETE',
      })

      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal menghapus koneksi.')
      }

      setConnection(null)
      setProbeResult(null)
      setIsUpdatingKey(false)
      onConnectionUpdated?.(null)
      setSuccessMessage('Koneksi Google Gemini API berhasil dihapus.')
      setTimeout(() => setSuccessMessage(null), 3000)
    } catch (err: any) {
      setError(err.message || 'Gagal menghapus koneksi.')
    } finally {
      setLoading(false)
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/70 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Modal Header */}
        <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between bg-slate-50 dark:bg-slate-950/50">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-blue-500 to-indigo-600 text-white flex items-center justify-center shadow-md shadow-blue-500/20">
              <KeyRound className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="text-base font-bold text-slate-900 dark:text-white">
                  Google Gemini Pro AI Connection
                </h3>
                <Badge variant="outline" className="border-blue-400 text-blue-600 dark:text-blue-400 text-[10px] py-0 px-1.5 font-mono">
                  BYOK
                </Badge>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Kredensial LLM dienkripsi simetris (AES-256-GCM) di server Cuan.
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Modal Content */}
        <div className="p-6 overflow-y-auto space-y-5 flex-1">
          {/* Notifications */}
          {error && (
            <Alert variant="destructive" className="py-2.5">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle className="text-xs font-bold">Validasi Gagal</AlertTitle>
              <AlertDescription className="text-xs">{error}</AlertDescription>
            </Alert>
          )}

          {successMessage && (
            <div className="p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-xl flex items-center gap-2.5 text-emerald-600 dark:text-emerald-400 text-xs font-semibold animate-in fade-in">
              <CheckCircle2 className="h-4 w-4 shrink-0" />
              <span>{successMessage}</span>
            </div>
          )}

          {/* Active Connection Status Card (Zero Credential Leakage) */}
          {connection && !isUpdatingKey ? (
            <div className="p-4 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="h-3 w-3 rounded-full bg-emerald-500 animate-pulse" />
                  <span className="text-xs font-bold text-slate-900 dark:text-slate-100 uppercase tracking-wider">
                    Koneksi Aktif Terverifikasi
                  </span>
                </div>
                <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 text-[10px] font-mono">
                  {connection.status.toUpperCase()}
                </Badge>
              </div>

              {/* Zero Credential Display */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                <div className="p-2.5 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800">
                  <span className="text-slate-400 text-[11px] block">Provider AI</span>
                  <span className="font-semibold text-slate-800 dark:text-slate-200 flex items-center gap-1.5 mt-0.5">
                    <Sparkles className="h-3.5 w-3.5 text-blue-500" />
                    Google Gemini Pro / Flash
                  </span>
                </div>

                <div className="p-2.5 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800">
                  <span className="text-slate-400 text-[11px] block">Enkripsi Kredensial</span>
                  <span className="font-mono text-emerald-600 dark:text-emerald-400 font-semibold flex items-center gap-1.5 mt-0.5">
                    <Lock className="h-3.5 w-3.5" />
                    AES-256-GCM Vault ••••••••
                  </span>
                </div>

                <div className="p-2.5 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 sm:col-span-2">
                  <span className="text-slate-400 text-[11px] block">Plan / Kuota Hint</span>
                  <span className="font-medium text-slate-700 dark:text-slate-300 mt-0.5 block">
                    {connection.plan_hint || 'Google Gemini Developer API (Active Quota)'}
                  </span>
                </div>
              </div>

              {/* Probe / Models Information */}
              {probeResult && (
                <div className="p-3 rounded-lg bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800/60 space-y-2 text-xs">
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-blue-700 dark:text-blue-300 flex items-center gap-1.5">
                      <Zap className="h-3.5 w-3.5 text-blue-500" />
                      Status Health Probe: Valid
                    </span>
                    <span className="font-mono text-[10px] text-blue-500">
                      Latency: {probeResult.latency_ms}ms
                    </span>
                  </div>

                  {probeResult.available_models && probeResult.available_models.length > 0 && (
                    <div>
                      <span className="text-[11px] text-slate-500 dark:text-slate-400 block mb-1">
                        Model Tersedia untuk Agent Persona:
                      </span>
                      <div className="flex flex-wrap gap-1.5">
                        {probeResult.available_models.map((m) => (
                          <Badge
                            key={m.id}
                            variant="outline"
                            className="text-[10px] font-mono border-blue-300 dark:border-blue-700/60 text-blue-700 dark:text-blue-300 bg-white/60 dark:bg-slate-900/60"
                          >
                            <Cpu className="h-2.5 w-2.5 mr-1 text-blue-500" />
                            {m.name || m.id}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* Actions */}
              <div className="flex items-center justify-between pt-2 gap-2 flex-wrap">
                <div className="flex items-center gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleVerifyExisting}
                    disabled={verifying}
                    className="h-8 text-xs gap-1.5"
                  >
                    <RefreshCw className={`h-3 w-3 ${verifying ? 'animate-spin' : ''}`} />
                    <span>Uji Status Kuota</span>
                  </Button>

                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => setIsUpdatingKey(true)}
                    className="h-8 text-xs text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white"
                  >
                    Ganti API Key
                  </Button>
                </div>

                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={handleDeleteConnection}
                  disabled={loading}
                  className="h-8 text-xs text-red-500 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-950/30 gap-1"
                >
                  <Trash2 className="h-3 w-3" />
                  <span>Putuskan Koneksi</span>
                </Button>
              </div>
            </div>
          ) : (
            /* Input / Update API Key Form */
            <form onSubmit={handleSaveConnection} className="space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300 flex items-center justify-between">
                  <span>Google Gemini Pro API Key</span>
                  <a
                    href="https://aistudio.google.com/app/apikey"
                    target="_blank"
                    rel="noreferrer"
                    className="text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1 font-normal text-[11px]"
                  >
                    <span>Dapatkan API Key di Google AI Studio</span>
                    <ExternalLink className="h-3 w-3" />
                  </a>
                </label>

                <div className="relative">
                  <Input
                    type={showApiKey ? 'text' : 'password'}
                    placeholder="AIzaSy..."
                    value={apiKeyInput}
                    onChange={(e) => setApiKeyInput(e.target.value)}
                    className="font-mono text-xs pr-10"
                    autoFocus
                  />
                  <button
                    type="button"
                    onClick={() => setShowApiKey(!showApiKey)}
                    className="absolute right-3 top-2.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                    title={showApiKey ? 'Sembunyikan' : 'Tampilkan'}
                  >
                    {showApiKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <p className="text-[11px] text-slate-400">
                  Key ini digunakan oleh Agent Cuan untuk inference model (gemini-1.5-pro, gemini-1.5-flash). Key tidak pernah ditampilkan kembali setelah disimpan.
                </p>
              </div>

              {/* Zero Credential Security Guarantee Banner */}
              <div className="p-3 rounded-xl bg-slate-100 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700/60 flex items-start gap-2.5 text-xs text-slate-600 dark:text-slate-300">
                <ShieldCheck className="h-4 w-4 text-emerald-500 shrink-0 mt-0.5" />
                <div className="space-y-0.5">
                  <span className="font-semibold text-slate-900 dark:text-slate-100 block">
                    Zero Credential Leakage Guarantee
                  </span>
                  <p className="text-[11px] text-slate-500 dark:text-slate-400">
                    Kredensial disimpan dengan enkripsi simetris AES-GCM 256-bit di KeyVault backend. API Publik hanya menyajikan status <code className="font-mono text-emerald-500">has_credentials: true</code> tanpa plaintext secret.
                  </p>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="flex items-center justify-end gap-2 pt-2">
                {isUpdatingKey && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      setIsUpdatingKey(false)
                      setApiKeyInput('')
                      setError(null)
                    }}
                    className="h-9 text-xs"
                  >
                    Batal
                  </Button>
                )}

                <Button
                  type="submit"
                  disabled={saving || !apiKeyInput.trim()}
                  className="h-9 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs gap-1.5 shadow-md shadow-emerald-600/20"
                >
                  {saving ? (
                    <>
                      <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                      <span>Memverifikasi & Menyimpan...</span>
                    </>
                  ) : (
                    <>
                      <CheckCircle2 className="h-3.5 w-3.5" />
                      <span>Simpan & Verifikasi Koneksi</span>
                    </>
                  )}
                </Button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
