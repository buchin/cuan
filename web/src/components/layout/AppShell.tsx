import { useState } from 'react'
import { NavUser } from './NavUser'
import { Project } from '../../api/types'
import { useTheme } from '../../context/ThemeContext'
import {
  Inbox,
  CheckSquare,
  FolderGit2,
  Users,
  Plus,
  ChevronLeft,
  ChevronRight,
  PanelLeft,
  Sun,
  Moon,
  Menu,
  X,
  Settings,
} from 'lucide-react'
import { cn } from '../../lib/utils'

export type AppTab = 'inbox' | 'tasks' | 'projects' | 'team' | 'settings'

interface AppShellProps {
  currentTab: AppTab
  onSelectTab: (tab: AppTab) => void
  activeProject?: Project
  projects: Project[]
  onSelectProject: (project?: Project) => void
  onCreateProjectClick?: () => void
  children: React.ReactNode
  hideBottomNav?: boolean
  // Inbox sub-panel
  inboxSubPanel?: React.ReactNode
  // Tasks sub-panel
  tasksSubPanel?: React.ReactNode
  // Projects sub-panel content (project list)
  projectsSubPanel?: React.ReactNode
  // Team sub-panel
  teamSubPanel?: React.ReactNode
  // Mobile drawer controls
  isMobileDrawerOpen?: boolean
  onMobileDrawerOpenChange?: (open: boolean) => void
}

const NAV_ITEMS: { id: AppTab; label: string; icon: React.ElementType }[] = [
  { id: 'inbox', label: 'Inbox', icon: Inbox },
  { id: 'tasks', label: 'Tasks', icon: CheckSquare },
  { id: 'projects', label: 'Projects', icon: FolderGit2 },
  { id: 'team', label: 'Team', icon: Users },
  { id: 'settings', label: 'Settings', icon: Settings },
]

