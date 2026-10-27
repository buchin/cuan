// Network-only: never persist account pages, chats, attachments, tokens or APIs.
self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', event => event.waitUntil(self.clients.claim()))
self.addEventListener('fetch', event => {
  const url = new URL(event.request.url)
  if (url.origin !== self.location.origin || event.request.mode !== 'navigate' || url.pathname.startsWith('/api/')) return
  event.respondWith(fetch(event.request, {cache: 'no-store'}).catch(() => new Response('<!doctype html><html lang="en"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Cuan — Offline</title><body style="font:16px system-ui;background:#191919;color:white;padding:32px;max-width:40em;margin:auto"><h1>You’re offline</h1><p>Connect to the internet to open Cuan. Your saved conversations stay on the server.</p><p><a style="color:#5ba0ff" href="/studio">Try again</a></p></body></html>', { status:503, headers:{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store'} })))
})
self.addEventListener('push', event => {
  let data = {}
  try { data = event.data?.json() || {} } catch { /* Generic notification only. */ }
  let href = '/notifications'
  try { const url = new URL(data.href, self.location.origin); if(url.origin===self.location.origin && /^\/(inbox|projects|tasks|notifications)(\/|$)/.test(url.pathname)) href=url.pathname+url.search } catch {}
  event.waitUntil(self.registration.showNotification('Cuan', {body:'There is an update that needs your attention.',icon:'/icons/cuan-192.png',badge:'/icons/cuan-192.png',tag:typeof data.tag==='string'?data.tag:'cuan-update',data:{href}}))
})
self.addEventListener('notificationclick', event => {
  event.notification.close()
  const href = event.notification.data?.href || '/notifications'
  event.waitUntil(self.clients.matchAll({type:'window',includeUncontrolled:true}).then(async windows => {
    for(const client of windows){if(new URL(client.url).origin===self.location.origin){await client.navigate(href);return client.focus()}}
    return self.clients.openWindow(href)
  }))
})
