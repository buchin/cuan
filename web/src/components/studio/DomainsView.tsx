import { useState, useEffect, useMemo } from 'react'
import { Card, CardHeader } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  Globe,
  Plus,
  RefreshCw,
  Search,
  AlertCircle,
  CheckCircle2,
  Clock,
  Trash2,
  ExternalLink,
  ShieldCheck,
  Shield,
  Cpu,
  Server,
  Sparkles,
  ChevronDown,
  ChevronUp,
  Copy,
  Check,
  X
} from 'lucide-react'
import {
  CustomDomainItem,
  DeploymentItem,
  CustomDomainInstructions,
  DomainVerifyResult
} from './types'
import { AddDomainModal } from './AddDomainModal'

interface DomainsViewProps {
  onNavigate?: (path: string) => void
  baseDomain?: string
  deployments?: DeploymentItem[]
  onOpenDeployWizard?: () => void
}

export function DomainsView({
  onNavigate: _onNavigate,
  baseDomain = 'cuan.dojo.cc',
  deployments: initialDeployments,
  onOpenDeployWizard
}: DomainsViewProps) {
  const [domains, setDomains] = useState<CustomDomainItem[]>([])
  const [deployments, setDeployments] = useState<DeploymentItem[]>(initialDeployments || [])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{
    type: 'success' | 'error' | 'info'
    message: string
  } | null>(null)

  // Search & Filters
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'verified' | 'pending' | 'failed'>('all')

  // Modals & Action States
  const [isAddModalOpen, setIsAddModalOpen] = useState(false)
  const [verifyingId, setVerifyingId] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<CustomDomainItem | null>(null)
  const [deleteLoading, setDeleteLoading] = useState(false)

  // DNS Guide expand per-domain
  const [expandedInstructionsId, setExpandedInstructionsId] = useState<string | null>(null)
  const [domainInstructions, setDomainInstructions] = useState<Record<string, CustomDomainInstructions>>({})
  const [loadingInstructionsId, setLoadingInstructionsId] = useState<string | null>(null)
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

  // Fetch all custom domains
  const fetchDomains = async () => {
    setError(null)
    try {
      const res = await fetch('/api/domains')
      if (!res.ok) {
        throw new Error(`Gagal memuat daftar domain (HTTP ${res.status})`)
      }
      const data = await res.json()
      setDomains(data.domains || [])
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan jaringan saat memuat custom domains.')
    } finally {
      setLoading(false)
    }
  }

  // Fetch deployments for mapping deployment info
  const fetchDeployments = async () => {
    try {
      const res = await fetch('/api/deployments')
      if (res.ok) {
        const data = await res.json()
        setDeployments(data.deployments || [])
      }
    } catch {
      // Ignore
    }
  }

  useEffect(() => {
    fetchDomains()
    fetchDeployments()
    const interval = setInterval(fetchDomains, 8000)
    return () => clearInterval(interval)
  }, [])

  // Action: Verify DNS Now
  const handleVerifyDns = async (domainItem: CustomDomainItem) => {
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
      await fetchDomains()
    } catch (err: any) {
      showToast('error', `Gagal memverifikasi DNS: ${err.message}`)
    } finally {
      setVerifyingId(null)
    }
  }

  // Action: Delete Domain
  const handleDeleteDomain = async () => {
    if (!deleteTarget) return
    setDeleteLoading(true)
    try {
      const res = await fetch(`/api/domains/${deleteTarget.id}`, {
        method: 'DELETE'
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }
      showToast('success', `Domain "${deleteTarget.domain}" berhasil dihapus.`)
      setDeleteTarget(null)
      await fetchDomains()
    } catch (err: any) {
      showToast('error', `Gagal menghapus domain: ${err.message}`)
    } finally {
      setDeleteLoading(false)
    }
  }

  // Toggle DNS Guide details
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

  // Format relative time
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

  // Statistics calculation
  const stats = useMemo(() => {
    const total = domains.length
    const verified = domains.filter((d) => d.dns_status === 'verified').length
    const pending = domains.filter((d) => d.dns_status === 'pending').length
    const sslActive = domains.filter((d) => d.ssl_status === 'active' || d.dns_status === 'verified').length
    return { total, verified, pending, sslActive }
  }, [domains])

  // Map deployment dictionary
  const deploymentMap = useMemo(() => {
    const map = new Map<string, DeploymentItem>()
    for (const dep of deployments) {
      map.set(dep.id, dep)
    }
    return map
  }, [deployments])

  // Filtered domains list
  const filteredDomains = useMemo(() => {
    return domains.filter((item) => {
      // Status filter
      if (statusFilter !== 'all' && item.dns_status !== statusFilter) {
        return false
      }

      // Search query
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase().trim()
        const matchDomain = item.domain.toLowerCase().includes(q)
        const dep = deploymentMap.get(item.deployment_id)
        const matchProject = dep?.project_name.toLowerCase().includes(q)
        const matchSlug = dep?.slug.toLowerCase().includes(q)
        if (!matchDomain && !matchProject && !matchSlug) {
          return false
        }
      }

      return true
    })
  }, [domains, statusFilter, searchQuery, deploymentMap])

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* Toast Notification */}
      {notification && (
        <div
          className={`p-4 rounded-xl flex items-center justify-between text-xs font-semibold shadow-md animate-in slide-in-from-top duration-200 ${
            notification.type === 'success'
              ? 'bg-emerald-500 text-white shadow-emerald-500/20'
              : notification.type === 'error'
              ? 'bg-red-500 text-white shadow-red-500/20'
              : 'bg-blue-600 text-white shadow-blue-600/20'
          }`}
        >
          <span>{notification.message}</span>
          <button onClick={() => setNotification(null)} className="opacity-80 hover:opacity-100">
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Error Alert */}
      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Kesalahan</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* 1. Stat Summary Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Total Custom Domain</p>
              <p className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white mt-0.5">
                {stats.total}
              </p>
            </div>
            <div className="h-10 w-10 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 flex items-center justify-center">
              <Globe className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">DNS Verified</p>
              <div className="flex items-center gap-2 mt-0.5">
                <p className="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">
                  {stats.verified}
                </p>
                {stats.verified > 0 && (
                  <span className="h-2.5 w-2.5 rounded-full bg-emerald-500 animate-pulse" />
                )}
              </div>
            </div>
            <div className="h-10 w-10 rounded-xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
              <CheckCircle2 className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Menunggu DNS</p>
              <p className="text-2xl font-bold tracking-tight text-amber-600 dark:text-amber-400 mt-0.5">
                {stats.pending}
              </p>
            </div>
            <div className="h-10 w-10 rounded-xl bg-amber-100 dark:bg-amber-950/60 text-amber-600 dark:text-amber-400 flex items-center justify-center">
              <Clock className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">On-Demand TLS Ready</p>
              <p className="text-2xl font-bold tracking-tight text-blue-600 dark:text-blue-400 mt-0.5">
                {stats.sslActive}
              </p>
            </div>
            <div className="h-10 w-10 rounded-xl bg-blue-100 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 flex items-center justify-center">
              <ShieldCheck className="h-5 w-5" />
            </div>
          </div>
        </Card>
      </div>

      {/* 2. Control & Search Header */}
      <div className="p-4 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xs flex flex-col md:flex-row items-stretch md:items-center justify-between gap-4">
        {/* Search Input */}
        <div className="relative flex-1">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
          <Input
            type="text"
            placeholder="Cari berdasarkan domain atau nama proyek..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 bg-slate-50 dark:bg-slate-950/60 border-slate-200 dark:border-slate-800 text-xs h-9.5"
          />
        </div>

        {/* Filter Buttons & Add Action */}
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex rounded-lg bg-slate-100 dark:bg-slate-800 p-0.5 text-xs">
            {(['all', 'verified', 'pending', 'failed'] as const).map((filter) => (
              <button
                key={filter}
                onClick={() => setStatusFilter(filter)}
                className={`px-3 py-1.5 rounded-md font-semibold capitalize transition-all ${
                  statusFilter === filter
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                }`}
              >
                {filter === 'all' ? 'Semua Status' : filter}
              </button>
            ))}
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              fetchDomains()
              fetchDeployments()
            }}
            className="h-9 text-xs flex items-center gap-1.5"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          <Button
            onClick={() => setIsAddModalOpen(true)}
            className="h-9 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 flex items-center gap-1.5"
          >
            <Plus className="h-4 w-4" />
            <span>Add Custom Domain</span>
          </Button>
        </div>
      </div>

      {/* 3. Domains List Content */}
      {loading && domains.length === 0 ? (
        <div className="h-64 flex flex-col items-center justify-center text-slate-400 gap-2 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800">
          <RefreshCw className="h-6 w-6 animate-spin text-emerald-500" />
          <span className="text-xs">Memuat data custom domains...</span>
        </div>
      ) : domains.length === 0 ? (
        /* Empty State */
        <div className="h-96 flex flex-col items-center justify-center text-center p-8 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-4">
          <div className="h-16 w-16 rounded-2xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center mx-auto shadow-inner">
            <Globe className="h-8 w-8" />
          </div>
          <div className="max-w-md space-y-1.5">
            <h3 className="text-base font-bold text-slate-900 dark:text-white">
              Belum ada Custom Domain terhubung
            </h3>
            <p className="text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
              Hubungkan domain atau subdomain pribadi milik Anda ke aplikasi yang telah di-deploy. Sistem Caddy On-Demand TLS akan menerbitkan sertifikat SSL Let's Encrypt secara otomatis tanpa downtime.
            </p>
          </div>
          <div className="pt-2 flex items-center gap-2">
            <Button
              onClick={() => setIsAddModalOpen(true)}
              className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 flex items-center gap-1.5"
            >
              <Plus className="h-4 w-4" />
              <span>Hubungkan Custom Domain Sekarang</span>
            </Button>
            {deployments.length === 0 && onOpenDeployWizard && (
              <Button
                variant="outline"
                onClick={onOpenDeployWizard}
                className="text-xs"
              >
                Deploy Aplikasi Terlebih Dahulu
              </Button>
            )}
          </div>
        </div>
      ) : filteredDomains.length === 0 ? (
        /* Filtered Empty */
        <div className="h-64 flex flex-col items-center justify-center text-center p-6 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-3">
          <Globe className="h-8 w-8 text-slate-400" />
          <div className="space-y-1 max-w-sm">
            <h4 className="text-sm font-bold text-slate-900 dark:text-white">
              Tidak ada domain yang cocok dengan pencarian
            </h4>
            <p className="text-xs text-slate-400">
              Ubah filter atau hapus kata kunci pencarian Anda.
            </p>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setSearchQuery('')
              setStatusFilter('all')
            }}
            className="text-xs"
          >
            Reset Filter
          </Button>
        </div>
      ) : (
        /* Domains Cards Grid / List */
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filteredDomains.map((cd) => {
            const dep = deploymentMap.get(cd.deployment_id)
            const isVerifying = verifyingId === cd.id
            const isExpanded = expandedInstructionsId === cd.id
            const instr = domainInstructions[cd.id]

            return (
              <Card
                key={cd.id}
                className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs hover:shadow-md transition-all flex flex-col justify-between overflow-hidden"
              >
                <CardHeader className="p-5 pb-3 space-y-3">
                  {/* Top Domain Info & Badges */}
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <a
                          href={`https://${cd.domain}`}
                          target="_blank"
                          rel="noreferrer"
                          className="font-bold text-slate-900 dark:text-white text-base hover:text-emerald-600 dark:hover:text-emerald-400 flex items-center gap-1.5 truncate"
                        >
                          <span className="truncate">{cd.domain}</span>
                          <ExternalLink className="h-3.5 w-3.5 text-slate-400 shrink-0" />
                        </a>
                      </div>
                      <p className="text-xs font-mono text-slate-400">ID: {cd.id}</p>
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0">
                      {cd.dns_status === 'verified' ? (
                        <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 gap-1 text-[11px] font-mono py-0.5 px-2">
                          <CheckCircle2 className="h-3 w-3" /> Verified
                        </Badge>
                      ) : cd.dns_status === 'failed' ? (
                        <Badge variant="destructive" className="gap-1 text-[11px] font-mono py-0.5 px-2">
                          <AlertCircle className="h-3 w-3" /> Failed
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="border-amber-400 text-amber-600 dark:text-amber-400 gap-1 text-[11px] font-mono py-0.5 px-2">
                          <Clock className="h-3 w-3" /> Pending DNS
                        </Badge>
                      )}

                      {cd.ssl_status === 'active' || cd.dns_status === 'verified' ? (
                        <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30 gap-1 text-[11px] font-mono py-0.5 px-2">
                          <ShieldCheck className="h-3 w-3" /> SSL Active
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="border-slate-300 dark:border-slate-700 text-slate-500 text-[11px] font-mono py-0.5 px-2">
                          <Shield className="h-3 w-3" /> SSL Pending
                        </Badge>
                      )}
                    </div>
                  </div>

                  {/* Linked Deployment Card */}
                  <div className="p-3 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-100 dark:border-slate-800/80 flex items-center justify-between text-xs">
                    <div className="min-w-0 pr-2">
                      <span className="text-[10px] text-slate-400 uppercase font-semibold block">Target Aplikasi</span>
                      <span className="font-semibold text-slate-800 dark:text-slate-200 truncate block">
                        {dep?.project_name || 'Deployment ' + cd.deployment_id}
                      </span>
                      <span className="text-slate-400 font-mono text-[11px]">/{dep?.slug || cd.deployment_id}</span>
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0">
                      {dep?.server_id ? (
                        <Badge variant="outline" className="border-indigo-400/60 bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 text-[10px] gap-1 font-mono">
                          <Server className="h-3 w-3" /> Remote VPS
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="border-emerald-400/60 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-[10px] gap-1 font-mono">
                          <Cpu className="h-3 w-3" /> Local Sandbox
                        </Badge>
                      )}
                      {dep?.internal_port ? (
                        <span className="font-mono text-xs text-slate-500 bg-white dark:bg-slate-900 px-1.5 py-0.5 rounded border border-slate-200 dark:border-slate-800">
                          :{dep.internal_port}
                        </span>
                      ) : null}
                    </div>
                  </div>

                  {/* Metadata: Verification token & last checked */}
                  <div className="flex items-center justify-between text-[11px] text-slate-400 pt-0.5">
                    <span className="truncate max-w-[200px]">
                      Pengecekan: {timeAgo(cd.last_checked_at)}
                    </span>
                    {cd.verification_token && (
                      <span className="font-mono text-[10px] text-slate-500 truncate">
                        Token: {cd.verification_token.slice(0, 10)}…
                      </span>
                    )}
                  </div>
                </CardHeader>

                {/* Collapsible DNS Setup Instructions */}
                {isExpanded && (
                  <div className="px-5 pb-3 animate-in fade-in duration-200">
                    <div className="p-3.5 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-200 dark:border-slate-800 space-y-2.5 text-xs">
                      <div className="flex items-center justify-between">
                        <span className="font-bold text-slate-800 dark:text-slate-200 flex items-center gap-1">
                          <Sparkles className="h-3.5 w-3.5 text-emerald-500" />
                          Instruksi DNS Provider
                        </span>
                        <span className="text-[10px] text-slate-400 font-mono">TTL: 300</span>
                      </div>

                      {loadingInstructionsId === cd.id ? (
                        <div className="p-3 text-center text-slate-400 text-xs">
                          <RefreshCw className="h-4 w-4 animate-spin mx-auto mb-1 text-emerald-500" />
                          <span>Memuat instruksi DNS...</span>
                        </div>
                      ) : (
                        <div className="space-y-2 font-mono text-[11px]">
                          {/* CNAME */}
                          <div className="p-2 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between gap-2">
                            <div className="min-w-0 pr-1">
                              <span className="text-[10px] text-slate-400 font-sans uppercase block">CNAME Target</span>
                              <span className="text-slate-500 font-sans">{cd.domain} → </span>
                              <span className="text-emerald-600 dark:text-emerald-400 font-bold">
                                {instr?.cname?.target || `${dep?.slug || cd.deployment_id}.${baseDomain}`}
                              </span>
                            </div>
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() =>
                                copyText(
                                  `cname_${cd.id}`,
                                  instr?.cname?.target || `${dep?.slug || cd.deployment_id}.${baseDomain}`
                                )
                              }
                              className="h-7 px-2 shrink-0"
                            >
                              {copiedKey === `cname_${cd.id}` ? (
                                <Check className="h-3 w-3 text-emerald-500" />
                              ) : (
                                <Copy className="h-3 w-3" />
                              )}
                            </Button>
                          </div>

                          {/* TXT Challenge */}
                          {cd.verification_token && (
                            <div className="p-2 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-750 flex items-center justify-between gap-2">
                              <div className="min-w-0 pr-1">
                                <span className="text-[10px] text-slate-400 font-sans uppercase block">TXT Token</span>
                                <span className="text-slate-400 font-sans">_cuan-challenge.{cd.domain}</span>
                                <div className="text-amber-600 dark:text-amber-400 truncate mt-0.5">
                                  {cd.verification_token}
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
                  </div>
                )}

                {/* Card Action Controls Footer */}
                <div className="p-4 pt-3 border-t border-slate-100 dark:border-slate-800/80 bg-slate-50/50 dark:bg-slate-900/50 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5">
                    <Button
                      size="sm"
                      onClick={() => handleVerifyDns(cd)}
                      disabled={isVerifying}
                      className="h-8 text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white flex items-center gap-1.5 shadow-sm"
                    >
                      {isVerifying ? (
                        <RefreshCw className="h-3 w-3 animate-spin" />
                      ) : (
                        <ShieldCheck className="h-3.5 w-3.5" />
                      )}
                      <span>Verify DNS Now</span>
                    </Button>

                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => toggleInstructions(cd)}
                      className="h-8 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center gap-1"
                    >
                      <span>DNS Guide</span>
                      {isExpanded ? (
                        <ChevronUp className="h-3 w-3" />
                      ) : (
                        <ChevronDown className="h-3 w-3" />
                      )}
                    </Button>
                  </div>

                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setDeleteTarget(cd)}
                    className="h-8 text-xs text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950 px-2.5 flex items-center gap-1"
                    title="Hapus domain"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    <span className="hidden sm:inline">Delete</span>
                  </Button>
                </div>
              </Card>
            )
          })}
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 animate-in fade-in duration-200">
          <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl max-w-md w-full p-6 space-y-4 animate-in zoom-in-95 duration-200">
            <div className="flex items-center gap-3 text-red-600 dark:text-red-400">
              <div className="h-10 w-10 rounded-xl bg-red-100 dark:bg-red-950/60 flex items-center justify-center shrink-0">
                <AlertCircle className="h-5 w-5" />
              </div>
              <div>
                <h3 className="font-bold text-slate-900 dark:text-white text-base">
                  Hapus Custom Domain
                </h3>
                <p className="text-xs text-slate-400">Konfirmasi pelepasan nama domain</p>
              </div>
            </div>

            <p className="text-xs text-slate-600 dark:text-slate-300 leading-relaxed">
              Apakah Anda yakin ingin menghapus domain{' '}
              <strong className="text-slate-900 dark:text-white font-mono">{deleteTarget.domain}</strong>? Rute reverse proxy Caddy akan dicabut dan pengunjung tidak akan dapat lagi mengakses aplikasi ini via domain tersebut.
            </p>

            <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setDeleteTarget(null)}
                disabled={deleteLoading}
                className="text-xs"
              >
                Batal
              </Button>
              <Button
                size="sm"
                onClick={handleDeleteDomain}
                disabled={deleteLoading}
                className="bg-red-600 hover:bg-red-500 text-white font-semibold text-xs flex items-center gap-1.5"
              >
                {deleteLoading ? (
                  <>
                    <RefreshCw className="h-3 w-3 animate-spin" />
                    <span>Menghapus...</span>
                  </>
                ) : (
                  <>
                    <Trash2 className="h-3.5 w-3.5" />
                    <span>Ya, Hapus Domain</span>
                  </>
                )}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Add Domain Modal */}
      <AddDomainModal
        isOpen={isAddModalOpen}
        onClose={() => setIsAddModalOpen(false)}
        onSuccess={() => {
          fetchDomains()
        }}
        deployments={deployments}
        baseDomain={baseDomain}
      />
    </div>
  )
}
