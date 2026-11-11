import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  Globe,
  Sparkles,
  Sun,
  Moon,
  HelpCircle,
  Server,
  Heart,
} from 'lucide-react'
import { CuanMark } from '../../components/brand/CuanMark'
import { useLanguage } from '../../context/LanguageContext'
import { useTheme } from '../../context/ThemeContext'

interface SponsorItem {
  id: string
  brand_name: string
  website_url?: string
  logo_url?: string
  slogan?: string
}

export function SponsorPage() {
  const { t, language, setPreference } = useLanguage()
  const { resolvedTheme, toggleTheme } = useTheme()

  const [sponsors, setSponsors] = useState<SponsorItem[]>([])

  useEffect(() => {
    // Fetch live approved sponsors if available
    fetch('/api/sponsors/current')
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data?.sponsors) {
          setSponsors(data.sponsors)
        }
      })
      .catch(() => null)
  }, [])

  const foundingBackerCheckoutURL = 'https://dojo.cc/checkout/cuan-founding-backer'
  const businessSponsorCheckoutURL = 'https://dojo.cc/checkout/cuan-business-sponsor'

  return (
    <div className="min-h-dvh bg-background text-foreground">
      {/* Top Navigation */}
      <header className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-6 sm:px-8 border-b border-border/40">
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

        <div className="flex items-center gap-2 sm:gap-4">
          <Link
            to="/"
            className="inline-flex min-h-9 items-center gap-1.5 px-3 text-xs sm:text-sm text-muted-foreground hover:text-foreground transition-colors whitespace-nowrap"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">{t('Kembali ke Beranda')}</span>
            <span className="sm:hidden">{t('Beranda')}</span>
          </Link>

          {/* Language Switcher */}
          <button
            type="button"
            onClick={() => setPreference(language === 'id' ? 'en' : 'id')}
            className="inline-flex min-h-9 items-center justify-center rounded-md border border-border/50 bg-muted/30 px-2.5 text-xs font-medium text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors cursor-pointer"
            title={language === 'id' ? 'Ganti ke English' : 'Switch to Bahasa Indonesia'}
          >
            {language === 'id' ? 'ID' : 'EN'}
          </button>

          {/* Theme Switcher */}
          <button
            type="button"
            onClick={toggleTheme}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border/50 bg-muted/30 text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors cursor-pointer"
            aria-label="Ganti tema"
            title={resolvedTheme === 'dark' ? 'Tema terang' : 'Tema gelap'}
          >
            {resolvedTheme === 'dark' ? (
              <Sun className="h-4 w-4" strokeWidth={1.5} />
            ) : (
              <Moon className="h-4 w-4" strokeWidth={1.5} />
            )}
          </button>
        </div>
      </header>

      <main className="pb-24">
        {/* Hero Section */}
        <section className="mx-auto max-w-4xl px-6 pt-12 pb-10 text-center sm:px-8 sm:pt-16 sm:pb-14">
          <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-border/60 bg-muted/40 px-3.5 py-1.5 text-xs font-semibold text-muted-foreground">
            <Heart className="h-3.5 w-3.5 text-rose-500 fill-rose-500/20" />
            <span>{t('Batch 1 · Founding Backers')}</span>
          </div>

          <h1 className="font-display text-4xl font-bold tracking-tight text-foreground sm:text-5xl md:text-6xl lg:leading-[1.1]">
            {t('Dukung Peluncuran Cuan v1.0 & Amankan Akses VIP Pertama')}
          </h1>

          <p className="mt-4 text-base sm:text-lg md:text-xl text-muted-foreground leading-relaxed max-w-2xl mx-auto">
            {t(
              'Cuan adalah software otonom mandiri (self-hosted di VPS) yang dirancang untuk pebisnis dan marketer non-teknis. Dukungan Anda mempercepat kurasi kode, integrasi ekosistem, dan peluncuran open source resmi.',
            )}
          </p>
        </section>

        {/* Value Highlights */}
        <section className="mx-auto max-w-5xl px-6 sm:px-8 mb-12">
          <div className="grid gap-6 sm:grid-cols-3">
            <div className="rounded-2xl border border-border/60 bg-card p-6 shadow-2xs">
              <div className="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-xl bg-muted/60 text-primary">
                <Sparkles className="h-5 w-5" strokeWidth={1.5} />
              </div>
              <h3 className="font-display text-base font-bold text-foreground">
                {t('Early VIP Access')}
              </h3>
              <p className="mt-2 text-xs sm:text-sm text-muted-foreground leading-relaxed">
                {t(
                  'Akses build rilis privat sebelum peluncuran publik. Coba fitur terbaru, integrasi antrean kerja otonom, dan paket skill lebih dulu.',
                )}
              </p>
            </div>

            <div className="rounded-2xl border border-border/60 bg-card p-6 shadow-2xs">
              <div className="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-xl bg-muted/60 text-primary">
                <Globe className="h-5 w-5" strokeWidth={1.5} />
              </div>
              <h3 className="font-display text-base font-bold text-foreground">
                {t('Logo & Dofollow Backlink')}
              </h3>
              <p className="mt-2 text-xs sm:text-sm text-muted-foreground leading-relaxed">
                {t(
                  'Nama brand atau profil Anda dicantumkan secara permanen di GitHub README dan landing page resmi Cuan sebagai bukti kontribusi nyata.',
                )}
              </p>
            </div>

            <div className="rounded-2xl border border-border/60 bg-card p-6 shadow-2xs">
              <div className="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-xl bg-muted/60 text-primary">
                <Server className="h-5 w-5" strokeWidth={1.5} />
              </div>
              <h3 className="font-display text-base font-bold text-foreground">
                {t('Konsultasi VPS & MCP')}
              </h3>
              <p className="mt-2 text-xs sm:text-sm text-muted-foreground leading-relaxed">
                {t(
                  'Kanal koordinasi langsung untuk bantuan konfigurasi VPS Ubuntu/Debian, pemasangan custom MCP server, dan grounding CRM profil bisnis Anda.',
                )}
              </p>
            </div>
          </div>
        </section>

        {/* Pricing Tiers Section */}
        <section className="mx-auto max-w-5xl px-6 sm:px-8 mb-16">
          <div className="text-center max-w-2xl mx-auto mb-10">
            <h2 className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              {t('Pilih Paket Dukungan Sponsor')}
            </h2>
            <p className="mt-2 text-sm sm:text-base text-muted-foreground">
              {t('Pembayaran satu kali (one-time contribution). Tanpa tagihan bulanan tersembunyi.')}
            </p>
          </div>

          <div className="grid gap-8 md:grid-cols-2 items-stretch">
            {/* TIER 1: FOUNDING BACKER */}
            <div className="flex flex-col justify-between rounded-2xl border border-border/80 bg-card p-6 sm:p-8 shadow-xs">
              <div>
                <div className="inline-flex items-center gap-1.5 rounded-full border border-border/60 bg-muted/40 px-3 py-1 text-xs font-semibold text-muted-foreground">
                  <span>{t('Untuk Marketer & Pengguna Awal')}</span>
                </div>
                <h3 className="font-display text-2xl font-bold text-foreground mt-3">
                  Founding Backer
                </h3>
                <p className="text-xs sm:text-sm text-muted-foreground mt-1 leading-relaxed">
                  {t('Dukungan ideal untuk pebisnis mandiri, solopreneur, dan penggemar otomasi AI.')}
                </p>

                <div className="mt-6 flex items-baseline gap-1.5">
                  <span className="font-display text-3xl sm:text-4xl font-bold text-foreground">
                    Rp 499.000
                  </span>
                  <span className="text-xs text-muted-foreground font-medium">/ {t('satu kali')}</span>
                </div>

                <div className="mt-6 space-y-3 border-t border-border/40 pt-6 text-xs sm:text-sm">
                  {[
                    'Akses VIP pertama ke build rilis Cuan sebelum publik',
                    'Nama / Profil tercantum permanen di GitHub README (Founding Backers)',
                    'Badge profil Founding Backer di komunitas',
                    'Akses kanal rilis prioritas untuk update langsung',
                    'Lifetime update dokumentasi instalasi & skill pack operasional',
                    'Akses early Discord / Telegram pengembang',
                  ].map((benefit) => (
                    <div key={benefit} className="flex items-start gap-2.5 text-foreground/90">
                      <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400 mt-0.5 shrink-0" strokeWidth={1.75} />
                      <span>{t(benefit)}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="mt-8 pt-6 border-t border-border/40">
                <a
                  href={foundingBackerCheckoutURL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="action-secondary w-full min-h-12 text-sm font-semibold justify-center gap-2 cursor-pointer"
                >
                  <span>{t('Dukung Sebagai Founding Backer')}</span>
                  <ArrowRight className="h-4 w-4" />
                </a>
                <p className="text-[11px] text-muted-foreground text-center mt-2">
                  {t('Pembayaran via Transfer Bank BCA (Lokal) & Stripe (Internasional)')}
                </p>
              </div>
            </div>

            {/* TIER 2: BUSINESS SPONSOR (HIGHLIGHTED) */}
            <div className="relative flex flex-col justify-between rounded-2xl border-2 border-primary/40 bg-primary/5 dark:bg-primary/10 p-6 sm:p-8 shadow-xs">
              <div className="absolute -top-3 right-6 rounded-full bg-primary px-3 py-0.5 text-[11px] font-semibold text-primary-foreground shadow-2xs">
                {t('Paling Direkomendasikan')}
              </div>

              <div>
                <div className="inline-flex items-center gap-1.5 rounded-full border border-primary/30 bg-primary/10 px-3 py-1 text-xs font-semibold text-primary">
                  <span>{t('Untuk Bisnis, Brand & Agensi')}</span>
                </div>
                <h3 className="font-display text-2xl font-bold text-foreground mt-3">
                  Business Sponsor
                </h3>
                <p className="text-xs sm:text-sm text-muted-foreground mt-1 leading-relaxed">
                  {t('Eksposur maksimal untuk brand Anda serta asistensi privat implementasi sistem.')}
                </p>

                <div className="mt-6 flex items-baseline gap-1.5">
                  <span className="font-display text-3xl sm:text-4xl font-bold text-foreground">
                    Rp 1.499.000
                  </span>
                  <span className="text-xs text-muted-foreground font-medium">/ {t('satu kali')}</span>
                </div>

                <div className="mt-6 space-y-3 border-t border-border/40 pt-6 text-xs sm:text-sm">
                  {[
                    'Semua hak & benefit Founding Backer',
                    'Logo brand & dofollow link permanen di Landing Page & GitHub README',
                    'Sesi konsultasi privat 1-on-1 via chat/call untuk arsitektur VPS & MCP',
                    'Prioritas permintaan integrasi format data & webhook bisnis Anda',
                    'Hak klaim lencana "Supported by Cuan Business Sponsor"',
                    'Koordinasi langsung dengan tim pengembang inti',
                  ].map((benefit) => (
                    <div key={benefit} className="flex items-start gap-2.5 text-foreground font-medium">
                      <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400 mt-0.5 shrink-0" strokeWidth={2} />
                      <span>{t(benefit)}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="mt-8 pt-6 border-t border-border/40">
                <a
                  href={businessSponsorCheckoutURL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="action-primary w-full min-h-12 text-sm font-semibold justify-center gap-2 shadow-xs cursor-pointer"
                >
                  <span>{t('Dukung Sebagai Business Sponsor')}</span>
                  <ArrowRight className="h-4 w-4" />
                </a>
                <p className="text-[11px] text-muted-foreground text-center mt-2">
                  {t('Pembayaran via Transfer Bank BCA (Lokal) & Stripe (Internasional)')}
                </p>
              </div>
            </div>
          </div>
        </section>

        {/* Existing Sponsors Showcase (if available) */}
        {sponsors.length > 0 && (
          <section className="mx-auto max-w-5xl px-6 sm:px-8 mb-16 text-center">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-6">
              {t('Sponsor & Backer Terdaftar')}
            </h3>
            <div className="flex flex-wrap items-center justify-center gap-6">
              {sponsors.map((sp) => (
                <div
                  key={sp.id}
                  className="inline-flex items-center gap-2 rounded-xl border border-border/60 bg-card px-4 py-2 text-xs font-semibold text-foreground shadow-2xs"
                >
                  {sp.logo_url && (
                    <img src={sp.logo_url} alt={sp.brand_name} className="h-5 w-auto object-contain" />
                  )}
                  <span>{sp.brand_name}</span>
                </div>
              ))}
            </div>
          </section>
        )}

        {/* FAQ Section */}
        <section className="mx-auto max-w-3xl px-6 sm:px-8">
          <div className="text-center mb-8">
            <h2 className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              {t('Pertanyaan yang Sering Diajukan')}
            </h2>
          </div>

          <div className="space-y-4">
            {[
              {
                q: 'Mengapa membuka program sponsor sebelum rilis publik?',
                a: 'Cuan adalah software open source mandiri (self-hosted) berlisensi Apache-2.0. Program sponsor ini mendanai kurasi kode, infrastruktur pengujian otomatis, serta penyusunan panduan instalasi 1-klik agar pebisnis non-teknis bisa langsung memakainya dengan tenang.',
              },
              {
                q: 'Apa metode pembayaran yang tersedia?',
                a: 'Pembayaran diproses aman via Dojo Checkout. Untuk sponsor lokal (Indonesia) mendukung Transfer Bank BCA, dan untuk sponsor internasional mendukung pembayaran via Stripe.',
              },
              {
                q: 'Bagaimana cara menyerahkan logo dan tautan website perusahaan?',
                a: 'Segera setelah proses pembayaran berhasil, Anda akan menerima email konfirmasi otomatis dengan formulir digital untuk mengunggah aset logo vektor/PNG transparan dan tautan website resmi yang ingin ditampilkan.',
              },
              {
                q: 'Kapan akses VIP build mulai diberikan?',
                a: 'Backer akan menerima undangan langsung ke repositori staging dan akses rilis privat via email sebelum tanggal pengumuman rilis open source publik.',
              },
            ].map(({ q, a }) => (
              <div key={q} className="rounded-xl border border-border/60 bg-card p-5 text-left shadow-2xs">
                <h4 className="font-display text-base font-bold text-foreground flex items-center gap-2">
                  <HelpCircle className="h-4 w-4 text-primary shrink-0" strokeWidth={1.75} />
                  <span>{t(q)}</span>
                </h4>
                <p className="mt-2 text-xs sm:text-sm text-muted-foreground leading-relaxed pl-6">
                  {t(a)}
                </p>
              </div>
            ))}
          </div>

          <div className="mt-12 text-center rounded-2xl border border-border/60 bg-muted/20 p-6 sm:p-8">
            <h3 className="font-display text-lg font-bold text-foreground">
              {t('Ada pertanyaan khusus sebelum bergabung?')}
            </h3>
            <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
              {t('Silakan hubungi kami via email di')} <a href="mailto:support@cuan.dojo.cc" className="font-medium text-primary hover:underline">support@cuan.dojo.cc</a>.
            </p>
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
        <div className="flex gap-5">
          <Link to="/" className="inline-flex min-h-11 items-center hover:text-foreground transition-colors">
            {t('Beranda')}
          </Link>
          <a
            className="inline-flex min-h-11 items-center hover:text-foreground transition-colors"
            href="https://github.com/buchin/cuan"
          >
            GitHub
          </a>
        </div>
      </footer>
    </div>
  )
}
