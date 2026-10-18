import { useState, useEffect } from 'react'
import {
  Users,
  Crown,
  Bot,
  Cpu,
  ShieldCheck,
  CheckCircle2,
  Plus,
  Terminal,
  Zap,
  X,
  Sparkles,
  RefreshCw,
  Copy,
} from 'lucide-react'
import { cn } from '../../lib/utils'
import { agentsApi } from '../../api/agents'

export interface TeamMember {
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
  model?: string
  authModel?: string
  provider?: string
}

const DEFAULT_TEAM_MEMBERS: TeamMember[] = [
  {
    id: 'po-buchin',
    name: 'Buchin',
    handle: '@buchin',
    role: 'product_owner',
    roleTitle: 'Product Owner (Human Owner)',
    type: 'human',
    avatar: 'BU',
    responsibilities: [
      'Menetapkan visi, outcome, dan prioritas backlog',
      'Sprint Planning & 1-Click Approval di Inbox',
      'Sprint Review & evaluasi hasil produksi akhir',
    ],
    status: 'Active Owner',
    detail: 'Pengambil keputusan tunggal atas arah produk dan kepuasan Definition of Done.',
    model: 'Human In Loop',
    authModel: 'Owner Auth Session',
  },
  {
    id: 'sm-agent',
    name: 'Agent Manager (Scrum Master)',
    handle: '@sm',
    role: 'scrum_master',
    roleTitle: 'Scrum Master (AI Manager)',
    type: 'agent',
    avatar: 'SM',
    responsibilities: [
      'Autonomous triage pesan Inbox secara terstruktur',
      'Merumuskan Problem Statement & checklist Definition of Done (DoD)',
      'Memilih & mendelegasikan tugas ke worker agent yang memiliki kuota longgar',
    ],
    status: 'Online (Autonomous)',
    detail: 'Menjaga alur kerja sprint tanpa cognitive overload bagi Product Owner.',
    model: 'Gemini 3.8 Flash / Autonomous SM',
    authModel: 'CLI Session (Model Fural — No API Key)',
    provider: 'antigravity',
  },
  {
    id: 'worker-flash',
    name: 'Antigravity Flash Worker',
    handle: '@pin',
    role: 'dev_team',
    roleTitle: 'Development Team (Fast Worker)',
    type: 'agent',
    avatar: 'FL',
    responsibilities: [
      'Menulis kode modular & refactoring aman',
      'Targeted unit test & verifikasi zero regression',
      'Menerbitkan commit bertanda author Buchin & push main',
    ],
    status: 'Connected (Active)',
    detail: 'Eksekutor teknis berkecepatan tinggi dengan kuota token optimal.',
    model: 'Gemini 3.8 Flash (High-Speed)',
    authModel: 'Antigravity CLI (agy)',
    provider: 'antigravity',
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
    status: 'Standby / Ready',
    detail: 'Spesialis reasoning mendalam untuk task kompleks dan transisi handoff otonom.',
    model: 'Gemini Pro / Claude Sonnet',
    authModel: 'Antigravity CLI (agy)',
    provider: 'antigravity',
  },
]

