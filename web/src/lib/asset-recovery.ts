const recoveryKey = 'cuan:asset-recovery-entry'

export function isAssetLoadFailure(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error || '')
  return /failed to fetch dynamically imported module|importing a module script failed|error loading dynamically imported module|unable to preload css|loading chunk .+ failed|failed to load module script/i.test(message)
}

// One recovery per loaded entry, preserving authentication and persisted drafts.
// If storage is unavailable, leave the retry control visible instead of looping.
export function recoverStaleAsset(error: unknown, entry: string, online: boolean, storage: Pick<Storage, 'getItem' | 'setItem'>, reload: () => void): boolean {
  if (!online || !entry || !isAssetLoadFailure(error)) return false
  try {
    if (storage.getItem(recoveryKey) === entry) return false
    storage.setItem(recoveryKey, entry)
  } catch { return false }
  reload()
  return true
}

export function installAssetRecovery() {
  window.addEventListener('vite:preloadError', event => {
    const entry = document.querySelector<HTMLScriptElement>('script[type="module"][src]')?.src || ''
    try {
      if (recoverStaleAsset((event as Event & { payload: unknown }).payload, entry, navigator.onLine, window.sessionStorage, () => window.location.reload())) event.preventDefault()
    } catch { /* Storage access can be restricted before the helper is called. */ }
  })
}
