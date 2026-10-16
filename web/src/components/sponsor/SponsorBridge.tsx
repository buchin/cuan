import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import {
  HeartHandshake,
  ArrowLeft,
  Trophy,
  ExternalLink,
  Sparkles,
  RefreshCw,
  Globe,
  Lock,
  ShieldAlert
} from 'lucide-react'

interface SponsorItem {
  id: string
  brand_name: string
  website_url?: string
  slogan?: string
  logo_path?: string
  amount_idr?: number
  paid_at?: string
  status: string
}

interface SponsorBridgeProps {
  onNavigate: (path: string) => void
}

export function SponsorBridge({ onNavigate }: SponsorBridgeProps) {
  const [sponsors, setSponsors] = useState<SponsorItem[]>([])
  const [loading, setLoading] = useState(true)

  const fetchCurrentSponsors = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/sponsors/current?batch=v1.0')
      if (res.ok) {
        const data = await res.json()
        setSponsors(data.sponsors || [])
      } else {
        // Graceful empty fallback
        setSponsors([])
      }
    } catch (err: any) {
      console.warn('Could not fetch current sponsors:', err)
      setSponsors([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchCurrentSponsors()
  }, [])

  const formatIDR = (amount?: number) => {
    if (!amount) return 'Rp 10.000+'
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      maximumFractionDigits: 0,
    }).format(amount)
  }

  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 flex flex-col selection:bg-emerald-500 selection:text-white">
      {/* Top Navbar */}
      <nav className="sticky top-0 z-40 w-full border-b border-slate-800 bg-slate-900/90 backdrop-blur-md">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <button
            onClick={() => onNavigate('/')}
            className="flex items-center gap-2 text-sm text-slate-300 hover:text-white transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            <span>Kembali ke Beranda Cuan</span>
          </button>

          <div className="flex items-center gap-3">
            <button
              onClick={() => onNavigate('/app')}
              className="text-xs text-slate-400 hover:text-white flex items-center gap-1.5 transition-colors"
            >
              <Lock className="h-3.5 w-3.5" />
              Studio Dashboard
            </button>
          </div>
        </div>
      </nav>

      {/* Hero Header */}
      <section className="pt-16 pb-12 px-4 sm:px-6 lg:px-8 text-center max-w-4xl mx-auto space-y-4">
        <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-semibold">
          <HeartHandshake className="h-4 w-4" />
          <span>Batch v1.0 Founding Backers</span>
        </div>
        <h1 className="text-3xl sm:text-5xl font-black text-white tracking-tight">
          Menjadi Founding Sponsor Cuan v1.0
        </h1>
        <p className="text-sm sm:text-base text-slate-300 max-w-2xl mx-auto leading-relaxed">
          Dukung kelangsungan proyek open-source Cuan dengan skema{' '}
          <strong className="text-emerald-400 font-semibold">Pay-What-You-Want (min Rp 10.000)</strong> via Dojo checkout.
          Dapatkan posisi abadi di Leaderboard, repositori GitHub, dan tautan dofollow permanen.
        </p>
      </section>

      {/* Value Propositions for Sponsors */}
      <section className="py-8 px-4 sm:px-6 lg:px-8 max-w-6xl mx-auto w-full">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 text-left">
          <div className="p-5 rounded-2xl bg-slate-800/80 border border-slate-700/80 space-y-2">
            <div className="flex items-center gap-2 text-amber-400 font-bold text-sm">
              <Trophy className="h-4 w-4" />
              <span>Leaderboard Homepage</span>
            </div>
            <p className="text-xs text-slate-300 leading-relaxed">
              Top 30 Backer tampil di homepage Cuan. Top 5 mendapatkan ukuran logo paling dominan dan slogan bisnis.
            </p>
          </div>

          <div className="p-5 rounded-2xl bg-slate-800/80 border border-slate-700/80 space-y-2">
            <div className="flex items-center gap-2 text-emerald-400 font-bold text-sm">
              <Globe className="h-4 w-4" />
              <span>GitHub README Abadi</span>
            </div>
            <p className="text-xs text-slate-300 leading-relaxed">
              Logo dan nama brand Anda tersinkronisasi permanen ke file <code className="text-emerald-300">README.md</code> di repositori buchin/cuan.
            </p>
          </div>

          <div className="p-5 rounded-2xl bg-slate-800/80 border border-slate-700/80 space-y-2">
            <div className="flex items-center gap-2 text-cyan-400 font-bold text-sm">
              <Sparkles className="h-4 w-4" />
              <span>Pay-What-You-Want</span>
            </div>
            <p className="text-xs text-slate-300 leading-relaxed">
              Nominal bebas (mulai Rp 10.000). Ranking transparan: kontribusi terbesar di atas, dan yang bayar lebih cepat berposisi lebih tinggi.
            </p>
          </div>

          <div className="p-5 rounded-2xl bg-slate-800/80 border border-slate-700/80 space-y-2">
            <div className="flex items-center gap-2 text-red-400 font-bold text-sm">
              <ShieldAlert className="h-4 w-4" />
              <span>Etika Anti-Judol Ketat</span>
            </div>
            <p className="text-xs text-slate-300 leading-relaxed">
              Brand Anda terlindungi. Semua sponsor dikurasi manual oleh owner; situs judol, porno, dan penipuan ditolak keras.
            </p>
          </div>
        </div>
      </section>

      {/* Live Leaderboard Section */}
      <section className="py-12 px-4 sm:px-6 lg:px-8 max-w-5xl mx-auto w-full flex-1">
        <div className="flex items-center justify-between mb-6">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <h2 className="text-xl sm:text-2xl font-bold text-white">Live Leaderboard Backers</h2>
              <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-xs">
                v1.0 Batch
              </Badge>
            </div>
            <p className="text-xs text-slate-400">
              Diurutkan secara dinamis berdasarkan kontribusi terbesar (<code className="text-slate-300 font-mono">amount DESC, paid_at ASC</code>)
            </p>
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={fetchCurrentSponsors}
            disabled={loading}
            className="border-slate-700 text-xs text-slate-300 hover:text-white"
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
            Perbarui
          </Button>
        </div>

        {/* Leaderboard content */}
        {loading ? (
          <div className="p-12 text-center bg-slate-850 rounded-2xl border border-slate-800 space-y-3">
            <RefreshCw className="h-7 w-7 text-emerald-400 animate-spin mx-auto" />
            <p className="text-xs text-slate-400">Memuat leaderboard sponsor terverifikasi...</p>
          </div>
        ) : sponsors.length === 0 ? (
          <div className="p-12 text-center bg-slate-850 rounded-2xl border border-slate-800 space-y-4">
            <div className="h-14 w-14 rounded-2xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center mx-auto border border-emerald-500/20">
              <Trophy className="h-7 w-7" />
            </div>
            <div className="space-y-1">
              <h3 className="font-bold text-lg text-white">Belum Ada Sponsor Terverifikasi</h3>
              <p className="text-xs text-slate-400 max-w-md mx-auto">
                Batch founding v1.0 baru saja dibuka! Jadilah sponsor pertama untuk mengamankan posisi teratas di Leaderboard Cuan.
              </p>
            </div>
            <div className="pt-2">
              <a
                href="#intake"
                className="inline-flex items-center gap-2 px-6 py-2.5 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm shadow-lg shadow-emerald-500/20"
              >
                <Sparkles className="h-4 w-4" />
                Jadilah Backer #1 Sekarang
              </a>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {sponsors.map((sp, idx) => {
              const isTop5 = idx < 5
              return (
                <div
                  key={sp.id}
                  className={`flex flex-col sm:flex-row items-start sm:items-center justify-between p-4 rounded-xl border transition-all ${
                    isTop5
                      ? 'bg-gradient-to-r from-slate-850 to-slate-900 border-amber-500/30 shadow-md'
                      : 'bg-slate-850/60 border-slate-800'
                  }`}
                >
                  <div className="flex items-center gap-4">
                    <div
                      className={`h-9 w-9 rounded-xl flex items-center justify-center font-black text-sm shrink-0 ${
                        idx === 0
                          ? 'bg-amber-400 text-slate-950 shadow-md shadow-amber-400/20'
                          : idx === 1
                          ? 'bg-slate-300 text-slate-950'
                          : idx === 2
                          ? 'bg-amber-700 text-white'
                          : 'bg-slate-800 text-slate-400'
                      }`}
                    >
                      #{idx + 1}
                    </div>

                    <div className="space-y-0.5">
                      <div className="flex items-center gap-2">
                        <span className="font-bold text-white text-sm">{sp.brand_name}</span>
                        {isTop5 && (
                          <Badge variant="outline" className="border-amber-500/40 text-amber-400 text-[10px] py-0 px-1">
                            Top 5
                          </Badge>
                        )}
                        {sp.website_url && (
                          <a
                            href={sp.website_url}
                            target="_blank"
                            rel="noreferrer noopener"
                            className="text-xs text-emerald-400 hover:underline flex items-center gap-0.5"
                          >
                            <ExternalLink className="h-3 w-3" />
                          </a>
                        )}
                      </div>
                      {sp.slogan && <p className="text-xs text-slate-400">{sp.slogan}</p>}
                    </div>
                  </div>

                  <div className="mt-2 sm:mt-0 text-left sm:text-right">
                    <div className="font-mono font-bold text-sm text-emerald-400">{formatIDR(sp.amount_idr)}</div>
                    <div className="text-[10px] text-slate-500">
                      {sp.paid_at ? new Date(sp.paid_at).toLocaleDateString('id-ID') : 'Verified Backer'}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </section>

      {/* Intake / Call to action */}
      <section id="intake" className="py-16 bg-slate-950 border-t border-slate-800 px-4 sm:px-6 lg:px-8 text-center">
        <div className="max-w-xl mx-auto space-y-4">
          <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-xs">
            Dojo Payment Gateway
          </Badge>
          <h3 className="text-2xl font-bold text-white">Siap Mendukung Cuan v1.0?</h3>
          <p className="text-xs sm:text-sm text-slate-400 leading-relaxed">
            Pendaftaran intake profil sponsor, upload logo SVG yang disanitasi anti-XSS, dan tombol redirect checkout Pay-What-You-Want Dojo aktif secara penuh pada rilis S02.3.
          </p>
          <div className="pt-3 flex flex-col sm:flex-row items-center justify-center gap-3">
            <button
              onClick={() => onNavigate('/')}
              className="w-full sm:w-auto px-6 py-2.5 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm shadow-lg shadow-emerald-500/20"
            >
              Kembali ke Landing Page
            </button>
            <button
              onClick={() => onNavigate('/app')}
              className="w-full sm:w-auto px-5 py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-sm font-semibold border border-slate-700"
            >
              Buka Studio Dashboard
            </button>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-slate-800 bg-slate-950 py-8 text-xs text-slate-500 text-center">
        Cuan Founding Backers Program • Supported by Dojo Payment Ecosystem • Apache-2.0
      </footer>
    </div>
  )
}
