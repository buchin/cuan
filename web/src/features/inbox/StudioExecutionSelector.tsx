import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import {
  Sparkles,
  FolderGit2,
  Bot,
  ChevronDown,
  Check,
  Zap,
  Eye,
  Brain,
  Plus,
  ExternalLink,
  GitBranch,
  X,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { type Agent, isModelVisionCapable } from '../../api/agents'
import { type Project } from '../../api/types'
import { connectionStatus } from '../agents/connection'

export function StudioExecutionSelector({
  projects,
  selectedProjectId,
  onSelectProject,
  onConnectProjectClick,
  agents,
  selectedAgentId,
  onSelectAgent,
  onAddAgentClick,
  disabled,
}: {
  projects: Project[]
  selectedProjectId: string
  onSelectProject: (id: string) => void
  onConnectProjectClick?: () => void
  agents: Agent[]
  selectedAgentId: string
  onSelectAgent: (id: string) => void
  onAddAgentClick?: () => void
  disabled?: boolean
}) {
  const { t } = useLanguage()
  const [isOpen, setIsOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<'project' | 'agent'>('project')

  // Close on Escape key press
  useEffect(() => {
    if (!isOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsOpen(false)
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen])

  const selectedProject = projects.find((p) => p.id === selectedProjectId)
  const selectedAgent = agents.find((a) => a.id === selectedAgentId)

  // Display label and icon on the compact pill button
  let pillIcon = <Sparkles className="h-3.5 w-3.5 text-amber-500" aria-hidden="true" />
  let pillLabel = t('Auto')

  if (selectedProject && selectedAgent) {
    pillIcon = <Sparkles className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
    pillLabel = `${selectedProject.name} · ${selectedAgent.name}`
  } else if (selectedProject) {
    pillIcon = <FolderGit2 className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
    pillLabel = selectedProject.name
  } else if (selectedAgent) {
    pillIcon = <Bot className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
    pillLabel = selectedAgent.name
  }

  const isCustomized = Boolean(selectedProjectId || selectedAgentId)

  return (
    <div className="relative inline-block text-left min-w-0">
      <button
        type="button"
        id="studio-execution-selector-pill"
        onClick={() => !disabled && setIsOpen((prev) => !prev)}
        disabled={disabled}
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        aria-label={t('Configure Execution Target and Agent')}
        className={`inline-flex min-h-11 max-w-full items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50 ${
          isCustomized
            ? 'border-primary/40 bg-primary/10 text-primary hover:bg-primary/15'
            : 'border-border/80 bg-muted/40 text-foreground hover:bg-muted/80 hover:border-border'
        }`}
      >
        {pillIcon}
        <span className="truncate max-w-[100px] sm:max-w-[180px]">{pillLabel}</span>
        <ChevronDown
          className={`h-3 w-3 text-muted-foreground transition-transform duration-150 ${
            isOpen ? 'rotate-180' : ''
          }`}
          aria-hidden="true"
        />
      </button>

      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/60 backdrop-blur-xs p-0 sm:p-4">
          {/* Backdrop click area */}
          <div
            className="absolute inset-0"
            onClick={() => setIsOpen(false)}
            aria-hidden="true"
          />

          {/* Sheet / Modal Dialog Container */}
          <div
            role="dialog"
            aria-modal="true"
            aria-label={t('Execution Settings')}
            className="relative w-full sm:max-w-md rounded-t-3xl sm:rounded-2xl border border-border bg-popover shadow-2xl flex flex-col max-h-[85vh] sm:max-h-[600px] overflow-hidden z-10 animate-in fade-in zoom-in-95 duration-150"
          >
            {/* Mobile pull bar */}
            <div className="sm:hidden pt-3 pb-1 flex justify-center">
              <div className="h-1 w-10 rounded-full bg-muted-foreground/30" />
            </div>

            {/* Header with Title and Close Button */}
            <div className="px-4 py-2.5 sm:px-5 sm:py-3 border-b border-border/50 flex items-center justify-between">
              <div>
                <h3 className="text-sm font-semibold text-foreground">
                  {t('Execution Settings')}
                </h3>
                <p className="text-[11px] text-muted-foreground">
                  {t('Configure target project and agent')}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                className="p-1.5 rounded-full text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                aria-label={t('Close')}
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Header Tabs: Project & Agent */}
            <div className="p-3 pb-0 bg-muted/20 border-b border-border/40">
              <div className="flex items-center gap-1.5 p-1 rounded-xl bg-muted/60 text-xs font-medium">
                <button
                  type="button"
                  onClick={() => setActiveTab('project')}
                  className={`flex-1 flex items-center justify-center gap-2 py-2 px-3 rounded-lg text-xs font-medium transition-all ${
                    activeTab === 'project'
                      ? 'bg-background text-foreground shadow-sm font-semibold'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <FolderGit2 className="h-4 w-4 text-primary" />
                  <span>{t('Project')}</span>
                  {selectedProjectId ? (
                    <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                  ) : null}
                </button>
                <button
                  type="button"
                  onClick={() => setActiveTab('agent')}
                  className={`flex-1 flex items-center justify-center gap-2 py-2 px-3 rounded-lg text-xs font-medium transition-all ${
                    activeTab === 'agent'
                      ? 'bg-background text-foreground shadow-sm font-semibold'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <Bot className="h-4 w-4 text-primary" />
                  <span>{t('Agent')}</span>
                  {selectedAgentId ? (
                    <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                  ) : null}
                </button>
              </div>

              {/* Quick Reset if customized */}
              {isCustomized && (
                <div className="flex items-center justify-between px-1 py-1.5 text-[11px]">
                  <span className="text-muted-foreground text-[10px]">
                    {selectedProject ? selectedProject.name : t('Auto Project')} ·{' '}
                    {selectedAgent ? selectedAgent.name : t('Auto Agent')}
                  </span>
                  <button
                    type="button"
                    onClick={() => {
                      onSelectProject('')
                      onSelectAgent('')
                    }}
                    className="inline-flex items-center gap-1 text-[11px] font-medium text-primary hover:underline"
                  >
                    <Sparkles className="h-3 w-3 text-amber-500" />
                    {t('Reset ke Otomatis')}
                  </button>
                </div>
              )}
            </div>

            {/* Scrollable Content Area */}
            <div className="flex-1 overflow-y-auto p-3 sm:p-4 space-y-1 pb-[calc(1.5rem+env(safe-area-inset-bottom))]">
              {/* Tab Content: Project */}
              {activeTab === 'project' && (
                <>
                  {/* Option: Auto Project */}
                  <button
                    type="button"
                    role="option"
                    aria-selected={!selectedProjectId}
                    onClick={() => {
                      onSelectProject('')
                      setIsOpen(false)
                    }}
                    className={`w-full flex items-start gap-2.5 rounded-xl px-2.5 py-2.5 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground ${
                      !selectedProjectId ? 'bg-accent/60 font-semibold' : ''
                    }`}
                  >
                    <div className="mt-0.5 rounded-lg bg-primary/10 p-1 text-primary shrink-0">
                      <Sparkles className="h-4 w-4" />
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
                          'Cuan otomatis mendeteksi target project dari obrolan, atau menggunakan project default bila tidak disebutkan.',
                        )}
                      </p>
                    </div>
                    {!selectedProjectId && <Check className="h-4 w-4 text-primary shrink-0 self-center" />}
                  </button>

                  {/* Section Header: Projects */}
                  <div className="px-2.5 pt-3 pb-1 flex items-center justify-between text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                    <span>{t('Projects')}</span>
                    <span className="text-[9px] bg-muted px-1.5 py-0.2 rounded font-normal">
                      {projects.length}
                    </span>
                  </div>

                  {/* Project list */}
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
                          onSelectProject(project.id)
                          setIsOpen(false)
                        }}
                        className={`w-full flex items-center justify-between gap-2 rounded-xl px-2.5 py-2.5 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground ${
                          isSelected ? 'bg-accent/60 font-semibold' : ''
                        }`}
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <span
                            className={`h-2.5 w-2.5 shrink-0 rounded-full ${
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

                  {/* Action Footer: Connect project */}
                  <div className="my-2 border-t border-border/50" />
                  <div className="flex items-center justify-between gap-1 px-1">
                    {onConnectProjectClick ? (
                      <button
                        type="button"
                        onClick={() => {
                          setIsOpen(false)
                          onConnectProjectClick()
                        }}
                        className="flex-1 flex items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-colors group"
                      >
                        <div className="flex h-5 w-5 items-center justify-center rounded-md border border-dashed border-border bg-muted/40 text-muted-foreground group-hover:border-primary/50 group-hover:text-primary transition-colors">
                          <Plus className="h-3 w-3" />
                        </div>
                        <span>{t('+ Create or connect project…')}</span>
                      </button>
                    ) : null}
                    <Link
                      to="/projects"
                      onClick={() => setIsOpen(false)}
                      className="inline-flex items-center gap-1 px-2.5 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors rounded-lg hover:bg-accent shrink-0"
                      title={t('Manage projects')}
                    >
                      <span>{t('Manage projects')}</span>
                      <ExternalLink className="h-3 w-3 ml-0.5" />
                    </Link>
                  </div>
                </>
              )}

              {/* Tab Content: Agent */}
              {activeTab === 'agent' && (
                <>
                  {/* Option: Auto Agent */}
                  <button
                    type="button"
                    role="option"
                    aria-selected={!selectedAgentId}
                    onClick={() => {
                      onSelectAgent('')
                      setIsOpen(false)
                    }}
                    className={`w-full flex items-start gap-2.5 rounded-xl px-2.5 py-2.5 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground ${
                      !selectedAgentId ? 'bg-accent/60 font-semibold' : ''
                    }`}
                  >
                    <div className="mt-0.5 rounded-lg bg-amber-500/10 p-1 text-amber-500 shrink-0">
                      <Sparkles className="h-4 w-4" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-foreground">{t('Auto (choose best)')}</span>
                        <span className="text-[10px] uppercase font-bold text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded">
                          {t('Recommended')}
                        </span>
                      </div>
                      <p className="text-[11px] text-muted-foreground mt-0.5 leading-tight">
                        {t('Cuan automatically selects the optimal agent and manages quota failover.')}
                      </p>
                    </div>
                    {!selectedAgentId && <Check className="h-4 w-4 text-primary shrink-0 self-center" />}
                  </button>

                  {/* Section Header: Team Specialists */}
                  <div className="px-2.5 pt-3 pb-1 flex items-center justify-between text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                    <span>{t('Team Specialists')}</span>
                    <span className="text-[9px] bg-muted px-1.5 py-0.2 rounded font-normal">{agents.length}</span>
                  </div>

                  {/* Agent list */}
                  {agents.map((agent) => {
                    const state = connectionStatus(agent, Date.now())
                    const vision = isModelVisionCapable(agent.model_id, agent.provider || agent.harness)
                    const isSelected = agent.id === selectedAgentId
                    const isInactive =
                      agent.enabled === false ||
                      agent.status === 'inactive' ||
                      state.state === 'disabled' ||
                      state.state === 'login_required'

                    const modelLower = (agent.model_id || '').toLowerCase()
                    const isThinking =
                      modelLower.includes('pro') ||
                      modelLower.includes('opus') ||
                      modelLower.includes('thinking') ||
                      modelLower.includes('sonnet')

                    return (
                      <button
                        key={agent.id}
                        type="button"
                        role="option"
                        disabled={isInactive}
                        aria-selected={isSelected}
                        onClick={() => {
                          onSelectAgent(agent.id)
                          setIsOpen(false)
                        }}
                        className={`w-full flex items-center justify-between gap-2 rounded-xl px-2.5 py-2.5 text-left text-xs transition-colors hover:bg-accent hover:text-accent-foreground disabled:opacity-40 disabled:pointer-events-none ${
                          isSelected ? 'bg-accent/60 font-semibold' : ''
                        }`}
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <span
                            className={`h-2.5 w-2.5 shrink-0 rounded-full ${
                              state.state === 'verified'
                                ? 'bg-emerald-500 ring-2 ring-emerald-500/20'
                                : state.state === 'quota_limited'
                                ? 'bg-amber-500'
                                : 'bg-zinc-400'
                            }`}
                            title={t(state.label)}
                          />
                          <div className="truncate">
                            <span className="font-medium text-foreground">{agent.name}</span>
                            <span className="ml-1.5 text-[10px] text-muted-foreground">
                              {t(state.label)}
                            </span>
                          </div>
                        </div>

                        <div className="flex items-center gap-1 shrink-0">
                          {isThinking ? (
                            <span className="inline-flex items-center gap-0.5 rounded bg-purple-500/10 px-1.5 py-0.5 text-[10px] font-medium text-purple-600 dark:text-purple-400">
                              <Brain className="h-2.5 w-2.5" />
                              Pro
                            </span>
                          ) : (
                            <span className="inline-flex items-center gap-0.5 rounded bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-medium text-blue-600 dark:text-blue-400">
                              <Zap className="h-2.5 w-2.5" />
                              Fast
                            </span>
                          )}
                          {vision && (
                            <span
                              className="inline-flex items-center rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground"
                              title={t('Vision capable')}
                            >
                              <Eye className="h-2.5 w-2.5 mr-0.5" />
                              Vision
                            </span>
                          )}
                          {isSelected && <Check className="h-4 w-4 text-primary ml-1" />}
                        </div>
                      </button>
                    )
                  })}

                  {/* Action Footer: Add agent */}
                  <div className="my-2 border-t border-border/50" />
                  <div className="flex items-center justify-between gap-1 px-1">
                    {onAddAgentClick ? (
                      <button
                        type="button"
                        onClick={() => {
                          setIsOpen(false)
                          onAddAgentClick()
                        }}
                        className="flex-1 flex items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-colors group"
                      >
                        <div className="flex h-5 w-5 items-center justify-center rounded-md border border-dashed border-border bg-muted/40 text-muted-foreground group-hover:border-primary/50 group-hover:text-primary transition-colors">
                          <Plus className="h-3 w-3" />
                        </div>
                        <span>{t('Add or connect agent…')}</span>
                      </button>
                    ) : null}
                    <Link
                      to="/settings/agents"
                      onClick={() => setIsOpen(false)}
                      className="inline-flex items-center gap-1 px-2.5 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors rounded-lg hover:bg-accent shrink-0"
                      title={t('Manage team')}
                    >
                      <span>{t('Manage team')}</span>
                      <ExternalLink className="h-3 w-3 ml-0.5" />
                    </Link>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
