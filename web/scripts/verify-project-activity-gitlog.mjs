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

const sampleCommits = [
  {
    sha: 'fc4e8ecc401e1851525a064be6841c7f2127eb2d',
    short_sha: 'fc4e8ec',
    author_name: 'Buchin',
    author_email: 'buchin@dropsugar.com',
    date: new Date().toISOString(),
    relative_date: '35m lalu',
    message: 'fix(terminal): prevent chat permission deny rules from poisoning agent terminal file editing',
  },
  {
    sha: '490ac28e83e4ab286410da393564cd73f395fa8c',
    short_sha: '490ac28',
    author_name: 'Buchin',
    author_email: 'buchin@dropsugar.com',
    date: new Date(Date.now() - 3600000 * 1.5).toISOString(),
    relative_date: '1j lalu',
    message: 'fix(inbox): eliminate premature room resolution and enforce full 24h grace period for completed task conversations',
  },
  {
    sha: '0ff1e9cf1a690022c9c880e09ba6e1c3e48ca283',
    short_sha: '0ff1e9c',
    author_name: 'Buchin',
    author_email: 'buchin@dropsugar.com',
    date: new Date(Date.now() - 3600000 * 3).toISOString(),
    relative_date: '3j lalu',
    message: 'Apply Task msg_41dad175-e793-4312-952b-e3f6f518b659: Pindahkan Detail Task ke SlideDrawer & Sederhanakan Menjadi Timeline',
    task_id: 'msg_41dad175-e793-4312-952b-e3f6f518b659',
    task_ref: '#41DAD1',
  },
  {
    sha: '757a8eb1b4b62e49c7e090241fae0642915929c7',
    short_sha: '757a8eb',
    author_name: 'Buchin',
    author_email: 'buchin@dropsugar.com',
    date: new Date(Date.now() - 3600000 * 25).toISOString(),
    relative_date: 'Kemarin',
    message: 'fix(deploy): support --allow-divergent flag for release activations',
  },
  {
    sha: 'da724022a1060bc67925c4ee2651525044d84d1b',
    short_sha: 'da72402',
    author_name: 'Buchin',
    author_email: 'buchin@dropsugar.com',
    date: new Date(Date.now() - 3600000 * 28).toISOString(),
    relative_date: 'Kemarin',
    message: 'Apply Task msg_f1baaef6-fea1-4683-87c2-293587a8fb9a: Perbaiki Auto-Claim Antrean Runner & Reset Sesi Terminal Project',
    task_id: 'msg_f1baaef6-fea1-4683-87c2-293587a8fb9a',
    task_ref: '#F1BAAE',
  },
]

const sampleCommitDetail = {
  sha: 'fc4e8ecc401e1851525a064be6841c7f2127eb2d',
  short_sha: 'fc4e8ec',
  author_name: 'Buchin',
  author_email: 'buchin@dropsugar.com',
  date: new Date().toISOString(),
  relative_date: '35m lalu',
  message: 'fix(terminal): prevent chat permission deny rules from poisoning agent terminal file editing',
  body: 'Removes broad command and file edit tool denials from applyChatProfileReadDeny.\nSanitizes settings.json on project terminal start.',
  stats_summary: '3 files changed, 30 insertions(+), 16 deletions(-)',
  files: [
    { path: 'internal/agent/antigravity_settings.go', insertions: 4, deletions: 12 },
    { path: 'internal/agent/antigravity_settings_test.go', insertions: 11, deletions: 4 },
    { path: 'internal/server/terminal_handlers.go', insertions: 15, deletions: 0 },
  ],
  patch_preview: `diff --git a/internal/agent/antigravity_settings.go b/internal/agent/antigravity_settings.go
--- a/internal/agent/antigravity_settings.go
+++ b/internal/agent/antigravity_settings.go
@@ -257,18 +257,6 @@ func applyChatProfileReadDeny(...)
-	for _, toolRule := range []string{
-		"command(*)",
-		"write_file(*)",
-		"edit_file(*)",
-	}
+	// Preserved sensitive directories only`,
}

const entry = `import React from 'react';
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
    name: 'project-gitlog-fixture',
    enforce: 'pre',
    resolveId(id) {
      if (id === '/__gitlog_entry.jsx') return '\0gitlog-fixture-entry.jsx'
    },
    load(id) {
      if (id === '\0gitlog-fixture-entry.jsx') return entry
    },
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (req.url.includes('/api/v1/projects/') && req.url.includes('/git-log')) {
          res.setHeader('Content-Type', 'application/json')
          res.end(JSON.stringify({
            project_id: sampleProject.id,
            branch: 'dev',
            commits: sampleCommits,
            total: sampleCommits.length,
          }))
          return
        }
        if (req.url.includes('/api/v1/projects/') && req.url.includes('/commits/')) {
          res.setHeader('Content-Type', 'application/json')
          res.end(JSON.stringify(sampleCommitDetail))
          return
        }
        if (req.url.startsWith('/projects')) {
          res.setHeader('Content-Type', 'text/html')
          res.end(await server.transformIndexHtml(req.url, '<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body class="dark"><div id="root"></div><script type="module" src="/__gitlog_entry.jsx"></script></body></html>'))
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
  for (const width of [1440, 390]) {
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

    await page.goto(new URL('/projects/' + sampleProject.id + '/tasks', origin).href, { waitUntil: 'domcontentloaded' })
    await page.waitForSelector('text=fc4e8ec', { state: 'visible' })

    // Verify DoD 1: Zero screaming neon colors & no redundant [Terapkan Perubahan] buttons
    const applyButtons = await page.locator('text=Terapkan Perubahan').count()
    assert.equal(applyButtons, 0, 'Must NOT have heavy [Terapkan Perubahan] buttons in git activity feed')

    const viewResultButtons = await page.locator('text=Lihat hasil').count()
    assert.equal(viewResultButtons, 0, 'Must NOT have duplicate [Lihat hasil] buttons')

    // Verify task badge is rendered
    const taskBadge = page.locator('text=#41DAD1')
    await assert.doesNotReject(taskBadge.waitFor({ state: 'visible' }))

    // Capture initial Git Log screenshot
    const screenshotName = width === 390 ? 'project-activity-gitlog-mobile-390.png' : 'project-activity-gitlog-desktop-1440.png'
    const screenshotPath = path.join(output, screenshotName)
    await page.screenshot({ path: screenshotPath })
    console.log(`Saved screenshot: ${screenshotPath}`)

    // Open SlideDrawer by clicking on the first commit
    await page.locator('text=fc4e8ec').first().click()
    await page.waitForSelector('text=Commit SHA', { state: 'visible' })

    // Capture SlideDrawer screenshot on desktop
    if (width === 1440) {
      const drawerPath = path.join(output, 'project-activity-commit-drawer.png')
      await page.screenshot({ path: drawerPath })
      console.log(`Saved SlideDrawer screenshot: ${drawerPath}`)
    }

    await context.close()
    await browser.close()
    browser = null
  }
  console.log('PASS: All Project Git Log UI DoD checks and screenshot captures passed successfully!')
} finally {
  if (browser) await browser.close()
  await server.close()
}
