import { useLanguage } from '../../context/LanguageContext'
import { useNavigate } from 'react-router-dom'
import { useState, useRef, useEffect } from 'react'
import { useAuth } from '../../context/AuthContext'
import { ReloadButton } from '../ui/ReloadButton'
import {
  Keyboard,
  LogOut,
  KeyRound,
  MoreHorizontal,
  Settings,
  Users,
  CheckSquare,
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
  onWork,
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
          aria-label={t('Account menu')}
          aria-expanded={menuOpen}
          aria-current={active ? 'page' : undefined}
          onClick={() => setMenuOpen(!menuOpen)}
          className={cn(
            'flex min-h-11 w-full items-center gap-2 p-2 text-left transition-colors rounded-md',
            !bottomNavigation && 'hover:bg-accent hover:text-accent-foreground',
            compact && 'justify-center p-2',
            sidebar && 'sidebar-account',
            sidebar &&
              active &&
              'bg-sidebar-accent text-sidebar-accent-foreground font-semibold',
            bottomNavigation &&
              'min-h-14 flex-col justify-center gap-1 rounded-none p-0 text-xs',
            bottomNavigation &&
              (active ? 'text-primary font-semibold' : 'text-muted-foreground'),
          )}
          title={compact ? `${username} · ${t('Account')}` : undefined}
        >
          {/* Avatar */}
          <div
            className={cn(
              'flex h-7 w-7 shrink-0 items-center justify-center bg-primary text-primary-foreground font-mono font-bold text-xs rounded-full',
              bottomNavigation && 'h-5 w-5 text-[9px]',
              bottomNavigation && !active && 'bg-muted text-muted-foreground',
            )}
          >
            {initials}
          </div>

          {bottomNavigation && <span>{t('Account')}</span>}

          {!compact && !bottomNavigation && (
            <div className="account-label grid min-w-0 flex-1 text-left leading-tight truncate">
              <span className="truncate font-semibold text-sm text-foreground">
                {username}
              </span>
              <span className="truncate text-[11px] text-muted-foreground">
                {t('Account')}
              </span>
            </div>
          )}

          {!compact && !bottomNavigation && (
            <MoreHorizontal className="account-label h-4 w-4 text-muted-foreground shrink-0" />
          )}
        </button>

        {/* Dropdown Menu */}
        {menuOpen && (
          <div
            data-account-popover
            role="region"
            aria-label={t('Account menu')}
            className={cn(
              'absolute z-50 min-w-56 bg-popover text-popover-foreground border border-border shadow-md p-1 rounded-lg',
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
            <div
              role="button"
              tabIndex={0}
              onClick={() => {
                setMenuOpen(false)
                navigate('/account')
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  setMenuOpen(false)
                  navigate('/account')
                }
              }}
              className="flex items-center gap-3 p-2 border-b border-border mb-1 rounded-md hover:bg-accent cursor-pointer transition-colors"
              title={t('Account overview')}
            >
              <div className="flex h-8 w-8 shrink-0 items-center justify-center bg-primary text-primary-foreground font-mono font-bold text-xs rounded-full">
                {initials}
              </div>
              <div className="grid flex-1 text-left leading-tight truncate">
                <span className="truncate font-bold text-xs text-foreground flex items-center gap-1.5">
                  {username}
                  <span className="inline-flex items-center px-1 py-0.5 text-[9px] font-mono bg-green-500/15 text-green-600 border border-green-500/30 rounded">
                    OWNER
                  </span>
                </span>
                <span className="truncate text-[10px] text-muted-foreground">
                  {email}
                </span>
              </div>
              <ReloadButton
                className="h-8 w-8 rounded hover:bg-accent text-muted-foreground hover:text-foreground shrink-0"
                iconClassName="h-4 w-4"
                title={t('Reload app')}
              />
            </div>

            <button
              type="button"
              onClick={() => {
                setMenuOpen(false)
                navigate('/account/contacts')
              }}
              className="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-foreground/90 hover:bg-accent hover:text-accent-foreground transition-colors text-left rounded-md"
            >
              <Users className="h-4 w-4 text-primary shrink-0" />
              <span>{t('Contacts')}</span>
            </button>
            <button
              type="button"
              className="flex min-h-11 w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm hover:bg-accent"
              onClick={() => {
                setMenuOpen(false)
                if (onSettings) onSettings()
                else navigate('/settings')
              }}
            >
              <Settings className="h-4 w-4" />
              <span>{t('Settings')}</span>
            </button>
            <button
              type="button"
              onClick={() => {
                setMenuOpen(false)
                navigate('/settings/profile')
              }}
              className="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-foreground/90 hover:bg-accent hover:text-accent-foreground transition-colors text-left rounded-md"
            >
              <KeyRound className="h-4 w-4 text-primary shrink-0" />
              <span>{t('Profile & password')}</span>
            </button>
            {onWork && (
              <button
                type="button"
                className="flex min-h-11 w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm hover:bg-accent"
                onClick={() => {
                  setMenuOpen(false)
                  onWork()
                }}
              >
                <CheckSquare className="h-4 w-4" />
                <span>{t('Tasks')}</span>
              </button>
            )}
            <button type="button" className="flex min-h-11 w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm hover:bg-accent" onClick={() => { setMenuOpen(false); window.dispatchEvent(new Event('cuan:keyboard-shortcuts')) }}>
              <Keyboard className="h-4 w-4" /><span>{t('Keyboard shortcuts')}</span><kbd className="ml-auto text-xs text-muted-foreground">?</kbd>
            </button>

            <ReloadButton
              variant="menu-item"
              label={t('Reload app')}
            />

            <div className="h-px bg-border my-1" />

            <button
              type="button"
              onClick={handleLogout}
              className="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-destructive hover:bg-destructive/10 transition-colors text-left rounded-md"
            >
              <LogOut className="h-4 w-4 shrink-0" />
              <span className="font-semibold">{t('Sign out')}</span>
            </button>
          </div>
        )}
      </div>
    </>
  )
}
