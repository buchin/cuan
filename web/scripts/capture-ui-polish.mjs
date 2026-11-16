import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.CUAN_BROWSER_MODULE || path.join(os.homedir(), '.local/lib/node_modules/playwright'))

const output = process.env.CUAN_TASK_ARTIFACTS_DIR || process.argv[2]
if (!output) throw Error('Supply an output directory')
await fs.mkdir(output, { recursive: true, mode: 0o700 })

const now = '2026-11-16T08:00:00Z'
const contacts = [
  {
    id: 'room-1',
    conversation_id: 'room-1',
    name: 'Website Redesign',
    type: 'customer',
    email: 'client@studio.test',
    unread_count: 2,
    resolved: false,
    status: 'online',
    last_message: 'Halo, bagaimana perkembangan pembaruan estetika monochrome dan Things 3?',
    updated_at: now,
    task_count: 1,
    active_tasks: 1,
  },
  {
    id: 'room-2',
    conversation_id: 'room-2',
    name: '@cuan Workspace Setup',
    type: 'agent',
    agent_id: 'agent_cuan',
    unread_count: 0,
    resolved: false,
    status: 'online',
    last_message: 'Pemeriksaan build frontend dan verifikasi visual sedang berjalan.',
    updated_at: now,
    task_count: 2,
    active_tasks: 1,
  },
  {
    id: 'room-3',
    conversation_id: 'room-3',
    name: 'Logo & Brand Identity',
    type: 'customer',
    email: 'brand@studio.test',
    unread_count: 0,
    resolved: true,
    status: 'offline',
    last_message: 'Pekerjaan selesai dan aset logo monochrome telah terverifikasi.',
    updated_at: now,
    task_count: 1,
    done_tasks: 1,
  },
]

const agents = [{ id: 'agent_cuan', name: '@cuan', status: 'active', enabled: true, harness: 'agy', provider: 'agy', supports_vision: true }]
const notifications = [
  {
    id: 'notif-1',
    title: 'Proposal balasan membutuhkan persetujuan',
    kind: 'approval',
    read: false,
    resolved: false,
    href: '/inbox/room-1',
    created_at: now,
  },
]

const entry = `import React from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter,useLocation} from 'react-router-dom';
import {LanguageProvider} from '/src/context/LanguageContext.tsx';
import {AuthProvider} from '/src/context/AuthContext.tsx';
import {NotificationsProvider} from '/src/context/NotificationsContext.tsx';
import {AppShell} from '/src/components/layout/AppShell.tsx';
import {InboxSubPanel} from '/src/features/inbox/InboxSubPanel.tsx';
import {LoginScreen} from '/src/components/auth/LoginScreen.tsx';
import '/src/index.css';

const contacts = ${JSON.stringify(contacts)};
const agents = ${JSON.stringify(agents)};
const noop = () => {};

function Scene() {
  const location = useLocation();
  if (location.pathname === '/login') {
    return React.createElement(LoginScreen, null);
  }
  const subPanel = React.createElement(InboxSubPanel, {
    contacts,
    agents,
    activeContactId: 'room-1',
    onSelectContact: noop,
    selectedTab: 'all',
    searchQuery: '',
    onSearchChange: noop,
    onTabChange: noop,
    onNewStudio: noop,
    onNewMessage: noop,
  });
  const isDesktop = typeof window !== 'undefined' && window.innerWidth >= 1200;
  return React.createElement(AppShell, {
    currentTab: 'inbox',
    onSelectTab: noop,
    inboxSubPanel: isDesktop ? subPanel : null,
  }, isDesktop ? null : subPanel);
}

createRoot(document.getElementById('root')).render(
  React.createElement(BrowserRouter, null,
    React.createElement(LanguageProvider, null,
      React.createElement(AuthProvider, null,
        React.createElement(NotificationsProvider, null,
          React.createElement(Scene, null)
        )
      )
    )
  )
);
`

