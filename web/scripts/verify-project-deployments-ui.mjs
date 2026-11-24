import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.CUAN_BROWSER_MODULE || path.join(os.homedir(), '.local/lib/node_modules/playwright'))
const output = process.argv[2] || process.env.CUAN_TASK_ARTIFACTS_DIR || '/Users/buchin/.gemini/antigravity-cli/brain/52f9ef52-0956-41b2-a934-09d82ceeae0d'
await fs.mkdir(output, { recursive: true, mode: 0o700 })

const sampleProject = {
  id: 'proj_4978b0b914eaaf0d588df037977fb652',
  name: 'cuan',
  repo_url: 'https://github.com/buchin/cuan.git',
  default_branch: 'dev',
  project_type: 'custom',
  created_at: new Date(Date.now() - 3600000 * 24 * 7).toISOString(),
  updated_at: new Date().toISOString(),
  is_archived: false,
}

const sampleDeployments = [
  {
    id: 'dep_00',
    project_id: sampleProject.id,
    commit_sha: 'a1b2c3d4e5f67890abcdef1234567890abcdef12',
    short_sha: 'a1b2c3d',
    commit_message: 'feat(deploy): add dedicated deployment lane and webhook ingestion',
    branch: 'dev',
    status: 'queued',
    trigger_source: 'webhook',
    trigger_author: 'github-actions',
    duration_seconds: 0,
    duration: '',
    started_at: '',
    completed_at: '',
    created_at: new Date(Date.now() - 60000).toISOString(),
    relative_time: '1 minute ago',
    steps: []
  },
  {
    id: 'dep_01',
    project_id: sampleProject.id,
    commit_sha: 'd381fe0123456789abcdef1234567890abcdef12',
    short_sha: 'd381fe0',
    commit_message: 'Refactor user profile update function',
    branch: 'main',
    status: 'success',
    trigger_source: 'git_push',
    trigger_author: 'Joe Dixon',
    duration_seconds: 42,
    duration: '42s',
    started_at: new Date(Date.now() - 3600000 * 24 * 18).toISOString(),
    completed_at: new Date(Date.now() - 3600000 * 24 * 18 + 42000).toISOString(),
    created_at: new Date(Date.now() - 3600000 * 24 * 18).toISOString(),
    relative_time: '18 days ago',
    steps: [
      {
        id: 'step-1',
        name: 'Source Checkout',
        status: 'success',
        duration: '3s',
        summary: 'Branch main fetched and checked out at d381fe0',
        log: 'git checkout main\nHEAD is now at d381fe0 Refactor user profile update function\nWorking tree clean.'
      },
      {
        id: 'step-2',
        name: 'Compile & Build',
        status: 'success',
        duration: '24s',
        summary: 'Build pipeline executed successfully',
        log: 'npm run build\n✓ 1,248 modules transformed\n✓ built in 2.14s\nBuild finished without errors.'
      },
      {
        id: 'step-3',
        name: 'Database Migration',
        status: 'success',
        duration: '8s',
        summary: 'Database schema migrations up to date',
        log: 'Applying database migrations...\n000046_project_deployments: applied\nAll migrations up to date.'
      },
      {
        id: 'step-4',
        name: 'Atomic Service Switch',
        status: 'success',
        duration: '4s',
        summary: 'Symlink updated and service daemon reloaded',
        log: 'Switching release symlink -> current\nReloading systemd service cuan.service...\nDaemon reloaded cleanly.'
      },
      {
        id: 'step-5',
        name: 'Health Verification',
        status: 'success',
        duration: '3s',
        summary: 'Health probe verified HTTP 200 OK',
        log: 'GET /healthz -> HTTP 200 OK (18ms)\nApplication is healthy and responding to traffic.'
      }
    ]
  },
  {
    id: 'dep_02',
    project_id: sampleProject.id,
    commit_sha: '057a77a987654321fedcba0987654321fedcba09',
    short_sha: '057a77a',
    commit_message: 'Fix issue with logout not properly clearing session',
    branch: 'main',
    status: 'success',
    trigger_source: 'git_push',
    trigger_author: 'Joe Dixon',
    duration_seconds: 38,
    duration: '38s',
    started_at: new Date(Date.now() - 3600000 * 24 * 19).toISOString(),
    completed_at: new Date(Date.now() - 3600000 * 24 * 19 + 38000).toISOString(),
    created_at: new Date(Date.now() - 3600000 * 24 * 19).toISOString(),
    relative_time: '19 days ago',
    steps: []
  },
  {
    id: 'dep_03',
    project_id: sampleProject.id,
    commit_sha: '057a77a987654321fedcba0987654321fedcba08',
    short_sha: '057a77a',
    commit_message: 'Add missing translations for Spanish locale',
    branch: 'main',
    status: 'canceled',
    trigger_source: 'git_push',
    trigger_author: 'Joe Dixon',
    duration_seconds: 14,
    duration: '14s',
    started_at: new Date(Date.now() - 3600000 * 24 * 19.1).toISOString(),
    completed_at: new Date(Date.now() - 3600000 * 24 * 19.1 + 14000).toISOString(),
    created_at: new Date(Date.now() - 3600000 * 24 * 19.1).toISOString(),
    relative_time: '19 days ago',
    steps: []
  },
  {
    id: 'dep_04',
    project_id: sampleProject.id,
    commit_sha: '057a77a987654321fedcba0987654321fedcba07',
    short_sha: '057a77a',
    commit_message: 'Improve documentation for API usage',
    branch: 'main',
    status: 'success',
    trigger_source: 'git_push',
    trigger_author: 'Joe Dixon',
    duration_seconds: 40,
    duration: '40s',
    started_at: new Date(Date.now() - 3600000 * 24 * 19.2).toISOString(),
    completed_at: new Date(Date.now() - 3600000 * 24 * 19.2 + 40000).toISOString(),
    created_at: new Date(Date.now() - 3600000 * 24 * 19.2).toISOString(),
    relative_time: '19 days ago',
    steps: []
  }
]

