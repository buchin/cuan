import { useState, useRef, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { FolderGit2, ChevronDown, Check, Plus, ExternalLink, GitBranch } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { type Project } from '../../api/types'

export function StudioProjectSelector({
  projects,
  selectedProjectIds,
  onToggleProject,
  onConnectProjectClick,
  disabled,
}: {
  projects: Project[]
  selectedProjectIds: string[]
  onToggleProject: (id: string) => void
  onConnectProjectClick?: () => void
  disabled?: boolean
}) {
  const { t } = useLanguage()
  const [isOpen, setIsOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  // Close when clicking outside or pressing Escape
  useEffect(() => {
    if (!isOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsOpen(false)
    }
    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen])

  // Derive trigger pill label
  let pillLabel = t('Select project')
  if (selectedProjectIds.length === 1) {
    const p = projects.find((item) => item.id === selectedProjectIds[0])
    pillLabel = p ? p.name : selectedProjectIds[0]
  } else if (selectedProjectIds.length > 1) {
    pillLabel = `${selectedProjectIds.length} ${t('Projects')}`
  } else if (projects.length > 0) {
    pillLabel = projects[0].name
  }

  return (
    <div className="relative inline-block text-left" ref={containerRef}>
      <button
        type="button"
        id="studio-project-selector-pill"
        onClick={() => !disabled && setIsOpen((prev) => !prev)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-label={t('Select Project')}
        className="inline-flex min-h-11 items-center gap-1.5 rounded-full border border-border/80 bg-muted/40 px-3 py-1.5 text-xs font-medium text-foreground transition-all hover:bg-muted/80 hover:border-border focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
      >
        <FolderGit2 className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
        <span className="truncate max-w-[130px] sm:max-w-[180px]">{pillLabel}</span>
        <ChevronDown
          className={`h-3 w-3 text-muted-foreground transition-transform duration-150 ${
            isOpen ? 'rotate-180' : ''
          }`}
          aria-hidden="true"
        />
      </button>

      {isOpen && (
        <div
          role="listbox"
          aria-label={t('Available Projects')}
          className="absolute bottom-full left-0 mb-2 z-50 w-72 sm:w-80 rounded-2xl border border-border bg-popover p-1.5 shadow-2xl outline-none max-h-[min(380px,calc(100vh-180px))] overflow-y-auto"
        >
          {/* Section Header: Projects */}
          <div className="px-3 pt-2 pb-1.5 flex items-center justify-between text-xs font-medium text-muted-foreground border-b border-border/40 mb-1">
            <span>{t('Projects')}</span>
            <span className="text-[11px] text-muted-foreground/60 font-normal">
              {projects.length}
            </span>
          </div>

          {/* Individual Projects List */}
          <div className="space-y-0.5 py-0.5">
            {projects.map((project) => {
              const isSelected = selectedProjectIds.includes(project.id)
              const isFailed = project.clone_status === 'failed'

              return (
                <button
                  key={project.id}
                  type="button"
                  role="option"
                  aria-selected={isSelected}
                  onClick={() => onToggleProject(project.id)}
                  className={`group w-full min-h-[44px] flex items-center justify-between gap-3 rounded-xl px-3 py-2.5 text-left text-xs transition-colors hover:bg-accent/80 hover:text-accent-foreground ${
                    isSelected ? 'bg-accent/50 text-foreground font-medium' : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <FolderGit2 className={`h-4 w-4 shrink-0 transition-colors ${isSelected ? 'text-primary' : 'text-muted-foreground/60'}`} aria-hidden="true" />
                    <span className="truncate text-foreground font-medium">
                      {project.name}
                    </span>
                    {isFailed && (
                      <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-rose-500" title={t('Sync failed')} />
                    )}
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {project.default_branch && (
                      <span
                        className="text-[11px] text-muted-foreground/50 hidden group-hover:inline-flex items-center gap-1 transition-opacity"
                        title={project.default_branch}
                      >
                        <GitBranch className="h-2.5 w-2.5" />
                        <span className="max-w-[70px] truncate">{project.default_branch}</span>
                      </span>
                    )}
                    {isSelected && (
                      <Check className="h-4 w-4 text-primary shrink-0 stroke-[2.5]" aria-hidden="true" />
                    )}
                  </div>
                </button>
              )
            })}
          </div>

          {/* Action Footer: Connect new project & Manage */}
          <div className="my-1 border-t border-border/50" />
          <div className="flex items-center justify-between gap-1 px-1 pt-0.5">
            {onConnectProjectClick ? (
              <button
                type="button"
                onClick={() => {
                  setIsOpen(false)
                  onConnectProjectClick()
                }}
                className="flex-1 flex items-center gap-2 rounded-lg px-2 py-1.5 text-left text-xs font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-colors group"
              >
                <div className="flex h-5 w-5 items-center justify-center rounded-md border border-dashed border-border bg-muted/40 text-muted-foreground group-hover:border-primary/50 group-hover:text-primary transition-colors">
                  <Plus className="h-3 w-3" />
                </div>
                <span>{t('+ Connect new project…')}</span>
              </button>
            ) : (
              <Link
                to="/projects"
                onClick={() => setIsOpen(false)}
                className="flex-1 flex items-center gap-2 rounded-lg px-2 py-1.5 text-left text-xs font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-colors group"
              >
                <div className="flex h-5 w-5 items-center justify-center rounded-md border border-dashed border-border bg-muted/40 text-muted-foreground group-hover:border-primary/50 group-hover:text-primary transition-colors">
                  <Plus className="h-3 w-3" />
                </div>
                <span>{t('+ Connect new project…')}</span>
              </Link>
            )}
            <Link
              to="/projects"
              onClick={() => setIsOpen(false)}
              className="inline-flex items-center gap-1 px-2 py-1.5 text-[11px] text-muted-foreground hover:text-foreground transition-colors rounded-lg hover:bg-accent shrink-0"
              title={t('Manage projects')}
            >
              <ExternalLink className="h-3 w-3" />
            </Link>
          </div>
        </div>
      )}
    </div>
  )
}
