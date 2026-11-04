import { useLanguage } from '../../context/LanguageContext'
import { useNavigate } from 'react-router-dom'
import { useState, useRef, useEffect } from 'react'
import { useAuth } from '../../context/AuthContext'
import {
  Users,
  LogOut,
  MoreHorizontal,
  Settings,
} from 'lucide-react'
import { cn } from '../../lib/utils'

interface NavUserProps {
  isCollapsed?: boolean
  sidebar?: boolean
  collapsed?: boolean
  dropdownPosition?: 'side' | 'bottom-right' | 'bottom-left' | 'top-right'
  bottomNavigation?: boolean
  active?: boolean
  onSettings?: () => void
  onWork?: () => void
}

export function NavUser({
  isCollapsed = false,
  sidebar = false,
  collapsed = false,
  dropdownPosition,
  onSettings,
  bottomNavigation = false,
  active = false,
}: NavUserProps) {
  const { t } = useLanguage()

  const navigate = useNavigate()
  const compact = isCollapsed || (sidebar && collapsed)
  const { user, logout } = useAuth()
  const [menuOpen, setMenuOpen] = useState(false)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const popoverRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(event.target as Node)
      ) {
        setMenuOpen(false)
      }
    }
    const keyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setMenuOpen(false)
        triggerRef.current?.focus()
      }
    }
    if (menuOpen) {
      popoverRef.current
        ?.querySelector<HTMLElement>('[data-account-popover] button')
        ?.focus()
      document.addEventListener('keydown', keyDown)
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', keyDown)
    }
  }, [menuOpen])

  const username = user?.display_name || user?.username || 'admin'
  const email = user?.email || 'admin@cuan.local'
  const initials = username.slice(0, 2).toUpperCase()

  const handleLogout = async () => {
    setMenuOpen(false)
    await logout()
  }

  return (
    <>
      <div className="relative w-full" ref={popoverRef}>
        {/* Trigger Button */}
        <button
          type="button"
          ref={triggerRef}
          data-testid={bottomNavigation ? 'bottom-nav-user-button' : undefined}
          aria-label={t('Account menu')}
          aria-expanded={menuOpen}
          aria-current={active ? 'page' : undefined}
          onClick={() => setMenuOpen(!menuOpen)}
          className={cn(
            'flex w-full items-center gap-2 text-left transition-colors rounded-md',
            sidebar ? 'min-h-14 px-2 py-2 text-sm' : 'min-h-11 p-2 text-sm',
            !bottomNavigation && 'hover:bg-accent hover:text-accent-foreground',
            compact && 'justify-center p-1',
            sidebar && 'sidebar-account',
            sidebar &&
              active &&
              'text-sidebar-accent-foreground font-medium',
            bottomNavigation &&
              'min-h-14 flex-col justify-center gap-1 rounded-none p-0 text-xs',
            bottomNavigation &&
              (active ? 'text-primary font-semibold' : 'text-muted-foreground'),
          )}
          title={compact ? `${username} · ${t('Account')}` : undefined}
        >
          {/* Avatar */}
          <div data-avatar="true"
            className={cn(
              'flex shrink-0 items-center justify-center font-mono font-bold rounded-none',
              bottomNavigation ? 'h-7 w-7 text-[10px]' : 'h-9 w-9 text-xs bg-primary text-primary-foreground',
              bottomNavigation && (active ? 'bg-primary/10 text-primary' : 'bg-muted/60 text-muted-foreground'),

            )}
          >
            {initials}
          </div>

          {bottomNavigation && <span>{t('Account')}</span>}

          {!compact && !bottomNavigation && (
            <div className="account-label grid min-w-0 flex-1 text-left leading-tight truncate">
              <span className="truncate font-medium text-sm text-foreground">
                {username}
              </span>
              <span className="truncate text-xs text-muted-foreground">
                {email}
              </span>
            </div>
          )}

          {!compact && !bottomNavigation && (
            <MoreHorizontal className="account-label h-3.5 w-3.5 text-muted-foreground shrink-0" />
          )}
        </button>

        {/* Dropdown Menu */}
        {menuOpen && (
          <div
            data-account-popover
            role="region"
            aria-label={t('Account menu')}
            className={cn(
              'absolute z-50 min-w-56 bg-popover text-popover-foreground border border-border/40 shadow-lg p-1.5 rounded-xl max-h-[calc(100dvh-5rem)] overflow-y-auto',
              dropdownPosition === 'top-right'
                ? 'bottom-full right-2 mb-2 w-64 max-w-[calc(100vw-1rem)]'
                : dropdownPosition === 'bottom-right'
                  ? 'right-0 top-full mt-1.5'
                  : dropdownPosition === 'bottom-left'
                    ? 'left-0 top-full mt-1.5'
                    : compact
                      ? 'left-full bottom-0 ml-2'
                      : 'bottom-full left-0 mb-1 w-full',
            )}
          >
            {/* User Identity Header */}
            <button
              type="button"
              onClick={() => { setMenuOpen(false); navigate('/account') }}
              className="mb-1 flex min-h-14 w-full items-center gap-3 border-b border-border/40 p-2 text-left transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              title={t('Account overview')}
            >
              <div data-avatar="true" className="flex h-10 w-10 shrink-0 items-center justify-center rounded-none bg-primary text-xs font-bold text-primary-foreground">
                {initials}
              </div>
              <div className="grid min-w-0 flex-1 gap-0.5">
                <span className="truncate text-sm font-medium text-foreground">{username}</span>
                <span className="truncate text-xs text-muted-foreground">{email}</span>
              </div>
            </button>

            <button
              type="button"
              data-testid="navuser-settings"
              onClick={() => {
                setMenuOpen(false)
                if (onSettings) onSettings()
                else navigate('/settings')
              }}
              className="flex min-h-11 w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm text-foreground/90 hover:bg-accent hover:text-accent-foreground transition-colors"
            >
              <Settings className="h-4 w-4 shrink-0 text-muted-foreground" />
              <span>{t('Settings')}</span>
            </button>

            <button type="button" onClick={() => { setMenuOpen(false); navigate('/contacts') }} className="flex min-h-11 w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm text-foreground/90 hover:bg-accent hover:text-accent-foreground">
              <Users className="h-4 w-4 shrink-0 text-muted-foreground" />
              <span>{t('Contacts')}</span>
            </button>

            <div className="my-1 h-px bg-border/40" />

            <button
              type="button"
              onClick={handleLogout}
              className="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-foreground transition-colors text-left rounded-md"
            >
              <LogOut className="h-4 w-4 shrink-0" />
              <span>{t('Sign out')}</span>
            </button>
          </div>
        )}
      </div>
    </>
  )
}
