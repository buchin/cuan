import React from 'react'
import { useAuth } from '../../context/AuthContext'
import { LoginScreen } from './LoginScreen'
import { CuanMark } from '../brand/CuanMark'

interface AuthGateProps {
  children: React.ReactNode
  onNavigate?: (path: string) => void
}

export function AuthGate({ children }: AuthGateProps) {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen bg-background bg-[#fafafa] dark:bg-[#121214] text-foreground flex items-center justify-center p-4">
        <div className="flex flex-col items-center gap-3">
          <CuanMark decorative className="h-10 w-10 text-foreground animate-pulse" />
          <p className="text-xs font-mono font-medium text-muted-foreground tracking-wider uppercase">
            Checking your session…
          </p>
        </div>
      </div>
    )
  }

  if (!isAuthenticated) {
    return <LoginScreen />
  }

  return <>{children}</>
}
