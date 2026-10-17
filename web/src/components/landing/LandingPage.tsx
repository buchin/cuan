import { useState, FormEvent } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Card } from '../ui/card'
import { Badge } from '../ui/badge'
import { Alert, AlertDescription, AlertTitle } from '../ui/alert'
import {
  Cpu,
  Layers,
  Lock,
  CheckCircle2,
  ArrowRight,
  Sparkles,
  Clock,
  HeartHandshake,
  AlertTriangle,
  Menu,
  X,
  Zap,
  ExternalLink,
  ShieldAlert,
  Server,
  BookOpen,
  Video,
  ShoppingBag,
  Percent,
  Bot,
  KeyRound,
  UploadCloud,
  Globe,
  FileArchive,
  Sliders
} from 'lucide-react'

interface LandingPageProps {
  onNavigate: (path: string) => void
  appMode?: 'marketing' | 'studio'
  onToggleMode?: (newMode: 'marketing' | 'studio') => void
}

export function LandingPage({ onNavigate, appMode = 'marketing', onToggleMode }: LandingPageProps) {
  // Waiting list state
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
      setErrorMessage('Format email tidak valid. Masukkan email yang benar (contoh: creator@domain.com).')
      return
    }

    setLoading(true)
    try {
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
      setBottomEmail('')
    } catch (err: any) {
      try {
        const fallbackRes = await fetch('/api/leads', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email: cleanEmail, source: source }),
        })
        if (fallbackRes.ok) {
          setSubmittedEmail(cleanEmail)
          setShowThankYouModal(true)
          setBottomEmail('')
          return
        }
      } catch {
        // ignore fallback failure
      }
      setErrorMessage(err.message || 'Terjadi kesalahan saat memproses pendaftaran. Silakan coba lagi.')
    } finally {
      setLoading(false)
    }
  }

  const navigateToStudio = () => {
    if (onToggleMode && appMode !== 'studio') {
      onToggleMode('studio')
    }
    onNavigate('/studio')
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
              <p className="text-[11px] text-slate-400 hidden sm:block">Agentic Project Management for Creators</p>
            </div>
          </div>

          {/* Desktop Nav Links */}
          <div className="hidden md:flex items-center gap-6 text-sm font-medium text-slate-300">
            <a href="#masalah" className="hover:text-emerald-400 transition-colors">
              Masalah Kreator
            </a>
            <a href="#solusi" className="hover:text-emerald-400 transition-colors">
              Solusi Cuan
            </a>
            <a href="#blueprints" className="hover:text-emerald-400 transition-colors">
              Blueprint Katalog
            </a>
            <button
              onClick={() => onNavigate('/sponsor')}
              className="text-emerald-400 hover:text-emerald-300 font-semibold flex items-center gap-1 transition-colors"
            >
              <HeartHandshake className="h-4 w-4" />
              Founding Sponsor
            </button>
            <button
              onClick={navigateToStudio}
              className="text-xs px-3 py-1.5 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 font-bold flex items-center gap-1.5 transition-all shadow-xs"
              title="Buka Studio Dashboard"
            >
              <Sparkles className="h-3.5 w-3.5 text-emerald-400" />
              <span>Buka Studio</span>
            </button>
            <button
              onClick={() => onNavigate('/app')}
              className="hover:text-white transition-colors text-slate-400 flex items-center gap-1 text-xs"
            >
              <Lock className="h-3.5 w-3.5" />
              Console
            </button>
          </div>

          {/* CTA & Mobile Toggle */}
          <div className="flex items-center gap-3">
            <button
              onClick={navigateToStudio}
              className="hidden sm:inline-flex items-center justify-center px-4 py-2 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm transition-all shadow-md shadow-emerald-500/25 active:scale-95 gap-1.5"
            >
              <Sparkles className="h-4 w-4" />
              <span>Mulai di Studio</span>
            </button>
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
              Masalah Kreator
            </a>
            <a
              href="#solusi"
              onClick={() => setMobileMenuOpen(false)}
              className="block py-2 text-sm text-slate-200 hover:text-emerald-400"
            >
              Solusi Cuan
            </a>
            <a
              href="#blueprints"
              onClick={() => setMobileMenuOpen(false)}
              className="block py-2 text-sm text-slate-200 hover:text-emerald-400"
            >
              Blueprint Katalog
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
                navigateToStudio()
              }}
              className="w-full text-left py-2 text-sm text-emerald-400 font-bold flex items-center gap-1.5"
            >
              <Sparkles className="h-4 w-4" />
              Buka Studio (/studio)
            </button>
            <div className="pt-2">
              <button
                onClick={() => {
                  setMobileMenuOpen(false)
                  navigateToStudio()
                }}
                className="w-full text-center block py-2.5 rounded-xl bg-emerald-500 text-slate-950 font-bold text-sm"
              >
                Mulai di Studio Sekarang
              </button>
            </div>
          </div>
        )}
      </nav>

      {/* 2. HERO SECTION */}
      <section className="relative pt-16 pb-20 md:pt-24 md:pb-28 overflow-hidden">
        {/* Glow Effects */}
        <div className="absolute top-1/4 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[350px] bg-emerald-500/10 blur-[130px] pointer-events-none rounded-full" />
        <div className="absolute top-1/3 right-10 w-[300px] h-[300px] bg-purple-500/10 blur-[120px] pointer-events-none rounded-full" />

        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 relative z-10 text-center">
          {/* Badge Tag */}
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-semibold mb-6">
            <Sparkles className="h-3.5 w-3.5" />
            <span>Agentic Project Management for Creators</span>
          </div>

          {/* Hero Headline */}
          <h1 className="text-3xl sm:text-5xl lg:text-6xl font-black tracking-tight text-white max-w-4xl mx-auto leading-tight sm:leading-tight">
            Kelola Proyek Kreator & Eksekusi Konten{' '}
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-emerald-400 via-teal-300 to-cyan-400">
              Tanpa To-Do List Mati.
            </span>
          </h1>

          {/* Hero Subtitle */}
          <p className="mt-6 text-base sm:text-lg text-slate-300 max-w-3xl mx-auto leading-relaxed">
            Platform all-in-one untuk <strong className="text-white">Blogger, YouTuber, Olshop, dan Affiliate</strong>.
            Mulai instan dari template blueprint, jalankan AI Agent langsung di kartu task Kanban, dan dapatkan draf konten,
            skrip video, serta sales copy berkualitas tinggi dengan Google Gemini Pro tanpa fragmentasi tools.
          </p>

          {/* Hero Action CTAs */}
          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-3.5 max-w-lg mx-auto">
            <Button
              onClick={navigateToStudio}
              className="w-full sm:w-auto h-12 px-8 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm shadow-xl shadow-emerald-500/25 flex items-center justify-center gap-2 transition-all active:scale-95"
            >
              <Sparkles className="h-4 w-4" />
              <span>Buka Studio Sekarang</span>
              <ArrowRight className="h-4 w-4" />
            </Button>

            <a
              href="#blueprints"
              className="w-full sm:w-auto h-12 px-6 rounded-xl bg-slate-800/80 hover:bg-slate-700/80 text-white font-semibold text-sm border border-slate-700 flex items-center justify-center gap-2 transition-all"
            >
              <Layers className="h-4 w-4 text-emerald-400" />
              <span>Eksplor 4 Blueprint</span>
            </a>
          </div>

          {/* Trust Badges */}
          <div className="mt-14 pt-8 border-t border-slate-800/80 grid grid-cols-2 sm:grid-cols-4 gap-4 text-xs text-slate-400">
            <div className="flex items-center justify-center gap-2">
              <Layers className="h-4 w-4 text-emerald-400" />
              <span>4 Blueprint Alur Kerja</span>
            </div>
            <div className="flex items-center justify-center gap-2">
              <Bot className="h-4 w-4 text-purple-400" />
              <span>In-Task Agent Runner</span>
            </div>
            <div className="flex items-center justify-center gap-2">
              <KeyRound className="h-4 w-4 text-amber-400" />
              <span>BYOK Key Vault Aman</span>
            </div>
            <div className="flex items-center justify-center gap-2">
              <Server className="h-4 w-4 text-emerald-400" />
              <span>100% Self-Hosted Apache-2.0</span>
            </div>
          </div>
        </div>
      </section>

      {/* 3. PROBLEM (MASALAH KREATOR) SECTION */}
      <section id="masalah" className="py-20 bg-slate-950 border-t border-b border-slate-800/80 relative">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <Badge variant="outline" className="border-red-500/40 text-red-400 text-xs py-1 px-3 mb-3 uppercase tracking-wider">
              Problem • Realita Lapangan Kreator
            </Badge>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-white tracking-tight">
              Punya Ratusan Ide Konten, Tapi Berakhir Jadi To-Do List Mati?
            </h2>
            <p className="mt-4 text-slate-400 text-base leading-relaxed">
              Sebagai solopreneur dan kreator multi-channel, Anda dituntut konsisten memproduksi konten setiap hari. Namun kenyataannya, energi Anda terkuras habis oleh manajemen tugas manual dan tools yang terpecah-pecah.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            {/* Problem Card 1 */}
            <Card className="bg-slate-900/90 border-slate-800 hover:border-red-500/40 transition-all p-6 space-y-3">
              <div className="h-12 w-12 rounded-xl bg-red-500/10 text-red-400 flex items-center justify-center mb-2">
                <ShieldAlert className="h-6 w-6" />
              </div>
              <h3 className="text-lg font-bold text-white">1. To-Do List Mati di Backlog</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Trello, Notion, atau Jira biasa hanya menjadi kuburan ide. Kartu task menumpuk berbulan-bulan di kolom Backlog tanpa pernah dieksekusi karena Anda tidak punya tim asisten, researcher, atau scriptwriter sendiri.
              </p>
            </Card>

            {/* Problem Card 2 */}
            <Card className="bg-slate-900/90 border-slate-800 hover:border-red-500/40 transition-all p-6 space-y-3">
              <div className="h-12 w-12 rounded-xl bg-red-500/10 text-red-400 flex items-center justify-center mb-2">
                <AlertTriangle className="h-6 w-6" />
              </div>
              <h3 className="text-lg font-bold text-white">2. Overwhelmed Multi-Channel</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Harus meriset SEO blog, menulis skrip video 10 menit, menyusun email nurture jualan produk digital, dan membuat komparasi affiliate sekaligus. Mengganti konteks kerja terus-menerus memicu creative burnout yang parah.
              </p>
            </Card>

            {/* Problem Card 3 */}
            <Card className="bg-slate-900/90 border-slate-800 hover:border-red-500/40 transition-all p-6 space-y-3">
              <div className="h-12 w-12 rounded-xl bg-red-500/10 text-red-400 flex items-center justify-center mb-2">
                <Zap className="h-6 w-6" />
              </div>
              <h3 className="text-lg font-bold text-white">3. Tools AI Terfragmentasi</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Langganan belasan SaaS AI secara terpisah (ChatGPT, Jasper, Copy.ai) dengan format yang berantakan, tagihan per bulan membengkak, dan prompt terbuang sia-sia tanpa tersinkronisasi ke alur kerja proyek nyata.
              </p>
            </Card>

            {/* Problem Card 4 (PO Explicit Requirement) */}
            <Card className="bg-slate-900/90 border-slate-800 hover:border-red-500/40 transition-all p-6 space-y-3">
              <div className="h-12 w-12 rounded-xl bg-red-500/10 text-red-400 flex items-center justify-center mb-2">
                <FileArchive className="h-6 w-6" />
              </div>
              <h3 className="text-lg font-bold text-white">4. Script Teronggok di Hard Disk</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Membeli script web di CodeCanyon/Envato/GitHub, tapi mengendap berbulan-bulan karena bingung setup .env, bingung impor skema database .sql, instalasi dependensi rumit, dan kebingungan deploy ke server/VPS.
              </p>
            </Card>
          </div>
        </div>
      </section>

      {/* 4. AGITATION (Dampak Tanpa Eksekusi) */}
      <section id="agitasi" className="py-20 bg-slate-900 relative">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <Badge variant="outline" className="border-amber-500/40 text-amber-400 text-xs py-1 px-3 mb-3 uppercase tracking-wider">
              Agitation • Dampak Tanpa Eksekusi Agentic
            </Badge>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-white tracking-tight">
              Setiap Hari yang Terbuang Tanpa Eksekusi,{' '}
              <span className="text-amber-400">Audiens & Potensi Cuan Menguap.</span>
            </h2>
            <p className="mt-4 text-slate-400 text-base leading-relaxed">
              Konsistensi adalah mata uang utama di ekonomi kreator. Ketika Anda terjebak dalam rutinitas administratif tanpa bantuan agent terpadu, inilah harga mahal yang Anda bayar:
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="p-6 rounded-2xl bg-gradient-to-b from-slate-800 to-slate-900 border border-slate-800 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-400">
                  <Clock className="h-5 w-5" />
                </div>
                <h4 className="font-bold text-base text-white">Kehilangan Momentum Tren</h4>
              </div>
              <p className="text-sm text-slate-400 leading-relaxed">
                Topik yang sedang viral di YouTube atau Google Search tidak menunggu Anda. Kompetitor yang bergerak cepat merebut traffic audiens hanya karena mereka bisa publish draf konten 10x lebih cepat.
              </p>
            </div>

            <div className="p-6 rounded-2xl bg-gradient-to-b from-slate-800 to-slate-900 border border-slate-800 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-400">
                  <Zap className="h-5 w-5" />
                </div>
                <h4 className="font-bold text-base text-white">Biaya Langganan Bocor</h4>
              </div>
              <p className="text-sm text-slate-400 leading-relaxed">
                Membayar puluhan subscription tool AI dan VPS tanpa menghasilkan konversi nyata. Modal terkuras sebelum aset digital dan funnel jualan Anda sempat menghasilkan cuan.
              </p>
            </div>

            <div className="p-6 rounded-2xl bg-gradient-to-b from-slate-800 to-slate-900 border border-slate-800 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-400">
                  <Cpu className="h-5 w-5" />
                </div>
                <h4 className="font-bold text-base text-white">Creative Burnout Berkepanjangan</h4>
              </div>
              <p className="text-sm text-slate-400 leading-relaxed">
                Alih-alih menikmati proses kreasi dan interaksi dengan audiens, energi Anda habis untuk menyalin prompt, merapikan format teks manual, dan mengatur file dokumen yang berserakan.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* 5. SOLUTION (SOLUSI CUAN: AGENTIC CREATOR STUDIO) */}
      <section id="solusi" className="py-20 bg-slate-950 border-t border-slate-800 relative">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-xs py-1 px-3 mb-3 uppercase tracking-wider">
              Solution • Terobosan Cuan
            </Badge>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-white tracking-tight">
              Cuan: Ubah Task Menjadi Deliverable Nyata{' '}
              <span className="text-emerald-400">dengan In-Task AI Agent.</span>
            </h2>
            <p className="mt-4 text-slate-400 text-base leading-relaxed">
              Cuan bukan to-do list pasif. Setiap task dihubungkan langsung ke AI Agent persona yang siap mengeksekusi instruksi, menghasilkan draf konten berkualitas tinggi, dan otomatis mencentang kriteria Definition of Done (DoD).
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {/* Feature 1: AI Script Onboarding from ZIP */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center font-bold">
                <UploadCloud className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">AI Script Onboarding (from ZIP)</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Drop arsip ZIP script web hasil beli di marketplace. AI Inspector membedah tech stack, ekstrak konfigurasi .env, mendeteksi skema database .sql, dan merangkai setup pipeline otomatis di Kanban.
              </p>
            </div>

            {/* Feature 2: Modular Workspace OS */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-indigo-500/10 text-indigo-400 flex items-center justify-center font-bold">
                <Sliders className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">Modular Workspace OS</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Navigasi adaptif tanpa bloatware. Sidebar dan navbar hanya me-render modul aktif proyek (tasks, discussions, repos, servers, publishing, support) dengan fleksibilitas "+ Add Modules" kapan saja.
              </p>
            </div>

            {/* Feature 3: 1-Click WordPress Publishing */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-blue-500/10 text-blue-400 flex items-center justify-center font-bold">
                <Globe className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">1-Click WordPress Publishing</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Publikasikan artikel deliverable task langsung ke WordPress REST API (status draft/publish) dalam 1 klik dengan enkripsi Application Password AES-256-GCM tanpa risiko kebocoran kredensial.
              </p>
            </div>

            {/* Feature 4: Fullscreen Teleprompter */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-red-500/10 text-red-400 flex items-center justify-center font-bold">
                <Video className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">Fullscreen Teleprompter Studio</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Mode teleprompter fullscreen untuk YouTuber & video creator. Dilengkapi auto-scroll halus terkontrol kecepatan, pengatur ukuran font, mode mirror horizontal, dan shortcut keyboard.
              </p>
            </div>

            {/* Feature 5: In-Task Agent Runner */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-purple-500/10 text-purple-400 flex items-center justify-center font-bold">
                <Bot className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">In-Task Agent Runner & Blueprints</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Tekan tombol "Run with Agent" pada kartu task Kanban. Agent membaca instruksi prompt & DoD, mengeksekusi pembuatan konten via Google Gemini Pro, dan otomatis mencentang kriteria DoD.
              </p>
            </div>

            {/* Feature 6: 100% Self-Hosted & Zero Lock-in */}
            <div className="p-6 rounded-2xl bg-slate-900 border border-slate-800 hover:border-emerald-500/40 transition-all space-y-3">
              <div className="h-10 w-10 rounded-xl bg-teal-500/10 text-teal-400 flex items-center justify-center font-bold">
                <Server className="h-5 w-5" />
              </div>
              <h3 className="text-lg font-bold text-white">100% Self-Hosted & Key Vault (BYOK)</h3>
              <p className="text-sm text-slate-400 leading-relaxed">
                Lisensi open-source Apache-2.0. Berjalan di server VPS milik Anda sendiri. Seluruh data proyek, draf konten, kredensial dan database tersimpan aman di bawah kendali penuh Anda.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* 6. BLUEPRINT CATALOG SHOWCASE SECTION */}
      <section id="blueprints" className="py-20 bg-slate-900 relative">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <Badge variant="outline" className="border-emerald-500/40 text-emerald-400 text-xs py-1 px-3 mb-3 uppercase tracking-wider">
              Katalog Blueprint • Instant Production Pipeline
            </Badge>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-white tracking-tight">
              4 Template Siap Pakai untuk Setiap Jenis Kreator
            </h2>
            <p className="mt-4 text-slate-400 text-base leading-relaxed">
              Tidak perlu merancang workflow dan prompt dari nol. Pilih blueprint yang sesuai, dan Cuan otomatis menyiapkan ruang kolaborasi, alur pipeline task, serta AI Agent persona khusus.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Blueprint 1: Blog Publisher */}
            <div className="p-6 rounded-3xl bg-slate-950 border border-slate-800 hover:border-emerald-500/40 transition-all flex flex-col justify-between space-y-6">
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="h-12 w-12 rounded-2xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center">
                    <BookOpen className="h-6 w-6" />
                  </div>
                  <Badge className="bg-emerald-500/15 text-emerald-400 border border-emerald-500/30 text-[10px] font-mono">
                    SEO & Content
                  </Badge>
                </div>
                <div>
                  <h3 className="text-xl font-bold text-white">Blog Publisher</h3>
                  <p className="text-xs text-slate-400 mt-1 leading-relaxed">
                    Pipeline pembuatan konten SEO otomatis untuk niche blogger dan publisher Google AdSense.
                  </p>
                </div>
                <div className="p-3.5 rounded-xl bg-slate-900 border border-slate-800/80 space-y-2 text-xs">
                  <div className="flex items-center gap-2 text-emerald-400 font-semibold">
                    <Bot className="h-4 w-4" />
                    <span>Agent Persona: SEO Specialist</span>
                  </div>
                  <div className="text-slate-400 text-[11px] space-y-1">
                    <p>• Riset Keyword Niche & 5 Topic Cluster</p>
                    <p>• Draf Artikel Pilar 2.000+ Kata Teroptimasi SEO</p>
                    <p>• Optimasi On-Page, Alt Text & JSON-LD Schema</p>
                  </div>
                </div>
              </div>

              <Button
                onClick={navigateToStudio}
                className="w-full bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs h-10 rounded-xl gap-2"
              >
                <span>Pakai Blueprint Blog</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Button>
            </div>

            {/* Blueprint 2: YouTube Channel */}
            <div className="p-6 rounded-3xl bg-slate-950 border border-slate-800 hover:border-red-500/40 transition-all flex flex-col justify-between space-y-6">
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="h-12 w-12 rounded-2xl bg-red-500/10 text-red-400 flex items-center justify-center">
                    <Video className="h-6 w-6" />
                  </div>
                  <Badge className="bg-red-500/15 text-red-400 border border-red-500/30 text-[10px] font-mono">
                    Video & Script
                  </Badge>
                </div>
                <div>
                  <h3 className="text-xl font-bold text-white">YouTube Channel</h3>
                  <p className="text-xs text-slate-400 mt-1 leading-relaxed">
                    Alur kerja produksi video end-to-end dari riset tren, hook retensi tinggi, skrip 10 menit, hingga metadata.
                  </p>
                </div>
                <div className="p-3.5 rounded-xl bg-slate-900 border border-slate-800/80 space-y-2 text-xs">
                  <div className="flex items-center gap-2 text-red-400 font-semibold">
                    <Bot className="h-4 w-4" />
                    <span>Agent Persona: YouTube Scriptwriter</span>
                  </div>
                  <div className="text-slate-400 text-[11px] space-y-1">
                    <p>• Ideasi Video Viral & Analisis Minat Penonton</p>
                    <p>• 5 Opsi Judul CTR Tinggi & 30-Detik Hook Pertama</p>
                    <p>• Full Scriptwriting 10 Menit lengkap Narasi & Visual Cues</p>
                  </div>
                </div>
              </div>

              <Button
                onClick={navigateToStudio}
                className="w-full bg-red-600 hover:bg-red-500 text-white font-bold text-xs h-10 rounded-xl gap-2"
              >
                <span>Pakai Blueprint YouTube</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Button>
            </div>

            {/* Blueprint 3: Online Shop / Digital Product */}
            <div className="p-6 rounded-3xl bg-slate-950 border border-slate-800 hover:border-blue-500/40 transition-all flex flex-col justify-between space-y-6">
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="h-12 w-12 rounded-2xl bg-blue-500/10 text-blue-400 flex items-center justify-center">
                    <ShoppingBag className="h-6 w-6" />
                  </div>
                  <Badge className="bg-blue-500/15 text-blue-400 border border-blue-500/30 text-[10px] font-mono">
                    Funnel & Sales
                  </Badge>
                </div>
                <div>
                  <h3 className="text-xl font-bold text-white">Online Shop / Digital Product</h3>
                  <p className="text-xs text-slate-400 mt-1 leading-relaxed">
                    Copywriting penjualan berkonversi tinggi, urutan email nurture 3-part, dan matriks penanganan keberatan.
                  </p>
                </div>
                <div className="p-3.5 rounded-xl bg-slate-900 border border-slate-800/80 space-y-2 text-xs">
                  <div className="flex items-center gap-2 text-blue-400 font-semibold">
                    <Bot className="h-4 w-4" />
                    <span>Agent Persona: Direct-Response Copywriter</span>
                  </div>
                  <div className="text-slate-400 text-[11px] space-y-1">
                    <p>• Sales Page Copy Formula PAS (Problem-Agitate-Solution)</p>
                    <p>• 3-Part Email Nurture (Welcome, Story, Hard Pitch)</p>
                    <p>• 7 Jawaban Pertanyaan Umum & Objection Handling FAQ</p>
                  </div>
                </div>
              </div>

              <Button
                onClick={navigateToStudio}
                className="w-full bg-blue-600 hover:bg-blue-500 text-white font-bold text-xs h-10 rounded-xl gap-2"
              >
                <span>Pakai Blueprint Olshop</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Button>
            </div>

            {/* Blueprint 4: Affiliate Site */}
            <div className="p-6 rounded-3xl bg-slate-950 border border-slate-800 hover:border-amber-500/40 transition-all flex flex-col justify-between space-y-6">
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="h-12 w-12 rounded-2xl bg-amber-500/10 text-amber-400 flex items-center justify-center">
                    <Percent className="h-6 w-6" />
                  </div>
                  <Badge className="bg-amber-500/15 text-amber-400 border border-amber-500/30 text-[10px] font-mono">
                    Affiliate & Review
                  </Badge>
                </div>
                <div>
                  <h3 className="text-xl font-bold text-white">Affiliate Site</h3>
                  <p className="text-xs text-slate-400 mt-1 leading-relaxed">
                    Matriks komparasi produk, breakdown fitur mendalam, dan review berorientasi konversi klik affiliate.
                  </p>
                </div>
                <div className="p-3.5 rounded-xl bg-slate-900 border border-slate-800/80 space-y-2 text-xs">
                  <div className="flex items-center gap-2 text-amber-400 font-semibold">
                    <Bot className="h-4 w-4" />
                    <span>Agent Persona: Affiliate Reviewer</span>
                  </div>
                  <div className="text-slate-400 text-[11px] space-y-1">
                    <p>• Matriks Komparasi 3 Produk Kompetitor Niche</p>
                    <p>• Review Mendalam 1.500 Kata (Verdict, Pros & Cons)</p>
                    <p>• 3 Variasi Tombol CTA Berkonversi Tinggi</p>
                  </div>
                </div>
              </div>

              <Button
                onClick={navigateToStudio}
                className="w-full bg-amber-600 hover:bg-amber-500 text-white font-bold text-xs h-10 rounded-xl gap-2"
              >
                <span>Pakai Blueprint Affiliate</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </div>
      </section>

      {/* 7. HOW IT WORKS (3 SIMPLE STEPS) */}
      <section className="py-20 bg-slate-950 border-t border-slate-800">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-2xl mx-auto mb-16">
            <h2 className="text-3xl font-extrabold text-white tracking-tight">
              Alur Kerja 3 Langkah Menuju Konten Siap Pakai
            </h2>
            <p className="mt-3 text-slate-400 text-sm">
              Sangat intuitif dan zero-friction untuk semua jenis kreator.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            <div className="relative text-center p-6 bg-slate-900 rounded-2xl border border-slate-800 space-y-3">
              <div className="w-12 h-12 rounded-full bg-emerald-500 text-slate-950 font-black text-xl flex items-center justify-center mx-auto mb-2">
                1
              </div>
              <h3 className="font-bold text-lg text-white">Pilih Blueprint Kreator</h3>
              <p className="text-xs text-slate-400 leading-relaxed">
                Pilih template Blog, YouTube, Olshop, atau Affiliate di Studio Wizard untuk langsung menghasilkan papan Kanban dan alur task terstruktur.
              </p>
            </div>

            <div className="relative text-center p-6 bg-slate-900 rounded-2xl border border-slate-800 space-y-3">
              <div className="w-12 h-12 rounded-full bg-purple-500 text-white font-black text-xl flex items-center justify-center mx-auto mb-2">
                2
              </div>
              <h3 className="font-bold text-lg text-white">Klik "Run with Agent"</h3>
              <p className="text-xs text-slate-400 leading-relaxed">
                Tekan tombol Run with Agent pada kartu task. AI Agent membaca instruksi & kriteria DoD lalu mengeksekusi konten otomatis via Google Gemini Pro.
              </p>
            </div>

            <div className="relative text-center p-6 bg-slate-900 rounded-2xl border border-slate-800 space-y-3">
              <div className="w-12 h-12 rounded-full bg-emerald-500 text-slate-950 font-black text-xl flex items-center justify-center mx-auto mb-2">
                3
              </div>
              <h3 className="font-bold text-lg text-white">Buka Output & Salin 1-Klik</h3>
              <p className="text-xs text-slate-400 leading-relaxed">
                Buka tab Agent Output pada Task Detail, periksa hasil render Markdown bersih, salin dengan 1-klik, dan gunakan langsung untuk publikasi!
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* 8. FOUNDING SPONSOR BRIDGE TEASER */}
      <section className="py-16 bg-gradient-to-r from-slate-950 via-slate-900 to-slate-950 border-t border-b border-emerald-500/20">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 text-center space-y-6">
          <Badge variant="outline" className="border-emerald-500/50 text-emerald-400 text-xs py-1 px-3 uppercase tracking-wider">
            Inisiatif Komunitas • Pay-What-You-Want
          </Badge>
          <h2 className="text-2xl sm:text-4xl font-extrabold text-white">
            Ingin Brand Anda Tampil Abadi di Rilis Cuan v1.0?
          </h2>
          <p className="text-slate-300 max-w-2xl mx-auto text-sm sm:text-base leading-relaxed">
            Dukung pengembangan open-source Cuan sebagai Founding Backer. Dapatkan posisi prestisius di Leaderboard Homepage, logo permanen di <code className="text-emerald-400 font-mono">README.md</code> repositori GitHub buchin/cuan, dan dofollow link seumur hidup (min Rp 10.000 via Dojo payment).
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-4">
            <button
              onClick={() => onNavigate('/sponsor')}
              className="inline-flex items-center gap-2 px-6 py-3 rounded-xl bg-gradient-to-r from-emerald-500 to-teal-500 hover:from-emerald-400 hover:to-teal-400 text-slate-950 font-bold text-sm shadow-xl shadow-emerald-500/20 transition-all active:scale-95"
            >
              <HeartHandshake className="h-4 w-4" />
              <span>Lihat Founding Backer di /sponsor</span>
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

      {/* 9. BOTTOM WAITING LIST & STUDIO CTA */}
      <section className="py-20 bg-slate-950 relative">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
          <div className="p-8 sm:p-12 rounded-3xl bg-gradient-to-b from-slate-900 to-slate-950 border border-slate-800 shadow-2xl relative overflow-hidden">
            <div className="absolute top-0 right-0 w-64 h-64 bg-emerald-500/10 blur-[90px] rounded-full pointer-events-none" />

            <h2 className="text-2xl sm:text-4xl font-black text-white tracking-tight">
              Mulai Bangun Proyek Kreator Anda Hari Ini
            </h2>
            <p className="mt-4 text-sm sm:text-base text-slate-300 max-w-xl mx-auto leading-relaxed">
              Cuan sudah siap digunakan di Studio Dashboard. Buka Studio sekarang untuk membuat proyek dari Blueprint atau daftarkan email Anda untuk update rilis berkala.
            </p>

            <div className="mt-8 flex flex-col sm:flex-row items-center justify-center gap-3">
              <Button
                onClick={navigateToStudio}
                className="w-full sm:w-auto h-12 px-8 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm shadow-lg shadow-emerald-500/25 flex items-center justify-center gap-2"
              >
                <Sparkles className="h-4 w-4" />
                <span>Masuk ke Cuan Studio</span>
                <ArrowRight className="h-4 w-4" />
              </Button>
            </div>

            <form
              onSubmit={(e) => handleWaitingListSubmit(e, bottomEmail, 'footer_cta')}
              className="mt-8 max-w-md mx-auto flex flex-col sm:flex-row gap-3 pt-6 border-t border-slate-800/80"
            >
              <Input
                type="email"
                placeholder="creator@domain.com"
                value={bottomEmail}
                onChange={(e) => setBottomEmail(e.target.value)}
                disabled={loading}
                required
                className="bg-slate-950 border-slate-700 text-white placeholder:text-slate-500 text-xs h-11 rounded-xl focus:ring-emerald-500 focus:border-emerald-500"
              />
              <Button
                type="submit"
                disabled={loading}
                className="h-11 px-5 rounded-xl bg-slate-800 hover:bg-slate-700 text-white font-bold text-xs whitespace-nowrap border border-slate-700"
              >
                {loading ? 'Mendaftarkan...' : 'Update Email'}
              </Button>
            </form>

            {errorMessage && (
              <Alert variant="destructive" className="mt-3 bg-red-950/70 border-red-800 text-red-200 text-left max-w-md mx-auto">
                <AlertTriangle className="h-4 w-4" />
                <AlertTitle>Validasi Pendaftaran</AlertTitle>
                <AlertDescription className="text-xs">{errorMessage}</AlertDescription>
              </Alert>
            )}

            <p className="mt-4 text-xs text-slate-400">
              Integrasi Dojo Autoresponder aktif. Email sambutan otomatis dikirimkan ke inbox Anda.
            </p>
          </div>
        </div>
      </section>

      {/* 10. FOOTER */}
      <footer className="border-t border-slate-800 bg-slate-950 py-10 text-xs text-slate-400">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <span className="font-bold text-slate-200">CUAN</span>
            <span>— Agentic Project Management for Creators</span>
          </div>
          <div className="flex items-center gap-6">
            <button onClick={() => onNavigate('/sponsor')} className="hover:text-emerald-400 transition-colors">
              Founding Sponsor
            </button>
            <button onClick={() => onNavigate('/sponsors')} className="hover:text-emerald-400 transition-colors">
              Direktori Backers
            </button>
            <button onClick={navigateToStudio} className="hover:text-emerald-400 text-emerald-400 transition-colors font-semibold">
              Cuan Studio
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
          Licensed under Apache-2.0. Author: Buchin &lt;buchin@dropsugar.com&gt;. Agentic Project Management, Blueprint Engine & In-Task Execution for Creators.
        </div>
      </footer>

      {/* 11. THANK YOU MODAL */}
      {showThankYouModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="relative w-full max-w-lg bg-slate-900 border border-slate-700/80 rounded-3xl shadow-2xl p-6 sm:p-8 space-y-6 text-left">
            <button
              onClick={() => setShowThankYouModal(false)}
              className="absolute top-5 right-5 text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800"
            >
              <X className="h-5 w-5" />
            </button>

            <div className="flex items-start gap-4">
              <div className="h-12 w-12 rounded-2xl bg-emerald-500/20 text-emerald-400 flex items-center justify-center shrink-0 border border-emerald-500/40">
                <CheckCircle2 className="h-7 w-7" />
              </div>
              <div className="space-y-1">
                <h3 className="text-xl font-bold text-white tracking-tight">
                  Pendaftaran Berhasil!
                </h3>
                <p className="text-xs text-slate-400">
                  Email Anda telah terdaftar di ekosistem Cuan Creator Studio.
                </p>
              </div>
            </div>

            <div className="p-4 rounded-2xl bg-slate-950 border border-slate-800 space-y-2 text-xs">
              <span className="text-slate-400 block font-mono text-[11px]">Terdaftar untuk:</span>
              <span className="text-emerald-400 font-mono font-bold text-sm block">
                {submittedEmail}
              </span>
            </div>

            <div className="flex items-center justify-end gap-3 pt-2">
              <Button
                onClick={() => {
                  setShowThankYouModal(false)
                  navigateToStudio()
                }}
                className="bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-xs h-10 px-5 rounded-xl gap-1.5"
              >
                <span>Buka Cuan Studio</span>
                <ArrowRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
