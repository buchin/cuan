import { useEffect, useState } from 'react'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import {
  Trophy,
  ExternalLink,
  Search,
  ArrowLeft,
  HeartHandshake,
  Calendar,
  Sparkles,
  RefreshCw,
  Building2
} from 'lucide-react'

interface SponsorItem {
  id: string
  batch_id?: string
  brand_name: string
  website_url?: string
  slogan?: string
  logo_path?: string
  amount_idr?: number
  paid_at?: string
  status: string
  created_at?: string
}

interface BatchItem {
  id: string
  batch_slug: string
  name: string
  status: string
}

interface SponsorArchiveProps {
  onNavigate: (path: string) => void
  initialBatch?: string
}

export function SponsorArchive({ onNavigate, initialBatch }: SponsorArchiveProps) {
  const [batches, setBatches] = useState<BatchItem[]>([])
  const [selectedBatch, setSelectedBatch] = useState<string>(() => {
    if (initialBatch) return initialBatch
    const pathParts = window.location.pathname.split('/')
    if (pathParts.length > 2 && pathParts[2]) {
      return pathParts[2]
    }
    return 'v1.0'
  })

  const [sponsors, setSponsors] = useState<SponsorItem[]>([])
  const [loading, setLoading] = useState<boolean>(true)
  const [searchQuery, setSearchQuery] = useState<string>('')

  const fetchBatches = async () => {
    try {
      const res = await fetch('/api/sponsors/batches')
      if (res.ok) {
        const json = await res.json()
        if (json.batches && json.batches.length > 0) {
          setBatches(json.batches)
        }
      }
    } catch (err) {
      console.warn('Failed to fetch batches:', err)
    }
  }

  const fetchSponsors = async (batchSlug: string) => {
    setLoading(true)
    try {
      const res = await fetch(`/api/sponsors/current?batch=${encodeURIComponent(batchSlug)}`)
      if (res.ok) {
        const data = await res.json()
        setSponsors(data.sponsors || [])
      } else {
        setSponsors([])
      }
    } catch (err) {
      console.warn('Failed to load archive sponsors:', err)
      setSponsors([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchBatches()
  }, [])

  useEffect(() => {
    fetchSponsors(selectedBatch)
  }, [selectedBatch])

  const formatIDR = (amount?: number) => {
    if (!amount) return 'Rp 10.000+'
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      maximumFractionDigits: 0,
    }).format(amount)
  }

  const filteredSponsors = sponsors.filter((sp) => {
    if (!searchQuery.trim()) return true
    const q = searchQuery.toLowerCase()
    return (
      sp.brand_name.toLowerCase().includes(q) ||
      (sp.slogan && sp.slogan.toLowerCase().includes(q))
    )
  })

  const totalRaised = sponsors.reduce((acc, curr) => acc + (curr.amount_idr || 0), 0)

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
            <span>Beranda Cuan</span>
          </button>

          <div className="flex items-center gap-3">
            <Button
              size="sm"
              onClick={() => onNavigate('/sponsor')}
              className="bg-emerald-600 hover:bg-emerald-500 text-white text-xs gap-1.5 font-bold shadow-md shadow-emerald-600/20"
            >
              <Sparkles className="h-3.5 w-3.5" />
              Jadilah Sponsor
            </Button>
            <button
              onClick={() => onNavigate('/app')}
              className="text-xs text-slate-400 hover:text-white transition-colors"
            >
              Studio Dashboard
            </button>
          </div>
        </div>
      </nav>

      {/* Hero Header */}
      <section className="pt-14 pb-8 px-4 sm:px-6 lg:px-8 text-center max-w-4xl mx-auto space-y-3">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-semibold">
          <HeartHandshake className="h-3.5 w-3.5" />
          <span>Direktori Abadi Backers Cuan</span>
        </div>
        <h1 className="text-3xl sm:text-4xl font-black text-white tracking-tight">
          Arsip Sponsor & Founding Backers
        </h1>
        <p className="text-xs sm:text-sm text-slate-300 max-w-2xl mx-auto leading-relaxed">
          Daftar seluruh brand, agensi, dan individu terverifikasi yang mendanai pengembangan Cuan. Setiap profil mendapatkan tautan permanen, logo di README GitHub, dan apresiasi abadi komunitas open-source.
        </p>

        {/* Aggregate Stats */}
        <div className="pt-3 flex items-center justify-center gap-6 text-xs text-slate-400">
          <div className="flex items-center gap-1.5">
            <Building2 className="h-4 w-4 text-emerald-400" />
            <span>
              <strong className="text-white font-bold">{sponsors.length}</strong> Brand Terverifikasi
            </span>
          </div>
          <div className="h-3 w-px bg-slate-700" />
          <div className="flex items-center gap-1.5">
            <Trophy className="h-4 w-4 text-amber-400" />
            <span>
              Total Kontribusi: <strong className="text-emerald-400 font-bold font-mono">{formatIDR(totalRaised)}</strong>
            </span>
          </div>
        </div>
      </section>

      {/* Main Directory Area */}
      <main className="flex-1 max-w-6xl mx-auto w-full px-4 sm:px-6 lg:px-8 py-6 space-y-6">
        {/* Batch Filter & Search Control */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-slate-850 p-3.5 rounded-2xl border border-slate-800">
          {/* Batch Selector Pills */}
          <div className="flex items-center gap-2 overflow-x-auto pb-1 sm:pb-0">
            <span className="text-xs text-slate-400 font-medium shrink-0 mr-1">Batch:</span>
            {(batches.length > 0 ? batches : [{ id: 'b1', batch_slug: 'v1.0', name: 'v1.0 Founding Backers', status: 'open' }]).map((b) => {
              const isActive = selectedBatch === b.batch_slug
              return (
                <button
                  key={b.batch_slug}
                  onClick={() => {
                    setSelectedBatch(b.batch_slug)
                    window.history.pushState({}, '', `/sponsors/${b.batch_slug}`)
                  }}
                  className={`px-3 py-1.5 rounded-xl text-xs font-semibold shrink-0 transition-all ${
                    isActive
                      ? 'bg-emerald-500 text-slate-950 shadow-md shadow-emerald-500/20'
                      : 'bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700'
                  }`}
                >
                  {b.name}
                </button>
              )
            })}
          </div>

          {/* Search Input */}
          <div className="relative w-full sm:w-64">
            <Search className="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
            <Input
              type="text"
              placeholder="Cari brand / slogan..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9 h-9 text-xs bg-slate-900 border-slate-700 text-white placeholder:text-slate-500 focus:border-emerald-500"
            />
          </div>
        </div>

        {/* Sponsors Grid */}
        {loading ? (
          <div className="p-16 text-center bg-slate-850 rounded-2xl border border-slate-800 space-y-3">
            <RefreshCw className="h-8 w-8 text-emerald-400 animate-spin mx-auto" />
            <p className="text-xs text-slate-400">Memuat data arsip sponsor batch {selectedBatch}...</p>
          </div>
        ) : filteredSponsors.length === 0 ? (
          <div className="p-16 text-center bg-slate-850 rounded-2xl border border-slate-800 space-y-4">
            <div className="h-14 w-14 rounded-2xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center mx-auto border border-emerald-500/20">
              <Building2 className="h-7 w-7" />
            </div>
            <div className="space-y-1">
              <h3 className="font-bold text-lg text-white">
                {searchQuery ? 'Tidak Ada Brand yang Cocok' : 'Belum Ada Sponsor di Batch Ini'}
              </h3>
              <p className="text-xs text-slate-400 max-w-md mx-auto">
                {searchQuery
                  ? `Tidak ditemukan sponsor dengan kata kunci "${searchQuery}". Coba kata kunci lain.`
                  : `Batch ${selectedBatch} baru dibuka. Jadilah brand perintis pertama yang terdaftar di arsip ini!`}
              </p>
            </div>
            <div className="pt-2">
              <Button
                onClick={() => onNavigate('/sponsor')}
                className="bg-emerald-600 hover:bg-emerald-500 text-white text-xs gap-1.5 font-bold"
              >
                <Sparkles className="h-3.5 w-3.5" />
                Daftar Sebagai Backer #{sponsors.length + 1}
              </Button>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {filteredSponsors.map((sp, idx) => {
              const isTop5 = idx < 5
              return (
                <div
                  key={sp.id}
                  className={`rounded-2xl p-5 border flex flex-col justify-between transition-all hover:border-slate-700 ${
                    isTop5
                      ? 'bg-gradient-to-b from-slate-850 to-slate-900 border-amber-500/30 shadow-lg'
                      : 'bg-slate-850/80 border-slate-800'
                  }`}
                >
                  <div className="space-y-3">
                    {/* Header Card: Rank + Logo / Avatar */}
                    <div className="flex items-start justify-between gap-3">
                      <div className="flex items-center gap-3">
                        {sp.logo_path ? (
                          <div className="h-12 w-12 rounded-xl bg-slate-900 border border-slate-700 p-1 flex items-center justify-center overflow-hidden shrink-0">
                            <img
                              src={`/${sp.logo_path}`}
                              alt={sp.brand_name}
                              className="max-h-full max-w-full object-contain"
                            />
                          </div>
                        ) : (
                          <div className="h-12 w-12 rounded-xl bg-gradient-to-br from-emerald-500/20 to-cyan-500/20 border border-emerald-500/30 text-emerald-400 flex items-center justify-center font-bold text-sm shrink-0">
                            {sp.brand_name.substring(0, 2).toUpperCase()}
                          </div>
                        )}

                        <div className="space-y-0.5">
                          <h4 className="font-bold text-white text-sm line-clamp-1 flex items-center gap-1.5">
                            <span>{sp.brand_name}</span>
                            {isTop5 && (
                              <Badge
                                variant="outline"
                                className="border-amber-500/40 text-amber-400 text-[10px] py-0 px-1 shrink-0"
                              >
                                Top 5
                              </Badge>
                            )}
                          </h4>
                          {sp.website_url && (
                            <a
                              href={sp.website_url}
                              target="_blank"
                              rel="noopener noreferrer nofollow"
                              className="text-xs text-cyan-400 hover:underline flex items-center gap-1 line-clamp-1"
                            >
                              <span>{sp.website_url.replace(/^https?:\/\//, '')}</span>
                              <ExternalLink className="h-3 w-3 shrink-0" />
                            </a>
                          )}
                        </div>
                      </div>

                      <div
                        className={`h-7 w-7 rounded-lg flex items-center justify-center font-black text-xs shrink-0 ${
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
                    </div>

                    {/* Slogan */}
                    {sp.slogan ? (
                      <p className="text-xs text-slate-300 line-clamp-2 italic leading-relaxed">
                        "{sp.slogan}"
                      </p>
                    ) : (
                      <p className="text-xs text-slate-500 italic">Founding Backer Cuan v1.0</p>
                    )}
                  </div>

                  {/* Footer Card */}
                  <div className="pt-4 mt-3 border-t border-slate-800 flex items-center justify-between text-xs">
                    <span className="font-mono font-bold text-emerald-400">{formatIDR(sp.amount_idr)}</span>
                    <span className="text-[11px] text-slate-500 flex items-center gap-1">
                      <Calendar className="h-3 w-3" />
                      {sp.paid_at ? new Date(sp.paid_at).toLocaleDateString('id-ID') : 'Verified'}
                    </span>
                  </div>
                </div>
              )
            })}
          </div>
        )}

        {/* Bottom CTA Banner */}
        <section className="mt-12 p-8 rounded-3xl bg-gradient-to-r from-emerald-950/60 to-slate-900 border border-emerald-500/30 text-center space-y-4">
          <div className="max-w-xl mx-auto space-y-2">
            <h3 className="text-xl sm:text-2xl font-bold text-white">Ingin Brand Anda Tampil di Direktori Ini?</h3>
            <p className="text-xs sm:text-sm text-slate-300 leading-relaxed">
              Dukung perkembangan Cuan mulai dari <strong>Rp 10.000 (Pay-What-You-Want)</strong>. Brand Anda akan mendapatkan visibilitas permanen di direktori ini dan GitHub README.
            </p>
          </div>
          <div className="pt-1">
            <Button
              size="lg"
              onClick={() => onNavigate('/sponsor')}
              className="bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-extrabold text-sm px-8 shadow-lg shadow-emerald-500/20 gap-2"
            >
              <Sparkles className="h-4 w-4" />
              Daftar Jadi Founding Backer Sekarang
            </Button>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800 bg-slate-950 py-8 text-xs text-slate-500 text-center">
        Cuan Public Directory • Diperbarui otomatis dari Database & Webhook Dojo • Apache-2.0
      </footer>
    </div>
  )
}
