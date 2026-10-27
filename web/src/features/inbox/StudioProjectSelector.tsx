import { useState, useRef, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { FolderGit2, ChevronDown, Check, Sparkles, Plus, ExternalLink, GitBranch } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { type Project } from '../../api/types'

export function StudioProjectSelector({
  projects,
  selectedProjectId,
  onSelect,
  onConnectProjectClick,
  disabled,
}: {
  projects: Project[]
  selectedProjectId: string
  onSelect: (id: string) => void
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

  const selectedProject = projects.find((p) => p.id === selectedProjectId)

  // Display label on the pill button
  const pillLabel = selectedProject ? selectedProject.name : t('Project: Auto')

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
        <span className="truncate max-w-[120px] sm:max-w-[160px]">{pillLabel}</span>
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
          className="absolute bottom-full left-0 mb-2 z-50 w-76 sm:w-84 rounded-2xl border border-border bg-popover p-1.5 shadow-2xl outline-none max-h-[min(420px,calc(100vh-180px))] overflow-y-auto"
        >
          {/* Option: Auto */}
          <button
            type="button"
            role="option"
            aria-selected={!selectedProjectId}
            onClick={() => {
              onSelect('')
              setIsOpen(false)
            }}
            className={`w-full flex items-start gap-2.5 rounded-xl px-2.5 py-2 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground ${
              !selectedProjectId ? 'bg-accent/60 font-semibold' : ''
            }`}
          >
            <div className="mt-0.5 rounded-lg bg-primary/10 p-1 text-primary shrink-0">
              <Sparkles className="h-3.5 w-3.5" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between">
                <span className="font-medium text-foreground">{t('Auto (Default Project)')}</span>
                <span className="text-[10px] uppercase font-bold text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded">
                  {t('Recommended')}
                </span>
              </div>
              <p className="text-[11px] text-muted-foreground mt-0.5 leading-tight">
                {t(
                  'Cuan automatically routes to the default project or prepares a new workspace according to your plan.',
                )}
              </p>
            </div>
            {!selectedProjectId && <Check className="h-4 w-4 text-primary shrink-0 self-center" />}
          </button>

          {/* Section Header: Projects */}
          <div className="px-2.5 pt-2 pb-1 flex items-center justify-between text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
            <span>{t('Projects')}</span>
            <span className="text-[9px] bg-muted px-1.5 py-0.2 rounded font-normal">
              {projects.length}
            </span>
          </div>

          {/* Individual Projects */}
          {projects.map((project) => {
            const isSelected = project.id === selectedProjectId
            const isSynced =
              project.clone_status === 'connected' || project.clone_status === 'synced'
            const isFailed = project.clone_status === 'failed'

            return (
              <button
                key={project.id}
                type="button"
                role="option"
                aria-selected={isSelected}
                onClick={() => {
                  onSelect(project.id)
                  setIsOpen(false)
                }}
                className={`w-full flex items-center justify-between gap-2 rounded-xl px-2.5 py-2 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground ${
                  isSelected ? 'bg-accent/60 font-semibold' : ''
                }`}
              >
                <div className="flex items-center gap-2 min-w-0">
                  {/* Status dot */}
                  <span
                    className={`h-2 w-2 shrink-0 rounded-full ${
                      isFailed
                        ? 'bg-rose-500'
                        : isSynced
                        ? 'bg-emerald-500 ring-2 ring-emerald-500/20'
                        : 'bg-zinc-400'
                    }`}
                  />
                  <div className="truncate">
                    <span className="font-medium text-foreground">{project.name}</span>
                  </div>
                </div>

                <div className="flex items-center gap-1 shrink-0">
                  {project.default_branch && (
                    <span
                      className="inline-flex items-center gap-0.5 rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground"
                      title={project.default_branch}
                    >
                      <GitBranch className="h-2.5 w-2.5 mr-0.5" />
                      <span className="max-w-[70px] truncate">{project.default_branch}</span>
                    </span>
                  )}
                  {isSelected && <Check className="h-4 w-4 text-primary ml-1" />}
                </div>
              </button>
            )
          })}

          {/* Action Footer: Connect new project & Manage */}
          <div className="my-1 border-t border-border/50" />
          <div className="flex items-center justify-between gap-1 px-1">
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
            ) : null}
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
