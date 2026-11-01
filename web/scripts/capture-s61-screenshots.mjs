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

// Create standalone HTML representing the Inbox chat stream in dark mode
const htmlTemplate = `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 61 — Calm Low-Contrast Chat Bubbles Audit</title>
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
    }
    .preview-canvas {
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      background-color: #141416;
    }
  </style>
</head>
<body class="dark">
  <div class="preview-canvas calm-chat">
    <!-- Top Calm Header -->
    <header class="calm-chat-header px-4 py-3 border-b border-border/30 bg-[#18181a] flex items-center justify-between sticky top-0 z-10">
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
        <span class="text-xs text-muted-foreground font-mono bg-muted/40 px-2 py-0.5 rounded">#S61-T2</span>
      </div>
    </header>

    <!-- Chat Stream Content -->
    <main class="flex-1 p-4 calm-stream-constraint space-y-4">
      <!-- Quiet Date Divider (Zero White Pill Glare) -->
      <div class="flex justify-center my-4">
        <time class="calm-day-label text-xs font-medium text-muted-foreground select-none">
          Today
        </time>
      </div>

      <!-- Incoming Message (Left: Low-Contrast Monochromatic Micro-Tint) -->
      <div class="flex flex-col items-start wa-message-row">
        <div class="calm-message-bubble calm-incoming max-w-[85%] rounded-2xl px-3.5 py-2.5">
          <div class="calm-plain-text text-[15px] leading-relaxed text-foreground">
            Halo Pak Buchin! Saya sudah meninjau backlog Sprint 61. Fokus kita sekarang adalah menuntaskan estetika Things 3 & Bear Notes pada gelembung obrolan chat.
          </div>
          <div class="calm-markdown-footer mt-1 flex justify-end">
            <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:14</span>
          </div>
        </div>
      </div>

      <!-- Outgoing Message (Right: Monochromatic Neutral) -->
      <div class="flex flex-col items-end wa-message-row">
        <div class="calm-message-bubble calm-outgoing max-w-[85%] rounded-2xl px-3.5 py-2.5">
          <div class="calm-plain-text text-[15px] leading-relaxed text-foreground">
            Bagus. Pastikan border stroke putih yang keras disingkirkan, pil tanggal Today/Yesterday tidak silau, dan kartu task mengalir borderless.
          </div>
          <div class="calm-markdown-footer mt-1 flex justify-end items-center gap-1">
            <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:16</span>
            <svg class="h-3.5 w-3.5 text-[#8e8e93]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
            </svg>
          </div>
        </div>
      </div>

      <!-- Incoming Message with Assigned to task link (Clean Typography, No Neon Badge) -->
      <div class="flex flex-col items-start wa-message-row">
        <div class="calm-message-bubble calm-incoming max-w-[85%] rounded-2xl px-3.5 py-2.5 space-y-2">
          <div class="calm-plain-text text-[15px] leading-relaxed text-foreground">
            Siap! Task S61-T2 sudah di-update ke status doing. Kita juga mengeliminasi kotak kawat ganda di kartu task.
          </div>
          <!-- Clean Borderless Task Reference -->
          <div class="pt-1">
            <span class="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors cursor-pointer bg-transparent border-0 p-0">
              <svg class="h-3.5 w-3.5 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/>
              </svg>
              <span>Assigned to task</span>
            </span>
          </div>
          <div class="calm-markdown-footer mt-1 flex justify-end">
            <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:18</span>
          </div>
        </div>
      </div>

      <!-- Streamlined Borderless Agent Process Indicator -->
      <div class="mt-2.5 flex w-full self-stretch flex-col items-start" aria-label="Agent response progress">
        <div class="calm-message-bubble calm-incoming !bg-transparent !border-0 !shadow-none !p-0 relative min-w-0 max-w-xl w-full">
          <div class="flex items-center gap-2 py-1 text-xs text-muted-foreground">
            <svg class="h-3.5 w-3.5 animate-spin text-muted-foreground" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <span class="font-medium text-muted-foreground">The agent is verifying calm chat bubbles and quiet dividers…</span>
            <span class="text-[11px] tabular-nums text-muted-foreground/70">4s</span>
          </div>
        </div>
      </div>

      <!-- Streamlined Borderless TaskCard in Stream -->
      <div class="calm-message-bubble calm-incoming !bg-transparent !border-0 !shadow-none !p-0 relative min-w-0 max-w-xl w-full mt-2">
        <div data-card="task-card" class="rounded-xl border border-white/5 bg-white/[0.02] p-3.5 text-card-foreground space-y-2">
          <div class="flex items-center justify-between text-xs text-muted-foreground">
            <div class="flex items-center gap-2">
              <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold bg-blue-500/10 text-blue-400">
                <span class="h-1.5 w-1.5 rounded-full bg-blue-400"></span>
                Working
              </span>
              <span class="font-mono">#LQIDR</span>
            </div>
            <span class="text-[11px]">Just now</span>
          </div>
          <h4 class="text-sm font-semibold text-foreground">
            S61-T2: Calm Low-Contrast Chat Bubbles, Quiet Date Dividers & Things/Bear CSS Overhaul
          </h4>
          <p class="text-xs text-muted-foreground line-clamp-1">
            Menyingkirkan border stroke putih keras, 0 white pill glare, borderless status cards.
          </p>
        </div>
      </div>
    </main>

    <!-- Bottom Composer -->
    <footer class="calm-composer border-t border-border/30 bg-[#141416] p-3">
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
</body>
</html>`

const previewHtmlPath = path.join(screenshotsDir, 'preview.html')
fs.writeFileSync(previewHtmlPath, htmlTemplate, 'utf8')

async function run() {
  console.log('Launching Chromium for S61 visual audit...')
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
  const mobileScreenshotPath = path.join(screenshotsDir, 's61-calm-chat-bubbles-mobile.png')
  await mobilePage.screenshot({ path: mobileScreenshotPath, fullPage: true })
  console.log('Mobile screenshot saved to:', mobileScreenshotPath)

  // 2. Desktop Viewport (1280 x 800)
  console.log('Capturing Desktop Viewport (1280x800)...')
  const desktopContext = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
    colorScheme: 'dark'
  })
  const desktopPage = await desktopContext.newPage()
  await desktopPage.goto('file://' + previewHtmlPath)
  await desktopPage.waitForTimeout(500)
  const desktopScreenshotPath = path.join(screenshotsDir, 's61-calm-chat-bubbles-desktop.png')
  await desktopPage.screenshot({ path: desktopScreenshotPath, fullPage: true })
  console.log('Desktop screenshot saved to:', desktopScreenshotPath)

  await browser.close()
  console.log('Audit screenshots generated successfully!')
}

run().catch((err) => {
  console.error('Error capturing screenshots:', err)
  process.exit(1)
})