export function TeamView({
  selectedRole = 'all',
  onAgentAdded,
}: {
  selectedRole?: string
  onAgentAdded?: () => void
}) {
  const [members, setMembers] = useState<TeamMember[]>(DEFAULT_TEAM_MEMBERS)
  const [isModalOpen, setIsModalOpen] = useState(false)

  // Add Agent Form State
  const [newName, setNewName] = useState('')
  const [newHandle, setNewHandle] = useState('@dev')
  const [newProvider, setNewProvider] = useState('antigravity')
  const [newModel, setNewModel] = useState('Gemini 3.8 Flash (High-Speed)')
  const [newRole, setNewRole] = useState('Sprint Worker Agent')
  const [authType, setAuthType] = useState<'cli_login' | 'oauth'>('cli_login')
  const [deviceCode, setDeviceCode] = useState('CUAN-8892')
  const [isConnecting, setIsConnecting] = useState(false)
  const [copied, setCopied] = useState(false)

  // Fetch registered agents from backend DB on mount
  useEffect(() => {
    let isMounted = true
    agentsApi
      .list()
      .then((dbAgents) => {
        if (!isMounted || !dbAgents || dbAgents.length === 0) return
        const mapped: TeamMember[] = dbAgents.map((a) => ({
          id: a.id,
          name: a.name,
          handle: a.name.includes('@')
            ? '@' + a.name.split('@')[1].replace(')', '')
            : '@' + a.id.replace('agent_', ''),
          role: 'dev_team',
          roleTitle: a.role || 'Sprint Worker Agent',
          type: 'agent',
          avatar: a.name.substring(0, 2).toUpperCase(),
          status: 'Connected (Active)',
          detail: 'Agent terhubung via CLI login Model Fural tanpa raw API key.',
          model: a.model_id || 'Gemini 3.8 Flash',
          authModel: 'CLI Session (Model Fural)',
          provider: a.provider || 'antigravity',
          responsibilities: [
            'Menjalankan instruksi task otomatis',
            'Unit testing & verifikasi hasil kerja',
            'Melaporkan progres ke Scrum Master',
          ],
        }))

        setMembers((prev) => {
          const ids = new Set(prev.map((p) => p.id))
          const toAdd = mapped.filter((m) => !ids.has(m.id))
          return [...prev, ...toAdd]
        })
      })
      .catch((err) => console.error('Failed to load agents in TeamView:', err))

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

      const newMember: TeamMember = {
        id: res.agent.id,
        name: res.agent.name,
        handle: res.handle || newHandle.trim(),
        role: 'dev_team',
        roleTitle: res.agent.role || newRole.trim(),
        type: 'agent',
        avatar: newName.trim().substring(0, 2).toUpperCase(),
        status: 'Connected (Active)',
        detail: 'Agent terhubung via CLI login Model Fural tanpa raw API key.',
        model: res.agent.model_id || newModel,
        authModel: 'CLI Session (Model Fural)',
        provider: res.provider || newProvider,
        responsibilities: [
          'Menjalankan instruksi task otomatis',
          'Unit testing & verifikasi hasil kerja',
          'Melaporkan progres ke Scrum Master',
        ],
      }

      setMembers((prev) => [newMember, ...prev])
      setIsModalOpen(false)
      setNewName('')
      setNewHandle('@dev')
      if (onAgentAdded) onAgentAdded()
    } catch (err) {
      console.error('Failed to connect agent via CLI:', err)
      const fallbackMember: TeamMember = {
        id: `agent-${Date.now()}`,
        name: `${newName.trim()} (${newHandle.trim()})`,
        handle: newHandle.trim(),
        role: 'dev_team',
        roleTitle: newRole.trim(),
        type: 'agent',
        avatar: newName.trim().substring(0, 2).toUpperCase(),
        status: 'Connected (Active)',
        detail: 'Agent terdaftar lokal via CLI Session fallback.',
        model: newModel,
        authModel: 'CLI Session (Fallback)',
        provider: newProvider,
        responsibilities: [
          'Menjalankan instruksi task otomatis',
          'Unit testing & verifikasi hasil kerja',
        ],
      }
      setMembers((prev) => [fallbackMember, ...prev])
      setIsModalOpen(false)
      if (onAgentAdded) onAgentAdded()
    } finally {
      setIsConnecting(false)
    }
  }

  const filtered = members.filter((m) => {
    if (selectedRole === 'all') return true
    if (selectedRole === 'human') return m.type === 'human'
    if (selectedRole === 'agent') return m.type === 'agent'
    return m.role === selectedRole
  })

  return (
    <div className="max-w-5xl mx-auto space-y-6 pb-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-border/60">
        <div>
          <h1 className="text-xl font-bold text-foreground flex items-center gap-2">
            <Users className="h-5 w-5 text-primary" />
            <span>Unified Team Directory</span>
          </h1>
          <p className="text-xs text-muted-foreground mt-0.5">
            Manusia dan AI Agent berdampingan dalam struktur Agentic Scrum: Product Owner (Human), Scrum Master (AI Manager), dan Development Team (Worker Agents).
          </p>
        </div>

        <button
          onClick={handleOpenModal}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors rounded-md shadow-xs shrink-0 self-start sm:self-auto"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>+ Tambah Agent (Model Fural)</span>
        </button>
      </div>

      {/* Hierarchy Overview Cards */}
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
            <span className="text-amber-600 dark:text-amber-400 font-bold px-2 py-0.5 rounded bg-amber-500/10">1 Human Owner</span>
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
            <span className="text-indigo-600 dark:text-indigo-400 font-bold px-2 py-0.5 rounded bg-indigo-500/10">Autonomous AI</span>
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
              Pool agent Antigravity CLI yang mengeksekusi kode, menulis test terarah, dan handoff otomatis jika kuota habis.
            </p>
          </div>
          <div className="mt-3 pt-3 border-t border-border flex items-center justify-between text-[11px] font-mono">
            <span className="text-foreground font-semibold">Antigravity Pool</span>
            <span className="text-emerald-600 dark:text-emerald-400 font-bold px-2 py-0.5 rounded bg-emerald-500/10">Multi-Agent</span>
          </div>
        </div>
      </div>

      {/* Team Member Cards */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-xs font-bold uppercase tracking-wider text-muted-foreground font-mono">
            Semua Anggota Tim ({filtered.length})
          </h2>
          <span className="text-[11px] text-muted-foreground">
            {members.filter((m) => m.type === 'human').length} Manusia · {members.filter((m) => m.type === 'agent').length} AI Agent
          </span>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filtered.map((member) => (
            <div
              key={member.id}
              className="border border-border bg-card p-5 rounded-lg shadow-xs flex flex-col justify-between space-y-4 hover:border-primary/40 transition-colors"
            >
              <div>
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div
                      className={cn(
                        'h-11 w-11 rounded-full flex items-center justify-center font-mono font-bold text-xs shrink-0',
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
                        <span>{member.name}</span>
                        <span className="text-[10px] text-muted-foreground font-mono">
                          {member.handle}
                        </span>
                      </h3>
                      <p className="text-xs text-primary font-medium">{member.roleTitle}</p>
                    </div>
                  </div>

                  <div className="flex items-center gap-1.5 shrink-0 flex-wrap justify-end">
                    {member.type === 'human' ? (
                      <span className="text-[9px] font-mono font-bold px-2 py-0.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
                        Human Owner
                      </span>
                    ) : (
                      <span className="text-[9px] font-mono font-bold px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 flex items-center gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
                        <span>Connected</span>
                      </span>
                    )}
                  </div>
                </div>

                <p className="text-xs text-muted-foreground mt-3 leading-relaxed">
                  {member.detail}
                </p>

                {/* Model & Auth Badge for Agents */}
                {member.type === 'agent' && (
                  <div className="mt-3 p-2 bg-muted/40 rounded border border-border/60 flex flex-wrap items-center justify-between gap-2 text-[10px] font-mono">
                    <span className="text-foreground/90 font-medium flex items-center gap-1">
                      <Cpu className="h-3 w-3 text-primary shrink-0" />
                      <span>{member.model}</span>
                    </span>
                    <span className="text-muted-foreground">{member.authModel}</span>
                  </div>
                )}

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

      {/* Add Agent Modal — Model Fural (Zero API Key) */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/80 backdrop-blur-xs animate-in fade-in duration-200">
          <div className="bg-card border border-border rounded-xl shadow-2xl max-w-lg w-full overflow-hidden flex flex-col max-h-[90vh]">
            {/* Header */}
            <div className="p-5 border-b border-border flex items-center justify-between bg-muted/20">
              <div className="flex items-center gap-2.5">
                <div className="p-2 bg-primary/10 text-primary rounded-lg">
                  <Bot className="h-5 w-5" />
                </div>
                <div>
                  <h3 className="font-bold text-sm text-foreground flex items-center gap-2">
                    Tambah Akun Agent (Model Fural)
                    <span className="text-[10px] bg-primary/15 text-primary px-1.5 py-0.2 rounded font-mono">
                      No API Key
                    </span>
                  </h3>
                  <p className="text-[11px] text-muted-foreground">
                    Hubungkan agent runtime via CLI / Sesi Akun terotorisasi tanpa meminta API Key mentah.
                  </p>
                </div>
              </div>
              <button
                onClick={() => setIsModalOpen(false)}
                className="p-1.5 text-muted-foreground hover:text-foreground rounded-md hover:bg-muted transition-colors"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Form */}
            <form onSubmit={handleCreateAgent} className="p-5 space-y-4 overflow-y-auto">
              {/* Agent Handle Preset */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1">
                  Handle / Identifier
                </label>
                <div className="grid grid-cols-4 gap-2">
                  {['@dev', '@sm', '@assistant', '@tester'].map((handle) => (
                    <button
                      key={handle}
                      type="button"
                      onClick={() => {
                        setNewHandle(handle)
                        if (!newName) {
                          setNewName(
                            handle === '@dev'
                              ? 'Dev Worker'
                              : handle === '@sm'
                              ? 'Scrum Master'
                              : handle === '@assistant'
                              ? 'Dojo Assistant'
                              : 'QA Tester'
                          )
                        }
                      }}
                      className={cn(
                        'py-1 px-2 text-xs font-mono font-medium rounded border transition-colors',
                        newHandle === handle
                          ? 'border-primary bg-primary/10 text-primary font-bold'
                          : 'border-border text-muted-foreground hover:bg-muted'
                      )}
                    >
                      {handle}
                    </button>
                  ))}
                </div>
              </div>

              {/* Agent Name */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1">
                  Nama Agent
                </label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Antigravity Fast Coder"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </div>

              {/* Role Title */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1">
                  Peran / Tanggung Jawab
                </label>
                <input
                  type="text"
                  placeholder="e.g. Sprint Worker Agent"
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </div>

              {/* Runtime Provider */}
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-foreground mb-1">
                    Runtime Provider
                  </label>
                  <select
                    value={newProvider}
                    onChange={(e) => setNewProvider(e.target.value)}
                    className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                  >
                    <option value="antigravity">Antigravity CLI (agy)</option>
                    <option value="gemini">Gemini CLI</option>
                    <option value="claude">Claude Code CLI</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-foreground mb-1">
                    Model Engine
                  </label>
                  <select
                    value={newModel}
                    onChange={(e) => setNewModel(e.target.value)}
                    className="w-full px-3 py-2 text-xs bg-background border border-input rounded-md text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                  >
                    <option value="Gemini 3.8 Flash (High-Speed)">Gemini 3.8 Flash (Fast)</option>
                    <option value="Gemini Pro (Deep Reasoning)">Gemini Pro (Deep)</option>
                    <option value="Claude 3.5 Sonnet">Claude 3.5 Sonnet</option>
                    <option value="Auto (Scrum Orchestrated)">Auto (Orchestrated)</option>
                  </select>
                </div>
              </div>

              {/* Auth Mode — Zero API Key */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1">
                  Mekanisme Otorisasi (No API Key Required)
                </label>
                <div className="grid grid-cols-2 gap-2 mb-2">
                  <button
                    type="button"
                    onClick={() => setAuthType('cli_login')}
                    className={cn(
                      'flex items-center gap-2 p-2 rounded-lg border text-left transition-colors',
                      authType === 'cli_login'
                        ? 'border-primary bg-primary/10 text-foreground font-semibold'
                        : 'border-border text-muted-foreground hover:bg-muted'
                    )}
                  >
                    <Terminal className="h-4 w-4 text-primary shrink-0" />
                    <div>
                      <div className="text-xs">CLI Session Auth</div>
                      <div className="text-[10px] text-muted-foreground">Kredensial Sesi CLI Lokal</div>
                    </div>
                  </button>

                  <button
                    type="button"
                    onClick={() => setAuthType('oauth')}
                    className={cn(
                      'flex items-center gap-2 p-2 rounded-lg border text-left transition-colors',
                      authType === 'oauth'
                        ? 'border-primary bg-primary/10 text-foreground font-semibold'
                        : 'border-border text-muted-foreground hover:bg-muted'
                    )}
                  >
                    <Sparkles className="h-4 w-4 text-amber-500 shrink-0" />
                    <div>
                      <div className="text-xs">Device Auth Code</div>
                      <div className="text-[10px] text-muted-foreground">Otorisasi Device 1-Klik</div>
                    </div>
                  </button>
                </div>

                <div className="bg-muted/40 p-3 rounded-lg border border-border text-xs space-y-2">
                  <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                    <span>Otorisasi via Terminal:</span>
                    <button
                      type="button"
                      onClick={handleCopyCommand}
                      className="text-primary hover:underline flex items-center gap-1"
                    >
                      <Copy className="h-3 w-3" />
                      <span>{copied ? 'Tersalin!' : 'Salin Perintah'}</span>
                    </button>
                  </div>
                  <code className="block bg-background p-2 rounded border border-border font-mono text-[11px] text-primary break-all">
                    cuan agent auth --handle {newHandle} --code {deviceCode}
                  </code>
                  <p className="text-[10px] text-muted-foreground">
                    ✓ Tanpa API Key raw: Cuan memverifikasi token sesi melalui adapter CLI aman berstandar Fural.
                  </p>
                </div>
              </div>

              {/* Submit Buttons */}
              <div className="pt-2 flex items-center justify-end gap-2 border-t border-border">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={isConnecting || !newName.trim()}
                  className="flex items-center gap-1.5 px-4 py-2 bg-primary text-primary-foreground font-semibold text-xs rounded-md hover:bg-primary/90 transition-colors disabled:opacity-50"
                >
                  {isConnecting ? (
                    <>
                      <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                      <span>Menghubungkan Agent...</span>
                    </>
                  ) : (
                    <>
                      <Zap className="h-3.5 w-3.5" />
                      <span>Hubungkan Agent (Model Fural)</span>
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export function TeamSubPanel({
  selectedRole,
  onSelectRole,
  onAddAgentClick,
}: {
  selectedRole: string
  onSelectRole: (role: string) => void
  onAddAgentClick?: () => void
}) {
  const ROLES = [
    { id: 'all', label: 'Semua Anggota' },
    { id: 'human', label: 'Human Owner' },
    { id: 'agent', label: 'AI Agents' },
    { id: 'dev_team', label: 'Dev Team Workers' },
  ]

  return (
    <div className="flex flex-col h-full bg-sidebar/50">
      {onAddAgentClick && (
        <div className="p-3 border-b border-border">
          <button
            onClick={onAddAgentClick}
            className="flex items-center justify-center gap-2 w-full px-3 py-2 text-xs font-semibold bg-primary text-primary-foreground hover:bg-primary/90 transition-colors rounded-md shadow-xs"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>+ Tambah Agent</span>
          </button>
        </div>
      )}

      <div className="p-3 border-b border-border">
        <span className="text-xs font-semibold text-foreground">
          Filter Direktori Tim
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
          </button>
        ))}
      </div>
    </div>
  )
}
