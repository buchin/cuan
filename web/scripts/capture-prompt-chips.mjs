import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')
const artifactsDir = '/home/dev/.gemini/antigravity-cli/brain/058f3b31-4bbd-49c5-8ce7-4d9e4dd8a87d'

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

function renderPromptChipsHtml() {
  return `
    <div class="mt-3 pt-2.5 border-t border-border/40" data-testid="inbox-prompt-choices">
      <div class="text-[11px] font-medium text-muted-foreground mb-2 flex items-center gap-1.5">
        <svg class="h-3 w-3 text-muted-foreground/80 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
        </svg>
        <span>Pilihan Cepat</span>
      </div>
      <div class="flex flex-wrap gap-2">
        <button
          type="button"
          class="group inline-flex items-center gap-2 px-3 py-2 min-h-11 rounded-lg text-xs font-medium border text-foreground bg-muted/20 hover:bg-muted/40 border-border/60 hover:border-border transition-colors touch-manipulation cursor-pointer"
        >
          <span class="inline-flex items-center justify-center h-4 w-4 rounded-full bg-emerald-500/15 text-emerald-500 shrink-0 text-[10px]">✓</span>
          <span class="truncate">Selesai (Done)</span>
        </button>
        <button
          type="button"
          class="group inline-flex items-center gap-2 px-3 py-2 min-h-11 rounded-lg text-xs font-medium border text-foreground bg-muted/20 hover:bg-muted/40 border-border/60 hover:border-border transition-colors touch-manipulation cursor-pointer"
        >
          <span class="truncate">Lanjut Revisi</span>
        </button>
        <button
          type="button"
          class="group inline-flex items-center gap-2 px-3 py-2 min-h-11 rounded-lg text-xs font-medium border text-muted-foreground bg-transparent hover:bg-destructive/10 hover:text-destructive border-transparent hover:border-destructive/30 transition-colors touch-manipulation cursor-pointer"
        >
          <span class="truncate">Batal / Stop</span>
        </button>
      </div>
    </div>
  `
}

function createDesktopHtml() {
  return `<!DOCTYPE html>
<html class="dark">
<head>
  <meta charset="utf-8">
  <title>Cuan Inbox - Desktop 1-Tap Prompt Chips</title>
  <style>${baseStyles}</style>
</head>
<body>
  <div class="app-viewport flex-row">
    <!-- Calm Minimal Sidebar -->
    <div class="w-16 border-r border-border flex flex-col items-center py-4 bg-[#141416] shrink-0">
      <div class="h-9 w-9 rounded-xl bg-primary/20 text-primary font-bold flex items-center justify-center text-sm border border-primary/30 mb-6">
        C
      </div>
      <div class="flex flex-col gap-3">
        <div class="p-2.5 rounded-lg bg-accent text-accent-foreground">
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/></svg>
        </div>
        <div class="p-2.5 rounded-lg text-muted-foreground hover:text-foreground">
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/></svg>
        </div>
      </div>
    </div>

    <!-- Middle List: Rooms -->
    <div class="w-80 border-r border-border flex flex-col bg-[#141416] shrink-0">
      <div class="h-13 border-b border-border px-4 flex items-center justify-between">
        <span class="text-sm font-semibold text-foreground">Inbox</span>
        <span class="text-xs text-muted-foreground font-mono">1 active</span>
      </div>
      <div class="p-2">
        <div class="p-3 rounded-lg bg-accent/40 border border-border/40">
          <div class="flex items-center justify-between mb-1">
            <span class="text-xs font-semibold text-foreground">@cuan coordinator</span>
            <span class="text-[10px] text-muted-foreground font-mono">11:50 AM</span>
          </div>
          <p class="text-xs text-muted-foreground line-clamp-1">Refactor access grant CLI dan verifikasi multi-phase</p>
        </div>
      </div>
    </div>

    <!-- Main Chat Canvas -->
    <div class="flex-1 flex flex-col bg-background min-w-0">
      <header class="h-13 border-b border-border px-6 flex items-center justify-between bg-background shrink-0">
        <div class="flex items-center gap-3">
          <div class="h-8 w-8 rounded-full bg-primary/20 text-primary font-bold flex items-center justify-center text-xs border border-primary/30">
            @c
          </div>
          <div>
            <div class="text-sm font-semibold text-foreground">@cuan (Dojo Coordinator)</div>
            <div class="text-xs text-muted-foreground">Active Project: Dojo CC</div>
          </div>
        </div>
      </header>

      <div class="flex-1 overflow-y-auto p-6 space-y-4">
        <!-- User affirmative trigger -->
        <div class="flex flex-col items-end ml-auto max-w-lg">
          <div class="rounded-2xl px-4 py-2.5 bg-[#25272c] text-foreground text-sm shadow-xs">
            gas beresin
          </div>
          <span class="text-[10px] font-mono text-muted-foreground mt-1 mr-2">11:48 AM</span>
        </div>

        <!-- Assistant response with prompt choice chips -->
        <div class="flex flex-col items-start max-w-xl">
          <div class="rounded-2xl px-4 py-3 bg-[#1b1c1f] text-foreground text-sm leading-relaxed shadow-xs border border-border/30">
            <p class="mb-2">Saya telah memeriksa status proyek <strong>Dojo CC</strong> dan verifikasi task <code>#task_dojo_grant</code> selesai tanpa error.</p>
            <p class="text-muted-foreground">Apakah pekerjaan ini sudah sesuai atau ingin kita lanjutkan revisi berikutnya?</p>
            ${renderPromptChipsHtml()}
          </div>
          <span class="text-[10px] font-mono text-muted-foreground mt-1 ml-2">11:49 AM</span>
        </div>
      </div>

      <!-- Composer -->
      <div class="p-4 border-t border-border/40 bg-card/20">
        <div class="flex items-center gap-2 bg-card/60 border border-border/60 rounded-xl px-4 py-2.5">
          <input type="text" placeholder="Ketik pesan atau pilih opsi cepat di atas..." class="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground focus:outline-none" />
          <button class="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted/40">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/></svg>
          </button>
        </div>
      </div>
    </div>
  </div>
</body>
</html>`
}

