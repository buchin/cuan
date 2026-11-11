import { Link } from 'react-router-dom'
import {
  ArrowRight,
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
  const { t } = useLanguage()
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
          className="relative mx-auto max-w-6xl px-5 pb-16 pt-10 text-center sm:px-8 sm:pb-24 sm:pt-16"
          aria-labelledby="landing-title"
        >
          {/* Subtle Ambient Radial Glow */}
          <div
            aria-hidden="true"
            className="pointer-events-none absolute left-1/2 top-4 h-[320px] w-[500px] -translate-x-1/2 rounded-full bg-amber-500/10 blur-[100px] -z-10 sm:h-[420px] sm:w-[700px]"
          />

          <div className="mx-auto max-w-3xl">
            <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-border/60 bg-muted/40 px-4 py-1.5 text-xs font-medium text-muted-foreground shadow-2xs backdrop-blur-xs">
              <Sparkles className="h-3.5 w-3.5 text-primary" />
              <span>{t('✨ AI Agent for Marketers')}</span>
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
            <div className="mt-8 flex flex-wrap items-center justify-center gap-3.5">
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
            <p className="mt-4 text-xs font-normal text-muted-foreground">
              {t('Open source · Self-hosted di VPS sendiri · Zero SaaS markup')}
            </p>
          </div>

          {/* Centered 3-Device Mockup Showcase with Natural Shadow */}
          <div className="mx-auto mt-12 max-w-5xl sm:mt-16">
            <div className="relative">
              <img
                src="/landing/cuan.webp"
                alt={t('Cuan multi-device workspace on Laptop, Tablet, and Mobile')}
                width={2000}
                height={1088}
                className="h-auto w-full object-contain [filter:drop-shadow(0_20px_40px_rgba(0,0,0,0.08))_drop-shadow(0_8px_16px_rgba(0,0,0,0.04))]"
                loading="eager"
              />
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
                Icon: Sparkles,
                title: '1-Tap Interactive Prompt Chips & Zero Clue Dependency Context.',
                text: 'Structured choice buttons (prompt_choices) eliminating conversational dead-ends and typing fatigue on mobile, paired with Fural-inspired layered project context and recent DoD gaps for instant autonomous task completion without asking users for technical clues.',
              },
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
                text: 'Inspired by Things 3 and Bear Notes. Pure task rows with zero loud buttons, glitch-free mobile render, zero nested cards, progressive disclosure drawers, and peaceful typography.',
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
                title: 'Detached worker process & zero-kill deploy.',
                text: 'Tasks run in isolated execution scopes decoupled from supervisor restarts. Deploys complete instantly without waiting, and ongoing tasks finish uninterrupted.',
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
