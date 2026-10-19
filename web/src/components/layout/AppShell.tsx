import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { CuanMark } from '../brand/CuanMark'
import { NavUser } from './NavUser'
import { useTheme } from '../../context/ThemeContext'
import {
  Inbox,
  Users,
  CheckSquare,
  FolderGit2,
  Plus,
  PanelLeft,
  Sun,
  Moon,
  Settings,
} from 'lucide-react'
import { cn } from '../../lib/utils'

export type AppTab = 'inbox' | 'tasks' | 'projects' | 'contacts' | 'settings'
interface AppShellProps {
  currentTab: AppTab
  onSelectTab: (tab: AppTab) => void
  children: React.ReactNode
  newAction?: { label: string; onClick: () => void }
  hideBottomNav?: boolean
  inboxSubPanel?: React.ReactNode
  inboxUnread?: number
  pageTitle?: string
  breadcrumbs?: { label: string; to: string }[]
}
const NAV_ITEMS = [
  { id: 'inbox', label: 'Inbox', icon: Inbox },
  { id: 'contacts', label: 'Contacts', icon: Users },
  { id: 'tasks', label: 'Tasks', icon: CheckSquare },
  { id: 'projects', label: 'Projects', icon: FolderGit2 },
] as const

export function AppShell({
  currentTab,
  onSelectTab,
  children,
  newAction,
  hideBottomNav = false,
  inboxSubPanel,
  inboxUnread = 0,
  pageTitle,
  breadcrumbs = [],
}: AppShellProps) {
  const shell = useRef<HTMLDivElement>(null)
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem('cuan-nav-collapsed') === 'true'
    } catch {
      return false
    }
  })
  const [listOpen, setListOpen] = useState(true)
  const { resolvedTheme, toggleTheme } = useTheme()
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
    currentTab === 'settings'
      ? 'Settings'
      : NAV_ITEMS.find((n) => n.id === currentTab)?.label
  const navigate = (id: AppTab) => {
    onSelectTab(id)
  }
  const navButton = (id: AppTab, text: string, Icon: typeof Inbox) => (
    <button
      key={id}
      title={text}
      aria-label={text}
      aria-current={currentTab === id ? 'page' : undefined}
      onClick={() => navigate(id)}
      className={cn(
        'nav-action',
        currentTab === id &&
          'bg-sidebar-accent text-sidebar-accent-foreground font-semibold',
      )}
    >
      <Icon className="h-5 w-5 shrink-0" />
      <span className="nav-label">{text}</span>
    </button>
  )
  const themeButton = (
    <button
      title={`Tema ${resolvedTheme === 'dark' ? 'Dark' : 'Light'}`}
      aria-label="Ganti tema"
      onClick={toggleTheme}
      className="icon-action"
    >
      {resolvedTheme === 'dark' ? (
        <Moon className="h-5 w-5" />
      ) : (
        <Sun className="h-5 w-5" />
      )}
    </button>
  )
  return (
    <div
      ref={shell}
      className={cn(
        'app-shell flex w-full overflow-hidden bg-background',
        collapsed && 'nav-collapsed',
      )}
    >
      <nav
        aria-label="Navigasi utama"
        className="primary-navigation hidden shrink-0 flex-col border-r border-border bg-sidebar md:flex"
      >
        <div className="flex h-16 shrink-0 items-center gap-3 border-b border-border px-2.5">
          <CuanMark className="h-10 w-10" />
          <span className="nav-label text-lg font-semibold tracking-tight">
            cuan
          </span>
        </div>
        <div className="flex-1 space-y-1 p-2.5">
          {NAV_ITEMS.map((n) => navButton(n.id, n.label, n.icon))}
        </div>
        <div className="space-y-2 border-t border-border p-2.5">
          {navButton('settings', 'Settings', Settings)}
          <div className="flex flex-wrap items-center justify-center gap-1">
            {themeButton}
            <button
              className="icon-action hidden min-[1200px]:inline-flex"
              title={collapsed ? 'Perluas navigasi' : 'Ciutkan navigasi'}
              aria-label={collapsed ? 'Perluas navigasi' : 'Ciutkan navigasi'}
              onClick={() =>
                setCollapsed((value) => {
                  const next = !value
                  try {
                    localStorage.setItem('cuan-nav-collapsed', String(next))
                  } catch {}
                  return next
                })
              }
            >
              <PanelLeft className="h-5 w-5" />
            </button>
            <NavUser isCollapsed />
          </div>
        </div>
      </nav>
      {currentTab === 'inbox' && listOpen && (
        <section
          aria-label="Daftar percakapan"
          className="conversation-sidebar hidden h-full w-[280px] shrink-0 flex-col border-r border-border min-[1200px]:flex"
        >
          <div className="flex h-16 shrink-0 items-center border-b border-border px-4 text-base font-semibold">
            Percakapan
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">{inboxSubPanel}</div>
        </section>
      )}
      <main className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <header
          className={cn(
            'flex shrink-0 items-center justify-between gap-2 border-b border-border bg-background px-3 md:px-5',
            'app-page-header h-14 md:h-16',
            hideBottomNav && 'hidden md:flex',
          )}
        >
          <div className="flex min-w-0 items-center gap-2">
            <CuanMark className="h-6 w-6 md:hidden" />
            {breadcrumbs.length > 0 && (
              <nav
                aria-label="Hierarki halaman"
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
              {pageTitle || label}
            </h1>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {currentTab === 'inbox' && (
              <button
                className="icon-action hidden min-[1200px]:inline-flex"
                aria-label={
                  listOpen ? 'Sembunyikan percakapan' : 'Tampilkan percakapan'
                }
                onClick={() => setListOpen((v) => !v)}
              >
                <PanelLeft className="h-5 w-5" />
              </button>
            )}
            {newAction && (
              <button
                id="header-new-action"
                className="action-primary shrink-0 px-3"
                aria-label={newAction.label}
                title={newAction.label}
                onClick={newAction.onClick}
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                <span>New</span>
              </button>
            )}
            <div className="w-11 md:hidden">
              <NavUser
                isCollapsed
                dropdownPosition="bottom-right"
                onSettings={() => navigate('settings')}
              />
            </div>
          </div>
        </header>
        <div
          className={cn(
            'min-h-0 min-w-0 flex-1',
            currentTab === 'inbox'
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
          aria-label="Navigasi bawah"
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
                <Icon className="h-5 w-5" />
                {id === 'inbox' && inboxUnread > 0 && (
                  <span
                    aria-label={`${inboxUnread} pesan belum dibaca`}
                    className="absolute -right-3 -top-2 min-w-4 rounded-full bg-primary px-1 text-[10px] leading-4 text-primary-foreground"
                  >
                    {inboxUnread > 99 ? '99+' : inboxUnread}
                  </span>
                )}
              </span>
              {text}
            </button>
          ))}
        </nav>
      )}
    </div>
  )
}
