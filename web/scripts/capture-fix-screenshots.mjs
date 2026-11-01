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
  .app-viewport {
    height: 100vh;
    display: flex;
    flex-direction: column;
    background-color: #141416;
    overflow: hidden;
    position: relative;
  }
`

function createAgentRoomHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Agent Room (@sm) - TaskCard Without Draft</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport flex flex-col h-full bg-[#141416]">
    <!-- Top Header: @sm Connected -->
    <header class="h-14 shrink-0 border-b border-border/30 bg-[#18181a] px-4 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="h-8 w-8 rounded-full bg-secondary/80 flex items-center justify-center font-semibold text-xs text-foreground">
          🤖
        </div>
        <div>
          <h2 class="text-sm font-semibold text-foreground tracking-tight flex items-center gap-1.5">
            <span>@sm</span>
            <span class="inline-block h-2 w-2 rounded-full bg-emerald-500"></span>
          </h2>
          <p class="text-[11px] text-muted-foreground">Scrum Master AI • Connected</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs px-2.5 py-1 rounded-full bg-secondary/50 text-muted-foreground font-mono">cuan #01B7F8</span>
      </div>
    </header>

    <!-- Chat Message Stream -->
    <div class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4 max-w-3xl mx-auto w-full">
      <!-- User Operator Message -->
      <div class="flex flex-col items-end">
        <div class="max-w-[85%] rounded-2xl rounded-tr-xs bg-[#25272c] border border-border/20 p-3.5 text-sm text-foreground">
          Tolong eksekusi ekstraksi library publik runner untuk agent process & workspace.
          <span class="text-[10px] text-muted-foreground/70 block text-right mt-1">20:28</span>
        </div>
      </div>

      <!-- Agent Message with TaskCard (Queued, Zero Visitor Draft, Calm Design Diet) -->
      <div class="flex flex-col items-start w-full">
        <div class="w-full max-w-xl">
          <div data-card="task-card" class="rounded-xl border border-border/40 bg-card/40 p-4 text-card-foreground select-none space-y-3">
            <!-- Header: Status Pill, Duration, Project, Action Buttons -->
            <div class="flex flex-wrap items-center justify-between gap-2 border-b border-border/30 pb-2.5">
              <div class="flex items-center gap-2 min-w-0">
                <span class="inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold bg-amber-500/15 text-amber-800 dark:text-amber-200 border-amber-500/35">
                  <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                    <span class="animate-pulse absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-amber-500 dark:bg-amber-400"></span>
                  </span>
                  <span>Dalam antrean</span>
                </span>
                <span class="inline-flex items-center gap-1 text-[11px] font-mono font-medium text-muted-foreground bg-muted/60 px-2 py-0.5 rounded-md">
                  ⏱ 00:45
                </span>
                <span class="text-xs text-muted-foreground font-medium">cuan #01B7F8</span>
              </div>
              <div class="flex items-center gap-1">
                <span class="h-7 w-7 rounded-md text-muted-foreground flex items-center justify-center text-xs font-mono">&gt;_</span>
                <span class="h-7 w-7 rounded-md text-muted-foreground flex items-center justify-center text-xs">🔄</span>
              </div>
            </div>

            <!-- Title & Subtitle (Clean Calm Typography) -->
            <div class="space-y-1">
              <h4 class="text-sm font-semibold tracking-tight text-foreground break-words">
                Ekstraksi Library Publik: Agent Process &amp; Workspace Runner
              </h4>
              <p class="text-xs text-amber-600 dark:text-amber-400 font-medium">
                Waiting in worker queue...
              </p>
            </div>

            <!-- Notice: ZERO Visitor Reply Draft in Agent Room! -->

            <!-- Secondary Technical Details link -->
            <div class="pt-2 flex items-center justify-between gap-2 border-t border-border/30 text-xs">
              <span class="text-xs font-medium text-muted-foreground hover:text-foreground cursor-pointer">
                Detail Diagnostik &amp; Status Deploy →
              </span>
              <span class="text-[11px] text-muted-foreground">Baru saja</span>
            </div>

            <div class="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
              <span class="text-muted-foreground/70">Assignee:</span>
              <span class="font-medium text-foreground">@sm</span>
            </div>

            <!-- Footer: Quick actions -->
            <div class="pt-2 flex items-center justify-between gap-2 border-t border-border/30 text-xs text-muted-foreground">
              <div class="flex items-center gap-3">
                <span class="hover:text-foreground cursor-pointer">🔗 Copy link</span>
                <span class="hover:text-foreground cursor-pointer">↗ Open task</span>
              </div>
              <time class="text-[11px]">20:29</time>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</body>
</html>`
}

function createCustomerRoomHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Customer Room - TaskCompletionCard With Clean Bear Notes Draft</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport flex flex-col h-full bg-[#141416]">
    <!-- Top Header: Customer Room -->
    <header class="h-14 shrink-0 border-b border-border/30 bg-[#18181a] px-4 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="h-8 w-8 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center font-semibold text-xs">
          CS
        </div>
        <div>
          <h2 class="text-sm font-semibold text-foreground tracking-tight">Pelanggan: Budi Santoso</h2>
          <p class="text-[11px] text-muted-foreground flex items-center gap-1.5">
            <span class="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
            <span>Online • Customer Support Chat</span>
          </p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs px-2.5 py-1 rounded-full bg-primary/10 text-primary font-medium">Customer Room</span>
      </div>
    </header>

    <!-- Chat Message Stream -->
    <div class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4 max-w-3xl mx-auto w-full">
      <!-- Visitor Inquiry -->
      <div class="flex flex-col items-start">
        <div class="max-w-[85%] rounded-2xl rounded-tl-xs bg-[#1e1e22] border border-border/30 p-3.5 text-sm text-foreground">
          Halo, video episode 4 kemarin sempat buffering terus ya kak?
          <span class="text-[10px] text-muted-foreground/70 block mt-1">20:25</span>
        </div>
      </div>

      <!-- Operator Response with Completed Task Card -->
      <div class="flex flex-col items-start w-full">
        <div class="w-full max-w-xl">
          <div data-testid="task-completion-card" class="rounded-xl border border-border/40 bg-card/40 p-4 space-y-3.5 text-card-foreground">
            <!-- Header -->
            <div class="flex items-center justify-between gap-2 border-b border-border/30 pb-2.5">
              <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-emerald-500/15 text-emerald-800 dark:text-emerald-300 border border-emerald-500/30">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                <span>Selesai</span>
              </span>
              <div class="flex items-center gap-2 text-xs">
                <span class="text-xs font-mono text-muted-foreground">#VD49E1</span>
              </div>
            </div>

            <!-- Title & Summary -->
            <div class="space-y-1">
              <h4 class="text-sm font-semibold tracking-tight text-foreground">
                Optimasi Cache Streaming Video CDN
              </h4>
              <p class="text-xs text-muted-foreground leading-relaxed">
                Cache edge CDN telah di-purge dan bitrate streaming dinormalisasi.
              </p>
            </div>

            <!-- Clean Bear Notes Style Visitor Reply Draft (ZERO NESTED BOXES!) -->
            <div data-testid="task-card-draft-section" class="border-t border-border/30 pt-2.5 space-y-2">
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-1.5 font-semibold text-xs text-primary">
                  <span>✨ Draf Balasan Pengunjung</span>
                </div>
                <button
                  type="button"
                  data-testid="task-card-use-draft-btn"
                  class="min-h-7 px-2.5 py-1 text-xs font-medium rounded-lg bg-primary/10 hover:bg-primary/20 text-primary border border-primary/20 inline-flex items-center gap-1.5 cursor-pointer shadow-none"
                >
                  <span>↗ Gunakan Draf di Input</span>
                </button>
              </div>
              <p class="text-xs text-foreground/90 italic border-l-2 border-primary/40 pl-3 py-1 whitespace-pre-wrap leading-relaxed break-words">
                "Halo kak Budi, mohon maaf atas kendalanya ya. Cache streaming CDN untuk episode tersebut sudah berhasil kami optimasi dan sekarang sudah lancar diputar kembali. Silakan dicoba ya kak!"
              </p>
            </div>

            <!-- SlideDrawer link -->
            <div class="pt-2 flex items-center justify-between gap-2 border-t border-border/30 text-xs">
              <span class="text-xs font-medium text-muted-foreground hover:text-foreground cursor-pointer">
                Detail Diagnostik &amp; Status Deploy →
              </span>
              <span class="text-[11px] text-muted-foreground">2m lalu</span>
            </div>
          </div>
        </div>
      </div>
    </div>
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

  // 1. Mobile viewport (390 x 844) - Agent Room
  const pageMobileAgent = await browser.newPage({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  await pageMobileAgent.setContent(createAgentRoomHtml())
  await pageMobileAgent.screenshot({
    path: path.join(screenshotsDir, 'fix-task-card-agent-room-mobile.png'),
    fullPage: false,
  })

  // 2. Desktop viewport (1280 x 800) - Agent Room
  const pageDesktopAgent = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  })
  await pageDesktopAgent.setContent(createAgentRoomHtml())
  await pageDesktopAgent.screenshot({
    path: path.join(screenshotsDir, 'fix-task-card-agent-room-desktop.png'),
    fullPage: false,
  })

  // 3. Mobile viewport (390 x 844) - Customer Room
  const pageMobileCustomer = await browser.newPage({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  await pageMobileCustomer.setContent(createCustomerRoomHtml())
  await pageMobileCustomer.screenshot({
    path: path.join(screenshotsDir, 'fix-task-card-customer-room-mobile.png'),
    fullPage: false,
  })

  // 4. Desktop viewport (1280 x 800) - Customer Room
  const pageDesktopCustomer = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  })
  await pageDesktopCustomer.setContent(createCustomerRoomHtml())
  await pageDesktopCustomer.screenshot({
    path: path.join(screenshotsDir, 'fix-task-card-customer-room-desktop.png'),
    fullPage: false,
  })

  await browser.close()
  console.log('Screenshots captured successfully!')
}

capture().catch((err) => {
  console.error(err)
  process.exit(1)
})
