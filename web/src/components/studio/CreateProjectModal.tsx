import { useState, useEffect } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardFooter } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Alert, AlertDescription, AlertTitle } from '../ui/alert'
import {
  FolderPlus,
  X,
  Sparkles,
  GitBranch,
  Globe,
  Layers,
  Check,
  AlertCircle,
  RefreshCw,
  Box,
  Settings2,
  Rocket,
  Activity,
  MessageSquare
} from 'lucide-react'
import { cn } from '../../lib/utils'

interface CreateProjectModalProps {
  isOpen: boolean
  onClose: () => void
  onCreateProject: (projectData: {
    id: string
    name: string
    type: string
    status: string
    repo_url?: string
    enabled_modules: string[]
  }) => Promise<void> | void
  token?: string | null
}

const PROJECT_TYPES = [
  {
    id: 'saas',
    name: 'SaaS / Web App',
    description: 'Aplikasi modern dengan Docker runtime, API backend, dan frontend SPA.',
    icon: Layers,
    color: 'text-indigo-400',
    defaultModules: ['overview', 'tasks', 'deployer', 'telemetry', 'rooms', 'settings'],
  },
  {
    id: 'wordpress',
    name: 'WordPress Turnkey',
    description: 'WordPress cloud dengan MariaDB terisolasi, WP-CLI, dan AI Webmaster studio.',
    icon: Globe,
    color: 'text-blue-400',
    defaultModules: ['overview', 'wordpress', 'tasks', 'deployer', 'telemetry', 'settings'],
  },
  {
    id: 'creator',
    name: 'Creator Hub',
    description: 'Pipeline multi-channel content, publishing automation, dan research agents.',
    icon: Sparkles,
    color: 'text-purple-400',
    defaultModules: ['overview', 'tasks', 'rooms', 'telemetry', 'settings'],
  },
  {
    id: 'blank',
    name: 'Blank Custom Project',
    description: 'Workspace bersih tanpa template bawaan, siap dikonfigurasi sesuai kebutuhan.',
    icon: Box,
    color: 'text-emerald-400',
    defaultModules: ['overview', 'tasks', 'repos', 'deployer', 'rooms', 'settings'],
  },
]

const AVAILABLE_MODULES = [
  { id: 'overview', name: 'Overview', icon: Layers, desc: 'Ringkasan & metrik' },
  { id: 'wordpress', name: 'WordPress Cloud', icon: Globe, desc: 'Turnkey WP deployment' },
  { id: 'repos', name: 'Repos', icon: GitBranch, desc: 'Git sync & branches' },
  { id: 'deployer', name: 'Deployments', icon: Rocket, desc: '1-Click script & container' },
  { id: 'telemetry', name: 'Telemetry', icon: Activity, desc: 'Observability & health' },
  { id: 'rooms', name: 'Studio Rooms', icon: MessageSquare, desc: 'Agent & human chat' },
  { id: 'settings', name: 'Settings', icon: Settings2, desc: 'Domains, servers & env' },
]

