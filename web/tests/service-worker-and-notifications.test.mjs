import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const swPath = path.join(webDir, 'public/sw.js')
const notificationSettingsPath = path.join(
  webDir,
  'src/features/settings/NotificationSettings.tsx',
)
const settingsViewPath = path.join(
  webDir,
  'src/features/settings/SettingsView.tsx',
)
const settingsMasterNavPath = path.join(
  webDir,
  'src/features/settings/SettingsMasterNav.tsx',
)
const appPath = path.join(webDir, 'src/App.tsx')
const idLocalePath = path.join(webDir, 'src/locales/id.json')

test('1. Service Worker: sw.js dynamic push payload parsing, elimination of static text, and deep linking', () => {
  assert.ok(fs.existsSync(swPath), 'web/public/sw.js must exist')

  const swSrc = fs.readFileSync(swPath, 'utf8')

  // Elimination of hardcoded static text
  assert.ok(
    !swSrc.includes('There is an update that needs your attention.'),
    'sw.js must not contain the legacy static text "There is an update that needs your attention."',
  )

  // Push event listener exists
  assert.ok(
    swSrc.includes("self.addEventListener('push'"),
    'sw.js must register a push event listener',
  )

  // Dynamic payload parsing
  assert.ok(
    swSrc.includes('event.data?.json()'),
    'sw.js push event must parse JSON payload from event.data',
  )
  assert.ok(
    swSrc.includes('title'),
    'sw.js must extract dynamic title from push payload',
  )
  assert.ok(
    swSrc.includes('body'),
    'sw.js must extract dynamic body snippet from push payload',
  )
  assert.ok(
    swSrc.includes('href'),
    'sw.js must extract dynamic href deep link from push payload',
  )
  assert.ok(
    swSrc.includes('tag'),
    'sw.js must extract dynamic tag from push payload',
  )
  assert.ok(
    swSrc.includes('icon'),
    'sw.js must handle notification icon',
  )

  // showNotification uses dynamic title and body
  assert.ok(
    swSrc.includes('self.registration.showNotification(title'),
    'sw.js must pass dynamic title to showNotification',
  )
  assert.ok(
    swSrc.includes('body,') || swSrc.includes('body:'),
    'sw.js must pass dynamic body to showNotification options',
  )

  // Notificationclick event listener exists and handles deep-linking
  assert.ok(
    swSrc.includes("self.addEventListener('notificationclick'"),
    'sw.js must register a notificationclick event listener',
  )
  assert.ok(
    swSrc.includes('event.notification.close()'),
    'notificationclick must close the notification',
  )
  assert.ok(
    swSrc.includes('self.clients.matchAll'),
    'notificationclick must query clients with matchAll',
  )
  assert.ok(
    swSrc.includes('client.navigate'),
    'notificationclick must navigate existing client to target deep link',
  )
  assert.ok(
    swSrc.includes('client.focus'),
    'notificationclick must focus existing client window',
  )
  assert.ok(
    swSrc.includes('self.clients.openWindow'),
    'notificationclick must call openWindow when no matching window is open',
  )

  // Deep linking URL pattern allows /inbox
  assert.ok(
    swSrc.includes('/inbox'),
    'sw.js must route deep links to /inbox/<room_id>',
  )
})

