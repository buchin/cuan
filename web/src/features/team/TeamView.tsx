import {
  Users,
  Crown,
  Bot,
  Cpu,
  ShieldCheck,
  CheckCircle2,
} from 'lucide-react'
import { cn } from '../../lib/utils'

interface TeamMember {
  id: string
  name: string
  handle: string
  role: 'product_owner' | 'scrum_master' | 'dev_team'
  roleTitle: string
  type: 'human' | 'agent'
  avatar: string
  responsibilities: string[]
  status: string
  detail: string
}

const TEAM_MEMBERS: TeamMember[] = [
  {
    id: 'po-buchin',
    name: 'Buchin',
    handle: '@buchin',
    role: 'product_owner',
    roleTitle: 'Product Owner (PO)',
    type: 'human',
    avatar: 'BU',
    responsibilities: [
      'Menetapkan visi, outcome, dan prioritas backlog',
      'Sprint Planning & 1-Click Approval di Inbox',
      'Sprint Review & evaluasi hasil produksi akhir',
    ],
    status: 'Active Owner',
    detail: 'Pengambil keputusan tunggal atas arah produk dan kepuasan Definition of Done.',
  },
  {
    id: 'sm-agent',
    name: 'Agent Manager (Scrum Master)',
    handle: '@sm',
    role: 'scrum_master',
    roleTitle: 'Scrum Master (SM Agent)',
    type: 'agent',
    avatar: 'SM',
    responsibilities: [
      'Autonomous triage pesan Inbox secara terstruktur',
      'Merumuskan Problem Statement & checklist Definition of Done (DoD)',
      'Memilih & mendelegasikan tugas ke worker agent yang memiliki kuota longgar',
    ],
    status: 'Online (Autonomous)',
    detail: 'Menjaga alur kerja sprint tanpa cognitive overload bagi Product Owner.',
  },
  {
    id: 'worker-flash',
    name: 'Antigravity Flash Worker',
    handle: '@pin',
    role: 'dev_team',
    roleTitle: 'Development Team (Worker)',
    type: 'agent',
    avatar: 'FL',
    responsibilities: [
      'Menulis kode modular & refactoring aman',
      'Targeted unit test & verifikasi zero regression',
      'Menerbitkan commit bertanda author Buchin & push main',
    ],
    status: 'Running Tasks',
    detail: 'Eksekutor teknis berkecepatan tinggi dengan kuota token optimal.',
  },
  {
    id: 'worker-pro',
    name: 'Antigravity Pro Architect',
    handle: '@pin2',
    role: 'dev_team',
    roleTitle: 'Development Team (Architect)',
    type: 'agent',
    avatar: 'PR',
    responsibilities: [
      'Deep architectural review & migration safety',
      'Handoff continuation saat worker primer mencapai limit',
      'Penyusunan audit pack & dokumentasi D2 TALA',
    ],
    status: 'Standby / Handoff Active',
    detail: 'Spesialis reasoning mendalam untuk task kompleks dan transisi handoff otonom.',
  },
]