export function AppShell({
  currentTab,
  onSelectTab,
  projects,
  onSelectProject,
  onCreateProjectClick,
  children,
  hideBottomNav = false,
  inboxSubPanel,
  tasksSubPanel,
  projectsSubPanel,
  teamSubPanel,
  isMobileDrawerOpen,
  onMobileDrawerOpenChange,
}: AppShellProps) {
  const [subPanelOpen, setSubPanelOpen] = useState(true)
  const [internalDrawerOpen, setInternalDrawerOpen] = useState(false)
  const isDrawerOpen = isMobileDrawerOpen !== undefined ? isMobileDrawerOpen : internalDrawerOpen
  const setDrawerOpen = (open: boolean) => {
    setInternalDrawerOpen(open)
    if (onMobileDrawerOpenChange) onMobileDrawerOpenChange(open)
  }
  const { resolvedTheme, toggleTheme } = useTheme()

  const getSubPanelContent = () => {
    switch (currentTab) {
      case 'inbox':
        return inboxSubPanel ?? <DefaultSubPanel title="Inbox" description="Pesan & diskusi dengan Scrum Master" />
      case 'tasks':
        return tasksSubPanel ?? <DefaultSubPanel title="Tasks" description="Global Task Board & Backlog" />
      case 'projects':
        return projectsSubPanel ?? (
          <DefaultProjectsSubPanel
            projects={projects}
            onSelectProject={onSelectProject}
            onCreateProjectClick={onCreateProjectClick}
          />
        )
      case 'team':
        return teamSubPanel ?? <DefaultSubPanel title="Team" description="Unified Team: Manusia & AI Agent" />
      case 'settings':
        return <DefaultSubPanel title="Settings" description="Konfigurasi Proyek, Domain & Server" />
    }
  }

  return (
    <div className="h-screen md:h-screen h-[100dvh] w-screen flex overflow-hidden bg-background">
      {/* ── TIER 1: Primary Icon Rail (48px) — Hidden on mobile ── */}
      <nav className="hidden md:flex w-12 shrink-0 flex-col border-r border-border bg-sidebar">
        {/* Logo */}
        <div className="h-12 flex items-center justify-center border-b border-sidebar-border shrink-0">
          <div className="h-7 w-7 bg-primary text-primary-foreground font-mono font-black text-xs flex items-center justify-center rounded">
            C
          </div>
        </div>

        {/* Nav Icons */}
        <div className="flex-1 flex flex-col items-center py-3 gap-1">
          {NAV_ITEMS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => {
                onSelectTab(id)
                setSubPanelOpen(true)
              }}
              title={label}
              className={cn(
                'w-9 h-9 flex items-center justify-center rounded-md transition-colors',
                currentTab === id
                  ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                  : 'text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent/50'
              )}
            >
              <Icon className="h-4 w-4" />
            </button>
          ))}

          {onCreateProjectClick && (
            <button
              onClick={onCreateProjectClick}
              title="Quick Add Project"
              className="w-9 h-9 mt-2 flex items-center justify-center rounded-md text-sidebar-foreground/60 hover:text-primary hover:bg-sidebar-accent/50 transition-colors"
            >
              <Plus className="h-4 w-4" />
            </button>
          )}
        </div>

        {/* Footer: theme toggle + collapse toggle + user avatar */}
        <div className="flex flex-col items-center py-3 gap-2 border-t border-sidebar-border shrink-0">
          <button
            onClick={toggleTheme}
            title={`Tema ${resolvedTheme === 'dark' ? 'Dark' : 'Light'}`}
            className="w-9 h-9 flex items-center justify-center rounded-md text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent/50 transition-colors"
          >
            {resolvedTheme === 'dark' ? (
              <Moon className="h-4 w-4 text-primary" />
            ) : (
              <Sun className="h-4 w-4 text-amber-500" />
            )}
          </button>
          <button
            onClick={() => setSubPanelOpen((o) => !o)}
            title={subPanelOpen ? 'Collapse sub-panel' : 'Expand sub-panel'}
            className="w-9 h-9 flex items-center justify-center rounded-md text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent/50 transition-colors"
          >
            <PanelLeft className="h-4 w-4" />
          </button>
          <NavUser isCollapsed />
        </div>
      </nav>

      {/* ── TIER 2: Contextual Sub-Panel (240px, collapsible) — Hidden on mobile ── */}
      <div
        className={cn(
          'hidden md:flex shrink-0 flex-col border-r border-border bg-sidebar/50 overflow-hidden transition-all duration-200',
          subPanelOpen ? 'w-60' : 'w-0'
        )}
      >
        {subPanelOpen && (
          <>
            {/* Sub-panel Header */}
            <div className="h-12 px-4 flex items-center justify-between border-b border-border shrink-0">
              <span className="text-xs font-semibold text-foreground capitalize">
                {NAV_ITEMS.find((n) => n.id === currentTab)?.label}
              </span>
              <button
                onClick={() => setSubPanelOpen(false)}
                className="p-1 text-muted-foreground hover:text-foreground rounded transition-colors"
              >
                <ChevronLeft className="h-3.5 w-3.5" />
              </button>
            </div>

            {/* Sub-panel Content */}
            <div className="flex-1 overflow-y-auto">
              {getSubPanelContent()}
            </div>
          </>
        )}
      </div>

      {/* ── TIER 3: Main Viewport ── */}
      <main className="flex-1 flex flex-col h-full overflow-hidden">
        {/* Top Bar Header */}
        <header
          className={cn(
            'h-12 border-b border-border px-3 md:px-4 flex items-center justify-between bg-background shrink-0',
            hideBottomNav && 'hidden md:flex'
          )}
        >
          <div className="flex items-center gap-2 text-xs">
            {/* Mobile Hamburger Menu button (☰) */}
            <button
              onClick={() => setDrawerOpen(true)}
              className="md:hidden p-1.5 -ml-1 text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors"
              title="Buka Menu Navigasi"
              aria-label="Open Navigation Drawer"
            >
              <Menu className="h-5 w-5" />
            </button>
            {!subPanelOpen && (
              <button
                onClick={() => setSubPanelOpen(true)}
                className="hidden md:flex p-1.5 text-muted-foreground hover:text-foreground rounded-md transition-colors"
                title="Open sub-panel"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            )}
            {/* Mobile brand badge */}
            <div className="hidden sm:flex md:hidden h-6 w-6 bg-primary text-primary-foreground font-mono font-black text-[11px] items-center justify-center rounded shrink-0">
              C
            </div>
            <div className="flex items-center gap-1.5 text-muted-foreground">
              <span className="font-semibold text-foreground">Cuan</span>
              <span>/</span>
              <span className="capitalize font-medium text-foreground">
                {NAV_ITEMS.find((n) => n.id === currentTab)?.label}
              </span>
            </div>
          </div>

          {/* Mobile Right Actions: Theme Toggle & User Profile */}
          <div className="flex md:hidden items-center gap-1.5">
            <button
              onClick={toggleTheme}
              title={`Tema ${resolvedTheme === 'dark' ? 'Dark' : 'Light'}`}
              className="w-8 h-8 flex items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
            >
              {resolvedTheme === 'dark' ? (
                <Moon className="h-4 w-4 text-primary" />
              ) : (
                <Sun className="h-4 w-4 text-amber-500" />
              )}
            </button>
            <div className="relative">
              <NavUser isCollapsed dropdownPosition="bottom-right" />
            </div>
          </div>
        </header>

        {/* Content Area with pb-16 for mobile bottom navigation (when not hidden) */}
        <div
          className={cn(
            'flex-1 overflow-hidden',
            hideBottomNav ? 'pb-0' : 'pb-16 md:pb-0',
            currentTab === 'inbox' ? 'p-0' : 'overflow-auto p-4 md:p-6'
          )}
        >
          {children}
        </div>
      </main>

      {/* ── Mobile Slide-Over Drawer / Sidebar (< md) ── */}
      {isDrawerOpen && (
        <div
          className="fixed inset-0 bg-black/60 backdrop-blur-xs z-50 md:hidden animate-in fade-in duration-200"
          onClick={() => setDrawerOpen(false)}
          aria-hidden="true"
        />
      )}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 w-72 max-w-[85vw] bg-sidebar border-r border-border z-50 md:hidden flex flex-col shadow-2xl transition-transform duration-300 ease-in-out',
          isDrawerOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        {/* Drawer Header */}
        <div className="h-14 px-4 flex items-center justify-between border-b border-sidebar-border shrink-0">
          <div className="flex items-center gap-2.5">
            <div className="h-7 w-7 bg-primary text-primary-foreground font-mono font-black text-xs flex items-center justify-center rounded">
              C
            </div>
            <div>
              <span className="font-bold text-sm text-foreground">Cuan</span>
              <span className="text-[10px] text-muted-foreground ml-1.5 font-mono">v0.6.0</span>
            </div>
          </div>
          <button
            onClick={() => setDrawerOpen(false)}
            className="p-1.5 text-muted-foreground hover:text-foreground hover:bg-sidebar-accent/50 rounded-md transition-colors"
            title="Tutup Menu"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Drawer Navigation List */}
        <div className="flex-1 overflow-y-auto p-3 space-y-1">
          <div className="px-2 py-1 text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">
            Navigasi Utama
          </div>
          {NAV_ITEMS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => {
                onSelectTab(id)
                setDrawerOpen(false)
              }}
              className={cn(
                'w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-xs font-medium transition-colors text-left',
                currentTab === id
                  ? 'bg-primary text-primary-foreground font-semibold shadow-xs'
                  : 'text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/50'
              )}
            >
              <Icon className="h-4 w-4 shrink-0" />
              <span className="flex-1 truncate">{label}</span>
              {id === 'tasks' && (
                <span
                  className={cn(
                    'text-[10px] px-1.5 py-0.5 rounded font-mono font-medium',
                    currentTab === id
                      ? 'bg-primary-foreground/20 text-primary-foreground'
                      : 'bg-sidebar-accent text-sidebar-foreground'
                  )}
                >
                  Global Board
                </span>
              )}
            </button>
          ))}

          {/* Contextual Sub-panel inside Mobile Drawer */}
          <div className="pt-3 mt-3 border-t border-sidebar-border">
            <div className="px-2 py-1 text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">
              Sub-Panel: {NAV_ITEMS.find((n) => n.id === currentTab)?.label}
            </div>
            <div className="mt-1 bg-background/50 rounded-lg p-2 border border-border/50 text-xs">
              {getSubPanelContent()}
            </div>
          </div>
        </div>

        {/* Drawer Footer */}
        <div className="p-3 border-t border-sidebar-border bg-sidebar/50 shrink-0 flex items-center justify-between">
          <button
            onClick={toggleTheme}
            className="flex items-center gap-2 px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground hover:bg-sidebar-accent/50 rounded-md transition-colors"
          >
            {resolvedTheme === 'dark' ? (
              <>
                <Moon className="h-4 w-4 text-primary" />
                <span>Dark Mode</span>
              </>
            ) : (
              <>
                <Sun className="h-4 w-4 text-amber-500" />
                <span>Light Mode</span>
              </>
            )}
          </button>
          <NavUser isCollapsed dropdownPosition="bottom-right" />
        </div>
      </aside>

      {/* ── Mobile Bottom Navigation Bar (md:hidden) — Hidden when chat is active in mobile ── */}
      {!hideBottomNav && (
        <nav className="md:hidden fixed bottom-0 left-0 right-0 h-16 bg-sidebar border-t border-border z-30 flex items-center justify-around px-2 shadow-lg">
          {NAV_ITEMS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => onSelectTab(id)}
              className={cn(
                'flex flex-col items-center justify-center flex-1 py-1 gap-1 transition-colors relative',
                currentTab === id
                  ? 'text-primary font-bold'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <Icon className="h-5 w-5" />
              <span className="text-[10px] tracking-tight">{label}</span>
            </button>
          ))}
        </nav>
      )}
    </div>
  )
}

