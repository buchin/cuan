import { useState, useRef, useEffect } from 'react'
import { useAuth } from '../../context/AuthContext'
import { ChangePasswordModal } from '../auth/ChangePasswordModal'
import {
  LogOut,
  KeyRound,
  MoreHorizontal,
  Settings,
  Activity,
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
  const compact = isCollapsed || (sidebar && collapsed)
  const { user, logout } = useAuth()
  const [menuOpen, setMenuOpen] = useState(false)
  const [passwordModalOpen, setPasswordModalOpen] = useState(false)
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

  const username = user?.username || 'admin'
  const email = user?.email || 'admin@cuan.local'
  const initials = username.slice(0, 2).toUpperCase()

  const handleLogout = async () => {
    setMenuOpen(false)
    await logout()
  }

  const handleOpenPasswordModal = () => {
    setMenuOpen(false)
    setPasswordModalOpen(true)
  }

  return (
    <>
      <div className="relative w-full" ref={popoverRef}>
        {/* Trigger Button */}
        <button
          type="button"
          ref={triggerRef}
          aria-label="Account menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen(!menuOpen)}
          className={cn(
            'flex min-h-11 w-full items-center gap-2 p-2 text-left hover:bg-accent hover:text-accent-foreground transition-colors rounded-md',
            compact && 'justify-center p-2',
            sidebar && 'sidebar-account',
            bottomNavigation &&
              'min-h-14 flex-col justify-center gap-1 rounded-none p-0 text-xs',
            bottomNavigation &&
              (active || menuOpen
                ? 'text-primary font-semibold'
                : 'text-muted-foreground'),
          )}
          title={compact ? username : undefined}
        >
          {/* Avatar */}
          <div
            className={cn(
              'flex h-7 w-7 shrink-0 items-center justify-center bg-primary text-primary-foreground font-mono font-bold text-xs rounded-full',
              bottomNavigation && 'h-5 w-5 text-[9px]',
            )}
          >
            {initials}
          </div>

          {bottomNavigation && <span>Account</span>}

          {!compact && !bottomNavigation && (
            <div className="account-label grid min-w-0 flex-1 text-left leading-tight truncate">
              <span className="truncate font-semibold text-sm text-foreground">
                {username}
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
            aria-label="Account menu"
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
            <div className="flex items-center gap-3 p-2 border-b border-border mb-1">
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
            </div>

            {onWork && (
              <button
                type="button"
                className="flex min-h-11 w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm hover:bg-accent"
                onClick={() => {
                  setMenuOpen(false)
                  onWork()
                }}
              >
                <Activity className="h-4 w-4" />
                <span>Work monitor</span>
              </button>
            )}
            {onSettings && (
              <button
                type="button"
                className="flex min-h-11 w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm hover:bg-accent"
                onClick={() => {
                  setMenuOpen(false)
                  onSettings()
                }}
              >
                <Settings className="h-4 w-4" />
                <span>Settings</span>
              </button>
            )}
            {/* Menu Actions */}
            <button
              type="button"
              onClick={handleOpenPasswordModal}
              className="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-foreground/90 hover:bg-accent hover:text-accent-foreground transition-colors text-left rounded-md"
            >
              <KeyRound className="h-4 w-4 text-primary shrink-0" />
              <span>Profile & password</span>
            </button>

            <div className="h-px bg-border my-1" />

            <button
              type="button"
              onClick={handleLogout}
              className="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-destructive hover:bg-destructive/10 transition-colors text-left rounded-md"
            >
              <LogOut className="h-4 w-4 shrink-0" />
              <span className="font-semibold">Sign out</span>
            </button>
          </div>
        )}
      </div>

      {/* Password Change Dialog Modal */}
      <ChangePasswordModal
        isOpen={passwordModalOpen}
        onClose={() => setPasswordModalOpen(false)}
      />
    </>
  )
}
