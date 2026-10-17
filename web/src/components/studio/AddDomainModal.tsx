import { useState, useId } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  Globe,
  Plus,
  RefreshCw,
  Check,
  Copy,
  AlertCircle,
  X,
  ShieldCheck,
  CheckCircle2,
  Clock,
  Sparkles
} from 'lucide-react'
import {
  DeploymentItem,
  CustomDomainItem,
  CustomDomainInstructions,
  CreateDomainResult,
  DomainVerifyResult
} from './types'

export function validateRFC1123Domain(domain: string): { valid: boolean; normalized: string; error?: string } {
  let d = domain.trim().toLowerCase()
  if (!d) {
    return { valid: false, normalized: '', error: 'Nama domain tidak boleh kosong' }
  }
  // Strip leading scheme if accidentally typed
  if (d.startsWith('http://') || d.startsWith('https://')) {
    d = d.replace(/^https?:\/\//, '')
  }
  // Strip trailing slash or path
  if (d.includes('/')) {
    d = d.split('/')[0]
  }
  // Strip port
  if (d.includes(':')) {
    d = d.split(':')[0]
  }

  if (d.includes(' ') || d.includes('@')) {
    return { valid: false, normalized: d, error: 'Domain tidak boleh mengandung spasi atau karakter "@"' }
  }

  if (d.length > 253) {
    return { valid: false, normalized: d, error: 'Panjang domain tidak boleh melebihi 253 karakter' }
  }

  const labels = d.split('.')
  if (labels.length < 2) {
    return { valid: false, normalized: d, error: 'Domain harus memiliki minimal nama dan ekstensi (misal: app.startup.com)' }
  }

  const labelRegex = /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$/
  for (const label of labels) {
    if (label.length === 0 || label.length > 63) {
      return { valid: false, normalized: d, error: `Label "${label}" harus antara 1 sampai 63 karakter` }
    }
    if (!labelRegex.test(label)) {
      return { valid: false, normalized: d, error: `Label "${label}" tidak valid atau diawali/diakhiri tanda minus (-)` }
    }
  }

  const tld = labels[labels.length - 1]
  if (/^\d+$/.test(tld)) {
    return { valid: false, normalized: d, error: 'Top-Level Domain (TLD) tidak boleh berupa angka saja' }
  }

  return { valid: true, normalized: d }
}

interface AddDomainModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess?: (domain: CustomDomainItem, instructions: CustomDomainInstructions) => void
  deployment?: DeploymentItem | null
  deployments?: DeploymentItem[]
  baseDomain?: string
}

