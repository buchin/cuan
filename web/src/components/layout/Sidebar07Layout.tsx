import { useState, useEffect } from 'react'
import {
  SidebarProvider,
  Sidebar,
  SidebarHeader,
  SidebarContent,
  SidebarFooter,
  SidebarInset,
  SidebarTrigger,
  SidebarRail,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarGroupContent
} from '../ui/sidebar'
import { NavUser } from './NavUser'
import { BreadcrumbProjectSwitcher } from './BreadcrumbProjectSwitcher'
import { CreateProjectModal } from '../studio/CreateProjectModal'
import { useTheme } from '../../context/ThemeContext'
import {
  LayoutDashboard,
  Kanban,
  Layers,
  MessageSquare,
  Bot,
  Users,
  Globe,
  GitBranch,
  Rocket,
  Activity,
  Settings2,
  Server,
  HardDrive,
  ShieldCheck,
  ArrowLeft,
  ChevronRight,
  ExternalLink,
  Sun,
  Moon,
  Command,
  FolderGit2
} from 'lucide-react'
import { cn } from '../../lib/utils'

export interface ProjectOption {
  id: string
  name: string
  type?: string
  status?: string
  is_archived?: boolean
  repo_url?: string
  enabled_modules?: string[]
}

interface Sidebar07LayoutProps {
  currentPath: string
  onNavigate: (path: string) => void
  activeProject?: ProjectOption
  projects?: ProjectOption[]
  onSelectProject?: (project: ProjectOption) => void
  onCreateProject?: (projectData: any) => Promise<void> | void
  appMode?: 'marketing' | 'studio'
  onToggleMode?: (newMode: 'marketing' | 'studio') => void
  navMode?: 'global' | 'project'
  onToggleNavMode?: (mode: 'global' | 'project') => void
  children: React.ReactNode
}

const DEFAULT_PROJECTS: ProjectOption[] = [
  { id: 'cuan-core', name: 'Cuan Platform Core', type: 'saas', status: 'active' },
  { id: 'saas-dojo', name: 'SaaS / Dojo Workspace', type: 'saas', status: 'active' },
  { id: 'creator-hub', name: 'Creator Hub Workspace', type: 'creator', status: 'active' },
  { id: 'blank-workspace', name: 'Blank Workspace', type: 'blank', status: 'active' },
]

