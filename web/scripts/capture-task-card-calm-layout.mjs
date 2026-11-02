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
const chatCalmCss = fs.existsSync(path.join(webDir, 'src/features/inbox/chat-calm.css'))
  ? fs.readFileSync(path.join(webDir, 'src/features/inbox/chat-calm.css'), 'utf8')
  : ''

const baseStyles = `
  ${compiledCss}
  ${chatCalmCss}
  body {
    background-color: #141416 !important;
    color: #ededed;
    margin: 0;
    padding: 0;
    font-family: 'Geist Variable', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
`

function createChatLayoutHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Chat Stream - Calm Task Card & Zero Layout Shift</title>
  <style>${baseStyles}</style>
</head>
<body class="dark bg-[#141416] text-[#ededed]">
  <div class="max-w-[390px] md:max-w-3xl mx-auto min-h-screen flex flex-col bg-[#141416] border-x border-border/40">
    <!-- Header -->
    <header class="h-14 shrink-0 border-b border-border/40 px-4 flex items-center justify-between bg-card/40 backdrop-blur sticky top-0 z-10">
      <div class="flex items-center gap-2.5">
        <div class="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center text-primary font-bold text-xs">
          🤖
        </div>
        <div>
          <div class="flex items-center gap-1.5">
            <span class="text-sm font-semibold">@sm</span>
            <span class="inline-flex h-2 w-2 rounded-full bg-emerald-500"></span>
          </div>
          <span class="text-[11px] text-muted-foreground">Connected</span>
        </div>
      </div>
    </header>

    <!-- Message Stream -->
    <main class="flex-1 p-4 space-y-4 overflow-y-auto">
      <!-- Human Message -->
      <div class="flex justify-end">
        <div class="calm-message-bubble bg-[#25272c] border border-border/30 rounded-2xl px-4 py-2.5 text-sm max-w-[85%] text-foreground">
          Approved: integrasi taskfsm & dodeval serta live deployment
        </div>
      </div>

      <!-- Agent Response with Embedded Task Card -->
      <div class="flex flex-col items-start space-y-2">
        <div class="calm-message-bubble bg-[#1b1c1f] border border-border/30 rounded-2xl px-4 py-2.5 text-sm max-w-full text-foreground space-y-2">
          <p class="leading-relaxed">Work started. I'll report progress and the result here.</p>
        </div>

        <!-- Stable Loaded Task Card (Zero CLS, Calm Hairline, 44px touch targets) -->
        <div class="w-full max-w-xl">
          <div data-card="task-card" class="rounded-xl border border-border/40 bg-card/40 p-4 text-card-foreground select-none space-y-3 min-h-[44px]">
            <div class="flex flex-wrap items-center justify-between gap-2 border-b border-border/30 pb-2.5">
              <div class="flex items-center gap-2 min-w-0">
                <span class="inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold bg-blue-500/15 text-blue-800 dark:text-blue-200 border-blue-500/35">
                  <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-500 opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-blue-600 dark:bg-blue-400"></span>
                  </span>
                  <span>Working</span>
                </span>
                <span data-testid="task-card-live-timer" class="inline-flex items-center gap-1 text-[11px] font-mono font-medium text-muted-foreground bg-muted/60 px-2 py-0.5 rounded-md">
                  ⏱ 02:45
                </span>
                <span class="text-xs text-muted-foreground font-medium flex items-center gap-1">
                  📁 cuan <span class="font-mono text-muted-foreground/80">#2665FD</span>
                </span>
              </div>
            </div>

            <div class="space-y-1">
              <h4 class="text-sm font-semibold text-foreground break-words leading-snug">
                Sesi Baru: Integrasi taskfsm & dodeval serta Live Deployment
              </h4>
              <p class="text-xs text-blue-600 dark:text-blue-400 font-medium">
                Running build and unit verification...
              </p>
            </div>

            <div class="pt-2 flex items-center justify-between gap-2 border-t border-border/30 text-xs">
              <button class="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground">
                Detail Diagnostik & Status Deploy →
              </button>
              <span class="text-[11px] text-muted-foreground">Just now</span>
            </div>

            <div class="flex items-center justify-between gap-2 pt-2 border-t border-border/40">
              <button class="min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center gap-1.5 text-muted-foreground hover:bg-muted/40">
                📋 Copy link
              </button>
              <button data-testid="task-card-terminal-btn" class="min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center gap-1.5 text-muted-foreground hover:bg-muted/40">
                <span class="text-emerald-500">&gt;_</span> Live Terminal
              </button>
            </div>
          </div>
        </div>

        <!-- Calm Placeholder for Cache Miss (Zero layout jump, matching card dimensions) -->
        <div class="w-full max-w-xl">
          <div data-card="task-card" class="rounded-xl border border-border/40 bg-card/40 p-4 text-card-foreground select-none space-y-2.5 min-h-[44px]">
            <div class="flex items-center justify-between gap-2 border-b border-border/30 pb-2">
              <div class="flex items-center gap-2">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-muted-foreground/60"></span>
                </span>
                <span class="font-mono text-xs text-muted-foreground">#7B89A2</span>
              </div>
              <span class="text-[11px] text-muted-foreground/70 font-sans">Memuat tugas…</span>
            </div>
            <div class="space-y-1.5 py-0.5">
              <div class="h-4 w-2/3 bg-muted/40 rounded-sm"></div>
              <div class="h-3 w-1/3 bg-muted/30 rounded-sm"></div>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Calm Floating Surface Composer -->
    <footer class="p-3 border-t border-border/40 bg-card/30">
      <div class="flex items-center gap-2 rounded-2xl bg-[#1e2023] px-3.5 py-2 border border-border/30">
        <input type="text" placeholder="Tulis instruksi atau pesan…" class="flex-1 bg-transparent text-sm text-foreground outline-none" />
        <button class="h-8 w-8 rounded-full bg-primary/20 text-primary flex items-center justify-center">
          ➤
        </button>
      </div>
    </footer>
  </div>
</body>
</html>`
}

async function capture() {
  const browser = await chromium.launch({
    executablePath: path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium'),
    env: {
      ...process.env,
      LD_LIBRARY_PATH:
        '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' +
        (process.env.LD_LIBRARY_PATH || ''),
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox'],
  })

  // 1. Mobile 390px
  const pageMobile = await browser.newPage({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  await pageMobile.setContent(createChatLayoutHtml())
  const mobilePath = path.join(screenshotsDir, 'calm-chat-task-layout-mobile-390px.png')
  await pageMobile.screenshot({ path: mobilePath, fullPage: false })

  // 2. Desktop
  const pageDesktop = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  })
  await pageDesktop.setContent(createChatLayoutHtml())
  const desktopPath = path.join(screenshotsDir, 'calm-chat-task-layout-desktop.png')
  await pageDesktop.screenshot({ path: desktopPath, fullPage: false })

  await browser.close()
  console.log('Calm layout screenshots captured successfully!')
}

capture().catch((err) => {
  console.error(err)
  process.exit(1)
})
