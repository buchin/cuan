import { useState, useEffect } from 'react'
import {
  Bot,
  Cpu,
  Terminal,
  Zap,
  Plus,
  ShieldCheck,
  X,
  Sparkles,
  KeyRound,
  CheckCircle2,
  RefreshCw,
  Copy,
} from 'lucide-react'
import { cn } from '../../lib/utils'
import { agentsApi } from '../../api/agents'

export interface AgentAccount {
  id: string
  name: string
  handle?: string
  provider?: string
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
  authModel?: string
}

export const INITIAL_AGENTS: AgentAccount[] = [
  {
    id: 'antigravity-flash',
    name: 'Antigravity Flash Worker',
    handle: '@pin',
    provider: 'antigravity',
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
    authModel: 'CLI Session (Fural)',
  },
  {
    id: 'antigravity-pro',
    name: 'Antigravity Pro Architect',
    handle: '@pin2',
    provider: 'antigravity',
    role: 'Deep Reasoning & Refactor Specialist',
    status: 'idle',
    model: 'Gemini Pro / Claude Sonnet',
    tokenQuota: {
      used: 340000,
      limit: 800000,
      unit: 'tokens',
    },
    capabilities: ['Architecture Review', 'Complex Migrations', 'Security Audits', 'Zero Regression'],
    authModel: 'CLI Session (Fural)',
  },
  {
    id: 'agent-general-runner',
    name: 'General Task Runner',
    handle: '@dev',
    provider: 'antigravity',
    role: 'Headless Task Implementer',
    status: 'idle',
    model: 'In-Process Go Runner & Shell Driver',
    tokenQuota: {
      used: 45000,
      limit: 500000,
      unit: 'tokens',
    },
    capabilities: ['Go', 'TypeScript', 'Docker', 'PostgreSQL', 'Caddy'],
    authModel: 'In-Process Runner',
  },
]

