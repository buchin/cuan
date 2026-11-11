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
  Coins,
  Sparkles,
  Bot,
  Terminal,
  ExternalLink,
  Check,
  X,
  Minus,
} from 'lucide-react'
import { CuanMark } from '../../components/brand/CuanMark'
import { useLanguage } from '../../context/LanguageContext'
import { useTheme } from '../../context/ThemeContext'

const repository = 'https://github.com/buchin/cuan'

export function LandingPage() {
  const { t, language, setPreference } = useLanguage()
  const { resolvedTheme, toggleTheme } = useTheme()

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
            href="#comparison"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground sm:inline-flex"
          >
            {t('Perbandingan')}
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
              <span>{t('AI Project Manager untuk Bisnis & Kreator')}</span>
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
              {t('Bebas biaya token dobel · BYOK & CLI resmi (Claude, Codex, DeepSeek, Gemini)')}
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
                  Icon: Bot,
                  title: 'Chatbot web cuma memberi saran teks, eksekusi tetap manual',
                  refText: 'ChatGPT, Claude.ai',
                  refUrl: 'https://chatgpt.com',
                  desc: 'Chatbot percakapan dirancang untuk tanya-jawab dan drafting ide. Mereka tidak terhubung langsung ke server atau database bisnis Anda, sehingga Anda tetap harus manual copy-paste kode, menyatukan webhook, dan tebak-tebak errornya sendiri.',
                },
                {
                  Icon: Terminal,
                  title: 'Tool koding CLI dibuat untuk programmer di terminal lokal',
                  refText: 'Claude Code, Antigravity, Codex',
                  refUrl: 'https://docs.anthropic.com/en/docs/agents-and-tools/claude-code',
                  desc: 'Sangat andal untuk developer, tetapi wajib mengoperasikan layar terminal hitam (bash/git) yang membingungkan non-teknis. Berjalan di laptop lokal sehingga langsung mati begitu laptop Anda tutup atau sleep, dan tidak bisa diakses santai dari HP.',
                },
                {
                  Icon: Activity,
                  title: 'Agen otonom (Hermes, Muse, Nod) minim rem pengaman & bukan untuk bisnis',
                  refText: 'Hermes, Muse, Nod',
                  refUrl: 'https://github.com/NousResearch/hermes-agent',
                  desc: 'Bagus untuk riset pengembang, tetapi bukan untuk operasional bisnis harian. Tanpa Approval Card dan verifikasi visual, eksekusi mandiri rawan looping token tak berujung dan mengubah konfigurasi tanpa izin pemilik bisnis.',
                },
              ].map(({ Icon, title, refText, refUrl, desc }) => (
                <div
                  key={title}
                  className="flex flex-col justify-between rounded-xl border border-border/60 bg-card p-6 shadow-2xs transition-colors hover:border-border"
                >
                  <div>
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
                  <div className="mt-5 pt-3.5 border-t border-border/40">
                    <a
                      href={refUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1.5 text-[11px] font-mono text-muted-foreground/70 hover:text-foreground transition-colors"
                      title={t('Lihat referensi resmi')}
                    >
                      <span>{t('Referensi:')} {refText}</span>
                      <ExternalLink className="h-3 w-3 opacity-60" />
                    </a>
                  </div>
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

                <div className="mt-6 -mx-2 -mb-2">
                  <img
                    src="/landing/step-chat.png"
                    alt={t('Pratinjau chat Cuan')}
                    width={1098}
                    height={1610}
                    loading="lazy"
                    className="w-full h-auto object-contain [filter:drop-shadow(0_12px_24px_rgba(0,0,0,0.06))]"
                  />
                </div>
              </li>

              {/* STEP 02: APPROVAL CARD */}
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

                <div className="mt-6 -mx-2 -mb-2">
                  <img
                    src="/landing/step-approve.png"
                    alt={t('Pratinjau Approval Card Cuan')}
                    width={1098}
                    height={1610}
                    loading="lazy"
                    className="w-full h-auto object-contain [filter:drop-shadow(0_12px_24px_rgba(0,0,0,0.06))]"
                  />
                </div>
              </li>

              {/* STEP 03: BERES */}
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

                <div className="mt-6 -mx-2 -mb-2">
                  <img
                    src="/landing/step-done.png"
                    alt={t('Pratinjau tugas terverifikasi Cuan')}
                    width={1098}
                    height={1610}
                    loading="lazy"
                    className="w-full h-auto object-contain [filter:drop-shadow(0_12px_24px_rgba(0,0,0,0.06))]"
                  />
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
              <div className="mb-3 inline-flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5 text-emerald-500" />
                <span>{t('Mengapa Cuan Berbeda?')}</span>
              </div>
              <h2
                id="comparison-title"
                className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl md:text-4xl"
              >
                {t('Bukan Sekadar Chatbot atau Tool Programmer')}
              </h2>
              <p className="mt-3 text-sm leading-relaxed text-muted-foreground sm:text-base">
                {t(
                  'Perbandingan faktual antara chatbot percakapan biasa, tool koding developer, framework agen otonom, dan Cuan.',
                )}
              </p>
            </div>

            {/* DESKTOP TABLE VIEW (hidden sm:block) */}
            <div className="mt-10 hidden sm:block overflow-x-auto pb-4">
              <table className="w-full min-w-[760px] border-collapse text-left text-xs sm:text-sm">
                <thead>
                  <tr className="border-b border-border/60">
                    <th className="py-3.5 pr-4 font-semibold text-muted-foreground w-[24%]">
                      {t('Kebutuhan Bisnis Anda')}
                    </th>
                    <th className="px-3 py-3.5 font-medium text-muted-foreground text-center w-[19%]">
                      <div className="font-medium text-foreground">{t('Chatbot Web')}</div>
                      <a
                        href="https://chatgpt.com"
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-0.5 text-[10px] font-mono text-muted-foreground/60 hover:text-foreground mt-0.5"
                      >
                        ChatGPT/Claude <ExternalLink className="h-2.5 w-2.5 opacity-60" />
                      </a>
                    </th>
                    <th className="px-3 py-3.5 font-medium text-muted-foreground text-center w-[19%]">
                      <div className="font-medium text-foreground">{t('Tool Koding CLI')}</div>
                      <a
                        href="https://docs.anthropic.com/en/docs/agents-and-tools/claude-code"
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-0.5 text-[10px] font-mono text-muted-foreground/60 hover:text-foreground mt-0.5"
                      >
                        Claude Code/Codex <ExternalLink className="h-2.5 w-2.5 opacity-60" />
                      </a>
                    </th>
                    <th className="px-3 py-3.5 font-medium text-muted-foreground text-center w-[19%]">
                      <div className="font-medium text-foreground">{t('Agen Otonom')}</div>
                      <a
                        href="https://github.com/NousResearch/hermes-agent"
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-0.5 text-[10px] font-mono text-muted-foreground/60 hover:text-foreground mt-0.5"
                      >
                        Hermes / Muse / Nod <ExternalLink className="h-2.5 w-2.5 opacity-60" />
                      </a>
                    </th>
                    <th className="px-3 py-3.5 font-semibold text-foreground text-center w-[19%] bg-primary/5 dark:bg-primary/10 rounded-t-lg border-x border-t border-border/50">
                      <div className="inline-flex items-center gap-1.5 font-semibold">
                        <CuanMark decorative className="h-4 w-4" />
                        cuan
                      </div>
                      <div className="text-[10px] text-muted-foreground font-normal mt-0.5">
                        {t('AI Project Manager')}
                      </div>
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/40">
                  {[
                    {
                      label: 'Lingkungan eksekusi',
                      chatbot: { icon: 'x', text: 'Hanya teks di browser' },
                      coder: { icon: 'warn', text: 'Terminal laptop lokal' },
                      hermes: { icon: 'warn', text: 'Python di mesin dev' },
                      cuan: { icon: 'check', text: 'Server VPS 24/7 mandiri' },
                    },
                    {
                      label: 'Antarmuka & kemudahan pakai',
                      chatbot: { icon: 'warn', text: 'Chat teks saja' },
                      coder: { icon: 'x', text: 'Wajib terminal hitam & git' },
                      hermes: { icon: 'x', text: 'CLI & dashboard teknis' },
                      cuan: { icon: 'check', text: 'Web & Mobile tenang (ramah non-teknis)' },
                    },
                    {
                      label: 'Rem pengaman persetujuan',
                      chatbot: { icon: 'x', text: 'Tidak ada (hanya saran)' },
                      coder: { icon: 'warn', text: 'Prompt teks [y/n]' },
                      hermes: { icon: 'x', text: 'Eksekusi mandiri tanpa gate' },
                      cuan: { icon: 'check', text: 'Approval Card 1-Tap' },
                    },
                    {
                      label: 'Quality Gate & verifikasi selesai',
                      chatbot: { icon: 'x', text: 'Tidak ada pengujian' },
                      coder: { icon: 'warn', text: 'Testing kode programmer' },
                      hermes: { icon: 'warn', text: 'Self-evaluation LLM' },
                      cuan: { icon: 'check', text: 'Bukti visual + DoD otomatis' },
                    },
                    {
                      label: 'Alat operasional bisnis (WhatsApp/CRM)',
                      chatbot: { icon: 'x', text: 'Tidak ada' },
                      coder: { icon: 'x', text: 'Tidak ada' },
                      hermes: { icon: 'warn', text: 'Bridge bot dasar (tanpa CRM)' },
                      cuan: { icon: 'check', text: 'Inbox WhatsApp & CRM Kontak' },
                    },
                    {
                      label: 'Model biaya token AI',
                      chatbot: { icon: 'x', text: 'Langganan per akun' },
                      coder: { icon: 'check', text: 'Langganan developer' },
                      hermes: { icon: 'warn', text: 'Bayar API per token' },
                      cuan: { icon: 'check', text: 'Bebas dobel biaya (CLI / BYOK)' },
                    },
                  ].map((row, idx, arr) => (
                    <tr key={row.label} className="hover:bg-muted/30 transition-colors">
                      <td className="py-3.5 pr-4 font-medium text-foreground">
                        {t(row.label)}
                      </td>
                      <td className="px-3 py-3.5 text-center text-muted-foreground">
                        <div className="flex flex-col items-center gap-1">
                          {row.chatbot.icon === 'check' && <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />}
                          {row.chatbot.icon === 'warn' && <Minus className="h-4 w-4 text-amber-500/70" />}
                          {row.chatbot.icon === 'x' && <X className="h-4 w-4 text-muted-foreground/40" />}
                          <span className="text-[11px] leading-tight">{t(row.chatbot.text)}</span>
                        </div>
                      </td>
                      <td className="px-3 py-3.5 text-center text-muted-foreground">
                        <div className="flex flex-col items-center gap-1">
                          {row.coder.icon === 'check' && <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />}
                          {row.coder.icon === 'warn' && <Minus className="h-4 w-4 text-amber-500/70" />}
                          {row.coder.icon === 'x' && <X className="h-4 w-4 text-muted-foreground/40" />}
                          <span className="text-[11px] leading-tight">{t(row.coder.text)}</span>
                        </div>
                      </td>
                      <td className="px-3 py-3.5 text-center text-muted-foreground">
                        <div className="flex flex-col items-center gap-1">
                          {row.hermes.icon === 'check' && <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />}
                          {row.hermes.icon === 'warn' && <Minus className="h-4 w-4 text-amber-500/70" />}
                          {row.hermes.icon === 'x' && <X className="h-4 w-4 text-muted-foreground/40" />}
                          <span className="text-[11px] leading-tight">{t(row.hermes.text)}</span>
                        </div>
                      </td>
                      <td className={`px-3 py-3.5 text-center bg-primary/5 dark:bg-primary/10 border-x border-border/50 ${idx === arr.length - 1 ? 'border-b rounded-b-lg' : ''}`}>
                        <div className="flex flex-col items-center gap-1 font-medium text-foreground">
                          {row.cuan.icon === 'check' && <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />}
                          {row.cuan.icon === 'warn' && <Minus className="h-4 w-4 text-amber-500/70" />}
                          {row.cuan.icon === 'x' && <X className="h-4 w-4 text-muted-foreground/40" />}
                          <span className="text-[11px] leading-tight text-foreground font-medium">{t(row.cuan.text)}</span>
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
                  chatbot: { icon: 'x', text: 'Hanya teks di browser' },
                  coder: { icon: 'warn', text: 'Terminal laptop lokal' },
                  hermes: { icon: 'warn', text: 'Python di mesin dev' },
                  cuan: { icon: 'check', text: 'Server VPS 24/7 mandiri' },
                },
                {
                  label: 'Antarmuka & kemudahan pakai',
                  desc: 'Kenyamanan akses untuk tim non-teknis',
                  chatbot: { icon: 'warn', text: 'Chat teks saja' },
                  coder: { icon: 'x', text: 'Wajib terminal hitam & git' },
                  hermes: { icon: 'x', text: 'CLI & dashboard teknis' },
                  cuan: { icon: 'check', text: 'Web & Mobile tenang (ramah non-teknis)' },
                },
                {
                  label: 'Rem pengaman persetujuan',
                  desc: 'Kontrol manusia sebelum aksi menyentuh sistem',
                  chatbot: { icon: 'x', text: 'Tidak ada (hanya saran)' },
                  coder: { icon: 'warn', text: 'Prompt teks [y/n]' },
                  hermes: { icon: 'x', text: 'Eksekusi mandiri tanpa gate' },
                  cuan: { icon: 'check', text: 'Approval Card 1-Tap' },
                },
                {
                  label: 'Quality Gate & verifikasi selesai',
                  desc: 'Pembuktian nyata sebelum tugas dianggap tuntas',
                  chatbot: { icon: 'x', text: 'Tidak ada pengujian' },
                  coder: { icon: 'warn', text: 'Testing kode programmer' },
                  hermes: { icon: 'warn', text: 'Self-evaluation LLM' },
                  cuan: { icon: 'check', text: 'Bukti visual + DoD otomatis' },
                },
                {
                  label: 'Alat operasional bisnis (WhatsApp/CRM)',
                  desc: 'Integrasi chat pelanggan dan database kontak',
                  chatbot: { icon: 'x', text: 'Tidak ada' },
                  coder: { icon: 'x', text: 'Tidak ada' },
                  hermes: { icon: 'warn', text: 'Bridge bot dasar (tanpa CRM)' },
                  cuan: { icon: 'check', text: 'Inbox WhatsApp & CRM Kontak' },
                },
                {
                  label: 'Model biaya token AI',
                  desc: 'Struktur biaya pemakaian kecerdasan buatan',
                  chatbot: { icon: 'x', text: 'Langganan per akun' },
                  coder: { icon: 'check', text: 'Langganan developer' },
                  hermes: { icon: 'warn', text: 'Bayar API per token' },
                  cuan: { icon: 'check', text: 'Bebas dobel biaya (CLI / BYOK)' },
                },
              ].map((row) => (
                <div
                  key={row.label}
                  className="rounded-xl border border-border/60 bg-card p-4 shadow-2xs"
                >
                  <div className="mb-3">
                    <h3 className="font-display text-sm font-semibold text-foreground">
                      {t(row.label)}
                    </h3>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {t(row.desc)}
                    </p>
                  </div>

                  {/* Cuan Highlight Card */}
                  <div className="mb-3 flex items-start gap-2.5 rounded-lg border border-primary/25 bg-primary/5 dark:bg-primary/10 p-3">
                    <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400 mt-0.5 shrink-0" />
                    <div>
                      <div className="inline-flex items-center gap-1.5 text-xs font-semibold text-foreground">
                        <CuanMark decorative className="h-3.5 w-3.5" />
                        cuan ({t('AI Project Manager')})
                      </div>
                      <div className="mt-0.5 text-xs font-medium text-foreground">
                        {t(row.cuan.text)}
                      </div>
                    </div>
                  </div>

                  {/* Alternatives Comparison List */}
                  <div className="space-y-2 border-t border-border/40 pt-2.5 text-xs">
                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-[11px]">Chatbot (ChatGPT/Claude)</span>
                      <span className="inline-flex items-center gap-1 text-[11px] font-medium text-foreground">
                        {row.chatbot.icon === 'check' && <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />}
                        {row.chatbot.icon === 'warn' && <Minus className="h-3.5 w-3.5 text-amber-500/80" />}
                        {row.chatbot.icon === 'x' && <X className="h-3.5 w-3.5 text-muted-foreground/40" />}
                        <span>{t(row.chatbot.text)}</span>
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-[11px]">Tool Koding (Claude Code)</span>
                      <span className="inline-flex items-center gap-1 text-[11px] font-medium text-foreground">
                        {row.coder.icon === 'check' && <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />}
                        {row.coder.icon === 'warn' && <Minus className="h-3.5 w-3.5 text-amber-500/80" />}
                        {row.coder.icon === 'x' && <X className="h-3.5 w-3.5 text-muted-foreground/40" />}
                        <span>{t(row.coder.text)}</span>
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="text-[11px]">{t('Hermes / Muse / Nod')}</span>
                      <span className="inline-flex items-center gap-1 text-[11px] font-medium text-foreground">
                        {row.hermes.icon === 'check' && <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />}
                        {row.hermes.icon === 'warn' && <Minus className="h-3.5 w-3.5 text-amber-500/80" />}
                        {row.hermes.icon === 'x' && <X className="h-3.5 w-3.5 text-muted-foreground/40" />}
                        <span>{t(row.hermes.text)}</span>
                      </span>
                    </div>
                  </div>
                </div>
              ))}

              {/* Reference Links on Mobile */}
              <div className="mt-4 flex flex-wrap items-center justify-center gap-x-4 gap-y-2 text-[11px] text-muted-foreground">
                <a
                  href="https://chatgpt.com"
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 hover:text-foreground"
                >
                  ChatGPT/Claude <ExternalLink className="h-2.5 w-2.5 opacity-60" />
                </a>
                <span>·</span>
                <a
                  href="https://docs.anthropic.com/en/docs/agents-and-tools/claude-code"
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 hover:text-foreground"
                >
                  Claude Code <ExternalLink className="h-2.5 w-2.5 opacity-60" />
                </a>
                <span>·</span>
                <a
                  href="https://github.com/NousResearch/hermes-agent"
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 hover:text-foreground"
                >
                  Hermes / Muse / Nod <ExternalLink className="h-2.5 w-2.5 opacity-60" />
                </a>
              </div>
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
                  title: 'Multi-channel unified customer inbox',
                  text: 'Manage customer conversations from WhatsApp and live chat in one calm, monochromatic stream with quiet whisper notes for operators.',
                },
                {
                  Icon: Users,
                  title: 'Kolaborasi Tim & 1-Klik Undangan WhatsApp',
                  text: 'Undang staf dan admin dalam 1-klik via link WhatsApp tanpa ribet konfigurasi server email. Hak akses Owner vs Member menjaga kredensial AI tetap aman.',
                },
                {
                  Icon: Sparkles,
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
