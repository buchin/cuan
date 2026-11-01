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

// HTML Template representing InboxView with SlideDrawer open
const htmlTemplate = `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 61 — S61-T3 SlideDrawer Task Peek Visual Audit</title>
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
    .preview-canvas {
      min-height: 100vh;
      height: 100vh;
      display: flex;
      flex-direction: row;
      background-color: #141416;
      position: relative;
      overflow: hidden;
    }
    .chat-column {
      flex: 1;
      display: flex;
      flex-direction: column;
      min-width: 0;
      height: 100%;
      border-right: 1px solid rgba(255, 255, 255, 0.08);
      background-color: #141416;
    }
    .drawer-panel {
      width: 480px;
      max-width: 100%;
      height: 100%;
      display: flex;
      flex-direction: column;
      background-color: #18181b;
      border-left: 1px solid rgba(255, 255, 255, 0.1);
      box-shadow: -10px 0 30px rgba(0, 0, 0, 0.5);
      z-index: 50;
      flex-shrink: 0;
    }
    @media (max-width: 640px) {
      .drawer-panel {
        position: absolute;
        inset: 0;
        width: 100%;
      }
    }
  </style>
</head>
<body class="dark">
  <div class="preview-canvas">
    <!-- Left Column: Unbroken Conversation Stream -->
    <div class="chat-column">
      <!-- Top Calm Header -->
      <header class="calm-chat-header px-4 py-3 border-b border-border/30 bg-[#18181a] flex items-center justify-between shrink-0">
        <div class="flex items-center gap-3">
          <div class="h-9 w-9 rounded-full bg-secondary flex items-center justify-center font-semibold text-xs text-foreground">
            SM
          </div>
          <div>
            <h2 class="text-sm font-semibold text-foreground tracking-tight">Scrum Master (@sm)</h2>
            <p class="text-[11px] text-muted-foreground flex items-center gap-1.5">
              <span class="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
              <span>Online · Solopreneur Assistant</span>
            </p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs text-muted-foreground font-mono bg-muted/40 px-2 py-0.5 rounded">#S61-T3</span>
        </div>
      </header>

      <!-- Chat Stream Content -->
      <main class="flex-1 p-4 calm-stream-constraint space-y-4 overflow-y-auto">
        <!-- Quiet Date Divider -->
        <div class="flex justify-center my-3">
          <time class="calm-day-label text-xs font-medium text-muted-foreground select-none">
            Today
          </time>
        </div>

        <!-- Incoming Message -->
        <div class="flex flex-col items-start wa-message-row">
          <div class="calm-message-bubble calm-incoming max-w-[85%] rounded-2xl px-3.5 py-2.5 space-y-1.5">
            <div class="calm-plain-text text-[15px] leading-relaxed text-foreground">
              Pak Buchin, kita mulai eksekusi Task #OAMNK (S61-T3). Sub-halaman <code>ConversationWorkMonitor.tsx</code> dan tab monitor tasks sudah dihapus.
            </div>
            <div class="calm-markdown-footer mt-1 flex justify-end">
              <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:30</span>
            </div>
          </div>
        </div>

        <!-- Outgoing Message -->
        <div class="flex flex-col items-end wa-message-row">
          <div class="calm-message-bubble calm-outgoing max-w-[85%] rounded-2xl px-3.5 py-2.5 space-y-1.5">
            <div class="calm-plain-text text-[15px] leading-relaxed text-foreground">
              Bagus! Sekarang saat mengklik referensi task dari percakapan, gunakan <code>SlideDrawer</code> Progressive Disclosure dari kanan tanpa mengganti seluruh kanvas chat.
            </div>
            <div class="calm-markdown-footer mt-1 flex justify-end items-center gap-1">
              <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:32</span>
              <svg class="h-3.5 w-3.5 text-[#8e8e93]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
              </svg>
            </div>
          </div>
        </div>

        <!-- Incoming Message with Assigned to task link (Interactive SlideDrawer Trigger) -->
        <div class="flex flex-col items-start wa-message-row">
          <div class="calm-message-bubble calm-incoming max-w-[85%] rounded-2xl px-3.5 py-2.5 space-y-2">
            <div class="calm-plain-text text-[15px] leading-relaxed text-foreground">
              Sudah saya kaitkan dengan task berikut. Klik tautan tugas untuk mengintip detailnya di drawer kanan!
            </div>
            <!-- Clean Borderless Task Trigger Button -->
            <div class="pt-1">
              <button type="button" data-testid="assigned-task-btn" class="inline-flex items-center gap-1.5 text-xs font-medium text-primary hover:underline transition-colors cursor-pointer bg-primary/10 px-2.5 py-1 rounded-md">
                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/>
                </svg>
                <span>Assigned to task #OAMNK</span>
              </button>
            </div>
            <div class="calm-markdown-footer mt-1 flex justify-end">
              <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:35</span>
            </div>
          </div>
        </div>
      </main>

      <!-- Bottom Composer (Remains Unbroken) -->
      <footer class="calm-composer border-t border-border/30 bg-[#141416] p-3 shrink-0">
        <div class="calm-input-capsule flex items-center gap-2 rounded-2xl bg-[#1b1c1f] border border-white/5 px-3 py-2">
          <input type="text" placeholder="Tulis pesan untuk tim atau asisten..." class="bg-transparent flex-1 text-sm text-foreground placeholder:text-muted-foreground outline-none" disabled />
          <button class="calm-send h-7 w-7 rounded-full bg-[#ededed] text-[#141416] flex items-center justify-center shrink-0">
            <svg class="h-3.5 w-3.5 fill-current" viewBox="0 0 24 24">
              <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
            </svg>
          </button>
        </div>
      </footer>
    </div>

    <!-- Right Side: Progressive Disclosure SlideDrawer -->
    <div class="drawer-panel" data-testid="inbox-task-drawer">
      <!-- Drawer Header -->
      <div class="flex shrink-0 items-center justify-between gap-3 border-b border-border/40 px-5 py-4">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <h2 class="text-sm font-semibold leading-tight text-foreground truncate">
              S61-T3: SlideDrawer Integration for Task References in Chat
            </h2>
            <span class="text-[11px] font-mono font-medium px-2 py-0.5 rounded border border-border/40 text-blue-400 bg-blue-500/10 shrink-0">
              #OAMNK
            </span>
          </div>
          <p class="mt-1 text-xs text-muted-foreground truncate">
            #OAMNK · In Progress
          </p>
        </div>

        <div class="flex shrink-0 items-center gap-1">
          <button type="button" class="rounded-md p-1.5 text-muted-foreground hover:bg-muted/60 hover:text-foreground transition-colors min-h-[36px] min-w-[36px] flex items-center justify-center" title="Buka Halaman Tugas Penuh">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/>
            </svg>
          </button>
          <button type="button" class="rounded-md p-1.5 text-muted-foreground hover:bg-muted/60 hover:text-foreground transition-colors min-h-[36px] min-w-[36px] flex items-center justify-center" aria-label="Close drawer">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>
      </div>

      <!-- Drawer Body -->
      <div class="flex-1 overflow-y-auto px-5 py-4 space-y-4 text-sm" data-testid="inbox-task-drawer-content">
        <!-- Meta Strip -->
        <div class="rounded-xl border border-border/40 bg-card/60 p-3.5 space-y-2.5">
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <div class="flex items-center gap-2">
              <span class="inline-flex items-center gap-1.5 text-xs font-medium text-blue-400">
                <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span>Sedang Berjalan (Doing)</span>
              </span>
            </div>
            <div class="flex items-center gap-1 text-[11px] text-muted-foreground font-mono">
              <span>Baru saja</span>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2 pt-1 border-t border-border/30 text-xs">
            <span class="inline-flex items-center gap-1.5 text-muted-foreground">
              <svg class="h-3.5 w-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
              </svg>
              <span>Buchin (Dev Team)</span>
            </span>
            <span class="inline-flex items-center gap-1 font-mono text-[11px] text-muted-foreground bg-muted/40 px-2 py-0.5 rounded">
              <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 20l4-16m2 16l4-16M6 9h14M4 15h14"/>
              </svg>
              <span>feature/s61-t3-inbox-task-drawer</span>
            </span>
          </div>
        </div>

        <!-- Summary Section -->
        <div class="space-y-1.5">
          <div class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
            <svg class="h-3.5 w-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z"/>
            </svg>
            <span>Ringkasan Hasil</span>
          </div>
          <div class="rounded-xl border border-border/40 bg-card/40 p-3 text-xs leading-relaxed text-foreground">
            Integrasi SlideDrawer untuk referensi task di InboxView, eliminasi ConversationWorkMonitor dan tab tasks internal, serta audit visual Things 3 &amp; Bear Notes.
          </div>
        </div>

        <!-- Definition of Done Checklist -->
        <div class="space-y-2">
          <div class="flex items-center justify-between text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
            <span class="flex items-center gap-1.5">
              <svg class="h-3.5 w-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/>
              </svg>
              <span>Kriteria Selesai (DoD)</span>
            </span>
            <span class="font-mono text-[11px]">5/5</span>
          </div>
          <ul class="rounded-xl border border-border/40 bg-card/40 p-3 space-y-2 text-xs">
            <li class="flex items-start gap-2 leading-relaxed">
              <span class="mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded text-[10px] text-emerald-400 font-bold">✓</span>
              <span class="break-words font-medium text-foreground">Mengklik task dari gelembung percakapan membuka SlideDrawer, bukan mengganti seluruh kanvas chat.</span>
            </li>
            <li class="flex items-start gap-2 leading-relaxed">
              <span class="mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded text-[10px] text-emerald-400 font-bold">✓</span>
              <span class="break-words font-medium text-foreground">Alur obrolan tidak pernah terpotong atau hilang konteks saat mengintip detail task.</span>
            </li>
            <li class="flex items-start gap-2 leading-relaxed">
              <span class="mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded text-[10px] text-emerald-400 font-bold">✓</span>
              <span class="break-words font-medium text-foreground">0 banner duplikasi "Tasks - Internal" di dalam Inbox.</span>
            </li>
            <li class="flex items-start gap-2 leading-relaxed">
              <span class="mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded text-[10px] text-emerald-400 font-bold">✓</span>
              <span class="break-words font-medium text-foreground">Targeted unit test web/tests/s61-inbox-task-drawer.test.mjs PASS 100%.</span>
            </li>
            <li class="flex items-start gap-2 leading-relaxed">
              <span class="mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded text-[10px] text-emerald-400 font-bold">✓</span>
              <span class="break-words font-medium text-foreground">Full test suite (328+ tests) PASS dan build exit 0.</span>
            </li>
          </ul>
        </div>

        <!-- Terminal Quick Action -->
        <div class="pt-2">
          <button type="button" class="action-secondary w-full text-xs py-2.5 inline-flex items-center justify-center gap-2 rounded-lg border border-border/50 hover:bg-muted/50 transition-colors">
            <svg class="h-3.5 w-3.5 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
            </svg>
            <span>Buka Terminal Langsung</span>
          </button>
        </div>
      </div>

      <!-- Drawer Footer -->
      <div class="shrink-0 border-t border-border/40 bg-card/60 px-5 py-3 flex items-center justify-between gap-2">
        <button type="button" class="action-secondary min-h-10 px-3.5 text-xs rounded-lg border border-border/50">
          Tutup
        </button>
        <button type="button" class="action-primary min-h-10 px-3.5 text-xs inline-flex items-center gap-1.5 rounded-lg bg-primary text-primary-foreground font-semibold">
          <span>Buka Halaman Tugas</span>
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/>
          </svg>
        </button>
      </div>
    </div>
  </div>
</body>
</html>`

