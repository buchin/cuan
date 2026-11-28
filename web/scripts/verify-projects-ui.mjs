import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.CUAN_BROWSER_MODULE || path.join(os.homedir(), '.local/lib/node_modules/playwright'))
const output = process.argv[2] || process.env.CUAN_TASK_ARTIFACTS_DIR
if (!output) throw Error('Supply an output directory')
await fs.mkdir(output, { recursive: true, mode: 0o700 })

const projects = [
  {
    id: 'proj_1',
    name: 'barukopi.id',
    repo_url: '',
    project_type: 'custom',
    task_progress: { total: 0, done: 0 },
    created_at: new Date(Date.now() - 3600000 * 3).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 3).toISOString(),
    is_archived: false,
  },
  {
    id: 'proj_2',
    name: 'cuan-landing',
    repo_url: 'https://github.com/buchin/cuan-landing',
    project_type: 'custom',
    task_progress: { total: 1, done: 1 },
    created_at: new Date(Date.now() - 3600000 * 2).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 2).toISOString(),
    is_archived: false,
  },
  {
    id: 'proj_3',
    name: 'reuploadr',
    repo_url: 'https://github.com/buchin/reuploadr',
    project_type: 'custom',
    task_progress: { total: 2, done: 2 },
    created_at: new Date(Date.now() - 3600000 * 2).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 2).toISOString(),
    is_archived: false,
  },
  {
    id: 'proj_4',
    name: 'laravel-mcp-bridge',
    repo_url: 'https://github.com/buchin/laravel-mcp-bridge',
    project_type: 'custom',
    task_progress: { total: 0, done: 0 },
    created_at: new Date(Date.now() - 3600000 * 4).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 4).toISOString(),
    is_archived: false,
  },
  {
    id: 'proj_5',
    name: 'dracin',
    repo_url: 'https://github.com/buchin/dracin',
    project_type: 'custom',
    task_progress: { total: 3, done: 3 },
    created_at: new Date(Date.now() - 3600000 * 5).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 5).toISOString(),
    is_archived: false,
  },
  {
    id: 'proj_6',
    name: 'nerd',
    repo_url: 'https://github.com/buchin/nerd',
    project_type: 'custom',
    task_progress: { total: 1, done: 1 },
    created_at: new Date(Date.now() - 3600000 * 6).toISOString(),
    updated_at: new Date(Date.now() - 3600000 * 6).toISOString(),
    is_archived: false,
  },
  {
    id: 'proj_7',
    name: 'dojo',
    repo_url: 'https://github.com/buchin/dojo',
    project_type: 'custom',
    task_progress: { total: 5, done: 2, active: 1 },
    created_at: new Date(Date.now() - 3600000 * 8).toISOString(),
    updated_at: new Date(Date.now() - 1800000).toISOString(),
    metadata: { cuan_settings: { goal: 'Dojo platform for digital goods, member...' } },
    is_archived: false,
  },
]

