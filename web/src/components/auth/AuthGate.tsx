import React from 'react'
import { useAuth } from '../../context/AuthContext'
import { LoginScreen } from './LoginScreen'
import { RefreshCw } from 'lucide-react'

interface AuthGateProps {
  children: React.ReactNode
  onNavigate?: (path: string) => void
}

export function AuthGate({ children, onNavigate }: AuthGateProps) {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen bg-background text-foreground flex items-center justify-center p-4">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="h-8 w-8 text-primary animate-spin" />
          <p className="text-xs font-mono font-medium text-muted-foreground tracking-wider uppercase">
            Memeriksa Sesi Cuan...
          </p>
        </div>
      </div>
    )
  }

  if (!isAuthenticated) {
    return <LoginScreen onNavigate={onNavigate} />
  }

  return <>{children}</>
}
