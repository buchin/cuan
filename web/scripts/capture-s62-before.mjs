import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')
const screenshotsDir = path.join(repoRoot, 'screenshots')

if (!fs.existsSync(screenshotsDir)) {
  fs.mkdirSync(screenshotsDir, { recursive: true })
}

const require = createRequire(import.meta.url)
const playwrightPath = path.join(process.env.HOME || '/home/dev', '.local/lib/node_modules/playwright')
const { chromium } = require(playwrightPath)

// Find compiled CSS
const distAssetsDir = path.join(webDir, 'dist/assets')
const cssFiles = fs.readdirSync(distAssetsDir).filter((f) => f.endsWith('.css'))
const mainCssFile = cssFiles.find((f) => f.startsWith('style-')) || cssFiles[0]
const compiledCss = fs.readFileSync(path.join(distAssetsDir, mainCssFile), 'utf8')
const chatCalmCss = fs.readFileSync(path.join(webDir, 'src/features/inbox/chat-calm.css'), 'utf8')

const beforeHtml = `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 62 — Before Mobile Navigation & Duplicated Schedules</title>
  <style>
    ${compiledCss}
    ${chatCalmCss}
  </style>
  <style>
    body {
      background-color: #141416 !important;
      color: #ededed;
      margin: 0;
      padding: 0;
      font-family: 'Geist Variable', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      -webkit-font-smoothing: antialiased;
      overflow-x: hidden;
    }
  </style>
</head>
<body class="dark">
  <div class="app-shell flex h-screen w-full flex-col overflow-hidden bg-background">
    <!-- Mobile Header -->
    <header class="app-page-header flex h-14 shrink-0 items-center justify-between border-b border-border bg-background px-3">
      <div class="flex items-center gap-2">
        <div class="h-6 w-6 rounded bg-primary flex items-center justify-center font-bold text-xs text-primary-foreground">C</div>
        <h1 class="page-title text-base font-semibold tracking-tight">Chat</h1>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-muted-foreground font-mono bg-muted/40 px-2 py-0.5 rounded">Before S62</span>
      </div>
    </header>

    <!-- Main Content Area: Inbox list -->
    <main class="flex-1 overflow-y-auto divide-y divide-border/30 pb-16">
      <div class="p-3 bg-muted/10 border-b border-border/20 text-xs text-muted-foreground">
        ⚠️ <strong>State Sebelum S62:</strong> Bottom nav memiliki <em>Projects</em> (Pillar 3), sedangkan <em>Contacts</em> tersembunyi jauh di dalam drawer Menu. Selain itu, menu <em>Schedules</em> mengalami duplikasi di sidebar dan drawer.
      </div>
      <div class="p-3.5 space-y-1">
        <div class="flex items-center justify-between">
          <span class="text-sm font-semibold text-foreground">Scrum Master (@sm)</span>
          <span class="text-[11px] font-mono text-muted-foreground">11:02</span>
        </div>
        <div class="text-xs font-medium text-foreground">S62-T1 Mobile Nav Overhaul</div>
        <p class="text-xs text-zinc-400 line-clamp-2">Perombakan 4 pilar navigasi mobile dan eliminasi duplikasi Schedules.</p>
      </div>
      <div class="p-3.5 space-y-1">
        <div class="flex items-center justify-between">
          <span class="text-sm font-semibold text-foreground">Hermes (@hermes)</span>
          <span class="text-[11px] font-mono text-muted-foreground">09:15</span>
        </div>
        <div class="text-xs font-medium text-foreground">Customer Support</div>
        <p class="text-xs text-zinc-400 line-clamp-2">Pelanggan menanyakan kontak tim dan otomasi jadwal rutinitas.</p>
      </div>
    </main>

    <!-- BEFORE: 4-Pillar Bottom Navigation with Projects instead of Contacts -->
    <nav aria-label="Bottom navigation" class="bottom-navigation fixed bottom-0 left-0 right-0 z-30 flex items-center border-t border-border bg-sidebar md:hidden">
      <!-- Pillar 1: Chat -->
      <button class="flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs font-semibold text-primary">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
        <span>Chat</span>
      </button>

      <!-- Pillar 2: Tasks -->
      <button class="flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs text-muted-foreground">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><polyline points="9 11 12 14 22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>
        <span>Tasks</span>
      </button>

      <!-- Pillar 3 (OLD): Projects (Contacts is missing from bottom nav!) -->
      <button class="flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs text-muted-foreground">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><path d="M9 20H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H20a2 2 0 0 1 2 2v1"/><circle cx="12" cy="13" r="2"/><path d="M14 13h3"/><circle cx="19" cy="13" r="2"/><path d="m14 13-3 5"/></svg>
        <span>Projects</span>
      </button>

      <!-- Pillar 4: Menu -->
      <button class="flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs text-muted-foreground">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="18" y2="18"/></svg>
        <span>Menu</span>
      </button>
    </nav>
  </div>
</body>
</html>`

async function capture() {
  console.log('Launching Playwright Chromium for Before screenshot...')
  const chromiumExecutable = path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium')
  const browser = await chromium.launch({
    executablePath: chromiumExecutable,
    env: {
      ...process.env,
      LD_LIBRARY_PATH: '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' + (process.env.LD_LIBRARY_PATH || '')
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  })

  try {
    const page = await browser.newPage({
      viewport: { width: 390, height: 844 }, // Mobile standard iPhone 12/13/14
      deviceScaleFactor: 2,
    })

    await page.setContent(beforeHtml, { waitUntil: 'load' })
    await page.waitForTimeout(500)

    const outputPath = path.join(screenshotsDir, 's62-t1-mobile-nav-before.png')
    await page.screenshot({ path: outputPath, fullPage: false })
    console.log(`Saved Before screenshot: ${outputPath}`)
  } finally {
    await browser.close()
  }
}

capture().catch((err) => {
  console.error('Error capturing screenshot:', err)
  process.exit(1)
})
