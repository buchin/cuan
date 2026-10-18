import { useState, useRef, useEffect } from 'react'
import { useAuth } from '../../context/AuthContext'
import { useTheme } from '../../context/ThemeContext'
import { ChangePasswordModal } from '../auth/ChangePasswordModal'
import {
  LogOut,
  KeyRound,
  Sun,
  Moon,
  ChevronsUpDown
} from 'lucide-react'
import { cn } from '../../lib/utils'

interface NavUserProps {
  isCollapsed?: boolean
}

export function NavUser({ isCollapsed = false }: NavUserProps) {
  const { user, logout } = useAuth()
  const { resolvedTheme, toggleTheme } = useTheme()
  const [menuOpen, setMenuOpen] = useState(false)
  const [passwordModalOpen, setPasswordModalOpen] = useState(false)
  const popoverRef = useRef<HTMLDivElement>(null)

  // Close popover when clicking outside
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

  // Get initials for avatar (e.g. "AD" or "BU")
  const initials = username
    .slice(0, 2)
    .toUpperCase()

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
        <button
          type="button"
          onClick={() => setMenuOpen(!menuOpen)}
          data-sidebar="nav-user"
          aria-expanded={menuOpen}
          aria-haspopup="true"
          title={`${username} (${email})`}
          className={cn(
            "flex w-full items-center gap-3 p-2 text-left text-xs font-medium rounded-none transition-colors",
            "hover:bg-[#d5dce8] dark:hover:bg-[#1f2335] text-[#1a1b26] dark:text-[#c0caf5] outline-hidden select-none",
            menuOpen && "bg-[#d5dce8] dark:bg-[#1f2335]",
            isCollapsed && "justify-center p-1.5"
          )}
        >
          {/* Avatar / Initials */}
          <div className="relative flex h-8 w-8 shrink-0 items-center justify-center bg-[#7aa2f7] text-[#16161e] font-mono font-bold text-xs shadow-xs border border-[#cfd6e6] dark:border-[#24283b]">
            {initials}
            <span className="absolute bottom-0 right-0 h-2 w-2 rounded-none bg-emerald-500 ring-1 ring-emerald-400/50" />
          </div>

          {/* User Details (hidden when sidebar collapsed) */}
          {!isCollapsed && (
            <div className="grid flex-1 text-left leading-tight truncate">
              <span className="truncate font-semibold text-[#1a1b26] dark:text-[#c0caf5]">
                {username}
              </span>
              <span className="truncate text-[10px] text-[#565f89]">
                {email}
              </span>
            </div>
          )}

          {!isCollapsed && (
            <ChevronsUpDown className="ml-auto h-4 w-4 shrink-0 text-[#565f89]" />
          )}
        </button>

        {/* Dropdown / Popover Menu */}
        {menuOpen && (
          <div
            className={cn(
              "absolute z-50 min-w-56 bg-[#e2e7f1] dark:bg-[#16161e] text-[#1a1b26] dark:text-[#c0caf5] border border-[#cfd6e6] dark:border-[#24283b] shadow-xl p-1",
              isCollapsed
                ? "left-full bottom-0 ml-2"
                : "bottom-full left-0 mb-1 w-full"
            )}
          >
            {/* User Identity Header */}
            <div className="flex items-center gap-3 p-2 border-b border-border/80">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center bg-primary text-primary-foreground font-mono font-bold text-xs">
                {initials}
              </div>
              <div className="grid flex-1 text-left leading-tight truncate">
                <span className="truncate font-bold text-xs text-foreground flex items-center gap-1.5">
                  {username}
                  <span className="inline-flex items-center px-1 py-0.2 text-[9px] font-mono bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                    OWNER
                  </span>
                </span>
                <span className="truncate text-[10px] text-muted-foreground">
                  {email}
                </span>
              </div>
            </div>

            {/* Menu Actions */}
            <div className="py-1">
              {/* Profile / Change Password */}
              <button
                type="button"
                onClick={handleOpenPasswordModal}
                className="flex w-full items-center gap-2 px-2.5 py-2 text-xs text-foreground/90 hover:bg-accent hover:text-accent-foreground transition-colors text-left"
              >
                <KeyRound className="h-4 w-4 text-primary shrink-0" />
                <span>Profil & Ganti Password</span>
              </button>

              {/* Theme Toggle (Tokyo Night Dark / Light) */}
              <button
                type="button"
                onClick={toggleTheme}
                className="flex w-full items-center justify-between px-2.5 py-2 text-xs text-foreground/90 hover:bg-accent hover:text-accent-foreground transition-colors text-left"
              >
                <div className="flex items-center gap-2">
                  {resolvedTheme === 'dark' ? (
                    <Moon className="h-4 w-4 text-primary shrink-0" />
                  ) : (
                    <Sun className="h-4 w-4 text-amber-500 shrink-0" />
                  )}
                  <span>Tema {resolvedTheme === 'dark' ? 'Tokyo Night' : 'Light Mode'}</span>
                </div>
                <span className="text-[10px] font-mono text-muted-foreground uppercase px-1.5 py-0.5 bg-muted">
                  {resolvedTheme}
                </span>
              </button>
            </div>

            <div className="h-[1px] bg-border my-1" />

            {/* Logout */}
            <div className="py-0.5">
              <button
                type="button"
                onClick={handleLogout}
                className="flex w-full items-center gap-2 px-2.5 py-2 text-xs text-red-500 dark:text-red-400 hover:bg-destructive/15 transition-colors text-left"
              >
                <LogOut className="h-4 w-4 shrink-0" />
                <span className="font-semibold">Keluar (Logout)</span>
              </button>
            </div>
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