export function TeamView({ selectedRole = 'all' }: { selectedRole?: string }) {
  const filtered = selectedRole === 'all'
    ? TEAM_MEMBERS
    : TEAM_MEMBERS.filter((m) => m.role === selectedRole)

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-foreground flex items-center gap-2">
          <Users className="h-5 w-5 text-primary" />
          <span>Team Directory & Agentic Scrum Hierarchy</span>
        </h1>
        <p className="text-xs text-muted-foreground mt-0.5">
          Struktur organisasi 3-Tier Agentic Scrum: Product Owner (manusia), Scrum Master (AI Manager), dan Development Team (Worker Agents).
        </p>
      </div>

      {/* Hierarchy Card */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* PO Card */}
        <div className="p-4 border border-border bg-card rounded-lg flex flex-col justify-between shadow-xs">
          <div>
            <div className="flex items-center gap-2 text-amber-500 mb-2">
              <Crown className="h-4 w-4" />
              <span className="text-xs font-bold uppercase tracking-wider font-mono">Product Owner</span>
            </div>
            <p className="text-xs text-muted-foreground leading-relaxed">
              Manusia memegang kendali atas apa yang dibangun dan menyetujui hasil sprint. Bebas dari kerepotan kanban mikro.
            </p>
          </div>
          <div className="mt-3 pt-3 border-t border-border flex items-center justify-between text-[11px] font-mono">
            <span className="text-foreground font-semibold">Buchin</span>
            <span className="text-amber-600 dark:text-amber-400 font-bold">1 Human</span>
          </div>
        </div>

        {/* SM Card */}
        <div className="p-4 border border-border bg-card rounded-lg flex flex-col justify-between shadow-xs">
          <div>
            <div className="flex items-center gap-2 text-indigo-500 mb-2">
              <Bot className="h-4 w-4" />
              <span className="text-xs font-bold uppercase tracking-wider font-mono">Scrum Master</span>
            </div>
            <p className="text-xs text-muted-foreground leading-relaxed">
              AI Manager otonom yang menyaring pesan Inbox, mendistilasi checklist DoD, dan menugaskan worker agent.
            </p>
          </div>
          <div className="mt-3 pt-3 border-t border-border flex items-center justify-between text-[11px] font-mono">
            <span className="text-foreground font-semibold">@sm Agent</span>
            <span className="text-indigo-600 dark:text-indigo-400 font-bold">Autonomous</span>
          </div>
        </div>

        {/* Dev Team Card */}
        <div className="p-4 border border-border bg-card rounded-lg flex flex-col justify-between shadow-xs">
          <div>
            <div className="flex items-center gap-2 text-emerald-500 mb-2">
              <Cpu className="h-4 w-4" />
              <span className="text-xs font-bold uppercase tracking-wider font-mono">Dev Team (Workers)</span>
            </div>
            <p className="text-xs text-muted-foreground leading-relaxed">
              Pool agent Antigravity yang mengeksekusi kode, menulis test terarah, dan handoff otomatis jika kuota habis.
            </p>
          </div>
          <div className="mt-3 pt-3 border-t border-border flex items-center justify-between text-[11px] font-mono">
            <span className="text-foreground font-semibold">Antigravity Pool</span>
            <span className="text-emerald-600 dark:text-emerald-400 font-bold">Multi-Agent</span>
          </div>
        </div>
      </div>

      {/* Team Member Cards */}
      <div className="space-y-3">
        <h2 className="text-xs font-bold uppercase tracking-wider text-muted-foreground font-mono">
          Anggota Tim ({filtered.length})
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filtered.map((member) => (
            <div
              key={member.id}
              className="border border-border bg-card p-5 rounded-lg shadow-xs flex flex-col justify-between space-y-4"
            >
              <div>
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div
                      className={cn(
                        'h-10 w-10 rounded-full flex items-center justify-center font-mono font-bold text-xs',
                        member.type === 'human'
                          ? 'bg-amber-500/15 text-amber-600 dark:text-amber-400 border border-amber-500/30'
                          : member.role === 'scrum_master'
                          ? 'bg-indigo-500/15 text-indigo-600 dark:text-indigo-400 border border-indigo-500/30'
                          : 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30'
                      )}
                    >
                      {member.avatar}
                    </div>
                    <div>
                      <h3 className="font-bold text-sm text-foreground flex items-center gap-2">
                        {member.name}
                        <span className="text-[10px] text-muted-foreground font-mono">
                          {member.handle}
                        </span>
                      </h3>
                      <p className="text-xs text-primary font-medium">{member.roleTitle}</p>
                    </div>
                  </div>

                  <span className="text-[9px] font-mono px-2 py-0.5 rounded bg-muted text-muted-foreground shrink-0">
                    {member.status}
                  </span>
                </div>

                <p className="text-xs text-muted-foreground mt-3 leading-relaxed">
                  {member.detail}
                </p>

                {/* Responsibilities */}
                <div className="mt-3 pt-3 border-t border-border space-y-1.5">
                  <div className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold">
                    Tanggung Jawab Utama
                  </div>
                  <ul className="space-y-1">
                    {member.responsibilities.map((r, i) => (
                      <li key={i} className="text-[11px] text-foreground/80 flex items-start gap-1.5">
                        <CheckCircle2 className="h-3 w-3 text-emerald-500 shrink-0 mt-0.5" />
                        <span>{r}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>

              <div className="pt-3 border-t border-border flex items-center justify-between text-[10px] text-muted-foreground font-mono">
                <span>Tipe: {member.type.toUpperCase()}</span>
                <span className="flex items-center gap-1">
                  <ShieldCheck className="h-3 w-3 text-emerald-500" />
                  Verified
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export function TeamSubPanel({
  selectedRole,
  onSelectRole,
}: {
  selectedRole: string
  onSelectRole: (role: string) => void
}) {
  const ROLES = [
    { id: 'all', label: 'All Members', count: 4 },
    { id: 'product_owner', label: 'Product Owner', count: 1 },
    { id: 'scrum_master', label: 'Scrum Master', count: 1 },
    { id: 'dev_team', label: 'Development Team', count: 2 },
  ]

  return (
    <div className="flex flex-col h-full bg-sidebar/50">
      <div className="p-3 border-b border-border">
        <span className="text-xs font-semibold text-foreground">
          Peran Tim Scrum
        </span>
      </div>
      <div className="p-2 space-y-1">
        {ROLES.map((r) => (
          <button
            key={r.id}
            onClick={() => onSelectRole(r.id)}
            className={cn(
              'w-full px-3 py-2 text-xs text-left rounded-md flex items-center justify-between transition-colors',
              selectedRole === r.id
                ? 'bg-accent text-accent-foreground font-semibold'
                : 'text-muted-foreground hover:text-foreground hover:bg-accent/40'
            )}
          >
            <span>{r.label}</span>
            <span className="text-[10px] font-mono opacity-70">{r.count}</span>
          </button>
        ))}
      </div>
    </div>
  )
}
