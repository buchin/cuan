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
      <header className="mx-auto flex min-h-20 max-w-6xl items-center justify-between gap-3 px-5 sm:px-8">
        <Link
          to="/"
          className="inline-flex min-h-11 items-center gap-2 font-semibold tracking-tight"
          aria-label="Cuan"
        >
          <CuanMark decorative className="h-8 w-8" />
          <span className="text-xl">cuan</span>
        </Link>
        <nav
          aria-label={t('Website navigation')}
          className="flex items-center gap-2 sm:gap-4"
        >
          <a
            href="#how-it-works"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground sm:inline-flex"
          >
            {t('Cara kerja')}
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
        <section
          className="relative mx-auto max-w-6xl px-6 pb-20 pt-12 text-center sm:px-8 sm:pb-28 sm:pt-20"
          aria-labelledby="landing-title"
        >
          <div className="mx-auto max-w-3xl">
            <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-border/60 bg-muted/30 px-3.5 py-1 text-xs font-medium text-muted-foreground shadow-2xs">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" aria-hidden="true" />
              <span>{t('AI Agent for Marketers')}</span>
            </div>
            <h1
              id="landing-title"
              className="text-4xl font-bold tracking-tight text-[#1B1616] dark:text-foreground sm:text-6xl md:text-7xl lg:text-[84px] lg:leading-[1.05]"
            >
              {t('Chat. Setujui. Beres.')}
            </h1>
            <p className="mt-5 text-lg font-normal text-[#5A5252] dark:text-muted-foreground sm:text-2xl sm:leading-relaxed">
              {t('Kelola bisnis tanpa pusing urusan teknis.')}
            </p>
            <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
              <Link to="/login" className="action-primary min-h-12 px-7 text-base font-medium shadow-xs">
                {t('Get started')}
                <ArrowRight className="h-4 w-4" />
              </Link>
              <a
                href={repository}
                target="_blank"
                rel="noreferrer"
                className="action-secondary min-h-12 px-6 text-base font-medium"
              >
                GitHub
                <ChevronRight className="h-4 w-4" />
              </a>
            </div>
          </div>

          {/* Centered 3-Device Mockup Showcase with Clean Natural Depth */}
          <div className="mx-auto mt-14 max-w-5xl sm:mt-18">
            <div className="relative">
              <img
                src="/landing/cuan.webp"
                alt={t('Cuan multi-device workspace on Laptop, Tablet, and Mobile')}
                width={2000}
                height={1088}
                className="h-auto w-full object-contain [filter:drop-shadow(0_20px_40px_rgba(0,0,0,0.07))_drop-shadow(0_4px_12px_rgba(0,0,0,0.03))]"
                loading="eager"
              />
            </div>
          </div>

          {/* Trust Highlights */}
          <div className="mx-auto mt-12 flex flex-wrap items-center justify-center gap-x-8 gap-y-3 text-xs text-muted-foreground">
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

        <section
          id="how-it-works"
          className="scroll-mt-6 border-y border-border/60 bg-muted/20 px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="how-title"
        >
          <div className="mx-auto max-w-6xl">
            <h2
              id="how-title"
              className="text-2xl font-semibold tracking-tight sm:text-3xl"
            >
              {t('How Cuan works')}
            </h2>
            <p className="mt-3 max-w-2xl text-sm leading-relaxed text-muted-foreground">
              {t(
                'You don’t need to organize tasks or choose an agent to begin. Start with what you want to achieve.',
              )}
            </p>
            <ol className="mt-10 grid gap-8 md:grid-cols-3">
              {[
                {
                  step: '01',
                  name: 'Chat',
                  title: 'Describe what you need in plain words',
                  text: 'Start in Chat from your phone or desktop. Share a goal, paste a reference, or upload a script. Cuan reviews your project and context automatically.',
                },
                {
                  step: '02',
                  name: 'Setujui',
                  title: 'Review and approve a clear plan',
                  text: 'Before touching anything, Cuan presents a structured Approval Card outlining the exact scope and verification criteria. One tap to approve.',
                },
                {
                  step: '03',
                  name: 'Beres',
                  title: 'Verified outcomes delivered to your server',
                  text: 'Autonomous agents execute the work, run automated tests, inspect real visual output, and report back when everything is 100% verified and live.',
                },
              ].map((item) => (
                <li key={item.step} className="rounded-xl border border-border/50 bg-card p-6 shadow-2xs">
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs font-semibold text-muted-foreground tabular-nums">
                      {item.step}
                    </span>
                    <span className="rounded-full bg-muted/60 px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                      {t(item.name)}
                    </span>
                  </div>
                  <h3 className="mt-4 text-base font-semibold leading-snug">
                    {t(item.title)}
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {t(item.text)}
                  </p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section
          className="mx-auto max-w-6xl px-6 py-16 sm:px-8 sm:py-24"
          aria-labelledby="control-title"
        >
          <div className="max-w-2xl">
            <h2
              id="control-title"
              className="text-2xl font-semibold tracking-tight sm:text-3xl"
            >
              {t('Stay in control, without staying on top of everything')}
            </h2>
            <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
              {t(
                'Your goals and decisions guide the work. Cuan handles the coordination and shows you what actually happened.',
              )}
            </p>
          </div>
          <div className="mt-10 grid gap-x-8 gap-y-8 sm:grid-cols-2 lg:grid-cols-3">
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
        </section>
        <section className="border-t border-border px-5 py-12 sm:px-8">
          <div className="mx-auto flex max-w-6xl flex-col items-start justify-between gap-5 sm:flex-row sm:items-center">
            <div>
              <h2 className="text-2xl font-semibold tracking-tight">
                {t('What would you like to get done?')}
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                {t('Start with the goal. Let Cuan organize the next step.')}
              </p>
            </div>
            <Link to="/login" className="action-primary min-h-12 px-5">
              {t('Get started')}
              <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
        </section>
      </main>
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
