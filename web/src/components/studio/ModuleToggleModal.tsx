import { useState, useEffect } from 'react'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import {
  X,
  Kanban,
  MessageSquare,
  FolderGit2,
  Server,
  Globe,
  HelpCircle,
  Sparkles,
  Sliders,
  Rocket
} from 'lucide-react'

export interface ModuleItem {
  id: string
  name: string
  description: string
  icon: any
  color: string
  badge?: string
}

export const AVAILABLE_MODULES: ModuleItem[] = [
  {
    id: 'tasks',
    name: 'Tasks & Kanban',
    description: 'Papan kerja interaktif, otomatisasi DoD checklist, dan eksekusi AI Agent langsung di kartu task.',
    icon: Kanban,
    color: 'text-indigo-500 bg-indigo-500/10 border-indigo-500/20',
  },
  {
    id: 'deployer',
    name: '1-Click Script Deployer',
    description: 'Automated database provisioning, AES-256-GCM vault, build runner pipeline, dan live supervisor console.',
    icon: Rocket,
    color: 'text-cyan-500 bg-cyan-500/10 border-cyan-500/20',
    badge: 'Deployer',
  },
  {
    id: 'discussions',
    name: 'Discussion Rooms',
    description: 'Ruang kolaborasi tim & agen AI dengan live streaming SSE dan fitur distill task otomatis.',
    icon: MessageSquare,
    color: 'text-emerald-500 bg-emerald-500/10 border-emerald-500/20',
  },
  {
    id: 'repositories',
    name: 'Git Repositories',
    description: 'Koneksi repositori Git/GitHub remote, inspeksi branch, dan sinkronisasi source code workspace.',
    icon: FolderGit2,
    color: 'text-purple-500 bg-purple-500/10 border-purple-500/20',
  },
  {
    id: 'servers',
    name: 'Servers (BYOS)',
    description: 'Koneksi VPS mandiri (Bring Your Own Server), isolasi deployment, domain kustom & observabilitas.',
    icon: Server,
    color: 'text-blue-500 bg-blue-500/10 border-blue-500/20',
  },
  {
    id: 'publishing',
    name: 'Publishing Hub',
    description: '1-Click publish artikel ke WordPress REST API dan ekspor konten multi-format (Markdown/HTML).',
    icon: Globe,
    color: 'text-amber-500 bg-amber-500/10 border-amber-500/20',
    badge: 'Creator',
  },
  {
    id: 'support',
    name: 'Support Hub',
    description: 'Pusat investigasi tiket pengguna, diagnosa lisensi, aktivasi produk, dan customer assistance.',
    icon: HelpCircle,
    color: 'text-rose-500 bg-rose-500/10 border-rose-500/20',
    badge: 'Ops',
  },
]

interface ModuleToggleModalProps {
  isOpen: boolean
  onClose: () => void
  projectId?: string
  projectName?: string
  enabledModules?: string[]
  currentModules?: string[]
  onSaveModules: (newModules: string[]) => Promise<void> | void
}

