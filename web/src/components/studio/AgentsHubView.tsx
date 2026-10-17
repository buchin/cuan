import { useState, useEffect, useMemo, FormEvent } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  Bot,
  Plus,
  RefreshCw,
  Search,
  KeyRound,
  Sparkles,
  ShieldCheck,
  AlertCircle,
  Zap,
  Check,
  X
} from 'lucide-react'
import { AgentDTO, CreateAgentPayload, UpdateAgentPayload, AgentConnectionDTO } from './types'
import { AgentCard } from './AgentCard'
import { AgentConnectionModal } from './AgentConnectionModal'

interface AgentsHubViewProps {
  onNavigate?: (path: string) => void
}

const ROLE_SUGGESTIONS = [
  'Fullstack Architect',
  'Backend Go Specialist',
  'Frontend UI/UX Craftsman',
  'DevOps & SRE Engineer',
  'QA & Test Automation Specialist',
  'Product & Scrum Facilitator',
]

const AVATAR_OPTIONS = ['🤖', '⚡', '🛠️', '🚀', '🧠', '🎨', '🔍', '🛡️', '💻', '🎯']

const MODEL_OPTIONS = [
  { id: 'gemini-1.5-pro', name: 'Gemini 1.5 Pro', desc: 'Recommended: Deep reasoning, complex code generation, 2M context' },
  { id: 'gemini-1.5-flash', name: 'Gemini 1.5 Flash', desc: 'Fast & lightweight: Quick iterations & high throughput' },
  { id: 'gemini-2.0-flash', name: 'Gemini 2.0 Flash', desc: 'Next-gen multimodal & ultra low latency' },
]

