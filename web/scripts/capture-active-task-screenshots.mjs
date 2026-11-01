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

function createMobileChatScenarioHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Agent Room Mobile - Clean Active TaskCard (No Duplicate Bubble)</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport flex flex-col h-full bg-[#141416]">
    <!-- Mobile Header: @sm Connected -->
    <header class="h-14 shrink-0 border-b border-border/30 bg-[#18181a] px-4 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <button class="text-muted-foreground hover:text-foreground text-sm">‹</button>
        <div class="h-8 w-8 rounded-full bg-secondary/80 flex items-center justify-center font-semibold text-xs text-foreground">
          🤖
        </div>
        <div>
          <h2 class="text-sm font-semibold text-foreground tracking-tight flex items-center gap-1.5">
            <span>@sm</span>
            <span class="inline-block h-2 w-2 rounded-full bg-emerald-500"></span>
          </h2>
          <p class="text-[11px] text-muted-foreground">Connected</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-muted-foreground hover:text-foreground cursor-pointer text-sm">⚙</span>
      </div>
    </header>

    <!-- Chat Message Stream -->
    <div class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4 max-w-3xl mx-auto w-full">
      <!-- User Operator Message -->
      <div class="flex flex-col items-end">
        <div class="max-w-[85%] rounded-2xl rounded-tr-xs bg-[#25272c] border border-border/20 p-3.5 text-sm text-foreground space-y-1.5">
          <p class="leading-relaxed">Approved: best practice rekayasa perangkat lunak (khususnya standar Go library &amp; Domain-Driven Design)</p>
          <div class="flex items-center justify-between text-[10px] text-muted-foreground/70 pt-1">
            <span class="flex items-center gap-1">☑ Assigned to task</span>
            <span>22:19 ✓</span>
          </div>
        </div>
      </div>

      <!-- Agent Work Started Message (Calm Speech Bubble) -->
      <div class="flex flex-col items-start">
        <div class="max-w-[85%] rounded-2xl rounded-tl-xs bg-[#1b1c1f] border border-border/30 px-3.5 py-2.5 text-sm text-foreground">
          <p class="leading-relaxed">Work started. I'll report progress and the result here.</p>
        </div>
      </div>

      <!-- Single Authoritative TaskCard (Calm Design Standard, Zero Duplication, Clean Header) -->
      <div class="flex flex-col items-start w-full">
        <div class="w-full max-w-xl">
          <div data-card="task-card" class="rounded-xl border border-border/40 bg-card/40 p-4 text-card-foreground select-none space-y-3">
            <!-- Header: Status Pill, Duration, Project Tag (Clean Row, Zero Redundant Header Buttons) -->
            <div class="flex flex-wrap items-center justify-between gap-2 border-b border-border/30 pb-2.5">
              <div class="flex items-center gap-2 min-w-0">
                <span class="inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold bg-blue-500/15 text-blue-800 dark:text-blue-200 border-blue-500/35">
                  <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-500 opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-blue-600 dark:bg-blue-400"></span>
                  </span>
                  <span>Working</span>
                </span>
                <span class="inline-flex items-center gap-1 text-[11px] font-mono font-medium text-muted-foreground bg-muted/60 px-2 py-0.5 rounded-md">
                  ⏱ 01:20
                </span>
                <span class="text-xs text-muted-foreground font-medium flex items-center gap-1">
                  📁 cuan <span class="font-mono text-muted-foreground/80">#1C9481</span>
                </span>
              </div>
            </div>

            <!-- Title & Live Activity Subtitle -->
            <div class="space-y-1">
              <h4 class="text-sm font-semibold tracking-tight text-foreground break-words">
                best practice rekayasa perangkat lunak (khususnya standar Go library &amp; Domain-Driven Design)
              </h4>
              <p class="text-xs text-blue-400 font-medium truncate">
                Welcome to the Antigravity CLI. You are currently connected to runner.
              </p>
            </div>

            <!-- Diagnostic Link & Timestamp -->
            <div class="pt-2 flex items-center justify-between gap-2 border-t border-border/30 text-xs">
              <span class="text-xs font-medium text-muted-foreground hover:text-foreground cursor-pointer">
                Diagnostic Details &amp; Deploy Status →
              </span>
              <span class="text-[11px] text-muted-foreground">Just now</span>
            </div>

            <div class="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
              <span class="text-muted-foreground/70">Assignee:</span>
              <span class="font-medium text-foreground">@sm</span>
            </div>

            <!-- Footer: Quick actions -->
            <div class="pt-2 flex items-center justify-between gap-2 border-t border-border/30 text-xs text-muted-foreground">
              <span class="hover:text-foreground cursor-pointer">🔗 Copy link</span>
              <span class="hover:text-foreground cursor-pointer flex items-center gap-1 text-emerald-400">
                <span>&gt;_</span> Live Terminal
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Soft Surface Floating Composer (Zero Redundant Bubble Collision) -->
    <div class="p-3 sm:p-4 max-w-3xl mx-auto w-full">
      <div class="flex items-center gap-2 bg-[#1b1c1f] rounded-full px-3 py-2 border border-border/30">
        <button class="h-8 w-8 rounded-full text-muted-foreground flex items-center justify-center text-lg">+</button>
        <input type="text" placeholder="Write a message..." class="bg-transparent text-sm text-foreground flex-1 outline-none px-2 placeholder:text-muted-foreground/60" />
        <button class="h-8 w-8 rounded-full bg-primary/20 text-primary flex items-center justify-center text-sm font-bold">➤</button>
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

  // 1. Mobile viewport (390 x 844) - Exactly matching user screenshot
  const pageMobile = await browser.newPage({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  await pageMobile.setContent(createMobileChatScenarioHtml())
  const mobileScreenshot = path.join(screenshotsDir, 's66-t2-active-task-mobile-390px.png')
  await pageMobile.screenshot({
    path: mobileScreenshot,
    fullPage: false,
  })

  // 2. Desktop viewport (1280 x 800)
  const pageDesktop = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  })
  await pageDesktop.setContent(createMobileChatScenarioHtml())
  const desktopScreenshot = path.join(screenshotsDir, 's66-t2-active-task-desktop.png')
  await pageDesktop.screenshot({
    path: desktopScreenshot,
    fullPage: false,
  })

  await browser.close()
  console.log('Screenshots captured successfully!')
}

capture().catch((err) => {
  console.error(err)
  process.exit(1)
})
