import { useState, useMemo } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import {
  FolderGit2,
  Archive,
  ArchiveRestore,
  Search,
  Plus,
  GitBranch,
  ChevronRight
} from 'lucide-react'
import { ProjectItemDTO } from './types'
import { cn } from '../../lib/utils'

interface ProjectsDirectoryViewProps {
  projects: ProjectItemDTO[]
  activeProjectId: string
  onSelectProject: (id: string) => void
  onArchiveProject: (id: string, name: string) => Promise<void>
  onUnarchiveProject: (id: string, name: string) => Promise<void>
  onCreateProjectClick: () => void
  onNavigate?: (path: string) => void
}

export function ProjectsDirectoryView({
  projects,
  activeProjectId,
  onSelectProject,
  onArchiveProject,
  onUnarchiveProject,
  onCreateProjectClick,
  onNavigate,
}: ProjectsDirectoryViewProps) {
  const [activeTab, setActiveTab] = useState<'active' | 'archived'>('active')
  const [searchQuery, setSearchQuery] = useState('')
  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)

  // Split projects into Active and Archived
  const activeProjects = useMemo(() => {
    return projects.filter((p) => !p.is_archived && p.status !== 'archived')
  }, [projects])

  const archivedProjects = useMemo(() => {
    return projects.filter((p) => p.is_archived || p.status === 'archived')
  }, [projects])

  const displayedList = useMemo(() => {
    const list = activeTab === 'active' ? activeProjects : archivedProjects
    if (!searchQuery.trim()) return list
    const q = searchQuery.toLowerCase()
    return list.filter(
      (p) =>
        p.name.toLowerCase().includes(q) ||
        p.id.toLowerCase().includes(q) ||
        (p.repo_url && p.repo_url.toLowerCase().includes(q)) ||
        (p.project_type && p.project_type.toLowerCase().includes(q))
    )
  }, [activeTab, activeProjects, archivedProjects, searchQuery])

  const handleArchive = async (id: string, name: string) => {
    setActionLoadingId(id)
    try {
      await onArchiveProject(id, name)
    } finally {
      setActionLoadingId(null)
    }
  }

  const handleUnarchive = async (id: string, name: string) => {
    setActionLoadingId(id)
    try {
      await onUnarchiveProject(id, name)
    } finally {
      setActionLoadingId(null)
    }
  }

  const getProjectTypeBadge = (type?: string) => {
    switch (type) {
      case 'wordpress':
        return (
          <Badge variant="outline" className="text-[10px] font-mono border-blue-500/30 text-blue-500 bg-blue-500/10">
            WordPress
          </Badge>
        )
      case 'creator':
      case 'blog':
      case 'youtube':
      case 'olshop':
      case 'affiliate':
        return (
          <Badge variant="outline" className="text-[10px] font-mono border-purple-500/30 text-purple-500 bg-purple-500/10 capitalize">
            {type}
          </Badge>
        )
      case 'saas':
        return (
          <Badge variant="outline" className="text-[10px] font-mono border-indigo-500/30 text-indigo-500 bg-indigo-500/10">
            SaaS
          </Badge>
        )
      default:
        return (
          <Badge variant="outline" className="text-[10px] font-mono border-emerald-500/30 text-emerald-500 bg-emerald-500/10">
            Git Project
          </Badge>
        )
    }
  }

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      {/* Header Banner */}
      <div className="p-6 rounded-2xl bg-card text-card-foreground border border-border shadow-xs space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-3.5">
            <div className="h-12 w-12 rounded-2xl bg-primary/10 text-primary border border-primary/20 flex items-center justify-center shrink-0">
              <FolderGit2 className="h-6 w-6" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-lg font-bold tracking-tight text-foreground">
                  Direktori Proyek & Workspace
                </h1>
                <Badge variant="outline" className="text-[10px] font-mono border-primary/30 text-primary">
                  Active vs Archived
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground mt-0.5">
                Kelola proyek aktif dan proyek terarsip. Proyek terarsip otomatis disembunyikan dari dropdown navigasi dan switcher workspace aktif.
              </p>
            </div>
          </div>

          <Button
            size="sm"
            onClick={onCreateProjectClick}
            className="bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold gap-1.5 shadow-sm"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>+ Hubungkan Proyek</span>
          </Button>
        </div>

        {/* Tab Switcher & Search Bar */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 pt-3 border-t border-border">
          {/* Active vs Archived Tab Switcher */}
          <div className="flex items-center rounded-xl bg-muted p-1 gap-1 text-xs font-semibold">
            <button
              type="button"
              onClick={() => {
                setActiveTab('active')
                setSearchQuery('')
              }}
              className={cn(
                'flex items-center gap-2 px-3.5 py-1.5 rounded-lg transition-all',
                activeTab === 'active'
                  ? 'bg-card text-foreground shadow-xs font-bold'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <span>Proyek Aktif</span>
              <span
                className={cn(
                  'px-1.5 py-0.2 rounded-full text-[10px] font-mono',
                  activeTab === 'active'
                    ? 'bg-primary/10 text-primary'
                    : 'bg-muted text-muted-foreground'
                )}
              >
                {activeProjects.length}
              </span>
            </button>

            <button
              type="button"
              onClick={() => {
                setActiveTab('archived')
                setSearchQuery('')
              }}
              className={cn(
                'flex items-center gap-2 px-3.5 py-1.5 rounded-lg transition-all',
                activeTab === 'archived'
                  ? 'bg-card text-foreground shadow-xs font-bold'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <Archive className="h-3.5 w-3.5 text-muted-foreground" />
              <span>Proyek Terarsip</span>
              <span
                className={cn(
                  'px-1.5 py-0.2 rounded-full text-[10px] font-mono',
                  activeTab === 'archived'
                    ? 'bg-zinc-500/20 text-zinc-300 font-bold'
                    : 'bg-muted text-muted-foreground'
                )}
              >
                {archivedProjects.length}
              </span>
            </button>
          </div>

          {/* Quick Search */}
          <div className="relative w-full sm:w-64">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
            <Input
              type="text"
              placeholder={`Cari proyek ${activeTab === 'active' ? 'aktif' : 'terarsip'}...`}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-8 h-8 text-xs bg-muted/40 border-border"
            />
          </div>
        </div>
      </div>

      {/* Projects Grid */}
      {displayedList.length === 0 ? (
        <div className="p-12 text-center bg-card text-card-foreground rounded-2xl border border-border shadow-xs space-y-3">
          <div className="h-12 w-12 rounded-2xl bg-muted text-muted-foreground flex items-center justify-center mx-auto">
            {activeTab === 'active' ? <FolderGit2 className="h-6 w-6" /> : <Archive className="h-6 w-6" />}
          </div>
          <div className="max-w-md mx-auto space-y-1">
            <h3 className="text-sm font-bold text-slate-800 dark:text-slate-200">
              {searchQuery
                ? 'Tidak ada proyek yang sesuai dengan pencarian'
                : activeTab === 'active'
                ? 'Belum ada proyek aktif'
                : 'Belum ada proyek yang diarsipkan'}
            </h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              {searchQuery
                ? 'Coba ganti kata kunci pencarian Anda.'
                : activeTab === 'active'
                ? 'Hubungkan repository GitHub atau buat proyek baru untuk memulai.'
                : 'Proyek yang tidak lagi aktif dapat diarsipkan agar tidak memenuhi menu navigasi.'}
            </p>
          </div>
          {searchQuery ? (
            <Button variant="outline" size="sm" onClick={() => setSearchQuery('')} className="text-xs">
              Reset Pencarian
            </Button>
          ) : activeTab === 'active' ? (
            <Button size="sm" onClick={onCreateProjectClick} className="text-xs bg-emerald-600 hover:bg-emerald-500 text-white">
              + Hubungkan Proyek Baru
            </Button>
          ) : null}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {displayedList.map((p) => {
            const isSelected = activeProjectId === p.id
            const isArchived = p.is_archived || p.status === 'archived'
            const isLoading = actionLoadingId === p.id

            return (
              <Card
                key={p.id}
                className={cn(
                  'p-5 bg-card text-card-foreground border transition-all flex flex-col justify-between space-y-4 rounded-2xl shadow-xs',
                  isSelected && !isArchived
                    ? 'border-primary ring-1 ring-primary/30'
                    : 'border-border hover:border-border/80',
                  isArchived && 'bg-muted/40 opacity-90'
                )}
              >
                {/* Card Top: Header & Badges */}
                <div className="space-y-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2.5 truncate">
                      <div
                        className={cn(
                          'p-2 rounded-xl shrink-0 border',
                          isArchived
                            ? 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20'
                            : isSelected
                            ? 'bg-primary/15 text-primary border-primary/30'
                            : 'bg-muted text-muted-foreground border-border'
                        )}
                      >
                        {isArchived ? <Archive className="h-4 w-4" /> : <FolderGit2 className="h-4 w-4" />}
                      </div>
                      <div className="truncate">
                        <h4 className="text-xs font-bold text-foreground truncate" title={p.name}>
                          {p.name}
                        </h4>
                        <span className="text-[10px] font-mono text-muted-foreground block truncate">{p.id}</span>
                      </div>
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0">
                      {isArchived ? (
                        <Badge variant="outline" className="text-[10px] font-mono border-zinc-500/40 text-zinc-400 bg-zinc-500/10 uppercase">
                          Arsip
                        </Badge>
                      ) : (
                        getProjectTypeBadge(p.project_type)
                      )}
                    </div>
                  </div>

                  {/* Repo URL / Branch */}
                  <div className="p-2.5 rounded-xl bg-muted/40 border border-border space-y-1.5 text-[11px]">
                    <div className="flex items-center justify-between text-muted-foreground">
                      <span className="flex items-center gap-1 truncate font-mono">
                        <GitBranch className="h-3 w-3 shrink-0" />
                        <span className="truncate">{p.default_branch || 'main'}</span>
                      </span>
                      {p.clone_status && (
                        <span
                          className={cn(
                            'text-[9px] font-mono uppercase px-1 py-0.2 rounded-xs border',
                            p.clone_status === 'synced'
                              ? 'border-primary/30 text-primary bg-primary/10'
                              : 'border-border text-muted-foreground'
                          )}
                        >
                          {p.clone_status}
                        </span>
                      )}
                    </div>
                    {p.repo_url && (
                      <p className="text-[10px] font-mono text-muted-foreground truncate" title={p.repo_url}>
                        {p.repo_url.replace('https://github.com/', '')}
                      </p>
                    )}
                  </div>

                  {/* Enabled Modules */}
                  {p.enabled_modules && p.enabled_modules.length > 0 && (
                    <div className="flex flex-wrap gap-1">
                      {p.enabled_modules.map((m) => (
                        <span
                          key={m}
                          className="text-[9px] font-mono text-muted-foreground bg-muted px-1.5 py-0.5 rounded"
                        >
                          {m}
                        </span>
                      ))}
                    </div>
                  )}
                </div>

                {/* Card Actions */}
                <div className="pt-3 border-t border-border flex items-center justify-between gap-2">
                  {isArchived ? (
                    /* Archived Action: Restore / Unarchive */
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={isLoading}
                      onClick={() => handleUnarchive(p.id, p.name)}
                      className="w-full text-xs font-semibold border-primary/40 text-primary hover:bg-primary/10 gap-1.5"
                    >
                      <ArchiveRestore className="h-3.5 w-3.5" />
                      <span>{isLoading ? 'Memulihkan...' : 'Pulihkan ke Aktif'}</span>
                    </Button>
                  ) : (
                    /* Active Actions: Switch / Open Workspace & Archive Button */
                    <>
                      <Button
                        type="button"
                        size="sm"
                        variant={isSelected ? 'default' : 'outline'}
                        onClick={() => {
                          onSelectProject(p.id)
                          if (onNavigate) {
                            onNavigate('/tasks')
                          }
                        }}
                        className={cn(
                          'text-xs font-semibold flex-1 gap-1',
                          isSelected
                            ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                            : 'text-foreground'
                        )}
                      >
                        <span>{isSelected ? 'Workspace Aktif' : 'Pilih Workspace'}</span>
                        <ChevronRight className="h-3.5 w-3.5" />
                      </Button>

                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        disabled={isLoading}
                        onClick={() => handleArchive(p.id, p.name)}
                        className="text-xs text-muted-foreground hover:text-amber-500 hover:bg-amber-500/10 gap-1 px-2.5"
                        title="Arsipkan proyek (sembunyikan dari navbar dan switcher)"
                      >
                        <Archive className="h-3.5 w-3.5" />
                        <span className="hidden sm:inline">Arsipkan</span>
                      </Button>
                    </>
                  )}
                </div>
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
