import { PageHeader, PageHeaderBrand, PageTitle } from './PageHeader'
import { getSettingsBackTarget } from '../../lib/settings-navigation'
import { IconButton } from '../ui/IconButton'
import { KeyboardShortcuts } from './KeyboardShortcuts'
import { UniversalSearch } from './UniversalSearch'
import { useLocation, useNavigate } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useRef, useState, type ComponentType } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { CuanMark } from '../brand/CuanMark'
import { NavUser } from './NavUser'
import { CreateAction } from './CreateAction'
import { SlideDrawer } from './SlideDrawer'
import {
  Inbox,
  FolderGit2,
  PanelLeft,
  Users,
  Settings,
  Plus,
} from 'lucide-react'
import { cn } from '../../lib/utils'

export type AppTab =
  | 'notifications'
  | 'chat'
  | 'studio'
  | 'inbox'
  | 'tasks'
  | 'schedules'
  | 'projects'
  | 'contacts'
  | 'agents'
  | 'team'
  | 'skills'
  | 'settings'
  | 'account'
interface AppShellProps {
  currentTab: AppTab
  onSelectTab: (tab: AppTab) => void
  children: React.ReactNode
  headerActions?: React.ReactNode
  secondaryNavigation?: React.ReactNode
  newAction?: { label: string; onClick: () => void }
  hideBottomNav?: boolean
  sessionWorkspace?: boolean
  projectWorkspace?: boolean
  contentHeader?: boolean
  inboxSubPanel?: React.ReactNode
  inboxUnread?: number
  inboxUnreadCount?: number
  pageTitleIsContent?: boolean
  pageTitle?: string
  breadcrumbs?: { label: string; to: string }[]
  hideSearch?: boolean
}
const NAV_ITEMS = [
  { id: 'studio', label: 'New work', icon: Plus },
  { id: 'inbox', label: 'Inbox', icon: Inbox },
  { id: 'projects', label: 'Projects', icon: FolderGit2 },
] as const

export interface NavItemConfig {
  id: AppTab
  label: string
  icon: ComponentType<{ className?: string }>
  to: string
  description?: string
}

export interface NavGroupConfig {
  id: 'main' | 'manage' | 'workspace' | 'operations' | 'system'
  label?: string
  items: NavItemConfig[]
}

export const NAV_GROUPS: NavGroupConfig[] = [
  {
    id: 'main',
    label: '',
    items: [
      { id: 'studio', label: 'New work', icon: Plus, to: '/studio' },
      { id: 'inbox', label: 'Inbox', icon: Inbox, to: '/inbox', description: 'Chat & pesan masuk' },
    ],
  },
  {
    id: 'manage',
    label: 'Manage',
    items: [
      { id: 'projects', label: 'Projects', icon: FolderGit2, to: '/projects', description: 'Workspaces & repo' },
      { id: 'contacts', label: 'Contacts', icon: Users, to: '/contacts', description: 'CRM & memori pelanggan' },
      { id: 'settings', label: 'Settings', icon: Settings, to: '/settings', description: 'Pengaturan & preferensi' },
    ],
  },
]