function createMobileHtml() {
  return `<!DOCTYPE html>
<html class="dark">
<head>
  <meta charset="utf-8">
  <title>Cuan Inbox - Mobile 390px 1-Tap Prompt Chips</title>
  <style>${baseStyles}</style>
</head>
<body>
  <div class="app-viewport">
    <!-- Mobile Header -->
    <header class="h-12 border-b border-border px-4 flex items-center justify-between bg-background shrink-0">
      <div class="flex items-center gap-2">
        <div class="h-7 w-7 rounded-full bg-primary/20 text-primary font-bold flex items-center justify-center text-xs border border-primary/30">
          @c
        </div>
        <div>
          <div class="text-xs font-semibold text-foreground">@cuan</div>
          <div class="text-[10px] text-muted-foreground">Dojo CC</div>
        </div>
      </div>
      <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
    </header>

    <!-- Chat Messages -->
    <div class="flex-1 overflow-y-auto p-3 space-y-3">
      <!-- User message -->
      <div class="flex flex-col items-end ml-auto max-w-[85%]">
        <div class="rounded-2xl px-3.5 py-2 bg-[#25272c] text-foreground text-xs shadow-xs">
          gas beresin
        </div>
        <span class="text-[9px] font-mono text-muted-foreground mt-0.5 mr-1.5">11:48</span>
      </div>

      <!-- Assistant message with mobile 44px min-h-11 chips -->
      <div class="flex flex-col items-start max-w-[92%]">
        <div class="rounded-2xl px-3.5 py-3 bg-[#1b1c1f] text-foreground text-xs leading-relaxed shadow-xs border border-border/30">
          <p class="mb-1.5">Verifikasi task <code>#task_dojo_grant</code> selesai tanpa error.</p>
          <p class="text-muted-foreground mb-1">Silakan pilih kelanjutan langkah berikut:</p>
          ${renderPromptChipsHtml()}
        </div>
        <span class="text-[9px] font-mono text-muted-foreground mt-0.5 ml-1.5">11:49</span>
      </div>
    </div>

    <!-- Mobile Composer -->
    <div class="p-2.5 border-t border-border/40 bg-card/20 shrink-0">
      <div class="flex items-center gap-2 bg-card/60 border border-border/60 rounded-xl px-3 py-2">
        <input type="text" placeholder="Ketik pesan..." class="flex-1 bg-transparent text-xs text-foreground placeholder:text-muted-foreground focus:outline-none" />
        <button class="p-1.5 rounded-lg text-primary hover:bg-muted/40">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/></svg>
        </button>
      </div>
    </div>
  </div>
</body>
</html>`
}

async function capture() {
  const executablePath = path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium')
  const browser = await chromium.launch({
    headless: true,
    executablePath,
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu'],
    env: {
      ...process.env,
      LD_LIBRARY_PATH:
        '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' +
        (process.env.LD_LIBRARY_PATH || ''),
    },
  })
  
  // 1. Desktop 1280x800
  const desktopPage = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2
  })
  await desktopPage.setContent(createDesktopHtml())
  await desktopPage.waitForTimeout(300)
  const desktopPath = path.join(artifactsDir, 'inbox-prompt-chips-desktop.png')
  await desktopPage.screenshot({ path: desktopPath })
  console.log('Saved desktop screenshot:', desktopPath)

  // 2. Mobile 390x844 (iPhone 14 / modern standard mobile viewport)
  const mobilePage = await browser.newPage({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2
  })
  await mobilePage.setContent(createMobileHtml())
  await mobilePage.waitForTimeout(300)
  const mobilePath = path.join(artifactsDir, 'inbox-prompt-chips-mobile-390px.png')
  await mobilePage.screenshot({ path: mobilePath })
  console.log('Saved mobile screenshot:', mobilePath)

  await browser.close()
}

capture().catch((err) => {
  console.error('Capture failed:', err)
  process.exit(1)
})
