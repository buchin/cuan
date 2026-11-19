// Capture StudioProjectSelector in action on desktop and mobile (light & dark mode)
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.CUAN_BROWSER_MODULE || path.join(os.homedir(), '.local/lib/node_modules/playwright'))

const webRoot = path.resolve('web')
process.chdir(webRoot)

const outputDir = path.resolve(process.env.CUAN_TASK_ARTIFACTS_DIR || process.argv[2] || 'artifacts')
await fs.mkdir(outputDir, { recursive: true, mode: 0o700 })

const projects = [
  {
    id: 'proj-cuan',
    name: 'cuan',
    default_branch: 'main',
    git_remote: 'github.com/cuan/cuan',
    clone_status: 'synced',
    task_progress: { total: 12, completed: 8 },
  },
  {
    id: 'proj-cuan-landing',
    name: 'cuan-landing',
    default_branch: 'main',
    git_remote: 'github.com/cuan/cuan-landing',
    clone_status: 'synced',
    task_progress: { total: 4, completed: 4 },
  },
  {
    id: 'proj-cuan-docs',
    name: 'cuan-docs',
    default_branch: 'main',
    git_remote: 'github.com/cuan/cuan-docs',
    clone_status: 'connected',
    task_progress: { total: 1, completed: 0 },
  },
  {
    id: 'default',
    name: 'Default',
    default_branch: 'main',
    is_archived: true,
  },
]

const agents = [
  {
    id: 'agent_cuan',
    name: '@cuan',
    status: 'idle',
    enabled: true,
    harness: 'codex',
    provider: 'codex',
    supports_vision: true,
  },
]

const entry = `import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { ThemeProvider } from '/src/context/ThemeContext.tsx';
import { LanguageProvider } from '/src/context/LanguageContext.tsx';
import { AuthProvider } from '/src/context/AuthContext.tsx';
import { AppShell } from '/src/components/layout/AppShell.tsx';
import { StudioView } from '/src/features/inbox/StudioView.tsx';
import '/src/index.css';

const projects = ${JSON.stringify(projects)};
const agents = ${JSON.stringify(agents)};
const noop = () => {};

function App() {
  return React.createElement(
    BrowserRouter,
    null,
    React.createElement(
      ThemeProvider,
      null,
      React.createElement(
        LanguageProvider,
        null,
        React.createElement(
          AuthProvider,
          null,
          React.createElement(
            'div',
            { className: 'flex h-screen w-full flex-col bg-background text-foreground' },
            React.createElement(
              AppShell,
              { currentTab: 'studio', onSelectTab: noop },
              React.createElement(
                'div',
                { className: 'flex-1 overflow-y-auto' },
                React.createElement(StudioView, {
                  onAccepted: noop,
                  onRetrySetup: noop,
                  onCreateProject: noop,
                  checkRuntime: false,
                  autoFocus: false,
                  projects: projects,
                  hasProject: true,
                  firstGoal: false,
                })
              )
            )
          )
        )
      )
    )
  );
}

createRoot(document.getElementById('root')).render(React.createElement(App));
`

