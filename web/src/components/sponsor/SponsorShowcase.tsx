import { useEffect, useState, FormEvent, ChangeEvent } from 'react'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Alert, AlertDescription, AlertTitle } from '../ui/alert'
import {
  HeartHandshake,
  ArrowLeft,
  Trophy,
  ExternalLink,
  Sparkles,
  RefreshCw,
  Globe,
  Lock,
  ShieldAlert,
  Upload,
  CheckCircle2,
  AlertTriangle,
  Building2,
  Mail,
  Phone,
  X
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

interface SponsorShowcaseProps {
  onNavigate: (path: string) => void
}

const PRESET_AMOUNTS = [
  { label: 'Rp 25.000', value: 25000 },
  { label: 'Rp 50.000', value: 50000 },
  { label: 'Rp 100.000', value: 100000 },
  { label: 'Rp 250.000', value: 250000 },
  { label: 'Rp 500.000', value: 500000 },
  { label: 'Rp 1.000.000', value: 1000000 },
  { label: 'Rp 2.500.000', value: 2500000 },
  { label: 'Rp 5.000.000', value: 5000000 },
]

export function SponsorShowcase({ onNavigate }: SponsorShowcaseProps) {
  // Leaderboard states
  const [sponsors, setSponsors] = useState<SponsorItem[]>([])
  const [leaderboardLoading, setLeaderboardLoading] = useState(true)

  // Form states
  const [brandName, setBrandName] = useState('')
  const [websiteUrl, setWebsiteUrl] = useState('')
  const [slogan, setSlogan] = useState('')
  const [contactEmail, setContactEmail] = useState('')
  const [contactWa, setContactWa] = useState('')
  const [amountIdr, setAmountIdr] = useState<number>(50000)
  const [customAmountInput, setCustomAmountInput] = useState<string>('50000')
  const [agreedAntiJudol, setAgreedAntiJudol] = useState(false)

  // Logo upload states
  const [logoFile, setLogoFile] = useState<File | null>(null)
  const [logoPreviewUrl, setLogoPreviewUrl] = useState<string | null>(null)
  const [uploadedLogoPath, setUploadedLogoPath] = useState<string>('')
  const [uploadingLogo, setUploadingLogo] = useState(false)
  const [logoUploadError, setLogoUploadError] = useState<string | null>(null)

  // Submission & Checkout states
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const fetchCurrentSponsors = async () => {
    setLeaderboardLoading(true)
    try {
      const res = await fetch('/api/sponsors/current?batch=v1.0')
      if (res.ok) {
        const data = await res.json()
        setSponsors(data.sponsors || [])
      } else {
        setSponsors([])
      }
    } catch (err: any) {
      console.warn('Could not fetch current sponsors:', err)
      setSponsors([])
    } finally {
      setLeaderboardLoading(false)
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

  // Handle Logo Selection & Async Upload
  const handleLogoChange = async (e: ChangeEvent<HTMLInputElement>) => {
    setLogoUploadError(null)
    const file = e.target.files?.[0]
    if (!file) return

    // Client-side file size check (2MB)
    if (file.size > 2 * 1024 * 1024) {
      setLogoUploadError('Ukuran file logo melebihi batas maksimal 2MB.')
      return
    }

    // Client-side MIME check
    const validMimes = ['image/svg+xml', 'image/png', 'image/webp']
    if (!validMimes.includes(file.type) && !file.name.toLowerCase().endsWith('.svg')) {
      setLogoUploadError('Format file tidak didukung: Harap unggah logo SVG, PNG, atau WebP.')
      return
    }

    setLogoFile(file)
    const objectUrl = URL.createObjectURL(file)
    setLogoPreviewUrl(objectUrl)

    // Upload immediately to backend
    setUploadingLogo(true)
    try {
      const formData = new FormData()
      formData.append('logo', file)

      const res = await fetch('/api/sponsors/upload-logo', {
        method: 'POST',
        body: formData,
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Gagal mengunggah logo')
      }

      setUploadedLogoPath(data.logo_path)
    } catch (err: any) {
      setLogoUploadError(err.message || 'Gagal mengunggah logo ke server')
      setUploadedLogoPath('')
    } finally {
      setUploadingLogo(false)
    }
  }

  const handleRemoveLogo = () => {
    setLogoFile(null)
    if (logoPreviewUrl) {
      URL.revokeObjectURL(logoPreviewUrl)
    }
    setLogoPreviewUrl(null)
    setUploadedLogoPath('')
    setLogoUploadError(null)
  }

  const handleAmountSelect = (val: number) => {
    setAmountIdr(val)
    setCustomAmountInput(val.toString())
  }

  const handleCustomAmountChange = (e: ChangeEvent<HTMLInputElement>) => {
    const raw = e.target.value.replace(/[^0-9]/g, '')
    setCustomAmountInput(raw)
    const num = parseInt(raw, 10)
    if (!isNaN(num)) {
      setAmountIdr(num)
    } else {
      setAmountIdr(0)
    }
  }

  // Submit Intake & Redirect to Dojo PWYW Checkout
  const handleIntakeSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setFormError(null)

    // Validations
    if (!brandName.trim()) {
      setFormError('Nama Brand wajib diisi.')
      return
    }

    if (!websiteUrl.trim()) {
      setFormError('Website URL wajib diisi.')
      return
    }

    const trimmedUrl = websiteUrl.trim().toLowerCase()
    if (trimmedUrl.startsWith('javascript:') || trimmedUrl.startsWith('data:') || trimmedUrl.startsWith('vbscript:')) {
      setFormError('URL tidak valid: skrip berbahaya dilarang keras.')
      return
    }

    if (!contactEmail.trim() || !contactEmail.includes('@') || !contactEmail.includes('.')) {
      setFormError('Alamat email kontak tidak valid.')
      return
    }

    if (amountIdr < 10000) {
      setFormError('Nominal Pay-What-You-Want minimal Rp 10.000.')
      return
    }

    if (!agreedAntiJudol) {
      setFormError('Anda wajib menyetujui aturan etika anti-judol Cuan sebelum melanjutkan.')
      return
    }

    setSubmitting(true)
    try {
      const payload = {
        brand_name: brandName.trim(),
        website_url: websiteUrl.trim(),
        slogan: slogan.trim(),
        logo_path: uploadedLogoPath,
        contact_email: contactEmail.trim(),
        contact_wa: contactWa.trim(),
        amount_idr: amountIdr,
        return_url: `${window.location.origin}/sponsor/pending`,
      }

      const res = await fetch('/api/sponsors/checkout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Gagal menyiapkan sesi checkout')
      }

      // Order created successfully!
      const orderId = data.dojo_order_id || data.order_id
      const checkoutUrl = data.checkout_url

      // Store order ID locally for easy status lookup
      try {
        localStorage.setItem('cuan_last_order_id', orderId)
      } catch (e) {}

      // Open checkout in new window or redirect, and navigate this tab to pending status
      if (checkoutUrl) {
        window.open(checkoutUrl, '_blank')
      }
      onNavigate(`/sponsor/pending?order_id=${encodeURIComponent(orderId)}`)
    } catch (err: any) {
      setFormError(err.message || 'Terjadi kesalahan saat memproses checkout sponsorship')
    } finally {
      setSubmitting(false)
    }
  }

  // Estimate potential leaderboard position
  const estimateRank = () => {
    if (sponsors.length === 0) return 1
    let rank = 1
    for (const sp of sponsors) {
      if ((sp.amount_idr || 0) > amountIdr) {
        rank++
      }
    }
    return rank
  }

  const estimatedRank = estimateRank()

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
              onClick={() => onNavigate('/sponsors')}
              className="text-xs text-slate-300 hover:text-white flex items-center gap-1.5 transition-colors font-medium"
            >
              <Globe className="h-3.5 w-3.5 text-emerald-400" />
              Direktori Arsip Backers
            </button>
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
      <section className="pt-14 pb-10 px-4 sm:px-6 lg:px-8 text-center max-w-4xl mx-auto space-y-4">
        <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-semibold">
          <HeartHandshake className="h-4 w-4" />
          <span>Batch v1.0 Founding Backers</span>
        </div>
        <h1 className="text-3xl sm:text-5xl font-black text-white tracking-tight leading-tight">
          Menjadi Founding Sponsor Cuan v1.0
        </h1>
        <p className="text-xs sm:text-base text-slate-300 max-w-2xl mx-auto leading-relaxed">
          Dukung kelangsungan proyek open-source Cuan dengan skema{' '}
          <strong className="text-emerald-400 font-semibold">Pay-What-You-Want (min Rp 10.000)</strong> via Dojo checkout.
          Dapatkan posisi abadi di Leaderboard, repositori GitHub, dan tautan dofollow permanen.
        </p>

        <div className="pt-2 flex flex-wrap items-center justify-center gap-3">
          <a
            href="#intake"
            className="px-6 py-2.5 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-extrabold text-sm shadow-lg shadow-emerald-500/20 inline-flex items-center gap-2 transition-transform active:scale-95"
          >
            <Sparkles className="h-4 w-4" />
            Daftar Founding Backer Sekarang ↓
          </a>
          <button
            onClick={() => onNavigate('/sponsors')}
            className="px-5 py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700"
          >
            Buka Direktori Arsip (/sponsors)
          </button>
        </div>
      </section>

      {/* Value Propositions */}
      <section className="py-6 px-4 sm:px-6 lg:px-8 max-w-6xl mx-auto w-full">
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4 text-left">
          <div className="p-4 rounded-2xl bg-slate-850 border border-slate-800 space-y-1.5">
            <div className="flex items-center gap-2 text-amber-400 font-bold text-xs">
              <Trophy className="h-4 w-4" />
              <span>Leaderboard Homepage</span>
            </div>
            <p className="text-xs text-slate-300 leading-relaxed">
              Top 30 Backer tampil di homepage Cuan. Top 5 mendapatkan visual dominan dan slogan bisnis.
            </p>
          </div>

          <div className="p-4 rounded-2xl bg-slate-850 border border-slate-800 space-y-1.5">
            <div className="flex items-center gap-2 text-emerald-400 font-bold text-xs">
              <Globe className="h-4 w-4" />
              <span>GitHub README Abadi</span>
            </div>
            <p className="text-xs text-slate-300 leading-relaxed">
              Logo dan nama brand Anda tersinkronisasi permanen ke file <code className="text-emerald-300">README.md</code> buchin/cuan.
            </p>
          </div>

          <div className="p-4 rounded-2xl bg-slate-850 border border-slate-800 space-y-1.5">
            <div className="flex items-center gap-2 text-cyan-400 font-bold text-xs">
              <Sparkles className="h-4 w-4" />
              <span>Pay-What-You-Want</span>
            </div>
            <p className="text-xs text-slate-300 leading-relaxed">
              Nominal bebas (mulai Rp 10.000). Ranking adil: nominal tertinggi di atas, tanggal bayar tercepat diprioritaskan.
            </p>
          </div>

          <div className="p-4 rounded-2xl bg-slate-850 border border-slate-800 space-y-1.5">
            <div className="flex items-center gap-2 text-red-400 font-bold text-xs">
              <ShieldAlert className="h-4 w-4" />
              <span>Etika Anti-Judol Ketat</span>
            </div>
            <p className="text-xs text-slate-300 leading-relaxed">
              Brand Anda terlindungi. Semua sponsor dikurasi manual oleh owner; situs judol, porno, dan penipuan ditolak keras.
            </p>
          </div>
        </div>
      </section>

      {/* Anti-Judol Ethical Policy Banner */}
      <section className="py-2 px-4 sm:px-6 lg:px-8 max-w-5xl mx-auto w-full">
        <div className="p-4 rounded-2xl bg-red-950/40 border border-red-500/40 flex items-start gap-3.5 shadow-md">
          <ShieldAlert className="h-5 w-5 text-red-400 shrink-0 mt-0.5" />
          <div className="space-y-1">
            <h4 className="text-xs font-bold text-red-300 uppercase tracking-wider">
              Kebijakan Zero-Tolerance Anti-Judol & Standar Etika
            </h4>
            <p className="text-xs text-slate-300 leading-relaxed">
              Cuan berkomitmen menjaga integritas ekosistem open-source dan reputasi sponsor yang sah.
              Kami menerapkan penyaringan manual oleh Owner terhadap seluruh pendaftaran. Situs atau brand yang terafiliasi dengan{' '}
              <strong>judi online, pornografi, penipuan, atau konten ilegal</strong> akan langsung ditolak pada tahap kurasi tanpa pengembalian dana (no-refund policy).
            </p>
          </div>
        </div>
      </section>

      {/* Live Leaderboard Section */}
      <section className="py-10 px-4 sm:px-6 lg:px-8 max-w-5xl mx-auto w-full space-y-6">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <h2 className="text-xl sm:text-2xl font-black text-white">Live Leaderboard Backers</h2>
              <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-xs">
                v1.0 Batch
              </Badge>
            </div>
            <p className="text-xs text-slate-400">
              Diurutkan secara dinamis berdasarkan kontribusi terbesar (<code className="text-slate-300 font-mono">ORDER BY amount DESC, paid_at ASC</code>)
            </p>
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={fetchCurrentSponsors}
            disabled={leaderboardLoading}
            className="border-slate-700 text-xs text-slate-300 hover:text-white"
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${leaderboardLoading ? 'animate-spin' : ''}`} />
            Perbarui
          </Button>
        </div>

        {leaderboardLoading ? (
          <div className="p-12 text-center bg-slate-850 rounded-2xl border border-slate-800 space-y-3">
            <RefreshCw className="h-7 w-7 text-emerald-400 animate-spin mx-auto" />
            <p className="text-xs text-slate-400">Memuat data leaderboard sponsor terverifikasi...</p>
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
                  className={`flex flex-col sm:flex-row items-start sm:items-center justify-between p-4 rounded-2xl border transition-all ${
                    isTop5
                      ? 'bg-gradient-to-r from-slate-850 via-slate-850 to-slate-900 border-amber-500/40 shadow-xl'
                      : 'bg-slate-850/70 border-slate-800'
                  }`}
                >
                  <div className="flex items-center gap-4">
                    {/* Rank Badge */}
                    <div
                      className={`h-10 w-10 rounded-xl flex items-center justify-center font-black text-sm shrink-0 ${
                        idx === 0
                          ? 'bg-amber-400 text-slate-950 shadow-lg shadow-amber-400/20'
                          : idx === 1
                          ? 'bg-slate-200 text-slate-950'
                          : idx === 2
                          ? 'bg-amber-700 text-white'
                          : idx < 5
                          ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                          : 'bg-slate-800 text-slate-400'
                      }`}
                    >
                      #{idx + 1}
                    </div>

                    {/* Logo (if available) */}
                    {sp.logo_path ? (
                      <div className="h-11 w-11 rounded-xl bg-slate-900 border border-slate-700 p-1 flex items-center justify-center overflow-hidden shrink-0">
                        <img
                          src={`/${sp.logo_path}`}
                          alt={sp.brand_name}
                          className="max-h-full max-w-full object-contain"
                        />
                      </div>
                    ) : (
                      <div className="h-11 w-11 rounded-xl bg-slate-800 border border-slate-700 text-emerald-400 font-bold text-xs flex items-center justify-center shrink-0">
                        {sp.brand_name.substring(0, 2).toUpperCase()}
                      </div>
                    )}

                    {/* Brand details */}
                    <div className="space-y-0.5">
                      <div className="flex items-center gap-2">
                        <span className={`font-bold text-white ${isTop5 ? 'text-base' : 'text-sm'}`}>
                          {sp.brand_name}
                        </span>
                        {isTop5 && (
                          <Badge
                            variant="outline"
                            className="border-amber-500/40 text-amber-400 text-[10px] py-0 px-1 font-bold"
                          >
                            Top 5 Dominan
                          </Badge>
                        )}
                        {sp.website_url && (
                          <a
                            href={sp.website_url}
                            target="_blank"
                            rel="noopener noreferrer nofollow"
                            className="text-xs text-emerald-400 hover:underline flex items-center gap-0.5 ml-1"
                            title={`Kunjungi ${sp.brand_name}`}
                          >
                            <span>Kunjungi</span>
                            <ExternalLink className="h-3 w-3" />
                          </a>
                        )}
                      </div>
                      {sp.slogan && (
                        <p className={`text-slate-300 ${isTop5 ? 'text-xs italic font-medium' : 'text-xs text-slate-400'}`}>
                          "{sp.slogan}"
                        </p>
                      )}
                    </div>
                  </div>

                  {/* Amount and Verification Date */}
                  <div className="mt-3 sm:mt-0 text-left sm:text-right shrink-0">
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

      {/* Intake & PWYW Checkout Section */}
      <section id="intake" className="py-14 bg-slate-950 border-t border-slate-800 px-4 sm:px-6 lg:px-8">
        <div className="max-w-2xl mx-auto space-y-6">
          <div className="text-center space-y-2">
            <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-xs px-3 py-1">
              Formulir Pendaftaran Backer
            </Badge>
            <h2 className="text-2xl sm:text-3xl font-extrabold text-white tracking-tight">
              Intake Profil Sponsor & Checkout Dojo
            </h2>
            <p className="text-xs sm:text-sm text-slate-400">
              Lengkapi profil brand Anda, tentukan nominal donasi Pay-What-You-Want, dan selesaikan pembayaran via Dojo Checkout.
            </p>
          </div>

          <form onSubmit={handleIntakeSubmit} className="bg-slate-900 border border-slate-800 rounded-3xl p-6 sm:p-8 space-y-6 shadow-2xl">
            {formError && (
              <Alert variant="destructive">
                <AlertTriangle className="h-4 w-4" />
                <AlertTitle>Validasi Gagal</AlertTitle>
                <AlertDescription className="text-xs">{formError}</AlertDescription>
              </Alert>
            )}

            {/* Field 1: Brand Name */}
            <div className="space-y-1.5">
              <label className="text-xs font-bold text-slate-200 flex items-center justify-between">
                <span>Nama Brand / Usaha <span className="text-emerald-400">*</span></span>
                <span className="text-[10px] text-slate-500 font-normal">Tampil di Leaderboard & README</span>
              </label>
              <div className="relative">
                <Building2 className="absolute left-3.5 top-3 h-4 w-4 text-slate-500" />
                <Input
                  type="text"
                  placeholder="Contoh: Dropsugar Studio, Acme Cloud"
                  value={brandName}
                  onChange={(e) => setBrandName(e.target.value)}
                  required
                  className="pl-10 text-xs bg-slate-950 border-slate-700 text-white placeholder:text-slate-600 focus:border-emerald-500 h-10 rounded-xl"
                />
              </div>
            </div>

            {/* Field 2: Website URL */}
            <div className="space-y-1.5">
              <label className="text-xs font-bold text-slate-200 flex items-center justify-between">
                <span>Website URL <span className="text-emerald-400">*</span></span>
                <span className="text-[10px] text-slate-500 font-normal">Tautan aman rel="noopener noreferrer nofollow"</span>
              </label>
              <div className="relative">
                <Globe className="absolute left-3.5 top-3 h-4 w-4 text-slate-500" />
                <Input
                  type="text"
                  placeholder="https://brandanda.com atau brandanda.com"
                  value={websiteUrl}
                  onChange={(e) => setWebsiteUrl(e.target.value)}
                  required
                  className="pl-10 text-xs bg-slate-950 border-slate-700 text-white placeholder:text-slate-600 focus:border-emerald-500 h-10 rounded-xl"
                />
              </div>
            </div>

            {/* Field 3: Slogan */}
            <div className="space-y-1.5">
              <label className="text-xs font-bold text-slate-200 flex items-center justify-between">
                <span>Slogan / Pesan Singkat</span>
                <span className="text-[10px] text-slate-500 font-normal">{slogan.length}/120 karakter (Anti-XSS)</span>
              </label>
              <Input
                type="text"
                maxLength={120}
                placeholder="Contoh: The All-in-One Deployment Platform for Solo Hackers"
                value={slogan}
                onChange={(e) => setSlogan(e.target.value)}
                className="text-xs bg-slate-950 border-slate-700 text-white placeholder:text-slate-600 focus:border-emerald-500 h-10 rounded-xl"
              />
            </div>

            {/* Field 4: Logo Upload with Anti-XSS Sanitization */}
            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-200 flex items-center justify-between">
                <span>Upload Logo Brand</span>
                <span className="text-[10px] text-slate-500 font-normal">SVG, PNG, WebP (Maks 2MB)</span>
              </label>

              {logoUploadError && (
                <div className="p-2.5 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-xs flex items-center gap-2">
                  <AlertTriangle className="h-4 w-4 shrink-0" />
                  <span>{logoUploadError}</span>
                </div>
              )}

              {logoPreviewUrl ? (
                <div className="p-4 rounded-2xl bg-slate-950 border border-slate-700 flex items-center justify-between gap-4">
                  <div className="flex items-center gap-3">
                    <div className="h-12 w-12 rounded-xl bg-slate-900 border border-slate-700 p-1 flex items-center justify-center overflow-hidden">
                      <img src={logoPreviewUrl} alt="Preview" className="max-h-full max-w-full object-contain" />
                    </div>
                    <div className="space-y-0.5">
                      <div className="text-xs font-bold text-white flex items-center gap-1.5">
                        <span>{logoFile?.name}</span>
                        {uploadedLogoPath && <CheckCircle2 className="h-3.5 w-3.5 text-emerald-400" />}
                      </div>
                      <div className="text-[10px] text-slate-400">
                        {uploadingLogo
                          ? 'Mengunggah & mensanitasi SVG...'
                          : uploadedLogoPath
                          ? 'Tersimpan & disanitasi aman dari XSS'
                          : 'Siap diunggah'}
                      </div>
                    </div>
                  </div>

                  <button
                    type="button"
                    onClick={handleRemoveLogo}
                    className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>
              ) : (
                <label className="border-2 border-dashed border-slate-700 hover:border-emerald-500 rounded-2xl p-5 flex flex-col items-center justify-center cursor-pointer transition-colors bg-slate-950/60">
                  <Upload className="h-6 w-6 text-slate-400 mb-2" />
                  <span className="text-xs font-semibold text-slate-200">
                    Klik untuk memilih file logo atau seret ke sini
                  </span>
                  <span className="text-[10px] text-slate-500 mt-1">
                    Format SVG disanitasi otomatis secara ketat anti-XSS
                  </span>
                  <input
                    type="file"
                    accept=".svg,.png,.webp,image/svg+xml,image/png,image/webp"
                    onChange={handleLogoChange}
                    className="hidden"
                  />
                </label>
              )}
            </div>

            {/* Field 5: Contact Info */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-200 flex items-center gap-1">
                  <Mail className="h-3.5 w-3.5 text-slate-400" />
                  <span>Email Kontak PIC <span className="text-emerald-400">*</span></span>
                </label>
                <Input
                  type="email"
                  placeholder="founder@brandanda.com"
                  value={contactEmail}
                  onChange={(e) => setContactEmail(e.target.value)}
                  required
                  className="text-xs bg-slate-950 border-slate-700 text-white placeholder:text-slate-600 focus:border-emerald-500 h-10 rounded-xl"
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-200 flex items-center gap-1">
                  <Phone className="h-3.5 w-3.5 text-slate-400" />
                  <span>WhatsApp / Telegram (Opsional)</span>
                </label>
                <Input
                  type="text"
                  placeholder="08123456789"
                  value={contactWa}
                  onChange={(e) => setContactWa(e.target.value)}
                  className="text-xs bg-slate-950 border-slate-700 text-white placeholder:text-slate-600 focus:border-emerald-500 h-10 rounded-xl"
                />
              </div>
            </div>

            {/* Field 6: Pay-What-You-Want Selector */}
            <div className="space-y-3 pt-2 border-t border-slate-800">
              <div className="flex items-center justify-between">
                <div>
                  <label className="text-xs font-bold text-white block">
                    Pilih Nominal Pay-What-You-Want (min Rp 10.000)
                  </label>
                  <p className="text-[11px] text-slate-400">
                    Kontribusi bebas sesuai apresiasi Anda. Nominal lebih tinggi menempati peringkat teratas.
                  </p>
                </div>
                <div className="text-right">
                  <span className="text-[10px] text-slate-400 block">Estimasi Ranking</span>
                  <Badge variant="outline" className="border-amber-500/40 text-amber-400 text-xs font-bold">
                    #{estimatedRank}
                  </Badge>
                </div>
              </div>

              {/* Preset Chips */}
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                {PRESET_AMOUNTS.map((preset) => {
                  const isSelected = amountIdr === preset.value
                  return (
                    <button
                      key={preset.value}
                      type="button"
                      onClick={() => handleAmountSelect(preset.value)}
                      className={`py-2 px-3 rounded-xl text-xs font-bold transition-all border ${
                        isSelected
                          ? 'bg-emerald-500 text-slate-950 border-emerald-400 shadow-md shadow-emerald-500/20'
                          : 'bg-slate-950 text-slate-300 border-slate-800 hover:border-slate-700 hover:text-white'
                      }`}
                    >
                      {preset.label}
                    </button>
                  )
                })}
              </div>

              {/* Custom Amount Input */}
              <div className="pt-1">
                <div className="relative">
                  <div className="absolute left-3.5 top-3 text-xs font-bold text-slate-500 font-mono">Rp</div>
                  <Input
                    type="text"
                    value={customAmountInput}
                    onChange={handleCustomAmountChange}
                    placeholder="Masukkan nominal custom..."
                    className="pl-10 font-mono text-sm font-bold bg-slate-950 border-slate-700 text-white focus:border-emerald-500 h-10 rounded-xl"
                  />
                </div>
                {amountIdr < 10000 && (
                  <p className="text-[10px] text-red-400 mt-1">
                    * Nominal minimal sponsorship adalah Rp 10.000
                  </p>
                )}
              </div>
            </div>

            {/* Anti-Judol Agreement Checkbox */}
            <div className="pt-2">
              <label className="flex items-start gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  checked={agreedAntiJudol}
                  onChange={(e) => setAgreedAntiJudol(e.target.checked)}
                  className="mt-0.5 rounded border-slate-700 bg-slate-950 text-emerald-500 focus:ring-emerald-500 h-4 w-4"
                  required
                />
                <span className="text-xs text-slate-300 leading-relaxed select-none">
                  Saya menjamin bahwa brand/website yang saya daftarkan{' '}
                  <strong className="text-white">bukan merupakan situs judi online (judol), pornografi, atau skema penipuan</strong>,
                  dan saya menyetujui bahwa pendaftaran yang melanggar etika ini akan ditolak pada kurasi Owner tanpa pengembalian dana.
                </span>
              </label>
            </div>

            {/* Submit / Checkout Button */}
            <div className="pt-2">
              <Button
                type="submit"
                disabled={submitting || uploadingLogo || amountIdr < 10000 || !agreedAntiJudol}
                className="w-full h-12 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-black text-sm shadow-xl shadow-emerald-500/20 gap-2 transition-transform active:scale-[0.99]"
              >
                {submitting ? (
                  <>
                    <RefreshCw className="h-4 w-4 animate-spin" />
                    <span>Menyiapkan Sesi Pembayaran Dojo...</span>
                  </>
                ) : (
                  <>
                    <Sparkles className="h-4 w-4" />
                    <span>Lanjut ke Pembayaran Dojo ({formatIDR(amountIdr)})</span>
                  </>
                )}
              </Button>
              <p className="text-[11px] text-center text-slate-500 mt-2">
                Dojo Checkout mendukung QRIS, Virtual Account (BCA, Mandiri, BNI, BRI), dan E-Wallet.
              </p>
            </div>
          </form>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-slate-800 bg-slate-950 py-8 text-xs text-slate-500 text-center">
        Cuan Founding Backers • Supported by Dojo Payment Ecosystem • Apache-2.0 License
      </footer>
    </div>
  )
}
