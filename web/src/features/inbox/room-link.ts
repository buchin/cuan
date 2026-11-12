/**
 * Utility functions for detecting and parsing internal Cuan conversation and task links.
 * Supports relative paths (/inbox/:id, /chat/:id, /tasks/:id) and absolute internal URLs
 * hosted on Cuan domains (e.g. cuan.work, app.cuan.work, localhost, etc.).
 */

export function isCuanHost(hostname: string, host?: string): boolean {
  if (!hostname) return false
  const h = hostname.toLowerCase()

  // Match browser runtime origin if in browser
  if (typeof window !== 'undefined') {
    if (h === window.location.hostname.toLowerCase()) return true
    if (host && host.toLowerCase() === window.location.host.toLowerCase()) return true
  }

  // Local development hostnames
  if (h === 'localhost' || h === '127.0.0.1' || h === '0.0.0.0' || h === 'cuan-server' || h === 'cuan') {
    return true
  }

  // Production and staging host patterns
  if (/^(?:[a-z0-9-]+\.)*dojo\.cc$/i.test(h)) return true
  if (/^(?:[a-z0-9-]+\.)*cuan\.(?:id|io|local|cc|app)$/i.test(h)) return true

  return false
}

/**
 * Extracts conversation room ID from internal /inbox/:id or /chat/:id routes,
 * including relative paths and internal host cuan URLs.
 */
export function extractConversationRoomId(href: string): string | null {
  if (!href) return null
  const clean = href.trim()

  // Match relative paths: /inbox/:id or /chat/:id
  const cleanPath = clean.split(/[?#]/)[0].replace(/\/+$/, '')
  const relMatch = cleanPath.match(/^\/(?:inbox|chat)\/([A-Za-z0-9_-]+)$/i)
  if (relMatch) {
    return relMatch[1]
  }

  // Match absolute URLs
  try {
    if (/^https?:\/\//i.test(clean)) {
      const url = new URL(clean)
      if (isCuanHost(url.hostname, url.host)) {
        const urlPath = url.pathname.replace(/\/+$/, '')
        const absMatch = urlPath.match(/^\/(?:inbox|chat)\/([A-Za-z0-9_-]+)$/i)
        if (absMatch) {
          return absMatch[1]
        }
      }
    }
  } catch {
    // Malformed URL, return null
  }

  return null
}

/**
 * Checks whether an href is a conversation route (/inbox or /chat).
 */
export function isConversationRoute(href: string): boolean {
  return extractConversationRoomId(href) !== null
}

/**
 * Extracts task ID from internal /tasks/:id routes, including relative paths
 * and internal host cuan URLs.
 */
export function extractTaskId(href: string): string | null {
  if (!href) return null
  const clean = href.trim()

  const cleanPath = clean.split(/[?#]/)[0].replace(/\/+$/, '')
  const relMatch = cleanPath.match(/^\/tasks\/([A-Za-z0-9_-]+)$/i)
  if (relMatch) {
    return relMatch[1]
  }

  try {
    if (/^https?:\/\//i.test(clean)) {
      const url = new URL(clean)
      if (isCuanHost(url.hostname, url.host)) {
        const urlPath = url.pathname.replace(/\/+$/, '')
        const absMatch = urlPath.match(/^\/tasks\/([A-Za-z0-9_-]+)$/i)
        if (absMatch) {
          return absMatch[1]
        }
      }
    }
  } catch {
    // Malformed URL, return null
  }

  return null
}

/**
 * Checks whether an href is a task route (/tasks/:id).
 */
export function isTaskRoute(href: string): boolean {
  return extractTaskId(href) !== null
}
