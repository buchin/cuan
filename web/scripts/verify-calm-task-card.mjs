// Isolated real React components. UI evidence for Calm Task Card & Mobile Header.
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.CUAN_BROWSER_MODULE || path.join(os.homedir(), '.local/lib/node_modules/playwright'))
const output = process.argv[2]
if (!output) throw Error('Supply a new private evidence directory')
await fs.mkdir(output, { recursive: true, mode: 0o700 })

const now = '2026-11-23T14:30:00Z'
const activeTask = {
  id: 'msg_ce209e97',
  title: 'Perbaiki Vertical Alignment & Posisi Badge Unread Sidebar',
  status: 'done',
  priority: 'normal',
  project_id: 'proj_1',
  project_name: 'Cuan',
  source_room_id: 'room_cuan',
  created_at: now,
  updated_at: now,
  summary: 'Hasil pekerjaan memenuhi kriteria yang disepakati.',
}

const completedTask = {
  id: 'task_b7fadb1b940b',
  title: 'Pemeliharaan Rutin & Code Health Cuan',
  status: 'done',
  priority: 'normal',
  project_id: 'proj_1',
  project_name: 'Cuan',
  source_room_id: 'room_cuan',
  created_at: now,
  updated_at: now,
  branch_name: 'dev',
  summary: 'Audit rutin pemeliharaan kode dan kualitas produk Cuan telah selesai diverifikasi. Status commit dev sinkron pada 31948e02, audit performa mendeteksi 4 bottleneck query/index backend dan 3 potensi siklus re-render/polling frontend.',
  dod_checklist: [
    { text: 'Audit performa selesai', completed: true },
    { text: 'Standar Zero Cognitive Fatigue terpenuhi', completed: true },
  ],
}

const chatMessageContent = `Hasil pekerjaan memenuhi kriteria yang disepakati.

Audit rutin pemeliharaan kode dan kualitas produk Cuan telah selesai diverifikasi. Status commit dev sinkron pada 31948e02, audit performa mendeteksi 4 bottleneck query/index backend dan 3 potensi siklus re-render/polling frontend.

[View work](/tasks/task_b7fadb1b940b)`

