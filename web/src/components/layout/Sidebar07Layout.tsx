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
import { useTheme } from '../../context/ThemeContext'
import {
  Layers,
  Kanban,
  MessageSquare,
  Rocket,
  Globe,
  GitBranch,
  Server,
  HardDrive,
  Activity,
  ShieldCheck,
  ChevronDown,
  Check,
  ExternalLink,
  Sun,
  Moon,
  FolderGit2,
  Command
} from 'lucide-react'
import { cn } from '../../lib/utils'

export interface ProjectOption {
  id: string
  name: string
  type?: string
  status?: string
}

interface Sidebar07LayoutProps {
  currentPath: string
  onNavigate: (path: string) => void
  activeProject?: ProjectOption
  projects?: ProjectOption[]
  onSelectProject?: (project: ProjectOption) => void
  appMode?: 'marketing' | 'studio'
  onToggleMode?: (newMode: 'marketing' | 'studio') => void
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
  appMode = 'studio',
  onToggleMode,
  children,
}: Sidebar07LayoutProps) {
  const { resolvedTheme, toggleTheme } = useTheme()
  const [projectDropdownOpen, setProjectDropdownOpen] = useState(false)
  const [currentProject, setCurrentProject] = useState<ProjectOption>(
    activeProject || projects[0] || DEFAULT_PROJECTS[0]
  )

  useEffect(() => {
    if (activeProject) {
      setCurrentProject(activeProject)
    }
  }, [activeProject])

  // Derive active navigation item
  const getActiveTab = () => {
    if (currentPath.includes('/tasks')) return 'tasks'
    if (currentPath.includes('/discussions')) return 'discussions'
    if (currentPath.includes('/deployer')) return 'deployer'
    if (currentPath.includes('/wordpress')) return 'wordpress'
    if (currentPath.includes('/repositories')) return 'repositories'
    if (currentPath.includes('/servers')) return 'servers'
    if (currentPath.includes('/domains')) return 'domains'
    if (currentPath.includes('/observability')) return 'observability'
    if (currentPath.includes('/sponsors') || currentPath.includes('/admin')) return 'sponsors'
    return 'projects'
  }

  const activeTab = getActiveTab()

  const handleProjectSelect = (proj: ProjectOption) => {
    setCurrentProject(proj)
    setProjectDropdownOpen(false)
    onSelectProject?.(proj)
  }

  // Breadcrumbs title helper
  const getTabLabel = (tab: string) => {
    switch (tab) {
      case 'tasks': return 'Tasks & Kanban'
      case 'discussions': return 'Project Discussions'
      case 'deployer': return '1-Click Script Deployer'
      case 'wordpress': return 'WordPress Cloud'
      case 'repositories': return 'Repositories'
      case 'servers': return 'BYOS Servers'
      case 'domains': return 'Custom Domains'
      case 'observability': return 'Observability'
      case 'sponsors': return 'Moderasi Sponsor'
      default: return 'Projects Dashboard'
    }
  }

  return (
    <SidebarProvider defaultOpen={true}>
      <div className="flex min-h-screen w-full bg-background text-foreground">
        {/* Modern Sidebar-07 Shell */}
        <Sidebar collapsible="icon" className="border-r border-sidebar-border bg-sidebar">
          {/* Sidebar Header: Workspace / Project Switcher */}
          <SidebarHeader className="border-b border-sidebar-border p-2">
            <div className="relative">
              <button
                type="button"
                onClick={() => setProjectDropdownOpen(!projectDropdownOpen)}
                className="flex w-full items-center gap-2.5 p-2 text-left hover:bg-sidebar-accent hover:text-sidebar-accent-foreground transition-colors group select-none"
                title="Beralih Workspace Proyek"
              >
                <div className="flex h-8 w-8 shrink-0 items-center justify-center bg-primary text-primary-foreground font-mono font-bold text-xs shadow-xs">
                  <Command className="h-4 w-4" />
                </div>
                <div className="grid flex-1 text-left leading-tight truncate">
                  <span className="truncate font-bold text-xs text-sidebar-foreground">
                    {currentProject.name}
                  </span>
                  <span className="truncate text-[10px] text-sidebar-foreground/60 font-mono">
                    {currentProject.type ? `${currentProject.type.toUpperCase()} • ACTIVE` : 'WORKSPACE'}
                  </span>
                </div>
                <ChevronDown className="h-4 w-4 shrink-0 text-sidebar-foreground/50 transition-transform duration-200 group-hover:text-sidebar-foreground" />
              </button>

              {/* Workspace Switcher Popover */}
              {projectDropdownOpen && (
                <div className="absolute left-0 top-full mt-1.5 z-50 w-64 bg-popover text-popover-foreground border border-border shadow-2xl p-1 animate-in fade-in-50 zoom-in-95">
                  <div className="px-2 py-1.5 text-[10px] font-bold uppercase tracking-wider text-muted-foreground border-b border-border/80">
                    Pilih Workspace Proyek
                  </div>
                  <div className="max-h-60 overflow-y-auto py-1">
                    {projects.map((proj) => (
                      <button
                        key={proj.id}
                        type="button"
                        onClick={() => handleProjectSelect(proj)}
                        className={cn(
                          "flex w-full items-center justify-between gap-2 px-2.5 py-2 text-xs transition-colors text-left",
                          currentProject.id === proj.id
                            ? "bg-accent text-accent-foreground font-semibold"
                            : "hover:bg-muted text-foreground"
                        )}
                      >
                        <div className="flex items-center gap-2 truncate">
                          <FolderGit2 className="h-3.5 w-3.5 text-primary shrink-0" />
                          <span className="truncate">{proj.name}</span>
                        </div>
                        {currentProject.id === proj.id && (
                          <Check className="h-3.5 w-3.5 text-primary shrink-0" />
                        )}
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </SidebarHeader>

          {/* Sidebar Content: Navigation Groups */}
          <SidebarContent>
            {/* Group 1: Workspace & Tasks */}
            <SidebarGroup>
              <SidebarGroupLabel>Studio & Tasks</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      isActive={activeTab === 'projects'}
                      onClick={() => onNavigate('/studio')}
                      tooltip="Projects Dashboard"
                    >
                      <Layers className="h-4 w-4 text-emerald-500 shrink-0" />
                      <span>Projects</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>

                  <SidebarMenuItem>
                    <SidebarMenuButton
                      isActive={activeTab === 'tasks'}
                      onClick={() => onNavigate('/studio/tasks')}
                      tooltip="Multi-Project Tasks & Kanban"
                    >
                      <Kanban className="h-4 w-4 text-indigo-400 shrink-0" />
                      <span>Tasks & Kanban</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>

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
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>

            {/* Group 2: Deployments & Cloud */}
            <SidebarGroup>
              <SidebarGroupLabel>Cloud & Deployments</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      isActive={activeTab === 'deployer'}
                      onClick={() => onNavigate('/studio/deployer')}
                      tooltip="1-Click Script Deployer"
                    >
                      <Rocket className="h-4 w-4 text-primary shrink-0" />
                      <span>1-Click Deployer</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>

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

                  <SidebarMenuItem>
                    <SidebarMenuButton
                      isActive={activeTab === 'repositories'}
                      onClick={() => onNavigate('/studio/repositories')}
                      tooltip="Git Repositories"
                    >
                      <GitBranch className="h-4 w-4 text-amber-400 shrink-0" />
                      <span>Repositories</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>

            {/* Group 3: Infrastructure */}
            <SidebarGroup>
              <SidebarGroupLabel>Infrastruktur</SidebarGroupLabel>
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
                      isActive={activeTab === 'observability'}
                      onClick={() => onNavigate('/studio/observability')}
                      tooltip="Observability & Health"
                    >
                      <Activity className="h-4 w-4 text-rose-400 shrink-0" />
                      <span>Observability</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>

            {/* Group 4: Admin Platform */}
            <SidebarGroup>
              <SidebarGroupLabel>Admin & Sponsors</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
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
          </SidebarContent>

          {/* Sidebar Footer: NavUser with Avatar, Profile & Logout */}
          <SidebarFooter>
            <NavUser />
          </SidebarFooter>

          <SidebarRail />
        </Sidebar>

        {/* Sidebar Inset: Header + Main Content */}
        <SidebarInset>
          {/* Top Header Bar */}
          <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center justify-between border-b border-border bg-background/95 px-4 backdrop-blur-sm">
            <div className="flex items-center gap-3">
              <SidebarTrigger />
              <div className="h-4 w-[1px] bg-border" />

              {/* Breadcrumb Navigation */}
              <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span className="font-semibold text-foreground">Cuan</span>
                <span>/</span>
                <span className="hidden sm:inline font-medium text-foreground/80">{currentProject.name}</span>
                <span className="hidden sm:inline">/</span>
                <span className="text-primary font-semibold">{getTabLabel(activeTab)}</span>
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
                    "hidden sm:flex items-center gap-1.5 px-2 py-1 text-[11px] font-mono border transition-colors",
                    appMode === 'studio'
                      ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-400"
                      : "border-blue-500/40 bg-blue-500/10 text-blue-400"
                  )}
                  title="Klik untuk beralih mode runtime"
                >
                  <span className={cn("h-1.5 w-1.5 rounded-full", appMode === 'studio' ? "bg-emerald-400 animate-pulse" : "bg-blue-400")} />
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
    </SidebarProvider>
  )
}