// ── Default Sub-panel Components ──

function DefaultSubPanel({ title, description }: { title: string; description: string }) {
  return (
    <div className="p-4 text-center text-muted-foreground">
      <p className="text-xs font-medium text-foreground">{title}</p>
      <p className="text-[11px] mt-1">{description}</p>
    </div>
  )
}

function DefaultProjectsSubPanel({
  projects,
  onSelectProject,
  onCreateProjectClick,
}: {
  projects: Project[]
  onSelectProject: (project?: Project) => void
  onCreateProjectClick?: () => void
}) {
  const active = projects.filter((p) => !p.is_archived)

  return (
    <div className="flex flex-col h-full">
      {/* Create button */}
      {onCreateProjectClick && (
        <div className="p-3 border-b border-border">
          <button
            onClick={onCreateProjectClick}
            className="flex items-center gap-2 w-full px-3 py-2 text-xs font-medium bg-primary text-primary-foreground hover:bg-primary/90 transition-colors rounded-md"
          >
            <Plus className="h-3.5 w-3.5" />
            New Project
          </button>
        </div>
      )}

      {/* Project list */}
      <div className="flex-1 overflow-y-auto py-2">
        {active.length === 0 ? (
          <p className="px-4 py-3 text-[11px] text-muted-foreground italic">No projects yet</p>
        ) : (
          active.map((p) => (
            <button
              key={p.id}
              onClick={() => onSelectProject(p)}
              className="flex items-center gap-2.5 w-full px-3 py-2 text-left text-xs text-foreground/80 hover:bg-accent hover:text-accent-foreground transition-colors"
            >
              <FolderGit2 className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <span className="truncate">{p.name}</span>
            </button>
          ))
        )}
      </div>
    </div>
  )
}