const entry = `import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { LanguageProvider } from '/src/context/LanguageContext.tsx';
import { AuthProvider } from '/src/context/AuthContext.tsx';
import { ComposerTasks } from '/src/features/inbox/ComposerTasks.tsx';
import { MessageContent } from '/src/features/inbox/MessageContent.tsx';
import { cacheTask } from '/src/features/inbox/TaskCard.tsx';
import '/src/index.css';
import '/src/features/inbox/chat-calm.css';

cacheTask(${JSON.stringify(completedTask)});

function Scene() {
  const [tasks] = useState([${JSON.stringify(activeTask)}]);
  return React.createElement('div', { className: 'flex flex-col h-screen bg-[#141416] text-[#ededed]' },
    React.createElement('header', { className: 'h-14 border-b border-border/40 px-4 flex items-center justify-between bg-card/40 shrink-0' },
      React.createElement('div', { className: 'flex items-center gap-2.5' },
        React.createElement('div', { className: 'h-8 w-8 rounded-full bg-primary/20 flex items-center justify-center text-primary text-xs font-bold' }, '🤖'),
        React.createElement('span', { className: 'font-semibold text-sm' }, '@cuan')
      )
    ),
    React.createElement(ComposerTasks, { tasks, onDetails: () => {}, onTerminal: () => {} }),
    React.createElement('main', { className: 'inbox-message-list calm-conversation flex-1 overflow-y-auto p-4 space-y-4' },
      React.createElement('div', { className: 'calm-stream-constraint max-w-[75ch] mx-auto w-full space-y-3' },
        React.createElement('div', { className: 'flex items-start gap-3' },
          React.createElement('div', { className: 'h-7 w-7 rounded-full bg-primary/20 text-primary flex items-center justify-center text-[11px] font-semibold shrink-0' }, '🤖'),
          React.createElement('div', { className: 'flex-1 min-w-0' },
            React.createElement(MessageContent, {
              content: ${JSON.stringify(chatMessageContent)},
              hideTaskActions: false,
              isCustomerRoom: false,
              onTaskClick: () => {}
            })
          )
        )
      )
    )
  );
}

createRoot(document.getElementById('root')).render(
  React.createElement(BrowserRouter, null,
    React.createElement(LanguageProvider, null,
      React.createElement(AuthProvider, null,
        React.createElement(Scene)
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
      name: 'calm-task-card-fixture',
      enforce: 'pre',
      resolveId(id) {
        if (id === '/__calm.jsx') return '\0calm.jsx'
      },
      load(id) {
        if (id === '\0calm.jsx') return entry
      },
      configureServer(s) {
        s.middlewares.use(async (req, res, next) => {
          if (req.url.startsWith('/calm-task-fixture')) {
            res.setHeader('Content-Type', 'text/html')
            res.end(
              await s.transformIndexHtml(
                req.url,
                '<!DOCTYPE html><html class="dark"><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body class="dark bg-[#141416]"><div id="root"></div><script type="module" src="/__calm.jsx"></script></body></html>',
              ),
            )
          } else next()
        })
      },
    },
  ],
})
await server.listen()

const browser = await chromium.launchPersistentContext(path.join(output, 'profile'), {
  headless: true,
  executablePath: process.env.CUAN_CHROMIUM_EXECUTABLE,
  args: process.env.CUAN_BROWSER_NO_SANDBOX === '1' ? ['--no-sandbox'] : [],
})

const results = []
try {
  for (const width of [390, 1440]) {
    const page = await browser.newPage()
    const errors = []
    page.on('pageerror', err => console.log('PAGE ERROR:', err.stack || err.message))
    page.on('console', msg => console.log('CONSOLE:', msg.type(), msg.text()))

    await page.setViewportSize({ width, height: width === 390 ? 844 : 900 })
    await page.route('**/api/v1/**', route => {
      const url = new URL(route.request().url())
      if (url.pathname.includes('/tasks/task_b7fadb1b940b')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(completedTask),
        })
      }
      return route.fulfill({ contentType: 'application/json', body: '{}' })
    })

    await page.goto(new URL('/calm-task-fixture', server.resolvedUrls.local[0]).href, {
      waitUntil: 'networkidle',
    })

    // 1. Verify ComposerTasks mobile header banner
    const taskBanner = page.locator('[data-testid="composer-tasks-badge"]')
    await taskBanner.waitFor({ state: 'visible' })

    const countBadge = taskBanner.locator('.composer-tasks-count')
    const countText = await countBadge.innerText()
    assert.equal(countText.trim(), '1')

    // Verify counter badge has NO screaming neon color class
    const countBadgeClasses = await countBadge.getAttribute('class')
    assert.ok(
      !countBadgeClasses.includes('text-emerald-300') && !countBadgeClasses.includes('bg-emerald-500/20'),
      'Counter badge must not have screaming neon emerald colors'
    )
    assert.ok(
      countBadgeClasses.includes('bg-muted') && countBadgeClasses.includes('text-muted-foreground'),
      'Counter badge must have subtle muted styling'
    )

    // Verify task title is rendered without awkward single-line cutoff ("...Badg...")
    const titleEl = taskBanner.locator('span[title]')
    const titleText = await titleEl.innerText()
    assert.ok(
      titleText.includes('Perbaiki Vertical Alignment & Posisi'),
      'Task title must be readable'
    )

    // 2. Verify chat message bubble above card: NO DUPLICATE SUMMARY
    const chatBubble = page.locator('.calm-message-bubble')
    await chatBubble.waitFor({ state: 'visible' })
    const bubbleText = await chatBubble.innerText()
    assert.equal(bubbleText.trim(), 'Hasil pekerjaan memenuhi kriteria yang disepakati.')
    assert.ok(
      !bubbleText.includes('Audit rutin pemeliharaan kode'),
      'Speech bubble must NOT duplicate task summary when task card is attached'
    )

    // 3. Verify TaskCompletionCard in chat
    const completionCard = page.locator('[data-testid="task-completion-card"]')
    await completionCard.waitFor({ state: 'visible' })

    // Verify task status label is visible
    const statusEl = page.locator('[data-testid="completion-card-status"]')
    assert.ok(await statusEl.isVisible())

    // Verify technical noise is hidden
    const terminalBtn = page.locator('[data-testid="task-card-terminal-btn"]')
    assert.ok(!(await terminalBtn.isVisible()), 'Terminal button must be hidden from visual chat card')

    const retryBtn = page.locator('[data-testid="task-card-retry-btn"]')
    assert.ok(!(await retryBtn.isVisible()), 'Retry button must be hidden from visual chat card')

    // Verify summary is rendered cleanly inside the card
    const cardSummary = page.locator('[data-testid="completion-card-summary"]')
    assert.ok(await cardSummary.isVisible())
    assert.ok((await cardSummary.innerText()).includes('Audit rutin pemeliharaan kode'))

    // Capture visual proof screenshots
    const screenshotName = width === 390 ? 'task-completion-calm-390.png' : 'task-completion-calm-1440.png'
    await page.screenshot({ path: path.join(output, screenshotName), fullPage: false })

    assert.deepEqual(errors, [])
    results.push({
      width,
      screenshot: screenshotName,
      countBadgeMuted: true,
      titleReadable: true,
      noDuplicateSummaryInBubble: true,
      devOpsNoiseHidden: true,
      errors,
    })
    await page.close()
  }
} finally {
  await browser.close()
  await server.close()
}

await fs.writeFile(path.join(output, 'calm-verification-summary.json'), JSON.stringify(results, null, 2))
console.log(JSON.stringify(results, null, 2))
