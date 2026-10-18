import { useState } from 'react'
import { Project } from '../../api/types'
import { FolderGit2, Plus, GitBranch, ArrowRight, Archive } from 'lucide-react'

interface ProjectsViewProps {
  projects: Project[]
  onSelectProject: (project: Project) => void
  onCreateProject: () => void
  onArchiveProject?: (id: string, isArchived: boolean) => void
}

export function ProjectsView({
  projects,
  onSelectProject,
  onCreateProject,
  onArchiveProject,
}: ProjectsViewProps) {
  const [activeTab, setActiveTab] = useState<'active' | 'archived'>('active')
  const [search, setSearch] = useState('')

  const activeProjects = projects.filter((p) => !p.is_archived)
  const archivedProjects = projects.filter((p) => p.is_archived)

  const displayedProjects = (activeTab === 'active' ? activeProjects : archivedProjects).filter((p) =>
    p.name.toLowerCase().includes(search.toLowerCase()) ||
    (p.repo_url && p.repo_url.toLowerCase().includes(search.toLowerCase()))
  )

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      {/* Top Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-[#1a1b26] dark:text-[#c0caf5]">
            Projects Directory
          </h1>
          <p className="text-xs text-[#565f89] mt-0.5">
            Manage your applications, content websites, and creator assets.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <input
            type="text"
            placeholder="Search projects..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="px-3 py-1.5 text-xs bg-[#e2e7f1] dark:bg-[#16161e] border border-[#cfd6e6] dark:border-[#24283b] text-[#1a1b26] dark:text-[#c0caf5] placeholder-[#565f89] focus:outline-none focus:border-[#7aa2f7]"
          />

          <button
            onClick={onCreateProject}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-[#7aa2f7] text-[#16161e] font-semibold text-xs hover:bg-[#89b4fa] transition-colors"
          >
            <Plus className="h-4 w-4" />
            <span>Create Project</span>
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-4 border-b border-[#cfd6e6] dark:border-[#24283b] text-xs">
        <button
          onClick={() => setActiveTab('active')}
          className={`pb-2.5 font-medium transition-colors border-b-2 ${
            activeTab === 'active'
              ? 'border-[#7aa2f7] text-[#7aa2f7]'
              : 'border-transparent text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5]'
          }`}
        >
          Active Projects ({activeProjects.length})
        </button>
        <button
          onClick={() => setActiveTab('archived')}
          className={`pb-2.5 font-medium transition-colors border-b-2 ${
            activeTab === 'archived'
              ? 'border-[#7aa2f7] text-[#7aa2f7]'
              : 'border-transparent text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5]'
          }`}
        >
          Archived ({archivedProjects.length})
        </button>
      </div>

      {/* Projects Grid or Empty State */}
      {displayedProjects.length === 0 ? (
        <div className="border border-dashed border-[#cfd6e6] dark:border-[#24283b] p-12 text-center bg-[#f0f3f8] dark:bg-[#16161e]/30">
          <FolderGit2 className="h-10 w-10 text-[#565f89] mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-[#1a1b26] dark:text-[#c0caf5]">
            {activeTab === 'active' ? 'No active projects yet' : 'No archived projects'}
          </h3>
          <p className="text-xs text-[#565f89] max-w-sm mx-auto mt-1 mb-5">
            {activeTab === 'active'
              ? 'Create your first project to start managing tasks and assigning AI Agents.'
              : 'Archived projects will appear here.'}
          </p>
          {activeTab === 'active' && (
            <button
              onClick={onCreateProject}
              className="inline-flex items-center gap-2 px-4 py-2 bg-[#7aa2f7] text-[#16161e] font-semibold text-xs hover:bg-[#89b4fa] transition-colors"
            >
              <Plus className="h-4 w-4" />
              <span>Create New Project</span>
            </button>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {displayedProjects.map((project) => (
            <div
              key={project.id}
              className="group border border-[#cfd6e6] dark:border-[#24283b] bg-[#e2e7f1] dark:bg-[#16161e] p-5 flex flex-col justify-between hover:border-[#7aa2f7] dark:hover:border-[#7aa2f7] transition-all"
            >
              <div>
                <div className="flex items-start justify-between gap-2 mb-2">
                  <span className="text-[10px] uppercase font-mono px-2 py-0.5 bg-[#7aa2f7]/10 text-[#7aa2f7] border border-[#7aa2f7]/20">
                    {project.project_type || 'custom'}
                  </span>
                  {onArchiveProject && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        onArchiveProject(project.id, !project.is_archived)
                      }}
                      className="text-[#565f89] hover:text-[#bb9af7] transition-colors"
                      title={project.is_archived ? 'Unarchive' : 'Archive project'}
                    >
                      <Archive className="h-3.5 w-3.5" />
                    </button>
                  )}
                </div>

                <h3
                  onClick={() => onSelectProject(project)}
                  className="font-bold text-sm text-[#1a1b26] dark:text-[#c0caf5] group-hover:text-[#7aa2f7] cursor-pointer transition-colors"
                >
                  {project.name}
                </h3>

                {project.repo_url && (
                  <p className="text-[11px] text-[#565f89] flex items-center gap-1.5 mt-2 truncate">
                    <GitBranch className="h-3 w-3 shrink-0" />
                    <span className="truncate">{project.repo_url}</span>
                  </p>
                )}
              </div>

              <div className="pt-4 mt-4 border-t border-[#cfd6e6] dark:border-[#24283b] flex items-center justify-between text-xs">
                <span className="text-[10px] text-[#565f89]">
                  Updated {new Date(project.updated_at || project.created_at).toLocaleDateString()}
                </span>
                <button
                  onClick={() => onSelectProject(project)}
                  className="flex items-center gap-1 text-[#7aa2f7] font-semibold text-xs hover:underline"
                >
                  <span>Open</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
