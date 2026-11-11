import { createContext, useContext, useEffect, useState } from 'react'
interface InstallPrompt extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: string }>
}
const PWAContext = createContext({
  installed: false,
  canInstall: false,
  install: async () => {},
  error: '',
})
export function PWAProvider({ children }: { children: React.ReactNode }) {
  const [prompt, setPrompt] = useState<InstallPrompt | null>(null),
    [installed, setInstalled] = useState(false),
    [error, setError] = useState('')
  useEffect(() => {
    const media = window.matchMedia('(display-mode: standalone)')
    const check = () =>
      setInstalled(
        media.matches ||
          Boolean(
            (navigator as Navigator & { standalone?: boolean }).standalone,
          ),
      )
    check()
    media.addEventListener('change', check)
    const isPublicLanding =
      typeof window !== 'undefined' &&
      (window.location.pathname === '/' ||
        window.location.pathname === '/sponsor' ||
        window.location.pathname === '/sponsors')

    const ready = (e: Event) => {
      e.preventDefault()
      if (isPublicLanding) return
      setPrompt(e as InstallPrompt)
    }
    const done = () => {
      setPrompt(null)
      setInstalled(true)
    }
    window.addEventListener('beforeinstallprompt', ready)
    window.addEventListener('appinstalled', done)
    if (!isPublicLanding && 'serviceWorker' in navigator && window.isSecureContext)
      navigator.serviceWorker
        .register('/sw.js', { updateViaCache: 'none' })
        .catch(() =>
          setError('App installation is unavailable. Reload and try again.'),
        )
    return () => {
      media.removeEventListener('change', check)
      window.removeEventListener('beforeinstallprompt', ready)
      window.removeEventListener('appinstalled', done)
    }
  }, [])
  async function install() {
    if (!prompt) return
    setError('')
    try {
      await prompt.prompt()
      await prompt.userChoice
      setPrompt(null)
    } catch {
      setError('Could not open the install prompt. Use your browser menu.')
    }
  }
  return (
    <PWAContext.Provider
      value={{ installed, canInstall: !!prompt, install, error }}
    >
      {children}
    </PWAContext.Provider>
  )
}
export const usePWA = () => useContext(PWAContext)
