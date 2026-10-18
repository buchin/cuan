import { useState, useRef, useEffect, useMemo } from 'react'
import {
  FolderGit2,
  ChevronDown,
  Search,
  Check,
  Plus,
  Layers,
  Globe,
  Sparkles
} from 'lucide-react'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { cn } from '../../lib/utils'
import { ProjectOption } from './Sidebar07Layout'

interface BreadcrumbProjectSwitcherProps {
  currentProject: ProjectOption
  projects: ProjectOption[]
  onSelectProject: (project: ProjectOption) => void
  onCreateProjectClick?: () => void
}

export function BreadcrumbProjectSwitcher({
  currentProject,
  projects,
  onSelectProject,
  onCreateProjectClick,
}: BreadcrumbProjectSwitcherProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Close dropdown on click outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [isOpen])

  // Reset search when opening
  useEffect(() => {
    if (isOpen) {
      setSearchQuery('')
    }
  }, [isOpen])

  // Filtered projects: Project terarsip otomatis tidak muncul di breadcrumb switcher navbar
  const filteredProjects = useMemo(() => {
    return projects.filter((p) => {
      const isArchived = p.status === 'archived' || p.is_archived === true
      if (isArchived) return false

      const matchesSearch =
        p.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        p.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (p.type && p.type.toLowerCase().includes(searchQuery.toLowerCase()))

      return matchesSearch
    })
  }, [projects, searchQuery])

  const getProjectIcon = (type?: string) => {
    switch (type) {
      case 'wordpress':
        return <Globe className="h-3.5 w-3.5 text-blue-400 shrink-0" />
      case 'creator':
        return <Sparkles className="h-3.5 w-3.5 text-purple-400 shrink-0" />
      case 'saas':
        return <Layers className="h-3.5 w-3.5 text-indigo-400 shrink-0" />
      default:
        return <FolderGit2 className="h-3.5 w-3.5 text-primary shrink-0" />
    }
  }

  const handleSelect = (proj: ProjectOption) => {
    onSelectProject(proj)
    setIsOpen(false)
  }

  return (
    <div className="relative inline-block text-left" ref={dropdownRef}>
      {/* Trigger Button */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          'flex items-center gap-2 px-2 py-1 text-xs font-medium rounded-md border transition-all select-none',
          isOpen
            ? 'border-primary bg-primary/10 text-foreground ring-1 ring-primary/40'
            : 'border-border/80 bg-card hover:bg-muted/80 text-foreground'
        )}
        title="Ganti workspace project secara 1-klik"
        aria-haspopup="true"
        aria-expanded={isOpen}
      >
        <div className="flex items-center gap-1.5 truncate max-w-[140px] sm:max-w-[200px] md:max-w-[260px]">
          {getProjectIcon(currentProject.type)}
          <span className="truncate font-semibold text-xs text-foreground">
            {currentProject.name}
          </span>
        </div>

        {/* Status Badge Indicator */}
        <span
          className={cn(
            'hidden sm:inline-flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.2 border rounded-xs uppercase',
            currentProject.status === 'archived'
              ? 'border-zinc-500/40 bg-zinc-500/10 text-zinc-400'
              : 'border-emerald-500/40 bg-emerald-500/10 text-emerald-400'
          )}
        >
          <span
            className={cn(
              'h-1.5 w-1.5 rounded-full',
              currentProject.status === 'archived' ? 'bg-zinc-400' : 'bg-emerald-400 animate-pulse'
            )}
          />
          {currentProject.status === 'archived' ? 'Arsip' : 'Active'}
        </span>

        <ChevronDown
          className={cn(
            'h-3.5 w-3.5 text-muted-foreground transition-transform duration-200',
            isOpen && 'rotate-180 text-foreground'
          )}
        />
      </button>

      {/* Popover Switcher Menu */}
      {isOpen && (
        <div className="absolute left-0 top-full mt-1.5 z-50 w-72 sm:w-80 bg-popover text-popover-foreground border border-border shadow-2xl rounded-md p-1.5 animate-in fade-in-50 zoom-in-95">
          {/* Header & Search */}
          <div className="p-1 space-y-2 border-b border-border/80 pb-2">
            <div className="flex items-center justify-between px-1">
              <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                Ganti Workspace Proyek
              </span>
              <span className="text-[10px] text-muted-foreground font-mono">
                {filteredProjects.length} Aktif
              </span>
            </div>

            {/* Quick Filter Search Input */}
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                type="text"
                placeholder="Cari proyek aktif (nama, ID, tipe)..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="h-8 pl-8 pr-3 text-xs bg-muted/40 border-border/80 focus-visible:ring-1 focus-visible:ring-primary"
                autoFocus
              />
            </div>
          </div>

          {/* Project List */}
          <div className="max-h-64 overflow-y-auto py-1 space-y-0.5 divide-y divide-border/40">
            {filteredProjects.length === 0 ? (
              <div className="p-4 text-center text-xs text-muted-foreground space-y-1">
                <p>Tidak ada proyek yang sesuai.</p>
                {searchQuery && (
                  <button
                    type="button"
                    onClick={() => setSearchQuery('')}
                    className="text-[11px] text-primary underline"
                  >
                    Reset pencarian
                  </button>
                )}
              </div>
            ) : (
              filteredProjects.map((proj) => {
                const isSelected = currentProject.id === proj.id
                const isArchived = proj.status === 'archived'

                return (
                  <button
                    key={proj.id}
                    type="button"
                    onClick={() => handleSelect(proj)}
                    className={cn(
                      'flex w-full items-center justify-between gap-2.5 px-2.5 py-2 text-xs transition-colors text-left rounded-xs',
                      isSelected
                        ? 'bg-primary/10 text-primary font-semibold'
                        : 'hover:bg-muted/70 text-foreground'
                    )}
                  >
                    <div className="flex items-center gap-2 truncate">
                      <div className="p-1 rounded-xs bg-background/80 border border-border/80">
                        {getProjectIcon(proj.type)}
                      </div>
                      <div className="grid text-left truncate leading-tight">
                        <span className="truncate font-medium text-xs text-foreground">
                          {proj.name}
                        </span>
                        <div className="flex items-center gap-1.5 text-[10px] text-muted-foreground font-mono">
                          <span className="uppercase">{proj.type || 'workspace'}</span>
                          {isArchived && (
                            <span className="text-zinc-400">• Arsip</span>
                          )}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0">
                      {isArchived ? (
                        <span className="h-1.5 w-1.5 rounded-full bg-zinc-500" title="Proyek Terarsip" />
                      ) : (
                        <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" title="Proyek Aktif" />
                      )}
                      {isSelected && <Check className="h-3.5 w-3.5 text-primary" />}
                    </div>
                  </button>
                )
              })
            )}
          </div>

          {/* Footer Action: Create New Project */}
          {onCreateProjectClick && (
            <div className="pt-1.5 mt-1 border-t border-border/80">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  setIsOpen(false)
                  onCreateProjectClick()
                }}
                className="w-full text-xs font-medium justify-center gap-1.5 border-dashed border-primary/40 text-primary hover:bg-primary/10 hover:border-primary"
              >
                <Plus className="h-3.5 w-3.5" />
                Create New Project
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
