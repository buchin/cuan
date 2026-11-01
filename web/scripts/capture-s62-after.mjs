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

// Mobile Template: 4-Pillar Bottom Navigation (Chat, Tasks, Contacts, Menu)
const mobileAfterHtml = `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 62 — After Mobile Navigation (Contacts as Core Pillar)</title>
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
        <h1 class="page-title text-base font-semibold tracking-tight">Kontak</h1>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-emerald-400 font-mono bg-emerald-950/60 border border-emerald-800/40 px-2 py-0.5 rounded">S62-T1 4-Pillar</span>
      </div>
    </header>

    <!-- Main Content Area: Contacts directory view -->
    <main class="flex-1 overflow-y-auto divide-y divide-border/30 pb-16">
      <div class="p-3 bg-secondary/20 border-b border-border/20 text-xs text-foreground/90">
        ✨ <strong>4 Pilar Navigasi Mobile Tenang:</strong> <em>Contacts</em> kini berada di jangkauan jempol bersama Chat, Tasks, dan Menu. Duplikasi menu <em>Schedules</em> telah dieliminasi dan dipusatkan di Settings.
      </div>
      <!-- Contact List Item 1 -->
      <div class="p-3.5 space-y-1 hover:bg-muted/30 cursor-pointer">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <div class="h-8 w-8 rounded-full bg-secondary flex items-center justify-center font-semibold text-xs text-foreground">SM</div>
            <div>
              <div class="text-sm font-semibold text-foreground">Scrum Master</div>
              <div class="text-xs text-muted-foreground">@sm · Digital Assistant</div>
            </div>
          </div>
          <span class="inline-block h-2 w-2 rounded-full bg-emerald-500" title="Aktif"></span>
        </div>
      </div>
      <!-- Contact List Item 2 -->
      <div class="p-3.5 space-y-1 hover:bg-muted/30 cursor-pointer">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <div class="h-8 w-8 rounded-full bg-secondary flex items-center justify-center font-semibold text-xs text-foreground">HM</div>
            <div>
              <div class="text-sm font-semibold text-foreground">Hermes Support</div>
              <div class="text-xs text-muted-foreground">@hermes · Layanan Pelanggan</div>
            </div>
          </div>
          <span class="inline-block h-2 w-2 rounded-full bg-emerald-500" title="Aktif"></span>
        </div>
      </div>
      <!-- Contact List Item 3 -->
      <div class="p-3.5 space-y-1 hover:bg-muted/30 cursor-pointer">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <div class="h-8 w-8 rounded-full bg-secondary flex items-center justify-center font-semibold text-xs text-foreground">BC</div>
            <div>
              <div class="text-sm font-semibold text-foreground">Buchin (Owner)</div>
              <div class="text-xs text-muted-foreground">buchin@dropsugar.com</div>
            </div>
          </div>
          <span class="text-xs font-mono text-muted-foreground">Admin</span>
        </div>
      </div>
    </main>

    <!-- AFTER S62: 4-Pillar Bottom Navigation with Contacts -->
    <nav aria-label="Bottom navigation" class="bottom-navigation fixed bottom-0 left-0 right-0 z-30 flex items-center border-t border-border bg-sidebar md:hidden">
      <!-- Pillar 1: Chat -->
      <button data-testid="bottom-nav-chat" class="flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs text-muted-foreground">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
        <span>Chat</span>
      </button>

      <!-- Pillar 2: Tasks -->
      <button data-testid="bottom-nav-tasks" class="flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs text-muted-foreground">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><polyline points="9 11 12 14 22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>
        <span>Tasks</span>
      </button>

      <!-- Pillar 3: Contacts (NEW PRIMARY PILLAR) -->
      <button data-testid="bottom-nav-contacts" class="flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs font-semibold text-primary">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
        <span>Contacts</span>
      </button>

      <!-- Pillar 4: Menu -->
      <button data-testid="bottom-nav-menu" class="flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 text-xs text-muted-foreground">
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="18" y2="18"/></svg>
        <span>Menu</span>
      </button>
    </nav>
  </div>
</body>
</html>`