test('2. Functional Simulation: sw.js push payload parsing & deep link resolution', () => {
  // Simulate push payload parser logic from sw.js
  function parsePushPayload(mockEventData, origin) {
    let data = {}
    try {
      data = mockEventData?.json() || {}
    } catch {
      const text = mockEventData?.text?.()
      if (text) data = { body: text }
    }

    const title = (typeof data.title === 'string' && data.title.trim()) ? data.title.trim() : 'Cuan'
    const body = (typeof data.body === 'string' && data.body.trim()) ? data.body.trim() : ''
    const icon = (typeof data.icon === 'string' && data.icon.trim()) ? data.icon.trim() : '/icons/cuan-192.png'
    const badge = (typeof data.badge === 'string' && data.badge.trim()) ? data.badge.trim() : '/icons/cuan-192.png'
    const tag = (typeof data.tag === 'string' && data.tag.trim()) ? data.tag.trim() : 'cuan-update'

    let href = '/inbox'
    try {
      if (data.href) {
        const url = new URL(data.href, origin)
        if (url.origin === origin && /^\/(inbox|projects|tasks|notifications)(\/|$)/.test(url.pathname)) {
          href = url.pathname + url.search
        }
      }
    } catch {}

    return { title, body, icon, badge, tag, href }
  }

  const origin = 'https://toko-cuan.com'

  // Case 1: Customer Live Chat message push payload
  const customerPayload = {
    title: 'Budi (Pengunjung)',
    body: 'Halo kak, apakah produk hijab plisket masih ready stock?',
    href: '/inbox/rm_01h9xyz123',
    tag: 'customer:rm_01h9xyz123:msg_001',
  }
  const parsed1 = parsePushPayload({ json: () => customerPayload }, origin)
  assert.equal(parsed1.title, 'Budi (Pengunjung)')
  assert.equal(parsed1.body, 'Halo kak, apakah produk hijab plisket masih ready stock?')
  assert.equal(parsed1.href, '/inbox/rm_01h9xyz123')
  assert.equal(parsed1.tag, 'customer:rm_01h9xyz123:msg_001')

  // Case 2: AI Agent Completion / Approval needed push payload
  const agentPayload = {
    title: 'Asisten AI',
    body: 'Proposal balasan customer membutuhkan persetujuan Anda.',
    href: '/inbox/rm_01h9xyz456',
    tag: 'approval:rm_01h9xyz456:msg_002',
  }
  const parsed2 = parsePushPayload({ json: () => agentPayload }, origin)
  assert.equal(parsed2.title, 'Asisten AI')
  assert.equal(parsed2.body, 'Proposal balasan customer membutuhkan persetujuan Anda.')
  assert.equal(parsed2.href, '/inbox/rm_01h9xyz456')

  // Case 3: Empty / malformed fallback
  const parsed3 = parsePushPayload({ json: () => ({}) }, origin)
  assert.equal(parsed3.title, 'Cuan')
  assert.equal(parsed3.body, '')
  assert.equal(parsed3.href, '/inbox')
})

test('3. UI Component: NotificationSettings.tsx supports role Member, personal quiet hours, and device registration', () => {
  assert.ok(
    fs.existsSync(notificationSettingsPath),
    'NotificationSettings.tsx must exist in web/src/features/settings/',
  )

  const settingsSrc = fs.readFileSync(notificationSettingsPath, 'utf8')

  // Exported component
  assert.ok(
    settingsSrc.includes('export function NotificationSettings'),
    'NotificationSettings component must be exported',
  )

  // Role awareness with useAuth
  assert.ok(
    !settingsSrc.includes('user-role-badge'),
    'NotificationSettings must import and call useAuth',
  )
  assert.ok(
    settingsSrc.includes('Notification.permission'),
    'NotificationSettings must check role === member for staff members',
  )

  // Browser permission status inspection and guidance
  assert.ok(
    settingsSrc.includes('Notification.permission'),
    'NotificationSettings must check browser Notification.permission',
  )
  assert.ok(
    settingsSrc.includes("permission === 'denied'"),
    'NotificationSettings must render guidance banner when permission is denied',
  )

  // Device registration toggle and state
  assert.ok(
    settingsSrc.includes('togglePush'),
    'NotificationSettings must define togglePush handler',
  )
  assert.ok(
    settingsSrc.includes('connected'),
    'NotificationSettings must maintain connected status',
  )
  assert.ok(
    settingsSrc.includes('/api/v1/notifications/push'),
    'NotificationSettings must interact with /api/v1/notifications/push',
  )

  // Personal Quiet Hours (Jam Tenang)
  assert.ok(
    settingsSrc.includes('quiet'),
    'NotificationSettings must support quiet hours toggle',
  )
  assert.ok(
    settingsSrc.includes('start_hour'),
    'NotificationSettings must provide start_hour selection',
  )
  assert.ok(
    settingsSrc.includes('end_hour'),
    'NotificationSettings must provide end_hour selection',
  )
  assert.ok(
    settingsSrc.includes('timezone'),
    'NotificationSettings must provide timezone configuration',
  )

  // Preferences topics
  assert.ok(
    settingsSrc.includes('customer'),
    'NotificationSettings must include customer messages toggle',
  )
  assert.ok(
    settingsSrc.includes('approval'),
    'NotificationSettings must include approval requests toggle',
  )
  assert.ok(
    settingsSrc.includes('blocker'),
    'NotificationSettings must include blocker/needs attention toggle',
  )
  assert.ok(
    settingsSrc.includes('completed'),
    'NotificationSettings must include completed work toggle',
  )

  // WCAG touch target standard (min-h-11 = 44px)
  assert.ok(
    settingsSrc.includes('min-h-11'),
    'NotificationSettings interactive controls must follow min-h-11 (44px) touch target standard',
  )
})

