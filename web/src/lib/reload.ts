/**
 * Controlled reload mechanism:
 * 1. Cleans Cache Storage API (window.caches.delete)
 * 2. Updates or unregisters service workers so frontend assets are fetched fresh
 * 3. Preserves client application state (e.g. localStorage auth token, theme, language)
 * 4. Force-reloads the window bypassing the browser cache
 */
export async function reloadApp(): Promise<void> {
  // 1. Clear Cache Storage API
  if (typeof window !== 'undefined' && 'caches' in window) {
    try {
      const keys = await window.caches.keys()
      await Promise.all(keys.map((key) => window.caches.delete(key)))
    } catch {
      // Ignore cache clearing errors
    }
  }

  // 2. Clear or update service worker registrations
  if (typeof navigator !== 'undefined' && 'serviceWorker' in navigator) {
    try {
      const registrations = await navigator.serviceWorker.getRegistrations()
      await Promise.all(
        registrations.map(async (reg) => {
          try {
            await reg.update()
          } catch {
            await reg.unregister()
          }
        }),
      )
    } catch {
      // Ignore service worker registration errors
    }
  }

  // 3. Trigger hard reload with bypass cache
  try {
    ;(window.location as unknown as { reload: (force?: boolean) => void }).reload(true)
  } catch {
    window.location.reload()
  }
}
