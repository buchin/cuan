import { useState, useEffect } from 'react'
import {
  Users,
  Crown,
  Bot,
  Cpu,
  ShieldCheck,
  CheckCircle2,
  Plus,
  Zap,
  X,
  RefreshCw,
} from 'lucide-react'
import { cn } from '../../lib/utils'
import { agentsApi, type HarnessAuth } from '../../api/agents'

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
    authModel: 'Sesi Host Server (Tanpa API Key)',
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
  const [members, setMembers] = useState<TeamMember[]>(DEFAULT_TEAM_MEMBERS.filter(member => member.type === 'human'))
  const [isModalOpen, setIsModalOpen] = useState(false)

  // S23.2: Direct UI Add Agent Form State
  const [newHandle, setNewHandle] = useState('@dev')
  const [newHarness, setNewHarness] = useState<'agy' | 'claude' | 'codex' | 'custom'>('agy')
  const [apiKey, setApiKey] = useState('')
  const [customEndpoint, setCustomEndpoint] = useState('')
  const [useHostSession, setUseHostSession] = useState(true)
  const [isConnecting, setIsConnecting] = useState(false)
  const [auth, setAuth] = useState<HarnessAuth | null>(null)
  const [authBusy, setAuthBusy] = useState(false)
  const [authError, setAuthError] = useState('')
  const [authCode, setAuthCode] = useState('')

  useEffect(() => {
    if (!isModalOpen || newHarness === 'custom') { setAuthBusy(false); return }
    let active = true
    setAuth(null)
    setAuthError('')
    setAuthBusy(true)
    agentsApi.authStatus(newHarness, true).then(value => { if (active) setAuth(value) })
      .catch(() => { if (active) setAuthError('Tidak dapat memeriksa koneksi server. Coba lagi.') })
      .finally(() => { if (active) setAuthBusy(false) })
    return () => { active = false }
  }, [isModalOpen, newHarness])

  const closeModal = () => {
    if (auth?.session_id) void agentsApi.authCancel(auth.session_id).catch(() => {})
    setAuth(null)
    setAuthCode('')
    setIsModalOpen(false)
  }

  const login = async () => {
    setAuthBusy(true)
    setAuthError('')
    try { setAuth(await agentsApi.authInit()) }
    catch (error) { setAuthError(error instanceof Error ? error.message : 'Login gagal. Mulai ulang login.') }
    finally { setAuthBusy(false) }
  }

  const submitCode = async () => {
    if (!auth?.session_id) return
    setAuthBusy(true)
    setAuthError('')
    try { setAuth(await agentsApi.authCode(auth.session_id, authCode)); setAuthCode('') }
    catch (error) { setAuthError(error instanceof Error ? error.message : 'Kode gagal diverifikasi. Mulai ulang login.') }
    finally { setAuthCode(''); setAuthBusy(false) }
  }

  // Fetch registered agents from backend DB on mount
  useEffect(() => {
    let isMounted = true
    agentsApi
      .list()
      .then((dbAgents) => {
        if (!isMounted || !dbAgents || dbAgents.length === 0) return
        const mapped: TeamMember[] = dbAgents.map((a) => {
          const handle = a.name.startsWith('@')
            ? a.name
            : a.name.includes('@')
            ? '@' + a.name.split('@')[1].replace(')', '')
            : '@' + a.id.replace('agent_', '')
          const harness = a.harness || a.provider || 'agy'

          return {
            id: a.id,
            name: a.name,
            handle: handle,
            role: 'dev_team',
            roleTitle: `Harness: ${harness.toUpperCase()}`,
            type: 'agent',
            avatar: handle.replace('@', '').substring(0, 2).toUpperCase() || 'AG',
            status: a.enabled === false || a.status === 'inactive' ? 'Disabled' : 'Registered — connection unverified',
            detail: `Agent terhubung dengan harness ${harness}.`,
            model: a.model_id || harness,
            authModel: a.model_id ? a.model_id : `Harness ${harness.toUpperCase()}`,
            provider: harness,
            responsibilities: [
              'Merespons secara dinamis sesuai konteks pesan (SM saat koordinasi, Developer saat teknis)',
              'Menjalankan instruksi task otomatis via PTY runner',
              'Melaporkan progres ke Scrum Master',
            ],
          }
        })

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
    setIsModalOpen(true)
  }

  const handleHandleChange = (val: string) => {
    let clean = val.trim().toLowerCase()
    if (!clean.startsWith('@')) {
      clean = '@' + clean.replace(/^@+/, '')
    }
    clean = '@' + clean.slice(1).replace(/[\s\t\n]+/g, '_').replace(/[^a-z0-9_-]/g, '')
    setNewHandle(clean)
  }

  const handleCreateAgent = async (e: React.FormEvent) => {
    e.preventDefault()
    const sanitized = newHandle.trim()
    if (!sanitized || sanitized === '@' || (newHarness !== 'custom' && !auth?.verified)) return

    setIsConnecting(true)
    try {
      const connID = `conn_${sanitized.replace(/^@/, '')}_${Date.now()}`
      const isHostSession = newHarness === 'agy' && useHostSession
      const activeApiKey = isHostSession ? '' : apiKey.trim()

      const agentRes = newHarness === 'agy'
        ? (await agentsApi.connectCLI({ name: sanitized, handle: sanitized, provider: 'agy', auth_type: 'oauth' })).agent
        : await agentsApi.create({ name: sanitized, harness: newHarness, connection_id: connID, enabled: true, api_key: activeApiKey, endpoint: customEndpoint.trim(), host_session: isHostSession })

      let computedAuthModel = ''
      if (newHarness === 'custom') {
        computedAuthModel = customEndpoint ? `Endpoint: ${customEndpoint}` : 'Custom API Key'
      } else if (newHarness === 'agy') {
        computedAuthModel = isHostSession ? 'Sesi Host Server / Env Default' : 'Google AI Studio Key'
      } else {
        computedAuthModel = `${newHarness.toUpperCase()} API Key`
      }

      const newMember: TeamMember = {
        id: agentRes.id || `agent-${Date.now()}`,
        name: sanitized,
        handle: sanitized,
        role: 'dev_team',
        roleTitle: `Harness: ${newHarness === 'agy' ? 'Antigravity (agy)' : newHarness === 'claude' ? 'Claude' : newHarness === 'codex' ? 'Codex' : 'Custom Model'}`,
        type: 'agent',
        avatar: sanitized.replace(/^@/, '').substring(0, 2).toUpperCase() || 'AG',
        status: 'Connected (Active)',
        detail: `Agent terhubung dengan harness ${newHarness}.`,
        model: newHarness === 'custom' ? 'Custom LLM' : `${newHarness.toUpperCase()} Engine`,
        authModel: computedAuthModel,
        provider: newHarness,
        responsibilities: [
          'Merespons dinamis sesuai konteks pesan (SM saat koordinasi, Developer saat teknis)',
          'Menjalankan eksekusi task PTY runner',
          'Melaporkan progres ke Scrum Master',
        ],
      }

      setMembers((prev) => [newMember, ...prev])
      setIsModalOpen(false)
      setNewHandle('@dev')
      setApiKey('')
      setCustomEndpoint('')
      setUseHostSession(true)
      if (onAgentAdded) onAgentAdded()
    } catch (err) {
      console.error('Failed to connect agent via UI:', err)
      setAuthError(err instanceof Error ? err.message : 'Koneksi gagal. Agen belum terhubung.')
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
          <span>+ Tambah Agen</span>
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
                        <span>{member.status}</span>
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

      {/* S23.2: Fural-Style Add Agent Modal UI */}
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
                    Tambah Agen
                    <span className="text-[10px] bg-primary/15 text-primary px-1.5 py-0.2 rounded font-mono">
                      Zero API Key
                    </span>
                  </h3>
                  <p className="text-[11px] text-muted-foreground">
                    Hubungkan akun agent mandiri dengan memilih Harness dan input nama @handle.
                  </p>
                </div>
              </div>
              <button
                onClick={closeModal}
                className="p-1.5 text-muted-foreground hover:text-foreground rounded-md hover:bg-muted transition-colors"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Form */}
            <form onSubmit={handleCreateAgent} className="p-5 space-y-4 overflow-y-auto">
              {/* 1. Pilihan Harness */}
              <div>
                <label className="block text-xs font-semibold text-foreground mb-1.5">
                  Pilihan Harness (Runtime Agent)
                </label>
                <div className="grid grid-cols-2 gap-2">
                  {[
                    { id: 'agy', label: 'Antigravity (agy)', desc: 'Google Deepmind CLI' },
                    { id: 'claude', label: 'Claude', desc: 'Anthropic Claude Code' },
                    { id: 'codex', label: 'Codex', desc: 'OpenAI Codex Runner' },
                    { id: 'custom', label: 'Custom Model', desc: 'Custom LLM / API Key' },
                  ].map((h) => (
                    <button
                      key={h.id}
                      type="button"
                      onClick={() => setNewHarness(h.id as any)}
                      className={cn(
                        'p-2.5 rounded-lg border text-left transition-all',
                        newHarness === h.id
                          ? 'border-primary bg-primary/10 ring-1 ring-primary text-foreground'
                          : 'border-border text-muted-foreground hover:bg-muted/60'
                      )}
                    >
                      <div className="text-xs font-bold text-foreground flex items-center justify-between">
                        <span>{h.label}</span>
                        {newHarness === h.id && (
                          <span className="h-2 w-2 rounded-full bg-primary" />
                        )}
                      </div>
                      <div className="text-[10px] text-muted-foreground mt-0.5">{h.desc}</div>
                    </button>
                  ))}
                </div>
              </div>

              {/* 2. Agent Name (@handle) dengan prefix @ dan auto-sanitasi */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="block text-xs font-semibold text-foreground">
                    Agent Name (Mention Handle)
                  </label>
                  <span className="text-[10px] font-mono text-muted-foreground">
                    Prefix @ & auto-sanitasi
                  </span>
                </div>
                <div className="relative">
                  <input
                    type="text"
                    required
                    placeholder="@dev"
                    value={newHandle}
                    onChange={(e) => handleHandleChange(e.target.value)}
                    className="w-full px-3 py-2 text-xs font-mono bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                  />
                </div>
                <div className="flex items-center gap-1.5 mt-2">
                  <span className="text-[11px] text-muted-foreground">Preset cepat:</span>
                  {['@dev', '@sm', '@coder', '@tester'].map((h) => (
                    <button
                      key={h}
                      type="button"
                      onClick={() => setNewHandle(h)}
                      className={cn(
                        'px-2 py-0.5 text-[10px] font-mono rounded border transition-colors',
                        newHandle === h
                          ? 'border-primary bg-primary/10 text-primary font-bold'
                          : 'border-border text-muted-foreground hover:bg-muted'
                      )}
                    >
                      {h}
                    </button>
                  ))}
                </div>
              </div>

              {/* 3. Panel Status Sesi Akun / API Key jika custom model */}
              {newHarness !== 'custom' ? (
                <div className="bg-muted/40 p-3 rounded-lg border border-border text-xs space-y-2">
                  <p role="status">{authBusy ? 'Memeriksa koneksi provider…' : auth?.verified ? 'Koneksi provider terverifikasi' : auth?.message || 'Login belum diverifikasi'}</p>
                  {newHarness === 'agy' && !auth?.verified && <>
                    <button type="button" disabled={authBusy} onClick={login} className="px-3 py-2 rounded bg-primary text-primary-foreground">Login dengan Google</button>
                    {auth?.auth_url && <>
                      <a href={auth.auth_url} target="_blank" rel="noopener noreferrer" className="block underline">Buka otorisasi Google</a>
                      <p>Selesaikan login di browser, lalu tempel kode. Sesi berlaku 5 menit.</p>
                      <input type="password" autoComplete="off" aria-label="Kode otorisasi Google" value={authCode} onChange={event => setAuthCode(event.target.value)} className="w-full p-2 bg-background border rounded" />
                      <button type="button" disabled={authBusy || !authCode.trim()} onClick={submitCode} className="px-3 py-2 border rounded">Verifikasi kode dan koneksi</button>
                    </>}
                  </>}
                  {newHarness !== 'agy' && <p>Autentikasi harness ini belum dapat diverifikasi.</p>}
                </div>
              ) : (
                <div className="bg-muted/40 p-3 rounded-lg border border-border text-xs space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-foreground flex items-center gap-1.5">
                      <Zap className="h-3.5 w-3.5 text-amber-500" />
                      <span>Konfigurasi Custom Model</span>
                    </span>
                    <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 font-bold">
                      API Key Required
                    </span>
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-foreground mb-1">
                      API Key / Access Token
                    </label>
                    <input
                      type="password"
                      placeholder="sk-..."
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      className="w-full px-3 py-1.5 text-xs bg-background border border-input rounded-md text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-foreground mb-1">
                      Custom Endpoint / Base URL (Opsional)
                    </label>
                    <input
                      type="text"
                      placeholder="https://api.openai.com/v1 atau http://localhost:11434"
                      value={customEndpoint}
                      onChange={(e) => setCustomEndpoint(e.target.value)}
                      className="w-full px-3 py-1.5 text-xs bg-background border border-input rounded-md text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                    />
                  </div>
                </div>
              )}

              {authError && <p role="alert" className="text-sm text-destructive">{authError}</p>}
              {/* Submit Buttons */}
              <div className="pt-2 flex items-center justify-end gap-2 border-t border-border">
                <button
                  type="button"
                  onClick={closeModal}
                  className="px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={isConnecting || authBusy || !newHandle || newHandle === '@' || (newHarness !== 'custom' && !auth?.verified)}
                  className="flex items-center gap-1.5 px-4 py-2 bg-primary text-primary-foreground font-semibold text-xs rounded-md hover:bg-primary/90 transition-colors disabled:opacity-50"
                >
                  {isConnecting ? (
                    <>
                      <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                      <span>Menghubungkan...</span>
                    </>
                  ) : (
                    <>
                      <Plus className="h-3.5 w-3.5" />
                      <span>+ Hubungkan Agen</span>
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
