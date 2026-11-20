// Real React components visual verification for Inbox Navigation Unread Badge.
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.CUAN_BROWSER_MODULE || path.join(os.homedir(), '.local/lib/node_modules/playwright'))

const output = process.argv[2] || process.env.CUAN_TASK_ARTIFACTS_DIR || '/tmp/cuan-task-agent-2304699457/artifacts/ui'
await fs.mkdir(output, { recursive: true, mode: 0o700 })

const now = '2026-11-20T10:00:00Z'
// 19 unresolved conversations (as in user report)
const mockContacts = Array.from({ length: 19 }, (_, i) => ({
  id: `conv_${i + 1}`,
  conversation_id: `conv_${i + 1}`,
  name: `Percakapan ${i + 1}`,
  type: 'agent',
  resolved: false,
  has_messages: true,
  unread_count: 0,
  status: 'online',
  last_message: 'Halo sob, update terkini terkait pekerjaan.',
  updated_at: now,
}))
const agents = [{ id: 'agent_cuan', name: '@cuan', status: 'active', enabled: true, harness: 'agy', provider: 'agy' }]

const entry = `import React, { useState, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, useSearchParams } from 'react-router-dom';
import { LanguageProvider } from '/src/context/LanguageContext.tsx';
import { AuthProvider } from '/src/context/AuthContext.tsx';
import { AppShell } from '/src/components/layout/AppShell.tsx';
import { InboxSubPanel } from '/src/features/inbox/InboxSubPanel.tsx';
import '/src/index.css';

const baseContacts = ${JSON.stringify(mockContacts)};
const agents = ${JSON.stringify(agents)};
const noop = () => {};

function BadgeScene() {
  const [params] = useSearchParams();
  const unreadMode = params.get('mode') === 'unread';
  const [unreadCount, setUnreadCount] = useState(unreadMode ? 2 : 0);

  useEffect(() => {
    setUnreadCount(unreadMode ? 2 : 0);
  }, [unreadMode]);

  return React.createElement(
    AppShell,
    {
      currentTab: 'inbox',
      onSelectTab: noop,
      inboxUnread: unreadCount,
      inboxSubPanel: React.createElement(InboxSubPanel, {
        contacts: baseContacts,
        agents,
        activeContactId: baseContacts[0].id,
        onSelectContact: noop,
        selectedTab: 'all',
        searchQuery: '',
        onSearchChange: noop,
        onTabChange: noop,
        onNewStudio: noop,
        onNewMessage: noop,
      }),
    },
    React.createElement(
      'div',
      { className: 'p-6' },
      React.createElement('h2', { className: 'text-xl font-bold' }, 'Inbox Workspace'),
      React.createElement(
        'p',
        { className: 'text-muted-foreground mt-2' },
        'Mode: ' + (unreadMode ? '2 Unread Messages' : 'Zero Unread (19 unresolved conversations read)')
      ),
      React.createElement(
        'div',
        { className: 'mt-4 flex gap-2' },
        React.createElement(
          'button',
          {
            'data-testid': 'btn-mark-all-read',
            onClick: () => setUnreadCount(0),
            className: 'px-3 py-1.5 bg-secondary text-secondary-foreground rounded text-sm',
          },
          'Mark All Read'
        ),
        React.createElement(
          'button',
          {
            'data-testid': 'btn-simulate-unread',
            onClick: () => setUnreadCount(2),
            className: 'px-3 py-1.5 bg-primary text-primary-foreground rounded text-sm',
          },
          'Receive 2 Messages'
        )
      )
    )
  );
}

createRoot(document.getElementById('root')).render(
  React.createElement(
    BrowserRouter,
    null,
    React.createElement(
      LanguageProvider,
      null,
      React.createElement(
        AuthProvider,
        null,
        React.createElement(BadgeScene)
      )
    )
  )
);
`

const webRoot = path.resolve('web')
process.chdir(webRoot)

const server = await createServer({
  root: webRoot,
  envDir: path.resolve(output),
  cacheDir: path.join(path.resolve(output), 'vite-cache'),
  server: { host: '127.0.0.1', port: 0 },
  plugins: [
    {
      name: 'badge-fixture',
      enforce: 'pre',
      resolveId(id) {
        if (id === '/__badge_entry.js') return '\0badge-entry.js'
      },
      load(id) {
        if (id === '\0badge-entry.js') return entry
      },
      configureServer(s) {
        s.middlewares.use(async (req, res, next) => {
          if (req.url.startsWith('/inbox')) {
            res.setHeader('Content-Type', 'text/html')
            res.end(
              await s.transformIndexHtml(
                req.url,
                '<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/__badge_entry.js"></script></body></html>'
              )
            )
          } else next()
        })
      },
    },
  ],
})

await server.listen()
const origin = server.resolvedUrls.local[0]
let browser

