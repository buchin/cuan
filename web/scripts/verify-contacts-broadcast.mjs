import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const { chromium } = require(
  process.env.CUAN_BROWSER_MODULE ||
    path.join(os.homedir(), '.local/lib/node_modules/playwright'),
)

const outputDir = process.argv[2]
if (!outputDir) throw Error('Supply an output directory for visual artifacts')
await fs.mkdir(outputDir, { recursive: true, mode: 0o700 })

const now = '2026-11-19T03:30:00Z'
const mockContacts = [
  {
    id: 'c1-alice',
    name: 'Alice Smith',
    email: 'alice@example.com',
    product: 'Live chat',
    conversation_id: 'conv-1',
    tags: ['backer', 'vip'],
    updated_at: now,
  },
  {
    id: 'c2-bob',
    name: 'Bob Jones',
    email: 'bob@example.com',
    product: 'Live chat',
    conversation_id: 'conv-2',
    tags: ['leads'],
    updated_at: now,
  },
  {
    id: 'c3-charlie',
    name: 'Charlie Doe',
    email: 'charlie@example.com',
    product: 'Manual',
    is_manual: true,
    conversation_id: 'conv-3',
    tags: ['backer'],
    updated_at: now,
  },
]

const mockTags = [
  { tag: 'backer', count: 2 },
  { tag: 'leads', count: 1 },
  { tag: 'vip', count: 1 },
]

const audienceResult = {
  total_unique: 3,
  crm_count: 2,
  custom_count: 2,
  recipients: [
    {
      email: 'alice@example.com',
      name: 'Alice Smith',
      source: 'both',
      contact_id: 'c1-alice',
      tags: ['backer', 'vip'],
    },
    {
      email: 'charlie@example.com',
      name: 'Charlie Doe',
      source: 'crm',
      contact_id: 'c3-charlie',
      tags: ['backer'],
    },
    {
      email: 'david@investor.test',
      name: 'David Investor',
      source: 'custom',
    },
  ],
}

const entry = `import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { LanguageProvider } from '/src/context/LanguageContext.tsx';
import { AuthProvider } from '/src/context/AuthContext.tsx';
import { AppShell } from '/src/components/layout/AppShell.tsx';
import { ContactsView } from '/src/features/contacts/ContactsView.tsx';
import '/src/index.css';

const noop = () => {};

function App() {
  return React.createElement(
    BrowserRouter,
    null,
    React.createElement(
      LanguageProvider,
      null,
      React.createElement(
        AuthProvider,
        null,
        React.createElement(
          AppShell,
          { currentTab: 'contacts', onSelectTab: noop },
          React.createElement(ContactsView, null)
        )
      )
    )
  );
}

createRoot(document.getElementById('root')).render(React.createElement(App));`

const webRoot = path.resolve('web')
process.chdir(webRoot)

