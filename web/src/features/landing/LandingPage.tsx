import { useMemo } from 'react'
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
  Terminal,
  Sun,
  Moon,
  Clock,
  Coins,
  AlertCircle,
} from 'lucide-react'
import { CuanMark } from '../../components/brand/CuanMark'
import { ApprovalCard } from '../inbox/ApprovalCard'
import { TaskCompletionCard } from '../inbox/TaskCompletionCard'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { useTheme } from '../../context/ThemeContext'

const repository = 'https://github.com/buchin/cuan'

export function LandingPage() {
  const { t, language, setPreference } = useLanguage()
  const { resolvedTheme, toggleTheme } = useTheme()

  const sampleCompletedTask: Task = useMemo(
    () => ({
      id: 't_87e92',
      title: t('Integrasi Webhook Form & Bot WhatsApp'),
      status: 'done',
      priority: 'normal',
      summary: t(
        'Webhook receiver aktif dan terhubung ke WhatsApp. 3 skenario tes pengiriman notifikasi otomatis sukses tanpa downtime.'
      ),
      dod_checklist: [
        { text: t('Pasang endpoint receiver webhook form order'), completed: true },
        { text: t('Format template notifikasi WhatsApp pesanan'), completed: true },
        { text: t('Uji coba simulasi kirim data (DoD 100% pass)'), completed: true },
      ],
      created_at: new Date(Date.now() - 300000).toISOString(),
      updated_at: new Date().toISOString(),
    }),
    [t],
  )

  return (
    <div className="min-h-dvh bg-background text-foreground">
      {/* Navigation Header */}
      <header className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-5 sm:px-8">
        <Link
          to="/"
          className="inline-flex min-h-11 items-center gap-2 font-semibold tracking-tight"
          aria-label="Cuan"
        >
          <CuanMark decorative className="h-7 w-7" />
          <span className="text-xl">cuan</span>
        </Link>
        <nav
          aria-label={t('Website navigation')}
          className="flex items-center gap-2 sm:gap-4"
        >
          <a
            href="#problem"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground sm:inline-flex"
          >
            {t('Masalah')}
          </a>
          <a
            href="#how-it-works"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground sm:inline-flex"
          >
            {t('Cara kerja')}
          </a>
          <a
            href="#features"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground sm:inline-flex"
          >
            {t('Fitur')}
          </a>
          <a
            href={repository}
            target="_blank"
            rel="noreferrer"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground sm:inline-flex"
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
              <Sun className="h-4 w-4" />
            ) : (
              <Moon className="h-4 w-4" />
            )}
          </button>

          <Link to="/login" className="action-secondary min-h-9 px-3.5 text-xs">
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
            <div className="mb-3.5 inline-flex items-center gap-2 rounded-full border border-border/60 bg-muted/30 px-3 py-1 text-xs font-medium text-muted-foreground shadow-2xs">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" aria-hidden="true" />
              <span>{t('AI Agent for Marketers')}</span>
            </div>
            <h1
              id="landing-title"
              className="font-display text-4xl font-bold tracking-tight text-[#1B1616] dark:text-foreground sm:text-5xl md:text-6xl lg:text-[68px] lg:leading-[1.08]"
            >
              {t('Chat. Setujui. Beres.')}
            </h1>
            <p className="mt-3 text-base font-normal text-[#5A5252] dark:text-muted-foreground sm:text-xl sm:leading-normal">
              {t('Kelola bisnis tanpa pusing urusan teknis.')}
            </p>
            <div className="mt-5 flex flex-wrap items-center justify-center gap-3">
              <Link to="/login" className="action-primary min-h-11 px-6 text-sm font-medium shadow-xs">
                {t('Mulai Sekarang')}
                <ArrowRight className="h-4 w-4" />
              </Link>
              <a
                href="#problem"
                className="action-secondary min-h-11 px-5 text-sm font-medium"
              >
                {t('Pelajari Cara Kerja')}
                <ChevronRight className="h-4 w-4" />
              </a>
            </div>
          </div>

          {/* Centered 3-Device Mockup Showcase - Optimized Above the Fold */}
          <div className="mx-auto mt-6 max-w-4xl sm:mt-8">
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
          <div className="mx-auto mt-8 flex flex-wrap items-center justify-center gap-x-8 gap-y-3 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <ShieldCheck className="h-3.5 w-3.5" />
              {t('Open source under Apache-2.0')}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <GitBranch className="h-3.5 w-3.5" />
              {t('Self-hosted on your own VPS')}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Zap className="h-3.5 w-3.5" />
              {t('Zero SaaS markup · BYOK & official CLIs')}
            </span>
          </div>
        </section>

        {/* 2. THE PROBLEM & AGITATE SECTION (P-A in PAS-P) */}
        <section
          id="problem"
          className="scroll-mt-6 border-t border-border/60 bg-muted/20 px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="problem-title"
        >
          <div className="mx-auto max-w-6xl">
            <div className="max-w-3xl">
              <h2
                id="problem-title"
                className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl md:text-4xl"
              >
                {t('Punya segudang ide bisnis, tapi sering mentok di urusan teknis?')}
              </h2>
              <p className="mt-3 text-sm leading-relaxed text-muted-foreground sm:text-base">
                {t(
                  'Banyak pebisnis dan marketer membuang puluhan jam berharga mengurusi operasional manual, membalas chat berulang, integrasi webhook, hingga error teknis alih-alih fokus mencari cuan.',
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
                  title: 'Langganan SaaS AI luar negeri bikin boncos',
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
                  className="rounded-xl border border-border/60 bg-card p-6 shadow-2xs transition-colors hover:border-border"
                >
                  <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-muted/60 text-muted-foreground">
                    <Icon className="h-5 w-5" />
                  </div>
                  <h3 className="font-display text-base font-semibold text-foreground">
                    {t(title)}
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
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
              <div className="mb-3 inline-flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
                <span>{t('Cara kerja Cuan')}</span>
              </div>
              <h2
                id="how-title"
                className="font-display text-2xl font-bold tracking-tight sm:text-3xl md:text-4xl"
              >
                {t('Kenalkan Cuan: AI Project Manager Siap Kerja Tanpa Pengawasan')}
              </h2>
              <p className="mt-3 text-sm leading-relaxed text-muted-foreground sm:text-base">
                {t(
                  'Anda bertindak sebagai direktur yang memberi instruksi lewat chat santai. Cuan memimpin tim agen spesialis—mulai dari otomasi operasional bisnis, layanan customer support, pembukuan mutasi, hingga koding dan deploy di server Anda sendiri sampai tuntas terverifikasi.',
                )}
              </p>
            </div>

            <ol className="mt-12 grid gap-8 lg:grid-cols-3">
              {/* STEP 01: CHAT */}
              <li className="flex flex-col justify-between rounded-xl border border-border/50 bg-card p-6 shadow-2xs">
                <div>
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs font-semibold text-muted-foreground tabular-nums">
                      01
                    </span>
                    <span className="rounded-full bg-muted/60 px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                      Chat
                    </span>
                  </div>
                  <h3 className="font-display mt-4 text-base font-semibold leading-snug">
                    {t('Describe what you need in plain words')}
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {t(
                      'Start in Chat from your phone or desktop. Share an operational goal, customer support workflow, financial receipt, or web feature. Cuan analyzes your context automatically.',
                    )}
                  </p>
                </div>

                {/* Real Cuan Chat Preview */}
                <div className="mt-6 rounded-xl border border-border/60 bg-muted/20 p-3.5 text-left text-xs space-y-3">
                  <div className="flex items-center justify-between border-b border-border/40 pb-2">
                    <div className="flex items-center gap-1.5 font-medium text-foreground">
                      <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                      <span className="font-semibold text-xs">@cuan</span>
                      <span className="text-[10px] text-muted-foreground font-mono">Project Manager</span>
                    </div>
                    <span className="text-[10px] text-muted-foreground font-mono">{t('Baru saja')}</span>
                  </div>
                  <div className="space-y-2">
                    <div className="rounded-lg bg-card/80 p-2.5 border border-border/50">
                      <div className="text-[10px] font-semibold text-muted-foreground mb-1 flex items-center gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-blue-500" />
                        {t('Anda (Owner)')}
                      </div>
                      <p className="text-foreground leading-relaxed">
                        &ldquo;{t('Tolong hubungkan webhook form order ke bot WhatsApp notifikasi admin.')}&rdquo;
                      </p>
                    </div>
                    <div className="rounded-lg bg-primary/5 p-2.5 border border-primary/20">
                      <div className="text-[10px] font-semibold text-primary mb-1 flex items-center gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                        @cuan
                      </div>
                      <p className="text-foreground/90 leading-relaxed text-[11px]">
                        {t('Siap! Konteks repositori dan webhook terbaca. Saya siapkan Approval Card rencana kerja dan kriteria pengujian sebelum eksekusi.')}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center justify-between pt-1 border-t border-border/30 text-[10px] text-muted-foreground">
                    <span>{t('Pilihan cepat:')}</span>
                    <div className="flex gap-1.5">
                      <span className="rounded-md bg-primary/10 text-primary font-medium px-2 py-0.5 border border-primary/20">
                        ✓ {t('Lanjutkan')}
                      </span>
                      <span className="rounded-md bg-muted px-2 py-0.5 text-muted-foreground">
                        {t('Sesuaikan')}
                      </span>
                    </div>
                  </div>
                </div>
              </li>

              {/* STEP 02: APPROVAL CARD (Real Component) */}
              <li className="flex flex-col justify-between rounded-xl border border-border/50 bg-card p-6 shadow-2xs">
                <div>
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs font-semibold text-muted-foreground tabular-nums">
                      02
                    </span>
                    <span className="rounded-full bg-muted/60 px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                      Setujui
                    </span>
                  </div>
                  <h3 className="font-display mt-4 text-base font-semibold leading-snug">
                    {t('Review and approve a clear plan')}
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {t(
                      'Before touching anything, Cuan presents a structured Approval Card outlining the scope, specialist agents (@sm, @dev, @support), and verification criteria. One tap to approve.',
                    )}
                  </p>
                </div>

                {/* Authentic ApprovalCard from Cuan codebase */}
                <div className="mt-6 text-left">
                  <ApprovalCard
                    plan={{
                      title: t('Integrasi Webhook Form & Bot WhatsApp'),
                      project_name: 'landing-cuan',
                      description: `- ${t('Pasang endpoint receiver webhook form order')}\n- ${t('Format template notifikasi WhatsApp pesanan')}\n- ${t('Uji coba simulasi kirim data (DoD 100% pass)')}`,
                    }}
                    disabled
                    className="mt-0 pointer-events-none select-none shadow-none"
                  />
                </div>
              </li>

              {/* STEP 03: BERES (Real TaskCompletionCard Component) */}
              <li className="flex flex-col justify-between rounded-xl border border-border/50 bg-card p-6 shadow-2xs">
                <div>
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs font-semibold text-muted-foreground tabular-nums">
                      03
                    </span>
                    <span className="rounded-full bg-muted/60 px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                      Beres
                    </span>
                  </div>
                  <h3 className="font-display mt-4 text-base font-semibold leading-snug">
                    {t('Verified outcomes delivered to your server')}
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {t(
                      'Autonomous agents execute operations, handle support, or write code, verifying Definition of Done criteria, and report back via an authentic Task Card with 100% proof.',
                    )}
                  </p>
                </div>

                {/* Authentic TaskCompletionCard from Cuan codebase */}
                <div className="mt-6 text-left">
                  <TaskCompletionCard
                    task={sampleCompletedTask}
                    className="mt-0 pointer-events-none select-none shadow-none text-left"
                  />
                </div>
              </li>
            </ol>
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
                className="font-display text-2xl font-bold tracking-tight sm:text-3xl"
              >
                {t('Tetap memegang kendali penuh, tanpa pusing urusan teknis')}
              </h2>
              <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
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
                  Icon: GitBranch,
                  title: 'Your workspace, your AI',
                  text: 'Self-host Cuan and connect your own AI subscriptions (Claude, Codex, DeepSeek). Keep your projects and credentials on your own server.',
                },
                {
                  Icon: CheckCircle2,
                  title: 'Autonomous Definition of Done loop',
                  text: 'Built-in quality gate runs targeted tests and verifies real component outputs automatically before marking any task complete.',
                },
                {
                  Icon: MessageSquare,
                  title: 'Multi-channel unified customer inbox',
                  text: 'Manage customer conversations from WhatsApp and live chat in one calm, monochromatic stream with quiet whisper notes for operators.',
                },
                {
                  Icon: Users,
                  title: 'Team collaboration and role access',
                  text: 'Collaborate with team members under robust role-based access control, keeping administrative settings and AI provider keys protected.',
                },
                {
                  Icon: Terminal,
                  title: 'Live interactive terminal inspection',
                  text: 'Stream stdout/stderr logs in real time with interactive chat steering to guide agents without restarting sessions from scratch.',
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
                <div key={title} className="rounded-xl border border-border/50 bg-card p-6 shadow-2xs">
                  <Icon className="mb-3.5 h-4.5 w-4.5 text-muted-foreground" />
                  <h3 className="text-sm font-semibold text-foreground">{t(title)}</h3>
                  <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
                    {t(text)}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* 5. FINAL PITCH / CTA SECTION */}
        <section className="border-t border-border px-6 py-16 sm:px-8 sm:py-20">
          <div className="mx-auto flex max-w-6xl flex-col items-start justify-between gap-6 sm:flex-row sm:items-center">
            <div className="max-w-xl">
              <h2 className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
                {t('Siap bebas dari pusing urusan teknis?')}
              </h2>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                {t('Fokus besarkan bisnis dan cari cuan. Biarkan agen Cuan yang merapikan operasional, melayani pelanggan, dan mengeksekusi kebutuhan teknis di server Anda.')}
              </p>
              <p className="mt-3 text-xs text-muted-foreground/80">
                {t('Open source · Lisensi Apache-2.0 · Kendali penuh di VPS sendiri')}
              </p>
            </div>
            <Link to="/login" className="action-primary min-h-12 px-6 text-base font-medium shadow-xs">
              {t('Mulai Sekarang')}
              <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 border-t border-border px-5 py-6 text-xs text-muted-foreground sm:px-8">
        <p>cuan · {t('Your always-on project manager.')}</p>
        <div className="flex gap-5">
          <a
            className="inline-flex min-h-11 items-center hover:text-foreground"
            href={repository + '#readme'}
          >
            {t('Self-hosting guide')}
          </a>
          <a
            className="inline-flex min-h-11 items-center hover:text-foreground"
            href={repository}
          >
            GitHub
          </a>
        </div>
      </footer>
    </div>
  )
}