export function AddDomainModal({
  isOpen,
  onClose,
  onSuccess,
  deployment,
  deployments = [],
  baseDomain = 'cuan.dojo.cc'
}: AddDomainModalProps) {
  if (!isOpen) return null

  const domainInputId = useId()
  const deploymentSelectId = useId()

  const [selectedDeploymentId, setSelectedDeploymentId] = useState<string>(
    deployment?.id || (deployments.length > 0 ? deployments[0].id : '')
  )
  const [domainInput, setDomainInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Step 2: Created state with instructions
  const [createdDomain, setCreatedDomain] = useState<CustomDomainItem | null>(null)
  const [instructions, setInstructions] = useState<CustomDomainInstructions | null>(null)

  // Verification state in Step 2
  const [verifying, setVerifying] = useState(false)
  const [verifyResult, setVerifyResult] = useState<DomainVerifyResult | null>(null)

  // Copied tracking
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  const validation = validateRFC1123Domain(domainInput)

  const activeDeployment = deployment || deployments.find((d) => d.id === selectedDeploymentId)

  const copyText = (key: string, text: string) => {
    navigator.clipboard.writeText(text)
    setCopiedKey(key)
    setTimeout(() => setCopiedKey(null), 2000)
  }

  const handleRegisterDomain = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (!activeDeployment) {
      setError('Pilih deployment target terlebih dahulu.')
      return
    }

    if (!validation.valid) {
      setError(validation.error || 'Format domain tidak valid.')
      return
    }

    setLoading(true)
    try {
      const res = await fetch(`/api/deployments/${activeDeployment.id}/domains`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain: validation.normalized })
      })

      const data: CreateDomainResult = await res.json()
      if (!res.ok) {
        throw new Error((data as any).error || `Gagal mendaftarkan domain (HTTP ${res.status})`)
      }

      setCreatedDomain(data.domain)
      setInstructions(data.instructions)
      if (onSuccess) {
        onSuccess(data.domain, data.instructions)
      }
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan saat mendaftarkan domain.')
    } finally {
      setLoading(false)
    }
  }

  const handleVerifyNow = async () => {
    if (!createdDomain) return
    setVerifying(true)
    try {
      const res = await fetch(`/api/domains/${createdDomain.id}/verify`, {
        method: 'POST'
      })
      const data: DomainVerifyResult = await res.json()
      setVerifyResult(data)
      if (data.domain) {
        setCreatedDomain(data.domain)
      }
    } catch (err: any) {
      setVerifyResult({
        status: 'failed',
        verified: false,
        dns_status: 'failed',
        domain: createdDomain,
        message: err.message || 'Gagal menghubungi DNS verification resolver'
      })
    } finally {
      setVerifying(false)
    }
  }

  const handleClose = () => {
    // Reset state on close
    setDomainInput('')
    setError(null)
    setCreatedDomain(null)
    setInstructions(null)
    setVerifyResult(null)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 animate-in fade-in duration-200">
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl max-w-2xl w-full max-h-[90vh] flex flex-col overflow-hidden animate-in zoom-in-95 duration-200">
        {/* Header */}
        <div className="p-5 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50/50 dark:bg-slate-900/50">
          <div className="flex items-center gap-2.5">
            <div className="h-9 w-9 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
              <Globe className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-base font-bold text-slate-900 dark:text-white">
                {createdDomain ? 'Panduan DNS Setup (Zero-Friction)' : 'Hubungkan Custom Domain (BYOD)'}
              </h2>
              <p className="text-xs text-slate-400">
                {createdDomain
                  ? `Konfigurasikan DNS record Anda untuk ${createdDomain.domain}`
                  : 'Gunakan nama domain kustom Anda sendiri dengan On-Demand TLS otomatis'}
              </p>
            </div>
          </div>
          <button
            onClick={handleClose}
            className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-6 overflow-y-auto space-y-5 flex-1">
          {error && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>Gagal Mendaftarkan Domain</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {!createdDomain ? (
            /* Step 1: Input Form */
            <form onSubmit={handleRegisterDomain} className="space-y-4">
              {/* Target Deployment Selector (if not preselected) */}
              {!deployment ? (
                <div className="space-y-1.5">
                  <label htmlFor={deploymentSelectId} className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Target Deployment / Proyek
                  </label>
                  <select
                    id={deploymentSelectId}
                    value={selectedDeploymentId}
                    onChange={(e) => setSelectedDeploymentId(e.target.value)}
                    className="w-full h-10 px-3 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-slate-100 text-sm focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    required
                  >
                    {deployments.length === 0 ? (
                      <option value="">Belum ada deployment aktif</option>
                    ) : (
                      deployments.map((d) => (
                        <option key={d.id} value={d.id}>
                          {d.project_name} (/{d.slug}) - {d.status}
                        </option>
                      ))
                    )}
                  </select>
                </div>
              ) : (
                <div className="p-3 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800 flex items-center justify-between text-xs">
                  <div>
                    <span className="text-slate-400 block text-[11px]">Target Deployment</span>
                    <span className="font-semibold text-slate-800 dark:text-slate-200">
                      {deployment.project_name}
                    </span>
                    <span className="text-slate-400 font-mono ml-1.5">/{deployment.slug}</span>
                  </div>
                  <Badge variant="outline" className="border-emerald-500/40 text-emerald-600 dark:text-emerald-400">
                    Port :{deployment.internal_port}
                  </Badge>
                </div>
              )}

              {/* Domain Input */}
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <label htmlFor={domainInputId} className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Nama Domain Kustom
                  </label>
                  <span className="text-[11px] text-slate-400 font-mono">RFC 1123 Compliant</span>
                </div>
                <Input
                  id={domainInputId}
                  type="text"
                  placeholder="misal: app.clientbrand.com atau myproject.id"
                  value={domainInput}
                  onChange={(e) => setDomainInput(e.target.value)}
                  className={`font-mono text-sm ${
                    domainInput && !validation.valid
                      ? 'border-red-500 focus-visible:ring-red-500'
                      : domainInput && validation.valid
                      ? 'border-emerald-500 focus-visible:ring-emerald-500'
                      : ''
                  }`}
                  autoFocus
                />
                {domainInput && !validation.valid && (
                  <p className="text-[11px] text-red-500 flex items-center gap-1">
                    <AlertCircle className="h-3 w-3 shrink-0" />
                    <span>{validation.error}</span>
                  </p>
                )}
                {domainInput && validation.valid && (
                  <p className="text-[11px] text-emerald-600 dark:text-emerald-400 flex items-center gap-1 font-mono">
                    <Check className="h-3 w-3 shrink-0" />
                    <span>Format valid: {validation.normalized}</span>
                  </p>
                )}
              </div>

              {/* Explanatory Info Card */}
              <div className="p-3.5 bg-emerald-50/60 dark:bg-emerald-950/20 rounded-xl border border-emerald-200 dark:border-emerald-800/60 space-y-1.5 text-xs text-emerald-900 dark:text-emerald-200">
                <div className="flex items-center gap-1.5 font-bold">
                  <Sparkles className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                  <span>On-Demand TLS & Auto DNS Routing</span>
                </div>
                <p className="text-[11px] leading-relaxed text-emerald-800 dark:text-emerald-300">
                  Setelah didaftarkan, sistem akan menyiapkan instruksi CNAME/TXT. Begitu DNS terverifikasi, reverse proxy Caddy secara instan menerbitkan sertifikat SSL Let's Encrypt saat kunjungan pertama ke domain Anda.
                </p>
              </div>

              <div className="pt-2 flex items-center justify-end gap-2">
                <Button type="button" variant="outline" onClick={handleClose} className="text-xs">
                  Batal
                </Button>
                <Button
                  type="submit"
                  disabled={loading || !domainInput || !validation.valid || !activeDeployment}
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 flex items-center gap-1.5"
                >
                  {loading ? (
                    <>
                      <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                      <span>Mendaftarkan Domain...</span>
                    </>
                  ) : (
                    <>
                      <Plus className="h-3.5 w-3.5" />
                      <span>Hubungkan Domain</span>
                    </>
                  )}
                </Button>
              </div>
            </form>
          ) : (
            /* Step 2: Zero-Friction DNS Setup Guide */
            <div className="space-y-5 animate-in fade-in duration-200">
              {/* Success Badge Banner */}
              <div className="p-3.5 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800 rounded-xl flex items-center justify-between gap-3">
                <div className="flex items-center gap-2.5">
                  <CheckCircle2 className="h-5 w-5 text-emerald-600 dark:text-emerald-400 shrink-0" />
                  <div>
                    <h4 className="text-xs font-bold text-emerald-900 dark:text-emerald-200">
                      Domain "{createdDomain.domain}" Berhasil Didaftarkan
                    </h4>
                    <p className="text-[11px] text-emerald-700 dark:text-emerald-400">
                      Tambahkan salah satu catatan DNS di bawah ini pada DNS Provider Anda (Cloudflare, Namecheap, Route53, dll).
                    </p>
                  </div>
                </div>
                <Badge variant="outline" className="border-amber-400 text-amber-600 dark:text-amber-400 text-[10px] uppercase font-mono">
                  DNS: {createdDomain.dns_status}
                </Badge>
              </div>

              {/* Instructions 1: CNAME Record (Recommended) */}
              {instructions?.cname && (
                <div className="p-4 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20 text-[10px] font-mono">
                        Opsi 1 (Rekomendasi Subdomain)
                      </Badge>
                      <span className="text-xs font-bold text-slate-800 dark:text-slate-200">
                        CNAME Record
                      </span>
                    </div>
                    <span className="text-[11px] text-slate-400">TTL: 300s</span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs font-mono">
                    <div className="p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between">
                      <div>
                        <span className="text-[10px] text-slate-400 uppercase font-sans block">Type</span>
                        <span className="font-bold text-slate-800 dark:text-slate-200">CNAME</span>
                      </div>
                    </div>
                    <div className="p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between">
                      <div className="min-w-0 pr-2">
                        <span className="text-[10px] text-slate-400 uppercase font-sans block">Host / Name</span>
                        <span className="font-bold text-slate-800 dark:text-slate-200 truncate block">
                          {instructions.cname.name}
                        </span>
                      </div>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => copyText('cname_name', instructions.cname.name)}
                        className="h-7 px-2 text-[11px] shrink-0"
                      >
                        {copiedKey === 'cname_name' ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>
                    <div className="sm:col-span-2 p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between">
                      <div className="min-w-0 pr-2">
                        <span className="text-[10px] text-slate-400 uppercase font-sans block">Target / Points to</span>
                        <span className="font-bold text-emerald-600 dark:text-emerald-400 truncate block">
                          {instructions.cname.target || `${activeDeployment?.slug}.${baseDomain}`}
                        </span>
                      </div>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() =>
                          copyText(
                            'cname_target',
                            instructions.cname.target || `${activeDeployment?.slug}.${baseDomain}`
                          )
                        }
                        className="h-7 px-2 text-[11px] shrink-0"
                      >
                        {copiedKey === 'cname_target' ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>
                  </div>
                </div>
              )}

              {/* Instructions 2: TXT Challenge Token */}
              {instructions?.txt && (
                <div className="p-4 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Badge className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20 text-[10px] font-mono">
                        Opsi 2 (Verifikasi Kepemilikan)
                      </Badge>
                      <span className="text-xs font-bold text-slate-800 dark:text-slate-200">
                        TXT Verification Record
                      </span>
                    </div>
                    <span className="text-[11px] text-slate-400">TTL: 300s</span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs font-mono">
                    <div className="p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between">
                      <div className="min-w-0 pr-2">
                        <span className="text-[10px] text-slate-400 uppercase font-sans block">Host / Record Name</span>
                        <span className="font-bold text-slate-800 dark:text-slate-200 truncate block">
                          {instructions.txt.name}
                        </span>
                      </div>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => copyText('txt_name', instructions.txt.name)}
                        className="h-7 px-2 text-[11px] shrink-0"
                      >
                        {copiedKey === 'txt_name' ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>

                    <div className="p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between">
                      <div className="min-w-0 pr-2">
                        <span className="text-[10px] text-slate-400 uppercase font-sans block">TXT Value / Token</span>
                        <span className="font-bold text-amber-600 dark:text-amber-400 truncate block">
                          {instructions.txt.value}
                        </span>
                      </div>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => copyText('txt_val', instructions.txt.value || '')}
                        className="h-7 px-2 text-[11px] shrink-0"
                      >
                        {copiedKey === 'txt_val' ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>
                  </div>
                </div>
              )}

              {/* Instructions 3: A Record Fallback */}
              {instructions?.a && (
                <div className="p-4 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Badge className="bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/20 text-[10px] font-mono">
                        Opsi 3 (Apex / Root Domain)
                      </Badge>
                      <span className="text-xs font-bold text-slate-800 dark:text-slate-200">
                        A Record Fallback
                      </span>
                    </div>
                    <span className="text-[11px] text-slate-400">TTL: 300s</span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs font-mono">
                    <div className="p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between">
                      <div className="min-w-0 pr-2">
                        <span className="text-[10px] text-slate-400 uppercase font-sans block">Host / Name</span>
                        <span className="font-bold text-slate-800 dark:text-slate-200 truncate block">
                          @ (atau {instructions.a.name})
                        </span>
                      </div>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => copyText('a_name', '@')}
                        className="h-7 px-2 text-[11px] shrink-0"
                      >
                        {copiedKey === 'a_name' ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>

                    <div className="p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between">
                      <div className="min-w-0 pr-2">
                        <span className="text-[10px] text-slate-400 uppercase font-sans block">Target IPv4</span>
                        <span className="font-bold text-slate-800 dark:text-slate-200 truncate block">
                          {instructions.a.target}
                        </span>
                      </div>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => copyText('a_target', instructions.a.target || '')}
                        className="h-7 px-2 text-[11px] shrink-0"
                      >
                        {copiedKey === 'a_target' ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>
                  </div>
                </div>
              )}

              {/* Dynamic Verification Result Banner */}
              {verifyResult && (
                <div
                  className={`p-4 rounded-xl border text-xs space-y-1.5 animate-in fade-in duration-200 ${
                    verifyResult.verified
                      ? 'bg-emerald-50 dark:bg-emerald-950/40 border-emerald-300 dark:border-emerald-800 text-emerald-900 dark:text-emerald-200'
                      : 'bg-amber-50 dark:bg-amber-950/40 border-amber-300 dark:border-amber-800 text-amber-900 dark:text-amber-200'
                  }`}
                >
                  <div className="flex items-center gap-2 font-bold">
                    {verifyResult.verified ? (
                      <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                    ) : (
                      <Clock className="h-4 w-4 text-amber-600 dark:text-amber-400" />
                    )}
                    <span>
                      {verifyResult.verified
                        ? 'Verifikasi DNS Sukses! Rute Caddy Aktif.'
                        : 'Propagasi DNS Belum Terdeteksi'}
                    </span>
                  </div>
                  <p className="text-[11px] leading-relaxed">
                    {verifyResult.message ||
                      (verifyResult.verified
                        ? 'Domain Anda kini terhubung dan siap melayani HTTPS traffic.'
                        : 'DNS resolver publik belum menemukan CNAME atau TXT token yang sesuai. Mohon tunggu 1–5 menit untuk propagasi DNS.')}
                  </p>
                </div>
              )}

              {/* Footer Actions */}
              <div className="pt-2 flex flex-col sm:flex-row items-center justify-between gap-3 border-t border-slate-100 dark:border-slate-800">
                <Button
                  type="button"
                  onClick={handleVerifyNow}
                  disabled={verifying}
                  className="w-full sm:w-auto h-9 bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs flex items-center justify-center gap-1.5 shadow-sm"
                >
                  {verifying ? (
                    <>
                      <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                      <span>Memverifikasi DNS...</span>
                    </>
                  ) : (
                    <>
                      <ShieldCheck className="h-4 w-4" />
                      <span>Verify DNS Now</span>
                    </>
                  )}
                </Button>

                <Button
                  type="button"
                  onClick={handleClose}
                  className="w-full sm:w-auto h-9 bg-slate-900 hover:bg-slate-800 dark:bg-white dark:hover:bg-slate-100 text-white dark:text-slate-900 font-semibold text-xs"
                >
                  Selesai & Tutup
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
