import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react'
import { useAuth } from './AuthContext'
import { apiFetch } from '../api/client'
export interface AppNotification {
  id: string
  kind: 'approval' | 'blocker' | 'completed' | 'customer'
  title: string
  context?: string
  summary?: string
  status?: string
  action_label?: string
  resolved?: boolean
  href: string
  created_at: string
  read: boolean
}
const NotificationsContext = createContext({
  items: [] as AppNotification[],
  loading: false,
  error: '',
  refresh: async () => {},
  markRead: async (_ids?: string[]) => {},
})
export function NotificationsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const { isAuthenticated } = useAuth(),
    [items, setItems] = useState<AppNotification[]>([]),
    [loading, setLoading] = useState(false),
    [error, setError] = useState('')
  const refresh = useCallback(async () => {
    if (!isAuthenticated) return
    setLoading(true)
    try {
      const result = await apiFetch<{ items: AppNotification[] }>(
        '/api/v1/notifications',
      )
      setItems(result.items || [])
      setError('')
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : 'Could not load notifications. Try again.',
      )
    } finally {
      setLoading(false)
    }
  }, [isAuthenticated])
  useEffect(() => {
    if (!isAuthenticated) {
      setItems([])
      return
    }
    void refresh()
    const update = () => {
      if (document.visibilityState === 'visible') void refresh()
    }
    const timer = setInterval(update, 30000)
    document.addEventListener('visibilitychange', update)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', update)
    }
  }, [isAuthenticated, refresh])
  async function markRead(ids?: string[]) {
    try {
      const result = await apiFetch<{ items: AppNotification[] }>(
        '/api/v1/notifications',
        { method: 'PATCH', body: JSON.stringify(ids ? { ids } : { all: true }) },
      )
      setItems((previous) => (result.items || []).map((item) => ({
        ...item, read: item.read || previous.some((old) => old.id === item.id && old.read),
      })))
      setError('')
    } catch (e) {
      setError('Could not mark notifications as read. Try again.')
      throw e
    }
  }
  return (
    <NotificationsContext.Provider
      value={{ items, loading, error, refresh, markRead }}
    >
      {children}
    </NotificationsContext.Provider>
  )
}
export const useNotifications = () => useContext(NotificationsContext)