const server = await createServer({
  root: webRoot,
  server: { host: '127.0.0.1', port: 0 },
  plugins: [
    {
      name: 'studio-fixture',
      resolveId(id) {
        if (id === '/__studio_entry.js') return '\0studio-fixture-entry.js'
      },
      load(id) {
        if (id === '\0studio-fixture-entry.js') return entry
      },
      configureServer(s) {
        s.middlewares.use(async (req, res, next) => {
          if (req.url === '/' || req.url === '/studio') {
            res.setHeader('Content-Type', 'text/html')
            res.end(
              await s.transformIndexHtml(
                req.url,
                '<!DOCTYPE html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"></head><body class="bg-background text-foreground"><div id="root"></div><script type="module" src="/__studio_entry.js"></script></body></html>'
              )
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
const port = server.httpServer.address().port
const baseUrl = `http://127.0.0.1:${port}/studio`
console.log(`Fixture server listening at ${baseUrl}`)

const browser = await chromium.launch({ headless: true })

try {
  // Test desktop (1280x800)
  for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      colorScheme: theme,
    })
    await context.addInitScript(({ th }) => {
      localStorage.setItem('cuan-language', 'id')
      localStorage.setItem('cuan_token', 'mock-token')
      localStorage.setItem('cuan-theme', th)
      if (th === 'dark') {
        document.documentElement.classList.add('dark')
      } else {
        document.documentElement.classList.remove('dark')
      }
    }, { th: theme })

    const page = await context.newPage()
    page.on('console', (msg) => {
      if (msg.type() === 'error') console.log('Page error:', msg.text())
    })

    // Mock backend API requests ONLY
    await page.route('**/api/v1/**', async (route) => {
      const url = new URL(route.request().url())
      if (url.pathname.endsWith('/auth/status')) {
        return route.fulfill({ json: { setup_required: false } })
      }
      if (url.pathname.endsWith('/auth/me')) {
        return route.fulfill({
          json: { user: { id: 'user-1', name: 'Buchin', role: 'owner' } },
        })
      }
      if (url.pathname.endsWith('/projects')) {
        return route.fulfill({ json: projects })
      }
      if (url.pathname.endsWith('/agents')) {
        return route.fulfill({ json: agents })
      }
      return route.fulfill({ json: {} })
    })

    await page.goto(baseUrl, { waitUntil: 'networkidle' })
    await page.waitForTimeout(600)

    // Capture initial closed state
    await page.screenshot({
      path: path.join(outputDir, `studio-01-closed-${theme}-desktop.png`),
      fullPage: false,
    })
    console.log(`Captured studio-01-closed-${theme}-desktop.png`)

    // Click project selector pill to open dropdown
    const pill = page.locator('#studio-project-selector-pill')
    await pill.waitFor({ state: 'visible' })
    await pill.click()
    await page.waitForTimeout(300)

    // Assert that Default is NOT rendered and no duplicate plus icon in connect label
    const defaultOptionCount = await page.locator('button[role="option"]:has-text("Default")').count()
    assert.equal(defaultOptionCount, 0, 'Dropdown must NOT render Default project')
    const cuanOptionCount = await page.locator('button[role="option"]:has-text("cuan")').count()
    assert.ok(cuanOptionCount > 0, 'Dropdown must render registered cuan project')

    const connectButton = page.locator('div[role="listbox"] a[href="/projects"], div[role="listbox"] button')
    const connectText = (await connectButton.first().textContent() || '').trim()
    assert.ok(
      !connectText.startsWith('+'),
      `Connect button label must not start with duplicate +: "${connectText}"`
    )

    // Capture dropdown open state
    await page.screenshot({
      path: path.join(outputDir, `studio-02-open-${theme}-desktop.png`),
      fullPage: false,
    })
    console.log(`Captured studio-02-open-${theme}-desktop.png`)

    // In light theme, also test clicking another project checkbox (multi-select)
    if (theme === 'light') {
      const landingOption = page.locator('button[role="option"]:has-text("cuan-landing")')
      await landingOption.click()
      await page.waitForTimeout(300)

      await page.screenshot({
        path: path.join(outputDir, 'studio-03-multiselect-light-desktop.png'),
        fullPage: false,
      })
      console.log('Captured studio-03-multiselect-light-desktop.png')
    }

    await context.close()
  }

  // Test mobile (390x844)
  for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({
      viewport: { width: 390, height: 844 },
      hasTouch: true,
      colorScheme: theme,
    })
    await context.addInitScript(({ th }) => {
      localStorage.setItem('cuan-language', 'id')
      localStorage.setItem('cuan_token', 'mock-token')
      localStorage.setItem('cuan-theme', th)
      if (th === 'dark') {
        document.documentElement.classList.add('dark')
      } else {
        document.documentElement.classList.remove('dark')
      }
    }, { th: theme })

    const page = await context.newPage()
    await page.route('**/api/v1/**', async (route) => {
      const url = new URL(route.request().url())
      if (url.pathname.endsWith('/auth/status')) {
        return route.fulfill({ json: { setup_required: false } })
      }
      if (url.pathname.endsWith('/auth/me')) {
        return route.fulfill({
          json: { user: { id: 'user-1', name: 'Buchin', role: 'owner' } },
        })
      }
      if (url.pathname.endsWith('/projects')) {
        return route.fulfill({ json: projects })
      }
      if (url.pathname.endsWith('/agents')) {
        return route.fulfill({ json: agents })
      }
      return route.fulfill({ json: {} })
    })

    await page.goto(baseUrl, { waitUntil: 'networkidle' })
    await page.waitForTimeout(600)

    // Capture mobile closed
    await page.screenshot({
      path: path.join(outputDir, `studio-04-closed-${theme}-mobile.png`),
      fullPage: false,
    })
    console.log(`Captured studio-04-closed-${theme}-mobile.png`)

    // Click project selector pill to open dropdown
    const pill = page.locator('#studio-project-selector-pill')
    await pill.waitFor({ state: 'visible' })
    await pill.click()
    await page.waitForTimeout(300)

    // Assert that Default is NOT rendered and no duplicate plus icon in connect label
    const defaultOptionCount = await page.locator('button[role="option"]:has-text("Default")').count()
    assert.equal(defaultOptionCount, 0, 'Mobile dropdown must NOT render Default project')
    const cuanOptionCount = await page.locator('button[role="option"]:has-text("cuan")').count()
    assert.ok(cuanOptionCount > 0, 'Mobile dropdown must render registered cuan project')

    const connectButton = page.locator('div[role="listbox"] a[href="/projects"], div[role="listbox"] button')
    const connectText = (await connectButton.first().textContent() || '').trim()
    assert.ok(
      !connectText.startsWith('+'),
      `Mobile connect button label must not start with duplicate +: "${connectText}"`
    )

    // Capture mobile dropdown open
    await page.screenshot({
      path: path.join(outputDir, `studio-05-open-${theme}-mobile.png`),
      fullPage: false,
    })
    console.log(`Captured studio-05-open-${theme}-mobile.png`)

    await context.close()
  }
} finally {
  await browser.close()
  await server.close()
  console.log('Verification captures completed successfully.')
}