export function AgentsView({
  agents: propAgents = INITIAL_AGENTS,
  onAddAgent,
}: {
  agents?: AgentAccount[]
  onAddAgent?: (agent: AgentAccount) => void
}) {
  const [agentsList, setAgentsList] = useState<AgentAccount[]>(propAgents)
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [newHandle, setNewHandle] = useState('@dev')
  const [newProvider, setNewProvider] = useState('antigravity')
  const [newModel, setNewModel] = useState('Gemini 3.8 Flash (High-Speed)')
  const [newRole, setNewRole] = useState('Sprint Worker Agent')
  const [authType, setAuthType] = useState<'cli_login' | 'oauth'>('cli_login')
  const [deviceCode, setDeviceCode] = useState('CUAN-8892')
  const [isConnecting, setIsConnecting] = useState(false)
  const [copied, setCopied] = useState(false)

  // Fetch agents from backend DB on mount
  useEffect(() => {
    let isMounted = true
    agentsApi
      .list()
      .then((dbAgents) => {
        if (!isMounted || !dbAgents || dbAgents.length === 0) return
        const mapped: AgentAccount[] = dbAgents.map((a) => ({
          id: a.id,
          name: a.name,
          handle: a.name.includes('@') ? '@' + a.name.split('@')[1].replace(')', '') : '@' + a.id.replace('agent_', ''),
          provider: a.provider || 'antigravity',
          role: a.role || 'Sprint Worker Agent',
          status: 'idle',
          model: a.model_id || 'Gemini 3.8 Flash',
          tokenQuota: {
            used: 12000,
            limit: 1000000,
            unit: 'tokens',
          },
          capabilities: ['Code Implementation', 'Automated Testing', 'Git Commits', 'Interactive CLI Chat'],
          authModel: 'CLI Session (Fural)',
        }))

        // Merge with defaults avoiding duplicate IDs
        setAgentsList((prev) => {
          const ids = new Set(prev.map((p) => p.id))
          const toAdd = mapped.filter((m) => !ids.has(m.id))
          return [...prev, ...toAdd]
        })
      })
      .catch((err) => console.error('Failed to load agents from API:', err))

    return () => {
      isMounted = false
    }
  }, [])

  const handleOpenModal = () => {
    const randomCode = 'CUAN-' + Math.floor(1000 + Math.random() * 9000)
    setDeviceCode(randomCode)
    setIsModalOpen(true)
  }

  const handleCopyCommand = () => {
    const cmd = `cuan agent auth --handle ${newHandle} --runtime ${newProvider}`
    navigator.clipboard?.writeText(cmd)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleCreateAgent = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return

    setIsConnecting(true)
    try {
      const res = await agentsApi.connectCLI({
        name: newName.trim(),
        handle: newHandle.trim(),
        provider: newProvider,
        model_id: newModel,
        role: newRole.trim(),
        auth_type: authType,
        session_token: `cuan_cli_session_${Date.now()}`,
      })

      const newAgent: AgentAccount = {
        id: res.agent.id,
        name: res.agent.name,
        handle: res.handle || newHandle.trim(),
        provider: res.provider || newProvider,
        role: res.agent.role || newRole.trim(),
        status: 'idle',
        model: res.agent.model_id || newModel,
        tokenQuota: {
          used: 0,
          limit: 1000000,
          unit: 'tokens',
        },
        capabilities: ['Code Implementation', 'Automated Testing', 'Git Commits', 'Interactive CLI Chat'],
        authModel: 'CLI Session (Model Fural)',
      }

      setAgentsList((prev) => [newAgent, ...prev])
      if (onAddAgent) onAddAgent(newAgent)
      setIsModalOpen(false)
      setNewName('')
      setNewHandle('@dev')
    } catch (err) {
      console.error('Failed to connect agent via CLI:', err)
      const fallbackAgent: AgentAccount = {
        id: `agent-${Date.now()}`,
        name: `${newName.trim()} (${newHandle.trim()})`,
        handle: newHandle.trim(),
        provider: newProvider,
        role: newRole.trim(),
        status: 'idle',
        model: newModel,
        tokenQuota: {
          used: 0,
          limit: 1000000,
          unit: 'tokens',
        },
        capabilities: ['Code Implementation', 'Automated Testing', 'Git Commits'],
        authModel: 'CLI Session (Fallback)',
      }
      setAgentsList((prev) => [fallbackAgent, ...prev])
      if (onAddAgent) onAddAgent(fallbackAgent)
      setIsModalOpen(false)
    } finally {
      setIsConnecting(false)
    }
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
            Daftar akun worker agent terdaftar dengan otorisasi CLI (Model Fural — Tanpa API Key).
          </p>
        </div>

        <button
          onClick={handleOpenModal}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold bg-primary text-primary-foreground hover:bg-primary/90 transition-colors rounded-md shadow-xs cursor-pointer"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>Tambah Akun Agent (Model Fural)</span>
        </button>
      </div>

      {/* Agents Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {agentsList.map((agent) => {
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
                  <div className="flex items-center gap-1.5">
                    {agent.isPrimary && (
                      <span className="inline-flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
                        <Sparkles className="h-2.5 w-2.5" />
                        Primary
                      </span>
                    )}
                    <span className="flex items-center gap-1 text-[10px] font-mono font-bold px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                      <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
                      Connected
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-1.5">
                  <h3 className="font-bold text-sm text-foreground truncate">{agent.name}</h3>
                  {agent.handle && (
                    <span className="text-[10px] text-muted-foreground font-mono bg-muted px-1.5 py-0.2 rounded">
                      {agent.handle}
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-muted-foreground mt-0.5">{agent.role}</p>

                {/* Runtime & Engine */}
                <div className="mt-3 pt-3 border-t border-border space-y-1">
                  <div className="flex items-center justify-between text-[10px]">
                    <span className="text-muted-foreground uppercase tracking-wider font-semibold">
                      Runtime / Provider
                    </span>
                    <span className="font-mono text-primary font-bold">
                      {agent.provider === 'antigravity' ? 'Antigravity CLI (agy)' : agent.provider || 'CLI Session'}
                    </span>
                  </div>
                  <p className="text-[11px] text-foreground font-mono truncate">{agent.model}</p>
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
                  No API Key (Model Fural)
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

      {/* Add Agent Modal (Model Fural — No API Key) */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
          <div className="w-full max-w-lg bg-card text-card-foreground border border-border rounded-xl shadow-2xl p-6 max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between pb-4 border-b border-border mb-4">
              <div className="flex items-center gap-2">
                <div className="h-8 w-8 rounded-lg bg-primary/10 border border-primary/20 flex items-center justify-center text-primary">
                  <Bot className="h-5 w-5" />
                </div>
                <div>
                  <h3 className="text-sm font-bold text-foreground">
                    Tambah Akun Agent &amp; CLI Login
                  </h3>
                  <p className="text-[11px] text-muted-foreground">
                    Model Fural — Autentikasi sesi akun tanpa meminta raw API Key
                  </p>
                </div>
              </div>
              <button
                onClick={() => setIsModalOpen(false)}
                className="text-muted-foreground hover:text-foreground p-1 rounded-md hover:bg-muted transition-colors cursor-pointer"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleCreateAgent} className="space-y-4">
              {/* Agent Name */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1.5">
                  Nama Agent
                </label>
                <input
                  type="text"
                  required
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="Contoh: Dev Worker, Antigravity Assistant"
                  className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                />
              </div>

              {/* Handle / Mention */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1.5">
                  Handle / Mention
                </label>
                <div className="flex items-center gap-1.5 mb-2 flex-wrap">
                  {['@dev', '@assistant', '@sm', '@reviewer', '@qa'].map((h) => (
                    <button
                      key={h}
                      type="button"
                      onClick={() => setNewHandle(h)}
                      className={cn(
                        'text-[10px] font-mono px-2 py-1 rounded transition-colors',
                        newHandle === h
                          ? 'bg-primary text-primary-foreground font-bold'
                          : 'bg-muted text-muted-foreground hover:bg-accent'
                      )}
                    >
                      {h}
                    </button>
                  ))}
                </div>
                <input
                  type="text"
                  required
                  value={newHandle}
                  onChange={(e) => setNewHandle(e.target.value)}
                  placeholder="@dev"
                  className="w-full px-3 py-2 text-xs font-mono bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                />
              </div>

              {/* Runtime / Provider Choice */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1.5">
                  Runtime / Provider
                </label>
                <div className="grid grid-cols-3 gap-2">
                  {[
                    { id: 'antigravity', label: 'Antigravity CLI (agy)', desc: 'Autonomous Worker' },
                    { id: 'gemini-cli', label: 'Gemini CLI', desc: 'Google Cloud' },
                    { id: 'claude-cli', label: 'Claude CLI', desc: 'Anthropic' },
                  ].map((prov) => (
                    <button
                      key={prov.id}
                      type="button"
                      onClick={() => setNewProvider(prov.id)}
                      className={cn(
                        'p-2.5 rounded-lg border text-left transition-all text-xs flex flex-col justify-between',
                        newProvider === prov.id
                          ? 'border-primary bg-primary/10 text-primary font-bold shadow-xs'
                          : 'border-border bg-card text-muted-foreground hover:bg-accent hover:text-foreground'
                      )}
                    >
                      <span className="text-[11px] font-semibold">{prov.label}</span>
                      <span className="text-[9px] opacity-70 mt-0.5">{prov.desc}</span>
                    </button>
                  ))}
                </div>
              </div>

              {/* Model / Engine */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1.5">
                  Model / Engine
                </label>
                <select
                  value={newModel}
                  onChange={(e) => setNewModel(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                >
                  <option value="Gemini 3.8 Flash (High-Speed)">Gemini 3.8 Flash (High-Speed) — Rekomendasi</option>
                  <option value="Gemini Pro / Claude Sonnet">Gemini Pro / Claude Sonnet — Reasoning Tinggi</option>
                  <option value="In-Process Go Runner">In-Process Go Runner &amp; Shell Driver</option>
                </select>
              </div>

              {/* Role */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1.5">
                  Peran / Spesialisasi
                </label>
                <input
                  type="text"
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value)}
                  placeholder="Contoh: Development Team (Worker)"
                  className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                />
              </div>

              {/* ── Mekanisme Connect/Authorize (Model Fural — No API Key) ── */}
              <div className="p-3.5 rounded-xl border border-primary/20 bg-primary/5 space-y-2.5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1.5 text-xs font-bold text-primary">
                    <KeyRound className="h-4 w-4" />
                    <span>Mekanisme Otorisasi Akun (Model Fural)</span>
                  </div>
                  <span className="text-[9px] font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-bold border border-emerald-500/20">
                    Zero API Key
                  </span>
                </div>

                <p className="text-[11px] text-muted-foreground leading-relaxed">
                  Tidak perlu memasukkan API Key mentah. Kredensial agen dikelola secara otonom melalui sesi login akun/CLI storage yang aman.
                </p>

                {/* Auth Mode Toggle */}
                <div className="flex rounded-md bg-muted p-0.5 text-xs">
                  <button
                    type="button"
                    onClick={() => setAuthType('cli_login')}
                    className={cn(
                      'flex-1 py-1 rounded text-center transition-all font-medium text-[11px]',
                      authType === 'cli_login'
                        ? 'bg-background text-foreground shadow-xs font-bold'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                  >
                    CLI Session Token
                  </button>
                  <button
                    type="button"
                    onClick={() => setAuthType('oauth')}
                    className={cn(
                      'flex-1 py-1 rounded text-center transition-all font-medium text-[11px]',
                      authType === 'oauth'
                        ? 'bg-background text-foreground shadow-xs font-bold'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                  >
                    Google OAuth / Device Auth
                  </button>
                </div>

                {/* Device Code & CLI Command Snippet */}
                <div className="bg-background/90 p-2.5 rounded-lg border border-border/60 space-y-2 font-mono text-[11px]">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Device Authorization Code:</span>
                    <span className="font-bold text-primary tracking-wider">{deviceCode}</span>
                  </div>
                  <div className="flex items-center justify-between gap-2 pt-1 border-t border-border/40">
                    <span className="text-[10px] text-muted-foreground truncate">
                      cuan agent auth --handle {newHandle} --runtime {newProvider}
                    </span>
                    <button
                      type="button"
                      onClick={handleCopyCommand}
                      title="Salin Perintah CLI"
                      className="text-muted-foreground hover:text-foreground p-1 rounded hover:bg-muted transition-colors shrink-0"
                    >
                      {copied ? <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                    </button>
                  </div>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="pt-3 border-t border-border flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="px-3.5 py-2 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={isConnecting || !newName.trim()}
                  className="px-4 py-2 text-xs font-bold bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 rounded-md transition-all flex items-center gap-2 shadow-xs cursor-pointer"
                >
                  {isConnecting && <RefreshCw className="h-3.5 w-3.5 animate-spin" />}
                  <span>{isConnecting ? 'Menghubungkan Sesi CLI...' : 'Hubungkan & Otorisasi Agent'}</span>
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