const entry = `import React from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter} from 'react-router-dom';
import {LanguageProvider} from '/src/context/LanguageContext.tsx';
import {AuthProvider} from '/src/context/AuthContext.tsx';
import {AppShell} from '/src/components/layout/AppShell.tsx';
import {ProjectsWorkspace} from '/src/features/projects/ProjectsWorkspace.tsx';
import '/src/index.css';

const projects = ${JSON.stringify(projects)};
const noop = () => {};

function AppFixture() {
  return React.createElement(AppShell, {
    currentTab: 'projects',
    onSelectTab: noop,
  }, React.createElement(ProjectsWorkspace, {
    projects,
    loading: false,
    error: '',
    onRetry: noop,
  }));
}

createRoot(document.getElementById('root')).render(
  React.createElement(BrowserRouter, null,
    React.createElement(LanguageProvider, null,
      React.createElement(AuthProvider, null,
        React.createElement(AppFixture)
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
  cacheDir: path.join(path.resolve(output), 'vite-projects-cache'),
  server: { host: '127.0.0.1', port: 0 },
  plugins: [{
    name: 'projects-fixture',
    enforce: 'pre',
    resolveId(id) {
      if (id === '/__projects_entry.jsx') return '\0projects-fixture-entry.jsx'
    },
    load(id) {
      if (id === '\0projects-fixture-entry.jsx') return entry
    },
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (req.url.startsWith('/projects')) {
          res.setHeader('Content-Type', 'text/html')
          res.end(await server.transformIndexHtml(req.url, '<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body class="dark"><div id="root"></div><script type="module" src="/__projects_entry.jsx"></script></body></html>'))
        } else next()
      })
    }
  }]
})

await server.listen()
const origin = server.resolvedUrls.local[0]
let browser

try {
  for (const width of [390, 1440]) {
    browser = await chromium.launch({
      headless: true,
      executablePath: process.env.CUAN_CHROMIUM_EXECUTABLE,
      args: process.env.CUAN_BROWSER_NO_SANDBOX === '1' ? ['--no-sandbox'] : []
    })
    const context = await browser.newContext({
      viewport: { width, height: width === 390 ? 844 : 900 },
      hasTouch: width === 390,
    })
    await context.addInitScript(() => {
      localStorage.setItem('cuan-language', 'id')
      localStorage.setItem('cuan-theme', 'dark')
      localStorage.setItem('cuan_token', 'isolated-fixture')
    })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', err => errors.push(err.stack || err.message))

    await page.goto(new URL('/projects', origin).href, { waitUntil: 'domcontentloaded' })
    await page.waitForSelector('[data-project-row]', { state: 'visible' })

    const rows = page.locator('[data-project-row]')
    const rowCount = await rows.count()
    assert.equal(rowCount, 7, 'Expected 7 project rows')

    // 1. Verify no repetitive 'GitHub terhubung' or 'GitHub connected'
    const fullText = await page.locator('aside[aria-label="Daftar project"]').innerText()
    assert.ok(!fullText.includes('GitHub terhubung'), 'Must cut repetitive "GitHub terhubung"')
    assert.ok(!fullText.includes('GitHub connected'), 'Must cut repetitive "GitHub connected"')
    assert.ok(!fullText.includes('No repository needed'), 'Must cut repetitive "No repository needed"')

    // 2. Verify alignment: all project title text elements must have the EXACT SAME x position
    const titleXs = []
    for (let i = 0; i < rowCount; i++) {
      const row = rows.nth(i)
      const titleEl = row.locator('.project-list-row [data-list-row-content] span.truncate').first()
      const box = await titleEl.boundingBox()
      assert.ok(box, `Title element box must exist for row ${i}`)
      titleXs.push(Math.round(box.x))
    }
    const firstX = titleXs[0]
    for (let i = 1; i < titleXs.length; i++) {
      assert.equal(
        titleXs[i],
        firstX,
        `Row ${i} title X (${titleXs[i]}) does not match Row 0 title X (${firstX}). Names must be perfectly aligned straight left!`
      )
    }

    // 3. Verify consistent second line: concise Indonesian relative timestamp without "Diperbarui" prefix or truncation
    for (let i = 0; i < rowCount; i++) {
      const row = rows.nth(i)
      const descEl = row.locator('.project-list-row [data-list-row-content] span.text-xs.text-muted-foreground').first()
      const descText = await descEl.innerText()
      console.log(`Row ${i} description: "${descText}"`)
      assert.ok(
        descText === 'Baru saja' || /\d+\s+(mnt|jam|hari)\s+lalu$/.test(descText),
        `Row ${i} description ("${descText}") must be a concise Indonesian relative timestamp`
      )
    }

    // 4. Verify ProjectIcon: no heavy bg-muted class
    const icons = page.locator('.project-list-row [role="img"]')
    for (let i = 0; i < rowCount; i++) {
      const classAttr = await icons.nth(i).getAttribute('class')
      assert.ok(!classAttr.includes('bg-muted'), `Icon at row ${i} must not have heavy "bg-muted"`)
    }

    // Capture screenshot
    const screenshotName = width === 390 ? 'projects-mobile-390.png' : 'projects-desktop-1440.png'
    const screenshotPath = path.join(output, screenshotName)
    await page.screenshot({ path: screenshotPath })
    console.log(`Saved screenshot: ${screenshotPath}`)

    await context.close()
    await browser.close()
    browser = null
  }
  console.log('PASS: All 4 project list DoD criteria verified successfully!')
} finally {
  if (browser) await browser.close()
  await server.close()
}