export function AgentsHubView({ onNavigate: _onNavigate }: AgentsHubViewProps) {
  const [agents, setAgents] = useState<AgentDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // Gemini Connection state
  const [connection, setConnection] = useState<AgentConnectionDTO | null>(null)
  const [isConnectionModalOpen, setIsConnectionModalOpen] = useState(false)

  // Filters & Search
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [modelFilter, setModelFilter] = useState('all')

  // Create / Edit Modal state
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [editingAgent, setEditingAgent] = useState<AgentDTO | null>(null)
  const [modalLoading, setModalLoading] = useState(false)
  const [modalError, setModalError] = useState<string | null>(null)

  // Form Fields
  const [formName, setFormName] = useState('')
  const [formRole, setFormRole] = useState('')
  const [formAvatar, setFormAvatar] = useState('🤖')
  const [formModelId, setFormModelId] = useState('gemini-1.5-pro')
  const [formSystemPrompt, setFormSystemPrompt] = useState('')
  const [formStatus, setFormStatus] = useState('active')

  // Toast Helper
  const showToast = (type: 'success' | 'error', message: string) => {
    setNotification({ type, message })
    setTimeout(() => setNotification(null), 4000)
  }

  // Fetch Connection Status
  const fetchConnection = async () => {
    try {
      const res = await fetch('/api/agent-connections?provider=google')
      if (res.ok) {
        const data = await res.json()
        const conns: AgentConnectionDTO[] = data.connections || []
        setConnection(conns.length > 0 ? conns[0] : null)
      }
    } catch {}
  }

  // Fetch Agents
  const fetchAgents = async () => {
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams()
      if (statusFilter !== 'all') params.set('status', statusFilter)
      if (searchQuery.trim()) params.set('search', searchQuery.trim())

      const res = await fetch(`/api/agents?${params.toString()}`)
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: Gagal memuat data agents`)
      }
      const data = await res.json()
      setAgents(data.agents || [])
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan saat memuat katalog agen.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchAgents()
    fetchConnection()
  }, [statusFilter])

  // Open Create Modal
  const handleOpenCreateModal = () => {
    setEditingAgent(null)
    setFormName('')
    setFormRole('Fullstack Architect')
    setFormAvatar('🤖')
    setFormModelId('gemini-1.5-pro')
    setFormSystemPrompt(
      'Anda adalah AI Software Engineering Agent untuk proyek Cuan. Berorientasi pada kepuasan target market, kode bersih, pengujian targeted, dan penuntasan Definition of Done (DoD).'
    )
    setFormStatus('active')
    setModalError(null)
    setIsModalOpen(true)
  }

  // Open Edit Modal
  const handleOpenEditModal = (agent: AgentDTO) => {
    setEditingAgent(agent)
    setFormName(agent.name)
    setFormRole(agent.role || '')
    setFormAvatar(agent.avatar || '🤖')
    setFormModelId(agent.model_id || 'gemini-1.5-pro')
    setFormSystemPrompt(agent.system_prompt || '')
    setFormStatus(agent.status || 'active')
    setModalError(null)
    setIsModalOpen(true)
  }

  // Submit Form (Create or Update)
  const handleFormSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!formName.trim()) {
      setModalError('Nama agen wajib diisi.')
      return
    }

    setModalLoading(true)
    setModalError(null)

    try {
      if (editingAgent) {
        // Update Agent
        const payload: UpdateAgentPayload = {
          name: formName.trim(),
          role: formRole.trim(),
          avatar: formAvatar.trim(),
          model_id: formModelId,
          system_prompt: formSystemPrompt.trim(),
          status: formStatus,
        }

        const res = await fetch(`/api/agents/${editingAgent.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })

        if (!res.ok) {
          const err = await res.json().catch(() => ({}))
          throw new Error(err.error || 'Gagal memperbarui agen')
        }

        const data = await res.json()
        setAgents((prev) => prev.map((a) => (a.id === editingAgent.id ? data.agent : a)))
        showToast('success', `Agen "${data.agent.name}" berhasil diperbarui.`)
      } else {
        // Create Agent
        const payload: CreateAgentPayload = {
          name: formName.trim(),
          role: formRole.trim(),
          avatar: formAvatar.trim(),
          model_id: formModelId,
          provider: 'google',
          system_prompt: formSystemPrompt.trim(),
          status: formStatus,
        }

        const res = await fetch('/api/agents', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })

        if (!res.ok) {
          const err = await res.json().catch(() => ({}))
          throw new Error(err.error || 'Gagal membuat agen')
        }

        const data = await res.json()
        setAgents((prev) => [data.agent, ...prev])
        showToast('success', `Agen "${data.agent.name}" berhasil didaftarkan ke Agent Hub!`)
      }

      setIsModalOpen(false)
    } catch (err: any) {
      setModalError(err.message || 'Terjadi kesalahan saat menyimpan agen.')
    } finally {
      setModalLoading(false)
    }
  }

  // Delete Agent
  const handleDeleteAgent = async (agentId: string) => {
    const target = agents.find((a) => a.id === agentId)
    if (!confirm(`Apakah Anda yakin ingin menghapus persona agen "${target?.name || agentId}"?`)) {
      return
    }

    try {
      const res = await fetch(`/api/agents/${agentId}`, {
        method: 'DELETE',
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || 'Gagal menghapus agen')
      }

      setAgents((prev) => prev.filter((a) => a.id !== agentId))
      showToast('success', 'Agen berhasil dihapus.')
    } catch (err: any) {
      showToast('error', err.message || 'Gagal menghapus agen.')
    }
  }

  // Toggle Status
  const handleToggleStatus = async (agent: AgentDTO, newStatus: string) => {
    try {
      const res = await fetch(`/api/agents/${agent.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: newStatus }),
      })

      if (res.ok) {
        const data = await res.json()
        setAgents((prev) => prev.map((a) => (a.id === agent.id ? data.agent : a)))
        showToast('success', `Status agen "${agent.name}" diubah ke ${newStatus.toUpperCase()}.`)
      }
    } catch {}
  }

  // Filtered List
  const filteredAgents = useMemo(() => {
    return agents.filter((agent) => {
      // Model filter
      if (modelFilter !== 'all' && agent.model_id !== modelFilter) {
        return false
      }
      // Search filter
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        const matchName = agent.name.toLowerCase().includes(q)
        const matchRole = agent.role.toLowerCase().includes(q)
        const matchPrompt = agent.system_prompt.toLowerCase().includes(q)
        if (!matchName && !matchRole && !matchPrompt) {
          return false
        }
      }
      return true
    })
  }, [agents, modelFilter, searchQuery])

  // Stats calculation
  const stats = useMemo(() => {
    const total = agents.length
    const active = agents.filter((a) => a.status === 'active').length
    const proModels = agents.filter((a) => a.model_id.includes('pro')).length
    return { total, active, proModels }
  }, [agents])

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      {/* Toast Notification Banner */}
      {notification && (
        <div
          className={`p-4 rounded-xl flex items-center justify-between text-xs font-semibold shadow-md animate-in slide-in-from-top duration-200 ${
            notification.type === 'success'
              ? 'bg-emerald-500 text-white shadow-emerald-500/20'
              : 'bg-red-500 text-white shadow-red-500/20'
          }`}
        >
          <span>{notification.message}</span>
          <button onClick={() => setNotification(null)} className="opacity-80 hover:opacity-100">
            ✕
          </button>
        </div>
      )}

      {/* Error Alert */}
      {error && (
        <Alert variant="destructive" className="py-2.5">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle className="text-xs font-bold">Kesalahan</AlertTitle>
          <AlertDescription className="text-xs">{error}</AlertDescription>
        </Alert>
      )}

      {/* Top Banner & Header */}
      <div className="p-6 rounded-2xl bg-gradient-to-r from-slate-900 via-indigo-950/60 to-slate-900 border border-indigo-500/30 text-white shadow-xl shadow-indigo-950/20 flex flex-col md:flex-row items-start md:items-center justify-between gap-6">
        <div className="flex items-start gap-4">
          <div className="h-12 w-12 rounded-2xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white flex items-center justify-center shrink-0 shadow-lg shadow-indigo-500/30">
            <Bot className="h-6 w-6" />
          </div>
          <div className="space-y-1">
            <div className="flex items-center gap-2.5 flex-wrap">
              <h1 className="text-lg sm:text-xl font-extrabold tracking-tight text-white">
                Native Agents Hub
              </h1>
              <Badge className="bg-indigo-500/20 text-indigo-300 border-indigo-500/40 text-[10px] py-0 px-2 font-mono">
                Sprint 9 Architecture
              </Badge>
              {connection ? (
                <Badge className="bg-emerald-500/20 text-emerald-300 border-emerald-500/40 text-[10px] py-0 px-2 font-mono flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                  Gemini API Connected
                </Badge>
              ) : (
                <Badge className="bg-amber-500/20 text-amber-300 border-amber-500/40 text-[10px] py-0 px-2 font-mono">
                  Gemini API Belum Terhubung
                </Badge>
              )}
            </div>
            <p className="text-xs sm:text-sm text-slate-300 max-w-2xl leading-relaxed">
              Katalog persona agen AI otonom terintegrasi Google Gemini Pro (BYOK) untuk kolaborasi pengembangan software langsung di Discussion Rooms, Task Boards, dan In-Browser Terminal.
            </p>
          </div>
        </div>

        {/* Header Action Buttons */}
        <div className="flex items-center gap-3 shrink-0 self-end md:self-center flex-wrap">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsConnectionModalOpen(true)}
            className="h-9 text-xs border-indigo-400/40 bg-indigo-950/40 hover:bg-indigo-900/60 text-indigo-200 gap-1.5"
          >
            <KeyRound className="h-3.5 w-3.5 text-indigo-400" />
            <span>{connection ? 'Kelola Gemini Key' : 'Hubungkan Gemini API'}</span>
          </Button>

          <Button
            onClick={handleOpenCreateModal}
            className="h-9 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 gap-1.5"
          >
            <Plus className="h-4 w-4" />
            <span>Daftarkan Agen Baru</span>
          </Button>
        </div>
      </div>

      {/* Stats Summary Row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Total Persona Agen</p>
              <p className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white mt-0.5">
                {stats.total}
              </p>
            </div>
            <div className="h-10 w-10 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 flex items-center justify-center">
              <Bot className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Agen Aktif</p>
              <div className="flex items-center gap-2 mt-0.5">
                <p className="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">
                  {stats.active}
                </p>
                {stats.active > 0 && <span className="h-2.5 w-2.5 rounded-full bg-emerald-500 animate-pulse" />}
              </div>
            </div>
            <div className="h-10 w-10 rounded-xl bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
              <Zap className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Gemini 1.5 Pro</p>
              <p className="text-2xl font-bold tracking-tight text-purple-600 dark:text-purple-400 mt-0.5">
                {stats.proModels}
              </p>
            </div>
            <div className="h-10 w-10 rounded-xl bg-purple-50 dark:bg-purple-950/40 text-purple-600 dark:text-purple-400 flex items-center justify-center">
              <Sparkles className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">AI Provider Key</p>
              <p className="text-xs font-semibold text-slate-800 dark:text-slate-200 mt-2 truncate">
                {connection ? 'AES-256 Encrypted' : 'Belum Dikonfigurasi'}
              </p>
            </div>
            <div className={`h-10 w-10 rounded-xl flex items-center justify-center ${connection ? 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400' : 'bg-slate-100 dark:bg-slate-800 text-slate-400'}`}>
              <ShieldCheck className="h-5 w-5" />
            </div>
          </div>
        </Card>
      </div>

      {/* Filter and Search Toolbar */}
      <div className="p-3.5 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xs flex flex-wrap items-center justify-between gap-3 text-xs">
        {/* Search */}
        <div className="relative flex-1 min-w-[200px] max-w-sm">
          <Search className="h-3.5 w-3.5 absolute left-3 top-2.5 text-slate-400" />
          <Input
            type="text"
            placeholder="Cari agen berdasarkan nama, role, atau prompt..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="h-8 pl-8 text-xs"
          />
        </div>

        {/* Filters */}
        <div className="flex items-center gap-2 flex-wrap">
          {/* Status Filter */}
          <div className="flex items-center gap-1">
            <span className="text-slate-400 text-[11px]">Status:</span>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="h-7 text-xs px-2 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-700 dark:text-slate-300 font-medium"
            >
              <option value="all">Semua Status</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
              <option value="archived">Archived</option>
            </select>
          </div>

          {/* Model Filter */}
          <div className="flex items-center gap-1">
            <span className="text-slate-400 text-[11px]">Model:</span>
            <select
              value={modelFilter}
              onChange={(e) => setModelFilter(e.target.value)}
              className="h-7 text-xs px-2 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-700 dark:text-slate-300 font-medium"
            >
              <option value="all">Semua Model</option>
              <option value="gemini-1.5-pro">Gemini 1.5 Pro</option>
              <option value="gemini-1.5-flash">Gemini 1.5 Flash</option>
              <option value="gemini-2.0-flash">Gemini 2.0 Flash</option>
            </select>
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={fetchAgents}
            className="h-7 text-xs px-2 text-slate-500"
            title="Refresh katalog agen"
          >
            <RefreshCw className={`h-3 w-3 ${loading ? 'animate-spin' : ''}`} />
          </Button>

          {(searchQuery || statusFilter !== 'all' || modelFilter !== 'all') && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setSearchQuery('')
                setStatusFilter('all')
                setModelFilter('all')
              }}
              className="h-7 text-[11px] text-slate-400 hover:text-slate-600 px-2"
            >
              Reset Filter
            </Button>
          )}
        </div>
      </div>

      {/* Agents Grid */}
      {loading ? (
        <div className="p-16 flex flex-col items-center justify-center gap-3">
          <RefreshCw className="h-8 w-8 text-emerald-500 animate-spin" />
          <p className="text-xs text-slate-500">Memuat katalog agen dari server...</p>
        </div>
      ) : filteredAgents.length === 0 ? (
        <div className="p-12 text-center bg-white dark:bg-slate-900 border border-dashed border-slate-200 dark:border-slate-800 rounded-2xl space-y-3">
          <div className="h-12 w-12 rounded-2xl bg-indigo-50 dark:bg-indigo-950/40 text-indigo-500 flex items-center justify-center mx-auto">
            <Bot className="h-6 w-6" />
          </div>
          <div>
            <h3 className="text-sm font-bold text-slate-900 dark:text-white">
              Belum Ada Agen Didaftarkan
            </h3>
            <p className="text-xs text-slate-500 max-w-md mx-auto mt-1">
              Tambahkan persona agen AI baru untuk mengotomasi pengerjaan kode, arsitektur backend, atau pengujian otomatis.
            </p>
          </div>
          <Button
            size="sm"
            onClick={handleOpenCreateModal}
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs gap-1.5"
          >
            <Plus className="h-4 w-4" />
            <span>Daftarkan Agen Pertama</span>
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {filteredAgents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              onEdit={handleOpenEditModal}
              onDelete={handleDeleteAgent}
              onToggleStatus={handleToggleStatus}
            />
          ))}
        </div>
      )}

      {/* Modal Connection API Key */}
      <AgentConnectionModal
        isOpen={isConnectionModalOpen}
        onClose={() => setIsConnectionModalOpen(false)}
        onConnectionUpdated={(updated) => {
          setConnection(updated)
        }}
      />

      {/* Create / Edit Agent Persona Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/70 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
            {/* Modal Header */}
            <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between bg-slate-50 dark:bg-slate-950/50">
              <div className="flex items-center gap-3">
                <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white flex items-center justify-center shadow-md shadow-indigo-500/20 text-lg">
                  {formAvatar}
                </div>
                <div>
                  <h3 className="text-base font-bold text-slate-900 dark:text-white">
                    {editingAgent ? 'Edit Persona Agen' : 'Daftarkan Persona Agen Baru'}
                  </h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    Konfigurasi nama, role, model Google Gemini default, dan system prompt.
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setIsModalOpen(false)}
                className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Modal Form */}
            <form onSubmit={handleFormSubmit} className="p-6 overflow-y-auto space-y-4 flex-1">
              {modalError && (
                <Alert variant="destructive" className="py-2.5">
                  <AlertCircle className="h-4 w-4" />
                  <AlertTitle className="text-xs font-bold">Gagal Menyimpan</AlertTitle>
                  <AlertDescription className="text-xs">{modalError}</AlertDescription>
                </Alert>
              )}

              {/* Avatar Selector */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Avatar / Icon Persona
                </label>
                <div className="flex items-center gap-2 flex-wrap">
                  {AVATAR_OPTIONS.map((emoji) => (
                    <button
                      key={emoji}
                      type="button"
                      onClick={() => setFormAvatar(emoji)}
                      className={`h-9 w-9 rounded-xl text-lg flex items-center justify-center transition-all ${
                        formAvatar === emoji
                          ? 'bg-indigo-500 text-white shadow-md shadow-indigo-500/30 scale-105'
                          : 'bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700'
                      }`}
                    >
                      {emoji}
                    </button>
                  ))}
                </div>
              </div>

              {/* Agent Name */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Nama Agen *
                </label>
                <Input
                  type="text"
                  placeholder="e.g. Cuan Senior Architect, Backend Bot, UI Craftsman"
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  className="text-xs"
                  required
                />
              </div>

              {/* Role & Role Suggestions */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Role / Spesialisasi
                </label>
                <Input
                  type="text"
                  placeholder="e.g. Fullstack Architect, Backend Go Specialist"
                  value={formRole}
                  onChange={(e) => setFormRole(e.target.value)}
                  className="text-xs"
                />
                <div className="flex items-center gap-1.5 flex-wrap pt-1">
                  <span className="text-[10px] text-slate-400">Saran Role:</span>
                  {ROLE_SUGGESTIONS.map((r) => (
                    <button
                      key={r}
                      type="button"
                      onClick={() => setFormRole(r)}
                      className="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-indigo-500/10 hover:text-indigo-500 transition-colors"
                    >
                      +{r}
                    </button>
                  ))}
                </div>
              </div>

              {/* Model Selector */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Model Default (Google Gemini)
                </label>
                <div className="grid grid-cols-1 gap-2">
                  {MODEL_OPTIONS.map((m) => (
                    <label
                      key={m.id}
                      className={`p-3 rounded-xl border flex items-start gap-3 cursor-pointer transition-all ${
                        formModelId === m.id
                          ? 'border-indigo-500 bg-indigo-500/5 dark:bg-indigo-500/10'
                          : 'border-slate-200 dark:border-slate-800 hover:border-slate-300'
                      }`}
                    >
                      <input
                        type="radio"
                        name="model_id"
                        value={m.id}
                        checked={formModelId === m.id}
                        onChange={() => setFormModelId(m.id)}
                        className="mt-0.5 text-indigo-600 focus:ring-indigo-500"
                      />
                      <div className="space-y-0.5 min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-bold text-slate-900 dark:text-white">
                            {m.name}
                          </span>
                          {m.id === 'gemini-1.5-pro' && (
                            <Badge className="bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/30 text-[9px] py-0 px-1 font-mono">
                              Recommended
                            </Badge>
                          )}
                        </div>
                        <p className="text-[11px] text-slate-500 dark:text-slate-400">
                          {m.desc}
                        </p>
                      </div>
                    </label>
                  ))}
                </div>
              </div>

              {/* System Prompt */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  System Prompt (Instruksi Dasar Persona)
                </label>
                <textarea
                  rows={4}
                  value={formSystemPrompt}
                  onChange={(e) => setFormSystemPrompt(e.target.value)}
                  placeholder="Tuliskan arahan persona, batasan kerja, atau panduan arsitektur..."
                  className="w-full text-xs font-mono p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 focus:outline-hidden focus:ring-2 focus:ring-indigo-500 resize-y leading-relaxed"
                />
              </div>

              {/* Status Selector */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Status
                </label>
                <select
                  value={formStatus}
                  onChange={(e) => setFormStatus(e.target.value)}
                  className="h-8 text-xs px-2.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-700 dark:text-slate-300 font-medium w-full"
                >
                  <option value="active">Active (Tersedia untuk pengerjaan task)</option>
                  <option value="inactive">Inactive (Ditangguhkan sementara)</option>
                  <option value="archived">Archived (Diarsipkan)</option>
                </select>
              </div>

              {/* Modal Footer Actions */}
              <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-200 dark:border-slate-800">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setIsModalOpen(false)}
                  className="h-9 text-xs"
                >
                  Batal
                </Button>
                <Button
                  type="submit"
                  disabled={modalLoading || !formName.trim()}
                  className="h-9 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 gap-1.5"
                >
                  {modalLoading ? (
                    <>
                      <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                      <span>Menyimpan...</span>
                    </>
                  ) : (
                    <>
                      <Check className="h-3.5 w-3.5" />
                      <span>{editingAgent ? 'Simpan Perubahan' : 'Daftarkan Agen'}</span>
                    </>
                  )}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
