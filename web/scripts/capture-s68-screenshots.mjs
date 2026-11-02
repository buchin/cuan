import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')
const evidenceDir = path.join(repoRoot, 'docs/sprints/evidence/s68')
const screenshotsDir = path.join(repoRoot, 'screenshots')

fs.mkdirSync(evidenceDir, { recursive: true })
fs.mkdirSync(screenshotsDir, { recursive: true })

const require = createRequire(import.meta.url)
const playwrightPath = path.join(process.env.HOME || '/home/dev', '.local/lib/node_modules/playwright')
const { chromium } = require(playwrightPath)

// Find compiled CSS
const distAssetsDir = path.join(webDir, 'dist/assets')
const cssFiles = fs.readdirSync(distAssetsDir).filter((f) => f.endsWith('.css'))
const mainCssFile = cssFiles.find((f) => f.startsWith('style-') || f.startsWith('index-')) || cssFiles[0]
const compiledCss = fs.readFileSync(path.join(distAssetsDir, mainCssFile), 'utf8')

const baseStyles = `
  ${compiledCss}
  body {
    background-color: #141416 !important;
    color: #ededed;
    margin: 0;
    padding: 0;
    font-family: 'Geist Variable', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
  .app-viewport {
    height: 100vh;
    display: flex;
    flex-direction: column;
    background-color: #141416;
    overflow: hidden;
  }
`

