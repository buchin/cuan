import { useState, useEffect } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
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
  ExternalLink,
  ShieldCheck,
  CheckCircle2,
  Clock,
  Trash2,
  ChevronDown,
  ChevronUp,
  Sparkles
} from 'lucide-react'
import {
  DeploymentItem,
  CustomDomainItem,
  CustomDomainInstructions,
  DomainVerifyResult
} from './types'
import { AddDomainModal } from './AddDomainModal'

interface CustomDomainsModalProps {
  deployment: DeploymentItem | null
  isOpen: boolean
  onClose: () => void
  baseDomain?: string
  onDomainsUpdated?: () => void
}

export function CustomDomainsModal({
  deployment,
  isOpen,
  onClose,
  baseDomain = 'cuan.dojo.cc',
  onDomainsUpdated
}: CustomDomainsModalProps) {
  if (!isOpen || !deployment) return null

  const [domains, setDomains] = useState<CustomDomainItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{
    type: 'success' | 'error' | 'info'
    message: string
  } | null>(null)

  // Sub-modal state
  const [isAddModalOpen, setIsAddModalOpen] = useState(false)

  // Per-domain action states
  const [verifyingId, setVerifyingId] = useState<string | null>(null)
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null)
  const [deleteLoadingId, setDeleteLoadingId] = useState<string | null>(null)
  const [expandedInstructionsId, setExpandedInstructionsId] = useState<string | null>(null)
  const [domainInstructions, setDomainInstructions] = useState<Record<string, CustomDomainInstructions>>({})
  const [loadingInstructionsId, setLoadingInstructionsId] = useState<string | null>(null)

  // Copied indicator
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  const showToast = (type: 'success' | 'error' | 'info', message: string) => {
    setNotification({ type, message })
    setTimeout(() => setNotification(null), 4000)
  }

  const copyText = (key: string, text: string) => {
    navigator.clipboard.writeText(text)
    setCopiedKey(key)
    setTimeout(() => setCopiedKey(null), 2000)
  }

  const fetchDeploymentDomains = async () => {
    setError(null)
    try {
      const res = await fetch(`/api/deployments/${deployment.id}/domains`)
      if (!res.ok) {
        throw new Error(`Gagal memuat domain (HTTP ${res.status})`)
      }
      const data = await res.json()
      setDomains(data.domains || [])
    } catch (err: any) {
      setError(err.message || 'Gagal memuat custom domains untuk deployment ini.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen && deployment) {
      setLoading(true)
      fetchDeploymentDomains()
    }
  }, [isOpen, deployment?.id])

  // Fetch DNS instructions for a specific domain
  const toggleInstructions = async (domainItem: CustomDomainItem) => {
    if (expandedInstructionsId === domainItem.id) {
      setExpandedInstructionsId(null)
      return
    }

    setExpandedInstructionsId(domainItem.id)

    if (domainInstructions[domainItem.id]) {
      return
    }

    setLoadingInstructionsId(domainItem.id)
    try {
      const res = await fetch(`/api/domains/${domainItem.id}`)
      if (res.ok) {
        const data = await res.json()
        if (data.instructions) {
          setDomainInstructions((prev) => ({
            ...prev,
            [domainItem.id]: data.instructions
          }))
        }
      }
    } catch {
      // Ignore
    } finally {
      setLoadingInstructionsId(null)
    }
  }

  // Action: Verify DNS Now
  const handleVerify = async (domainItem: CustomDomainItem) => {
    setVerifyingId(domainItem.id)
    try {
      const res = await fetch(`/api/domains/${domainItem.id}/verify`, {
        method: 'POST'
      })
      const data: DomainVerifyResult = await res.json()

      if (data.verified) {
        showToast('success', `DNS untuk ${domainItem.domain} terverifikasi! Rute Caddy aktif.`)
      } else {
        showToast('info', data.message || `DNS untuk ${domainItem.domain} belum terdeteksi propagasi.`)
      }

      await fetchDeploymentDomains()
      if (onDomainsUpdated) onDomainsUpdated()
    } catch (err: any) {
      showToast('error', `Gagal memverifikasi DNS: ${err.message}`)
    } finally {
      setVerifyingId(null)
    }
  }

  // Action: Delete Domain
  const handleDeleteDomain = async (domainItem: CustomDomainItem) => {
    setDeleteLoadingId(domainItem.id)
    try {
      const res = await fetch(`/api/domains/${domainItem.id}`, {
        method: 'DELETE'
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }
      showToast('success', `Domain ${domainItem.domain} berhasil dihapus.`)
      setDeleteConfirmId(null)
      await fetchDeploymentDomains()
      if (onDomainsUpdated) onDomainsUpdated()
    } catch (err: any) {
      showToast('error', `Gagal menghapus domain: ${err.message}`)
    } finally {
      setDeleteLoadingId(null)
    }
  }

  const timeAgo = (dateStr?: string): string => {
    if (!dateStr) return 'Belum pernah dicek'
    try {
      const diffSec = Math.floor((new Date().getTime() - new Date(dateStr).getTime()) / 1000)
      if (diffSec < 60) return `${diffSec} detik lalu`
      if (diffSec < 3600) return `${Math.floor(diffSec / 60)} menit lalu`
      if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} jam lalu`
      return `${Math.floor(diffSec / 86400)} hari lalu`
    } catch {
      return dateStr
    }
  }

  const getDnsStatusBadge = (status: string) => {
    switch (status) {
      case 'verified':
        return (
          <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 gap-1 text-[11px] font-mono">
            <CheckCircle2 className="h-3 w-3" /> Verified
          </Badge>
        )
      case 'failed':
        return (
          <Badge variant="destructive" className="gap-1 text-[11px] font-mono">
            <AlertCircle className="h-3 w-3" /> Failed
          </Badge>
        )
      default:
        return (
          <Badge variant="outline" className="border-amber-400 text-amber-600 dark:text-amber-400 gap-1 text-[11px] font-mono">
            <Clock className="h-3 w-3" /> Pending
          </Badge>
        )
    }
  }

  const getSslStatusBadge = (status: string) => {
    switch (status) {
      case 'active':
        return (
          <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30 gap-1 text-[11px] font-mono">
            <ShieldCheck className="h-3 w-3" /> SSL Active
          </Badge>
        )
      case 'error':
      case 'failed':
        return (
          <Badge variant="destructive" className="gap-1 text-[11px] font-mono">
            <AlertCircle className="h-3 w-3" /> SSL Error
          </Badge>
        )
      default:
        return (
          <Badge variant="outline" className="border-slate-300 dark:border-slate-700 text-slate-500 text-[11px] font-mono">
            SSL Pending
          </Badge>
        )
    }
  }

  return (
    <>
      <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 animate-in fade-in duration-200">
        <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl max-w-2xl w-full max-h-[90vh] flex flex-col overflow-hidden animate-in zoom-in-95 duration-200">
          {/* Header */}
          <div className="p-5 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50/50 dark:bg-slate-900/50">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
                <Globe className="h-5 w-5" />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <h2 className="text-base font-bold text-slate-900 dark:text-white">
                    Custom Domains
                  </h2>
                  <Badge variant="outline" className="font-mono text-[10px] text-slate-400">
                    /{deployment.slug}
                  </Badge>
                </div>
                <p className="text-xs text-slate-400">
                  Kelola domain kustom untuk {deployment.project_name}
                </p>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Button
                size="sm"
                onClick={() => setIsAddModalOpen(true)}
                className="h-8 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-sm flex items-center gap-1.5"
              >
                <Plus className="h-3.5 w-3.5" />
                <span>Add Domain</span>
              </Button>
              <button
                onClick={onClose}
                className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
          </div>

          {/* Toast Notification */}
          {notification && (
            <div
              className={`p-3 text-xs font-semibold flex items-center justify-between animate-in slide-in-from-top ${
                notification.type === 'success'
                  ? 'bg-emerald-500 text-white'
                  : notification.type === 'error'
                  ? 'bg-red-500 text-white'
                  : 'bg-blue-600 text-white'
              }`}
            >
              <span>{notification.message}</span>
              <button onClick={() => setNotification(null)} className="opacity-80 hover:opacity-100">
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          )}

          {/* Error Alert */}
          {error && (
            <div className="p-4">
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle>Kesalahan</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            </div>
          )}

          {/* Domain List Body */}
          <div className="p-6 overflow-y-auto space-y-4 flex-1">
            {loading ? (
              <div className="h-48 flex flex-col items-center justify-center text-slate-400 gap-2">
                <RefreshCw className="h-5 w-5 animate-spin text-emerald-500" />
                <span className="text-xs">Memuat daftar domain...</span>
              </div>
            ) : domains.length === 0 ? (
              /* Empty State */
              <div className="h-64 flex flex-col items-center justify-center text-center p-6 rounded-2xl border-2 border-dashed border-slate-200 dark:border-slate-800 space-y-3">
                <div className="h-12 w-12 rounded-2xl bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 flex items-center justify-center mx-auto">
                  <Globe className="h-6 w-6" />
                </div>
                <div className="space-y-1 max-w-sm">
                  <h3 className="text-sm font-bold text-slate-900 dark:text-white">
                    Belum ada custom domain terhubung
                  </h3>
                  <p className="text-xs text-slate-400 leading-relaxed">
                    Hubungkan domain atau subdomain pribadi Anda (seperti <code className="text-emerald-600 dark:text-emerald-400">app.client.com</code>) ke aplikasi ini dengan On-Demand TLS otomatis.
                  </p>
                </div>
                <Button
                  size="sm"
                  onClick={() => setIsAddModalOpen(true)}
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-sm flex items-center gap-1.5 mt-2"
                >
                  <Plus className="h-3.5 w-3.5" />
                  <span>Hubungkan Domain Pertama</span>
                </Button>
              </div>
            ) : (
              /* List of Domains */
              domains.map((cd) => {
                const isVerifying = verifyingId === cd.id
                const isDeleting = deleteLoadingId === cd.id
                const isDeleteConfirm = deleteConfirmId === cd.id
                const isExpanded = expandedInstructionsId === cd.id
                const instr = domainInstructions[cd.id]

                return (
                  <Card
                    key={cd.id}
                    className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs space-y-3"
                  >
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                      <div className="space-y-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <a
                            href={`https://${cd.domain}`}
                            target="_blank"
                            rel="noreferrer"
                            className="text-base font-bold text-slate-900 dark:text-white hover:text-emerald-600 dark:hover:text-emerald-400 flex items-center gap-1.5 truncate"
                          >
                            <span>{cd.domain}</span>
                            <ExternalLink className="h-3.5 w-3.5 text-slate-400 shrink-0" />
                          </a>
                          {getDnsStatusBadge(cd.dns_status)}
                          {getSslStatusBadge(cd.ssl_status)}
                        </div>
                        <div className="flex items-center gap-2 text-[11px] text-slate-400 flex-wrap">
                          <span>Terakhir diperiksa: {timeAgo(cd.last_checked_at)}</span>
                          {cd.verification_token && (
                            <span className="font-mono text-slate-500 truncate max-w-xs">
                              Token: {cd.verification_token.slice(0, 12)}…
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Action buttons */}
                      <div className="flex items-center gap-1.5 self-end sm:self-center shrink-0">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => toggleInstructions(cd)}
                          className="h-8 text-xs font-semibold px-2.5 flex items-center gap-1"
                        >
                          <span>DNS Guide</span>
                          {isExpanded ? (
                            <ChevronUp className="h-3 w-3" />
                          ) : (
                            <ChevronDown className="h-3 w-3" />
                          )}
                        </Button>

                        <Button
                          size="sm"
                          onClick={() => handleVerify(cd)}
                          disabled={isVerifying}
                          className="h-8 text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white flex items-center gap-1"
                        >
                          {isVerifying ? (
                            <RefreshCw className="h-3 w-3 animate-spin" />
                          ) : (
                            <ShieldCheck className="h-3.5 w-3.5" />
                          )}
                          <span>Verify</span>
                        </Button>

                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setDeleteConfirmId(isDeleteConfirm ? null : cd.id)}
                          className="h-8 text-xs text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950 px-2"
                          title="Hapus domain"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </div>

                    {/* Delete Confirmation Warning */}
                    {isDeleteConfirm && (
                      <div className="p-3 bg-red-50 dark:bg-red-950/40 rounded-xl border border-red-200 dark:border-red-900 space-y-2 text-xs">
                        <div className="flex items-center gap-2 text-red-800 dark:text-red-300 font-semibold">
                          <AlertCircle className="h-4 w-4" />
                          <span>Hapus domain "{cd.domain}"?</span>
                        </div>
                        <p className="text-[11px] text-red-700 dark:text-red-400">
                          Rute reverse proxy Caddy akan dicabut. Pengunjung tidak akan dapat lagi mengakses aplikasi ini via domain tersebut.
                        </p>
                        <div className="flex items-center gap-2 pt-1">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => setDeleteConfirmId(null)}
                            className="h-7 text-xs"
                          >
                            Batal
                          </Button>
                          <Button
                            size="sm"
                            onClick={() => handleDeleteDomain(cd)}
                            disabled={isDeleting}
                            className="h-7 bg-red-600 hover:bg-red-500 text-white text-xs font-semibold"
                          >
                            {isDeleting ? 'Menghapus...' : 'Ya, Hapus'}
                          </Button>
                        </div>
                      </div>
                    )}

                    {/* Collapsible DNS Guide View */}
                    {isExpanded && (
                      <div className="p-3.5 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800 space-y-3 text-xs animate-in fade-in duration-200">
                        <div className="flex items-center justify-between">
                          <span className="font-bold text-slate-800 dark:text-slate-200 flex items-center gap-1.5">
                            <Sparkles className="h-3.5 w-3.5 text-emerald-500" />
                            Instruksi DNS Provider untuk {cd.domain}
                          </span>
                          <span className="text-[11px] text-slate-400">TTL: 300</span>
                        </div>

                        {loadingInstructionsId === cd.id ? (
                          <div className="p-4 text-center text-slate-400">
                            <RefreshCw className="h-4 w-4 animate-spin mx-auto mb-1 text-emerald-500" />
                            <span>Memuat konfigurasi DNS...</span>
                          </div>
                        ) : (
                          <div className="space-y-2 font-mono text-[11px]">
                            {/* CNAME */}
                            <div className="p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between gap-2">
                              <div>
                                <span className="text-[10px] text-slate-400 font-sans uppercase block">CNAME Host & Target</span>
                                <span className="font-bold text-slate-800 dark:text-slate-200">{cd.domain}</span>
                                <span className="text-slate-400 mx-1">→</span>
                                <span className="text-emerald-600 dark:text-emerald-400 font-bold">
                                  {instr?.cname?.target || `${deployment.slug}.${baseDomain}`}
                                </span>
                              </div>
                              <Button
                                size="sm"
                                variant="ghost"
                                onClick={() =>
                                  copyText(
                                    `cname_${cd.id}`,
                                    instr?.cname?.target || `${deployment.slug}.${baseDomain}`
                                  )
                                }
                                className="h-7 px-2"
                              >
                                {copiedKey === `cname_${cd.id}` ? (
                                  <Check className="h-3 w-3 text-emerald-500" />
                                ) : (
                                  <Copy className="h-3 w-3" />
                                )}
                              </Button>
                            </div>

                            {/* TXT */}
                            {cd.verification_token && (
                              <div className="p-2.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between gap-2">
                                <div className="min-w-0 pr-1">
                                  <span className="text-[10px] text-slate-400 font-sans uppercase block">TXT Challenge Token</span>
                                  <span className="text-slate-500 font-sans">Host: </span>
                                  <span className="text-slate-800 dark:text-slate-200">_cuan-challenge.{cd.domain}</span>
                                  <div className="text-amber-600 dark:text-amber-400 truncate mt-0.5">
                                    Token: {cd.verification_token}
                                  </div>
                                </div>
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  onClick={() => copyText(`txt_${cd.id}`, cd.verification_token || '')}
                                  className="h-7 px-2 shrink-0"
                                >
                                  {copiedKey === `txt_${cd.id}` ? (
                                    <Check className="h-3 w-3 text-emerald-500" />
                                  ) : (
                                    <Copy className="h-3 w-3" />
                                  )}
                                </Button>
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </Card>
                )
              })
            )}
          </div>

          {/* Footer */}
          <div className="p-4 border-t border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50/50 dark:bg-slate-900/50">
            <span className="text-xs text-slate-400">
              Total {domains.length} domain terhubung
            </span>
            <Button
              onClick={onClose}
              variant="outline"
              size="sm"
              className="text-xs"
            >
              Tutup
            </Button>
          </div>
        </div>
      </div>

      {/* Sub-modal: Add New Domain Form */}
      <AddDomainModal
        isOpen={isAddModalOpen}
        onClose={() => {
          setIsAddModalOpen(false)
          fetchDeploymentDomains()
          if (onDomainsUpdated) onDomainsUpdated()
        }}
        onSuccess={() => {
          fetchDeploymentDomains()
          if (onDomainsUpdated) onDomainsUpdated()
        }}
        deployment={deployment}
        baseDomain={baseDomain}
      />
    </>
  )
}
