import { useState, useEffect, useMemo } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  ShieldCheck,
  CheckCircle2,
  AlertCircle,
  Ban,
  RefreshCw,
  Search,
  ExternalLink,
  Mail,
  Phone,
  Clock,
  Coins,
  History,
  X,
  ArrowUpRight,
  ShieldAlert,
  Globe
} from 'lucide-react'

export interface SponsorAdminItem {
  id: string
  batch_id: string
  brand_name: string
  website_url: string
  slogan: string
  logo_path: string
  contact_email: string
  contact_wa: string
  status: 'pending_review' | 'approved' | 'rejected'
  rejection_reason?: string
  amount_idr: number
  paid_at?: string
  created_at: string
  updated_at: string
}

export interface ActivityItem {
  id: number
  sponsor_id: string
  order_id?: string
  action: string
  details: string
  actor: string
  created_at: string
}

interface AdminSponsorPanelProps {
  token: string
  onNavigate: (path: string) => void
  onNotify?: (msg: string) => void
  ownerEmail?: string
}

export function AdminSponsorPanel({ token, onNavigate, onNotify, ownerEmail }: AdminSponsorPanelProps) {
  const [sponsors, setSponsors] = useState<SponsorAdminItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [activeFilter, setActiveFilter] = useState<'pending_review' | 'approved' | 'rejected' | 'all'>('pending_review')
  const [searchQuery, setSearchQuery] = useState('')
  const [counts, setCounts] = useState({ pending: 0, approved: 0, rejected: 0, total: 0 })

  // Action states
  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // Reject Modal state
  const [rejectingSponsor, setRejectingSponsor] = useState<SponsorAdminItem | null>(null)
  const [rejectReason, setRejectReason] = useState('')
  const [rejectSubmitting, setRejectSubmitting] = useState(false)
  const [syncingReadme, setSyncingReadme] = useState(false)

  // Audit Trail Modal state
  const [activityModalSponsor, setActivityModalSponsor] = useState<SponsorAdminItem | null>(null)
  const [activities, setActivities] = useState<ActivityItem[]>([])
  const [activitiesLoading, setActivitiesLoading] = useState(false)

  const presetReasons = [
    'Terindikasi Situs Judi Online / Slot / Kasino (Blacklist Judol)',
    'Melanggar Etika dan Panduan Integritas Komunitas Cuan',
    'Konten Dewasa / Pornografi / Aktivitas Ilegal',
    'URL Website Palsu / Phishing / Terindikasi Penipuan',
    'Logo atau Materi Mengandung Unsur Hak Cipta Ilegal'
  ]

  const fetchSponsors = async () => {
    setLoading(true)
    setError(null)
    try {
      const url = `/api/admin/sponsors?status=${activeFilter}`
      const res = await fetch(url, {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        }
      })
      if (!res.ok) {
        if (res.status === 401) {
          throw new Error('Sesi autentikasi telah berakhir. Silakan login kembali.')
        }
        throw new Error(`Gagal memuat daftar kurasi: kode respons HTTP ${res.status}`)
      }
      const data = await res.json()
      setSponsors(data.sponsors || [])
      setCounts({
        pending: data.pending_count ?? 0,
        approved: data.approved_count ?? 0,
        rejected: data.rejected_count ?? 0,
        total: data.total_count ?? 0
      })
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan jaringan')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchSponsors()
  }, [activeFilter])

  const handleApprove = async (sp: SponsorAdminItem) => {
    setActionLoadingId(sp.id)
    try {
      const res = await fetch(`/api/admin/sponsors/${sp.id}/approve`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        }
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.message || 'Gagal menyetujui sponsor')
      }
      const brand = sp.brand_name
      const msg = `Sponsor "${brand}" berhasil disetujui! Logo langsung live di grid /sponsor.`
      setNotification({ type: 'success', message: msg })
      if (onNotify) onNotify(msg)
      await fetchSponsors()
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Gagal menyetujui sponsor' })
    } finally {
      setActionLoadingId(null)
    }
  }

  const handleOpenRejectModal = (sp: SponsorAdminItem) => {
    setRejectingSponsor(sp)
    setRejectReason(presetReasons[0])
  }

  const handleConfirmReject = async () => {
    if (!rejectingSponsor) return
    setRejectSubmitting(true)
    try {
      const res = await fetch(`/api/admin/sponsors/${rejectingSponsor.id}/reject`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ reason: rejectReason })
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.message || 'Gagal menolak sponsor')
      }
      const brand = rejectingSponsor.brand_name || (rejectingSponsor as any).BrandName
      const msg = `Sponsor "${brand}" berhasil ditolak & dikarantina anti-judol. Alasan tersimpan di audit trail.`
      setNotification({ type: 'success', message: msg })
      if (onNotify) onNotify(msg)
      setRejectingSponsor(null)
      await fetchSponsors()
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Gagal menolak sponsor' })
    } finally {
      setRejectSubmitting(false)
    }
  }

  const handleManualSyncReadme = async () => {
    setSyncingReadme(true)
    try {
      const res = await fetch('/api/admin/sponsors/sync-readme', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Gagal sinkronisasi README')
      }
      const count = data.sponsors_count || 0
      const msg = `README.md & direktori arsip /sponsors berhasil disinkronkan (${count} sponsor aktif).`
      setNotification({ type: 'success', message: msg })
      if (onNotify) onNotify(msg)
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Gagal sinkronisasi README' })
    } finally {
      setSyncingReadme(false)
    }
  }

  const handleViewActivities = async (sp: SponsorAdminItem) => {
    setActivityModalSponsor(sp)
    setActivitiesLoading(true)
    try {
      const res = await fetch(`/api/admin/sponsors/${sp.id}/activities`, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      if (res.ok) {
        const data = await res.json()
        setActivities(data.activities || [])
      } else {
        setActivities([])
      }
    } catch {
      setActivities([])
    } finally {
      setActivitiesLoading(false)
    }
  }

  const filteredSponsors = useMemo(() => {
    if (!searchQuery.trim()) return sponsors
    const q = searchQuery.toLowerCase()
    return sponsors.filter(s => {
      const name = (s.brand_name || (s as any).BrandName || '').toLowerCase()
      const url = (s.website_url || (s as any).WebsiteURL || '').toLowerCase()
      const email = (s.contact_email || (s as any).ContactEmail || '').toLowerCase()
      const wa = (s.contact_wa || (s as any).ContactWA || '').toLowerCase()
      const slogan = (s.slogan || (s as any).Slogan || '').toLowerCase()
      return name.includes(q) || url.includes(q) || email.includes(q) || wa.includes(q) || slogan.includes(q)
    })
  }, [sponsors, searchQuery])

  const formatIDR = (num?: number) => {
    const val = num || 0
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      maximumFractionDigits: 0
    }).format(val)
  }

  const formatDate = (iso?: string) => {
    if (!iso) return 'Belum tercatat'
    try {
      const d = new Date(iso)
      return d.toLocaleDateString('id-ID', {
        day: 'numeric',
        month: 'short',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      })
    } catch {
      return iso
    }
  }

  return (
    <div className="space-y-6">
      {/* Top Banner / Heading */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 bg-white p-6 rounded-xl border border-slate-200 shadow-sm">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="p-1.5 bg-emerald-100 text-emerald-800 rounded-lg">
              <ShieldCheck className="h-5 w-5" />
            </span>
            <h2 className="text-xl font-bold tracking-tight text-slate-900">
              Panel Kurasi & Moderasi Sponsor Anti-Judol
            </h2>
            <Badge variant="outline" className="text-xs bg-emerald-50 text-emerald-700 border-emerald-200">
              Single-Owner: {ownerEmail || 'Terautentikasi'}
            </Badge>
          </div>
          <p className="text-xs text-slate-600 max-w-2xl">
            Tinjau order terbayar dan kurasi profil calon sponsor. Penolakan bermodal anti-judol menjamin 100% brand
            judi online dan pornografi tercegah tampil di endpoint publik atau direktori GitHub Cuan.
          </p>
        </div>

        <div className="flex items-center gap-2.5">
          <Button
            variant="outline"
            size="sm"
            onClick={() => onNavigate('/sponsor')}
            className="text-xs gap-1.5 border-emerald-300 text-emerald-800 hover:bg-emerald-50"
          >
            <Globe className="h-3.5 w-3.5 text-emerald-600" />
            Lihat Grid Live (/sponsor)
            <ArrowUpRight className="h-3.5 w-3.5 opacity-60" />
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={fetchSponsors}
            disabled={loading}
            className="text-xs gap-1.5"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh Data
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={handleManualSyncReadme}
            disabled={syncingReadme}
            className="text-xs gap-1.5 border-blue-300 text-blue-800 hover:bg-blue-50"
          >
            <RefreshCw className={`h-3.5 w-3.5 text-blue-600 ${syncingReadme ? 'animate-spin' : ''}`} />
            Sync README & Arsip
          </Button>
        </div>
      </div>

      {/* Notifications */}
      {notification && (
        <Alert
          variant={notification.type === 'error' ? 'destructive' : 'default'}
          className={notification.type === 'success' ? 'bg-emerald-50 text-emerald-900 border-emerald-200' : ''}
        >
          {notification.type === 'success' ? (
            <CheckCircle2 className="h-4 w-4 text-emerald-600" />
          ) : (
            <AlertCircle className="h-4 w-4" />
          )}
          <AlertTitle className="font-semibold text-xs">
            {notification.type === 'success' ? 'Aksi Berhasil' : 'Perhatian'}
          </AlertTitle>
          <AlertDescription className="text-xs flex items-center justify-between">
            <span>{notification.message}</span>
            <button
              onClick={() => setNotification(null)}
              className="text-slate-500 hover:text-slate-900 ml-4 font-bold"
            >
              ✕
            </button>
          </AlertDescription>
        </Alert>
      )}

      {/* Filter Tabs and Search Bar */}
      <div className="flex flex-col sm:flex-row justify-between items-stretch sm:items-center gap-3">
        {/* Filter Tabs */}
        <div className="flex flex-wrap items-center gap-1.5 bg-slate-100 p-1 rounded-lg border border-slate-200">
          <button
            onClick={() => setActiveFilter('pending_review')}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md transition-all ${
              activeFilter === 'pending_review'
                ? 'bg-white text-slate-900 shadow-sm'
                : 'text-slate-600 hover:text-slate-900'
            }`}
          >
            <Clock className="h-3.5 w-3.5 text-amber-500" />
            Pending Review
            <Badge
              variant="outline"
              className={`ml-1 text-[11px] px-1.5 py-0 border-0 ${
                counts.pending > 0
                  ? 'bg-amber-100 text-amber-900 font-bold'
                  : 'bg-slate-200 text-slate-600'
              }`}
            >
              {counts.pending}
            </Badge>
          </button>

          <button
            onClick={() => setActiveFilter('approved')}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md transition-all ${
              activeFilter === 'approved'
                ? 'bg-white text-slate-900 shadow-sm'
                : 'text-slate-600 hover:text-slate-900'
            }`}
          >
            <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
            Approved
            <Badge
              variant="outline"
              className="ml-1 text-[11px] px-1.5 py-0 border-0 bg-emerald-100 text-emerald-900"
            >
              {counts.approved}
            </Badge>
          </button>

          <button
            onClick={() => setActiveFilter('rejected')}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md transition-all ${
              activeFilter === 'rejected'
                ? 'bg-white text-slate-900 shadow-sm'
                : 'text-slate-600 hover:text-slate-900'
            }`}
          >
            <Ban className="h-3.5 w-3.5 text-rose-500" />
            Rejected
            <Badge
              variant="outline"
              className="ml-1 text-[11px] px-1.5 py-0 border-0 bg-rose-100 text-rose-900"
            >
              {counts.rejected}
            </Badge>
          </button>

          <button
            onClick={() => setActiveFilter('all')}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md transition-all ${
              activeFilter === 'all'
                ? 'bg-white text-slate-900 shadow-sm'
                : 'text-slate-600 hover:text-slate-900'
            }`}
          >
            Semua
            <Badge
              variant="outline"
              className="ml-1 text-[11px] px-1.5 py-0 border-0 bg-slate-200 text-slate-700"
            >
              {counts.total}
            </Badge>
          </button>
        </div>

        {/* Search Input */}
        <div className="relative min-w-[240px]">
          <Search className="h-3.5 w-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <Input
            placeholder="Cari brand, URL, atau email..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-8 text-xs bg-white h-9"
          />
        </div>
      </div>

      {/* Main Content Area */}
      {error ? (
        <Card className="bg-red-50 border-red-200 p-6 text-center">
          <CardContent className="space-y-3">
            <AlertCircle className="h-8 w-8 text-red-600 mx-auto" />
            <p className="text-sm font-medium text-red-900">{error}</p>
            <Button size="sm" onClick={fetchSponsors} variant="outline" className="text-xs">
              Coba Lagi
            </Button>
          </CardContent>
        </Card>
      ) : loading && sponsors.length === 0 ? (
        <Card className="bg-white p-12 text-center border-slate-200">
          <CardContent className="space-y-3">
            <RefreshCw className="h-8 w-8 text-emerald-600 animate-spin mx-auto" />
            <p className="text-xs text-slate-500 font-medium">Memuat data antrean kurasi sponsor...</p>
          </CardContent>
        </Card>
      ) : filteredSponsors.length === 0 ? (
        <Card className="bg-white p-12 text-center border-dashed border-2 border-slate-200">
          <CardContent className="space-y-3">
            {activeFilter === 'pending_review' ? (
              <>
                <div className="inline-flex p-3 bg-emerald-50 text-emerald-600 rounded-full mb-1">
                  <CheckCircle2 className="h-8 w-8" />
                </div>
                <h3 className="text-base font-bold text-slate-800">Antrean Kurasi Bersih!</h3>
                <p className="text-xs text-slate-500 max-w-md mx-auto">
                  Tidak ada order terbayar yang menunggu peninjauan owner. Seluruh transaksi sponsor telah
                  dimoderasi dengan aman.
                </p>
              </>
            ) : (
              <>
                <Search className="h-8 w-8 text-slate-400 mx-auto" />
                <h3 className="text-base font-bold text-slate-800">Tidak ada sponsor ditemukan</h3>
                <p className="text-xs text-slate-500">
                  {searchQuery ? `Tidak ada hasil pencarian untuk "${searchQuery}".` : 'Belum ada data pada kategori ini.'}
                </p>
              </>
            )}
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {filteredSponsors.map((sp) => {
            const brand = sp.brand_name || (sp as any).BrandName
            const url = sp.website_url || (sp as any).WebsiteURL
            const slogan = sp.slogan || (sp as any).Slogan
            const logoPath = sp.logo_path || (sp as any).LogoPath
            const email = sp.contact_email || (sp as any).ContactEmail
            const wa = sp.contact_wa || (sp as any).ContactWA
            const status = sp.status || (sp as any).Status
            const reason = sp.rejection_reason || (sp as any).RejectionReason
            const amount = sp.amount_idr || (sp as any).AmountIDR || 0
            const paidAt = sp.paid_at || (sp as any).PaidAt
            const isActing = actionLoadingId === sp.id

            const logoSrc = logoPath
              ? logoPath.startsWith('http') || logoPath.startsWith('/uploads')
                ? logoPath
                : `/${logoPath}`
              : null

            return (
              <Card
                key={sp.id}
                className={`bg-white border transition-all shadow-sm ${
                  status === 'pending_review'
                    ? 'border-amber-200 hover:border-amber-300'
                    : status === 'rejected'
                    ? 'border-rose-200 bg-rose-50/20'
                    : 'border-slate-200 hover:border-emerald-300'
                }`}
              >
                <div className="p-5 flex flex-col md:flex-row gap-5 items-start justify-between">
                  {/* Brand & Logo Preview */}
                  <div className="flex gap-4 items-start flex-1 min-w-0">
                    {/* Logo container */}
                    <div className="w-16 h-16 rounded-xl border border-slate-200 bg-slate-50 flex items-center justify-center p-2 shrink-0 overflow-hidden shadow-xs">
                      {logoSrc ? (
                        <img
                          src={logoSrc}
                          alt={brand}
                          className="w-full h-full object-contain"
                          onError={(e) => {
                            // Fallback to text avatar
                            (e.target as HTMLElement).style.display = 'none'
                          }}
                        />
                      ) : (
                        <span className="text-xl font-black text-slate-400">
                          {brand ? brand.charAt(0).toUpperCase() : 'S'}
                        </span>
                      )}
                    </div>

                    {/* Metadata details */}
                    <div className="space-y-1.5 min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-base font-bold text-slate-900 truncate">
                          {brand}
                        </h3>

                        {/* Status Badges */}
                        {status === 'pending_review' && (
                          <Badge className="bg-amber-100 text-amber-800 border-amber-300 gap-1 text-[11px] font-semibold">
                            <Clock className="h-3 w-3 animate-spin" />
                            Pending Review
                          </Badge>
                        )}
                        {status === 'approved' && (
                          <Badge className="bg-emerald-100 text-emerald-800 border-emerald-300 gap-1 text-[11px] font-semibold">
                            <CheckCircle2 className="h-3 w-3" />
                            Approved & Live di /sponsor
                          </Badge>
                        )}
                        {status === 'rejected' && (
                          <Badge className="bg-rose-100 text-rose-800 border-rose-300 gap-1 text-[11px] font-semibold">
                            <Ban className="h-3 w-3" />
                            Ditolak (Blacklist Judol)
                          </Badge>
                        )}

                        <span className="text-xs font-mono text-slate-400">#{sp.id}</span>
                      </div>

                      {slogan && (
                        <p className="text-xs text-slate-600 italic">
                          "{slogan}"
                        </p>
                      )}

                      {/* Info Chips */}
                      <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs text-slate-600 pt-1">
                        {url && (
                          <a
                            href={url.startsWith('http') ? url : `https://${url}`}
                            target="_blank"
                            rel="nofollow noopener noreferrer"
                            className="inline-flex items-center gap-1 text-emerald-700 hover:text-emerald-900 hover:underline font-medium"
                          >
                            <ExternalLink className="h-3 w-3" />
                            {url.replace(/^https?:\/\//, '')}
                          </a>
                        )}

                        {email && (
                          <a
                            href={`mailto:${email}`}
                            className="inline-flex items-center gap-1 text-slate-600 hover:text-slate-900"
                          >
                            <Mail className="h-3 w-3 text-slate-400" />
                            {email}
                          </a>
                        )}

                        {wa && (
                          <a
                            href={`https://wa.me/${wa.replace(/[^0-9]/g, '')}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 text-slate-600 hover:text-slate-900"
                          >
                            <Phone className="h-3 w-3 text-slate-400" />
                            {wa}
                          </a>
                        )}
                      </div>

                      {/* Rejection Reason Notice */}
                      {status === 'rejected' && reason && (
                        <div className="mt-2 p-2.5 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-900 flex items-start gap-2">
                          <ShieldAlert className="h-4 w-4 text-rose-600 shrink-0 mt-0.5" />
                          <div>
                            <span className="font-semibold">Alasan Penolakan Anti-Judol:</span> {reason}
                          </div>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Financial & Actions Column */}
                  <div className="flex flex-col sm:flex-row md:flex-col items-end gap-3 shrink-0 w-full md:w-auto border-t md:border-t-0 pt-3 md:pt-0">
                    <div className="text-right">
                      <div className="text-xs text-slate-500 font-medium flex items-center justify-end gap-1">
                        <Coins className="h-3 w-3 text-amber-500" />
                        Nominal PWYW
                      </div>
                      <div className="text-lg font-black text-slate-900">
                        {formatIDR(amount)}
                      </div>
                      <div className="text-[11px] text-slate-500 flex items-center justify-end gap-1">
                        <Clock className="h-3 w-3 text-slate-400" />
                        Bayar: {paidAt ? formatDate(paidAt) : 'Pending'}
                      </div>
                    </div>

                    {/* Action buttons */}
                    <div className="flex items-center gap-2 pt-1 w-full sm:w-auto justify-end">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleViewActivities(sp)}
                        className="text-xs gap-1 text-slate-600 hover:text-slate-900 h-8"
                        title="Lihat Log Audit"
                      >
                        <History className="h-3.5 w-3.5" />
                        Audit
                      </Button>

                      {status === 'pending_review' ? (
                        <>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => handleOpenRejectModal(sp)}
                            disabled={isActing}
                            className="text-xs gap-1 h-8 bg-rose-600 hover:bg-rose-700 text-white"
                          >
                            <Ban className="h-3.5 w-3.5" />
                            Tolak (Judol)
                          </Button>

                          <Button
                            size="sm"
                            onClick={() => handleApprove(sp)}
                            disabled={isActing}
                            className="text-xs gap-1.5 h-8 bg-emerald-600 hover:bg-emerald-700 text-white shadow-xs font-semibold"
                          >
                            {isActing ? (
                              <>
                                <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                                Proses...
                              </>
                            ) : (
                              <>
                                <CheckCircle2 className="h-3.5 w-3.5" />
                                1-Klik Approve
                              </>
                            )}
                          </Button>
                        </>
                      ) : status === 'rejected' ? (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleApprove(sp)}
                          disabled={isActing}
                          className="text-xs gap-1 h-8 text-emerald-700 hover:bg-emerald-50 border-emerald-200"
                        >
                          <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
                          Pulihkan & Approve
                        </Button>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleOpenRejectModal(sp)}
                          disabled={isActing}
                          className="text-xs gap-1 h-8 text-rose-700 hover:bg-rose-50 border-rose-200"
                        >
                          <Ban className="h-3.5 w-3.5 text-rose-600" />
                          Batalkan (Reject)
                        </Button>
                      )}
                    </div>
                  </div>
                </div>
              </Card>
            )
          })}
        </div>
      )}

      {/* REJECT MODAL DIALOG */}
      {rejectingSponsor && (
        <div className="fixed inset-0 z-50 bg-slate-900/50 backdrop-blur-xs flex items-center justify-center p-4">
          <Card className="bg-white max-w-lg w-full shadow-xl border-slate-200 animate-in fade-in-50 zoom-in-95">
            <CardHeader className="space-y-1 pb-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-rose-600">
                  <ShieldAlert className="h-5 w-5" />
                  <CardTitle className="text-base font-bold text-slate-900">
                    Tolak Sponsor & Karantina Anti-Judol
                  </CardTitle>
                </div>
                <button
                  onClick={() => setRejectingSponsor(null)}
                  className="text-slate-400 hover:text-slate-700"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <CardDescription className="text-xs">
                Brand: <strong className="text-slate-800">{rejectingSponsor.brand_name || (rejectingSponsor as any).BrandName}</strong> ({rejectingSponsor.website_url || (rejectingSponsor as any).WebsiteURL})
              </CardDescription>
            </CardHeader>

            <CardContent className="space-y-4 pt-1">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700">Pilih Template Alasan:</label>
                <div className="space-y-1">
                  {presetReasons.map((p, idx) => (
                    <button
                      key={idx}
                      type="button"
                      onClick={() => setRejectReason(p)}
                      className={`w-full text-left p-2 rounded-md text-xs border transition-all ${
                        rejectReason === p
                          ? 'border-rose-300 bg-rose-50 text-rose-900 font-medium'
                          : 'border-slate-200 bg-slate-50 text-slate-700 hover:bg-slate-100'
                      }`}
                    >
                      {p}
                    </button>
                  ))}
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700">Kustomisasi Catatan Penolakan:</label>
                <textarea
                  rows={3}
                  value={rejectReason}
                  onChange={(e) => setRejectReason(e.target.value)}
                  placeholder="Tuliskan alasan penolakan pelanggaran etika/anti-judol..."
                  className="w-full text-xs p-2.5 rounded-md border border-slate-300 focus:outline-none focus:ring-2 focus:ring-rose-500"
                />
              </div>

              <div className="p-3 bg-amber-50 border border-amber-200 rounded-lg text-[11px] text-amber-900 space-y-1">
                <div className="font-semibold flex items-center gap-1">
                  <AlertCircle className="h-3.5 w-3.5 text-amber-600" />
                  Jaminan Keamanan & Audit Trail Cuan
                </div>
                <p>
                  Sponsor yang ditolak akan dicegah 100% dari leaderboard publik `/sponsor` dan arsip direktori GitHub.
                  Aksi ini dicatat secara atomik di tabel audit aktivitas atas nama email owner terautentikasi.
                </p>
              </div>

              <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-100">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setRejectingSponsor(null)}
                  disabled={rejectSubmitting}
                  className="text-xs"
                >
                  Batal
                </Button>
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  onClick={handleConfirmReject}
                  disabled={rejectSubmitting || !rejectReason.trim()}
                  className="text-xs gap-1.5 bg-rose-600 hover:bg-rose-700"
                >
                  {rejectSubmitting ? (
                    <>
                      <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                      Memproses Karantina...
                    </>
                  ) : (
                    <>
                      <Ban className="h-3.5 w-3.5" />
                      Konfirmasi Tolak & Karantina
                    </>
                  )}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* AUDIT TRAIL MODAL */}
      {activityModalSponsor && (
        <div className="fixed inset-0 z-50 bg-slate-900/50 backdrop-blur-xs flex items-center justify-center p-4">
          <Card className="bg-white max-w-lg w-full shadow-xl border-slate-200">
            <CardHeader className="space-y-1 pb-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-slate-900">
                  <History className="h-5 w-5 text-emerald-600" />
                  <CardTitle className="text-base font-bold">
                    Audit Trail: {activityModalSponsor.brand_name || (activityModalSponsor as any).BrandName}
                  </CardTitle>
                </div>
                <button
                  onClick={() => setActivityModalSponsor(null)}
                  className="text-slate-400 hover:text-slate-700"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <CardDescription className="text-xs">
                Riwayat seluruh aksi moderasi, webhook pembayaran, dan status perubahan.
              </CardDescription>
            </CardHeader>

            <CardContent className="space-y-3 pt-1">
              {activitiesLoading ? (
                <div className="p-8 text-center text-xs text-slate-500">
                  <RefreshCw className="h-6 w-6 animate-spin mx-auto mb-2 text-emerald-600" />
                  Memuat riwayat aktivitas...
                </div>
              ) : activities.length === 0 ? (
                <div className="p-6 text-center text-xs text-slate-500 bg-slate-50 rounded-lg">
                  Belum ada log aktivitas tercatat untuk sponsor ini.
                </div>
              ) : (
                <div className="space-y-2.5 max-h-80 overflow-y-auto pr-1">
                  {activities.map((act) => (
                    <div
                      key={act.id}
                      className="p-3 bg-slate-50 border border-slate-200 rounded-lg text-xs space-y-1"
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-semibold text-slate-800 capitalize">
                          {act.action.replace(/_/g, ' ')}
                        </span>
                        <span className="text-[11px] text-slate-400 font-mono">
                          {formatDate(act.created_at)}
                        </span>
                      </div>
                      <p className="text-slate-600">{act.details}</p>
                      <div className="text-[11px] text-slate-500 flex items-center gap-1 font-mono pt-0.5">
                        <span className="text-slate-400">Aktor:</span>
                        <span className="font-semibold text-emerald-700">{act.actor}</span>
                      </div>
                    </div>
                  ))}
                </div>
              )}

              <div className="flex justify-end pt-2 border-t border-slate-100">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setActivityModalSponsor(null)}
                  className="text-xs"
                >
                  Tutup
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}
