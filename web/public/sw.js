// Network-only: never persist account pages, chats, attachments, tokens or APIs.
self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', event => event.waitUntil(self.clients.claim()))
self.addEventListener('fetch', event => {
  const url = new URL(event.request.url)
  if (url.origin !== self.location.origin || event.request.mode !== 'navigate' || url.pathname.startsWith('/api/')) return
  event.respondWith(fetch(event.request, {cache: 'no-store'}).catch(() => new Response('<!doctype html><html lang="en"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Cuan — Offline</title><body style="font:16px system-ui;background:#191919;color:white;padding:32px;max-width:40em;margin:auto"><h1>You’re offline</h1><p>Connect to the internet to open Cuan. Your saved conversations stay on the server.</p><p><a style="color:#5ba0ff" href="/chat">Try again</a></p></body></html>', { status:503, headers:{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store'} })))
})
self.addEventListener('push', event => {
  let data = {}
  try {
    data = event.data?.json() || {}
  } catch {
    const text = event.data?.text()
    if (text) data = { body: text }
  }

  let title = (typeof data.title === 'string' && data.title.trim()) ? data.title.trim() : ''
  const kind = typeof data.kind === 'string' ? data.kind : ''
  const status = typeof data.status === 'string' ? data.status : ''
  const tag = typeof data.tag === 'string' ? data.tag : ''

  if (!title || title === '@cuan' || title.startsWith('@') || title === 'Cuan' || title === 'Asisten AI') {
    if (kind === 'approval' || status.toLowerCase().includes('approval') || status.toLowerCase().includes('review') || tag.startsWith('approval:')) {
      title = 'Butuh Persetujuan'
    } else if (kind === 'completed' || status.toLowerCase().includes('complete') || tag.includes(':done')) {
      title = 'Pekerjaan Selesai'
    } else if (kind === 'blocker' || status.toLowerCase().includes('attention') || tag.includes(':blocked')) {
      title = 'Perhatian Diperlukan'
    } else if (kind === 'customer') {
      title = 'Pesan Baru'
    } else {
      title = 'Pesan Baru'
    }
  }

  const body = (typeof data.body === 'string' && data.body.trim()) ? data.body.trim() : ''

  let icon = (typeof data.icon === 'string' && data.icon.trim()) ? data.icon.trim() : ''
  if (!icon || icon === '/icons/cuan-192.png') {
    const isTask = tag.startsWith('task:') || (typeof data.href === 'string' && data.href.includes('/tasks/'))

    if (isTask || kind === 'completed' || kind === 'approval' || kind === 'blocker') {
      if (kind === 'completed' || tag.includes(':done') || status.toLowerCase().includes('complete') || title.includes('✓ Selesai') || title.includes('Pekerjaan Selesai')) {
        icon = '/icons/task-done.png'
      } else if (kind === 'approval' || tag.includes(':review') || status.toLowerCase().includes('review') || status.toLowerCase().includes('approval') || title.includes('Persetujuan')) {
        icon = '/icons/task-review.png'
      } else if (kind === 'blocker' || tag.includes(':blocked') || status.toLowerCase().includes('attention')) {
        icon = '/icons/task-blocker.png'
      }
    }
  }
  if (!icon) {
    icon = '/icons/cuan-192.png'
  }

  const badge = (typeof data.badge === 'string' && data.badge.trim()) ? data.badge.trim() : '/icons/cuan-badge-96.png'
  const tag = (typeof data.tag === 'string' && data.tag.trim()) ? data.tag.trim() : 'cuan-update'

  let href = '/inbox'
  try {
    if (data.href) {
      const url = new URL(data.href, self.location.origin)
      if (url.origin === self.location.origin && /^\/(inbox|projects|tasks|notifications)(\/|$)/.test(url.pathname)) {
        href = url.pathname + url.search
      }
    }
  } catch {}

  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      icon,
      badge,
      tag,
      data: { href }
    })
  )
})
self.addEventListener('notificationclick', event => {
  event.notification.close()
  let targetHref = '/inbox'
  try {
    const rawHref = event.notification.data?.href
    if (rawHref) {
      const url = new URL(rawHref, self.location.origin)
      if (url.origin === self.location.origin && /^\/(inbox|projects|tasks|notifications)(\/|$)/.test(url.pathname)) {
        targetHref = url.pathname + url.search
      }
    }
  } catch {}

  const fullUrl = new URL(targetHref, self.location.origin).href
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(async windows => {
      for (const client of windows) {
        if (new URL(client.url).origin === self.location.origin) {
          if ('focus' in client) {
            await client.focus()
          }
          if ('postMessage' in client) {
            client.postMessage({
              type: 'cuan:notification-click',
              href: targetHref,
              url: fullUrl,
            })
          }
          if ('navigate' in client && client.url !== fullUrl) {
            await client.navigate(fullUrl).catch(() => {})
          }
          return
        }
      }
      if (self.clients.openWindow) {
        return self.clients.openWindow(fullUrl)
      }
    })
  )
})