export function CreateProjectModal({
  isOpen,
  onClose,
  onCreateProject,
  token,
}: CreateProjectModalProps) {
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [selectedType, setSelectedType] = useState('saas')
  const [repoUrl, setRepoUrl] = useState('')
  const [selectedModules, setSelectedModules] = useState<string[]>([
    'overview',
    'tasks',
    'deployer',
    'telemetry',
    'rooms',
    'settings',
  ])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (isOpen) {
      setName('')
      setSlug('')
      setSelectedType('saas')
      setRepoUrl('')
      setSelectedModules(['overview', 'tasks', 'deployer', 'telemetry', 'rooms', 'settings'])
      setError(null)
      setLoading(false)
    }
  }, [isOpen])

  // Auto-slug generator from name
  const handleNameChange = (val: string) => {
    setName(val)
    const generatedSlug = val
      .toLowerCase()
      .trim()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
    setSlug(generatedSlug)
  }

  const handleTypeSelect = (typeId: string) => {
    setSelectedType(typeId)
    const matched = PROJECT_TYPES.find((t) => t.id === typeId)
    if (matched) {
      setSelectedModules(matched.defaultModules)
    }
  }

  const toggleModule = (modId: string) => {
    if (modId === 'overview') return // Overview is always required
    setSelectedModules((prev) =>
      prev.includes(modId) ? prev.filter((m) => m !== modId) : [...prev, modId]
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (!name.trim()) {
      setError('Nama project wajib diisi.')
      return
    }

    const finalSlug = slug.trim() || name.toLowerCase().replace(/[^a-z0-9]+/g, '-')
    const finalId = finalSlug || `project-${Date.now()}`

    setLoading(true)
    try {
      // If repo_url is provided, attempt backend registration
      if (repoUrl.trim()) {
        try {
          await fetch('/api/v1/projects', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              ...(token ? { Authorization: `Bearer ${token}` } : {}),
            },
            body: JSON.stringify({
              repo_url: repoUrl.trim(),
              project_name: name.trim(),
            }),
          })
        } catch {
          // Fallback gracefully if backend is in offline/demo mode
        }
      }

      await onCreateProject({
        id: finalId,
        name: name.trim(),
        type: selectedType,
        status: 'active',
        repo_url: repoUrl.trim() || undefined,
        enabled_modules: selectedModules,
      })

      onClose()
    } catch (err: any) {
      setError(err.message || 'Gagal membuat project baru.')
    } finally {
      setLoading(false)
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in-50">
      <Card className="w-full max-w-2xl bg-card border-border text-card-foreground shadow-2xl overflow-hidden max-h-[90vh] flex flex-col">
        {/* Header */}
        <CardHeader className="flex flex-row items-center justify-between border-b border-border p-4 sm:p-5">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center bg-primary/10 text-primary border border-primary/20">
              <FolderPlus className="h-5 w-5" />
            </div>
            <div>
              <CardTitle className="text-base font-bold text-foreground">
                Create New Project
              </CardTitle>
              <CardDescription className="text-xs text-muted-foreground">
                Inisialisasi workspace terpadu untuk repository, deployment, dan automasi tim.
              </CardDescription>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 text-muted-foreground hover:text-foreground transition-colors"
            title="Tutup dialog"
          >
            <X className="h-4 w-4" />
          </button>
        </CardHeader>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-5">
          {error && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>Kesalahan</AlertTitle>
              <AlertDescription className="text-xs">{error}</AlertDescription>
            </Alert>
          )}

          {/* Project Name & Identifier */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-foreground flex items-center justify-between">
                <span>Nama Proyek *</span>
              </label>
              <Input
                type="text"
                placeholder="misal: Cuan Platform Core"
                value={name}
                onChange={(e) => handleNameChange(e.target.value)}
                required
                autoFocus
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-foreground flex items-center justify-between">
                <span>Slug / Identifier</span>
                <span className="text-[10px] text-muted-foreground font-mono">auto-generated</span>
              </label>
              <Input
                type="text"
                placeholder="cuan-platform-core"
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
              />
            </div>
          </div>

          {/* Project Type Selector */}
          <div className="space-y-2">
            <label className="text-xs font-semibold text-foreground">
              Tipe Workspace & Blueprint
            </label>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
              {PROJECT_TYPES.map((type) => {
                const IconComponent = type.icon
                const isSelected = selectedType === type.id
                return (
                  <button
                    key={type.id}
                    type="button"
                    onClick={() => handleTypeSelect(type.id)}
                    className={cn(
                      'flex items-start gap-3 p-3 text-left border transition-all',
                      isSelected
                        ? 'border-primary bg-primary/10 ring-1 ring-primary'
                        : 'border-border bg-card/50 hover:bg-muted/50'
                    )}
                  >
                    <div className={cn('p-2 shrink-0 bg-background border border-border/80', type.color)}>
                      <IconComponent className="h-4 w-4" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between">
                        <span className="font-semibold text-xs text-foreground">{type.name}</span>
                        {isSelected && <Check className="h-3.5 w-3.5 text-primary shrink-0" />}
                      </div>
                      <p className="text-[11px] text-muted-foreground line-clamp-2 mt-0.5 leading-snug">
                        {type.description}
                      </p>
                    </div>
                  </button>
                )
              })}
            </div>
          </div>

          {/* Optional Repository URL */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-foreground flex items-center gap-1.5">
              <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
              <span>GitHub / Git Repository URL (Opsional)</span>
            </label>
            <Input
              type="text"
              placeholder="https://github.com/organization/my-repo"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
            />
            <p className="text-[10px] text-muted-foreground">
              Dapat dikosongkan untuk inisialisasi workspace lokal atau menghubungkan repositori nanti.
            </p>
          </div>

          {/* Contextual Modules Selection */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-xs font-semibold text-foreground flex items-center gap-1.5">
                <Settings2 className="h-3.5 w-3.5 text-muted-foreground" />
                <span>Modul Kontekstual Aktif</span>
              </label>
              <span className="text-[10px] text-muted-foreground">
                {selectedModules.length} modul aktif
              </span>
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {AVAILABLE_MODULES.map((mod) => {
                const ModIcon = mod.icon
                const isChecked = selectedModules.includes(mod.id)
                return (
                  <button
                    key={mod.id}
                    type="button"
                    onClick={() => toggleModule(mod.id)}
                    className={cn(
                      'flex flex-col items-start p-2 border text-left transition-colors',
                      isChecked
                        ? 'border-primary/50 bg-primary/5 text-foreground'
                        : 'border-border/60 bg-muted/20 text-muted-foreground hover:bg-muted/40'
                    )}
                  >
                    <div className="flex items-center justify-between w-full mb-1">
                      <ModIcon className={cn('h-3.5 w-3.5', isChecked ? 'text-primary' : 'text-muted-foreground')} />
                      {isChecked && <Check className="h-3 w-3 text-primary" />}
                    </div>
                    <span className="text-[11px] font-medium leading-tight">{mod.name}</span>
                    <span className="text-[9px] text-muted-foreground truncate w-full mt-0.5">
                      {mod.desc}
                    </span>
                  </button>
                )
              })}
            </div>
          </div>
        </form>

        {/* Footer */}
        <CardFooter className="flex items-center justify-end gap-2.5 border-t border-border p-4 bg-muted/20">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={loading}
            className="text-xs"
          >
            Batal
          </Button>
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={loading || !name.trim()}
            className="bg-primary text-primary-foreground hover:bg-primary/90 text-xs font-semibold gap-2"
          >
            {loading ? (
              <>
                <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                Membuat Proyek...
              </>
            ) : (
              <>
                <FolderPlus className="h-3.5 w-3.5" />
                Buat Proyek Sekarang
              </>
            )}
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}