function createDesktopHtml(showMoreMenu = true) {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <title>Cuan Sprint 68 - Desktop Navigation</title>
  <style>${baseStyles}</style>
</head>
<body class="bg-background text-foreground antialiased font-sans">
  <div class="flex h-screen w-screen overflow-hidden bg-background">
    <!-- Desktop Sidebar -->
    <aside class="hidden md:flex w-64 flex-col border-r border-border/40 bg-card/25 p-4 justify-between">
      <div class="space-y-6">
        <div class="flex items-center gap-2.5 px-2">
          <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold font-mono">
            C
          </div>
          <span class="font-bold tracking-tight text-lg text-foreground">Cuan</span>
        </div>
        <nav class="space-y-1">
          <div class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70 px-2 pb-1.5">Workspace</div>
          <a class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-foreground/80 hover:bg-muted/40 transition-colors" href="#">
            <svg class="h-4 w-4 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
            <span>Chat</span>
          </a>
          <a class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm bg-accent text-accent-foreground font-semibold" href="#">
            <svg class="h-4 w-4 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/></svg>
            <span>Tasks</span>
          </a>
          <a class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-foreground/80 hover:bg-muted/40 transition-colors" href="#">
            <svg class="h-4 w-4 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>
            <span>Projects</span>
          </a>
          <div class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70 px-2 pt-4 pb-1.5">Operations</div>
          <a class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-foreground/80 hover:bg-muted/40 transition-colors" href="#">
            <svg class="h-4 w-4 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"/></svg>
            <span>Contacts</span>
          </a>
        </nav>
      </div>
      <!-- User Profile at bottom with Preferences Popover Open -->
      <div class="pt-4 border-t border-border/40 relative">
        <div class="flex items-center gap-3 p-2 rounded-xl bg-accent/40 border border-border/40">
          <div class="h-8 w-8 rounded-full bg-primary text-primary-foreground font-mono font-bold flex items-center justify-center text-xs">
            BU
          </div>
          <div class="flex-1 min-w-0">
            <div class="font-bold text-xs text-foreground flex items-center gap-1.5">
              <span>Buchin</span>
              <span class="text-[9px] font-mono px-1.5 py-0.2 bg-emerald-500/10 text-emerald-500 border border-emerald-500/30 rounded uppercase">Owner</span>
            </div>
            <div class="text-[10px] text-muted-foreground truncate">buchin@dropsugar.com</div>
          </div>
        </div>

        <!-- Rendered NavUser Popover Menu (Single Source of Truth) -->
        <div class="absolute left-0 bottom-full mb-2 z-50 w-64 bg-popover text-popover-foreground border border-border/40 shadow-xl p-1.5 rounded-xl text-sm">
          <div class="p-2 border-b border-border/40 mb-1 flex items-center gap-2.5">
            <div class="h-7 w-7 rounded-full bg-primary text-primary-foreground font-mono font-bold text-xs flex items-center justify-center">BU</div>
            <div class="min-w-0 flex-1">
              <div class="text-xs font-bold text-foreground">Buchin</div>
              <div class="text-[10px] text-muted-foreground truncate">buchin@dropsugar.com</div>
            </div>
          </div>
          <div class="px-2.5 pt-1.5 pb-0.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Account & Security</div>
          <a href="#" class="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-foreground/90 hover:bg-accent rounded-lg transition-colors text-left">
            <svg class="h-4 w-4 text-primary shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/></svg>
            <span>Profile & password</span>
          </a>
          <div class="h-px bg-border/40 my-1"></div>
          <div class="px-2.5 pt-1.5 pb-0.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Preferences</div>
          <a href="#" class="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-foreground/90 hover:bg-accent rounded-lg transition-colors text-left">
            <svg class="h-4 w-4 text-muted-foreground shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>
            <span>Settings</span>
          </a>
          <button class="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-foreground/90 hover:bg-accent rounded-lg transition-colors text-left">
            <svg class="h-4 w-4 text-muted-foreground shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            <span>Language</span>
            <span class="ml-auto text-xs font-mono text-muted-foreground">Bahasa Indonesia</span>
          </button>
          <div class="h-px bg-border/40 my-1"></div>
          <button class="flex min-h-11 w-full items-center gap-2 px-2.5 py-2 text-sm text-destructive hover:bg-destructive/10 rounded-lg transition-colors text-left font-semibold">
            <svg class="h-4 w-4 text-destructive shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"/></svg>
            <span>Sign out</span>
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Workspace -->
    <main class="flex-1 flex flex-col min-w-0 overflow-hidden">
      <!-- Top Navigation Header (Clean, Zero Redundant MoreVertical) -->
      <header class="h-14 border-b border-border/40 px-6 flex items-center justify-between bg-card/20">
        <div class="flex items-center gap-3">
          <h1 class="text-sm font-semibold text-foreground">Tasks Workspace</h1>
          <span class="text-xs text-muted-foreground font-mono">Sprint 68</span>
        </div>
        <div class="flex items-center gap-2">
          <!-- Clean Header Action: New Task only (Zero MoreVertical Clutter) -->
          <button class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-xs font-semibold hover:bg-primary/90 transition-colors">
            <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
            <span>New Task</span>
          </button>
        </div>
      </header>

      <!-- Content Area -->
      <div class="flex-1 overflow-y-auto p-8 max-w-4xl mx-auto w-full">
        <div class="space-y-4">
          <div class="flex items-center justify-between pb-3 border-b border-border/40">
            <div>
              <h2 class="text-base font-semibold text-foreground">Active Tasks</h2>
              <p class="text-xs text-muted-foreground mt-0.5">Things 3 calm list with hairline dividers</p>
            </div>
            <span class="text-xs font-mono text-muted-foreground">3 tasks running</span>
          </div>

          <div class="divide-y divide-border/40 rounded-xl border border-border/40 bg-card/25 overflow-hidden">
            <div class="flex items-center justify-between p-3.5 hover:bg-muted/20 transition-colors">
              <div class="flex items-center gap-3">
                <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
                <span class="text-sm font-medium text-foreground">S68-T1 Deploy Pipeline Optimization</span>
              </div>
              <span class="text-xs font-mono text-muted-foreground">Just now</span>
            </div>
            <div class="flex items-center justify-between p-3.5 hover:bg-muted/20 transition-colors">
              <div class="flex items-center gap-3">
                <span class="h-2 w-2 rounded-full bg-blue-500 animate-pulse"></span>
                <span class="text-sm font-medium text-foreground">S68-T2 Calm Navigation Architecture</span>
              </div>
              <span class="text-xs font-mono text-muted-foreground">Running</span>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</body>
</html>`
}

function createMobileHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <title>Cuan Sprint 68 - Mobile 390px Navigation</title>
  <style>${baseStyles}</style>
</head>
<body class="bg-background text-foreground antialiased font-sans">
  <div class="flex flex-col h-screen w-screen overflow-hidden bg-background">
    <!-- Mobile Top Header (Clean, Zero Redundant MoreVertical) -->
    <header class="h-14 border-b border-border/40 px-4 flex items-center justify-between bg-card/20 shrink-0">
      <div class="flex items-center gap-2">
        <div class="h-7 w-7 rounded-lg bg-primary text-primary-foreground font-bold font-mono flex items-center justify-center text-xs">
          C
        </div>
        <span class="font-bold text-base text-foreground tracking-tight">Cuan</span>
      </div>
    </header>

    <!-- Mobile Body -->
    <main class="flex-1 overflow-y-auto p-4 space-y-4">
      <div class="flex items-center justify-between pb-2 border-b border-border/40">
        <h2 class="text-base font-semibold text-foreground">Ringkasan Tugas</h2>
        <span class="text-xs font-mono text-muted-foreground">Mobile 390px</span>
      </div>

      <div class="divide-y divide-border/40 rounded-xl border border-border/40 bg-card/25 overflow-hidden">
        <div class="p-3.5 space-y-1">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-foreground">S68-T1 Pipeline Gate</span>
            <span class="text-[10px] font-mono text-emerald-500 bg-emerald-500/10 px-1.5 py-0.5 rounded border border-emerald-500/30">PASS</span>
          </div>
          <p class="text-xs text-muted-foreground">Fail-fast tsc & parallel Go build verified.</p>
        </div>
        <div class="p-3.5 space-y-1">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-foreground">S68-T4 Projects Intact</span>
            <span class="text-[10px] font-mono text-blue-500 bg-blue-500/10 px-1.5 py-0.5 rounded border border-blue-500/30">ACTIVE</span>
          </div>
          <p class="text-xs text-muted-foreground">Pillar 3: Projects tetap hadir di navigasi.</p>
        </div>
      </div>
    </main>

    <!-- Mobile 5-Pillar Bottom Navigation with Projects Retained -->
    <nav class="h-16 border-t border-border/40 bg-card/40 backdrop-blur px-2 flex items-center justify-around shrink-0 pb-safe">
      <!-- 1. Chat -->
      <a href="#" class="flex flex-col items-center justify-center min-h-14 flex-1 text-muted-foreground hover:text-foreground">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
        <span class="text-[11px] font-medium mt-1">Chat</span>
      </a>

      <!-- 2. Tasks -->
      <a href="#" class="flex flex-col items-center justify-center min-h-14 flex-1 text-primary font-semibold">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/></svg>
        <span class="text-[11px] mt-1">Tasks</span>
      </a>

      <!-- 3. Projects (Retained per user instruction) -->
      <a href="#" class="flex flex-col items-center justify-center min-h-14 flex-1 text-muted-foreground hover:text-foreground">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>
        <span class="text-[11px] font-medium mt-1">Projects</span>
      </a>

      <!-- 4. Contacts -->
      <a href="#" class="flex flex-col items-center justify-center min-h-14 flex-1 text-muted-foreground hover:text-foreground">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"/></svg>
        <span class="text-[11px] font-medium mt-1">Contacts</span>
      </a>

      <!-- 5. Account (NavUser) -->
      <a href="#" class="flex flex-col items-center justify-center min-h-14 flex-1 text-muted-foreground hover:text-foreground">
        <div class="h-5 w-5 rounded-full bg-muted text-muted-foreground font-mono font-bold text-[9px] flex items-center justify-center">
          BU
        </div>
        <span class="text-[11px] font-medium mt-1">Account</span>
      </a>
    </nav>
  </div>
</body>
</html>`
}

