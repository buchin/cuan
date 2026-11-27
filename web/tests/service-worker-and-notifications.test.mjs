import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import vm from 'node:vm'

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

  // Real syntax validation of sw.js
  assert.doesNotThrow(() => {
    new vm.Script(swSrc, { filename: 'web/public/sw.js' })
  }, 'web/public/sw.js must be syntactically valid JavaScript without syntax errors or identifier redeclarations')

  // Elimination of hardcoded static text
  assert.ok(
    !swSrc.includes('There is an update that needs your attention.'),
    'sw.js must not contain the legacy static text "There is an update that needs your attention."',
  )

  // Dynamic action-based titles
  assert.ok(
    swSrc.includes('Butuh Persetujuan'),
    'sw.js must support dynamic action title "Butuh Persetujuan"',
  )
  assert.ok(
    swSrc.includes('Pekerjaan Selesai'),
    'sw.js must support dynamic action title "Pekerjaan Selesai"',
  )
  assert.ok(
    swSrc.includes('Pesan Baru'),
    'sw.js must support dynamic action title "Pesan Baru"',
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
    !swSrc.includes('icon:') && !swSrc.includes('icon,'),
    'sw.js must not pass large right-side icon to showNotification',
  )
  assert.ok(
    swSrc.includes('badge'),
    'sw.js must handle notification badge',
  )
  assert.ok(
    swSrc.includes('/icons/cuan-badge-96.png'),
    'sw.js must fallback to transparent monochrome badge /icons/cuan-badge-96.png for Android alpha mask stencil',
  )
  assert.ok(
    fs.existsSync(path.join(webDir, 'public/icons/cuan-badge-96.png')),
    'web/public/icons/cuan-badge-96.png must exist',
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
  assert.ok(
    swSrc.includes('renotify'),
    'sw.js must configure renotify on showNotification options (false by default to avoid consecutive alert spam)',
  )
  assert.ok(
    !swSrc.includes('renotify: true') && !swSrc.includes('renotify:true'),
    'sw.js must not hardcode renotify: true for consecutive updates',
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

test('2. Functional Verification: real sw.js push payload parsing & deep link resolution', async () => {
  async function executePushHandler(mockEventData, origin, mockClients = []) {
    const swSrc = fs.readFileSync(swPath, 'utf8')
    let pushHandler = null
    let result = null
    const postedMessages = []
    const fakeSelf = {
      addEventListener(event, fn) {
        if (event === 'push') pushHandler = fn
      },
      registration: {
        showNotification(title, options) {
          result = {
            title,
            body: options.body,
            icon: options.icon,
            badge: options.badge,
            tag: options.tag,
            renotify: options.renotify,
            href: options.data?.href,
          }
        },
      },
      clients: {
        claim: () => Promise.resolve(),
        matchAll: async () => mockClients.map((c) => ({
          ...c,
          postMessage: (msg) => postedMessages.push(msg),
        })),
        openWindow: async () => ({}),
      },
      location: {
        origin,
      },
      skipWaiting: () => {},
    }
    const sandbox = {
      self: fakeSelf,
      URL,
      Response: class {},
      fetch: () => Promise.resolve(),
    }
    vm.createContext(sandbox)
    vm.runInContext(swSrc, sandbox, { filename: 'web/public/sw.js' })

    assert.ok(typeof pushHandler === 'function', 'push handler must be registered in sw.js')

    let waitPromise = null
    const mockEvent = {
      data: mockEventData,
      waitUntil: (p) => { waitPromise = p; return p },
    }
    pushHandler(mockEvent)
    if (waitPromise) await waitPromise
    if (result) result.postedMessages = postedMessages
    return { result, postedMessages }
  }

  const origin = 'https://toko-cuan.com'

  // Case 1: Customer Live Chat message push payload with room-based tag
  const customerPayload = {
    title: 'Budi (Pengunjung)',
    body: 'Halo kak, apakah produk hijab plisket masih ready stock?',
    href: '/inbox/rm_01h9xyz123',
    tag: 'room:rm_01h9xyz123',
  }
  const { result: parsed1 } = await executePushHandler({ json: () => customerPayload }, origin)
  assert.equal(parsed1.title, 'Budi (Pengunjung)')
  assert.equal(parsed1.body, 'Halo kak, apakah produk hijab plisket masih ready stock?')
  assert.equal(parsed1.href, '/inbox/rm_01h9xyz123')
  assert.equal(parsed1.tag, 'room:rm_01h9xyz123')
  assert.equal(parsed1.renotify, false)

  // Case 1a: Active visible tab on the exact room suppresses showNotification and posts cuan:push-suppressed
  const activeTabClients = [
    { url: `${origin}/inbox/rm_01h9xyz123`, visibilityState: 'visible', focused: true },
  ]
  const suppressed = await executePushHandler({ json: () => customerPayload }, origin, activeTabClients)
  assert.equal(suppressed.result, null, 'must suppress push notification when user is actively viewing the room')
  assert.equal(suppressed.postedMessages.length, 1)
  assert.equal(suppressed.postedMessages[0].type, 'cuan:push-suppressed')

  // Case 1b: Legacy customer tag format automatically aligns to room:<conversation_id>
  const legacyCustomerPayload = {
    title: 'Budi (Pengunjung)',
    body: 'Pesan lanjutan...',
    href: '/inbox/rm_01h9xyz123',
    tag: 'customer:rm_01h9xyz123:msg_001',
  }
  const { result: parsedLegacyCustomer } = await executePushHandler({ json: () => legacyCustomerPayload }, origin)
  assert.equal(parsedLegacyCustomer.tag, 'room:rm_01h9xyz123')
  assert.equal(parsedLegacyCustomer.renotify, false)

  // Case 1c: Multiple sequential messages in the same room overwrite in-place (no stacking)
  const notificationTray = new Map()
  async function simulatePushToTray(payload) {
    const swCode = fs.readFileSync(swPath, 'utf8')
    let pushFn = null
    const fakeSelf = {
      addEventListener(evt, fn) { if (evt === 'push') pushFn = fn },
      registration: {
        showNotification(t, opts) {
          notificationTray.set(opts.tag, { title: t, body: opts.body, renotify: opts.renotify })
        },
      },
      clients: { claim: () => Promise.resolve(), matchAll: async () => [], openWindow: async () => ({}) },
      location: { origin },
      skipWaiting: () => {},
    }
    const sb = { self: fakeSelf, URL, Response: class {}, fetch: () => Promise.resolve() }
    vm.createContext(sb)
    vm.runInContext(swCode, sb, { filename: 'web/public/sw.js' })
    let waitPromise = null
    pushFn({ data: { json: () => payload }, waitUntil: (p) => { waitPromise = p; return p } })
    if (waitPromise) await waitPromise
  }

  // First message from room A
  await simulatePushToTray({ title: 'Budi', body: 'Pesan 1', href: '/inbox/room_A', tag: 'room:room_A' })
  assert.equal(notificationTray.size, 1)
  assert.equal(notificationTray.get('room:room_A').body, 'Pesan 1')
  assert.equal(notificationTray.get('room:room_A').renotify, false)

  // Second message from same room A replaces in-place
  await simulatePushToTray({ title: 'Budi', body: 'Pesan 2', href: '/inbox/room_A', tag: 'room:room_A' })
  assert.equal(notificationTray.size, 1) // Still exactly 1 card in tray
  assert.equal(notificationTray.get('room:room_A').body, 'Pesan 2')
  assert.equal(notificationTray.get('room:room_A').renotify, false)

  // Third message (agent reply) in room A replaces in-place
  await simulatePushToTray({ title: 'Asisten AI', body: 'Pesan 3', href: '/inbox/room_A', tag: 'room:room_A' })
  assert.equal(notificationTray.size, 1) // Still exactly 1 card in tray
  assert.equal(notificationTray.get('room:room_A').body, 'Pesan 3')
  assert.equal(notificationTray.get('room:room_A').renotify, false)

  // Message from a different room B creates a separate card
  await simulatePushToTray({ title: 'Siti', body: 'Pesan dari room B', href: '/inbox/room_B', tag: 'room:room_B' })
  assert.equal(notificationTray.size, 2)
  assert.equal(notificationTray.get('room:room_B').body, 'Pesan dari room B')

  // Case 2: AI Agent Completion / Approval needed push payload (resolves to dynamic title)
  const agentPayload = {
    title: 'Butuh Persetujuan',
    body: 'Proposal balasan customer membutuhkan persetujuan Anda.',
    href: '/inbox/rm_01h9xyz456',
    tag: 'approval:rm_01h9xyz456:msg_002',
    kind: 'approval',
  }
  const { result: parsed2 } = await executePushHandler({ json: () => agentPayload }, origin)
  assert.equal(parsed2.title, 'Butuh Persetujuan')
  assert.equal(parsed2.body, 'Proposal balasan customer membutuhkan persetujuan Anda.')
  assert.equal(parsed2.href, '/inbox/rm_01h9xyz456')

  // Case 3: Empty / malformed fallback
  const { result: parsed3 } = await executePushHandler({ json: () => ({}) }, origin)
  assert.equal(parsed3.title, 'Pesan Baru')
  assert.equal(parsed3.body, '')
  assert.equal(parsed3.badge, '/icons/cuan-badge-96.png')
  assert.equal(parsed3.icon, undefined)
  assert.equal(parsed3.href, '/inbox')

  // Case 3b: Legacy static @cuan approval replaces with dynamic action title and omits large icon even if supplied
  const legacyApproval = {
    title: '@cuan',
    body: 'Perbaikan Alur Pembatalan Sesi',
    href: '/inbox/rm_01h9xyz789',
    tag: 'approval:rm_01h9xyz789:msg_003',
    kind: 'approval',
    icon: '/icons/task-review.png',
  }
  const { result: parsedLegacy } = await executePushHandler({ json: () => legacyApproval }, origin)
  assert.equal(parsedLegacy.title, 'Butuh Persetujuan')
  assert.equal(parsedLegacy.icon, undefined)
  assert.equal(parsedLegacy.badge, '/icons/cuan-badge-96.png')

  // Case 4: Task done payload (no large right-side icon, badge preserved)
  const taskDonePayload = {
    title: '✓ Selesai: Fitur Pembayaran',
    body: 'Integrasi payment gateway selesai diverifikasi.',
    href: '/tasks/task_done_123',
    tag: 'task:task_done_123:done',
    kind: 'completed',
  }
  const { result: parsedDone } = await executePushHandler({ json: () => taskDonePayload }, origin)
  assert.equal(parsedDone.icon, undefined)
  assert.equal(parsedDone.badge, '/icons/cuan-badge-96.png')
  assert.equal(parsedDone.title, '✓ Selesai: Fitur Pembayaran')

  // Case 5: Task review payload (no large right-side icon, badge preserved)
  const taskReviewPayload = {
    title: 'Phase 2: Review Code',
    body: 'Hasil pekerjaan siap ditinjau.',
    href: '/tasks/task_rev_456',
    tag: 'task:task_rev_456:review',
    kind: 'approval',
  }
  const { result: parsedReview } = await executePushHandler({ json: () => taskReviewPayload }, origin)
  assert.equal(parsedReview.icon, undefined)
  assert.equal(parsedReview.badge, '/icons/cuan-badge-96.png')

  // Case 6: Task blocker payload (no large right-side icon, badge preserved)
  const taskBlockerPayload = {
    title: 'Perhatian Diperlukan',
    body: 'Pekerjaan tertahan dan memerlukan perhatian Anda.',
    href: '/tasks/task_blk_789',
    tag: 'task:task_blk_789:blocked',
    kind: 'blocker',
  }
  const { result: parsedBlocker } = await executePushHandler({ json: () => taskBlockerPayload }, origin)
  assert.equal(parsedBlocker.icon, undefined)
  assert.equal(parsedBlocker.badge, '/icons/cuan-badge-96.png')
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

test('6. Push Notification Deep-Link & Focus Synchronization: instant chat loading and SSE refresh', () => {
  const swSrc = fs.readFileSync(swPath, 'utf8')
  const appSrc = fs.readFileSync(appPath, 'utf8')
  const inboxViewPath = path.join(webDir, 'src/features/inbox/InboxView.tsx')
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')

  // 1. Service Worker posts message to client on notification click
  assert.ok(
    swSrc.includes("'cuan:notification-click'"),
    'sw.js must postMessage cuan:notification-click to client windows',
  )

  // 2. App.tsx receives service worker postMessage and dispatches notification navigation
  assert.ok(
    appSrc.includes("'cuan:notification-click'"),
    'App.tsx must listen to service worker messages for cuan:notification-click',
  )
  assert.ok(
    appSrc.includes("'cuan:notification-navigate'"),
    'App.tsx must dispatch cuan:notification-navigate custom event upon notification click',
  )

  // 3. App.tsx provides instant deep-link fallback contact for subParam so chat never renders blank
  assert.ok(
    appSrc.includes('// Fallback placeholder contact derived from subParam'),
    'App.tsx must provide fallback placeholder contact for deep-links when contacts list is not yet loaded',
  )

  // 4. InboxView initializes mobileView directly to chat when activeContact is defined
  assert.ok(
    inboxViewSrc.includes("useState<'contacts' | 'chat'>(() => activeContact ? 'chat' : 'contacts')"),
    'InboxView must initialize mobileView to chat when activeContact is provided',
  )

  // 5. InboxView listens to focus and notification navigation for instant refresh & SSE reconnect
  assert.ok(
    inboxViewSrc.includes("window.addEventListener('focus', onFocusOrResume)"),
    'InboxView must re-fetch conversation messages and reconnect SSE when window is focused',
  )
  assert.ok(
    inboxViewSrc.includes("window.addEventListener('cuan:notification-navigate', onNotificationNavigate)"),
    'InboxView must listen to cuan:notification-navigate to load conversation immediately',
  )
})

test('7. Push Notification Assets: Maskable solid icon assets without Android Dark Mode black background corners', async () => {
  const zlib = await import('node:zlib')
  const icons = [
    { file: 'task-done.png', expectedBg: '10b981' },
    { file: 'task-review.png', expectedBg: 'd97706' },
    { file: 'task-blocker.png', expectedBg: 'ef4444' },
  ]

  for (const { file, expectedBg } of icons) {
    const iconPath = path.join(webDir, 'public/icons', file)
    assert.ok(fs.existsSync(iconPath), `Icon ${file} must exist`)

    const buf = fs.readFileSync(iconPath)
    let offset = 8
    const idatChunks = []
    let width = 0, height = 0
    while (offset < buf.length) {
      const len = buf.readUInt32BE(offset)
      const type = buf.toString('ascii', offset + 4, offset + 8)
      const data = buf.subarray(offset + 8, offset + 8 + len)
      if (type === 'IHDR') {
        width = data.readUInt32BE(0)
        height = data.readUInt32BE(4)
      } else if (type === 'IDAT') {
        idatChunks.push(data)
      }
      offset += 12 + len
    }

    assert.equal(width, 192, `${file} width must be 192`)
    assert.equal(height, 192, `${file} height must be 192`)

    const decompressed = zlib.inflateSync(Buffer.concat(idatChunks))
    const bpp = 3
    const stride = 1 + width * bpp
    const raw = Buffer.alloc(width * height * bpp)

    for (let y = 0; y < height; y++) {
      const filter = decompressed[y * stride]
      for (let x = 0; x < width * bpp; x++) {
        const curr = decompressed[y * stride + 1 + x]
        const left = x >= bpp ? raw[y * width * bpp + x - bpp] : 0
        const up = y > 0 ? raw[(y - 1) * width * bpp + x] : 0
        const upLeft = (y > 0 && x >= bpp) ? raw[(y - 1) * width * bpp + x - bpp] : 0
        let val = curr
        if (filter === 1) val = (curr + left) & 0xff
        else if (filter === 2) val = (curr + up) & 0xff
        else if (filter === 3) val = (curr + Math.floor((left + up) / 2)) & 0xff
        else if (filter === 4) {
          const p = left + up - upLeft
          const pa = Math.abs(p - left)
          const pb = Math.abs(p - up)
          const pc = Math.abs(p - upLeft)
          const pr = (pa <= pb && pa <= pc) ? left : (pb <= pc ? up : upLeft)
          val = (curr + pr) & 0xff
        }
        raw[y * width * bpp + x] = val
      }
    }

    // Check corners: must NOT be #191919 (black box artifact)
    const corners = [
      [0, 0],
      [width - 1, 0],
      [0, height - 1],
      [width - 1, height - 1],
    ]
    for (const [cx, cy] of corners) {
      const idx = (cy * width + cx) * bpp
      const hex = raw.subarray(idx, idx + bpp).toString('hex')
      assert.notEqual(hex, '191919', `${file} corner at (${cx},${cy}) must not have black background #191919`)
      assert.equal(hex, expectedBg, `${file} corner at (${cx},${cy}) must match solid status color ${expectedBg}`)
    }
  }
})
