import React, { createContext, useContext, useEffect, useState, useCallback } from 'react'

export interface OwnerInfo {
  id: string
  username?: string
  email: string
}

interface AuthContextType {
  user: OwnerInfo | null
  token: string
  isAuthenticated: boolean
  isLoading: boolean
  setupRequired: boolean | null
  login: (identifier: string, password: string) => Promise<void>
  logout: () => Promise<void>
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>
  refreshUser: () => Promise<void>
  getAuthHeaders: () => HeadersInit
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

const TOKEN_KEY = 'cuan_token'

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string>(() => {
    if (typeof window !== 'undefined') {
      return localStorage.getItem(TOKEN_KEY) || ''
    }
    return ''
  })
  const [user, setUser] = useState<OwnerInfo | null>(null)
  const [isLoading, setIsLoading] = useState<boolean>(true)
  const [setupRequired, setSetupRequired] = useState<boolean | null>(null)

  const getAuthHeaders = useCallback((): HeadersInit => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
    return headers
  }, [token])

  const refreshUser = useCallback(async () => {
    try {
      // 1. Check setup status
      const statusRes = await fetch('/api/v1/auth/status')
      if (statusRes.ok) {
        const statusData = await statusRes.json()
        setSetupRequired(statusData.setup_required ?? false)
      }

      // 2. If token exists, fetch profile from /api/v1/auth/me
      const currentToken = localStorage.getItem(TOKEN_KEY) || token
      if (currentToken) {
        const meRes = await fetch('/api/v1/auth/me', {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${currentToken}`,
          },
        })

        if (meRes.ok) {
          const meData = await meRes.json()
          const ownerInfo = meData.user || meData.owner
          if (ownerInfo) {
            setUser(ownerInfo)
            setToken(currentToken)
          }
        } else if (meRes.status === 401) {
          // Token expired or invalid
          localStorage.removeItem(TOKEN_KEY)
          setToken('')
          setUser(null)
        }
      } else {
        setUser(null)
      }
    } catch (err) {
      console.error('Failed to load user session:', err)
    } finally {
      setIsLoading(false)
    }
  }, [token])

  useEffect(() => {
    refreshUser()
  }, [refreshUser])

  const login = async (identifier: string, password: string) => {
    setIsLoading(true)
    try {
      const payload = {
        username: identifier.trim(),
        email: identifier.trim(),
        password: password.trim(),
      }

      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Username/email atau password salah')
      }

      const receivedToken = data.token
      const receivedUser: OwnerInfo = data.user || data.owner || {
        id: 'owner_admin',
        username: identifier,
        email: identifier,
      }

      localStorage.setItem(TOKEN_KEY, receivedToken)
      setToken(receivedToken)
      setUser(receivedUser)
      setSetupRequired(false)
    } finally {
      setIsLoading(false)
    }
  }

  const logout = async () => {
    try {
      const currentToken = localStorage.getItem(TOKEN_KEY) || token
      if (currentToken) {
        await fetch('/api/v1/auth/logout', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${currentToken}`,
          },
        })
      }
    } catch (err) {
      console.warn('Logout request error:', err)
    } finally {
      localStorage.removeItem(TOKEN_KEY)
      setToken('')
      setUser(null)
    }
  }

  const changePassword = async (currentPassword: string, newPassword: string) => {
    const currentToken = localStorage.getItem(TOKEN_KEY) || token
    const res = await fetch('/api/v1/auth/password', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(currentToken ? { 'Authorization': `Bearer ${currentToken}` } : {}),
      },
      body: JSON.stringify({
        current_password: currentPassword.trim(),
        new_password: newPassword.trim(),
      }),
    })

    const data = await res.json()
    if (!res.ok) {
      throw new Error(data.error || 'Gagal mengubah password')
    }
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        isAuthenticated: !!user,
        isLoading,
        setupRequired,
        login,
        logout,
        changePassword,
        refreshUser,
        getAuthHeaders,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