const webRoot = path.resolve('web')
process.chdir(webRoot)
const server = await createServer({
  root: webRoot,
  cacheDir: path.join(path.resolve(output), 'vite-polish-cache'),
  server: { host: '127.0.0.1', port: 0 },
  plugins: [
    {
      name: 'polish-fixture',
      enforce: 'pre',
      resolveId(id) {
        if (id === '/__polish_entry.jsx') return '\0polish-fixture-entry.jsx'
      },
      load(id) {
        if (id === '\0polish-fixture-entry.jsx') return entry
      },
      configureServer(s) {
        s.middlewares.use(async (req, res, next) => {
          if (req.url.startsWith('/login') || req.url.startsWith('/inbox')) {
            res.setHeader('Content-Type', 'text/html')
            res.end(
              await s.transformIndexHtml(
                req.url,
                '<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/__polish_entry.jsx"></script></body></html>',
              ),
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
    args: process.env.CUAN_BROWSER_NO_SANDBOX === '1' ? ['--no-sandbox'] : [],
  })

  // Capture mobile (390px) and desktop (1440px) in both light and dark mode
  const configurations = [
    { mode: 'light', dark: false, width: 390, height: 844 },
    { mode: 'light', dark: false, width: 1440, height: 900 },
    { mode: 'dark', dark: true, width: 390, height: 844 },
    { mode: 'dark', dark: true, width: 1440, height: 900 },
  ]

  for (const config of configurations) {
    const context = await browser.newContext({
      viewport: { width: config.width, height: config.height },
      hasTouch: config.width === 390,
    })
    await context.addInitScript(() => {
      localStorage.setItem('cuan-language', 'id')
      localStorage.setItem('cuan_token', 'isolated-fixture')
    })

    const page = await context.newPage()
    page.on('console', (msg) => console.log(`[PAGE LOG ${config.width}]`, msg.type(), msg.text()))
    page.on('pageerror', (err) => console.log(`[PAGE ERROR ${config.width}]`, err.stack || err.message))

    await page.route('**/api/**', async (route) => {
      const pathname = new URL(route.request().url()).pathname
      if (!pathname.startsWith('/api/')) return route.continue()
      if (pathname.includes('/auth/status')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ setup_required: false }) })
      if (pathname.includes('/auth/me')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ user: { id: 'owner', email: 'owner@cuan.test', name: 'Owner', role: 'owner' } }) })
      if (pathname.includes('/notifications')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ notifications }) })
      if (pathname.includes('/messages')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ messages: [], tasks: [], contact: contacts[0], requests: [] }) })
      if (pathname.includes('preferences')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: {} }) })
      if (pathname.includes('/agents')) return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ agents }) })
      if (pathname.endsWith('/stream') || pathname.endsWith('/events')) return route.fulfill({ status: 200, contentType: 'text/event-stream', body: '' })
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: {} }) })
    })

    // 1. Capture Login Screen
    await page.goto(new URL('/login', origin).href, { waitUntil: 'domcontentloaded' })
    await page.evaluate((dark) => document.documentElement.classList.toggle('dark', dark), config.dark)
    await page.waitForSelector('form', { timeout: 15000 })
    const loginImgName = `login-${config.width}-${config.mode}.png`
    await page.screenshot({ path: path.join(output, loginImgName), fullPage: true })
    console.log(`Captured ${loginImgName}`)

    // 2. Capture Inbox Screen (Unread vs Unresolved, Bell with amber dot)
    await page.goto(new URL('/inbox', origin).href, { waitUntil: 'domcontentloaded' })
    await page.evaluate((dark) => document.documentElement.classList.toggle('dark', dark), config.dark)
    await page.locator('[data-conversation-row]').first().waitFor({ state: 'visible', timeout: 15000 })

    // Click "All" / "Semua" tab so both unresolved (○) and resolved (✓) conversations are visible in the screenshot
    const allTab = page.getByRole('tab', { name: /^semua$|^all$/i })
    if (await allTab.count() > 0) {
      await allTab.first().click()
    }

    // Assert visual elements exist before screenshot
    const workflowIndicators = page.locator('[data-testid="workflow-status-indicator"]')
    assert.ok((await workflowIndicators.count()) >= 3, 'Must render at least 3 workflow status indicators')
    const unreadDots = page.locator('[data-testid="unread-micro-dot"]')
    assert.ok((await unreadDots.count()) >= 1, 'Must render at least 1 unread micro-dot')

    const inboxImgName = `inbox-${config.width}-${config.mode}.png`
    await page.screenshot({ path: path.join(output, inboxImgName), fullPage: true })
    console.log(`Captured ${inboxImgName}`)

    await context.close()
  }
} finally {
  await browser?.close()
  server.httpServer.closeAllConnections?.()
  await server.close()
  process.exit(0)
}
