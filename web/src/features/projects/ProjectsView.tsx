import { useState } from 'react'
import { Project, Task } from '../../api/types'
import {
  FolderGit2,
  Plus,
  GitBranch,
  ArrowRight,
  Archive,
  LayoutGrid,
  List,
  CheckSquare,
} from 'lucide-react'

interface ProjectsViewProps {
  projects: Project[]
  tasks?: Task[]
  onSelectProject: (project: Project) => void
  onCreateProject: () => void
  onArchiveProject?: (id: string, isArchived: boolean) => Promise<void>
}

export function ProjectsView({
  projects,
  tasks = [],
  onSelectProject,
  onCreateProject,
  onArchiveProject,
}: ProjectsViewProps) {
  const [activeTab, setActiveTab] = useState<'active' | 'archived'>('active')
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')
  const [search, setSearch] = useState('')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const archive = async (project: Project) => {
    if (
      !onArchiveProject ||
      busy ||
      !confirm(
        `${project.is_archived ? 'Pulihkan' : 'Arsipkan'} project "${project.name}"?`,
      )
    )
      return
    setBusy(project.id)
    setError('')
    try {
      await onArchiveProject(project.id, !project.is_archived)
    } catch {
      setError(
        'Perubahan belum tersimpan. Coba lagi; data project tetap tersedia.',
      )
    } finally {
      setBusy('')
    }
  }

  const activeProjects = projects.filter((p) => !p.is_archived)
  const archivedProjects = projects.filter((p) => p.is_archived)

  const displayedProjects = (
    activeTab === 'active' ? activeProjects : archivedProjects
  ).filter(
    (p) =>
      (p.name || '').toLowerCase().includes(search.toLowerCase()) ||
      (p.repo_url && p.repo_url.toLowerCase().includes(search.toLowerCase())),
  )

  const getTaskCountForProject = (projectId: string) => {
    return tasks.filter(
      (t) => t.project_id === projectId || t.project_ids?.includes(projectId),
    ).length
  }

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      {/* Top Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="page-title">Projects</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            {activeProjects.length} active · {archivedProjects.length} archived
          </p>
        </div>

        <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
          <input
            type="text"
            aria-label="Search projects"
            placeholder="Search projects…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="min-h-11 px-3 py-2 text-sm bg-background border border-input text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded-md w-full sm:w-48"
          />

          <div className="flex items-center border border-input rounded-md overflow-hidden">
            <button
              onClick={() => setViewMode('grid')}
              className={`inline-flex h-11 w-11 items-center justify-center transition-colors ${
                viewMode === 'grid'
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-background text-muted-foreground hover:text-foreground'
              }`}
              title="Grid view"
            >
              <LayoutGrid className="h-4 w-4" />
            </button>
            <button
              onClick={() => setViewMode('list')}
              className={`inline-flex h-11 w-11 items-center justify-center transition-colors ${
                viewMode === 'list'
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-background text-muted-foreground hover:text-foreground'
              }`}
              title="List view"
            >
              <List className="h-4 w-4" />
            </button>
          </div>

          <button
            onClick={onCreateProject}
            className="flex items-center gap-1.5 min-h-11 px-3 py-2 bg-primary text-primary-foreground font-semibold text-sm hover:bg-primary/90 transition-colors rounded-md"
          >
            <Plus className="h-4 w-4" />
            <span>New Project</span>
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-4 border-b border-border text-sm">
        <button
          onClick={() => setActiveTab('active')}
          className={`pb-2.5 font-medium transition-colors border-b-2 -mb-px ${
            activeTab === 'active'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          Active Projects ({activeProjects.length})
        </button>
        <button
          onClick={() => setActiveTab('archived')}
          className={`pb-2.5 font-medium transition-colors border-b-2 -mb-px ${
            activeTab === 'archived'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          Archived ({archivedProjects.length})
        </button>
      </div>

      {/* Projects Display or Empty State */}
      {displayedProjects.length === 0 ? (
        <div className="border border-dashed border-border p-12 text-center bg-muted/30 rounded-lg">
          <FolderGit2 className="h-10 w-10 text-muted-foreground mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-foreground">
            {activeTab === 'active'
              ? 'No active projects yet'
              : 'No archived projects'}
          </h3>
          <p className="text-sm text-muted-foreground max-w-sm mx-auto mt-1 mb-5">
            {activeTab === 'active'
              ? 'Create your first project to start managing tasks and assigning AI Agents.'
              : 'Archived projects will appear here.'}
          </p>
          {activeTab === 'active' && (
            <button
              onClick={onCreateProject}
              className="inline-flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground font-semibold text-sm hover:bg-primary/90 transition-colors rounded-md"
            >
              <Plus className="h-4 w-4" />
              <span>Create New Project</span>
            </button>
          )}
        </div>
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {displayedProjects.map((project) => {
            const taskCount = getTaskCountForProject(project.id)

            return (
              <div
                key={project.id}
                className="group border border-border bg-card p-5 flex flex-col justify-between hover:border-primary/50 transition-all rounded-lg shadow-sm"
              >
                <div>
                  <div className="flex items-start justify-between gap-2 mb-2">
                    <span className="text-xs uppercase font-mono px-2 py-0.5 bg-primary/10 text-primary border border-primary/20 rounded">
                      {project.project_type || 'custom'}
                    </span>
                    {onArchiveProject && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          void archive(project)
                        }}
                        className="icon-action text-muted-foreground"
                        disabled={!!busy}
                        title={
                          project.is_archived ? 'Unarchive' : 'Archive project'
                        }
                      >
                        <Archive className="h-3.5 w-3.5" />
                      </button>
                    )}
                  </div>

                  <button
                    type="button"
                    onClick={() => onSelectProject(project)}
                    className="min-h-11 text-left font-bold text-base text-foreground group-hover:text-primary transition-colors"
                  >
                    {project.name}
                  </button>

                  {project.repo_url && (
                    <p className="text-xs text-muted-foreground flex items-center gap-1.5 mt-2 truncate">
                      <GitBranch className="h-3 w-3 shrink-0" />
                      <span className="truncate">{project.repo_url}</span>
                    </p>
                  )}
                </div>

                <div className="pt-4 mt-4 border-t border-border flex items-center justify-between text-sm">
                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                    <CheckSquare className="h-3 w-3" />
                    <span>
                      {taskCount} {taskCount === 1 ? 'task' : 'tasks'}
                    </span>
                  </div>

                  <button
                    onClick={() => onSelectProject(project)}
                    className="inline-flex min-h-11 items-center gap-1 text-primary font-semibold text-sm hover:underline"
                  >
                    <span>Open Board</span>
                    <ArrowRight className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      ) : (
        /* List View */
        <div className="border border-border bg-card divide-y divide-border rounded-lg overflow-hidden shadow-sm">
          {displayedProjects.map((project) => {
            const taskCount = getTaskCountForProject(project.id)

            return (
              <div
                key={project.id}
                onClick={() => onSelectProject(project)}
                className="group p-4 flex items-center justify-between gap-4 hover:bg-accent cursor-pointer transition-colors"
              >
                <div className="flex items-center gap-3 min-w-0">
                  <FolderGit2 className="h-4 w-4 text-primary shrink-0" />
                  <div className="min-w-0">
                    <h3 className="font-bold text-sm text-foreground group-hover:text-primary truncate">
                      {project.name}
                    </h3>
                    {project.repo_url && (
                      <p className="text-xs text-muted-foreground truncate">
                        {project.repo_url}
                      </p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-6 shrink-0 text-sm">
                  <span className="text-xs uppercase font-mono px-2 py-0.5 bg-primary/10 text-primary border border-primary/20 rounded">
                    {project.project_type || 'custom'}
                  </span>

                  <span className="text-xs text-muted-foreground hidden sm:inline">
                    {taskCount} {taskCount === 1 ? 'task' : 'tasks'}
                  </span>

                  {onArchiveProject && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        void archive(project)
                      }}
                      className="icon-action text-muted-foreground"
                      disabled={!!busy}
                      title={
                        project.is_archived ? 'Unarchive' : 'Archive project'
                      }
                    >
                      <Archive className="h-3.5 w-3.5" />
                    </button>
                  )}

                  <ArrowRight className="h-4 w-4 text-primary group-hover:translate-x-1 transition-transform" />
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
