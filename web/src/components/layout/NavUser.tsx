import { useState, useRef, useEffect } from 'react'
import { useAuth } from '../../context/AuthContext'
import { ChangePasswordModal } from '../auth/ChangePasswordModal'
import { LogOut, KeyRound, ChevronsUpDown } from 'lucide-react'
import { cn } from '../../lib/utils'

interface NavUserProps {
  isCollapsed?: boolean
}

export function NavUser({ isCollapsed = false }: NavUserProps) {
  const { user, logout } = useAuth()
  const [menuOpen, setMenuOpen] = useState(false)
  const [passwordModalOpen, setPasswordModalOpen] = useState(false)
  const popoverRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(event.target as Node)) {
        setMenuOpen(false)
      }
    }
    if (menuOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
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
          onClick={() => setMenuOpen(!menuOpen)}
          className={cn(
            'flex w-full items-center gap-2 p-2 text-left hover:bg-accent hover:text-accent-foreground transition-colors rounded-md',
            isCollapsed && 'justify-center p-2'
          )}
          title={isCollapsed ? username : undefined}
        >
          {/* Avatar */}
          <div className="flex h-7 w-7 shrink-0 items-center justify-center bg-primary text-primary-foreground font-mono font-bold text-xs rounded-full">
            {initials}
          </div>

          {!isCollapsed && (
            <div className="grid flex-1 text-left leading-tight truncate">
              <span className="truncate font-semibold text-xs text-foreground">
                {username}
              </span>
              <span className="truncate text-[10px] text-muted-foreground">
                {email}
              </span>
            </div>
          )}

          {!isCollapsed && (
            <ChevronsUpDown className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
          )}
        </button>

        {/* Dropdown Menu */}
        {menuOpen && (
          <div
            className={cn(
              'absolute z-50 min-w-56 bg-popover text-popover-foreground border border-border shadow-md p-1 rounded-lg',
              isCollapsed
                ? 'left-full bottom-0 ml-2'
                : 'bottom-full left-0 mb-1 w-full'
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

            {/* Menu Actions */}
            <button
              type="button"
              onClick={handleOpenPasswordModal}
              className="flex w-full items-center gap-2 px-2.5 py-2 text-xs text-foreground/90 hover:bg-accent hover:text-accent-foreground transition-colors text-left rounded-md"
            >
              <KeyRound className="h-4 w-4 text-primary shrink-0" />
              <span>Profil & Ganti Password</span>
            </button>

            <div className="h-px bg-border my-1" />

            <button
              type="button"
              onClick={handleLogout}
              className="flex w-full items-center gap-2 px-2.5 py-2 text-xs text-destructive hover:bg-destructive/10 transition-colors text-left rounded-md"
            >
              <LogOut className="h-4 w-4 shrink-0" />
              <span className="font-semibold">Keluar (Logout)</span>
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
