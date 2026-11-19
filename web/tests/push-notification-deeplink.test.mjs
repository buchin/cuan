import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const appPath = path.join(webDir, 'src/App.tsx')
const swPath = path.join(webDir, 'public/sw.js')
const inboxViewPath = path.join(webDir, 'src/features/inbox/InboxView.tsx')

test('1. App.tsx: Cold start deep-link fallback ensures activeContact is immediately available', () => {
  const appSrc = fs.readFileSync(appPath, 'utf8')

  // Derive visibleContact with cold-start fallback
  assert.ok(
    appSrc.includes('// Fallback placeholder contact derived from subParam so deep-links directly render chat and load messages'),
    'App.tsx must define fallback contact when contacts list is not yet loaded',
  )
  assert.ok(
    appSrc.includes('id: subParam,') && appSrc.includes('conversation_id: subParam,'),
    'Fallback contact must use subParam as conversation_id and id',
  )
  assert.ok(
    appSrc.includes('type: \'agent\' as const'),
    'Fallback contact must define a valid contact type',
  )
})

test('2. App.tsx: Listens to Service Worker notification click messages', () => {
  const appSrc = fs.readFileSync(appPath, 'utf8')

  assert.ok(
    appSrc.includes("event.data?.type === 'cuan:notification-click'"),
    'App.tsx must listen for cuan:notification-click message from service worker',
  )
  assert.ok(
    appSrc.includes("window.dispatchEvent(\n          new CustomEvent('cuan:notification-navigate', { detail: { href: target } }),\n        )"),
    'App.tsx must dispatch cuan:notification-navigate event',
  )
})

test('3. sw.js: notificationclick posts message to open clients and safely navigates', () => {
  const swSrc = fs.readFileSync(swPath, 'utf8')

  assert.ok(
    swSrc.includes("client.postMessage({\n              type: 'cuan:notification-click',\n              href: targetHref,\n              url: fullUrl,\n            })"),
    'sw.js must post message with cuan:notification-click',
  )
  assert.ok(
    swSrc.includes('client.navigate(fullUrl).catch(() => {})'),
    'sw.js must safely catch navigation failures',
  )
})

test('4. InboxView.tsx: Instant reload and SSE reconnect on window focus and notification navigation', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')

  // Focus and visibilitychange listener
  assert.ok(
    inboxViewSrc.includes("window.addEventListener('focus', onFocusOrResume)"),
    'InboxView must listen to window focus',
  )
  assert.ok(
    inboxViewSrc.includes("document.addEventListener('visibilitychange', onFocusOrResume)"),
    'InboxView must listen to document visibilitychange',
  )
  assert.ok(
    inboxViewSrc.includes("window.addEventListener('cuan:notification-navigate', onNotificationNavigate)"),
    'InboxView must listen to cuan:notification-navigate',
  )

  // Disconnection recovery on focus
  assert.ok(
    inboxViewSrc.includes('if (!connected) {\n          clearTimeout(retry)\n          void connect()\n        }'),
    'InboxView must immediately reconnect SSE on focus if disconnected',
  )

  // Mobile view initialization
  assert.ok(
    inboxViewSrc.includes("const [mobileView, setMobileView] = useState<'contacts' | 'chat'>(() => activeContact ? 'chat' : 'contacts')"),
    'InboxView must initialize mobileView to chat when activeContact is provided',
  )
})