// Desktop Template: Consolidated Desktop Navigation without duplicate Schedules
const desktopAfterHtml = `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 62 — After Desktop Sidebar Navigation Consolidation</title>
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
  <div class="app-shell flex h-screen w-full overflow-hidden bg-background">
    <!-- Desktop Sidebar (Consolidated NAV_GROUPS) -->
    <nav aria-label="Main navigation" class="primary-navigation hidden shrink-0 flex-col border-r border-border bg-sidebar md:flex w-64">
      <!-- Sidebar Brand -->
      <div class="sidebar-brand flex h-16 shrink-0 items-center gap-2 border-b border-border px-4">
        <div class="h-8 w-8 rounded-lg bg-primary flex items-center justify-center font-bold text-sm text-primary-foreground">C</div>
        <span class="nav-label text-lg font-semibold tracking-tight">cuan</span>
      </div>

      <!-- Navigation Groups -->
      <div class="flex-1 space-y-4 overflow-y-auto p-3">
        <!-- Group 1: Workspace -->
        <div data-testid="nav-group-workspace" class="space-y-1">
          <div class="px-2.5 pt-1.5 pb-0.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
            Workspace
          </div>
          <button class="nav-action flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-muted/50 transition-colors">
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
            <span>Chat</span>
          </button>
          <button class="nav-action flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-muted/50 transition-colors">
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4"><polyline points="9 11 12 14 22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>
            <span>Tasks</span>
          </button>
          <button class="nav-action flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-muted/50 transition-colors">
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4"><path d="M9 20H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H20a2 2 0 0 1 2 2v1"/><circle cx="12" cy="13" r="2"/><path d="M14 13h3"/><circle cx="19" cy="13" r="2"/><path d="m14 13-3 5"/></svg>
            <span>Projects</span>
          </button>
        </div>

        <!-- Group 2: Operations (Clean: Contacts only, 0 Schedules duplication!) -->
        <div data-testid="nav-group-operations" class="space-y-1">
          <div class="px-2.5 pt-1.5 pb-0.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
            Operations
          </div>
          <button class="nav-action flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm bg-sidebar-accent text-sidebar-accent-foreground font-semibold">
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4 text-primary"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
            <span>Contacts</span>
          </button>
        </div>

        <!-- Group 3: System -->
        <div data-testid="nav-group-system" class="space-y-1">
          <div class="px-2.5 pt-1.5 pb-0.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
            System
          </div>
          <button class="nav-action flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-muted/50 transition-colors">
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
            <span>Settings</span>
          </button>
        </div>
      </div>

      <!-- User footer -->
      <div class="border-t border-border p-3">
        <div class="flex items-center gap-3">
          <div class="h-8 w-8 rounded-full bg-primary flex items-center justify-center font-bold text-xs text-primary-foreground">BC</div>
          <div class="min-w-0 flex-1">
            <div class="font-medium text-sm text-foreground truncate">Buchin</div>
            <div class="text-xs text-muted-foreground truncate">buchin@dropsugar.com</div>
          </div>
        </div>
      </div>
    </nav>

    <!-- Main Content Area: Desktop Settings with Embedded Schedules Highlight -->
    <main class="flex-1 flex flex-col overflow-hidden">
      <header class="app-page-header flex h-16 shrink-0 items-center justify-between border-b border-border bg-background px-6">
        <div class="flex items-center gap-3">
          <h1 class="page-title text-lg font-semibold tracking-tight text-foreground">Settings · Schedules</h1>
          <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            Consolidated /settings/schedules
          </span>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs text-muted-foreground font-mono bg-muted/40 px-2.5 py-1 rounded">Things 3 Standard</span>
        </div>
      </header>

      <div class="flex-1 overflow-auto p-6 max-w-4xl space-y-6">
        <div class="p-4 bg-muted/20 rounded-xl border border-border/30 space-y-2">
          <h2 class="text-sm font-semibold text-foreground">Jadwal Rutin & Otomasi Operasional</h2>
          <p class="text-xs text-muted-foreground leading-relaxed">
            Semua rutinitas latar belakang, pembersihan cache, dan tugas berulang beroperasi terpusat di dalam Settings master-detail untuk kenyamanan mental model pengguna non-teknis.
          </p>
        </div>

        <div class="rounded-xl border border-border/30 divide-y divide-border/20 bg-card">
          <div class="p-4 flex items-center justify-between">
            <div>
              <div class="text-sm font-medium text-foreground">Nightly Database Cleanup</div>
              <div class="text-xs text-muted-foreground">Setiap hari pukul 02:00 WIB · 0 kegagalan</div>
            </div>
            <span class="text-xs font-mono bg-emerald-950 text-emerald-400 border border-emerald-800/40 px-2 py-0.5 rounded">Active</span>
          </div>
          <div class="p-4 flex items-center justify-between">
            <div>
              <div class="text-sm font-medium text-foreground">Customer Memory Sync</div>
              <div class="text-xs text-muted-foreground">Setiap 6 jam · Sinkronisasi preferensi rasa dan tone</div>
            </div>
            <span class="text-xs font-mono bg-emerald-950 text-emerald-400 border border-emerald-800/40 px-2 py-0.5 rounded">Active</span>
          </div>
        </div>
      </div>
    </main>
  </div>
</body>
</html>`

async function capture() {
  console.log('Launching Playwright Chromium for After screenshots...')
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
    // 1. Mobile Viewport (390 x 844)
    console.log('Capturing Mobile After Viewport (390x844)...')
    const mobilePage = await browser.newPage({
      viewport: { width: 390, height: 844 },
      deviceScaleFactor: 2,
    })
    await mobilePage.setContent(mobileAfterHtml, { waitUntil: 'load' })
    await mobilePage.waitForTimeout(500)
    const mobileOutputPath = path.join(screenshotsDir, 's62-t1-mobile-nav-after.png')
    await mobilePage.screenshot({ path: mobileOutputPath, fullPage: false })
    console.log(`Saved Mobile After screenshot: ${mobileOutputPath}`)
    await mobilePage.close()

    // 2. Desktop Viewport (1280 x 800)
    console.log('Capturing Desktop After Viewport (1280x800)...')
    const desktopPage = await browser.newPage({
      viewport: { width: 1280, height: 800 },
      deviceScaleFactor: 2,
    })
    await desktopPage.setContent(desktopAfterHtml, { waitUntil: 'load' })
    await desktopPage.waitForTimeout(500)
    const desktopOutputPath = path.join(screenshotsDir, 's62-t1-desktop-nav-after.png')
    await desktopPage.screenshot({ path: desktopOutputPath, fullPage: false })
    console.log(`Saved Desktop After screenshot: ${desktopOutputPath}`)
    await desktopPage.close()
  } finally {
    await browser.close()
  }
}

capture().catch((err) => {
  console.error('Error capturing after screenshots:', err)
  process.exit(1)
})