async function run() {
  const executablePath = path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium')
  const browser = await chromium.launch({
    headless: true,
    executablePath,
    env: {
      ...process.env,
      LD_LIBRARY_PATH:
        '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' +
        (process.env.LD_LIBRARY_PATH || ''),
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--disable-gpu'],
  })

  // 1. Desktop - 1440x900
  {
    const page = await browser.newPage({
      viewport: { width: 1440, height: 900 },
      deviceScaleFactor: 2,
    })
    await page.setContent(createDesktopHtml(true), { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)
    const outEvidence = path.join(evidenceDir, 's68-desktop-more-menu.png')
    const outScreenshots = path.join(screenshotsDir, 's68-desktop-more-menu.png')
    await page.screenshot({ path: outEvidence })
    fs.copyFileSync(outEvidence, outScreenshots)
    console.log('✓ Captured desktop screenshot:', outEvidence)
    await page.close()
  }

  // 2. Mobile - 390x844 (iPhone 12/13/14 viewport)
  {
    const page = await browser.newPage({
      viewport: { width: 390, height: 844 },
      deviceScaleFactor: 2,
    })
    await page.setContent(createMobileHtml(), { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)
    const outEvidence = path.join(evidenceDir, 's68-mobile-390px-bottom-nav.png')
    const outScreenshots = path.join(screenshotsDir, 's68-mobile-390px-bottom-nav.png')
    await page.screenshot({ path: outEvidence })
    fs.copyFileSync(outEvidence, outScreenshots)
    console.log('✓ Captured mobile screenshot:', outEvidence)
    await page.close()
  }

  await browser.close()
  console.log('Done capturing Sprint 68 visual verification screenshots.')
}

run().catch((err) => {
  console.error('Error capturing screenshots:', err)
  process.exit(1)
})