export function ModuleToggleModal({
  isOpen,
  onClose,
  projectId,
  projectName = 'Proyek Aktif',
  enabledModules,
  currentModules,
  onSaveModules,
}: ModuleToggleModalProps) {
  const initialModules = enabledModules || currentModules || ['tasks', 'discussions']
  const [selectedModules, setSelectedModules] = useState<string[]>(initialModules)
  const [isSaving, setIsSaving] = useState(false)
  const [feedback, setFeedback] = useState<string | null>(null)

  useEffect(() => {
    setSelectedModules(enabledModules || currentModules || ['tasks', 'discussions'])
  }, [enabledModules, currentModules, isOpen])

  if (!isOpen) return null

  const handleToggle = (moduleId: string) => {
    // Keep at least 1 module active
    if (selectedModules.includes(moduleId)) {
      if (selectedModules.length <= 1) {
        setFeedback('Minimal satu modul harus tetap aktif.')
        setTimeout(() => setFeedback(null), 3000)
        return
      }
      setSelectedModules(selectedModules.filter((m) => m !== moduleId))
    } else {
      setSelectedModules([...selectedModules, moduleId])
    }
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await onSaveModules(selectedModules)
      onClose()
    } catch (err: any) {
      setFeedback(err.message || 'Gagal menyimpan perubahan modul.')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl w-full max-w-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between bg-gradient-to-r from-emerald-500/10 via-teal-500/5 to-slate-50 dark:to-slate-900">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-emerald-500 text-slate-950 font-bold flex items-center justify-center shadow-lg shadow-emerald-500/20">
              <Sliders className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-lg font-black text-slate-900 dark:text-white tracking-tight">
                  Modular Workspace OS
                </h2>
                <Badge className="bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30 text-[10px] font-mono py-0 px-2">
                  {selectedModules.length} Modul Aktif
                </Badge>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Atur modul workspace untuk <strong className="text-slate-700 dark:text-slate-200">{projectName}</strong>{projectId ? ` (${projectId})` : ''}. Sidebar dan navigasi beradaptasi otomatis.
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-xl hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Feedback Alert */}
        {feedback && (
          <div className="px-6 pt-4">
            <div className="p-3 text-xs rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-700 dark:text-amber-300">
              {feedback}
            </div>
          </div>
        )}

        {/* Modules Grid */}
        <div className="p-6 overflow-y-auto space-y-3 flex-1">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {AVAILABLE_MODULES.map((mod) => {
              const Icon = mod.icon
              const isEnabled = selectedModules.includes(mod.id)
              return (
                <div
                  key={mod.id}
                  onClick={() => handleToggle(mod.id)}
                  className={`p-4 rounded-2xl border transition-all cursor-pointer relative flex flex-col justify-between gap-3 ${
                    isEnabled
                      ? 'bg-emerald-500/5 dark:bg-emerald-950/20 border-emerald-500/40 shadow-xs'
                      : 'bg-slate-50/60 dark:bg-slate-950/40 border-slate-200 dark:border-slate-800/80 opacity-75 hover:opacity-100'
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2.5">
                      <div className={`h-9 w-9 rounded-xl flex items-center justify-center border ${mod.color}`}>
                        <Icon className="h-4 w-4" />
                      </div>
                      <div>
                        <div className="flex items-center gap-1.5">
                          <span className="text-xs font-bold text-slate-900 dark:text-white">
                            {mod.name}
                          </span>
                          {mod.badge && (
                            <span className="text-[9px] font-mono uppercase px-1.5 py-0.5 rounded-full bg-slate-200 dark:bg-slate-800 text-slate-600 dark:text-slate-400">
                              {mod.badge}
                            </span>
                          )}
                        </div>
                        <span className="text-[10px] text-slate-400 font-mono">module: {mod.id}</span>
                      </div>
                    </div>

                    <div
                      className={`h-5 w-9 rounded-full transition-colors relative flex items-center px-0.5 ${
                        isEnabled ? 'bg-emerald-500' : 'bg-slate-300 dark:bg-slate-700'
                      }`}
                    >
                      <div
                        className={`h-4 w-4 rounded-full bg-white shadow-xs transition-transform transform ${
                          isEnabled ? 'translate-x-4' : 'translate-x-0'
                        }`}
                      />
                    </div>
                  </div>

                  <p className="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
                    {mod.description}
                  </p>
                </div>
              )
            })}
          </div>

          <div className="p-3.5 rounded-2xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-xs text-slate-500 dark:text-slate-400 flex items-center gap-2.5">
            <Sparkles className="h-4 w-4 text-emerald-500 shrink-0" />
            <span>
              Preset cepat: <strong>SaaS/Dojo</strong> (tasks, discussions, repos, servers) &bull; <strong>Creator</strong> (tasks, discussions, publishing) &bull; <strong>Blank</strong> (tasks, discussions).
            </span>
          </div>
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 flex items-center justify-between">
          <div className="flex items-center gap-1.5">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedModules(['tasks', 'discussions', 'repositories', 'servers'])}
              className="text-[11px] h-7 px-2"
            >
              Preset SaaS
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedModules(['tasks', 'discussions', 'publishing'])}
              className="text-[11px] h-7 px-2"
            >
              Preset Creator
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedModules(['tasks', 'discussions'])}
              className="text-[11px] h-7 px-2"
            >
              Preset Blank
            </Button>
          </div>

          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={onClose} disabled={isSaving}>
              Batal
            </Button>
            <Button
              size="sm"
              onClick={handleSave}
              disabled={isSaving}
              className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold gap-1.5"
            >
              {isSaving ? 'Menyimpan...' : 'Terapkan Modul'}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