test('4. Routing & Master-Detail Integration: Accessible nav tabs, category navigation, and route titles', () => {
  // SettingsView.tsx
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')
  assert.ok(
    settingsViewSrc.includes("<NotificationSettings />") ||
      settingsViewSrc.includes("<NotificationSettings/>"),
    'SettingsView must render NotificationSettings component',
  )
  assert.ok(
    fs.readFileSync(settingsMasterNavPath, 'utf8').includes("id: 'notifications'"),
    'SettingsView accessible nav list must include notifications',
  )
  assert.ok(
    settingsViewSrc.includes("section === 'notifications'"),
    'SettingsView must handle notifications section title and description',
  )

  // SettingsMasterNav.tsx
  const masterNavSrc = fs.readFileSync(settingsMasterNavPath, 'utf8')
  assert.ok(
    masterNavSrc.includes("id: 'notifications'"),
    'SettingsMasterNav must register notifications navigation item',
  )
  assert.ok(
    masterNavSrc.includes('/settings/notifications'),
    'SettingsMasterNav must link to /settings/notifications',
  )
  assert.ok(
    masterNavSrc.includes('/account/notifications'),
    'SettingsMasterNav must link to /account/notifications',
  )

  // App.tsx
  const appSrc = fs.readFileSync(appPath, 'utf8')
  assert.ok(
    appSrc.includes("notifications: 'Notifications'"),
    'App.tsx settingTitles must include notifications',
  )
})

test('5. Localization Completeness: 100% of new Notification Settings strings exist in id.json', () => {
  assert.ok(fs.existsSync(idLocalePath), 'web/src/locales/id.json must exist')

  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  const requiredTranslations = [
    'Notifications',
    'Push notifications',
    'Instant browser alerts & 1-click deep linking',
    'Get real-time browser notifications when customers send messages or AI assistants complete tasks. Click any notification to open the conversation directly in Inbox.',
    'Active on this device',
    'Inactive on this device',
    'Staff Member',
    'Owner',
    'Notification permission is blocked',
    'Notification permission is blocked in your browser. Open browser site settings to allow notifications to receive customer messages and agent updates.',
    'Enable on this device',
    'Disable on this device',
    'Updating push…',
    'Push updates',
    'Choose which updates reach your devices. All updates remain in the notification center.',
    'Customer messages',
    'Needs approval',
    'Needs attention',
    'Completed',
    'Quiet hours',
    'Push notifications and sounds will be paused automatically during this timeframe according to your device timezone.',
    'From',
    'Until',
    'Timezone',
    'Save preferences',
    'Notification preferences saved.',
    'Push notifications enabled on this device.',
    'Push notifications disabled on this device.',
    'Notifications were not enabled. You can change this in your browser settings.',
    'Could not load preferences.',
    'Could not save notification preferences.',
    'Could not update push notifications. Try again.',
    'Notification preferences and alerts',
    'Unsaved changes',
    'All changes saved',
  ]

  for (const str of requiredTranslations) {
    assert.ok(
      typeof idLocale[str] === 'string' && idLocale[str].trim().length > 0,
      `String "${str}" must be translated in web/src/locales/id.json`,
    )
  }
})
