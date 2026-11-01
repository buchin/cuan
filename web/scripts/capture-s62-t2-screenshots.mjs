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
const mainCssFile = cssFiles.find((f) => f.startsWith('index-') || f.startsWith('style-')) || cssFiles[0]
const compiledCss = fs.readFileSync(path.join(distAssetsDir, mainCssFile), 'utf8')
const chatCalmCss = fs.readFileSync(path.join(webDir, 'src/features/inbox/chat-calm.css'), 'utf8')

const htmlTemplate = `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 62 — S62-T2 Chat Canvas & Composer Cleanup</title>
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
  <div class="preview-canvas wa-chat calm-chat">
    <!-- Top Calm Header (Seamless, Zero 1px Border-b Cut) -->
    <header class="wa-chat-header h-14 sm:h-16 px-3 sm:px-6 flex items-center justify-between shrink-0 border-b-0 sticky top-0 z-20">
      <div class="flex items-center gap-2 sm:gap-3 min-w-0">
        <!-- Mobile Back Button (<) -->
        <button
          type="button"
          title="Back to conversations"
          class="inline-flex h-10 w-10 sm:h-11 sm:w-11 items-center justify-center -ml-1 text-muted-foreground hover:text-foreground hover:bg-muted/50 rounded-full transition-colors shrink-0"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
        </button>

        <div class="relative shrink-0">
          <div class="h-9 w-9 rounded-full bg-secondary flex items-center justify-center text-sm font-bold border border-border/40">
            <svg class="h-4 w-4 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <rect x="3" y="11" width="18" height="10" rx="2" stroke-width="2" />
              <circle cx="12" cy="5" r="2" stroke-width="2" />
              <path d="M12 7v4" stroke-width="2" />
              <line x1="8" y1="16" x2="8" y2="16" stroke-width="2" stroke-linecap="round" />
              <line x1="16" y1="16" x2="16" y2="16" stroke-width="2" stroke-linecap="round" />
            </svg>
          </div>
        </div>

        <div class="min-w-0 flex flex-col justify-center">
          <div class="flex items-center gap-1.5 min-w-0">
            <h2 class="truncate text-base font-semibold leading-tight text-foreground">
              Hermes Support
            </h2>
          </div>
          <span class="inline-flex items-center gap-1.5 text-xs text-emerald-400">
            <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500"></span>
            Connected
          </span>
        </div>
      </div>

      <div class="flex shrink-0 items-center gap-1">
        <button
          type="button"
          class="h-9 w-9 flex items-center justify-center text-muted-foreground hover:text-foreground rounded-lg transition-colors"
          title="Manage agent connection"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <circle cx="12" cy="12" r="3" stroke-width="2" />
          </svg>
        </button>
      </div>
    </header>

    <!-- Main Chat Canvas (Zero Clutter, Zero Chip Overlap) -->
    <main class="flex-1 min-h-0 min-w-0 p-3 sm:p-6 overflow-y-auto space-y-3">
      <div class="calm-stream-constraint max-w-[75ch] mx-auto w-full space-y-3">
        <!-- Quiet Date Label -->
        <div class="flex justify-center my-3">
          <time class="calm-day-label text-xs font-medium text-muted-foreground select-none">
            Today
          </time>
        </div>

        <!-- Outgoing Message (User to Hermes AI) -->
        <div class="flex flex-col items-end wa-message-row">
          <div class="wa-message-bubble calm-message-bubble wa-outgoing calm-outgoing rounded-2xl px-4 py-2.5">
            <div class="wa-plain-text calm-plain-text text-[15px] leading-relaxed text-foreground">
              Halo Hermes, mohon bantu periksa status pesanan pelanggan Siti Rahma hari ini.
            </div>
            <div class="wa-markdown-footer calm-markdown-footer mt-1 flex justify-end items-center gap-1">
              <span class="wa-message-meta calm-message-meta text-[11px] text-[#8e8e93]">09:46</span>
              <svg class="h-3.5 w-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
              </svg>
            </div>
          </div>
        </div>

        <!-- Incoming Message (Hermes AI response — 0 Memory Chips Cluttering Timestamp) -->
        <div class="flex flex-col items-start wa-message-row">
          <div class="wa-message-bubble calm-message-bubble wa-incoming calm-incoming rounded-2xl px-4 py-2.5 space-y-1">
            <div class="wa-plain-text calm-plain-text text-[15px] leading-relaxed text-foreground">
              Tentu, Pak! Pesanan Siti Rahma (INV-2026-890) telah dikemas dan menunggu penjemputan kurir logistik sore ini pukul 15:00 WIB. Nomor resi otomatis dikirimkan via WhatsApp.
            </div>
            <!-- Clean unhindered footer with zero chip collision -->
            <div class="wa-markdown-footer calm-markdown-footer mt-1 flex justify-end">
              <span class="wa-message-meta calm-message-meta text-[11px] text-[#8e8e93]">09:47</span>
            </div>
          </div>
        </div>

        <!-- Another Outgoing Message -->
        <div class="flex flex-col items-end wa-message-row">
          <div class="wa-message-bubble calm-message-bubble wa-outgoing calm-outgoing rounded-2xl px-4 py-2.5">
            <div class="wa-plain-text calm-plain-text text-[15px] leading-relaxed text-foreground">
              Terima kasih, sangat cepat dan jelas!
            </div>
            <div class="wa-markdown-footer calm-markdown-footer mt-1 flex justify-end items-center gap-1">
              <span class="wa-message-meta calm-message-meta text-[11px] text-[#8e8e93]">09:48</span>
              <svg class="h-3.5 w-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
              </svg>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Bottom Floating Soft Surface Composer (Borderless, Zero 1px Border-t Cut, Bear Notes Elevation) -->
    <footer class="wa-composer calm-composer shrink-0 z-20 border-t-0 p-3 sm:p-4">
      <div class="mx-auto max-w-[75ch]">
        <div class="wa-input-capsule calm-input-capsule flex min-w-0 items-end gap-1.5 p-1.5">
          <!-- Attachment Button (+) -->
          <button
            type="button"
            class="h-9 w-9 rounded-full flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors shrink-0"
            title="Attach file"
          >
            <svg class="h-[18px] w-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
          </button>

          <!-- Input Textarea (Zero Wireframe, Pure Serene Input) -->
          <textarea
            rows="1"
            placeholder="Write a message…"
            class="wa-input calm-input min-w-0 flex-1 min-h-[38px] max-h-[140px] resize-none overflow-hidden py-2 text-[15px] sm:text-sm leading-5 pl-1.5 pr-2 outline-none"
          ></textarea>

          <!-- Quick replies (⚡) HIDDEN for AI agent conversations (Clean & Focused) -->

          <!-- Send Button -->
          <button
            type="button"
            class="wa-send h-9 w-9 rounded-full bg-[#ededed] text-[#141416] flex items-center justify-center active:scale-95 transition-all cursor-pointer shrink-0"
            title="Send message"
          >
            <svg class="h-[18px] w-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M12 5l7 7-7 7" />
            </svg>
          </button>
        </div>
      </div>
    </footer>
  </div>
</body>
</html>`

const previewHtmlPath = path.join(screenshotsDir, 's62-t2-chat-preview.html')
fs.writeFileSync(previewHtmlPath, htmlTemplate, 'utf8')

async function run() {
  console.log('Launching Chromium for S62-T2 visual audit...')
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
  const mobileScreenshotPath = path.join(screenshotsDir, 's62-t2-chat-cleanup-mobile-after.png')
  await mobilePage.screenshot({ path: mobileScreenshotPath, fullPage: true })
  console.log('Mobile after screenshot saved to:', mobileScreenshotPath)

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
  const desktopScreenshotPath = path.join(screenshotsDir, 's62-t2-chat-cleanup-desktop-after.png')
  await desktopPage.screenshot({ path: desktopScreenshotPath, fullPage: true })
  console.log('Desktop after screenshot saved to:', desktopScreenshotPath)

  await browser.close()
  console.log('S62-T2 screenshots generated successfully!')
}

run().catch((err) => {
  console.error('Error capturing screenshots:', err)
  process.exit(1)
})
