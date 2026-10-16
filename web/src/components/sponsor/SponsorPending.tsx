import { useEffect, useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '../ui/card'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import {
  Clock,
  CheckCircle2,
  AlertTriangle,
  RefreshCw,
  ArrowLeft,
  ShieldCheck,
  Trophy,
  ExternalLink,
  Building2
} from 'lucide-react'

interface SponsorPendingProps {
  onNavigate: (path: string) => void
  orderId?: string
}

interface StatusResponse {
  found: boolean
  status: 'pending_payment' | 'pending_review' | 'approved' | 'rejected' | 'failed' | 'not_found'
  message: string
  order?: {
    id: string
    dojo_order_id: string
    amount_idr: number
    status: string
    paid_at?: string
  }
  sponsor?: {
    id: string
    brand_name: string
    website_url: string
    slogan?: string
    logo_path?: string
    status: string
    rejection_reason?: string
  }
  leaderboard_url?: string
}

export function SponsorPending({ onNavigate, orderId: propOrderId }: SponsorPendingProps) {
  const [orderId, setOrderId] = useState<string>(() => {
    if (propOrderId) return propOrderId
    const params = new URLSearchParams(window.location.search)
    return params.get('order_id') || params.get('id') || ''
  })

  const [data, setData] = useState<StatusResponse | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  const [autoRefreshCount, setAutoRefreshCount] = useState<number>(0)

  const fetchStatus = async () => {
    if (!orderId) {
      setLoading(false)
      return
    }
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`/api/sponsors/status?order_id=${encodeURIComponent(orderId)}`)
      if (res.ok) {
        const json: StatusResponse = await res.json()
        setData(json)
      } else if (res.status === 404) {
        setData({
          found: false,
          status: 'not_found',
          message: 'Referensi order sponsorship tidak ditemukan di sistem Cuan.',
        })
      } else {
        throw new Error('Gagal memeriksa status pembayaran')
      }
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan saat memeriksa status')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchStatus()
  }, [orderId])

  // Periodic polling if pending
  useEffect(() => {
    if (!data || data.status === 'approved' || data.status === 'rejected' || autoRefreshCount > 15) {
      return
    }
    const timer = setTimeout(() => {
      setAutoRefreshCount((prev) => prev + 1)
      fetchStatus()
    }, 8000)
    return () => clearTimeout(timer)
  }, [data, autoRefreshCount])

  const formatIDR = (amount?: number) => {
    if (!amount) return 'Rp 10.000+'
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      maximumFractionDigits: 0,
    }).format(amount)
  }

  const currentStage = () => {
    if (!data || !data.found) return 0
    if (data.status === 'pending_payment') return 1
    if (data.status === 'pending_review') return 2
    if (data.status === 'approved') return 3
    if (data.status === 'rejected') return -1
    return 1
  }

  const stage = currentStage()

  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 flex flex-col selection:bg-emerald-500 selection:text-white">
      {/* Navigation */}
      <nav className="sticky top-0 z-40 w-full border-b border-slate-800 bg-slate-900/90 backdrop-blur-md">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 h-16 flex items-center justify-between">
          <button
            onClick={() => onNavigate('/sponsor')}
            className="flex items-center gap-2 text-sm text-slate-300 hover:text-white transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            <span>Kembali ke Halaman Sponsor</span>
          </button>
          <div className="flex items-center gap-2">
            <span className="text-xs text-slate-400">Status Pembayaran Dojo</span>
          </div>
        </div>
      </nav>

      {/* Main Container */}
      <main className="flex-1 max-w-3xl mx-auto w-full px-4 sm:px-6 py-10 space-y-6">
        {/* Header */}
        <div className="text-center space-y-2">
          <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-xs px-3 py-1">
            Konfirmasi Transaksi Founding Backer
          </Badge>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-white tracking-tight">
            Status Konfirmasi Sponsorship
          </h1>
          <p className="text-xs sm:text-sm text-slate-300 max-w-lg mx-auto">
            Halaman ini memantau status pembayaran Dojo Checkout dan moderasi kurasi anti-judol oleh Owner Cuan secara real-time.
          </p>
        </div>

        {/* Progress Tracker */}
        <div className="bg-slate-850 border border-slate-800 rounded-2xl p-5 shadow-lg">
          <div className="grid grid-cols-3 gap-2 text-center">
            {/* Step 1 */}
            <div className="space-y-1.5 flex flex-col items-center">
              <div
                className={`h-9 w-9 rounded-full flex items-center justify-center font-bold text-xs ${
                  stage >= 1
                    ? stage > 1
                      ? 'bg-emerald-500 text-slate-950'
                      : 'bg-amber-400 text-slate-950 animate-pulse'
                    : 'bg-slate-800 text-slate-500'
                }`}
              >
                {stage > 1 ? <CheckCircle2 className="h-5 w-5" /> : '1'}
              </div>
              <div className="text-xs font-semibold text-white">Pembayaran Dojo</div>
              <div className="text-[10px] text-slate-400">
                {stage > 1 ? 'Terverifikasi Webhook' : 'Menunggu / Diproses'}
              </div>
            </div>

            {/* Step 2 */}
            <div className="space-y-1.5 flex flex-col items-center">
              <div
                className={`h-9 w-9 rounded-full flex items-center justify-center font-bold text-xs ${
                  stage >= 2
                    ? stage > 2
                      ? 'bg-emerald-500 text-slate-950'
                      : 'bg-cyan-400 text-slate-950 animate-pulse'
                    : 'bg-slate-800 text-slate-500'
                }`}
              >
                {stage > 2 ? <CheckCircle2 className="h-5 w-5" /> : '2'}
              </div>
              <div className="text-xs font-semibold text-white">Kurasi Anti-Judol</div>
              <div className="text-[10px] text-slate-400">
                {stage > 2 ? 'Lulus Review' : stage === 2 ? 'Dalam Antrean Review' : 'Menunggu Step 1'}
              </div>
            </div>

            {/* Step 3 */}
            <div className="space-y-1.5 flex flex-col items-center">
              <div
                className={`h-9 w-9 rounded-full flex items-center justify-center font-bold text-xs ${
                  stage === 3
                    ? 'bg-amber-400 text-slate-950 shadow-lg shadow-amber-400/20'
                    : 'bg-slate-800 text-slate-500'
                }`}
              >
                {stage === 3 ? <Trophy className="h-5 w-5" /> : '3'}
              </div>
              <div className="text-xs font-semibold text-white">Leaderboard Live</div>
              <div className="text-[10px] text-slate-400">
                {stage === 3 ? 'Tayang di Web & GitHub' : 'Menunggu Approval'}
              </div>
            </div>
          </div>
        </div>

        {/* Status Card */}
        {loading && !data ? (
          <div className="bg-slate-850 border border-slate-800 rounded-2xl p-8 text-center space-y-3">
            <RefreshCw className="h-8 w-8 text-emerald-400 animate-spin mx-auto" />
            <p className="text-xs text-slate-300">Memeriksa status pesanan ke server Cuan...</p>
          </div>
        ) : error ? (
          <div className="bg-red-500/10 border border-red-500/30 rounded-2xl p-6 text-center space-y-3">
            <AlertTriangle className="h-8 w-8 text-red-400 mx-auto" />
            <h3 className="text-sm font-bold text-red-300">Gagal Memuat Status</h3>
            <p className="text-xs text-slate-400">{error}</p>
            <Button size="sm" variant="outline" onClick={fetchStatus} className="text-xs">
              Coba Lagi
            </Button>
          </div>
        ) : !data || !data.found ? (
          <div className="bg-slate-850 border border-slate-800 rounded-2xl p-8 text-center space-y-4">
            <AlertTriangle className="h-10 w-10 text-amber-400 mx-auto" />
            <div className="space-y-1">
              <h3 className="text-base font-bold text-white">Order Tidak Ditemukan</h3>
              <p className="text-xs text-slate-400 max-w-md mx-auto">
                Nomor pesanan yang dimasukkan tidak ditemukan. Pastikan Anda mengakses URL konfirmasi yang benar setelah checkout Dojo.
              </p>
            </div>
            <div className="max-w-xs mx-auto flex gap-2">
              <input
                type="text"
                placeholder="Masukkan Order ID (contoh: dojo_sp_...)"
                value={orderId}
                onChange={(e) => setOrderId(e.target.value.trim())}
                className="w-full text-xs p-2 rounded-lg bg-slate-900 border border-slate-700 text-white font-mono"
              />
              <Button size="sm" onClick={fetchStatus} className="bg-emerald-600 text-xs shrink-0">
                Cari
              </Button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            {/* Status Highlight Banner */}
            {data.status === 'pending_payment' && (
              <div className="bg-amber-500/10 border border-amber-500/30 rounded-2xl p-5 flex items-start gap-3.5">
                <Clock className="h-6 w-6 text-amber-400 shrink-0 mt-0.5 animate-spin" />
                <div className="space-y-1">
                  <h4 className="text-sm font-bold text-amber-300">Menunggu Verifikasi Pembayaran Dojo</h4>
                  <p className="text-xs text-slate-300 leading-relaxed">
                    Sistem sedang menunggu sinyal pembayaran dari gateway Dojo. Jika Anda baru saja menyelesaikan transaksi via QRIS/VA/E-Wallet, webhook akan terverifikasi secara otomatis dalam beberapa saat.
                  </p>
                </div>
              </div>
            )}

            {data.status === 'pending_review' && (
              <div className="bg-cyan-500/10 border border-cyan-500/30 rounded-2xl p-5 flex items-start gap-3.5">
                <ShieldCheck className="h-6 w-6 text-cyan-400 shrink-0 mt-0.5" />
                <div className="space-y-1">
                  <h4 className="text-sm font-bold text-cyan-300">Pembayaran Sukses! Sedang Dalam Tahap Kurasi Anti-Judol</h4>
                  <p className="text-xs text-slate-300 leading-relaxed">
                    Pembayaran Anda telah terkonfirmasi lunas via Dojo webhook. Profil dan logo Anda saat ini berada dalam antrean kurasi Owner Cuan untuk memastikan kepatuhan etika anti-judol (maks 1x24 jam).
                  </p>
                </div>
              </div>
            )}

            {data.status === 'approved' && (
              <div className="bg-emerald-500/15 border border-emerald-500/40 rounded-2xl p-5 flex items-start gap-3.5">
                <Trophy className="h-6 w-6 text-emerald-400 shrink-0 mt-0.5" />
                <div className="space-y-1">
                  <h4 className="text-sm font-bold text-emerald-300">Selamat! Profil Sponsor Anda Telah Disetujui</h4>
                  <p className="text-xs text-slate-300 leading-relaxed">
                    Brand Anda resmi menjadi Founding Backer Cuan v1.0 dan kini live di Leaderboard publik, direktori arsip, serta tersinkronisasi ke repositori GitHub buchin/cuan.
                  </p>
                </div>
              </div>
            )}

            {data.status === 'rejected' && (
              <div className="bg-red-500/15 border border-red-500/40 rounded-2xl p-5 flex items-start gap-3.5">
                <AlertTriangle className="h-6 w-6 text-red-400 shrink-0 mt-0.5" />
                <div className="space-y-1">
                  <h4 className="text-sm font-bold text-red-300">Pendaftaran Sponsor Ditolak</h4>
                  <p className="text-xs text-slate-300 leading-relaxed">
                    {data.sponsor?.rejection_reason || 'Pendaftaran tidak memenuhi kriteria etika anti-judol Cuan.'}
                  </p>
                </div>
              </div>
            )}

            {/* Order & Sponsor Details Card */}
            <Card className="bg-slate-850 border-slate-800 text-slate-100">
              <CardHeader className="pb-3 border-b border-slate-800">
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle className="text-base text-white">Detail Pesanan Sponsorship</CardTitle>
                    <CardDescription className="text-xs text-slate-400">
                      ID Referensi: <code className="text-emerald-400 font-mono">{data.order?.dojo_order_id || orderId}</code>
                    </CardDescription>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={fetchStatus}
                    disabled={loading}
                    className="border-slate-700 text-xs text-slate-300 hover:text-white"
                  >
                    <RefreshCw className={`h-3 w-3 mr-1 ${loading ? 'animate-spin' : ''}`} />
                    Perbarui
                  </Button>
                </div>
              </CardHeader>

              <CardContent className="pt-4 space-y-4">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="space-y-1">
                    <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Nama Brand</span>
                    <div className="text-sm font-bold text-white flex items-center gap-1.5">
                      <Building2 className="h-4 w-4 text-emerald-400" />
                      <span>{data.sponsor?.brand_name || '-'}</span>
                    </div>
                  </div>

                  <div className="space-y-1">
                    <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Nominal Kontribusi</span>
                    <div className="text-sm font-mono font-bold text-emerald-400">
                      {formatIDR(data.order?.amount_idr)}
                    </div>
                  </div>

                  <div className="space-y-1">
                    <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Website URL</span>
                    <div>
                      {data.sponsor?.website_url ? (
                        <a
                          href={data.sponsor.website_url}
                          target="_blank"
                          rel="noopener noreferrer nofollow"
                          className="text-xs text-cyan-400 hover:underline flex items-center gap-1"
                        >
                          <span>{data.sponsor.website_url}</span>
                          <ExternalLink className="h-3 w-3" />
                        </a>
                      ) : (
                        <span className="text-xs text-slate-500">-</span>
                      )}
                    </div>
                  </div>

                  <div className="space-y-1">
                    <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Status Cuan</span>
                    <div>
                      <Badge
                        variant="outline"
                        className={`text-xs ${
                          data.status === 'approved'
                            ? 'border-emerald-500/40 text-emerald-400 bg-emerald-500/10'
                            : data.status === 'pending_review'
                            ? 'border-cyan-500/40 text-cyan-400 bg-cyan-500/10'
                            : data.status === 'rejected'
                            ? 'border-red-500/40 text-red-400 bg-red-500/10'
                            : 'border-amber-500/40 text-amber-400 bg-amber-500/10'
                        }`}
                      >
                        {data.status.replace('_', ' ').toUpperCase()}
                      </Badge>
                    </div>
                  </div>
                </div>

                {data.sponsor?.slogan && (
                  <div className="p-3 bg-slate-900 rounded-xl border border-slate-800 space-y-1">
                    <span className="text-[10px] uppercase font-bold text-slate-400">Slogan Brand</span>
                    <p className="text-xs text-slate-200 italic font-medium">"{data.sponsor.slogan}"</p>
                  </div>
                )}
              </CardContent>

              <CardFooter className="pt-2 pb-4 border-t border-slate-800 flex flex-wrap gap-2 justify-between items-center">
                <span className="text-[11px] text-slate-400">
                  {data.order?.paid_at
                    ? `Dibayar pada: ${new Date(data.order.paid_at).toLocaleString('id-ID')}`
                    : 'Menunggu proses pembayaran gateway'}
                </span>

                <div className="flex gap-2">
                  {data.status === 'approved' && (
                    <Button
                      size="sm"
                      onClick={() => onNavigate('/sponsor')}
                      className="bg-emerald-600 hover:bg-emerald-500 text-white text-xs gap-1.5"
                    >
                      <Trophy className="h-3.5 w-3.5" />
                      Lihat di Leaderboard
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => onNavigate('/sponsors')}
                    className="border-slate-700 text-xs text-slate-300 hover:text-white"
                  >
                    Buka Direktori Arsip
                  </Button>
                </div>
              </CardFooter>
            </Card>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800 bg-slate-950 py-6 text-xs text-slate-500 text-center">
        Cuan Founding Backers Program • Verifikasi Pembayaran Dojo • Apache-2.0
      </footer>
    </div>
  )
}
