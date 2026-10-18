import { useState } from 'react'
import {
  Bot,
  Cpu,
  Terminal,
  Zap,
  Plus,
  ShieldCheck,
  X,
  Sparkles,
} from 'lucide-react'
import { cn } from '../../lib/utils'

export interface AgentAccount {
  id: string
  name: string
  role: string
  status: 'idle' | 'running' | 'offline'
  model: string
  tokenQuota: {
    used: number
    limit: number
    unit: string
  }
  capabilities: string[]
  isPrimary?: boolean
}

export const INITIAL_AGENTS: AgentAccount[] = [
  {
    id: 'antigravity-flash',
    name: 'Antigravity Flash Worker',
    role: 'Autonomous Sprint Worker',
    status: 'idle',
    model: 'Gemini 3.8 Flash (High-Speed)',
    tokenQuota: {
      used: 124500,
      limit: 1000000,
      unit: 'tokens',
    },
    capabilities: ['Fast Triage', 'TypeScript', 'React', 'Go Backend', 'Git Commits'],
    isPrimary: true,
  },
  {
    id: 'antigravity-pro',
    name: 'Antigravity Pro Architect',
    role: 'Deep Reasoning & Refactor Specialist',
    status: 'idle',
    model: 'Gemini Pro / Claude Sonnet',
    tokenQuota: {
      used: 340000,
      limit: 800000,
      unit: 'tokens',
    },
    capabilities: ['Architecture Review', 'Complex Migrations', 'Security Audits', 'Zero Regression'],
  },
  {
    id: 'agent-general-runner',
    name: 'General Task Runner',
    role: 'Headless Task Implementer',
    status: 'idle',
    model: 'In-Process Go Runner & Shell Driver',
    tokenQuota: {
      used: 45000,
      limit: 500000,
      unit: 'tokens',
    },
    capabilities: ['Go', 'TypeScript', 'Docker', 'PostgreSQL', 'Caddy'],
  },
]