const sampleCommits = [
  {
    sha: 'd381fe0123456789abcdef1234567890abcdef12',
    short_sha: 'd381fe0',
    author_name: 'Joe Dixon',
    author_email: 'joe@laravel.com',
    date: new Date(Date.now() - 3600000 * 24 * 18).toISOString(),
    relative_date: '18 hari lalu',
    message: 'Refactor user profile update function',
  },
  {
    sha: '057a77a987654321fedcba0987654321fedcba09',
    short_sha: '057a77a',
    author_name: 'Joe Dixon',
    author_email: 'joe@laravel.com',
    date: new Date(Date.now() - 3600000 * 24 * 19).toISOString(),
    relative_date: '19 hari lalu',
    message: 'Fix issue with logout not properly clearing session',
  }
]

const entry = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter} from 'react-router-dom';
import {LanguageProvider} from '/src/context/LanguageContext.tsx';
import {AuthProvider} from '/src/context/AuthContext.tsx';
import {AppShell} from '/src/components/layout/AppShell.tsx';
import {ProjectsWorkspace} from '/src/features/projects/ProjectsWorkspace.tsx';
import '/src/index.css';

const project = ${JSON.stringify(sampleProject)};
const projects = [project];
const noop = () => {};

function AppFixture() {
  return React.createElement(AppShell, {
    currentTab: 'projects',
    onSelectTab: noop,
  }, React.createElement(ProjectsWorkspace, {
    projects,
    project,
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
  server: { host: '127.0.0.1', port: 0 },
  plugins: [{
    name: 'project-deployments-fixture',
    enforce: 'pre',
    resolveId(id) {
      if (id === '/__deployments_entry.jsx') return '\0deployments-fixture-entry.jsx'
    },
    load(id) {
      if (id === '\0deployments-fixture-entry.jsx') return entry
    },
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (req.url.includes('/api/v1/projects/') && req.url.includes('/deployments')) {
          res.setHeader('Content-Type', 'application/json')
          res.end(JSON.stringify({
            project_id: sampleProject.id,
            project_name: sampleProject.name,
            branch: 'dev',
            domain: 'https://cuan.work',
            webhook_url: 'https://cuan.work/api/v1/projects/' + sampleProject.id + '/webhooks/deploy',
            webhook_secret: 'whsec_8f93e7104b2c15a894762e49c7160352',
            deployments: sampleDeployments,
            total: sampleDeployments.length,
            live_sha: sampleDeployments[1].commit_sha
          }))
          return
        }
        if (req.url.includes('/api/v1/projects/') && req.url.includes('/git-log')) {
          res.setHeader('Content-Type', 'application/json')
          res.end(JSON.stringify({
            project_id: sampleProject.id,
            branch: 'main',
            commits: sampleCommits,
            total: sampleCommits.length,
          }))
          return
        }
        if (req.url.startsWith('/projects')) {
          res.setHeader('Content-Type', 'text/html')
          res.end(await server.transformIndexHtml(req.url, '<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body class="dark bg-background text-foreground"><div id="root"></div><script type="module" src="/__deployments_entry.jsx"></script></body></html>'))
          return
        }
        next()
      })
    }
  }]
})

await server.listen()
const origin = server.resolvedUrls.local[0]
let browser

