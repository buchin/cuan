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
const mainCssFile = cssFiles.find((f) => f.startsWith('style-') || f.startsWith('index-')) || cssFiles[0]
const compiledCss = fs.readFileSync(path.join(distAssetsDir, mainCssFile), 'utf8')

const baseStyles = `
  ${compiledCss}
  body {
    background-color: #121214 !important;
    color: #ededed;
    margin: 0;
    padding: 0;
    font-family: 'Geist Variable', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
`

function createMobileInboxHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Chat / Inbox (Mobile 390px) - Calm Status & Active Task</title>
  <style>${baseStyles}</style>
</head>
<body class="dark bg-background text-foreground min-h-screen">
  <div class="max-w-[390px] mx-auto min-h-screen flex flex-col bg-background border-x border-border/40">
    <!-- Top Bar -->
    <header class="h-14 shrink-0 border-b border-border/40 px-4 flex items-center justify-between bg-card/60 backdrop-blur sticky top-0 z-10">
      <div class="flex items-center gap-2.5">
        <div class="h-7 w-7 rounded-full border-2 border-foreground flex items-center justify-center">
          <div class="h-2.5 w-2.5 rounded-full bg-foreground"></div>
        </div>
        <h1 class="text-base font-semibold">Chat</h1>
      </div>
      <div class="flex items-center gap-1.5">
        <button class="h-9 w-9 rounded-lg bg-primary/10 text-primary flex items-center justify-center">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
        </button>
        <button class="h-9 w-9 text-muted-foreground flex items-center justify-center text-lg">⋮</button>
      </div>
    </header>

    <!-- Search Bar -->
    <div class="px-4 py-2.5">
      <div class="relative w-full">
        <svg class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
        <input class="w-full min-h-11 rounded-lg border border-border/60 bg-muted/20 pl-10 pr-4 text-xs placeholder:text-muted-foreground focus:outline-none" placeholder="Search conversations..." />
      </div>
    </div>

    <!-- Filter Pills -->
    <div class="flex items-center gap-2 px-4 pb-2 border-b border-border/30">
      <span class="px-3.5 py-1.5 rounded-full bg-primary/20 text-primary text-xs font-medium">All</span>
      <span class="px-3 py-1.5 rounded-full bg-muted/40 text-muted-foreground text-xs font-medium">Agents 106</span>
      <span class="px-3 py-1.5 rounded-full bg-muted/40 text-muted-foreground text-xs font-medium">Customers 9</span>
    </div>

    <!-- Conversation List (Zero Visual Noise, No False Contradiction) -->
    <div class="flex-1 divide-y divide-border/40 overflow-y-auto">
      <!-- Item 1: Active Work in Progress (Clean, Calm, Working pulse, NO Connection Error) -->
      <div class="px-4 py-3.5 flex items-start gap-3 hover:bg-muted/20 transition-colors bg-accent/20 cursor-pointer">
        <div class="relative shrink-0 mt-0.5">
          <div class="h-10 w-10 rounded-full bg-secondary/80 flex items-center justify-center text-base border border-border/60">
            🤖
          </div>
          <!-- Working micro-indicator -->
          <div class="absolute -bottom-0.5 -right-0.5">
            <span class="relative flex h-2.5 w-2.5">
              <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-blue-500"></span>
            </span>
          </div>
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-1 mb-0.5">
            <span class="text-sm font-semibold text-foreground truncate">Kendala Runner Workspace ...</span>
            <span class="text-xs text-muted-foreground tabular-nums shrink-0">07:40</span>
          </div>
          <div class="flex items-center justify-between gap-2 mb-1">
            <span class="text-xs font-medium text-foreground/80 truncate">@sm</span>
            <!-- Active Task Badge -->
            <div class="inline-flex shrink-0 items-center gap-1.5 px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20">
              <span class="h-1.5 w-1.5 rounded-full bg-blue-400 animate-pulse"></span>
              <span class="text-[10px] font-medium">1 bekerja</span>
            </div>
          </div>
          <p class="text-xs text-zinc-400 line-clamp-2 leading-relaxed">
            Tugas Sesi Baru: Integrasi taskfsm & dodeval serta Live Deployment saat ini...
          </p>
        </div>
      </div>

      <!-- Item 2: Multi-Fase Plan (Clean, Active 2 Bekerja, Calm, No yellow error) -->
      <div class="px-4 py-3.5 flex items-start gap-3 hover:bg-muted/20 transition-colors cursor-pointer">
        <div class="relative shrink-0 mt-0.5">
          <div class="h-10 w-10 rounded-full bg-secondary/80 flex items-center justify-center text-base border border-border/60">
            🤖
          </div>
          <div class="absolute -bottom-0.5 -right-0.5">
            <span class="relative flex h-2.5 w-2.5">
              <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-blue-500"></span>
            </span>
          </div>
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-1 mb-0.5">
            <span class="text-sm font-medium text-foreground truncate">Rencana Multi-Fase: Ekstrak...</span>
            <span class="text-xs text-muted-foreground tabular-nums shrink-0">07:28</span>
          </div>
          <div class="flex items-center justify-between gap-2 mb-1">
            <span class="text-xs font-medium text-foreground/80 truncate">@sm</span>
            <div class="inline-flex shrink-0 items-center gap-1.5 px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20">
              <span class="h-1.5 w-1.5 rounded-full bg-blue-400 animate-pulse"></span>
              <span class="text-[10px] font-medium">2 bekerja</span>
            </div>
          </div>
          <p class="text-xs text-zinc-400 line-clamp-2 leading-relaxed">
            Halo! Pembaruan penting untuk cuan terkait pekerjaan: Sprint Plan: Ekstraksi...
          </p>
        </div>
      </div>

      <!-- Item 3: Customer Chat -->
      <div class="px-4 py-3.5 flex items-start gap-3 hover:bg-muted/20 transition-colors cursor-pointer">
        <div class="relative shrink-0 mt-0.5">
          <div class="h-10 w-10 rounded-full bg-secondary/80 flex items-center justify-center text-base border border-border/60 text-muted-foreground">
            👤
          </div>
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-1 mb-0.5">
            <span class="text-sm font-medium text-foreground truncate">Defri Putra</span>
            <span class="text-xs text-muted-foreground tabular-nums shrink-0">Yesterday</span>
          </div>
          <div class="flex items-center justify-between gap-2 mb-1">
            <span class="text-xs text-muted-foreground truncate">Percakapan</span>
          </div>
          <p class="text-xs text-zinc-400 line-clamp-2 leading-relaxed">
            Iya kayake sob.
          </p>
        </div>
      </div>
    </div>

    <!-- Mobile Bottom Navigation Bar -->
    <nav class="h-16 shrink-0 border-t border-border/40 bg-card/80 backdrop-blur px-2 flex items-center justify-around">
      <div class="flex flex-col items-center gap-1 text-primary">
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
        <span class="text-[10px] font-medium">Chat</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-muted-foreground">
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/></svg>
        <span class="text-[10px] font-medium">Tasks</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-muted-foreground">
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/></svg>
        <span class="text-[10px] font-medium">Projects</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-muted-foreground">
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/></svg>
        <span class="text-[10px] font-medium">Contacts</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-muted-foreground">
        <div class="h-5 w-5 rounded-full bg-muted/60 text-[10px] font-bold flex items-center justify-center">BU</div>
        <span class="text-[10px] font-medium">Account</span>
      </div>
    </nav>
  </div>
</body>
</html>`
}

async function run() {
  const browser = await chromium.launch({
    headless: true,
    executablePath: path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium'),
    env: {
      ...process.env,
      LD_LIBRARY_PATH:
        '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' +
        (process.env.LD_LIBRARY_PATH || ''),
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu'],
  })

  // 1. Mobile 390px
  const mobileContext = await browser.newContext({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  const mobilePage = await mobileContext.newPage()
  await mobilePage.setContent(createMobileInboxHtml(), { waitUntil: 'load' })
  await mobilePage.waitForTimeout(300)
  const mobileScreenshotPath = path.join(screenshotsDir, 'inbox-calm-status-mobile-390px.png')
  await mobilePage.screenshot({ path: mobileScreenshotPath, fullPage: true })

  await browser.close()
  console.log('Inbox calm status screenshot captured:', mobileScreenshotPath)
}

run().catch((err) => {
  console.error(err)
  process.exit(1)
})
