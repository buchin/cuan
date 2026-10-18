import { useState } from 'react'
import { useTheme } from '../../context/ThemeContext'
import { NavUser } from './NavUser'
import { Project } from '../../api/types'
import {
  FolderGit2,
  CheckSquare,
  Bot,
  Settings,
  ChevronDown,
  Sun,
  Moon,
  Plus,
  PanelLeftClose,
  PanelLeft
} from 'lucide-react'

interface AppShellProps {
  currentTab: 'projects' | 'tasks' | 'agents' | 'settings'
  onSelectTab: (tab: 'projects' | 'tasks' | 'agents' | 'settings') => void
  activeProject?: Project
  projects: Project[]
  onSelectProject: (project?: Project) => void
  onCreateProjectClick?: () => void
  children: React.ReactNode
}

export function AppShell({
  currentTab,
  onSelectTab,
  activeProject,
  projects,
  onSelectProject,
  onCreateProjectClick,
  children,
}: AppShellProps) {
  const { resolvedTheme, toggleTheme } = useTheme()
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [projectSwitcherOpen, setProjectSwitcherOpen] = useState(false)
  const [headerSwitcherOpen, setHeaderSwitcherOpen] = useState(false)

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#eaeef6] text-[#1a1b26] dark:bg-[#1a1b26] dark:text-[#a9b1d6] font-mono select-none">
      {/* Slim Modern Sidebar */}
      <aside
        className={`flex flex-col border-r border-[#cfd6e6] dark:border-[#24283b] bg-[#e2e7f1] dark:bg-[#16161e] transition-all duration-200 z-30 ${
          sidebarCollapsed ? 'w-16' : 'w-64'
        }`}
      >
        {/* Brand Header */}
        <div className="flex h-14 items-center justify-between px-4 border-b border-[#cfd6e6] dark:border-[#24283b]">
          {!sidebarCollapsed && (
            <div className="flex items-center gap-2">
              <div className="h-6 w-6 bg-[#7aa2f7] flex items-center justify-center text-[#16161e] font-bold text-xs">
                C
              </div>
              <span className="font-bold tracking-wider text-sm text-[#1a1b26] dark:text-[#c0caf5]">
                CUAN
              </span>
              <span className="text-[10px] uppercase tracking-widest px-1.5 py-0.5 bg-[#7aa2f7]/10 text-[#7aa2f7] border border-[#7aa2f7]/20">
                v0.2
              </span>
            </div>
          )}
          <button
            onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
            className="p-1.5 text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5] hover:bg-[#cfd6e6]/50 dark:hover:bg-[#24283b] transition-colors"
            title={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            {sidebarCollapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
          </button>
        </div>

        {/* Project Selector Badge */}
        {!sidebarCollapsed ? (
          <div className="p-3 border-b border-[#cfd6e6] dark:border-[#24283b] relative">
            <div className="text-[10px] uppercase font-semibold text-[#565f89] mb-1.5 px-1">
              Active Project
            </div>
            <button
              onClick={() => setProjectSwitcherOpen(!projectSwitcherOpen)}
              className="w-full flex items-center justify-between px-2.5 py-1.5 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] hover:border-[#7aa2f7] dark:hover:border-[#7aa2f7] transition-all text-left"
            >
              <div className="flex items-center gap-2 truncate">
                <div className={`h-2 w-2 rounded-none ${activeProject ? 'bg-[#9ece6a]' : 'bg-[#565f89]'}`} />
                <span className="truncate font-medium text-[#1a1b26] dark:text-[#c0caf5]">
                  {activeProject ? activeProject.name : 'All Projects'}
                </span>
              </div>
              <ChevronDown className="h-3.5 w-3.5 text-[#565f89] shrink-0" />
            </button>

            {/* Switcher Dropdown */}
            {projectSwitcherOpen && (
              <div className="absolute top-full left-3 right-3 mt-1 bg-[#e2e7f1] dark:bg-[#16161e] border border-[#cfd6e6] dark:border-[#24283b] shadow-xl z-50 p-1">
                <button
                  onClick={() => {
                    onSelectProject(undefined)
                    setProjectSwitcherOpen(false)
                  }}
                  className={`w-full text-left px-2 py-1.5 text-xs flex items-center justify-between hover:bg-[#7aa2f7]/10 ${
                    !activeProject ? 'text-[#7aa2f7] font-bold' : 'text-[#565f89]'
                  }`}
                >
                  <span>All Projects Overview</span>
                </button>
                <div className="h-[1px] bg-[#cfd6e6] dark:bg-[#24283b] my-1" />
                <div className="max-h-48 overflow-y-auto">
                  {projects.map((p) => (
                    <button
                      key={p.id}
                      onClick={() => {
                        onSelectProject(p)
                        setProjectSwitcherOpen(false)
                      }}
                      className={`w-full text-left px-2 py-1.5 text-xs truncate flex items-center gap-2 hover:bg-[#7aa2f7]/10 ${
                        activeProject?.id === p.id ? 'text-[#7aa2f7] font-bold' : 'text-[#343b58] dark:text-[#a9b1d6]'
                      }`}
                    >
                      <div className="h-1.5 w-1.5 bg-[#9ece6a]" />
                      <span className="truncate">{p.name}</span>
                    </button>
                  ))}
                </div>
                {onCreateProjectClick && (
                  <>
                    <div className="h-[1px] bg-[#cfd6e6] dark:bg-[#24283b] my-1" />
                    <button
                      onClick={() => {
                        setProjectSwitcherOpen(false)
                        onCreateProjectClick()
                      }}
                      className="w-full text-left px-2 py-1.5 text-xs text-[#7aa2f7] hover:bg-[#7aa2f7]/10 flex items-center gap-1.5 font-medium"
                    >
                      <Plus className="h-3.5 w-3.5" />
                      <span>New Project...</span>
                    </button>
                  </>
                )}
              </div>
            )}
          </div>
        ) : (
          <div className="py-3 flex justify-center border-b border-[#cfd6e6] dark:border-[#24283b]">
            <div className={`h-3 w-3 ${activeProject ? 'bg-[#9ece6a]' : 'bg-[#565f89]'}`} title={activeProject ? activeProject.name : 'All Projects'} />
          </div>
        )}

        {/* Navigation Items */}
        <nav className="flex-1 px-2 py-3 space-y-1">
          <button
            onClick={() => onSelectTab('projects')}
            className={`w-full flex items-center gap-3 px-3 py-2 text-xs font-medium transition-colors ${
              currentTab === 'projects'
                ? 'bg-[#7aa2f7]/15 text-[#7aa2f7] border-l-2 border-[#7aa2f7]'
                : 'text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5] hover:bg-[#cfd6e6]/30 dark:hover:bg-[#1f2335]'
            }`}
            title="Projects"
          >
            <FolderGit2 className="h-4 w-4 shrink-0" />
            {!sidebarCollapsed && <span>Projects</span>}
          </button>

          <button
            onClick={() => onSelectTab('tasks')}
            className={`w-full flex items-center gap-3 px-3 py-2 text-xs font-medium transition-colors ${
              currentTab === 'tasks'
                ? 'bg-[#7aa2f7]/15 text-[#7aa2f7] border-l-2 border-[#7aa2f7]'
                : 'text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5] hover:bg-[#cfd6e6]/30 dark:hover:bg-[#1f2335]'
            }`}
            title="Tasks"
          >
            <CheckSquare className="h-4 w-4 shrink-0" />
            {!sidebarCollapsed && <span>Task Board</span>}
          </button>

          <button
            onClick={() => onSelectTab('agents')}
            className={`w-full flex items-center gap-3 px-3 py-2 text-xs font-medium transition-colors ${
              currentTab === 'agents'
                ? 'bg-[#7aa2f7]/15 text-[#7aa2f7] border-l-2 border-[#7aa2f7]'
                : 'text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5] hover:bg-[#cfd6e6]/30 dark:hover:bg-[#1f2335]'
            }`}
            title="Agents"
          >
            <Bot className="h-4 w-4 shrink-0" />
            {!sidebarCollapsed && <span>AI Agents Hub</span>}
          </button>

          <button
            onClick={() => onSelectTab('settings')}
            className={`w-full flex items-center gap-3 px-3 py-2 text-xs font-medium transition-colors ${
              currentTab === 'settings'
                ? 'bg-[#7aa2f7]/15 text-[#7aa2f7] border-l-2 border-[#7aa2f7]'
                : 'text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5] hover:bg-[#cfd6e6]/30 dark:hover:bg-[#1f2335]'
            }`}
            title="Settings"
          >
            <Settings className="h-4 w-4 shrink-0" />
            {!sidebarCollapsed && <span>Settings</span>}
          </button>
        </nav>

        {/* Footer: User profile & Tokyo Night Theme Toggle */}
        <div className="p-2 border-t border-[#cfd6e6] dark:border-[#24283b] flex items-center justify-between">
          <div className="flex-1 truncate">
            <NavUser isCollapsed={sidebarCollapsed} />
          </div>
          <button
            onClick={toggleTheme}
            className="p-2 text-[#565f89] hover:text-[#e0af68] transition-colors"
            title={`Switch to ${resolvedTheme === 'dark' ? 'Light' : 'Dark'} mode`}
          >
            {resolvedTheme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>
        </div>
      </aside>

      {/* Main Container */}
      <main className="flex-1 flex flex-col h-full overflow-hidden bg-[#eaeef6] dark:bg-[#1a1b26]">
        {/* Top Header Bar */}
        <header className="h-14 border-b border-[#cfd6e6] dark:border-[#24283b] px-6 flex items-center justify-between bg-[#f0f3f8] dark:bg-[#16161e]/50 shrink-0">
          <div className="flex items-center gap-2 text-xs relative">
            <span className="text-[#565f89]">Cuan</span>
            <span className="text-[#565f89]">/</span>
            <div className="relative">
              <button
                onClick={() => setHeaderSwitcherOpen(!headerSwitcherOpen)}
                className="flex items-center gap-1.5 px-2 py-1 text-xs font-semibold text-[#1a1b26] dark:text-[#c0caf5] bg-[#d5dce8]/50 dark:bg-[#1f2335]/50 border border-transparent hover:border-[#7aa2f7] transition-all"
              >
                <div className={`h-2 w-2 rounded-none ${activeProject ? 'bg-[#9ece6a]' : 'bg-[#565f89]'}`} />
                <span>{activeProject ? activeProject.name : 'All Projects'}</span>
                <ChevronDown className="h-3 w-3 text-[#565f89]" />
              </button>

              {headerSwitcherOpen && (
                <div className="absolute top-full left-0 mt-1 w-56 bg-[#e2e7f1] dark:bg-[#16161e] border border-[#cfd6e6] dark:border-[#24283b] shadow-xl z-50 p-1">
                  <button
                    onClick={() => {
                      onSelectProject(undefined)
                      setHeaderSwitcherOpen(false)
                    }}
                    className={`w-full text-left px-2 py-1.5 text-xs flex items-center justify-between hover:bg-[#7aa2f7]/10 ${
                      !activeProject ? 'text-[#7aa2f7] font-bold' : 'text-[#565f89]'
                    }`}
                  >
                    <span>All Projects Overview</span>
                  </button>
                  <div className="h-[1px] bg-[#cfd6e6] dark:bg-[#24283b] my-1" />
                  <div className="max-h-48 overflow-y-auto">
                    {projects.map((p) => (
                      <button
                        key={p.id}
                        onClick={() => {
                          onSelectProject(p)
                          setHeaderSwitcherOpen(false)
                        }}
                        className={`w-full text-left px-2 py-1.5 text-xs truncate flex items-center gap-2 hover:bg-[#7aa2f7]/10 ${
                          activeProject?.id === p.id ? 'text-[#7aa2f7] font-bold' : 'text-[#343b58] dark:text-[#a9b1d6]'
                        }`}
                      >
                        <div className="h-1.5 w-1.5 bg-[#9ece6a]" />
                        <span className="truncate">{p.name}</span>
                      </button>
                    ))}
                  </div>
                  {onCreateProjectClick && (
                    <>
                      <div className="h-[1px] bg-[#cfd6e6] dark:bg-[#24283b] my-1" />
                      <button
                        onClick={() => {
                          setHeaderSwitcherOpen(false)
                          onCreateProjectClick()
                        }}
                        className="w-full text-left px-2 py-1.5 text-xs text-[#7aa2f7] hover:bg-[#7aa2f7]/10 flex items-center gap-1.5 font-medium"
                      >
                        <Plus className="h-3.5 w-3.5" />
                        <span>New Project...</span>
                      </button>
                    </>
                  )}
                </div>
              )}
            </div>
            <span className="text-[#565f89]">/</span>
            <span className="text-[#7aa2f7] uppercase tracking-wider text-[11px] font-bold">
              {currentTab}
            </span>
          </div>

          <div className="flex items-center gap-3">
            {onCreateProjectClick && (
              <button
                onClick={onCreateProjectClick}
                className="flex items-center gap-1.5 px-3 py-1.5 bg-[#7aa2f7] text-[#16161e] font-semibold text-xs hover:bg-[#89b4fa] transition-colors"
              >
                <Plus className="h-3.5 w-3.5" />
                <span>New Project</span>
              </button>
            )}
          </div>
        </header>

        {/* Active Content Area */}
        <div className="flex-1 overflow-auto p-6">
          {children}
        </div>
      </main>
    </div>
  )
}
