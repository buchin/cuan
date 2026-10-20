import { useNavigate } from 'react-router-dom'
import { useNotifications } from '../../context/NotificationsContext'
import { Bell } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useRef, useState, type ComponentType } from 'react'
import { Link } from 'react-router-dom'
import { CuanMark } from '../brand/CuanMark'
import { NavUser } from './NavUser'
import { CreateAction } from './CreateAction'
import { Inbox, Users, FolderGit2, PanelLeft, Settings } from 'lucide-react'
import { cn } from '../../lib/utils'

export type AppTab =
  | 'notifications'
  | 'studio'
  | 'inbox'
  | 'tasks'
  | 'projects'
  | 'contacts'
  | 'settings'
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
  inboxSubPanel?: React.ReactNode
  inboxUnread?: number
  pageTitleIsContent?: boolean
  pageTitle?: string
  breadcrumbs?: { label: string; to: string }[]
}
const NAV_ITEMS = [
  { id: 'studio', label: 'Studio', icon: CuanMark },
  { id: 'inbox', label: 'Inbox', icon: Inbox },
  { id: 'projects', label: 'Projects', icon: FolderGit2 },
  { id: 'contacts', label: 'Contacts', icon: Users },
] as const

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
  inboxSubPanel,
  inboxUnread = 0,
  pageTitleIsContent = false,
  pageTitle,
  breadcrumbs = [],
}: AppShellProps) {
  const { t } = useLanguage()

  const routerNavigate = useNavigate()
  const { items: notifications } = useNotifications()
  const unreadNotifications = notifications.filter((n) => !n.read).length
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
  const label =
    currentTab === 'notifications'
      ? 'Notifications'
      : currentTab === 'tasks'
        ? t('Work monitor')
        : currentTab === 'settings'
          ? t('Settings')
          : NAV_ITEMS.find((n) => n.id === currentTab)?.label
  const navigate = (id: AppTab) => {
    onSelectTab(id)
  }
  const navButton = (
    id: AppTab,
    text: string,
    Icon: ComponentType<{ className?: string }>,
  ) => (
    <button
      key={id}
      title={t(text)}
      aria-label={t(text)}
      aria-current={currentTab === id ? 'page' : undefined}
      onClick={() => navigate(id)}
      className={cn(
        'nav-action',
        currentTab === id &&
          'bg-sidebar-accent text-sidebar-accent-foreground font-semibold',
      )}
    >
      {id === 'studio' ? (
        <CuanMark decorative className="h-5 w-5 text-inherit" />
      ) : (
        <Icon className="h-5 w-5 shrink-0" />
      )}
      <span className="nav-label">{t(text)}</span>
    </button>
  )
  return (
    <div
      ref={shell}
      className={cn(
        'app-shell flex w-full overflow-hidden bg-background',
        effectiveCollapsed && 'nav-collapsed',
      )}
    >
      <nav
        aria-label={t('Main navigation')}
        className="primary-navigation hidden shrink-0 flex-col border-r border-border bg-sidebar md:flex"
      >
        <div className="sidebar-brand flex h-16 shrink-0 items-center gap-2 border-b border-border px-2.5">
          <CuanMark className="h-9 w-9" />
          <span className="nav-label text-lg font-semibold tracking-tight">
            cuan
          </span>
          <button
            className="sidebar-collapse icon-action hidden min-[1200px]:inline-flex"
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
            <PanelLeft className="h-4 w-4" />
          </button>
        </div>
        <div className="flex-1 space-y-1 p-2.5">
          {NAV_ITEMS.map((n) => navButton(n.id, n.label, n.icon))}
        </div>
        <div className="space-y-2 border-t border-border p-2.5">
          {navButton('settings', t('Settings'), Settings)}
          <NavUser
            sidebar
            collapsed={effectiveCollapsed}
            onWork={() => navigate('tasks')}
          />
        </div>
      </nav>
      {currentTab === 'inbox' && listOpen && (
        <section
          aria-label={t('Conversations')}
          className="conversation-sidebar hidden h-full w-[280px] shrink-0 flex-col border-r border-border min-[1200px]:flex"
        >
          <div className="flex h-16 shrink-0 items-center border-b border-border px-4 text-base font-semibold">
            {t('Conversations')}
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">{inboxSubPanel}</div>
        </section>
      )}
      <main className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <header
          className={cn(
            'flex shrink-0 items-center justify-between gap-2 border-b border-border bg-background px-3 md:px-5',
            'app-page-header h-14 md:h-16',
            sessionWorkspace ? 'hidden' : hideBottomNav && 'hidden md:flex',
          )}
        >
          <div className="flex min-w-0 items-center gap-2">
            <CuanMark className="h-6 w-6 md:hidden" />
            {breadcrumbs.length > 0 && (
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
            <h1 className="page-title min-w-0 truncate text-xl font-semibold tracking-tight">
              {pageTitleIsContent ? pageTitle : t(pageTitle || label || '')}
            </h1>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {headerActions}
            {currentTab !== 'notifications' && (
              <button
                type="button"
                className={cn('icon-action relative')}
                aria-label={t('Notifications')}
                onClick={() => routerNavigate('/notifications')}
              >
                <Bell className="h-5 w-5" />
                {unreadNotifications > 0 && (
                  <span
                    aria-hidden="true"
                    className="absolute right-0 top-0 min-w-4 rounded-full bg-primary px-1 text-[10px] leading-4 text-primary-foreground"
                  >
                    {unreadNotifications > 99 ? '99+' : unreadNotifications}
                  </span>
                )}
              </button>
            )}
            {currentTab === 'inbox' && (
              <button
                className="icon-action hidden min-[1200px]:inline-flex"
                aria-label={
                  listOpen ? t('Hide conversations') : t('Show conversations')
                }
                onClick={() => setListOpen((v) => !v)}
              >
                <PanelLeft className="h-5 w-5" />
              </button>
            )}
            {newAction && (
              <CreateAction
                id="header-new-action"
                label={newAction.label}
                onClick={newAction.onClick}
                kind={currentTab}
              />
            )}
          </div>
        </header>
        {secondaryNavigation}
        <div
          className={cn(
            'min-h-0 min-w-0 flex-1',
            currentTab === 'inbox' ||
              currentTab === 'contacts' ||
              projectWorkspace ||
              sessionWorkspace
              ? 'overflow-hidden'
              : 'overflow-auto p-4 md:p-6',
            !hideBottomNav && 'mobile-content-bottom',
          )}
        >
          {children}
        </div>
      </main>
      {!hideBottomNav && (
        <nav
          aria-label={t('Bottom navigation')}
          className="bottom-navigation fixed bottom-0 left-0 right-0 z-30 flex items-center border-t border-border bg-sidebar md:hidden"
        >
          {NAV_ITEMS.map(({ id, label: text, icon: Icon }) => (
            <button
              key={id}
              aria-current={currentTab === id ? 'page' : undefined}
              onClick={() => navigate(id)}
              className={cn(
                'flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs',
                currentTab === id
                  ? 'font-semibold text-primary'
                  : 'text-muted-foreground',
              )}
            >
              <span className="relative">
                {id === 'studio' ? (
                  <CuanMark decorative className="h-5 w-5 text-inherit" />
                ) : (
                  <Icon className="h-5 w-5" />
                )}
                {id === 'inbox' && inboxUnread > 0 && (
                  <span
                    aria-label={`${inboxUnread} unread messages`}
                    className="absolute -right-3 -top-2 min-w-4 rounded-full bg-primary px-1 text-[10px] leading-4 text-primary-foreground"
                  >
                    {inboxUnread > 99 ? '99+' : inboxUnread}
                  </span>
                )}
              </span>
              {t(text)}
            </button>
          ))}
          <div className="min-w-0 flex-1">
            <NavUser
              bottomNavigation
              active={currentTab === 'settings' || currentTab === 'tasks'}
              onWork={() => navigate('tasks')}
              dropdownPosition="top-right"
              onSettings={() => navigate('settings')}
            />
          </div>
        </nav>
      )}
    </div>
  )
}
