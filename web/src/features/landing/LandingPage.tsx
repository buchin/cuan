import { Link } from 'react-router-dom'
import {
  ArrowRight,
  ChevronRight,
  GitBranch,
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
} from 'lucide-react'
import { CuanMark } from '../../components/brand/CuanMark'
import { ApprovalCard } from '../inbox/ApprovalCard'
import { useLanguage } from '../../context/LanguageContext'
import { useTheme } from '../../context/ThemeContext'

const repository = 'https://github.com/buchin/cuan'

export function LandingPage() {
  const { t, language, setPreference } = useLanguage()
  const { resolvedTheme, toggleTheme } = useTheme()

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
          <span className="text-xl font-semibold">cuan</span>
        </Link>
        <nav
          aria-label={t('Website navigation')}
          className="flex items-center gap-2 sm:gap-4"
        >
          <a
            href="#problem"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors sm:inline-flex"
          >
            {t('Masalah')}
          </a>
          <a
            href="#how-it-works"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors sm:inline-flex"
          >
            {t('Cara kerja')}
          </a>
          <a
            href="#comparison"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors sm:inline-flex"
          >
            {t('Perbandingan')}
          </a>
          <a
            href="#features"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors sm:inline-flex"
          >
            {t('Fitur')}
          </a>
          <a
            href={repository}
            target="_blank"
            rel="noreferrer"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground transition-colors sm:inline-flex"
          >
            GitHub
          </a>

          {/* Language Switcher */}
          <button
            type="button"
            onClick={() => setPreference(language === 'id' ? 'en' : 'id')}
            className="inline-flex min-h-9 items-center justify-center rounded-md border border-border/50 bg-muted/30 px-2.5 text-xs font-medium text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors"
            title={language === 'id' ? 'Ganti ke English' : 'Switch to Bahasa Indonesia'}
          >
            {language === 'id' ? 'ID' : 'EN'}
          </button>

          {/* Theme Switcher */}
          <button
            type="button"
            onClick={toggleTheme}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border/50 bg-muted/30 text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors"
            aria-label="Ganti tema"
            title={resolvedTheme === 'dark' ? 'Tema terang' : 'Tema gelap'}
          >
            {resolvedTheme === 'dark' ? (
              <Sun className="h-4 w-4" strokeWidth={1.5} />
            ) : (
              <Moon className="h-4 w-4" strokeWidth={1.5} />
            )}
          </button>

          <Link to="/login" className="action-secondary min-h-10 px-4 text-sm font-semibold">
            {t('Masuk')}
          </Link>
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
            <div className="mt-6 flex flex-wrap items-center justify-center gap-3.5">
              <Link to="/login" className="action-primary min-h-12 px-7 text-base font-semibold shadow-xs">
                {t('Mulai Sekarang')}
                <ArrowRight className="h-4 w-4" strokeWidth={1.5} />
              </Link>
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
              {t('Bebas biaya token dobel · BYOK & CLI resmi (Claude, Codex, DeepSeek, Gemini)')}
            </span>
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
                  title: 'Hermes Agent & agen otonom (Muse, Nod) untuk automasi personal, bukan operasional bisnis',
                  desc: 'Hermes Agent dan agen personal seperti Nod sangat hebat untuk riset dan automasi di laptop pengembang. Namun tanpa Approval Card bisnis, Quality Gate visual, serta CRM percakapan terpadu, eksekusi mandirinya belum siap diserahkan ke tim non-teknis.',
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
                        {t('Siap! Konteks repositori dan webhook terbaca. Saya siapkan Approval Card rencana kerja dan kriteria pengujian sebelum eksekusi.')}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center justify-between pt-1 border-t border-border/30 text-[10px] text-muted-foreground font-mono">
                    <span>#msg_chat_01</span>
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
                      description: `- ${t('Pasang receiver webhook pesanan tanpa ekspos token')}\n- ${t('Sinkronkan nomor kontak & catatan riwayat pelanggan')}\n- ${t('Pengujian DoD: verifikasi visual & regresi 100%')}`,
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
                      <div className="font-bold text-foreground">{t('Chatbot Web')}</div>
                      <div className="text-xs text-muted-foreground font-normal mt-0.5">
                        ChatGPT, Claude
                      </div>
                    </th>
                    <th className="px-3 py-3.5 font-semibold text-center w-[19%]">
                      <div className="font-bold text-foreground">{t('Coding Agent & CLI')}</div>
                      <div className="text-xs text-muted-foreground font-normal mt-0.5">
                        Claude Code, Codex
                      </div>
                    </th>
                    <th className="px-3 py-3.5 font-semibold text-center w-[19%]">
                      <div className="font-bold text-foreground">{t('Hermes Agent')}</div>
                      <div className="text-xs text-muted-foreground font-normal mt-0.5">
                        Hermes Agent, Muse, Nod
                      </div>
                    </th>
                    <th className="px-3 py-3.5 font-bold text-foreground text-center w-[19%] bg-primary/5 dark:bg-primary/10 rounded-t-lg border-x border-t border-border/50">
                      <div className="inline-flex items-center gap-1.5 font-bold">
                        <CuanMark decorative className="h-4 w-4" />
                        cuan
                      </div>
                      <div className="text-xs text-muted-foreground font-normal mt-0.5">
                        {t('AI Project Manager')}
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
                      coder: { icon: 'warn', text: 'Terminal laptop lokal / IDE' },
                      hermes: { icon: 'warn', text: 'Aplikasi desktop / mesin lokal' },
                      cuan: { icon: 'check', text: 'Server VPS 24/7 mandiri' },
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
                      hermes: { icon: 'warn', text: 'Self-evaluation LLM' },
                      cuan: { icon: 'check', text: 'Bukti visual + DoD otomatis' },
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
                      hermes: { icon: 'warn', text: 'Bayar API per token (BYO-AI)' },
                      cuan: { icon: 'check', text: 'Bebas dobel biaya (CLI / BYOK)' },
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
                  coder: { icon: 'warn', text: 'Terminal laptop lokal / IDE' },
                  hermes: { icon: 'warn', text: 'Aplikasi desktop / mesin lokal' },
                  cuan: { icon: 'check', text: 'Server VPS 24/7 mandiri' },
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
                  hermes: { icon: 'warn', text: 'Self-evaluation LLM' },
                  cuan: { icon: 'check', text: 'Bukti visual + DoD otomatis' },
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
                  hermes: { icon: 'warn', text: 'Bayar API per token (BYO-AI)' },
                  cuan: { icon: 'check', text: 'Bebas dobel biaya (CLI / BYOK)' },
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
                        cuan ({t('AI Project Manager')})
                      </div>
                      <div className="mt-0.5 text-xs sm:text-sm font-semibold text-foreground">
                        {t(row.cuan.text)}
                      </div>
                    </div>
                  </div>

                  {/* Alternatives Comparison List */}
                  <div className="space-y-2 border-t border-border/40 pt-2.5 text-xs sm:text-sm">
                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-xs">Chatbot (ChatGPT/Claude)</span>
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-foreground">
                        {row.chatbot.icon === 'check' && <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                        {row.chatbot.icon === 'warn' && <Minus className="h-3.5 w-3.5 text-amber-500/80" strokeWidth={1.5} />}
                        {row.chatbot.icon === 'x' && <X className="h-3.5 w-3.5 text-muted-foreground/40" strokeWidth={1.5} />}
                        <span>{t(row.chatbot.text)}</span>
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-xs">Coding Agent (Claude Code/Codex)</span>
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-foreground">
                        {row.coder.icon === 'check' && <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" strokeWidth={1.5} />}
                        {row.coder.icon === 'warn' && <Minus className="h-3.5 w-3.5 text-amber-500/80" strokeWidth={1.5} />}
                        {row.coder.icon === 'x' && <X className="h-3.5 w-3.5 text-muted-foreground/40" strokeWidth={1.5} />}
                        <span>{t(row.coder.text)}</span>
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-xs">{t('Hermes Agent (Muse, Nod)')}</span>
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
                  text: 'Jalankan langganan resmi Anda (Claude Pro/Team via CLI claude, ChatGPT Plus via codex, Gemini via agy) atau bawa API key sendiri (DeepSeek, OpenAI, Ollama) tanpa markup token bulanan.',
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
                  title: 'Kolaborasi Tim & 1-Klik Undangan WhatsApp',
                  text: 'Undang staf dan admin dalam 1-klik via link WhatsApp tanpa ribet konfigurasi server email. Hak akses Owner vs Member menjaga kredensial AI tetap aman.',
                },
                {
                  Icon: Database,
                  title: 'CRM Kontak & Memori Bisnis (Grounding)',
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
            <Link to="/login" className="action-primary min-h-12 px-7 text-base font-semibold shadow-xs">
              {t('Mulai Sekarang')}
              <ArrowRight className="h-4 w-4" strokeWidth={1.5} />
            </Link>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 border-t border-border/60 px-6 py-6 text-sm text-muted-foreground sm:px-8">
        <p>cuan · {t('Your always-on project manager.')}</p>
        <div className="flex gap-5">
          <a
            className="inline-flex min-h-11 items-center hover:text-foreground transition-colors"
            href={repository + '#readme'}
          >
            {t('Self-hosting guide')}
          </a>
          <a
            className="inline-flex min-h-11 items-center hover:text-foreground transition-colors"
            href={repository}
          >
            GitHub
          </a>
        </div>
      </footer>
    </div>
  )
}
