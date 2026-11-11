import { useState, useRef } from 'react'
import { Link } from 'react-router-dom'
import {
  ArrowRight,
  ChevronRight,
  GitBranch,
  Github,
  MessageSquare,
  Mail,
  ShieldCheck,
  CheckCircle2,
  Zap,
  Volume2,
  Activity,
  Users,
  Sun,
  Moon,
  Clock,
  Coins,
  AlertCircle,
  Bot,
  Terminal,
  Check,
  X,
  Minus,
  Database,
  Loader2,
  Cpu,
  Server,
  Layers,
  Sparkles,
} from 'lucide-react'
import { CuanMark } from '../../components/brand/CuanMark'
import { ApprovalCard } from '../inbox/ApprovalCard'
import { useLanguage } from '../../context/LanguageContext'
import { useTheme } from '../../context/ThemeContext'

const repository = 'https://github.com/buchin/cuan'

export function LandingPage() {
  const { t, language, setPreference } = useLanguage()
  const { resolvedTheme, toggleTheme } = useTheme()

  const [email, setEmail] = useState('')
  const [otpCode, setOtpCode] = useState('')
  const [waitingListStep, setWaitingListStep] = useState<'email' | 'otp' | 'confirmed'>('email')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [showGithubModal, setShowGithubModal] = useState(false)
  const emailInputRef = useRef<HTMLInputElement>(null)
  const otpInputRef = useRef<HTMLInputElement>(null)

  const scrollToWaitingList = () => {
    setShowGithubModal(false)
    const el = document.getElementById('waiting-list')
    if (el) {
      el.scrollIntoView({ behavior: 'smooth' })
      setTimeout(() => {
        if (waitingListStep === 'otp') {
          otpInputRef.current?.focus()
        } else {
          emailInputRef.current?.focus()
        }
      }, 400)
    }
  }

  const handleRequestOtp = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = email.trim()
    if (!trimmed || !trimmed.includes('@')) {
      setFormError(t('Masukkan alamat email yang valid.'))
      return
    }

    setIsSubmitting(true)
    setFormError(null)

    try {
      // 1. Submit to Dojo OTP Magic Link endpoint for cuan-waiting-list product gate
      const dojoRes = await fetch('https://dojo.cc/api/v1/auth/magic-link', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: trimmed, product: 'cuan-waiting-list' }),
      })

      // 2. Submit to local Cuan waiting list intake (background fail-safe)
      fetch('/api/waiting-list', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: trimmed, source: 'hero_waiting_list' }),
      }).catch(() => {})

      if (!dojoRes.ok) {
        const data = await dojoRes.json().catch(() => ({}))
        throw new Error(data.error || t('Gagal mengirim kode verifikasi. Coba lagi sebentar lagi.'))
      }

      setWaitingListStep('otp')
      setTimeout(() => {
        otpInputRef.current?.focus()
      }, 100)
    } catch (err: any) {
      setFormError(err.message || t('Terjadi gangguan jaringan saat mengirim kode verifikasi.'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleVerifyOtp = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmedOtp = otpCode.trim()
    if (trimmedOtp.length !== 6) {
      setFormError(t('Masukkan 6 angka kode verifikasi dengan lengkap.'))
      return
    }

    setIsSubmitting(true)
    setFormError(null)

    try {
      const res = await fetch('https://dojo.cc/api/v1/auth/verify-otp', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim(), otp_code: trimmedOtp }),
      })

      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(
          data.error === 'Invalid code'
            ? t('Kode konfirmasi salah. Silakan periksa kembali email Anda.')
            : (data.error || t('Kode verifikasi tidak sesuai atau telah kedaluwarsa.'))
        )
      }

      setWaitingListStep('confirmed')
    } catch (err: any) {
      setFormError(err.message || t('Kode konfirmasi tidak sesuai atau telah kedaluwarsa.'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleResendOtp = async () => {
    setIsSubmitting(true)
    setFormError(null)
    try {
      await fetch('https://dojo.cc/api/v1/auth/magic-link', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim(), product: 'cuan-waiting-list' }),
      })
      setFormError(t('Kode verifikasi baru telah dikirim ke email Anda.'))
    } catch {
      setFormError(t('Gagal mengirim ulang kode. Silakan coba sebentar lagi.'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="min-h-dvh bg-background text-foreground">
      {/* Navigation Header */}
      <header className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-6 sm:px-8">
        <Link
          to="/"
          className="inline-flex min-h-11 items-center gap-2 font-medium tracking-tight"
          aria-label="Cuan"
        >
          <CuanMark decorative className="h-7 w-7" />
          <span className="font-display text-xl font-bold tracking-tight text-foreground">
            cuan
          </span>
        </Link>
        <nav
          aria-label={t('Website navigation')}
          className="flex items-center gap-2 sm:gap-4"
        >
          <a
            href="#problem"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors whitespace-nowrap sm:inline-flex"
          >
            {t('Masalah')}
          </a>
          <a
            href="#how-it-works"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors whitespace-nowrap sm:inline-flex"
          >
            {t('Cara kerja')}
          </a>
          <a
            href="#ecosystem"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors whitespace-nowrap lg:inline-flex"
          >
            {t('Integrasi & MCP')}
          </a>
          <a
            href="#comparison"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors whitespace-nowrap sm:inline-flex"
          >
            {t('Perbandingan')}
          </a>
          <a
            href="#features"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors whitespace-nowrap sm:inline-flex"
          >
            {t('Fitur')}
          </a>
          <Link
            to="/sponsor"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors whitespace-nowrap lg:inline-flex"
          >
            {t('Sponsor')}
          </Link>
          <button
            type="button"
            onClick={() => setShowGithubModal(true)}
            className="inline-flex min-h-9 items-center gap-1.5 rounded-md border border-border/50 bg-muted/30 px-2.5 text-xs font-medium text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors cursor-pointer whitespace-nowrap"
            title="GitHub Repository"
          >
            <Github className="h-3.5 w-3.5" strokeWidth={1.5} />
            <span className="hidden sm:inline">GitHub</span>
          </button>

          {/* Language Switcher */}
          <button
            type="button"
            onClick={() => setPreference(language === 'id' ? 'en' : 'id')}
            className="inline-flex min-h-9 items-center justify-center rounded-md border border-border/50 bg-muted/30 px-2.5 text-xs font-medium text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors shrink-0"
            title={language === 'id' ? 'Ganti ke English' : 'Switch to Bahasa Indonesia'}
          >
            {language === 'id' ? 'ID' : 'EN'}
          </button>

          {/* Theme Switcher */}
          <button
            type="button"
            onClick={toggleTheme}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border/50 bg-muted/30 text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors shrink-0"
            aria-label="Ganti tema"
            title={resolvedTheme === 'dark' ? 'Tema terang' : 'Tema gelap'}
          >
            {resolvedTheme === 'dark' ? (
              <Sun className="h-4 w-4" strokeWidth={1.5} />
            ) : (
              <Moon className="h-4 w-4" strokeWidth={1.5} />
            )}
          </button>

          {/* Waiting List Primary Action Button (desktop/tablet only; mobile uses the prominent Hero form directly below) */}
          <button
            type="button"
            onClick={scrollToWaitingList}
            className="action-primary hidden sm:inline-flex min-h-10 px-4 text-xs sm:text-sm font-semibold shadow-xs cursor-pointer whitespace-nowrap shrink-0"
          >
            {t('Gabung Waiting List')}
          </button>
        </nav>
      </header>

      <main>
        {/* 1. HOOK / HERO SECTION (Above the fold on standard MacBook 1440x900) */}
        <section
          className="relative mx-auto max-w-6xl px-6 pb-12 pt-6 text-center sm:px-8 sm:pb-16 sm:pt-8"
          aria-labelledby="landing-title"
        >
          <div className="mx-auto max-w-3xl">
            <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-border/60 bg-muted/40 px-3.5 py-1.5 text-xs font-semibold text-muted-foreground">
              <span className="h-2 w-2 rounded-full bg-emerald-500" aria-hidden="true" />
              <span>{t('AI Project Manager untuk Marketer')}</span>
            </div>
            <h1
              id="landing-title"
              className="font-display text-5xl font-bold tracking-tight text-foreground sm:text-6xl md:text-7xl lg:text-[76px] lg:leading-[1.08]"
            >
              {t('Chat. Setujui. Beres.')}
            </h1>
            <p className="mt-4 text-lg font-normal text-muted-foreground sm:text-xl md:text-2xl leading-relaxed">
              {t('Kelola bisnis tanpa pusing urusan teknis.')}
            </p>

            {/* 2-Step Waiting List Form with Dojo OTP & Sponsorship CTA */}
            <div id="waiting-list" className="mt-8 max-w-lg mx-auto scroll-mt-24">
              {waitingListStep === 'email' && (
                <div className="space-y-3">
                  <form onSubmit={handleRequestOtp} className="flex flex-col sm:flex-row gap-2.5">
                    <input
                      ref={emailInputRef}
                      type="email"
                      required
                      value={email}
                      onChange={(e) => {
                        setEmail(e.target.value)
                        setFormError(null)
                      }}
                      placeholder={t('Masukkan email Anda...')}
                      aria-label={t('Email untuk waiting list')}
                      className="min-h-12 flex-1 rounded-xl border border-border/70 bg-card px-4 text-base text-foreground placeholder:text-muted-foreground/60 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary shadow-2xs transition-colors"
                      disabled={isSubmitting}
                    />
                    <button
                      type="submit"
                      disabled={isSubmitting || !email.trim()}
                      className="action-primary min-h-12 px-6 text-base font-semibold shadow-xs shrink-0 inline-flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
                    >
                      {isSubmitting ? (
                        <>
                          <Loader2 className="h-4 w-4 animate-spin" />
                          <span>{t('Mengirim...')}</span>
                        </>
                      ) : (
                        <>
                          <span>{t('Gabung Waiting List')}</span>
                          <ArrowRight className="h-4 w-4" strokeWidth={1.5} />
                        </>
                      )}
                    </button>
                  </form>
                  {formError && (
                    <p className="text-xs text-rose-500 font-medium text-center">{formError}</p>
                  )}
                  <p className="text-xs text-muted-foreground text-center">
                    🔒 {t('Gratis · Tanpa kartu kredit · Prioritas rilis v1.0 & akses repositori')}
                  </p>
                </div>
              )}

              {waitingListStep === 'otp' && (
                <div className="rounded-2xl border border-border/80 bg-card p-5 sm:p-6 shadow-xs text-left space-y-4">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <div className="inline-flex items-center gap-1.5 text-xs font-semibold text-primary">
                        <Mail className="h-3.5 w-3.5" />
                        <span>{t('Verifikasi Email 6 Angka')}</span>
                      </div>
                      <h3 className="font-display text-lg font-bold text-foreground mt-0.5">
                        {t('Masukkan Kode Konfirmasi')}
                      </h3>
                      <p className="text-xs sm:text-sm text-muted-foreground mt-1">
                        {t('Kode verifikasi telah dikirim ke')}{' '}
                        <span className="font-semibold text-foreground">{email}</span>. {t('Silakan cek inbox atau folder spam.')}
                      </p>
                    </div>
                  </div>

                  <form onSubmit={handleVerifyOtp} className="space-y-3">
                    <div className="flex flex-col sm:flex-row gap-2.5">
                      <input
                        ref={otpInputRef}
                        type="text"
                        inputMode="numeric"
                        pattern="[0-9]*"
                        maxLength={6}
                        required
                        value={otpCode}
                        onChange={(e) => {
                          setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                          setFormError(null)
                        }}
                        placeholder="123456"
                        aria-label={t('6 digit kode verifikasi')}
                        className="min-h-12 flex-1 rounded-xl border border-border/70 bg-background px-4 text-center font-mono text-xl tracking-widest font-bold text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary shadow-2xs"
                        disabled={isSubmitting}
                      />
                      <button
                        type="submit"
                        disabled={isSubmitting || otpCode.trim().length !== 6}
                        className="action-primary min-h-12 px-6 text-base font-semibold shadow-xs shrink-0 inline-flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
                      >
                        {isSubmitting ? (
                          <>
                            <Loader2 className="h-4 w-4 animate-spin" />
                            <span>{t('Memverifikasi...')}</span>
                          </>
                        ) : (
                          <>
                            <span>{t('Verifikasi & Amankan Posisi')}</span>
                            <CheckCircle2 className="h-4 w-4" strokeWidth={1.5} />
                          </>
                        )}
                      </button>
                    </div>

                    {formError && (
                      <p className="text-xs text-rose-500 font-medium">{formError}</p>
                    )}

                    <div className="flex items-center justify-between text-xs text-muted-foreground pt-1">
                      <button
                        type="button"
                        onClick={() => {
                          setWaitingListStep('email')
                          setFormError(null)
                        }}
                        className="hover:text-foreground underline underline-offset-2 transition-colors cursor-pointer"
                      >
                        {t('Ganti email')}
                      </button>
                      <button
                        type="button"
                        onClick={handleResendOtp}
                        disabled={isSubmitting}
                        className="hover:text-foreground underline underline-offset-2 transition-colors cursor-pointer"
                      >
                        {t('Kirim ulang kode')}
                      </button>
                    </div>
                  </form>
                </div>
              )}

              {waitingListStep === 'confirmed' && (
                <div className="rounded-2xl border border-emerald-500/30 bg-emerald-500/5 dark:bg-emerald-500/10 p-5 sm:p-6 text-left space-y-4 shadow-xs">
                  <div className="flex items-center gap-2.5 text-emerald-600 dark:text-emerald-400 font-bold text-base sm:text-lg">
                    <CheckCircle2 className="h-5 w-5 shrink-0" strokeWidth={2} />
                    <span>{t('Posisi Anda di Waiting List Cuan v1.0 Resmi Terdaftar!')}</span>
                  </div>
                  <p className="text-sm text-foreground/90 leading-relaxed">
                    {t('Email Anda telah terverifikasi.')} {t('Kami telah mengirimkan email sambutan dan konfirmasi ke')}{' '}
                    <span className="font-semibold text-foreground">{email}</span>. {t('Notifikasi rilis v1.0 dan akses repositori akan dikirimkan langsung ke inbox Anda.')}
                  </p>

                  <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 space-y-2.5">
                    <div className="inline-flex items-center gap-1.5 text-xs font-semibold text-primary uppercase tracking-wider">
                      <Sparkles className="h-3.5 w-3.5" />
                      <span>{t('Akselerasi Peluncuran')}</span>
                    </div>
                    <div className="font-display text-base sm:text-lg font-bold text-foreground">
                      {t('Ingin akses VIP pertama & ikut mempercepat peluncuran Cuan?')}
                    </div>
                    <p className="text-xs sm:text-sm text-muted-foreground leading-relaxed">
                      {t('Jadilah Founding Sponsor untuk mendapatkan akses early-build, logo & tautan permanen di repositori GitHub & landing page, serta sesi konsultasi instalasi VPS.')}
                    </p>
                    <div className="pt-2">
                      <Link
                        to="/sponsor"
                        className="action-primary min-h-10 px-5 text-xs sm:text-sm font-semibold inline-flex items-center gap-2"
                      >
                        <span>{t('Lihat Halaman Sponsor & Benefit')}</span>
                        <ArrowRight className="h-3.5 w-3.5" />
                      </Link>
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="mt-6 flex flex-wrap items-center justify-center gap-3.5">
              <a
                href="#problem"
                className="action-secondary min-h-12 px-6 text-base font-semibold"
              >
                {t('Pelajari Cara Kerja')}
                <ChevronRight className="h-4 w-4" strokeWidth={1.5} />
              </a>
            </div>
          </div>

          {/* Centered 3-Device Mockup Showcase - Optimized Above the Fold */}
          <div className="mx-auto mt-8 max-w-4xl sm:mt-10">
            <div className="relative">
              <img
                src="/landing/cuan.webp"
                alt={t('Cuan multi-device workspace on Laptop, Tablet, and Mobile')}
                width={2000}
                height={1088}
                className="max-h-[480px] w-full object-contain [filter:drop-shadow(0_16px_36px_rgba(0,0,0,0.08))_drop-shadow(0_4px_12px_rgba(0,0,0,0.03))]"
                loading="eager"
              />
            </div>
          </div>

          {/* Trust Highlights */}
          <div className="mx-auto mt-8 flex flex-wrap items-center justify-center gap-x-8 gap-y-3 text-xs sm:text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <ShieldCheck className="h-4 w-4" strokeWidth={1.5} />
              {t('Open source under Apache-2.0')}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <GitBranch className="h-4 w-4" strokeWidth={1.5} />
              {t('Self-hosted on your own VPS')}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Zap className="h-4 w-4" strokeWidth={1.5} />
              {t('Bebas biaya token dobel · Bawa akun resmi atau API key sendiri (Claude, ChatGPT, Gemini, DeepSeek)')}
            </span>
          </div>
        </section>

        {/* 1.5. OPEN ECOSYSTEM & INTEGRATION (Bring Your Models, Channels, Apps, Skills & Custom MCP) */}
        <section
          id="ecosystem"
          className="scroll-mt-6 border-t border-border/60 bg-muted/10 px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="ecosystem-title"
        >
          <div className="mx-auto max-w-6xl">
            <div className="max-w-3xl">
              <div className="mb-3 inline-flex items-center gap-2 text-xs font-semibold text-muted-foreground">
                <Cpu className="h-4 w-4 text-primary" strokeWidth={1.5} />
                <span>{t('Ekosistem Terbuka & Integrasi')}</span>
              </div>
              <h2
                id="ecosystem-title"
                className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl md:text-4xl"
              >
                {t('Satu Sistem Mandiri untuk Seluruh Model AI, Aplikasi, & MCP')}
              </h2>
              <p className="mt-3 text-base sm:text-lg leading-relaxed text-muted-foreground">
                {t(
                  'Bebas dari vendor lock-in. Cuan berjalan di server VPS milik Anda sendiri, menghubungkan model AI resmi favorit, kanal komunikasi bisnis, dan standar terbuka Model Context Protocol (MCP).',
                )}
              </p>
            </div>

            <div className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
              {/* CARD 1: MODEL AI RESMI */}
              <div className="rounded-xl border border-border/60 bg-card p-6 shadow-2xs transition-colors hover:border-border/80">
                <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-muted/60 text-muted-foreground">
                  <Bot className="h-5 w-5" strokeWidth={1.5} />
                </div>
                <div className="text-xs font-semibold text-primary mb-1 uppercase tracking-wider">
                  Multi-Provider & BYOK
                </div>
                <h3 className="font-display text-lg font-bold text-foreground">
                  {t('Model AI Resmi (Bebas Biaya Dobel)')}
                </h3>
                <p className="mt-2.5 text-sm sm:text-base leading-relaxed text-muted-foreground">
                  {t(
                    'Hubungkan akun resmi Anda (Claude Pro, ChatGPT Plus, Gemini) atau pasang API key langsung (DeepSeek, OpenAI) tanpa markup margin token. Bebas ganti model sesuai kebutuhan tugas.',
                  )}
                </p>
                <div className="mt-4 flex flex-wrap gap-2 text-xs font-mono text-muted-foreground">
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">Claude 3.7</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">GPT-4o</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">Gemini 2.5</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">DeepSeek R1</span>
                </div>
              </div>

              {/* CARD 2: KANAL OPERASIONAL BISNIS */}
              <div className="rounded-xl border border-border/60 bg-card p-6 shadow-2xs transition-colors hover:border-border/80">
                <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-muted/60 text-muted-foreground">
                  <Server className="h-5 w-5" strokeWidth={1.5} />
                </div>
                <div className="text-xs font-semibold text-primary mb-1 uppercase tracking-wider">
                  24/7 Always-On VPS
                </div>
                <h3 className="font-display text-lg font-bold text-foreground">
                  {t('Infrastruktur Bisnis & Saluran Komunikasi')}
                </h3>
                <p className="mt-2.5 text-sm sm:text-base leading-relaxed text-muted-foreground">
                  {t(
                    'Agen terus bekerja di VPS Ubuntu/Debian saat Anda tidur atau laptop ditutup. Dilengkapi widget Live Chat web, peringatan email Resend untuk obrolan idle, dan webhook order otomatis.',
                  )}
                </p>
                <div className="mt-4 flex flex-wrap gap-2 text-xs font-mono text-muted-foreground">
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">VPS Ubuntu/Debian</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">Live Chat Widget</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">Resend Email</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">Webhook Order</span>
                </div>
              </div>

              {/* CARD 3: APPS, SKILLS & CUSTOM MCP */}
              <div className="rounded-xl border border-border/60 bg-card p-6 shadow-2xs transition-colors hover:border-border/80">
                <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-muted/60 text-muted-foreground">
                  <Layers className="h-5 w-5" strokeWidth={1.5} />
                </div>
                <div className="text-xs font-semibold text-primary mb-1 uppercase tracking-wider">
                  Extensibility & MCP
                </div>
                <h3 className="font-display text-lg font-bold text-foreground">
                  {t('Apps, Skills & Custom MCP Server')}
                </h3>
                <p className="mt-2.5 text-sm sm:text-base leading-relaxed text-muted-foreground">
                  {t(
                    'Dukungan penuh standar terbuka Model Context Protocol (MCP) untuk menyambungkan database Postgres/MySQL atau API custom internal, ditambah skill AI Vision struk, auto-responder, dan CRM grounding.',
                  )}
                </p>
                <div className="mt-4 flex flex-wrap gap-2 text-xs font-mono text-muted-foreground">
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">Custom MCP stdio/SSE</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">AI Vision Receipt</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">CRM Grounding</span>
                  <span className="rounded-md border border-border/60 bg-muted/40 px-2 py-1">Quality Gate DoD</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* 2A. BUSINESS OPERATIONS PROBLEM (Level 1: Real Business Headaches) */}
        <section
          id="problem"
          className="scroll-mt-6 border-t border-border/60 bg-muted/20 px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="problem-title"
        >
          <div className="mx-auto max-w-6xl">
            <div className="max-w-3xl">
              <div className="mb-3 inline-flex items-center gap-2 text-xs font-semibold text-muted-foreground">
                <AlertCircle className="h-4 w-4 text-amber-500/80" strokeWidth={1.5} />
                <span>{t('Tantangan Pebisnis & Marketer')}</span>
              </div>
              <h2
                id="problem-title"
                className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl md:text-4xl"
              >
                {t('Punya segudang ide bisnis, tapi sering mentok di urusan teknis?')}
              </h2>
              <p className="mt-3 text-base sm:text-lg leading-relaxed text-muted-foreground">
                {t(
                  'Banyak pebisnis dan marketer membuang puluhan jam berharga mengurusi operasional manual, membalas chat berulang, integrasi teknis, hingga error alih-alih fokus mencari cuan.',
                )}
              </p>
            </div>

            <div className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
              {[
                {
                  Icon: Clock,
                  title: 'Sewa tim teknis & operasional mahal',
                  desc: 'Gaji jutaan rupiah per bulan untuk programmer, admin, atau CS tambahan. Rawan miskomunikasi dan antrean berminggu-minggu bahkan untuk otomasi atau revisi kecil.',
                },
                {
                  Icon: Coins,
                  title: 'Langganan SaaS luar negeri bikin boncos',
                  desc: 'Tagihan bulanan membengkak dengan markup tinggi, kuota token dibatasi, dan data pelanggan serta rahasia bisnis Anda tersimpan di server pihak ketiga.',
                },
                {
                  Icon: AlertCircle,
                  title: 'Ngulik sendiri malah burnout',
                  desc: 'Mencoba utak-atik terminal, git, webhook, dan kodingan sendiri. Seharian habis membaca tutorial dan error log, waktu untuk jualan malah hilang.',
                },
              ].map(({ Icon, title, desc }) => (
                <div
                  key={title}
                  className="rounded-xl border border-border/60 bg-card p-6 shadow-2xs transition-colors hover:border-border/80"
                >
                  <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-muted/60 text-muted-foreground">
                    <Icon className="h-5 w-5" strokeWidth={1.5} />
                  </div>
                  <h3 className="font-display text-lg font-bold text-foreground">
                    {t(title)}
                  </h3>
                  <p className="mt-2.5 text-sm sm:text-base leading-relaxed text-muted-foreground">
                    {t(desc)}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* 2B. AI EXPECTATION VS REALITY (Level 2: The Bottlenecks of Existing AI) */}
        <section
          id="ai-limits"
          className="scroll-mt-6 border-t border-border/40 bg-card px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="ai-limits-title"
        >
          <div className="mx-auto max-w-6xl">
            <div className="max-w-3xl">
              <div className="mb-3 inline-flex items-center gap-2 text-xs font-semibold text-muted-foreground">
                <Bot className="h-4 w-4 text-primary" strokeWidth={1.5} />
                <span>{t('Realita AI Saat Ini')}</span>
              </div>
              <h2
                id="ai-limits-title"
                className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl md:text-4xl"
              >
                {t('Berharap AI membereskan segalanya, tapi malah nambah pusing?')}
              </h2>
              <p className="mt-3 text-base sm:text-lg leading-relaxed text-muted-foreground">
                {t(
                  'Banyak marketer mencoba berbagai AI terbaru untuk mengatasi masalah di atas, namun masing-masing punya batasan yang bikin pekerjaan tetap macet.',
                )}
              </p>
            </div>

            <div className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
              {[
                {
                  Icon: Bot,
                  title: 'Chatbot web cuma memberi saran teks, eksekusi tetap manual',
                  desc: 'Chatbot percakapan dirancang untuk tanya-jawab dan drafting ide. Mereka tidak terhubung langsung ke server atau database bisnis Anda, sehingga Anda tetap harus manual copy-paste kode, menyatukan webhook, dan tebak-tebak errornya sendiri.',
                },
                {
                  Icon: Terminal,
                  title: 'Tool koding CLI dibuat untuk programmer di terminal lokal',
                  desc: 'Sangat andal untuk developer, tetapi wajib mengoperasikan layar terminal hitam (bash/git) yang membingungkan non-teknis. Berjalan di laptop lokal sehingga langsung mati begitu laptop Anda tutup atau sleep, dan tidak bisa diakses santai dari HP.',
                },
                {
                  Icon: Activity,
                  title: 'AI otonom personal belum siap untuk operasional bisnis',
                  desc: 'Asisten otonom personal hebat untuk automasi di laptop developer atau eksperimen cloud tertutup. Namun tanpa Approval Card bisnis, Quality Gate visual, antrean multi-task di VPS, serta CRM percakapan terpadu, eksekusinya belum siap untuk operasional bisnis harian.',
                },
              ].map(({ Icon, title, desc }) => (
                <div
                  key={title}
                  className="rounded-xl border border-border/60 bg-muted/20 p-6 shadow-2xs transition-colors hover:border-border/80"
                >
                  <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-card text-muted-foreground border border-border/50">
                    <Icon className="h-5 w-5" strokeWidth={1.5} />
                  </div>
                  <h3 className="font-display text-lg font-bold text-foreground">
                    {t(title)}
                  </h3>
                  <p className="mt-2.5 text-sm sm:text-base leading-relaxed text-muted-foreground">
                    {t(desc)}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* 3. SOLUTION & PROCESS SECTION (S-P in PAS-P / StoryBrand 3-Step) */}
        <section
          id="how-it-works"
          className="scroll-mt-6 border-t border-border/60 px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="how-title"
        >
          <div className="mx-auto max-w-6xl">
            <div className="max-w-3xl">
              <div className="mb-3 inline-flex items-center gap-2 text-xs font-semibold text-muted-foreground">
                <CheckCircle2 className="h-4 w-4 text-emerald-500" strokeWidth={1.5} />
                <span>{t('Cara kerja Cuan')}</span>
              </div>
              <h2
                id="how-title"
                className="font-display text-2xl font-bold tracking-tight sm:text-3xl md:text-4xl"
              >
                {t('Kenalkan Cuan: AI Project Manager Siap Kerja Tanpa Pengawasan')}
              </h2>
              <p className="mt-3 text-base sm:text-lg leading-relaxed text-muted-foreground">
                {t(
                  'Anda bertindak sebagai owner yang memberi instruksi lewat chat santai. Cuan memimpin tim agen spesialis—mulai dari otomasi operasional bisnis, layanan customer support, pembukuan mutasi, hingga koding dan deploy di server Anda sendiri sampai tuntas terverifikasi.',
                )}
              </p>
            </div>

            <ol className="mt-12 grid gap-8 lg:grid-cols-3">
              {/* STEP 01: CHAT */}
              <li className="flex flex-col justify-between rounded-xl border border-border/50 bg-card p-6 shadow-2xs">
                <div>
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs font-bold text-muted-foreground tabular-nums">
                      01
                    </span>
                    <span className="rounded-md bg-muted/60 px-2.5 py-0.5 text-xs font-semibold text-muted-foreground">
                      Chat
                    </span>
                  </div>
                  <h3 className="font-display mt-4 text-lg font-bold leading-snug">
                    {t('Describe what you need in plain words')}
                  </h3>
                  <p className="mt-2.5 text-sm sm:text-base leading-relaxed text-muted-foreground">
                    {t(
                      'Start in Chat from your phone or desktop. Share an operational goal, customer support workflow, financial receipt, or web feature. Cuan analyzes your context automatically.',
                    )}
                  </p>
                </div>

                {/* Real Cuan Chat Preview */}
                <div className="mt-6 rounded-xl border border-border/60 bg-muted/20 p-4 text-left text-xs space-y-3">
                  <div className="flex items-center justify-between border-b border-border/40 pb-2">
                    <div className="flex items-center gap-1.5 font-medium text-foreground">
                      <span className="h-2 w-2 rounded-full bg-emerald-500" />
                      <span className="font-semibold text-xs">@cuan</span>
                      <span className="text-[10px] text-muted-foreground font-mono">Project Manager</span>
                    </div>
                    <span className="text-[10px] text-muted-foreground font-mono">{t('Baru saja')}</span>
                  </div>
                  <div className="space-y-2.5">
                    <div className="rounded-lg bg-card p-3 border border-border/50 shadow-2xs">
                      <div className="text-[10px] font-semibold text-muted-foreground mb-1 flex items-center gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-blue-500" />
                        {t('Anda (Owner)')}
                      </div>
                      <p className="text-foreground leading-relaxed">
                        &ldquo;{t('Tolong hubungkan webhook form order ke live chat pelanggan dan rekap mutasi otomatis.')}&rdquo;
                      </p>
                    </div>
                    <div className="rounded-lg bg-primary/5 dark:bg-primary/10 p-3 border border-primary/20">
                      <div className="text-[10px] font-semibold text-primary mb-1 flex items-center gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                        @cuan
                      </div>
                      <p className="text-foreground/90 leading-relaxed text-[11px]">
                        {t('Siap! Konteks alur kerja dan webhook terbaca. 3 tugas masuk antrean otomatis. Saya siapkan Approval Card rencana kerja dan kriteria pengujian sebelum eksekusi.')}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center justify-between pt-1 border-t border-border/30 text-[10px] text-muted-foreground font-mono">
                    <span className="inline-flex items-center gap-1.5 text-muted-foreground">
                      <span className="h-1.5 w-1.5 rounded-full bg-amber-500 animate-pulse" />
                      <span>{t('Antrean: 1 aktif · 2 menunggu')}</span>
                    </span>
                    <span className="rounded-md bg-muted/80 px-2 py-0.5 text-muted-foreground font-sans text-[11px]">
                      {t('Draft Approval Card')} &rarr;
                    </span>
                  </div>
                </div>
              </li>

              {/* STEP 02: APPROVAL CARD */}
              <li className="flex flex-col justify-between rounded-xl border border-border/50 bg-card p-6 shadow-2xs">
                <div>
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs font-bold text-muted-foreground tabular-nums">
                      02
                    </span>
                    <span className="rounded-md bg-muted/60 px-2.5 py-0.5 text-xs font-semibold text-muted-foreground">
                      Setujui
                    </span>
                  </div>
                  <h3 className="font-display mt-4 text-lg font-bold leading-snug">
                    {t('Review and approve a clear plan')}
                  </h3>
                  <p className="mt-2.5 text-sm sm:text-base leading-relaxed text-muted-foreground">
                    {t(
                      'Before touching anything, Cuan presents a structured Approval Card outlining the scope, specialist agents (@sm, @dev, @support), and verification criteria. One tap to approve.',
                    )}
                  </p>
                </div>

                {/* Real ApprovalCard Component from Cuan */}
                <div className="mt-6 text-left">
                  <ApprovalCard
                    plan={{
                      title: t('Integrasi Webhook Pesanan & CRM Kontak'),
                      project_name: 'cuan-workspace',
                      description: `- ${t('Pasang webhook pesanan otomatis ke sistem secara aman')}\n- ${t('Sinkronkan nomor kontak & catatan riwayat pelanggan')}\n- ${t('Pengujian otomatis & verifikasi visual 100% tuntas')}`,
                    }}
                    disabled={false}
                    onApprove={() => {}}
                    className="mt-0 pointer-events-none select-none shadow-none"
                  />
                </div>
              </li>

              {/* STEP 03: BERES */}
              <li className="flex flex-col justify-between rounded-xl border border-border/50 bg-card p-6 shadow-2xs">
                <div>
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs font-bold text-muted-foreground tabular-nums">
                      03
                    </span>
                    <span className="rounded-md bg-muted/60 px-2.5 py-0.5 text-xs font-semibold text-muted-foreground">
                      Beres
                    </span>
                  </div>
                  <h3 className="font-display mt-4 text-lg font-bold leading-snug">
                    {t('Verified outcomes delivered to your server')}
                  </h3>
                  <p className="mt-2.5 text-sm sm:text-base leading-relaxed text-muted-foreground">
                    {t(
                      'Autonomous agents execute operations, handle support, or write code, verifying Definition of Done criteria, and report back via an authentic Task Card with 100% proof.',
                    )}
                  </p>
                </div>

                {/* Authentic Verified Task Result Card */}
                <div className="mt-6 rounded-xl border border-border/60 bg-muted/20 p-4 text-left text-xs space-y-3">
                  <div className="flex items-center justify-between border-b border-border/40 pb-2">
                    <div className="flex items-center gap-1.5">
                      <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />
                      <span className="font-semibold text-xs text-foreground">
                        {t('Status: Selesai')}
                      </span>
                    </div>
                    <span className="rounded-md bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 px-2 py-0.5 text-[11px] font-medium">
                      ✓ {t('100% Teruji')}
                    </span>
                  </div>
                  <div>
                    <div className="text-[10px] font-mono text-muted-foreground">
                      {t('Hasil Eksekusi')}
                    </div>
                    <div className="font-semibold text-foreground text-sm mt-0.5">
                      {t('Webhook Live Chat & Mutasi Berhasil Aktif')}
                    </div>
                  </div>
                  <div className="rounded-lg bg-card p-3 border border-border/50 shadow-2xs space-y-2 text-[11px] text-foreground/90">
                    <div className="text-[10px] font-semibold text-emerald-600 dark:text-emerald-400">
                      {t('Bukti Kualitas (Quality Gate):')}
                    </div>
                    <div className="flex items-center gap-2">
                      <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" strokeWidth={1.5} />
                      <span>{t('4 pengujian otomatis lulus tanpa error')}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" strokeWidth={1.5} />
                      <span>{t('Bukti visual tangkapan layar terverifikasi')}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" strokeWidth={1.5} />
                      <span>{t('Deploy aman ke VPS tanpa downtime')}</span>
                    </div>
                  </div>
                  <div className="flex items-center justify-between pt-1 border-t border-border/30 text-[10px] text-muted-foreground font-mono">
                    <span>#task_94a2f</span>
                    <span className="text-primary font-sans font-medium text-[11px]">
                      {t('Laporan Terverifikasi')} &rarr;
                    </span>
                  </div>
                </div>
              </li>
            </ol>
          </div>
        </section>

        {/* 3.5. COMPARISON SECTION (Why Cuan is Different) */}
        <section
          id="comparison"
          className="scroll-mt-6 border-t border-border/60 bg-muted/10 px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="comparison-title"
        >
          <div className="mx-auto max-w-6xl">
            <div className="max-w-3xl">
              <div className="mb-3 inline-flex items-center gap-2 text-xs font-semibold text-muted-foreground">
                <CheckCircle2 className="h-4 w-4 text-emerald-500" strokeWidth={1.5} />
                <span>{t('Mengapa Cuan Berbeda?')}</span>
              </div>
              <h2
                id="comparison-title"
                className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl md:text-4xl"
              >
                {t('Bukan Sekadar Chatbot atau Tool Programmer')}
              </h2>
              <p className="mt-3 text-base sm:text-lg leading-relaxed text-muted-foreground">
                {t(
                  'Perbandingan faktual antara chatbot percakapan biasa, tool koding developer, framework agen otonom, dan Cuan.',
                )}
              </p>
            </div>

            {/* DESKTOP TABLE VIEW (hidden sm:block) */}
            <div className="mt-10 hidden sm:block overflow-x-auto pb-4">
              <table className="w-full min-w-[760px] border-collapse text-left text-sm">
                <thead>
                  <tr className="border-b border-border/60">
                    <th className="py-3.5 pr-4 font-bold text-foreground w-[24%]">
                      {t('Kebutuhan Bisnis Anda')}
                    </th>
                    <th className="px-3 py-3.5 font-semibold text-center w-[19%]">
                      <div className="font-bold text-foreground">{t('Chatbot Percakapan')}</div>
                      <div className="text-xs text-muted-foreground font-normal mt-0.5">
                        {t('Chatbot percakapan biasa')}
                      </div>
                    </th>
                    <th className="px-3 py-3.5 font-semibold text-center w-[19%]">
                      <div className="font-bold text-foreground">{t('Tool Koding Developer')}</div>
                      <div className="text-xs text-muted-foreground font-normal mt-0.5">
                        {t('Software koding CLI')}
                      </div>
                    </th>
                    <th className="px-3 py-3.5 font-semibold text-center w-[19%]">
                      <div className="font-bold text-foreground">{t('Asisten Personal')}</div>
                      <div className="text-xs text-muted-foreground font-normal mt-0.5">
                        {t('Asisten personal lokal & cloud')}
                      </div>
                    </th>
                    <th className="px-3 py-3.5 font-bold text-foreground text-center w-[19%] bg-primary/5 dark:bg-primary/10 rounded-t-lg border-x border-t border-border/50">
                      <div className="inline-flex items-center gap-1.5 font-bold">
                        <CuanMark decorative className="h-4 w-4" />
                        <span className="font-display tracking-tight text-sm font-bold text-foreground">
                          cuan
                        </span>
                      </div>
                      <div className="text-xs text-muted-foreground font-normal mt-0.5">
                        {t('AI Project Manager Bisnis')}
                      </div>
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/40">
                  {[
                    {
                      label: 'Lingkungan eksekusi',
                      desc: 'Di mana tugas benar-benar dijalankan',
                      chatbot: { icon: 'x', text: 'Cloud vendor (terisolasi dari VPS)' },
                      coder: { icon: 'warn', text: 'Terminal laptop lokal / software koding' },
                      hermes: { icon: 'warn', text: 'Mesin lokal / cloud vendor' },
                      cuan: { icon: 'check', text: 'Server VPS 24/7 mandiri' },
                    },
                    {
                      label: 'Antrean banyak tugas (Multi-task)',
                      desc: 'Memproses banyak instruksi tanpa ditunggui',
                      chatbot: { icon: 'x', text: 'Prompt manual 1 per 1' },
                      coder: { icon: 'warn', text: 'Wajib ditungguin di terminal' },
                      hermes: { icon: 'warn', text: 'Single-task / laptop aktif' },
                      cuan: { icon: 'check', text: 'Antrean otonom 24/7 di server' },
                    },
                    {
                      label: 'Antarmuka & kemudahan pakai',
                      desc: 'Kenyamanan akses untuk tim non-teknis',
                      chatbot: { icon: 'warn', text: 'Chat teks percakapan' },
                      coder: { icon: 'x', text: 'Wajib terminal hitam & git' },
                      hermes: { icon: 'warn', text: 'Desktop app personal & CLI' },
                      cuan: { icon: 'check', text: 'Web & Mobile tenang (ramah non-teknis)' },
                    },
                    {
                      label: 'Rem pengaman persetujuan',
                      desc: 'Kontrol manusia sebelum aksi menyentuh sistem',
                      chatbot: { icon: 'x', text: 'Tidak ada (hanya saran teks)' },
                      coder: { icon: 'warn', text: 'Prompt teks terminal [y/n]' },
                      hermes: { icon: 'warn', text: 'Prompt izin ad-hoc per aksi' },
                      cuan: { icon: 'check', text: 'Approval Card 1-Tap terstruktur' },
                    },
                    {
                      label: 'Quality Gate & verifikasi selesai',
                      desc: 'Pembuktian nyata sebelum tugas dianggap tuntas',
                      chatbot: { icon: 'x', text: 'Tidak ada pengujian sistem' },
                      coder: { icon: 'warn', text: 'Testing kode programmer' },
                      hermes: { icon: 'warn', text: 'Evaluasi internal bot' },
                      cuan: { icon: 'check', text: 'Bukti visual + tes otomatis tuntas' },
                    },
                    {
                      label: 'Alat operasional bisnis (Live Chat/CRM)',
                      desc: 'Integrasi live chat pelanggan dan database kontak',
                      chatbot: { icon: 'x', text: 'Tidak ada' },
                      coder: { icon: 'x', text: 'Tidak ada' },
                      hermes: { icon: 'warn', text: 'Personal agent (tanpa CRM/Inbox)' },
                      cuan: { icon: 'check', text: 'Inbox Live Chat & CRM Kontak' },
                    },
                    {
                      label: 'Model biaya token AI',
                      desc: 'Struktur biaya pemakaian kecerdasan buatan',
                      chatbot: { icon: 'x', text: 'Langganan per akun ($20/bln)' },
                      coder: { icon: 'warn', text: 'Langganan / API developer' },
                      hermes: { icon: 'warn', text: 'Bayar API per pemakaian' },
                      cuan: { icon: 'check', text: 'Bebas dobel biaya (pakai akun resmi sendiri)' },
                    },
                  ].map((row, idx, arr) => (
                    <tr key={row.label} className="hover:bg-muted/30 transition-colors">
                      <td className="py-3.5 pr-4 font-semibold text-foreground">
                        {t(row.label)}
                      </td>
                      <td className="px-3 py-3.5 text-center text-muted-foreground">
                        <div className="flex flex-col items-center gap-1">
                          {row.chatbot.icon === 'check' && <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                          {row.chatbot.icon === 'warn' && <Minus className="h-4 w-4 text-amber-500/70" strokeWidth={1.5} />}
                          {row.chatbot.icon === 'x' && <X className="h-4 w-4 text-muted-foreground/40" strokeWidth={1.5} />}
                          <span className="text-xs sm:text-sm leading-tight">{t(row.chatbot.text)}</span>
                        </div>
                      </td>
                      <td className="px-3 py-3.5 text-center text-muted-foreground">
                        <div className="flex flex-col items-center gap-1">
                          {row.coder.icon === 'check' && <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                          {row.coder.icon === 'warn' && <Minus className="h-4 w-4 text-amber-500/70" strokeWidth={1.5} />}
                          {row.coder.icon === 'x' && <X className="h-4 w-4 text-muted-foreground/40" strokeWidth={1.5} />}
                          <span className="text-xs sm:text-sm leading-tight">{t(row.coder.text)}</span>
                        </div>
                      </td>
                      <td className="px-3 py-3.5 text-center text-muted-foreground">
                        <div className="flex flex-col items-center gap-1">
                          {row.hermes.icon === 'check' && <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                          {row.hermes.icon === 'warn' && <Minus className="h-4 w-4 text-amber-500/70" strokeWidth={1.5} />}
                          {row.hermes.icon === 'x' && <X className="h-4 w-4 text-muted-foreground/40" strokeWidth={1.5} />}
                          <span className="text-xs sm:text-sm leading-tight">{t(row.hermes.text)}</span>
                        </div>
                      </td>
                      <td className={`px-3 py-3.5 text-center bg-primary/5 dark:bg-primary/10 border-x border-border/50 ${idx === arr.length - 1 ? 'border-b rounded-b-lg' : ''}`}>
                        <div className="flex flex-col items-center gap-1 font-semibold text-foreground">
                          {row.cuan.icon === 'check' && <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                          {row.cuan.icon === 'warn' && <Minus className="h-4 w-4 text-amber-500/70" strokeWidth={1.5} />}
                          {row.cuan.icon === 'x' && <X className="h-4 w-4 text-muted-foreground/40" strokeWidth={1.5} />}
                          <span className="text-xs sm:text-sm leading-tight text-foreground font-semibold">{t(row.cuan.text)}</span>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* MOBILE CARD VIEW (block sm:hidden) - Zero Horizontal Scroll, High Ergonomics */}
            <div className="mt-8 space-y-4 sm:hidden">
              {[
                {
                  label: 'Lingkungan eksekusi',
                  desc: 'Di mana tugas benar-benar dijalankan',
                  chatbot: { icon: 'x', text: 'Cloud vendor (terisolasi dari VPS)' },
                  coder: { icon: 'warn', text: 'Terminal laptop lokal / software koding' },
                  hermes: { icon: 'warn', text: 'Mesin lokal / cloud vendor (Meta)' },
                  cuan: { icon: 'check', text: 'Server VPS 24/7 mandiri' },
                },
                {
                  label: 'Antrean banyak tugas (Multi-task)',
                  desc: 'Memproses banyak instruksi tanpa ditunggui',
                  chatbot: { icon: 'x', text: 'Prompt manual 1 per 1' },
                  coder: { icon: 'warn', text: 'Wajib ditungguin di terminal' },
                  hermes: { icon: 'warn', text: 'Single-task / laptop aktif' },
                  cuan: { icon: 'check', text: 'Antrean otonom 24/7 di server' },
                },
                {
                  label: 'Antarmuka & kemudahan pakai',
                  desc: 'Kenyamanan akses untuk tim non-teknis',
                  chatbot: { icon: 'warn', text: 'Chat teks percakapan' },
                  coder: { icon: 'x', text: 'Wajib terminal hitam & git' },
                  hermes: { icon: 'warn', text: 'Desktop app personal & CLI' },
                  cuan: { icon: 'check', text: 'Web & Mobile tenang (ramah non-teknis)' },
                },
                {
                  label: 'Rem pengaman persetujuan',
                  desc: 'Kontrol manusia sebelum aksi menyentuh sistem',
                  chatbot: { icon: 'x', text: 'Tidak ada (hanya saran teks)' },
                  coder: { icon: 'warn', text: 'Prompt teks terminal [y/n]' },
                  hermes: { icon: 'warn', text: 'Prompt izin ad-hoc per aksi' },
                  cuan: { icon: 'check', text: 'Approval Card 1-Tap terstruktur' },
                },
                {
                  label: 'Quality Gate & verifikasi selesai',
                  desc: 'Pembuktian nyata sebelum tugas dianggap tuntas',
                  chatbot: { icon: 'x', text: 'Tidak ada pengujian sistem' },
                  coder: { icon: 'warn', text: 'Testing kode programmer' },
                  hermes: { icon: 'warn', text: 'Evaluasi internal bot' },
                  cuan: { icon: 'check', text: 'Bukti visual + tes otomatis tuntas' },
                },
                {
                  label: 'Alat operasional bisnis (Live Chat/CRM)',
                  desc: 'Integrasi live chat pelanggan dan database kontak',
                  chatbot: { icon: 'x', text: 'Tidak ada' },
                  coder: { icon: 'x', text: 'Tidak ada' },
                  hermes: { icon: 'warn', text: 'Personal agent (tanpa CRM/Inbox)' },
                  cuan: { icon: 'check', text: 'Inbox Live Chat & CRM Kontak' },
                },
                {
                  label: 'Model biaya token AI',
                  desc: 'Struktur biaya pemakaian kecerdasan buatan',
                  chatbot: { icon: 'x', text: 'Langganan per akun ($20/bln)' },
                  coder: { icon: 'warn', text: 'Langganan / API developer' },
                  hermes: { icon: 'warn', text: 'Bayar API per pemakaian' },
                  cuan: { icon: 'check', text: 'Bebas dobel biaya (pakai akun resmi sendiri)' },
                },
              ].map((row) => (
                <div
                  key={row.label}
                  className="rounded-xl border border-border/60 bg-card p-4 shadow-2xs"
                >
                  <div className="mb-3">
                    <h3 className="font-display text-base font-bold text-foreground">
                      {t(row.label)}
                    </h3>
                    <p className="mt-0.5 text-xs sm:text-sm text-muted-foreground">
                      {t(row.desc)}
                    </p>
                  </div>

                  {/* Cuan Highlight Card */}
                  <div className="mb-3 flex items-start gap-2.5 rounded-lg border border-primary/25 bg-primary/5 dark:bg-primary/10 p-3">
                    <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400 mt-0.5 shrink-0" strokeWidth={1.5} />
                    <div>
                      <div className="inline-flex items-center gap-1.5 text-xs sm:text-sm font-bold text-foreground">
                        <CuanMark decorative className="h-3.5 w-3.5" />
                        <span className="font-display tracking-tight font-bold text-foreground">
                          cuan
                        </span>
                        <span className="text-xs font-normal text-muted-foreground">({t('AI Project Manager')})</span>
                      </div>
                      <div className="mt-0.5 text-xs sm:text-sm font-semibold text-foreground">
                        {t(row.cuan.text)}
                      </div>
                    </div>
                  </div>

                  {/* Alternatives Comparison List */}
                  <div className="space-y-2 border-t border-border/40 pt-2.5 text-xs sm:text-sm">
                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-xs">{t('Chatbot percakapan biasa')}</span>
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-foreground">
                        {row.chatbot.icon === 'check' && <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                        {row.chatbot.icon === 'warn' && <Minus className="h-3.5 w-3.5 text-amber-500/80" strokeWidth={1.5} />}
                        {row.chatbot.icon === 'x' && <X className="h-3.5 w-3.5 text-muted-foreground/40" strokeWidth={1.5} />}
                        <span>{t(row.chatbot.text)}</span>
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-xs">{t('Software koding developer')}</span>
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-foreground">
                        {row.coder.icon === 'check' && <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                        {row.coder.icon === 'warn' && <Minus className="h-3.5 w-3.5 text-amber-500/80" strokeWidth={1.5} />}
                        {row.coder.icon === 'x' && <X className="h-3.5 w-3.5 text-muted-foreground/40" strokeWidth={1.5} />}
                        <span>{t(row.coder.text)}</span>
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-xs">{t('Asisten personal lokal & cloud')}</span>
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-foreground">
                        {row.hermes.icon === 'check' && <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                        {row.hermes.icon === 'warn' && <Minus className="h-3.5 w-3.5 text-amber-500/80" strokeWidth={1.5} />}
                        {row.hermes.icon === 'x' && <X className="h-3.5 w-3.5 text-muted-foreground/40" strokeWidth={1.5} />}
                        <span>{t(row.hermes.text)}</span>
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* 4. PROOF & SUPERPOWERS SECTION (P in PAS-P) */}
        <section
          id="features"
          className="scroll-mt-6 border-t border-border/60 bg-muted/20 px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="features-title"
        >
          <div className="mx-auto max-w-6xl">
            <div className="max-w-2xl">
              <h2
                id="features-title"
                className="font-display text-2xl font-bold tracking-tight sm:text-3xl md:text-4xl text-foreground"
              >
                {t('Tetap memegang kendali penuh, tanpa pusing urusan teknis')}
              </h2>
              <p className="mt-3 text-base sm:text-lg leading-relaxed text-muted-foreground">
                {t(
                  'Tujuan dan keputusan bisnis Anda yang memimpin arah kerja. Cuan mengurus detail teknis dan membuktikan hasil kerjanya secara transparan.',
                )}
              </p>
            </div>

            <div className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
              {[
                {
                  Icon: ShieldCheck,
                  title: 'Approval stays with you',
                  text: 'Review the plan before work starts. Get a clear account of changes, checks, and evidence before any code is published.',
                },
                {
                  Icon: Coins,
                  title: 'Bebas biaya dobel token AI',
                  text: 'Jalankan langganan resmi Anda (Claude Pro, ChatGPT Plus, Gemini) atau bawa API key sendiri (DeepSeek, OpenAI) tanpa markup token bulanan.',
                },
                {
                  Icon: CheckCircle2,
                  title: 'Autonomous Definition of Done loop',
                  text: 'Built-in quality gate runs targeted tests and verifies real component outputs automatically before marking any task complete.',
                },
                {
                  Icon: MessageSquare,
                  title: 'Inbox live chat pelanggan terpadu',
                  text: 'Kelola percakapan pelanggan dari widget live chat web dalam satu alur tenang, lengkap dengan catatan bisik privat untuk operator.',
                },
                {
                  Icon: Users,
                  title: 'Kolaborasi Tim & Undangan Staf via Email',
                  text: 'Undang staf dan admin dengan mudah via tautan email. Hak akses bertingkat Owner vs Member menjaga kredensial AI dan data sensitif tetap aman.',
                },
                {
                  Icon: Database,
                  title: 'CRM Kontak & Memori Bisnis Terpadu',
                  text: 'Database pelanggan terintegrasi dengan tag prospek (leads) dan riwayat percakapan. Cuan mengingat profil bisnis Anda untuk setiap respon cerdas agen.',
                },
                {
                  Icon: Mail,
                  title: 'Resend email alerts & idle chat transcripts.',
                  text: 'Never lose a customer lead. Automatically receive responsive email digests via Resend when live chat inquiries are unread or idle (>5-10m).',
                },
                {
                  Icon: Volume2,
                  title: 'Background audio chime & tab notifications.',
                  text: 'Harmonic Web Audio chime synthesizes subtle sound cues and dynamic browser tab badges when tasks finish in the background, with zero-friction silent autoplay fallback.',
                },
                {
                  Icon: Activity,
                  title: 'Worker observability & queue health.',
                  text: 'Monitor your cuan-runner daemon health, queue depth, and heartbeat directly in Settings with crystal-clear visual status indicators and zero terminal complexity.',
                },
              ].map(({ Icon, title, text }) => (
                <div key={title} className="rounded-xl border border-border/60 bg-card p-6 shadow-2xs transition-colors hover:border-border/80">
                  <Icon className="mb-3.5 h-5 w-5 text-muted-foreground" strokeWidth={1.5} />
                  <h3 className="text-base font-bold text-foreground">{t(title)}</h3>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {t(text)}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* 5. FINAL PITCH / CTA SECTION */}
        <section className="border-t border-border/60 px-6 py-16 sm:px-8 sm:py-20">
          <div className="mx-auto flex max-w-6xl flex-col items-start justify-between gap-6 sm:flex-row sm:items-center">
            <div className="max-w-xl">
              <h2 className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl md:text-4xl">
                {t('Siap bebas dari pusing urusan teknis?')}
              </h2>
              <p className="mt-3 text-base sm:text-lg leading-relaxed text-muted-foreground">
                {t('Fokus besarkan bisnis dan cari cuan. Biarkan agen Cuan yang merapikan operasional, melayani pelanggan, dan mengeksekusi kebutuhan teknis di server Anda.')}
              </p>
              <p className="mt-3 text-sm text-muted-foreground/80">
                {t('Open source · Lisensi Apache-2.0 · Kendali penuh di VPS sendiri')}
              </p>
            </div>
            <button
              type="button"
              onClick={scrollToWaitingList}
              className="action-primary min-h-12 px-7 text-base font-semibold shadow-xs cursor-pointer inline-flex items-center gap-2"
            >
              <span>{t('Gabung Waiting List')}</span>
              <ArrowRight className="h-4 w-4" strokeWidth={1.5} />
            </button>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 border-t border-border/60 px-6 py-6 text-sm text-muted-foreground sm:px-8">
        <p className="flex items-center gap-1.5">
          <span className="font-display tracking-tight font-bold text-foreground">
            cuan
          </span>
          <span>· {t('Your always-on project manager.')}</span>
        </p>
        <div className="flex flex-wrap items-center gap-5">
          <Link
            to="/sponsor"
            className="inline-flex min-h-11 items-center hover:text-foreground transition-colors font-medium text-primary"
          >
            {t('Program Sponsor Founding')}
          </Link>
          <a
            className="inline-flex min-h-11 items-center hover:text-foreground transition-colors"
            href={repository + '#readme'}
          >
            {t('Self-hosting guide')}
          </a>
          <button
            type="button"
            onClick={() => setShowGithubModal(true)}
            className="inline-flex min-h-11 items-center gap-1.5 hover:text-foreground transition-colors cursor-pointer"
          >
            <Github className="h-4 w-4" strokeWidth={1.5} />
            <span>GitHub</span>
          </button>
        </div>
      </footer>

      {/* GitHub Curation Notice Modal */}
      {showGithubModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/80 backdrop-blur-xs"
          role="dialog"
          aria-modal="true"
          aria-labelledby="github-modal-title"
          onClick={() => setShowGithubModal(false)}
        >
          <div
            className="w-full max-w-md rounded-2xl border border-border/80 bg-card p-6 shadow-xl space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-center gap-2.5">
                <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-muted/60 text-foreground">
                  <Github className="h-5 w-5" strokeWidth={1.5} />
                </div>
                <div>
                  <h3 id="github-modal-title" className="font-display text-base font-bold text-foreground">
                    {t('Repositori Open Source Cuan v1.0')}
                  </h3>
                  <div className="inline-flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400 font-medium">
                    <span className="h-1.5 w-1.5 rounded-full bg-amber-500 animate-pulse" />
                    <span>{t('Tahap Kurasi Final Rilis Publik (Apache-2.0)')}</span>
                  </div>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setShowGithubModal(false)}
                className="rounded-lg p-1 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors cursor-pointer"
                aria-label={t('Tutup')}
              >
                <X className="h-5 w-5" strokeWidth={1.5} />
              </button>
            </div>

            <p className="text-sm text-muted-foreground leading-relaxed">
              {t(
                'Repositori publik Cuan sedang disiapkan dan dirapikan untuk rilis open source resmi. Masukkan email Anda di Waiting List untuk mendapatkan notifikasi rilis pertama dan akses repositori prioritas.',
              )}
            </p>

            <div className="rounded-xl border border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground flex items-center justify-between">
              <span>{t('Lisensi Terbuka:')}</span>
              <span className="font-mono font-semibold text-foreground">Apache License 2.0</span>
            </div>

            <div className="pt-2 flex flex-col sm:flex-row gap-2">
              <button
                type="button"
                onClick={scrollToWaitingList}
                className="action-primary flex-1 min-h-11 px-4 text-sm font-semibold justify-center cursor-pointer"
              >
                {t('Daftar Waiting List Sekarang')}
              </button>
              <button
                type="button"
                onClick={() => setShowGithubModal(false)}
                className="action-secondary min-h-11 px-4 text-sm font-semibold justify-center cursor-pointer"
              >
                {t('Tutup')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