export function AppShell({
  currentTab,
  onSelectTab,
  children,
  headerActions,
  secondaryNavigation,
  newAction,
  hideBottomNav = false,
  sessionWorkspace = false,
  projectWorkspace = false,
  contentHeader = false,
  inboxSubPanel,
  inboxUnread = 0,
  inboxUnreadCount,
  pageTitleIsContent = false,
  pageTitle,
  breadcrumbs = [],
  hideSearch,
}: AppShellProps) {
  const { t } = useLanguage()
  const { user } = useAuth()
  const effectiveInboxUnread = inboxUnreadCount ?? inboxUnread

  const routerNavigate = useNavigate()
  const location = useLocation()
  const settingsBack = getSettingsBackTarget(location.pathname, location.search)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  useEffect(() => {
    setMobileMenuOpen(false)
  }, [location.pathname])

  // Eliminate redundant top navigation search icon on views with internal search inputs/filters or settings/account
  const hasInternalSearch =
    currentTab === 'tasks' ||
    currentTab === 'inbox' ||
    currentTab === 'projects' ||
    currentTab === 'contacts' ||
    currentTab === 'settings' ||
    currentTab === 'account' ||
    location.pathname.startsWith('/account/contacts') ||
    location.pathname.startsWith('/settings/agents') ||
    location.pathname.startsWith('/settings/agent') ||
    location.pathname.startsWith('/settings/team')

  const shouldHideSearch = hideSearch ?? hasInternalSearch
  const shell = useRef<HTMLDivElement>(null)
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem('cuan-nav-collapsed') === 'true'
    } catch {
      return false
    }
  })
  const [sessionExpanded, setSessionExpanded] = useState(false)
  const effectiveCollapsed = sessionWorkspace ? !sessionExpanded : collapsed
  const [listOpen, setListOpen] = useState(true)
  useEffect(() => {
    const viewport = window.visualViewport
    const update = () => {
      if (viewport) {
        shell.current?.style.setProperty(
          '--visible-height',
          `${viewport.height}px`,
        )
        shell.current?.style.setProperty(
          '--visible-top',
          `${viewport.offsetTop}px`,
        )
      }
    }
    update()
    viewport?.addEventListener('resize', update)
    viewport?.addEventListener('scroll', update)
    return () => {
      viewport?.removeEventListener('resize', update)
      viewport?.removeEventListener('scroll', update)
    }
  }, [])
  const isTabActive = (id: AppTab) => {
    if (id === 'inbox' || id === 'chat') {
      return (
        currentTab === 'inbox' ||
        currentTab === 'chat'
      )
    }
    if (id === 'agents') {
      return location.pathname.startsWith('/settings/agent')
    }
    if (id === 'team') {
      return location.pathname.startsWith('/settings/team')
    }
    if (id === 'skills') {
      return (
        location.pathname.startsWith('/settings/apps-skills') ||
        location.pathname.startsWith('/settings/skills')
      )
    }
    if (id === 'schedules') {
      return (
        currentTab === 'schedules' ||
        location.pathname.startsWith('/settings/schedules')
      )
    }
    if (id === 'settings') {
      return currentTab === 'settings'
    }
    return currentTab === id
  }

  const label =
    currentTab === 'notifications'
      ? 'Notifications'
      : currentTab === 'tasks'
        ? t('Tasks')
        : currentTab === 'settings'
          ? t('Settings')
          : currentTab === 'account'
            ? t('Account')
            : currentTab === 'contacts'
              ? t('Contacts')
              : currentTab === 'schedules'
                ? t('Schedules')
                : currentTab === 'projects'
                  ? t('Projects')
                  : currentTab === 'studio'
                    ? t('New work')
                    : currentTab === 'inbox' || currentTab === 'chat'
                    ? t('Inbox')
                    : (NAV_ITEMS as readonly { id: string; label: string }[]).find((n) => n.id === (currentTab as string))?.label

  useEffect(() => {
    const rawTitle = pageTitleIsContent ? pageTitle : t(pageTitle || label || '')
    document.title = rawTitle ? `${rawTitle} — Cuan` : 'Cuan — Your always-on project manager'
  }, [pageTitleIsContent, pageTitle, label, t])

  const navigate = (id: AppTab) => {
    onSelectTab(id)
  }
  const navButton = (
    id: AppTab,
    text: string,
    Icon: ComponentType<{ className?: string }>,
  ) => {
    const active = isTabActive(id)
    return (
      <button
        key={id}
        data-nav-tone={id}
        title={t(text)}
        aria-label={t(text)}
        aria-current={active ? 'page' : undefined}
        onClick={() => navigate(id)}
        className={cn(
          'nav-action transition-colors',
          active
            ? 'bg-sidebar-accent text-primary font-semibold'
            : 'text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-foreground',
        )}
      >
        <span className="relative flex items-center justify-center">
          {id === 'chat' ? (
            <CuanMark decorative className="h-4 w-4 text-inherit" />
          ) : (
            <Icon className="h-4 w-4 shrink-0 text-inherit" />
          )}
          {id === 'inbox' && effectiveInboxUnread > 0 && effectiveCollapsed && (
            <span
              data-testid="inbox-unread-badge-rail"
              aria-label={`${effectiveInboxUnread} ${t('unread messages')}`}
              className="absolute -right-2 -top-1.5 notification-count min-w-4 rounded-full bg-primary px-1 text-[9px] font-bold font-mono leading-4 text-primary-foreground"
            >
              {effectiveInboxUnread > 99 ? '99+' : effectiveInboxUnread}
            </span>
          )}
        </span>
        <span className="nav-label flex-1 text-left">{t(text)}</span>
        {id === 'inbox' && effectiveInboxUnread > 0 && !effectiveCollapsed && (
          <span
            data-testid="inbox-unread-badge-rail"
            aria-label={`${effectiveInboxUnread} ${t('unread messages')}`}
            className="notification-count ml-auto self-center inline-flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-bold font-mono leading-none tabular-nums text-primary-foreground"
          >
            {effectiveInboxUnread > 99 ? '99+' : effectiveInboxUnread}
          </span>
        )}
      </button>
    )
  }
  return (
    <div
      ref={shell}
      data-session-workspace={sessionWorkspace ? 'true' : undefined}
      className={cn(
        'app-shell flex w-full overflow-hidden bg-background',
        effectiveCollapsed && 'nav-collapsed',
      )}
    >
      <KeyboardShortcuts />
      <nav
        aria-label={t('Main navigation')}
        className="primary-navigation hidden shrink-0 flex-col border-r border-border bg-sidebar md:flex"
      >
        <div className="sidebar-brand flex h-13 shrink-0 items-center justify-between border-b border-border px-4 box-border">
          <Link
            to="/studio"
            aria-label={t('Start new work')}
            className="inline-flex h-8 items-center rounded-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
          >
            <CuanMark decorative className="h-7 w-7 shrink-0 text-foreground" />
          </Link>
          <button
            className="sidebar-collapse icon-action hidden min-[1200px]:inline-flex h-7 w-7"
            title={
              effectiveCollapsed ? t('Expand sidebar') : t('Collapse sidebar')
            }
            aria-label={
              effectiveCollapsed ? t('Expand sidebar') : t('Collapse sidebar')
            }
            onClick={() =>
              sessionWorkspace
                ? setSessionExpanded((value) => !value)
                : setCollapsed((value) => {
                    const next = !value
                    try {
                      localStorage.setItem('cuan-nav-collapsed', String(next))
                    } catch {}
                    return next
                  })
            }
          >
            <PanelLeft className="h-3.5 w-3.5" />
          </button>
        </div>
        <div className="flex-1 space-y-3 overflow-y-auto p-2">
          {NAV_GROUPS.map((group, index) => (
            <div
              key={group.id}
              data-testid={`nav-group-${group.id}`}
              className="space-y-0.5"
            >
              {!effectiveCollapsed ? (
                group.label ? (
                  <div className="px-3 pt-2.5 pb-1 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">
                    {t(group.label)}
                  </div>
                ) : index > 0 ? (
                  <div className="my-1.5 h-px bg-border/40" />
                ) : null
              ) : (
                index > 0 && <div className="my-1.5 h-px bg-border/40" />
              )}
              {group.items.map((n) => navButton(n.id, n.label, n.icon))}
            </div>
          ))}
        </div>
        <div className="border-t border-border p-2">
          <NavUser
            sidebar
            collapsed={effectiveCollapsed}
            active={currentTab === 'account'}
            onWork={() => navigate('tasks')}
            onSettings={() => navigate('settings')}
          />
        </div>
      </nav>
      {(currentTab === 'inbox' || currentTab === 'chat') && listOpen && (
        <section
          aria-label={t('Conversations')}
          className="conversation-sidebar hidden h-full w-[320px] min-[1200px]:w-[340px] shrink-0 flex-col border-r border-border min-[1200px]:flex"
        >
          <div className="min-h-0 flex-1 overflow-hidden">{inboxSubPanel}</div>
        </section>
      )}
      <main className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <PageHeader
          className={cn(
            currentTab === 'studio' || sessionWorkspace || contentHeader || currentTab === 'inbox' || currentTab === 'chat'
              ? 'hidden'
              : hideBottomNav && 'hidden md:flex',
          )}
        >
          <div className="flex min-w-0 items-center gap-2">
            {settingsBack ? (
              <Link to={settingsBack.to} aria-label={t(settingsBack.label)} title={t(settingsBack.label)} className="icon-action shrink-0" data-settings-back>
                <ArrowLeft aria-hidden="true" />
              </Link>
            ) : currentTab === 'notifications' ? (
              <button
                type="button"
                className="icon-action shrink-0"
                aria-label={t('Back')}
                onClick={() => {
                  const from = location.state?.notificationReturnTo
                  if (
                    typeof from === 'string' &&
                    from.startsWith('/') &&
                    !from.startsWith('//') &&
                    !from.split(/[?#]/)[0].startsWith('/notifications')
                  ) {
                    if (window.history.state?.idx > 0) routerNavigate(-1)
                    else routerNavigate(from, { replace: true })
                  } else routerNavigate('/chat', { replace: true })
                }}
              >
                <ArrowLeft className="h-5 w-5" />
              </button>
            ) : (
              <PageHeaderBrand />
            )}
            {!settingsBack && breadcrumbs.length > 0 && (
              <nav
                aria-label={t('Breadcrumbs')}
                className="hidden min-w-0 items-center gap-2 text-sm text-muted-foreground md:flex"
              >
                {breadcrumbs.map((crumb) => (
                  <span
                    key={crumb.to}
                    className="flex min-w-0 items-center gap-2"
                  >
                    <Link
                      className="inline-flex min-h-11 max-w-40 items-center hover:text-foreground focus-visible:underline"
                      to={crumb.to}
                    >
                      <span className="truncate">{crumb.label}</span>
                    </Link>
                    <span aria-hidden="true">/</span>
                  </span>
                ))}
              </nav>
            )}
            <PageTitle>
              {pageTitleIsContent ? pageTitle : t(pageTitle || label || '')}
            </PageTitle>

          </div>
          <div className="flex shrink-0 items-center gap-1">
            <UniversalSearch hideTrigger={shouldHideSearch} />
            {headerActions}
            {currentTab === 'inbox' && (
              <IconButton
                className="hidden min-[1200px]:inline-flex"
                label={
                  listOpen ? t('Hide conversations') : t('Show conversations')
                }
                onClick={() => setListOpen((v) => !v)}
              >
                <PanelLeft className="h-5 w-5" />
              </IconButton>
            )}
            {newAction && (
              <div className={(['projects', 'contacts', 'inbox'].includes(currentTab) || (currentTab === 'account' && location.pathname.includes('/contacts'))) && !(currentTab === 'inbox' && !listOpen) ? 'min-[1200px]:hidden' : undefined}>
              <CreateAction
                id="header-new-action"
                label={newAction.label}
                onClick={newAction.onClick}
                kind={currentTab}
              />
              </div>
            )}
          </div>
        </PageHeader>
        {secondaryNavigation}
        <div
          className={cn(
            'min-h-0 min-w-0 flex-1',
            currentTab === 'inbox' ||
              currentTab === 'contacts' ||
              projectWorkspace ||
              sessionWorkspace
              ? 'overflow-hidden'
              : currentTab === 'tasks'
                ? 'overflow-auto p-4 md:p-5'
                : currentTab === 'settings' || currentTab === 'account'
                  ? 'overflow-auto'
                  : 'overflow-auto p-3 sm:p-4 md:p-6',
            !hideBottomNav && 'mobile-content-bottom',
          )}
        >
          {children}
        </div>
      </main>
      {!hideBottomNav && (
        <>
          <nav
            aria-label={t('Bottom navigation')}
            className="bottom-navigation fixed bottom-0 left-0 right-0 z-30 flex items-center border-t border-border bg-sidebar md:hidden"
          >
            {/* Pillar 1: Inbox */}
            <button
              key="inbox"
              type="button"
              data-nav-tone="inbox"
              data-testid="bottom-nav-inbox"
              aria-label={t('Inbox')}
              aria-current={isTabActive('inbox') ? 'page' : undefined}
              onClick={() => {
                setMobileMenuOpen(false)
                navigate('inbox')
              }}
              className={cn(
                'flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs transition-colors',
                isTabActive('inbox')
                  ? 'font-semibold text-primary'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              <span className="relative flex h-7 w-7 items-center justify-center">
                <Inbox className="h-5 w-5 text-inherit" />
                {effectiveInboxUnread > 0 && (
                  <span
                    data-testid="inbox-unread-badge-bottom"
                    aria-label={`${effectiveInboxUnread} ${t('unread messages')}`}
                    className="absolute -right-3 -top-2 notification-count min-w-4 rounded-full bg-primary px-1 text-[10px] font-bold font-mono leading-4 text-primary-foreground"
                  >
                    {effectiveInboxUnread > 99 ? '99+' : effectiveInboxUnread}
                  </span>
                )}
              </span>
              <span className="truncate max-w-full px-0.5">{t('Inbox')}</span>
            </button>

            {/* Pillar 2: Projects */}
            <button
              key="projects"
              type="button"
              data-nav-tone="projects"
              data-testid="bottom-nav-projects"
              aria-label={t('Projects')}
              aria-current={isTabActive('projects') ? 'page' : undefined}
              onClick={() => {
                setMobileMenuOpen(false)
                navigate('projects')
              }}
              className={cn(
                'flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs transition-colors',
                isTabActive('projects')
                  ? 'font-semibold text-primary'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              <span className="flex h-7 w-7 items-center justify-center">
                <FolderGit2 className="h-5 w-5 text-inherit" />
              </span>
              <span className="truncate max-w-full px-0.5">{t('Projects')}</span>
            </button>

            {/* Pillar 3: New work (+) */}
            <button
              key="studio"
              type="button"
              data-nav-tone="studio"
              data-testid="bottom-nav-studio"
              aria-label={t('New work')}
              aria-current={currentTab === 'studio' ? 'page' : undefined}
              onClick={() => {
                setMobileMenuOpen(false)
                navigate('studio')
              }}
              className={cn(
                'flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs transition-colors',
                currentTab === 'studio'
                  ? 'font-semibold text-primary'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              <span
                data-action-trigger="true"
                data-testid="bottom-nav-studio-accent"
                className={cn(
                  'action-trigger-circle flex h-8 w-8 items-center justify-center rounded-full transition-colors',
                  currentTab === 'studio'
                    ? 'bg-primary/20 text-primary shadow-xs'
                    : 'bg-primary/10 text-primary hover:bg-primary/15',
                )}
              >
                <Plus aria-hidden="true" className="h-4.5 w-4.5 text-inherit" />
              </span>
              <span className="truncate max-w-full px-0.5">{t('New work')}</span>
            </button>

            {/* Pillar 4: Contacts */}
            <button
              key="contacts"
              type="button"
              data-nav-tone="contacts"
              data-testid="bottom-nav-contacts"
              aria-label={t('Contacts')}
              aria-current={isTabActive('contacts') ? 'page' : undefined}
              onClick={() => {
                setMobileMenuOpen(false)
                navigate('contacts')
              }}
              className={cn(
                'flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs transition-colors',
                isTabActive('contacts')
                  ? 'font-semibold text-primary'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              <span className="flex h-7 w-7 items-center justify-center">
                <Users className="h-5 w-5 text-inherit" />
              </span>
              <span className="truncate max-w-full px-0.5">{t('Contacts')}</span>
            </button>

            {/* Pillar 5: Settings */}
            <button
              key="settings"
              type="button"
              data-nav-tone="settings"
              data-testid="bottom-nav-settings"
              aria-label={t('Settings')}
              aria-current={isTabActive('settings') ? 'page' : undefined}
              onClick={() => {
                setMobileMenuOpen(false)
                navigate('settings')
              }}
              className={cn(
                'flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs transition-colors',
                isTabActive('settings')
                  ? 'font-semibold text-primary'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              <span className="flex h-7 w-7 items-center justify-center">
                <Settings className="h-5 w-5 text-inherit" />
              </span>
              <span className="truncate max-w-full px-0.5">{t('Settings')}</span>
            </button>
          </nav>

          {/* Mobile Menu Drawer */}
          <SlideDrawer
            isOpen={mobileMenuOpen}
            onClose={() => setMobileMenuOpen(false)}
            title={t('Menu')}
            description={t('Workspace & System')}
            size="sm"
            className="md:hidden"
            data-testid="mobile-menu-drawer"
          >
            <div className="space-y-6 py-2">
              {/* Workspace */}
              <div className="space-y-2">
                <div className="px-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                  {t('Workspace')}
                </div>
                <div className="space-y-1">
                  <button
                    type="button"
                    data-testid="menu-drawer-link-projects"
                    onClick={() => {
                      setMobileMenuOpen(false)
                      navigate('projects')
                    }}
                    className={cn(
                      'flex min-h-12 w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm transition-colors',
                      currentTab === 'projects'
                        ? 'bg-primary/10 text-primary font-semibold'
                        : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
                    )}
                  >
                    <div className={cn(
                      "flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors",
                      currentTab === 'projects' ? 'bg-primary/15 text-primary' : 'bg-muted/60 text-muted-foreground'
                    )}>
                      <FolderGit2 className="h-4 w-4 text-inherit" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className={cn("font-medium", currentTab === 'projects' ? 'text-primary font-semibold' : 'text-foreground')}>{t('Projects')}</div>
                      <div className="text-xs text-muted-foreground truncate">
                        {t('Workspaces & repo')}
                      </div>
                    </div>
                  </button>
                </div>
              </div>

              {/* Operations */}
              <div className="space-y-2">
                <div className="px-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                  {t('Operations')}
                </div>
                <div className="space-y-1">
                  <button
                    type="button"
                    data-testid="menu-drawer-link-contacts"
                    onClick={() => {
                      setMobileMenuOpen(false)
                      navigate('contacts')
                    }}
                    className={cn(
                      'flex min-h-12 w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm transition-colors',
                      currentTab === 'contacts'
                        ? 'bg-primary/10 text-primary font-semibold'
                        : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
                    )}
                  >
                    <div className={cn(
                      "flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors",
                      currentTab === 'contacts' ? 'bg-primary/15 text-primary' : 'bg-muted/60 text-muted-foreground'
                    )}>
                      <Users className="h-4 w-4 text-inherit" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className={cn("font-medium", currentTab === 'contacts' ? 'text-primary font-semibold' : 'text-foreground')}>{t('Contacts')}</div>
                      <div className="text-xs text-muted-foreground truncate">
                        {t('CRM & memori pelanggan')}
                      </div>
                    </div>
                  </button>
                </div>
              </div>

              {/* System */}
              <div className="space-y-2">
                <div className="px-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                  {t('System')}
                </div>
                <div className="space-y-1">
                  <button
                    type="button"
                    data-testid="menu-drawer-link-settings"
                    onClick={() => {
                      setMobileMenuOpen(false)
                      navigate('settings')
                    }}
                    className={cn(
                      'flex min-h-12 w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm transition-colors',
                      currentTab === 'settings'
                        ? 'bg-primary/10 text-primary font-semibold'
                        : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
                    )}
                  >
                    <div className={cn(
                      "flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors",
                      currentTab === 'settings' ? 'bg-primary/15 text-primary' : 'bg-muted/60 text-muted-foreground'
                    )}>
                      <Settings className="h-4 w-4 text-inherit" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className={cn("font-medium", currentTab === 'settings' ? 'text-primary font-semibold' : 'text-foreground')}>{t('Settings')}</div>
                      <div className="text-xs text-muted-foreground truncate">
                        {t('Profil, asisten & otak AI, tim')}
                      </div>
                    </div>
                  </button>
                </div>
              </div>

              <div className="h-px bg-border/40" />

              {/* Account */}
              <div className="space-y-2">
                <div className="px-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                  {t('Account')}
                </div>
                <div className="space-y-1">
                  <button
                    type="button"
                    data-testid="menu-drawer-link-account"
                    onClick={() => {
                      setMobileMenuOpen(false)
                      navigate('account')
                    }}
                    className={cn(
                      'flex min-h-14 w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm transition-colors',
                      currentTab === 'account'
                        ? 'bg-primary/10 text-primary font-semibold'
                        : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground',
                    )}
                  >
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground font-mono font-bold text-xs">
                      {user?.display_name ? user.display_name.slice(0, 2).toUpperCase() : 'CU'}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className={cn("font-medium", currentTab === 'account' ? 'text-primary font-semibold' : 'text-foreground')}>
                        {user?.display_name || user?.username || 'User'}
                      </div>
                      <div className="text-xs text-muted-foreground truncate">
                        {user?.email || 'user@cuan.local'}
                      </div>
                    </div>
                  </button>
                </div>
              </div>
            </div>
          </SlideDrawer>
        </>
      )}
    </div>
  )
}
