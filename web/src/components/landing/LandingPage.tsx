import { useState, FormEvent } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Card } from '../ui/card'
import { Badge } from '../ui/badge'
import { Alert, AlertDescription, AlertTitle } from '../ui/alert'
import {
  Rocket,
  ShieldCheck,
  Terminal,
  Database,
  Cpu,
  Layers,
  Lock,
  CheckCircle2,
  ArrowRight,
  Sparkles,
  Clock,
  HeartHandshake,
  AlertTriangle,
  FileCode,
  Menu,
  X,
  Zap,
  Check,
  ExternalLink,
  ChevronRight,
  ShieldAlert,
  Server,
  FolderGit2
} from 'lucide-react'

interface LandingPageProps {
  onNavigate: (path: string) => void
  appMode?: 'marketing' | 'studio'
  onToggleMode?: (newMode: 'marketing' | 'studio') => void
}

export function LandingPage({ onNavigate, appMode = 'marketing', onToggleMode }: LandingPageProps) {
  // Waiting list state
  const [email, setEmail] = useState('')
  const [bottomEmail, setBottomEmail] = useState('')
  const [loading, setLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [submittedEmail, setSubmittedEmail] = useState<string | null>(null)
  const [showThankYouModal, setShowThankYouModal] = useState(false)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  const validateEmail = (val: string): boolean => {
    const trimmed = val.trim()
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
    return emailRegex.test(trimmed) && trimmed.length <= 254
  }

  const handleWaitingListSubmit = async (e: FormEvent, targetEmail: string, source: string) => {
    e.preventDefault()
    setErrorMessage(null)

    const cleanEmail = targetEmail.trim()
    if (!cleanEmail) {
      setErrorMessage('Silakan masukkan alamat email Anda.')
      return
    }

    if (!validateEmail(cleanEmail)) {
      setErrorMessage('Format email tidak valid. Masukkan email yang benar (contoh: founder@domain.com).')
      return
    }

    setLoading(true)
    try {
      // Post to /api/waiting-list
      const res = await fetch('/api/waiting-list', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: cleanEmail,
          source: source,
        }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || data.message || 'Gagal mendaftar ke Waiting List')
      }

      setSubmittedEmail(cleanEmail)
      setShowThankYouModal(true)
      setEmail('')
      setBottomEmail('')
    } catch (err: any) {
      // Fallback try to /api/leads if /api/waiting-list was somehow intercepted
      try {
        const fallbackRes = await fetch('/api/leads', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email: cleanEmail, source: source }),
        })
        if (fallbackRes.ok) {
          setSubmittedEmail(cleanEmail)
          setShowThankYouModal(true)
          setEmail('')
          setBottomEmail('')
          return
        }
      } catch {
        // ignore fallback failure and show main error
      }
      setErrorMessage(err.message || 'Terjadi kesalahan saat memproses pendaftaran. Silakan coba lagi.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 flex flex-col selection:bg-emerald-500 selection:text-white">
      {/* 1. TOP NAVBAR */}
      <nav className="sticky top-0 z-40 w-full border-b border-slate-800 bg-slate-900/90 backdrop-blur-md">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3 cursor-pointer" onClick={() => onNavigate('/')}>
            <div className="h-9 w-9 rounded-xl bg-gradient-to-br from-emerald-400 to-teal-600 flex items-center justify-center shadow-lg shadow-emerald-500/20 text-slate-950 font-black text-lg">
              C
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="font-extrabold text-lg text-white tracking-tight">CUAN</span>
                <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-[10px] py-0 px-1.5 font-mono">
                  v1.0
                </Badge>
                <Badge
                  variant="outline"
                  className="border-emerald-500/40 bg-emerald-500/10 text-emerald-400 text-[10px] py-0 px-2 font-mono flex items-center gap-1"
                >
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                  Mode: {appMode}
                </Badge>
              </div>
              <p className="text-[11px] text-slate-400 hidden sm:block">Digital Project Studio</p>
            </div>
          </div>

          {/* Desktop Nav Links */}
          <div className="hidden md:flex items-center gap-6 text-sm font-medium text-slate-300">
            <a href="#masalah" className="hover:text-emerald-400 transition-colors">
              Masalah
            </a>
            <a href="#agitasi" className="hover:text-emerald-400 transition-colors">
              Agitasi
            </a>
            <a href="#solusi" className="hover:text-emerald-400 transition-colors">
              Solusi Cuan
            </a>
            <button
              onClick={() => onNavigate('/sponsor')}
              className="text-emerald-400 hover:text-emerald-300 font-semibold flex items-center gap-1 transition-colors"
            >
              <HeartHandshake className="h-4 w-4" />
              Founding Sponsor
            </button>
            <button
              onClick={() => {
                if (onToggleMode && appMode !== 'studio') {
                  onToggleMode('studio')
                }
                onNavigate('/studio')
              }}
              className="text-xs px-2.5 py-1 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 font-semibold flex items-center gap-1.5 transition-all"
              title="Buka Studio Dashboard"
            >
              <Sparkles className="h-3.5 w-3.5 text-emerald-400" />
              <span>Studio Deployer</span>
            </button>
            <button
              onClick={() => onNavigate('/app')}
              className="hover:text-white transition-colors text-slate-400 flex items-center gap-1"
            >
              <Lock className="h-3.5 w-3.5" />
              Owner Console
            </button>
          </div>

          {/* CTA & Mobile Toggle */}
          <div className="flex items-center gap-3">
            <a
              href="#waiting-list"
              className="hidden sm:inline-flex items-center justify-center px-4 py-2 rounded-lg bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-semibold text-sm transition-all shadow-md shadow-emerald-500/25 active:scale-95"
            >
              Gabung Waiting List
            </a>
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="p-2 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 md:hidden"
              aria-label="Toggle Menu"
            >
              {mobileMenuOpen ? <X className="h-6 w-6" /> : <Menu className="h-6 w-6" />}
            </button>
          </div>
        </div>

        {/* Mobile Dropdown Menu */}
        {mobileMenuOpen && (
          <div className="md:hidden border-b border-slate-800 bg-slate-900 px-4 pt-3 pb-5 space-y-3">
            <a
              href="#masalah"
              onClick={() => setMobileMenuOpen(false)}
              className="block py-2 text-sm text-slate-200 hover:text-emerald-400"
            >
              Masalah Pengembang
            </a>
            <a
              href="#agitasi"
              onClick={() => setMobileMenuOpen(false)}
              className="block py-2 text-sm text-slate-200 hover:text-emerald-400"
            >
              Dampak Tanpa Launching
            </a>
            <a
              href="#solusi"
              onClick={() => setMobileMenuOpen(false)}
              className="block py-2 text-sm text-slate-200 hover:text-emerald-400"
            >
              Solusi Cuan v1.0
            </a>
            <button
              onClick={() => {
                setMobileMenuOpen(false)
                onNavigate('/sponsor')
              }}
              className="w-full text-left py-2 text-sm text-emerald-400 font-semibold flex items-center gap-1.5"
            >
              <HeartHandshake className="h-4 w-4" />
              Founding Sponsorship (/sponsor)
            </button>
            <button
              onClick={() => {
                setMobileMenuOpen(false)
                onNavigate('/studio')
              }}
              className="w-full text-left py-2 text-sm text-emerald-400 font-medium flex items-center gap-1.5"
            >
              <Sparkles className="h-4 w-4" />
              Studio Deployer (/studio)
            </button>
            <button
              onClick={() => {
                setMobileMenuOpen(false)
                onNavigate('/app')
              }}
              className="w-full text-left py-2 text-sm text-slate-300 flex items-center gap-1.5"
            >
              <Lock className="h-4 w-4" />
              Owner Console (/admin)
            </button>
            <div className="pt-2">
              <a
                href="#waiting-list"
                onClick={() => setMobileMenuOpen(false)}
                className="w-full text-center block py-2.5 rounded-lg bg-emerald-500 text-slate-950 font-bold text-sm"
              >
                Gabung Waiting List
              </a>
            </div>
          </div>
        )}
      </nav>

      {/* 2. HERO SECTION */}
      <section className="relative pt-16 pb-20 md:pt-24 md:pb-28 overflow-hidden">
        {/* Glow Effects */}
        <div className="absolute top-1/4 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[350px] bg-emerald-500/10 blur-[130px] pointer-events-none rounded-full" />
        <div className="absolute top-1/3 right-10 w-[300px] h-[300px] bg-blue-500/10 blur-[120px] pointer-events-none rounded-full" />

        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 relative z-10 text-center">
          {/* Badge Tag */}
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-semibold mb-6">
            <Sparkles className="h-3.5 w-3.5" />
            <span>Digital Project Studio for Solo Developers & Founders</span>
          </div>

          {/* Hero Headline */}
          <h1 className="text-3xl sm:text-5xl lg:text-6xl font-black tracking-tight text-white max-w-4xl mx-auto leading-tight sm:leading-tight">
            Deploy Purchased Scripts & Tools Digital{' '}
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-emerald-400 via-teal-300 to-cyan-400">
              Tanpa Pusing Terminal dan Server.
            </span>
          </h1>

          {/* Hero Subtitle */}
          <p className="mt-6 text-base sm:text-lg text-slate-300 max-w-3xl mx-auto leading-relaxed">
            Studio mandiri untuk Solo Developers, Indie Hackers, dan Non-Tech Founders. Cukup{' '}
            <strong className="text-white font-semibold">Upload ZIP script</strong> atau hubungkan repo Git Anda —
            langsung online dengan lightweight container terisolasi, persistent database, dan reverse proxy HTTPS (Caddy) otomatis.
          </p>

          {/* Hero Waiting List Form */}
          <div id="waiting-list" className="mt-10 max-w-xl mx-auto">
            <form
              onSubmit={(e) => handleWaitingListSubmit(e, email, 'hero_cta')}
              className="flex flex-col sm:flex-row items-center gap-3 bg-slate-800/80 p-2 sm:p-2.5 rounded-2xl border border-slate-700/80 shadow-2xl backdrop-blur-md"
            >
              <div className="relative w-full">
                <Input
                  type="email"
                  placeholder="Masukkan email Anda (contoh: founder@domain.com)"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={loading}
                  required
                  className="bg-slate-900/90 border-slate-700 text-white placeholder:text-slate-500 text-sm h-12 rounded-xl focus:ring-emerald-500 focus:border-emerald-500 pl-4"
                />
              </div>
              <Button
                type="submit"
                disabled={loading}
                className="w-full sm:w-auto h-12 px-6 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm whitespace-nowrap shadow-lg shadow-emerald-500/25 transition-all"
              >
                {loading ? (
                  <span className="flex items-center gap-2">
                    <span className="h-4 w-4 border-2 border-slate-950 border-t-transparent rounded-full animate-spin" />
                    Mendaftarkan...
                  </span>
                ) : (
                  <span className="flex items-center gap-2">
                    Dapatkan Akses v1.0
                    <ArrowRight className="h-4 w-4" />
                  </span>
                )}
              </Button>
            </form>

            {errorMessage && (
              <Alert variant="destructive" className="mt-4 bg-red-950/70 border-red-800 text-red-200 text-left">
                <AlertTriangle className="h-4 w-4" />
                <AlertTitle>Validasi Pendaftaran</AlertTitle>
                <AlertDescription className="text-xs">{errorMessage}</AlertDescription>
              </Alert>
            )}

            <div className="mt-3 flex items-center justify-center gap-4 text-xs text-slate-400">
              <span className="flex items-center gap-1">
                <ShieldCheck className="h-3.5 w-3.5 text-emerald-400" />
                100% Bebas Spam
              </span>
              <span>•</span>
              <span className="flex items-center gap-1">
                <Check className="h-3.5 w-3.5 text-emerald-400" />
                Welcome Email via Dojo
              </span>
              <span>•</span>
              <span className="flex items-center gap-1">
                <Check className="h-3.5 w-3.5 text-emerald-400" />
                Akses Perdana Batch v1.0
              </span>
            </div>
          </div>

          {/* Trust Badges */}
          <div className="mt-14 pt-8 border-t border-slate-800/80 grid grid-cols-2 sm:grid-cols-4 gap-4 text-xs text-slate-400">
            <div className="flex items-center justify-center gap-2">
              <Server className="h-4 w-4 text-emerald-400" />
              <span>100% Self-Hosted</span>
            </div>
            <div className="flex items-center justify-center gap-2">
              <FileCode className="h-4 w-4 text-emerald-400" />
              <span>Open Source Apache-2.0</span>
            </div>
            <div className="flex items-center justify-center gap-2">
              <Lock className="h-4 w-4 text-emerald-400" />
              <span>Zero Vendor Lock-in</span>
            </div>
            <div className="flex items-center justify-center gap-2">
              <HeartHandshake className="h-4 w-4 text-emerald-400" />
              <span>Dojo Ecosystem Backed</span>
            </div>
          </div>
        </div>
      </section>

      {/* 3. PROBLEM (P) SECTION */}
      <section id="masalah" className="py-20 bg-slate-950 border-t border-b border-slate-800/80 relative">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <Badge variant="outline" className="border-red-500/40 text-red-400 text-xs py-1 px-3 mb-3 uppercase tracking-wider">
              Problem • Realita Lapangan
            </Badge>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-white tracking-tight">
              Beli Script Ratusan Dolar, Tapi Hanya Mengendap di Hard Drive?
            </h2>
            <p className="mt-4 text-slate-400 text-base leading-relaxed">
              Sebagai solo developer atau entrepreneur, Anda punya segudang ide produk digital. Namun perjalanan dari file script menjadi aplikasi live seringkali menjadi mimpi buruk teknis.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {/* Problem Card 1 */}
            <Card className="bg-slate-900/90 border-slate-800 hover:border-red-500/40 transition-all p-6">
              <div className="h-12 w-12 rounded-xl bg-red-500/10 text-red-400 flex items-center justify-center mb-5">
                <FolderGit2 className="h-6 w-6" />
              </div>
              <h3 className="text-lg font-bold text-white mb-2">1. Purchased Code Sitting Idle</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Anda sudah beli script PHP, WordPress theme, atau web tool di CodeCanyon, Dojo, atau Envato ($30–$300). Tapi file <code className="text-red-300">.zip</code> hanya mengendap di laptop karena setup server, Docker, database, dan Nginx/Caddy terlalu rumit.
              </p>
            </Card>

            {/* Problem Card 2 */}
            <Card className="bg-slate-900/90 border-slate-800 hover:border-red-500/40 transition-all p-6">
              <div className="h-12 w-12 rounded-xl bg-red-500/10 text-red-400 flex items-center justify-center mb-5">
                <ShieldAlert className="h-6 w-6" />
              </div>
              <h3 className="text-lg font-bold text-white mb-2">2. Tergantung Freelancer Mahal</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Membayar jasa freelancer $50–$100/jam hanya untuk instalasi script dasar atau mengubah hal kecil: ganti nomor WhatsApp, ubah link tombol, atau pasang payment gateway. Menguras runway modal sebelum bisnis sempat menghasilkan.
              </p>
            </Card>

            {/* Problem Card 3 */}
            <Card className="bg-slate-900/90 border-slate-800 hover:border-red-500/40 transition-all p-6">
              <div className="h-12 w-12 rounded-xl bg-red-500/10 text-red-400 flex items-center justify-center mb-5">
                <AlertTriangle className="h-6 w-6" />
              </div>
              <h3 className="text-lg font-bold text-white mb-2">3. Portofolio Tercecer Tanpa Kendali</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Mengelola banyak web tools, situs Adsense, tools affiliate, dan script mini di berbagai cPanel murahan. Tidak ada dashboard terpusat untuk memantau status hidup/matinya aset dan memastikan server tetap stabil.
              </p>
            </Card>
          </div>
        </div>
      </section>

      {/* 4. AGITATION (A) SECTION */}
      <section id="agitasi" className="py-20 bg-slate-900 relative">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <Badge variant="outline" className="border-amber-500/40 text-amber-400 text-xs py-1 px-3 mb-3 uppercase tracking-wider">
              Agitation • Dampak & Konsekuensi
            </Badge>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-white tracking-tight">
              Setiap Hari yang Terbuang Tanpa Launching,{' '}
              <span className="text-amber-400">Potensi Cuan Menguap.</span>
            </h2>
            <p className="mt-4 text-slate-400 text-base leading-relaxed">
              Waktu adalah aset paling berharga bagi seorang indie hacker. Ketika Anda terjebak dalam perangkap teknis konfigurasi server, inilah harga mahal yang sedang Anda bayar:
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="p-6 rounded-2xl bg-gradient-to-b from-slate-800 to-slate-900 border border-slate-800 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-400">
                  <Clock className="h-5 w-5" />
                </div>
                <h4 className="font-bold text-base text-white">Kehilangan Momentum & Cuan</h4>
              </div>
              <p className="text-sm text-slate-400 leading-relaxed">
                Peluang pasar dan tren audiens tidak menunggu Anda. Kompetitor dengan produk lebih sederhana sudah mulai mengumpulkan pengguna dan cuan duluan hanya karena mereka launch 2 minggu lebih cepat.
              </p>
            </div>

            <div className="p-6 rounded-2xl bg-gradient-to-b from-slate-800 to-slate-900 border border-slate-800 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-400">
                  <Zap className="h-5 w-5" />
                </div>
                <h4 className="font-bold text-base text-white">Modal Runway Bocor</h4>
              </div>
              <p className="text-sm text-slate-400 leading-relaxed">
                Terus membayar biaya sewa VPS, domain, dan subscription SaaS cloud bulanan yang mahal, sementara website dan script belum menghasilkan sepeser pun karena setup yang belum kelar-kelar.
              </p>
            </div>

            <div className="p-6 rounded-2xl bg-gradient-to-b from-slate-800 to-slate-900 border border-slate-800 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-400">
                  <Terminal className="h-5 w-5" />
                </div>
                <h4 className="font-bold text-base text-white">Burnout Teknis Malam Hari</h4>
              </div>
              <p className="text-sm text-slate-400 leading-relaxed">
                Alih-alih fokus ke strategi pemasaran dan kepuasan pelanggan, Anda habis energi begadang menatap layar hitam SSH, membetulkan error <code className="text-amber-300">502 Bad Gateway</code>, dan SSL certificate yang kedaluwarsa.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* 5. SOLUTION (S) SECTION */}
      <section id="solusi" className="py-20 bg-slate-950 border-t border-slate-800 relative">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-xs py-1 px-3 mb-3 uppercase tracking-wider">
              Solution • Terobosan Cuan v1.0
            </Badge>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-white tracking-tight">
              Cuan v1.0: Digital Project Studio Mandiri{' '}
              <span className="text-emerald-400">Tanpa Pusing Server.</span>
            </h2>
            <p className="mt-4 text-slate-400 text-base leading-relaxed">
              Cuan memperlakukan setiap aset digital Anda sebagai <strong className="text-white">1 Digital Project mandiri</strong>. Mengubah file ZIP menjadi bisnis beroperasi dengan zero-friction deployment.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {/* Feature 1 */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center font-bold">
                <Rocket className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">Universal ZIP & Git Input</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Cukup upload file <code className="text-emerald-300 font-mono">.zip</code> script Anda (WordPress, PHP, Node.js, Python, Static). GitHub opsional bagi developer teknis. Cuan mendeteksi stack otomatis.
              </p>
            </div>

            {/* Feature 2 */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center font-bold">
                <ShieldCheck className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">Turnkey Container & Auto-SSL</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Setiap project diisolasi dalam lightweight container Docker dengan volume persistent dan HTTPS reverse proxy otomatis (Caddy). Zero error SSL selamanya.
              </p>
            </div>

            {/* Feature 3 */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center font-bold">
                <Layers className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">Visual Environment Editor</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Lupakan kerumitan mengedit file <code className="text-emerald-300 font-mono">.env</code> manual via SSH nano. Atur konfigurasi database dan API keys lewat formulir visual yang bersih dan aman.
              </p>
            </div>

            {/* Feature 4 */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center font-bold">
                <Cpu className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">Agentic AI Co-Pilot (Chat-to-Modify)</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Ingin mengubah nomor WhatsApp tombol, edit warna tema, atau inject script Facebook Pixel? Cukup berikan prompt teks biasa. AI assistant menguji di staging sebelum live.
              </p>
            </div>

            {/* Feature 5 */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center font-bold">
                <Lock className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">100% Self-Hosted & Bebas Biaya</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Lisensi open-source Apache-2.0. Berjalan di VPS milik Anda sendiri. Data, kode, dan database Anda tidak pernah dikunci oleh vendor pihak ketiga.
              </p>
            </div>

            {/* Feature 6 */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center font-bold">
                <Database className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">Automated Health & Backup</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Pemantauan kesehatan aplikasi 24/7. Backup database otomatis dan kemampuan rollback 1-klik jika terjadi kesalahan modifikasi.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* 6. HOW IT WORKS (3 SIMPLE STEPS) */}
      <section className="py-20 bg-slate-900">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-2xl mx-auto mb-16">
            <h2 className="text-3xl font-extrabold text-white tracking-tight">
              Alur Kerja 3 Langkah Menuju Cuan
            </h2>
            <p className="mt-3 text-slate-400 text-sm">
              Sangat intuitif bahkan untuk pengguna non-teknis tanpa pengalaman Linux.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            <div className="relative text-center p-6 bg-slate-850 rounded-2xl border border-slate-800">
              <div className="w-12 h-12 rounded-full bg-emerald-500 text-slate-950 font-black text-xl flex items-center justify-center mx-auto mb-4">
                1
              </div>
              <h3 className="font-bold text-lg text-white mb-2">Upload Script / ZIP</h3>
              <p className="text-sm text-slate-400">
                Tarik dan lepaskan file script yang baru Anda beli, atau sambungkan repositori GitHub Anda.
              </p>
            </div>

            <div className="relative text-center p-6 bg-slate-850 rounded-2xl border border-slate-800">
              <div className="w-12 h-12 rounded-full bg-emerald-500 text-slate-950 font-black text-xl flex items-center justify-center mx-auto mb-4">
                2
              </div>
              <h3 className="font-bold text-lg text-white mb-2">Konfigurasi Visual & Deploy</h3>
              <p className="text-sm text-slate-400">
                Isi judul, domain, dan kunci env melalui antarmuka visual. Klik deploy dan Cuan menyiapkan segalanya.
              </p>
            </div>

            <div className="relative text-center p-6 bg-slate-850 rounded-2xl border border-slate-800">
              <div className="w-12 h-12 rounded-full bg-emerald-500 text-slate-950 font-black text-xl flex items-center justify-center mx-auto mb-4">
                3
              </div>
              <h3 className="font-bold text-lg text-white mb-2">Live & Siap Hasilkan Cuan</h3>
              <p className="text-sm text-slate-400">
                Aplikasi Anda aktif di internet dengan HTTPS aman. Pantau performa dan kembangkan tanpa hambatan.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* 7. FOUNDING SPONSOR BRIDGE TEASER */}
      <section className="py-16 bg-gradient-to-r from-slate-950 via-slate-900 to-slate-950 border-t border-b border-emerald-500/20">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 text-center space-y-6">
          <Badge variant="outline" className="border-emerald-500/50 text-emerald-400 text-xs py-1 px-3 uppercase tracking-wider">
            Inisiatif Komunitas • Pay-What-You-Want
          </Badge>
          <h2 className="text-2xl sm:text-4xl font-extrabold text-white">
            Ingin Brand Anda Tampil Abadi di Rilis Cuan v1.0?
          </h2>
          <p className="text-slate-300 max-w-2xl mx-auto text-sm sm:text-base leading-relaxed">
            Dukung pengembangan open-source Cuan sebagai Founding Backer. Dapatkan posisi prestisius di Leaderboard Homepage, logo permanen di <code className="text-emerald-400">README.md</code> repositori GitHub buchin/cuan, dan dofollow link seumur hidup (min Rp 10.000 via Dojo payment).
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-4">
            <button
              onClick={() => onNavigate('/sponsor')}
              className="inline-flex items-center gap-2 px-6 py-3 rounded-xl bg-gradient-to-r from-emerald-500 to-teal-500 hover:from-emerald-400 hover:to-teal-400 text-slate-950 font-bold text-sm shadow-xl shadow-emerald-500/20 transition-all active:scale-95"
            >
              <HeartHandshake className="h-4 w-4" />
              Lihat Founding Backer di /sponsor
              <ArrowRight className="h-4 w-4" />
            </button>
            <button
              onClick={() => onNavigate('/sponsors')}
              className="inline-flex items-center gap-1.5 px-5 py-3 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-sm font-semibold transition-all border border-slate-700"
            >
              Direktori Backer (/sponsors)
            </button>
          </div>
        </div>
      </section>

      {/* 8. BOTTOM WAITING LIST CTA */}
      <section className="py-20 bg-slate-950 relative">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
          <div className="p-8 sm:p-12 rounded-3xl bg-gradient-to-b from-slate-900 to-slate-950 border border-slate-800 shadow-2xl relative overflow-hidden">
            <div className="absolute top-0 right-0 w-64 h-64 bg-emerald-500/10 blur-[90px] rounded-full pointer-events-none" />

            <h2 className="text-2xl sm:text-4xl font-black text-white tracking-tight">
              Jadilah yang Pertama Menggunakan Cuan v1.0
            </h2>
            <p className="mt-4 text-sm sm:text-base text-slate-300 max-w-xl mx-auto leading-relaxed">
              Daftarkan email Anda sekarang. Kami akan mengirimkan Welcome Email dan notifikasi prioritas saat rilis perdana siap dipasang di VPS Anda.
            </p>

            <form
              onSubmit={(e) => handleWaitingListSubmit(e, bottomEmail, 'footer_cta')}
              className="mt-8 max-w-md mx-auto flex flex-col sm:flex-row gap-3"
            >
              <Input
                type="email"
                placeholder="founder@domain.com"
                value={bottomEmail}
                onChange={(e) => setBottomEmail(e.target.value)}
                disabled={loading}
                required
                className="bg-slate-950 border-slate-700 text-white placeholder:text-slate-500 text-sm h-12 rounded-xl focus:ring-emerald-500 focus:border-emerald-500"
              />
              <Button
                type="submit"
                disabled={loading}
                className="h-12 px-6 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm whitespace-nowrap shadow-lg shadow-emerald-500/25"
              >
                {loading ? 'Mendaftarkan...' : 'Gabung Waiting List'}
              </Button>
            </form>

            <p className="mt-4 text-xs text-slate-400">
              Integrasi Dojo Autoresponder aktif. Email sambutan akan otomatis dikirimkan ke inbox Anda.
            </p>
          </div>
        </div>
      </section>

      {/* 9. FOOTER */}
      <footer className="border-t border-slate-800 bg-slate-950 py-10 text-xs text-slate-400">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <span className="font-bold text-slate-200">CUAN</span>
            <span>— Digital Project Studio for Entrepreneurs</span>
          </div>
          <div className="flex items-center gap-6">
            <button onClick={() => onNavigate('/sponsor')} className="hover:text-emerald-400 transition-colors">
              Founding Sponsor
            </button>
            <button onClick={() => onNavigate('/sponsors')} className="hover:text-emerald-400 transition-colors">
              Direktori Backers
            </button>
            <button onClick={() => onNavigate('/studio')} className="hover:text-emerald-400 text-emerald-400 transition-colors font-semibold">
              Studio Deployer
            </button>
            <button onClick={() => onNavigate('/terms')} className="hover:text-emerald-400 transition-colors">
              Terms
            </button>
            <button onClick={() => onNavigate('/privacy')} className="hover:text-emerald-400 transition-colors">
              Privacy
            </button>
            <button onClick={() => onNavigate('/app')} className="hover:text-emerald-400 transition-colors">
              Console
            </button>
            <a
              href="https://github.com/buchin/cuan"
              target="_blank"
              rel="noreferrer"
              className="hover:text-emerald-400 transition-colors inline-flex items-center gap-1"
            >
              GitHub
              <ExternalLink className="h-3 w-3" />
            </a>
          </div>
        </div>
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 mt-4 text-center sm:text-left text-slate-400">
          Licensed under Apache-2.0. Author: Buchin &lt;buchin@dropsugar.com&gt;. Zero-friction self-hosted deployment engine.
        </div>
      </footer>

      {/* 10. THANK YOU MODAL / STATE */}
      {showThankYouModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="relative w-full max-w-lg bg-slate-900 border border-slate-700/80 rounded-3xl shadow-2xl p-6 sm:p-8 space-y-6 text-left">
            {/* Close Button */}
            <button
              onClick={() => setShowThankYouModal(false)}
              className="absolute top-5 right-5 text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800"
            >
              <X className="h-5 w-5" />
            </button>

            {/* Success Header */}
            <div className="flex items-start gap-4">
              <div className="h-12 w-12 rounded-2xl bg-emerald-500/20 text-emerald-400 flex items-center justify-center shrink-0 border border-emerald-500/40">
                <CheckCircle2 className="h-7 w-7" />
              </div>
              <div className="space-y-1">
                <h3 className="text-xl font-black text-white">Selamat! Anda Terdaftar</h3>
                <p className="text-xs text-slate-400">
                  Email <strong className="text-emerald-400">{submittedEmail}</strong> telah dicatat di Waiting List Cuan v1.0.
                </p>
              </div>
            </div>

            {/* Dojo Autoresponder Note */}
            <div className="p-4 rounded-xl bg-slate-850 border border-slate-700 text-xs text-slate-300 space-y-2">
              <div className="flex items-center gap-2 font-semibold text-emerald-400">
                <Sparkles className="h-4 w-4" />
                <span>Welcome Email Dikirim via Dojo</span>
              </div>
              <p>
                Sistem autoresponder Dojo telah memicu Welcome & Thank You Email ke alamat Anda. Silakan periksa inbox (atau tab Updates/Spam) dalam beberapa menit.
              </p>
            </div>

            {/* Bridge to Founding Sponsorship */}
            <div className="p-5 rounded-2xl bg-gradient-to-br from-emerald-950/40 to-slate-900 border border-emerald-500/30 space-y-3">
              <div className="flex items-center gap-2">
                <HeartHandshake className="h-5 w-5 text-emerald-400" />
                <h4 className="font-bold text-sm text-white">Dukung Cuan: Jadi Founding Backer</h4>
              </div>
              <p className="text-xs text-slate-300 leading-relaxed">
                Ingin nama/brand Anda tampil di <strong>Leaderboard Homepage v1.0</strong>, logo abadi di <strong>README.md</strong> GitHub, dan link dofollow permanen?
                Dukung kami dengan skema <em>Pay-What-You-Want</em> (minimal Rp 10.000 via Dojo payment).
              </p>
              <Button
                onClick={() => {
                  setShowThankYouModal(false)
                  onNavigate('/sponsor')
                }}
                className="w-full h-11 bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm gap-2 shadow-lg shadow-emerald-500/20"
              >
                <HeartHandshake className="h-4 w-4" />
                Menjadi Founding Sponsor di /sponsor
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>

            {/* Dismiss button */}
            <div className="pt-1 text-center">
              <button
                onClick={() => setShowThankYouModal(false)}
                className="text-xs text-slate-400 hover:text-white underline underline-offset-4"
              >
                Tutup & Kembali ke Beranda
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
