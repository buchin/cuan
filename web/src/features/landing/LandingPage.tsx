import { useState } from 'react'
import { Link } from 'react-router-dom'
import {
  ArrowRight,
  Check,
  ChevronRight,
  GitBranch,
  MessageSquare,
  Bell,
  Mail,
  ShieldCheck,
  CheckCircle2,
  Zap,
  Clock,
  Sparkles,
  Volume2,
  Activity,
  Cpu,
  Radio,
  Compass,
  Layers,
  Users,
  Calendar,
  Terminal,
  Workflow,
} from 'lucide-react'
import { CuanMark } from '../../components/brand/CuanMark'
import { useLanguage } from '../../context/LanguageContext'

const repository = 'https://github.com/buchin/cuan'
export function LandingPage() {
  const { t } = useLanguage(),
    [step, setStep] = useState<'plan' | 'work' | 'result'>('plan')
  const demo = {
    plan: {
      label: 'A clear plan, before work starts.',
      text: 'I’ll review the project, propose the changes and ask for your approval.',
      detail: 'You choose the outcome and approve the scope.',
    },
    work: {
      label: 'The work keeps moving.',
      text: 'The approved changes are underway. I’ll keep you updated and flag anything that needs a decision.',
      detail: 'Check progress when you want. Get notified when you’re needed.',
    },
    result: {
      label: 'A result you can check.',
      text: 'The changes are ready. Here’s what changed, what was verified and what needs your review.',
      detail: 'Review the result and decide what happens next.',
    },
  }[step]
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
          className="flex items-center gap-1 sm:gap-4"
        >
          <a
            href="#how-it-works"
            className="hidden min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground sm:inline-flex"
          >
            {t('How it works')}
          </a>
          <a
            href={repository}
            className="inline-flex min-h-11 items-center px-2 text-sm text-muted-foreground hover:text-foreground"
          >
            GitHub
          </a>
          <Link to="/login" className="action-secondary">
            {t('Sign in')}
          </Link>
        </nav>
      </header>
      <main>
        <section
          className="mx-auto grid max-w-6xl gap-10 px-5 pb-16 pt-10 sm:px-8 sm:pt-16 lg:grid-cols-[1.1fr_1fr] lg:items-center lg:gap-12 lg:pb-24 lg:pt-20"
          aria-labelledby="landing-title"
        >
          <div>
            <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-border/60 bg-muted/40 px-3.5 py-1 text-xs font-medium text-muted-foreground shadow-2xs">
              <Sparkles className="h-3.5 w-3.5 text-primary" />
              <span>{t('Calm & Layered UI System · Things 3 & Bear Notes Inspired')}</span>
            </div>
            <p className="mb-4 text-sm font-medium text-muted-foreground">
              {t('Your always-on project manager.')}
            </p>
            <h1
              id="landing-title"
              className="max-w-xl text-4xl font-semibold leading-[1.08] tracking-tight sm:text-5xl lg:text-6xl"
            >
              {t('Rest easy.')}
              <br />
              <span className="text-primary">
                {t('Cuan gets things done.')}
              </span>
            </h1>
            <p className="mt-6 max-w-lg text-base leading-relaxed text-muted-foreground sm:text-lg">
              {t(
                'Let Cuan handle your projects, so you can rest easy while the work gets done.',
              )}
            </p>
            <p className="mt-4 max-w-lg text-sm leading-relaxed text-muted-foreground">
              {t(
                'Set the goal. Approve the plan. Cuan takes care of the work and keeps you updated.',
              )}
            </p>
            <div className="mt-7 flex flex-wrap gap-3">
              <Link to="/login" className="action-primary min-h-12 px-5">
                {t('Get started')}
                <ArrowRight className="h-4 w-4" />
              </Link>
              <a
                href="#how-it-works"
                className="action-secondary min-h-12 px-5"
              >
                {t('See how it works')}
                <ChevronRight className="h-4 w-4" />
              </a>
            </div>
            <p className="mt-5 text-xs leading-relaxed text-muted-foreground">
              {t('Open source. Self-hosted. Connected to your AI accounts.')}
            </p>
          </div>
          <div className="min-w-0 overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
            <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
              <div className="flex min-w-0 items-center gap-2 text-sm font-medium">
                <CuanMark decorative className="h-5 w-5" />
                {t('A website update')}
              </div>
              <span className="text-xs text-muted-foreground">
                {t('Example workflow')}
              </span>
            </div>
            <div className="space-y-4 p-4 sm:p-5">
              <p className="ml-8 rounded-xl bg-muted p-4 text-sm leading-relaxed">
                {t('Help me improve our website’s mobile experience.')}
              </p>
              <div
                className="flex gap-2 border-b border-border"
                aria-label={t('Example stages')}
              >
                {(
                  [
                    { id: 'plan', label: 'Plan' },
                    { id: 'work', label: 'Work' },
                    { id: 'result', label: 'Result' },
                  ] as const
                ).map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    aria-pressed={step === item.id}
                    onClick={() => setStep(item.id)}
                    className={
                      'min-h-11 border-b-2 px-3 text-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ' +
                      (step === item.id
                        ? 'border-primary font-medium text-primary'
                        : 'border-transparent text-muted-foreground')
                    }
                  >
                    {t(item.label)}
                  </button>
                ))}
              </div>
              <div className="min-h-48 space-y-4" aria-live="polite">
                <div className="mr-4 rounded-xl border border-border p-4 text-sm leading-relaxed">
                  <p className="mb-2 font-medium">{t(demo.label)}</p>
                  <p className="text-muted-foreground">{t(demo.text)}</p>
                </div>
                <p className="flex items-start gap-2 text-xs leading-relaxed text-muted-foreground">
                  <Check className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                  {t(demo.detail)}
                </p>
              </div>
            </div>
          </div>
        </section>
        <section
          id="how-it-works"
          className="scroll-mt-6 border-y border-border bg-muted/30 px-5 py-14 sm:px-8 sm:py-20"
          aria-labelledby="how-title"
        >
          <div className="mx-auto max-w-6xl">
            <h2
              id="how-title"
              className="text-2xl font-semibold tracking-tight sm:text-3xl"
            >
              {t('A goal in. Work moving forward.')}
            </h2>
            <p className="mt-3 max-w-2xl text-sm leading-relaxed text-muted-foreground">
              {t(
                'You don’t need to organize tasks or choose an agent to begin. Start with what you want to achieve.',
              )}
            </p>
            <ol className="mt-8 grid gap-7 md:grid-cols-3">
              {[
                {
                  title: 'Tell Cuan the goal.',
                  text: 'Describe the outcome in Chat. Add screenshots, documents or references to make it clear.',
                },
                {
                  title: 'Agree on the plan.',
                  text: 'Cuan asks when something is unclear and proposes the scope. You approve before execution starts.',
                },
                {
                  title: 'Get on with your day.',
                  text: 'Cuan delegates the work, reports progress and brings the result back to the conversation.',
                },
              ].map((item, index) => (
                <li key={item.title}>
                  <span className="text-xs font-medium text-muted-foreground">
                    0{index + 1}
                  </span>
                  <h3 className="mt-3 text-lg font-semibold">
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
          className="mx-auto max-w-6xl px-5 py-14 sm:px-8 sm:py-20"
          aria-labelledby="control-title"
        >
          <div className="max-w-2xl">
            <h2
              id="control-title"
              className="text-2xl font-semibold tracking-tight sm:text-3xl"
            >
              {t('Stay in control, without staying on top of everything.')}
            </h2>
            <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
              {t(
                'Your goals and decisions guide the work. Cuan handles the coordination and shows you what actually happened.',
              )}
            </p>
          </div>
          <div className="mt-8 grid gap-x-8 gap-y-7 sm:grid-cols-2">
            {[
              {
                Icon: CheckCircle2,
                title: 'Smart Auto-Deploy Gate, Dual-Channel Envelope & 1-Click Actionable Draft.',
                text: 'Intelligent conditional deployment gate distinguishing code-fixes from investigations, dual-channel output envelopes with zero technical jargon, and hybrid actionable task cards with 1-click visitor draft insertion.',
              },
              {
                Icon: Users,
                title: '5-Pillar Navigation: Chat, Tasks, Projects, Contacts, User Profile.',
                text: 'Ergonomic bottom navigation bringing primary workflows into instant reach, accompanied by header quick access [⋮] for Agents, Schedules, and Settings.',
              },
              {
                Icon: Terminal,
                title: 'Pure Live PTY Terminal, Deep Payload Formatter & Mobile Safe-Area Ergonomics.',
                text: 'Pure full-viewport xterm.js terminal with deep runner payload parsing (human-readable action cards), graceful stream resilience on decoupled runners, calm Things 3 status indicator, mid-run steering bridge, and comfortable mobile safe-area touch ergonomics.',
              },
              {
                Icon: Zap,
                title: 'Autonomous Scrum Engine with AGENTS.md Injection.',
                text: 'Proactive event listener evaluates Definition of Done criteria, executes adaptive self-healing retries, and injects repository rules (§1–§13) directly into AI system prompts.',
              },
              {
                Icon: Calendar,
                title: 'Consolidated Schedules in Settings.',
                text: 'All routine automation and background cron schedules organized cleanly in /settings/schedules, eliminating sidebar clutter and mental friction.',
              },
              {
                Icon: MessageSquare,
                title: 'Borderless Chat Canvas & Floating Soft Surface Composer.',
                text: 'Zero 1px wireframe cuts on header and composer, zero noisy memory chips in chat stream, and an elevated soft surface input tint inspired by Bear Notes.',
              },
              {
                Icon: Mail,
                title: 'Apple Mail & Bear Inset List in Inbox.',
                text: '3-tier conversation list with sender and tabular-nums time, Geist typography subject with micro-indicator StatusDot, 2-line snippet in muted zinc-400, hairline dividers, and zero screen-wide quick bar clutter.',
              },
              {
                Icon: MessageSquare,
                title: 'Calm Low-Contrast Chat Bubbles & Quiet Dividers.',
                text: 'Monochromatic micro-tint chat bubbles, quiet muted date dividers with zero solid white pill glare, and borderless inline status and task cards.',
              },
              {
                Icon: Sparkles,
                title: 'SlideDrawer Progressive Disclosure.',
                text: 'Tap task references directly inside chat bubbles to slide open rich task inspection on the right without interrupting or obscuring the conversation stream.',
              },
              {
                Icon: Workflow,
                title: 'Micro-modular task orchestration & DoD evaluator.',
                text: 'Zero-dependency state machine engine (taskfsm) and automated markdown criteria evaluator (dodeval) enforce strict quality guardrails with lightning-fast execution (<10ms).',
              },
              {
                Icon: CheckCircle2,
                title: 'Things 3 Task Rows & Bear Notes Stream.',
                text: 'Inspired by Things 3 and Bear Notes. Pure task rows with contextual action buttons (1-click retry on failure, live terminal when running, zero redundant double actions), progressive disclosure drawers, and peaceful typography.',
              },
              {
                Icon: Compass,
                title: 'Clean Account vs Settings Separation.',
                text: 'Personal profile, appearance, and notifications cleanly isolated in /account with Apple-style grouped rows; workspace AI models, memory, and team config in /settings.',
              },
              {
                Icon: Bell,
                title: 'Know when you’re needed.',
                text: 'See approvals, blockers, completed work and new customer messages. Enable push and choose quiet hours.',
              },
              {
                Icon: Mail,
                title: 'Resend email alerts & idle chat transcripts.',
                text: 'Never lose a customer lead. Automatically receive responsive email digests via Resend when live chat inquiries are unread or idle (>5-10m).',
              },
              {
                Icon: ShieldCheck,
                title: 'Approval stays with you.',
                text: 'Review scope before work starts. Get a clear account of changes, checks and limitations when it finishes.',
              },
              {
                Icon: GitBranch,
                title: 'Your workspace. Your AI.',
                text: 'Self-host Cuan and connect your own supported AI accounts. Keep your projects and execution in your environment.',
              },
              {
                Icon: CheckCircle2,
                title: 'Autonomous quality loop.',
                text: 'Background quality evaluation validates changes automatically. If a gap is found, Cuan handles adaptive self-healing retries without disturbing you.',
              },
              {
                Icon: Zap,
                title: 'Isolated cuan-runner daemon & auto-undrain deployment.',
                text: 'Tasks are dispatched to a standalone worker daemon isolated from the web server. Deploy hooks automatically clean drain flags and restart workers with zero interruption to ongoing tasks.',
              },
              {
                Icon: Sparkles,
                title: 'De-cluttered chat & self-contained task cards.',
                text: 'Clean, distraction-free chat with self-contained task cards handling queued states, live pulse & duration timers, and seamless transitions to actionable completion cards.',
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
              {
                Icon: Cpu,
                title: 'Atomic execution engine & DoD evaluation.',
                text: 'Ultra-reliable state machine and automated Definition of Done evaluation run lightning-fast in isolated packages, guaranteeing zero timeout and robust task completion.',
              },
              {
                Icon: Radio,
                title: 'Real-time log streaming & interactive steering.',
                text: 'Live stdout/stderr streaming pipe brings instant transparency into active tasks, while interactive chat steering lets you guide the agent without restarting from scratch.',
              },
              {
                Icon: Clock,
                title: 'Zero-interruption worker & proactive chat.',
                text: 'Decoupled worker runs independently with progressive timeouts. Once completed, friendly zero-jargon updates and strategic next steps arrive in your chat.',
              },
              {
                Icon: Layers,
                title: 'Adaptive tool budgeting & deep project inspection.',
                text: 'Chat agents inspect codebases, repositories, and data without hitting premature turn quotas. Adaptive batching and loop guards prevent cut-off responses.',
              },
            ].map(({ Icon, title, text }) => (
              <div key={title} className="border-t border-border pt-5">
                <Icon className="mb-3 h-5 w-5 text-muted-foreground" />
                <h3 className="font-semibold">{t(title)}</h3>
                <p className="mt-2 max-w-lg text-sm leading-relaxed text-muted-foreground">
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