export function Sidebar07Layout({
  currentPath,
  onNavigate,
  activeProject,
  projects = DEFAULT_PROJECTS,
  onSelectProject,
  onCreateProject,
  appMode = 'studio',
  onToggleMode,
  navMode: propNavMode,
  onToggleNavMode,
  children,
}: Sidebar07LayoutProps) {
  const { resolvedTheme, toggleTheme } = useTheme()
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [currentProject, setCurrentProject] = useState<ProjectOption>(
    activeProject || projects[0] || DEFAULT_PROJECTS[0]
  )

  // Internal navigation mode: 'global' or 'project'
  const [internalNavMode, setInternalNavMode] = useState<'global' | 'project'>(() => {
    if (
      currentPath.includes('/wordpress') ||
      currentPath.includes('/repositories') ||
      currentPath.includes('/deployer') ||
      currentPath.includes('/observability') ||
      currentPath.includes('/settings') ||
      currentPath.includes('/project/')
    ) {
      return 'project'
    }
    return 'global'
  })

  const effectiveNavMode = propNavMode !== undefined ? propNavMode : internalNavMode

  const setEffectiveNavMode = (mode: 'global' | 'project') => {
    setInternalNavMode(mode)
    onToggleNavMode?.(mode)
  }

  // Update current project if activeProject prop changes
  useEffect(() => {
    if (activeProject) {
      setCurrentProject(activeProject)
    }
  }, [activeProject])

  // Sync navMode based on deep routes if propNavMode is not explicitly controlled
  useEffect(() => {
    if (propNavMode === undefined) {
      if (
        currentPath.includes('/wordpress') ||
        currentPath.includes('/repositories') ||
        currentPath.includes('/deployer') ||
        currentPath.includes('/observability') ||
        currentPath.includes('/settings') ||
        currentPath.includes('/project/')
      ) {
        setInternalNavMode('project')
      }
    }
  }, [currentPath, propNavMode])

  const handleProjectSelect = (proj: ProjectOption) => {
    setCurrentProject(proj)
    onSelectProject?.(proj)
  }

  const handleCreateProjectSubmit = async (projData: any) => {
    if (onCreateProject) {
      await onCreateProject(projData)
    }
    setCurrentProject(projData)
    setEffectiveNavMode('project')
  }

  // Derive active tab based on path
  const getActiveTab = () => {
    if (currentPath.includes('/tasks')) return 'tasks'
    if (currentPath.includes('/discussions')) return 'discussions'
    if (currentPath.includes('/agents')) return 'agents'
    if (currentPath.includes('/team')) return 'team'
    if (currentPath.includes('/deployer')) return 'deployer'
    if (currentPath.includes('/wordpress')) return 'wordpress'
    if (currentPath.includes('/repositories')) return 'repositories'
    if (currentPath.includes('/servers')) return 'servers'
    if (currentPath.includes('/domains')) return 'domains'
    if (currentPath.includes('/observability')) return 'observability'
    if (currentPath.includes('/sponsors') || currentPath.includes('/admin')) return 'sponsors'
    if (currentPath.includes('/settings')) return 'settings'
    if (currentPath === '/projects' || currentPath === '/studio/projects') return 'projects'
    return 'dashboard'
  }

  const activeTab = getActiveTab()

  // Label for the last leaf of the breadcrumb
  const getTabLabel = (tab: string) => {
    switch (tab) {
      case 'dashboard':
        return effectiveNavMode === 'project' ? 'Overview' : 'Dashboard'
      case 'tasks':
        return 'Tasks & Kanban'
      case 'projects':
        return 'Projects Directory'
      case 'discussions':
        return effectiveNavMode === 'project' ? 'Studio Rooms' : 'Discussions'
      case 'agents':
        return 'AI Agents Hub'
      case 'team':
        return 'Team Workspace'
      case 'wordpress':
        return 'WordPress Cloud'
      case 'repositories':
        return 'Repositories'
      case 'deployer':
        return '1-Click Deployer'
      case 'observability':
        return 'Telemetry & Health'
      case 'servers':
        return 'BYOS Servers'
      case 'domains':
        return 'Custom Domains'
      case 'sponsors':
        return 'Moderasi Sponsor'
      case 'settings':
        return 'Settings'
      default:
        return 'Overview'
    }
  }

  return (
    <SidebarProvider defaultOpen={true}>
      <div className="flex min-h-screen w-full bg-background text-foreground">
        {/* Modern Sidebar-07 Shell */}
        <Sidebar collapsible="icon" className="border-r border-sidebar-border bg-sidebar">
          {/* Sidebar Header: Switches between Global Nav & Contextual Project Modul */}
          <SidebarHeader className="border-b border-sidebar-border p-2.5">
            {effectiveNavMode === 'project' ? (
              /* Contextual Project Header with Back to Global Navigation */
              <div className="space-y-2">
                <button
                  type="button"
                  onClick={() => {
                    setEffectiveNavMode('global')
                    onNavigate('/studio')
                  }}
                  className="flex w-full items-center gap-2 p-1.5 text-xs font-semibold text-muted-foreground hover:text-foreground hover:bg-sidebar-accent transition-colors rounded-xs group select-none"
                  title="Kembali ke Navigasi Global Platform"
                >
                  <ArrowLeft className="h-3.5 w-3.5 text-primary transition-transform group-hover:-translate-x-0.5" />
                  <span className="truncate">Navigasi Global</span>
                </button>

                <div className="flex items-center gap-2.5 p-1.5 bg-sidebar-accent/50 border border-sidebar-border rounded-xs">
                  <div className="flex h-7 w-7 shrink-0 items-center justify-center bg-primary text-primary-foreground font-mono font-bold text-xs">
                    <FolderGit2 className="h-3.5 w-3.5" />
                  </div>
                  <div className="grid flex-1 text-left leading-tight truncate">
                    <span className="truncate font-bold text-xs text-sidebar-foreground">
                      {currentProject.name}
                    </span>
                    <span className="truncate text-[9px] text-sidebar-foreground/60 font-mono uppercase">
                      Modul Proyek • {currentProject.type || 'SaaS'}
                    </span>
                  </div>
                </div>
              </div>
            ) : (
              /* Global Navigation Header */
              <div className="flex items-center justify-between p-1.5">
                <div className="flex items-center gap-2">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center bg-primary text-primary-foreground font-mono font-bold text-xs shadow-xs">
                    <Command className="h-4 w-4" />
                  </div>
                  <div className="grid text-left leading-tight">
                    <span className="font-bold text-xs text-sidebar-foreground">
                      Cuan Platform
                    </span>
                    <span className="text-[10px] text-sidebar-foreground/60 font-mono">
                      Global Workspace
                    </span>
                  </div>
                </div>

                <button
                  type="button"
                  onClick={() => setEffectiveNavMode('project')}
                  className="p-1 text-muted-foreground hover:text-foreground hover:bg-sidebar-accent transition-colors rounded-xs"
                  title="Buka Modul Proyek Kontekstual"
                >
                  <ChevronRight className="h-4 w-4" />
                </button>
              </div>
            )}
          </SidebarHeader>

          {/* Sidebar Content: Global Nav vs Contextual Project Modules */}
          <SidebarContent>
            {effectiveNavMode === 'global' ? (
              /* ============================================================== */
              /* 1. GLOBAL NAVIGATION MENU (S14.3 Requirement 1)                 */
              /* Menu: Dashboard, Tasks, Projects, Discussions, Agents, Team    */
              /* ============================================================== */
              <>
                <SidebarGroup>
                  <SidebarGroupLabel>Menu Utama</SidebarGroupLabel>
                  <SidebarGroupContent>
                    <SidebarMenu>
                      {/* 1. Dashboard */}
                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'dashboard' && effectiveNavMode === 'global'}
                          onClick={() => {
                            setEffectiveNavMode('global')
                            onNavigate('/studio')
                          }}
                          tooltip="Dashboard"
                        >
                          <LayoutDashboard className="h-4 w-4 text-emerald-400 shrink-0" />
                          <span>Dashboard</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>

                      {/* 2. Tasks */}
                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'tasks'}
                          onClick={() => onNavigate('/studio/tasks')}
                          tooltip="Tasks & Kanban"
                        >
                          <Kanban className="h-4 w-4 text-indigo-400 shrink-0" />
                          <span>Tasks</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>

                      {/* 3. Projects */}
                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'projects'}
                          onClick={() => onNavigate('/studio/projects')}
                          tooltip="Projects Directory"
                        >
                          <Layers className="h-4 w-4 text-blue-400 shrink-0" />
                          <span>Projects</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>

                      {/* 4. Discussions */}
                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'discussions'}
                          onClick={() => onNavigate('/studio/discussions')}
                          tooltip="Discussions"
                        >
                          <MessageSquare className="h-4 w-4 text-cyan-400 shrink-0" />
                          <span>Discussions</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>

                      {/* 5. Agents */}
                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'agents'}
                          onClick={() => onNavigate('/studio/agents')}
                          tooltip="AI Agents Hub"
                        >
                          <Bot className="h-4 w-4 text-purple-400 shrink-0" />
                          <span>Agents</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>

                      {/* 6. Team */}
                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'team'}
                          onClick={() => onNavigate('/studio/team')}
                          tooltip="Team & Collaborators"
                        >
                          <Users className="h-4 w-4 text-amber-400 shrink-0" />
                          <span>Team</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    </SidebarMenu>
                  </SidebarGroupContent>
                </SidebarGroup>

                {/* Secondary Group: Platform & Infra */}
                <SidebarGroup>
                  <SidebarGroupLabel>Infrastruktur & Admin</SidebarGroupLabel>
                  <SidebarGroupContent>
                    <SidebarMenu>
                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'servers'}
                          onClick={() => onNavigate('/studio/servers')}
                          tooltip="BYOS Remote Servers"
                        >
                          <Server className="h-4 w-4 text-purple-400 shrink-0" />
                          <span>BYOS Servers</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>

                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'domains'}
                          onClick={() => onNavigate('/studio/domains')}
                          tooltip="Custom Domains & Caddy TLS"
                        >
                          <HardDrive className="h-4 w-4 text-teal-400 shrink-0" />
                          <span>Custom Domains</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>

                      <SidebarMenuItem>
                        <SidebarMenuButton
                          isActive={activeTab === 'sponsors'}
                          onClick={() => onNavigate('/admin/sponsors')}
                          tooltip="Panel Moderasi Sponsor Anti-Judol"
                        >
                          <ShieldCheck className="h-4 w-4 text-emerald-400 shrink-0" />
                          <span>Moderasi Sponsor</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    </SidebarMenu>
                  </SidebarGroupContent>
                </SidebarGroup>
              </>
            ) : (
              /* ============================================================== */
              /* 2. CONTEXTUAL PROJECT MODULE SIDEBAR (S14.3 Requirement 3)      */
              /* Menu: Overview, WordPress Cloud, Repos, Deployments, Telemetry, */
              /*       Studio Rooms, Settings                                   */
              /* ============================================================== */
              <SidebarGroup>
                <SidebarGroupLabel>Modul Proyek: {currentProject.name}</SidebarGroupLabel>
                <SidebarGroupContent>
                  <SidebarMenu>
                    {/* 1. Overview */}
                    <SidebarMenuItem>
                      <SidebarMenuButton
                        isActive={activeTab === 'dashboard' && effectiveNavMode === 'project'}
                        onClick={() => onNavigate('/studio')}
                        tooltip="Project Overview"
                      >
                        <LayoutDashboard className="h-4 w-4 text-emerald-400 shrink-0" />
                        <span>Overview</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>

                    {/* 2. WordPress Cloud / Turnkey */}
                    <SidebarMenuItem>
                      <SidebarMenuButton
                        isActive={activeTab === 'wordpress'}
                        onClick={() => onNavigate('/studio/wordpress')}
                        tooltip="WordPress Cloud Studio"
                      >
                        <Globe className="h-4 w-4 text-blue-400 shrink-0" />
                        <span>WordPress Cloud</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>

                    {/* 3. Repos */}
                    <SidebarMenuItem>
                      <SidebarMenuButton
                        isActive={activeTab === 'repositories'}
                        onClick={() => onNavigate('/studio/repositories')}
                        tooltip="Git Repositories & Sync"
                      >
                        <GitBranch className="h-4 w-4 text-amber-400 shrink-0" />
                        <span>Repos</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>

                    {/* 4. Deployments */}
                    <SidebarMenuItem>
                      <SidebarMenuButton
                        isActive={activeTab === 'deployer'}
                        onClick={() => onNavigate('/studio/deployer')}
                        tooltip="1-Click Deployer"
                      >
                        <Rocket className="h-4 w-4 text-primary shrink-0" />
                        <span>Deployments</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>

                    {/* 5. Telemetry */}
                    <SidebarMenuItem>
                      <SidebarMenuButton
                        isActive={activeTab === 'observability'}
                        onClick={() => onNavigate('/studio/observability')}
                        tooltip="Telemetry, Observability & Alerts"
                      >
                        <Activity className="h-4 w-4 text-rose-400 shrink-0" />
                        <span>Telemetry</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>

                    {/* 6. Studio Rooms */}
                    <SidebarMenuItem>
                      <SidebarMenuButton
                        isActive={activeTab === 'discussions'}
                        onClick={() => onNavigate('/studio/discussions')}
                        tooltip="Studio Rooms & AI Agent Chat"
                      >
                        <MessageSquare className="h-4 w-4 text-cyan-400 shrink-0" />
                        <span>Studio Rooms</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>

                    {/* 7. Settings */}
                    <SidebarMenuItem>
                      <SidebarMenuButton
                        isActive={activeTab === 'settings' || activeTab === 'domains' || activeTab === 'servers'}
                        onClick={() => onNavigate('/studio/domains')}
                        tooltip="Project Settings & Infra"
                      >
                        <Settings2 className="h-4 w-4 text-zinc-400 shrink-0" />
                        <span>Settings</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
            )}
          </SidebarContent>

          {/* Sidebar Footer: NavUser with Avatar, Profile & Logout */}
          <SidebarFooter>
            <NavUser />
          </SidebarFooter>

          <SidebarRail />
        </Sidebar>

        {/* Sidebar Inset: Header + Main Content */}
        <SidebarInset>
          {/* ============================================================== */}
          {/* NAVBAR HEADER BREADCRUMB & SWITCHER (S14.3 Requirement 2)      */}
          {/* Format: Dashboard / [Project Switcher] / [Page / Modul]        */}
          {/* ============================================================== */}
          <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center justify-between border-b border-border bg-background/95 px-4 backdrop-blur-sm">
            <div className="flex items-center gap-3">
              <SidebarTrigger />
              <div className="h-4 w-[1px] bg-border" />

              {/* Breadcrumb Navigation */}
              <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-xs text-muted-foreground">
                {/* 1. Dashboard (Clickable root that resets to global navigation) */}
                <button
                  type="button"
                  onClick={() => {
                    setEffectiveNavMode('global')
                    onNavigate('/studio')
                  }}
                  className="font-semibold text-foreground hover:text-primary transition-colors cursor-pointer select-none"
                  title="Kembali ke Dashboard Utama"
                >
                  Dashboard
                </button>

                <span className="text-muted-foreground/60 select-none">/</span>

                {/* 2. Project Switcher Dropdown (with Search, Badge & 1-Click Switch) */}
                <BreadcrumbProjectSwitcher
                  currentProject={currentProject}
                  projects={projects}
                  onSelectProject={handleProjectSelect}
                  onCreateProjectClick={() => setIsCreateModalOpen(true)}
                />

                <span className="hidden sm:inline text-muted-foreground/60 select-none">/</span>

                {/* 3. Page / Modul Name */}
                <span className="hidden sm:inline text-primary font-semibold truncate max-w-[150px] md:max-w-[220px]">
                  {getTabLabel(activeTab)}
                </span>
              </nav>
            </div>

            {/* Right Top Header Actions */}
            <div className="flex items-center gap-2.5">
              {/* App Mode Switcher (S03.5) */}
              {onToggleMode && (
                <button
                  type="button"
                  onClick={() => onToggleMode(appMode === 'studio' ? 'marketing' : 'studio')}
                  className={cn(
                    'hidden sm:flex items-center gap-1.5 px-2 py-1 text-[11px] font-mono border transition-colors',
                    appMode === 'studio'
                      ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-400'
                      : 'border-blue-500/40 bg-blue-500/10 text-blue-400'
                  )}
                  title="Klik untuk beralih mode runtime"
                >
                  <span
                    className={cn(
                      'h-1.5 w-1.5 rounded-full',
                      appMode === 'studio' ? 'bg-emerald-400 animate-pulse' : 'bg-blue-400'
                    )}
                  />
                  Mode: {appMode}
                </button>
              )}

              {/* External Landing Page */}
              <button
                type="button"
                onClick={() => onNavigate('/')}
                className="hidden md:flex items-center gap-1.5 px-2.5 py-1 text-xs text-muted-foreground hover:text-foreground transition-colors border border-transparent hover:border-border"
                title="Buka Landing Page"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                <span>Beranda</span>
              </button>

              {/* Tokyo Night Theme Toggle */}
              <button
                type="button"
                onClick={toggleTheme}
                aria-label="Toggle Theme"
                className="flex h-8 w-8 items-center justify-center border border-border bg-card text-foreground hover:bg-accent transition-colors"
                title={`Beralih ke tema ${resolvedTheme === 'dark' ? 'Light' : 'Tokyo Night Dark'}`}
              >
                {resolvedTheme === 'dark' ? (
                  <Moon className="h-4 w-4 text-primary" />
                ) : (
                  <Sun className="h-4 w-4 text-amber-500" />
                )}
              </button>
            </div>
          </header>

          {/* Child Content Panes */}
          <div className="flex-1 p-4 md:p-6 bg-background text-foreground transition-colors">
            {children}
          </div>
        </SidebarInset>
      </div>

      {/* Global Interactive Create New Project Modal */}
      <CreateProjectModal
        isOpen={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        onCreateProject={handleCreateProjectSubmit}
      />
    </SidebarProvider>
  )
}
