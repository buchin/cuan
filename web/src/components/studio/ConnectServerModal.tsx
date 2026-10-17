import { useState, FormEvent } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import { Badge } from '../ui/badge'
import {
  Server,
  Key,
  ShieldCheck,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  X,
  Copy,
  Check,
  Terminal,
  Activity
} from 'lucide-react'
import { ServerItem, ServerSpecs, TestServerResult } from './types'

interface ConnectServerModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: (server: ServerItem) => void
}

const CUAN_DEFAULT_PUBLIC_KEY =
  'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIChM4vK7yG3jT9e1XqW4bVpL0mZnRsKuQ8wY2xZdF cuan-orchestrator@dojo.cc'

export function ConnectServerModal({
  isOpen,
  onClose,
  onSuccess
}: ConnectServerModalProps) {
  // Form fields
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState<number>(22)
  const [username, setUsername] = useState('ubuntu')
  const [authType, setAuthType] = useState<'paste_key' | 'public_key'>('paste_key')
  const [privateKey, setPrivateKey] = useState('')

  // Testing & Connecting State
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isTesting, setIsTesting] = useState(false)
  const [testPhase, setTestPhase] = useState<string | null>(null)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [createdServer, setCreatedServer] = useState<ServerItem | null>(null)
  const [testResult, setTestResult] = useState<{
    success: boolean
    status: string
    specs?: ServerSpecs
    error?: string
  } | null>(null)

  // Copy helper
  const [copiedSnippet, setCopiedSnippet] = useState(false)

  if (!isOpen) return null

  const handleCopySnippet = () => {
    const snippet = `mkdir -p ~/.ssh && chmod 700 ~/.ssh\necho "${CUAN_DEFAULT_PUBLIC_KEY}" >> ~/.ssh/authorized_keys\nchmod 600 ~/.ssh/authorized_keys`
    navigator.clipboard.writeText(snippet)
    setCopiedSnippet(true)
    setTimeout(() => setCopiedSnippet(false), 2500)
  }

  // Validate form
  const validate = (): boolean => {
    setErrorMessage(null)
    if (!name.trim()) {
      setErrorMessage('Nama server wajib diisi.')
      return false
    }
    if (!host.trim()) {
      setErrorMessage('Host atau IP remote server wajib diisi.')
      return false
    }
    if (!username.trim()) {
      setErrorMessage('Username SSH (misal: ubuntu atau root) wajib diisi.')
      return false
    }
    if (authType === 'paste_key' && !privateKey.trim()) {
      setErrorMessage('Harap tempelkan Private Key SSH.')
      return false
    }
    return true
  }

  // Create server record via POST /api/servers
  const createServerRecord = async (): Promise<ServerItem> => {
    const payload = {
      name: name.trim(),
      host: host.trim(),
      port: Number(port) || 22,
      username: username.trim(),
      auth_type: 'key',
      private_key: privateKey.trim()
    }

    const res = await fetch('/api/servers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })

    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      throw new Error(err.error || `Gagal mendaftarkan server (HTTP ${res.status})`)
    }

    const data = await res.json()
    return data.server as ServerItem
  }

  // Interactive Live Test Probe via POST /api/servers/{id}/test
  const runLiveProbe = async (targetServer: ServerItem) => {
    setIsTesting(true)
    setTestPhase('Melakukan koneksi SSH handshake...')
    setTestResult(null)

    try {
      setTestPhase(`Menghubungkan ke ${targetServer.username}@${targetServer.host}:${targetServer.port}...`)
      const res = await fetch(`/api/servers/${targetServer.id}/test`, {
        method: 'POST'
      })

      const data: TestServerResult = await res.json().catch(() => ({
        success: false,
        status: 'error',
        error: `HTTP ${res.status}`,
        server: targetServer
      }))

      if (data.success) {
        setTestResult({
          success: true,
          status: 'online',
          specs: data.specs || data.server.os_info
        })
        setCreatedServer(data.server)
      } else {
        setTestResult({
          success: false,
          status: data.status || 'offline',
          error: data.error || 'Gagal memvalidasi koneksi remote host.'
        })
        if (data.server) {
          setCreatedServer(data.server)
        }
      }
    } catch (err: any) {
      setTestResult({
        success: false,
        status: 'error',
        error: err.message || 'Koneksi jaringan terputus saat menguji server.'
      })
    } finally {
      setIsTesting(false)
      setTestPhase(null)
    }
  }

  // Triggered by "Test Connection" button
  const handleTestConnection = async () => {
    if (!validate()) return

    setIsSubmitting(true)
    setErrorMessage(null)

    try {
      let target = createdServer
      if (!target) {
        setTestPhase('Mendaftarkan server ke KeyVault...')
        target = await createServerRecord()
        setCreatedServer(target)
      }

      await runLiveProbe(target)
    } catch (err: any) {
      setErrorMessage(err.message || 'Terjadi kesalahan saat memproses server.')
    } finally {
      setIsSubmitting(false)
    }
  }

  // Triggered by "Connect Server" primary submit
  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!validate()) return

    // If already created and tested successfully, close with success
    if (createdServer && testResult?.success) {
      onSuccess(createdServer)
      return
    }

    setIsSubmitting(true)
    setErrorMessage(null)

    try {
      let target = createdServer
      if (!target) {
        setTestPhase('Mendaftarkan kredensial server terenkripsi...')
        target = await createServerRecord()
        setCreatedServer(target)
      }

      // Run live test
      await runLiveProbe(target)

      // Notify caller
      onSuccess(target)
    } catch (err: any) {
      setErrorMessage(err.message || 'Gagal menghubungkan remote server.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 overflow-y-auto animate-in fade-in duration-200">
      <div className="w-full max-w-2xl bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl overflow-hidden my-8">
        {/* Header */}
        <div className="p-6 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-indigo-100 dark:bg-indigo-950/60 text-indigo-600 dark:text-indigo-400 flex items-center justify-center">
              <Server className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-xl font-bold text-slate-900 dark:text-white">Connect Remote Server</h2>
                <Badge variant="outline" className="border-indigo-500/40 text-indigo-600 dark:text-indigo-400 text-[10px] font-mono">
                  BYOS
                </Badge>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Hubungkan VPS/Server Linux Anda via SSH untuk target eksekusi aplikasi Cuan.
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

        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {/* Error Banner */}
          {errorMessage && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>Gagal Memproses Server</AlertTitle>
              <AlertDescription>{errorMessage}</AlertDescription>
            </Alert>
          )}

          {/* Basic Connection Details */}
          <div className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                Nama Server <span className="text-red-500">*</span>
              </label>
              <Input
                type="text"
                placeholder="Contoh: Production SG-1 / Worker VPS"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
              <p className="text-[11px] text-slate-400">Nama alias pengenal server di Deploy Wizard Cuan.</p>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div className="sm:col-span-2 space-y-1.5">
                <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                  Host / IP Address <span className="text-red-500">*</span>
                </label>
                <Input
                  type="text"
                  placeholder="194.233.85.12 atau vps.domain.com"
                  value={host}
                  onChange={(e) => setHost(e.target.value)}
                  className="font-mono text-sm"
                  required
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                  SSH Port
                </label>
                <Input
                  type="number"
                  placeholder="22"
                  value={port}
                  onChange={(e) => setPort(Number(e.target.value) || 22)}
                  className="font-mono text-sm"
                  min={1}
                  max={65535}
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                SSH Username <span className="text-red-500">*</span>
              </label>
              <Input
                type="text"
                placeholder="ubuntu atau root"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="font-mono text-sm"
                required
              />
              <p className="text-[11px] text-slate-400">User dengan akses SSH dan permission eksekusi aplikasi.</p>
            </div>
          </div>

          {/* Authentication Method Selection */}
          <div className="space-y-3 pt-2 border-t border-slate-100 dark:border-slate-800">
            <label className="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
              <Key className="h-3.5 w-3.5 text-slate-400" />
              Metode Autentikasi SSH
            </label>

            <div className="grid grid-cols-2 gap-3">
              <button
                type="button"
                onClick={() => setAuthType('paste_key')}
                className={`p-3 rounded-xl border text-left transition-all ${
                  authType === 'paste_key'
                    ? 'border-indigo-500 bg-indigo-50/50 dark:bg-indigo-950/30 ring-1 ring-indigo-500'
                    : 'border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700'
                }`}
              >
                <Key className={`h-4 w-4 mb-1.5 ${authType === 'paste_key' ? 'text-indigo-600 dark:text-indigo-400' : 'text-slate-400'}`} />
                <p className="text-xs font-semibold text-slate-900 dark:text-white">Paste Private Key</p>
                <p className="text-[10px] text-slate-400">Gunakan SSH Key milik Anda sendiri</p>
              </button>

              <button
                type="button"
                onClick={() => setAuthType('public_key')}
                className={`p-3 rounded-xl border text-left transition-all ${
                  authType === 'public_key'
                    ? 'border-indigo-500 bg-indigo-50/50 dark:bg-indigo-950/30 ring-1 ring-indigo-500'
                    : 'border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700'
                }`}
              >
                <Terminal className={`h-4 w-4 mb-1.5 ${authType === 'public_key' ? 'text-indigo-600 dark:text-indigo-400' : 'text-slate-400'}`} />
                <p className="text-xs font-semibold text-slate-900 dark:text-white">Salin Cuan Public Key</p>
                <p className="text-[10px] text-slate-400">Tambahkan ke authorized_keys</p>
              </button>
            </div>

            {/* Sub-tab 1: Paste Private Key */}
            {authType === 'paste_key' && (
              <div className="space-y-2 pt-1">
                <div className="flex items-center justify-between">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    OpenSSH / RSA Private Key <span className="text-red-500">*</span>
                  </label>
                  <span className="text-[10px] text-slate-400 flex items-center gap-1 font-mono">
                    <ShieldCheck className="h-3 w-3 text-emerald-500" /> AES-256-GCM Vault
                  </span>
                </div>
                <textarea
                  rows={4}
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
                  value={privateKey}
                  onChange={(e) => setPrivateKey(e.target.value)}
                  className="w-full rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 p-3 font-mono text-xs text-slate-800 dark:text-slate-200 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  required
                />
                <p className="text-[11px] text-slate-400 flex items-center gap-1">
                  <ShieldCheck className="h-3.5 w-3.5 text-emerald-500 shrink-0" />
                  Kunci privat dienkripsi at-rest dan tidak pernah dibocorkan ke API publik.
                </p>
              </div>
            )}

            {/* Sub-tab 2: Copy Public Key instructions */}
            {authType === 'public_key' && (
              <div className="space-y-3 pt-1">
                <div className="p-3 bg-slate-50 dark:bg-slate-800/60 rounded-xl border border-slate-200 dark:border-slate-700/80 space-y-2">
                  <div className="flex items-center justify-between">
                    <p className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                      Jalankan perintah ini di VPS Anda:
                    </p>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={handleCopySnippet}
                      className="h-7 text-xs px-2 gap-1"
                    >
                      {copiedSnippet ? (
                        <>
                          <Check className="h-3 w-3 text-emerald-500" /> Tersalin
                        </>
                      ) : (
                        <>
                          <Copy className="h-3 w-3" /> Salin Perintah
                        </>
                      )}
                    </Button>
                  </div>
                  <pre className="p-2.5 bg-slate-900 rounded-lg text-emerald-400 text-xs font-mono overflow-x-auto whitespace-pre">
{`mkdir -p ~/.ssh && chmod 700 ~/.ssh
echo "${CUAN_DEFAULT_PUBLIC_KEY}" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys`}
                  </pre>
                  <p className="text-[11px] text-slate-500 dark:text-slate-400">
                    Setelah menambahkan public key di atas, tempelkan private key yang sesuai di bawah ini:
                  </p>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Private Key yang bersesuaian:
                  </label>
                  <textarea
                    rows={3}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY-----..."
                    value={privateKey}
                    onChange={(e) => setPrivateKey(e.target.value)}
                    className="w-full rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 p-3 font-mono text-xs text-slate-800 dark:text-slate-200 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>
              </div>
            )}
          </div>

          {/* Test Probe Live Progress & Results */}
          {(isTesting || testPhase || testResult) && (
            <div className="space-y-2 pt-2 border-t border-slate-100 dark:border-slate-800">
              {/* Testing Spinner */}
              {isTesting && (
                <div className="p-3.5 bg-indigo-50/70 dark:bg-indigo-950/30 rounded-xl border border-indigo-200 dark:border-indigo-800/80 flex items-center gap-3">
                  <RefreshCw className="h-4 w-4 text-indigo-600 dark:text-indigo-400 animate-spin shrink-0" />
                  <div className="text-xs">
                    <p className="font-semibold text-indigo-900 dark:text-indigo-200">
                      Menguji Konektivitas Server...
                    </p>
                    <p className="text-[11px] text-indigo-700 dark:text-indigo-400">{testPhase}</p>
                  </div>
                </div>
              )}

              {/* Test Probe Result Card */}
              {testResult && !isTesting && (
                <div
                  className={`p-4 rounded-xl border ${
                    testResult.success
                      ? 'bg-emerald-50/80 dark:bg-emerald-950/20 border-emerald-300 dark:border-emerald-800/60'
                      : 'bg-red-50/80 dark:bg-red-950/20 border-red-300 dark:border-red-800/60'
                  }`}
                >
                  <div className="flex items-start gap-3">
                    {testResult.success ? (
                      <CheckCircle2 className="h-5 w-5 text-emerald-600 dark:text-emerald-400 shrink-0 mt-0.5" />
                    ) : (
                      <AlertCircle className="h-5 w-5 text-red-600 dark:text-red-400 shrink-0 mt-0.5" />
                    )}

                    <div className="space-y-2 flex-1">
                      <div className="flex items-center justify-between">
                        <p
                          className={`text-xs font-bold ${
                            testResult.success
                              ? 'text-emerald-800 dark:text-emerald-300'
                              : 'text-red-800 dark:text-red-300'
                          }`}
                        >
                          {testResult.success
                            ? 'Koneksi SSH Berhasil Terverifikasi!'
                            : 'Gagal Menghubungi Remote Host'}
                        </p>
                        <Badge
                          variant={testResult.success ? 'success' : 'destructive'}
                          className="text-[10px] py-0 px-2"
                        >
                          {testResult.status.toUpperCase()}
                        </Badge>
                      </div>

                      {testResult.success && testResult.specs && (
                        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 pt-1 text-xs">
                          <div className="p-2 bg-white dark:bg-slate-900 rounded-lg border border-emerald-200 dark:border-emerald-900">
                            <span className="text-[10px] text-slate-400 block">Sistem Operasi</span>
                            <span className="font-semibold text-slate-700 dark:text-slate-200">
                              {testResult.specs.distro || testResult.specs.os || 'Linux'}
                            </span>
                          </div>
                          <div className="p-2 bg-white dark:bg-slate-900 rounded-lg border border-emerald-200 dark:border-emerald-900">
                            <span className="text-[10px] text-slate-400 block">CPU Cores</span>
                            <span className="font-semibold text-slate-700 dark:text-slate-200">
                              {testResult.specs.cores || 1} vCPU
                            </span>
                          </div>
                          <div className="p-2 bg-white dark:bg-slate-900 rounded-lg border border-emerald-200 dark:border-emerald-900">
                            <span className="text-[10px] text-slate-400 block">RAM Kapasitas</span>
                            <span className="font-semibold text-slate-700 dark:text-slate-200">
                              {testResult.specs.ram_mb
                                ? `${(testResult.specs.ram_mb / 1024).toFixed(1)} GB`
                                : '-'}
                            </span>
                          </div>
                          <div className="p-2 bg-white dark:bg-slate-900 rounded-lg border border-emerald-200 dark:border-emerald-900">
                            <span className="text-[10px] text-slate-400 block">SSH Latency</span>
                            <span className="font-semibold text-emerald-600 dark:text-emerald-400">
                              {testResult.specs.ping_ms || 1} ms
                            </span>
                          </div>
                        </div>
                      )}

                      {!testResult.success && (
                        <p className="text-xs text-red-700 dark:text-red-400 font-mono">
                          {testResult.error}
                        </p>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Modal Actions */}
          <div className="flex flex-col sm:flex-row items-center justify-between gap-3 pt-4 border-t border-slate-100 dark:border-slate-800">
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={isSubmitting || isTesting}
              className="w-full sm:w-auto text-xs"
            >
              Batal
            </Button>

            <div className="flex items-center gap-2 w-full sm:w-auto">
              {/* Standalone Interactive Test Connection Button */}
              <Button
                type="button"
                variant="outline"
                onClick={handleTestConnection}
                disabled={isSubmitting || isTesting}
                className="w-full sm:w-auto text-xs border-indigo-200 dark:border-indigo-800 text-indigo-700 dark:text-indigo-300 hover:bg-indigo-50 dark:hover:bg-indigo-950 flex items-center justify-center gap-1.5"
              >
                {isTesting ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Activity className="h-3.5 w-3.5" />
                )}
                <span>{isTesting ? 'Menguji...' : 'Test Connection'}</span>
              </Button>

              {/* Primary Connect / Finish Button */}
              <Button
                type="submit"
                disabled={isSubmitting || isTesting}
                className="w-full sm:w-auto bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-md shadow-indigo-600/20 flex items-center justify-center gap-1.5"
              >
                {isSubmitting ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : testResult?.success ? (
                  <Check className="h-3.5 w-3.5" />
                ) : (
                  <Server className="h-3.5 w-3.5" />
                )}
                <span>
                  {isSubmitting
                    ? 'Menghubungkan...'
                    : testResult?.success
                    ? 'Simpan & Selesai'
                    : 'Connect Server'}
                </span>
              </Button>
            </div>
          </div>
        </form>
      </div>
    </div>
  )
}