const server = await createServer({
  root: webRoot,
  cacheDir: path.join(os.tmpdir(), 'cuan-broadcast-vite-cache'),
  server: {
    host: '127.0.0.1',
    port: 0,
    fs: { allow: [webRoot, path.resolve('..'), '/opt/cuan/data'] },
  },
  plugins: [
    {
      name: 'contacts-broadcast-fixture',
      enforce: 'pre',
      resolveId(id) {
        if (id === '/__broadcast_entry.jsx') return '\0broadcast-entry.jsx'
      },
      load(id) {
        if (id === '\0broadcast-entry.jsx') return entry
      },
      configureServer(server) {
        server.middlewares.use(async (req, res, next) => {
          if (req.url.startsWith('/contacts')) {
            res.setHeader('Content-Type', 'text/html')
            res.end(
              await server.transformIndexHtml(
                req.url,
                '<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/__broadcast_entry.jsx"></script></body></html>',
              ),
            )
          } else {
            next()
          }
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
    args:
      process.env.CUAN_BROWSER_NO_SANDBOX === '1' ? ['--no-sandbox'] : [],
  })

  for (const width of [1440, 390]) {
    const isMobile = width === 390
    const context = await browser.newContext({
      viewport: { width, height: isMobile ? 844 : 960 },
      hasTouch: isMobile,
    })
    await context.addInitScript(() => {
      localStorage.setItem('cuan-language', 'id')
      localStorage.setItem('cuan_token', 'isolated-fixture-token')
      localStorage.setItem('theme', 'dark')
    })

    const page = await context.newPage()
    const errors = []
    page.on('pageerror', (err) => errors.push(err.stack || err.message))

    let lastBroadcastSendPayload = null

    await page.route('**/api/**', async (route) => {
      const url = new URL(route.request().url())
      const pathname = url.pathname
      if (!pathname.startsWith('/api/')) return route.continue()

      if (pathname.endsWith('/auth/status')) {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ setup_required: false }) })
      }
      if (pathname.endsWith('/auth/me')) {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ user: { id: 'owner', email: 'owner@example.test', name: 'Owner', role: 'owner' } }) })
      }
      if (pathname === '/api/v1/contacts/broadcast/tags') {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ tags: mockTags }) })
      }
      if (pathname === '/api/v1/contacts/broadcast/audience') {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(audienceResult) })
      }
      if (pathname === '/api/v1/contacts/broadcast/send') {
        lastBroadcastSendPayload = JSON.parse(route.request().postData())
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            status: 'success',
            total_recipients: 3,
            sent_count: 3,
            failed_count: 0,
            idempotency_key: lastBroadcastSendPayload.idempotency_key,
            already_sent: false,
            message: 'Berhasil mengirim broadcast ke 3 penerima',
          }),
        })
      }
      if (pathname === '/api/v1/contacts') {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ contacts: mockContacts }) })
      }
      if (pathname.endsWith('/archive')) {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [] }) })
      }
      if (pathname.includes('/preferences') || pathname.includes('/item-preferences')) {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: {} }) })
      }
      if (pathname.endsWith('/notifications')) {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [] }) })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
    })

    // Navigate to /contacts?view=broadcast to open the broadcast panel
    await page.goto(new URL('/contacts?view=broadcast', origin).href, {
      waitUntil: 'domcontentloaded',
    })

    // Wait for the Broadcast Email heading to appear
    const broadcastHeading = page.getByRole('heading', { name: 'Broadcast Email' })
    await broadcastHeading.waitFor({ state: 'visible', timeout: 15000 })

    // Verify tag chips are visible
    const backerTag = page.getByRole('checkbox', { name: /backer/i })
    await backerTag.waitFor({ state: 'visible', timeout: 10000 })

    // Click tag chip to select #backer
    await backerTag.click()

    // Type in custom email
    const customTextarea = page.locator('#custom-emails-textarea')
    await customTextarea.waitFor({ state: 'visible', timeout: 5000 })
    await customTextarea.fill('david@investor.test')

    // Wait for recipient preview to load in the broadcast panel
    const broadcastPanel = page.getByRole('complementary', { name: 'Broadcast Email' })
    await broadcastPanel.getByText('Daftar Penerima Unik').waitFor({ state: 'visible', timeout: 10000 })
    await broadcastPanel.getByText(/penerima unik/).waitFor({ state: 'visible', timeout: 10000 })
    await broadcastPanel.getByText('alice@example.com').waitFor({ state: 'visible', timeout: 10000 })

    // Fill message subject and content (Phase 2 composer)
    const subjectInput = page.locator('#broadcast-subject-input')
    await subjectInput.waitFor({ state: 'visible', timeout: 5000 })
    await subjectInput.fill('Pembaruan Fitur Terkini & Komunitas')

    const contentTextarea = page.locator('#broadcast-content-textarea')
    await contentTextarea.waitFor({ state: 'visible', timeout: 5000 })
    await contentTextarea.fill('Halo semuanya,\n\nTerima kasih telah mendukung kami. Berikut rangkuman pembaruan penting rilis terbaru.\n\nSalam,\nTim Cuan')

    // Verify RFC 8058 standard notice is visible
    await broadcastPanel.getByText(/List-Unsubscribe \(RFC 8058\)/).waitFor({ state: 'visible', timeout: 5000 })

    // Capture pre-send screenshot showing composer and preview
    const preScreenshotPath = path.join(
      outputDir,
      `broadcast-${isMobile ? 'mobile-390' : 'desktop-1440'}.png`,
    )
    await page.screenshot({ path: preScreenshotPath, fullPage: false })
    console.log(`Saved screenshot: ${preScreenshotPath}`)

    // Click 1-tap Send Broadcast button
    const sendButton = page.locator('#send-broadcast-button')
    await sendButton.waitFor({ state: 'visible', timeout: 5000 })
    await sendButton.click()

    // Wait for delivery success status indicator
    await broadcastPanel.getByText(/Sukses terkirim ke 3 penerima/).waitFor({ state: 'visible', timeout: 10000 })

    // Verify sent payload captured by mock
    assert.ok(lastBroadcastSendPayload, 'broadcast send payload must be captured')
    assert.equal(lastBroadcastSendPayload.subject, 'Pembaruan Fitur Terkini & Komunitas')
    assert.ok(lastBroadcastSendPayload.idempotency_key, 'idempotency_key must be present')
    assert.ok(lastBroadcastSendPayload.tags.includes('backer'), 'tags must include backer')

    // Capture post-send screenshot showing success state
    const sentScreenshotPath = path.join(
      outputDir,
      `broadcast-sent-${isMobile ? 'mobile-390' : 'desktop-1440'}.png`,
    )
    await page.screenshot({ path: sentScreenshotPath, fullPage: false })
    console.log(`Saved screenshot: ${sentScreenshotPath}`)

    assert.equal(errors.length, 0, `Page errors encountered: ${errors.join(', ')}`)
    await context.close()
  }
} finally {
  await browser?.close()
  await server.close()
}