export function AgentsView({
  agents = INITIAL_AGENTS,
  onAddAgent,
}: {
  agents?: AgentAccount[]
  onAddAgent?: (agent: AgentAccount) => void
}) {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [newModel, setNewModel] = useState('Gemini 3.8 Flash')
  const [newRole, setNewRole] = useState('Sprint Worker Agent')

  const handleCreateAgent = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return

    const newAgent: AgentAccount = {
      id: `agent-${Date.now()}`,
      name: newName.trim(),
      role: newRole.trim(),
      status: 'idle',
      model: newModel,
      tokenQuota: {
        used: 0,
        limit: 1000000,
        unit: 'tokens',
      },
      capabilities: ['Code Implementation', 'Automated Testing', 'Git Commits'],
    }

    if (onAddAgent) onAddAgent(newAgent)
    setIsModalOpen(false)
    setNewName('')
  }

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-foreground flex items-center gap-2">
            <Bot className="h-5 w-5 text-primary" />
            <span>AI Agents Hub (Antigravity Pool)</span>
          </h1>
          <p className="text-xs text-muted-foreground mt-0.5">
            Daftar akun worker agent terdaftar beserta visualisasi kuota token untuk eksekusi sprint backlog.
          </p>
        </div>

        <button
          onClick={() => setIsModalOpen(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold bg-primary text-primary-foreground hover:bg-primary/90 transition-colors rounded-md shadow-xs"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>Tambah Akun Agent</span>
        </button>
      </div>

      {/* Agents Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {agents.map((agent) => {
          const quotaPercent = Math.min(
            100,
            Math.round((agent.tokenQuota.used / agent.tokenQuota.limit) * 100)
          )
          const remainingPercent = 100 - quotaPercent

          return (
            <div
              key={agent.id}
              className="border border-border bg-card p-5 space-y-4 rounded-lg shadow-sm flex flex-col justify-between"
            >
              <div>
                <div className="flex items-center justify-between mb-3">
                  <div className="h-8 w-8 bg-primary/10 border border-primary/20 flex items-center justify-center text-primary rounded-md">
                    <Cpu className="h-4 w-4" />
                  </div>
                  <div className="flex items-center gap-2">
                    {agent.isPrimary && (
                      <span className="inline-flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
                        <Sparkles className="h-2.5 w-2.5" />
                        Preferred
                      </span>
                    )}
                    <span className="flex items-center gap-1 text-[10px] uppercase font-bold text-emerald-600 dark:text-emerald-400">
                      <div className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                      {agent.status}
                    </span>
                  </div>
                </div>

                <h3 className="font-bold text-sm text-foreground">{agent.name}</h3>
                <p className="text-[11px] text-muted-foreground mt-0.5">{agent.role}</p>

                {/* Engine / Model */}
                <div className="mt-3 pt-3 border-t border-border space-y-1">
                  <div className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold">
                    Engine / Model
                  </div>
                  <p className="text-[11px] text-foreground font-mono">{agent.model}</p>
                </div>

                {/* Token Quota Progress */}
                <div className="mt-3 space-y-1.5">
                  <div className="flex items-center justify-between text-[10px]">
                    <span className="text-muted-foreground font-semibold flex items-center gap-1">
                      <Zap className="h-3 w-3 text-amber-500" />
                      Kapasitas Kuota Token
                    </span>
                    <span className="font-mono text-foreground font-semibold">
                      {remainingPercent}% Sisa
                    </span>
                  </div>
                  <div className="h-2 w-full bg-muted rounded-full overflow-hidden">
                    <div
                      className={cn(
                        'h-full transition-all duration-300',
                        quotaPercent > 80
                          ? 'bg-rose-500'
                          : quotaPercent > 50
                          ? 'bg-amber-500'
                          : 'bg-emerald-500'
                      )}
                      style={{ width: `${quotaPercent}%` }}
                    />
                  </div>
                  <div className="flex items-center justify-between text-[9px] text-muted-foreground font-mono">
                    <span>Terpakai: {(agent.tokenQuota.used / 1000).toFixed(0)}k</span>
                    <span>Limit: {(agent.tokenQuota.limit / 1000).toFixed(0)}k</span>
                  </div>
                </div>

                {/* Skills */}
                <div className="mt-3 space-y-1.5">
                  <div className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold">
                    Capabilities
                  </div>
                  <div className="flex flex-wrap gap-1">
                    {agent.capabilities.map((cap) => (
                      <span
                        key={cap}
                        className="text-[9px] px-1.5 py-0.5 bg-muted text-muted-foreground rounded font-mono"
                      >
                        {cap}
                      </span>
                    ))}
                  </div>
                </div>
              </div>

              <div className="pt-3 border-t border-border flex items-center justify-between text-[10px] text-muted-foreground">
                <span className="flex items-center gap-1">
                  <ShieldCheck className="h-3 w-3 text-emerald-500" />
                  Sandbox Isolated
                </span>
                <span className="font-mono">ID: {agent.id.slice(0, 12)}</span>
              </div>
            </div>
          )
        })}
      </div>

      {/* Guide Card */}
      <div className="p-5 border border-border bg-card/60 rounded-lg">
        <h3 className="text-xs font-bold text-foreground mb-2 flex items-center gap-2">
          <Terminal className="h-4 w-4 text-primary" />
          <span>Mekanisme Delegasi Worker Agent</span>
        </h3>
        <p className="text-xs text-muted-foreground leading-relaxed">
          Dalam arsitektur <strong>Agentic Scrum</strong> Cuan, Scrum Master (AI Manager) secara otonom
          memilih worker agent dari pool berdasarkan sisa kuota token dan kompleksitas task yang dirumuskan.
          Worker yang dipilih mengeksekusi kode di sandbox git terisolasi, menjalankan unit test terarah,
          dan memposting bukti Definition of Done (DoD) langsung ke thread percakapan.
        </p>
      </div>

      {/* Add Agent Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
          <div className="w-full max-w-md bg-card text-card-foreground border border-border rounded-lg shadow-2xl p-6">
            <div className="flex items-center justify-between pb-4 border-b border-border mb-4">
              <div className="flex items-center gap-2">
                <Bot className="h-5 w-5 text-primary" />
                <h3 className="text-sm font-semibold text-foreground">
                  Tambah Akun Worker Agent
                </h3>
              </div>
              <button
                onClick={() => setIsModalOpen(false)}
                className="text-muted-foreground hover:text-foreground p-1 rounded"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleCreateAgent} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-foreground mb-1.5">
                  Nama Worker Agent
                </label>
                <input
                  type="text"
                  required
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="Contoh: Antigravity Subagent Beta"
                  className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground focus:outline-hidden focus:ring-1 focus:ring-ring"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-foreground mb-1.5">
                  Peran / Spesialisasi
                </label>
                <input
                  type="text"
                  required
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value)}
                  placeholder="Contoh: Frontend Bugfix Specialist"
                  className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground focus:outline-hidden focus:ring-1 focus:ring-ring"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-foreground mb-1.5">
                  Model / Engine
                </label>
                <select
                  value={newModel}
                  onChange={(e) => setNewModel(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground focus:outline-hidden focus:ring-1 focus:ring-ring"
                >
                  <option value="Gemini 3.8 Flash (High-Speed)">Gemini 3.8 Flash (High-Speed)</option>
                  <option value="Gemini Pro / Claude Sonnet">Gemini Pro / Claude Sonnet</option>
                  <option value="In-Process Go Runner">In-Process Go Runner</option>
                </select>
              </div>

              <div className="pt-3 border-t border-border flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="px-3 py-2 text-xs text-muted-foreground hover:text-foreground"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 text-xs font-semibold bg-primary text-primary-foreground hover:bg-primary/90 rounded-md"
                >
                  Simpan Agent
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export function AgentsSubPanel({
  agents = INITIAL_AGENTS,
  selectedAgentId,
  onSelectAgent,
}: {
  agents?: AgentAccount[]
  selectedAgentId?: string
  onSelectAgent?: (agent: AgentAccount) => void
}) {
  return (
    <div className="flex flex-col h-full bg-sidebar/50">
      <div className="p-3 border-b border-border">
        <span className="text-xs font-semibold text-foreground">
          Worker Pool ({agents.length})
        </span>
      </div>
      <div className="flex-1 overflow-y-auto divide-y divide-border/50">
        {agents.map((agent) => (
          <button
            key={agent.id}
            onClick={() => onSelectAgent && onSelectAgent(agent)}
            className={cn(
              'w-full p-3 text-left transition-colors flex flex-col gap-1',
              selectedAgentId === agent.id ? 'bg-accent/80' : 'hover:bg-accent/40'
            )}
          >
            <div className="flex items-center justify-between">
              <span className="font-semibold text-xs text-foreground truncate">
                {agent.name}
              </span>
              <span className="h-2 w-2 rounded-full bg-emerald-500" />
            </div>
            <p className="text-[11px] text-muted-foreground truncate">{agent.role}</p>
            <div className="flex items-center justify-between text-[10px] text-muted-foreground font-mono mt-1">
              <span>{agent.model.split(' ')[0]}</span>
              <span className="text-emerald-600 dark:text-emerald-400">
                {Math.round(((agent.tokenQuota.limit - agent.tokenQuota.used) / agent.tokenQuota.limit) * 100)}% kuota
              </span>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}