try {
  browser = await chromium.launch({
    headless: true,
    executablePath: process.env.CUAN_CHROMIUM_EXECUTABLE,
    args: ['--no-sandbox'],
  })

  // 1. Mobile 390px Viewport
  {
    const context = await browser.newContext({
      viewport: { width: 390, height: 844 },
      hasTouch: true,
    })
    await context.addInitScript(() => {
      localStorage.setItem('cuan-language', 'id')
      localStorage.setItem('cuan_token', 'isolated-fixture')
    })
    const page = await context.newPage()

    // 1A. Zero Unread state (19 unresolved conversations, but all read)
    await page.goto(`${origin}inbox?mode=zero`, { waitUntil: 'networkidle' })
    await page.waitForSelector('[data-testid="bottom-nav-inbox"]')

    // Assert that bottom nav badge is completely ABSENT
    const zeroBadgeBottom = await page.$('[data-testid="inbox-unread-badge-bottom"]')
    assert.equal(zeroBadgeBottom, null, 'Bottom nav inbox badge must NOT be rendered when unread count is 0')

    await page.screenshot({ path: path.join(output, 'inbox-badge-zero-mobile-390.png') })
    console.log('✓ Captured: inbox-badge-zero-mobile-390.png (Clean zero state)')

    // 1B. Unread state (2 unread messages arrive)
    await page.goto(`${origin}inbox?mode=unread`, { waitUntil: 'networkidle' })
    await page.waitForSelector('[data-testid="inbox-unread-badge-bottom"]')

    const unreadBadgeBottom = await page.$('[data-testid="inbox-unread-badge-bottom"]')
    assert.ok(unreadBadgeBottom, 'Bottom nav inbox badge must be rendered when unread count > 0')

    const badgeText = (await unreadBadgeBottom.textContent()).trim()
    assert.equal(badgeText, '2', 'Badge text must show "2"')

    const ariaLabel = await unreadBadgeBottom.getAttribute('aria-label')
    assert.equal(ariaLabel, '2 pesan belum dibaca', 'aria-label must accurately describe unread messages in Indonesian')

    await page.screenshot({ path: path.join(output, 'inbox-badge-unread-mobile-390.png') })
    console.log('✓ Captured: inbox-badge-unread-mobile-390.png (Active badge: 2 pesan belum dibaca)')

    // 1C. Reactive clearing when read
    await page.click('[data-testid="btn-mark-all-read"]')
    await page.waitForTimeout(100)
    const clearedBadgeBottom = await page.$('[data-testid="inbox-unread-badge-bottom"]')
    assert.equal(clearedBadgeBottom, null, 'Bottom nav inbox badge must disappear reactively when read')
    console.log('✓ Verified: Reactive clearing on mobile')

    await context.close()
  }

  // 2. Desktop 1440px Viewport
  {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 900 },
    })
    await context.addInitScript(() => {
      localStorage.setItem('cuan-language', 'id')
      localStorage.setItem('cuan_token', 'isolated-fixture')
    })
    const page = await context.newPage()

    // 2A. Zero Unread state on desktop
    await page.goto(`${origin}inbox?mode=zero`, { waitUntil: 'networkidle' })
    await page.waitForSelector('[data-nav-tone="inbox"]')

    const zeroBadgeRail = await page.$('[data-testid="inbox-unread-badge-rail"]')
    assert.equal(zeroBadgeRail, null, 'Desktop rail inbox badge must NOT be rendered when unread count is 0')

    await page.screenshot({ path: path.join(output, 'inbox-badge-zero-desktop-1440.png') })
    console.log('✓ Captured: inbox-badge-zero-desktop-1440.png (Clean desktop zero state)')

    // 2B. Unread state on desktop (2 unread messages)
    await page.goto(`${origin}inbox?mode=unread`, { waitUntil: 'networkidle' })
    await page.waitForSelector('[data-testid="inbox-unread-badge-rail"]')

    const unreadBadgeRail = await page.$('[data-testid="inbox-unread-badge-rail"]')
    assert.ok(unreadBadgeRail, 'Desktop rail inbox badge must be rendered when unread count > 0')

    const badgeRailText = (await unreadBadgeRail.textContent()).trim()
    assert.equal(badgeRailText, '2', 'Desktop badge text must show "2"')

    const railAriaLabel = await unreadBadgeRail.getAttribute('aria-label')
    assert.equal(railAriaLabel, '2 pesan belum dibaca', 'Desktop aria-label must accurately describe unread messages')

    await page.screenshot({ path: path.join(output, 'inbox-badge-unread-desktop-1440.png') })
    console.log('✓ Captured: inbox-badge-unread-desktop-1440.png (Active desktop badge: 2 pesan belum dibaca)')

    // 2C. Reactive clearing when read
    await page.click('[data-testid="btn-mark-all-read"]')
    await page.waitForTimeout(100)
    const clearedBadgeRail = await page.$('[data-testid="inbox-unread-badge-rail"]')
    assert.equal(clearedBadgeRail, null, 'Desktop rail inbox badge must disappear reactively when read')
    console.log('✓ Verified: Reactive clearing on desktop')

    await context.close()
  }

  console.log('ALL VISUAL & BEHAVIORAL VERIFICATIONS PASSED 100%!')
} finally {
  if (browser) await browser.close()
  await server.close()
}