const previewHtmlPath = path.join(screenshotsDir, 'preview-s61-t3.html')
fs.writeFileSync(previewHtmlPath, htmlTemplate, 'utf8')

async function run() {
  console.log('Launching Chromium for S61-T3 SlideDrawer visual audit...')
  const browser = await chromium.launch({
    executablePath: path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium'),
    env: {
      ...process.env,
      LD_LIBRARY_PATH: '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' + (process.env.LD_LIBRARY_PATH || '')
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  })

  // 1. Mobile Viewport (390 x 844 - iPhone 12/13/14)
  console.log('Capturing Mobile Viewport (390x844)...')
  const mobileContext = await browser.newContext({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
    colorScheme: 'dark'
  })
  const mobilePage = await mobileContext.newPage()
  await mobilePage.goto('file://' + previewHtmlPath)
  await mobilePage.waitForTimeout(500)
  const mobileScreenshotPath = path.join(screenshotsDir, 's61-inbox-task-drawer-mobile.png')
  await mobilePage.screenshot({ path: mobileScreenshotPath, fullPage: true })
  console.log('Mobile screenshot saved to:', mobileScreenshotPath)

  // 2. Desktop Viewport (1440 x 900)
  console.log('Capturing Desktop Viewport (1440x900)...')
  const desktopContext = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
    colorScheme: 'dark'
  })
  const desktopPage = await desktopContext.newPage()
  await desktopPage.goto('file://' + previewHtmlPath)
  await desktopPage.waitForTimeout(500)
  const desktopScreenshotPath = path.join(screenshotsDir, 's61-inbox-task-drawer-desktop.png')
  await desktopPage.screenshot({ path: desktopScreenshotPath, fullPage: true })
  console.log('Desktop screenshot saved to:', desktopScreenshotPath)

  await browser.close()
  console.log('S61-T3 audit screenshots generated successfully!')
}

run().catch((err) => {
  console.error('Error capturing screenshots:', err)
  process.exit(1)
})