try {
  browser = await chromium.launch({
    headless: true,
    executablePath: process.env.CUAN_CHROMIUM_EXECUTABLE,
    args: process.env.CUAN_BROWSER_NO_SANDBOX === '1' ? ['--no-sandbox'] : []
  })

  // 1. Desktop 1440px - Deployments List View
  const contextDesktop = await browser.newContext({
    viewport: { width: 1440, height: 900 },
  })
  await contextDesktop.addInitScript(() => {
    localStorage.setItem('cuan-language', 'id')
    localStorage.setItem('cuan-theme', 'dark')
    localStorage.setItem('cuan_token', 'isolated-fixture')
  })
  const pageDesktop = await contextDesktop.newPage()

  await pageDesktop.goto(new URL('/projects/' + sampleProject.id + '/deployments', origin).href, { waitUntil: 'domcontentloaded' })
  await pageDesktop.waitForSelector('#deployments-trigger-btn', { timeout: 8000 })
  await pageDesktop.waitForSelector('#deployments-visit-btn', { timeout: 8000 })
  await pageDesktop.waitForTimeout(600)

  // Verify list row elements
  const rowCount = await pageDesktop.locator('.group').count()
  assert.ok(rowCount >= 4, `expected at least 4 deployment rows, got ${rowCount}`)

  // Capture Desktop Deployments List (with queued lane item & webhook badge)
  const listShotPath = path.join(output, 'project-deployments-desktop-1440.png')
  await pageDesktop.screenshot({ path: listShotPath, fullPage: true })
  console.log(`Saved desktop deployments list screenshot to: ${listShotPath}`)

  // 2. Open Webhook SlideDrawer
  await pageDesktop.locator('#deployments-webhook-btn').click()
  await pageDesktop.waitForSelector('[data-slide-drawer], dialog[open]', { timeout: 8000 })
  await pageDesktop.waitForTimeout(600)

  // Verify webhook inputs are populated
  const webhookUrlInput = pageDesktop.locator('input[value*="webhooks/deploy"]')
  assert.ok(await webhookUrlInput.isVisible(), 'Webhook payload URL input should be visible')

  const webhookShotPath = path.join(output, 'project-deployments-webhook-drawer.png')
  await pageDesktop.screenshot({ path: webhookShotPath, fullPage: true })
  console.log(`Saved webhook drawer screenshot to: ${webhookShotPath}`)

  // Close webhook drawer
  await pageDesktop.keyboard.press('Escape')
  await pageDesktop.waitForTimeout(400)

  // 3. Click deployment row with steps to open SlideDrawer Timeline
  await pageDesktop.locator('[data-deployment-row="true"]').nth(1).click()
  await pageDesktop.waitForSelector('[data-slide-drawer], dialog[open]', { timeout: 8000 })
  await pageDesktop.waitForTimeout(600)

  // Verify continuous vertical connecting line exists in standard AccordionTimeline
  const connector = pageDesktop.locator('[data-timeline-connector]')
  await connector.first().waitFor({ state: 'attached', timeout: 6000 })
  assert.ok(await connector.count() > 0, 'continuous vertical connecting line should be present in AccordionTimeline')

  // Capture SlideDrawer with continuous vertical timeline
  const drawerShotPath = path.join(output, 'project-deployments-drawer-timeline.png')
  await pageDesktop.screenshot({ path: drawerShotPath, fullPage: true })
  console.log(`Saved timeline drawer screenshot to: ${drawerShotPath}`)

  // Close drawer
  await pageDesktop.keyboard.press('Escape')
  await pageDesktop.waitForTimeout(400)

  // 3. Mobile 390px - Responsive Deployments View
  const contextMobile = await browser.newContext({
    viewport: { width: 390, height: 844 },
    hasTouch: true,
  })
  await contextMobile.addInitScript(() => {
    localStorage.setItem('cuan-language', 'id')
    localStorage.setItem('cuan-theme', 'dark')
    localStorage.setItem('cuan_token', 'isolated-fixture')
  })
  const pageMobile = await contextMobile.newPage()
  await pageMobile.goto(new URL('/projects/' + sampleProject.id + '/deployments', origin).href, { waitUntil: 'domcontentloaded' })
  await pageMobile.waitForSelector('#deployments-trigger-btn', { timeout: 8000 })
  await pageMobile.waitForTimeout(600)

  const mobileShotPath = path.join(output, 'project-deployments-mobile-390.png')
  await pageMobile.screenshot({ path: mobileShotPath, fullPage: true })
  console.log(`Saved mobile deployments screenshot to: ${mobileShotPath}`)

  // 4. Verify Activity tab with Live badge
  await pageDesktop.goto(new URL('/projects/' + sampleProject.id + '/tasks', origin).href, { waitUntil: 'domcontentloaded' })
  await pageDesktop.waitForSelector('.group', { timeout: 8000 })
  await pageDesktop.waitForTimeout(600)

  // Check if "Live" badge is rendered
  const liveBadge = pageDesktop.locator('span:has-text("Live")')
  assert.ok(await liveBadge.first().isVisible(), 'Live badge should be visible on deployed commit in activity log')

  const liveBadgeShotPath = path.join(output, 'project-activity-live-badge.png')
  await pageDesktop.screenshot({ path: liveBadgeShotPath, fullPage: true })
  console.log(`Saved activity live badge screenshot to: ${liveBadgeShotPath}`)

} finally {
  if (browser) await browser.close()
  await server.close()
}
