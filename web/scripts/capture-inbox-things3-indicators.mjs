import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')

const artifactsDir = process.env.CUAN_TASK_ARTIFACTS_DIR || path.join(repoRoot, 'screenshots')
if (!fs.existsSync(artifactsDir)) {
  fs.mkdirSync(artifactsDir, { recursive: true })
}

const require = createRequire(import.meta.url)
const playwright = require(process.env.CUAN_BROWSER_MODULE || 'playwright-core')
const { chromium } = playwright

// Find compiled CSS
const distAssetsDir = path.join(webDir, 'dist/assets')
const cssFiles = fs.readdirSync(distAssetsDir).filter((f) => f.endsWith('.css'))
const mainCssFile = cssFiles.find((f) => f.startsWith('index-') || f.startsWith('style-')) || cssFiles[0]
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
    font-family: 'Geist Variable', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
`

function createInboxHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Inbox - Things 3 Calm Dynamic Task Indicators</title>
  <style>${baseStyles}</style>
</head>
<body class="dark bg-background text-foreground min-h-screen">
  <div class="max-w-2xl mx-auto min-h-screen flex flex-col bg-background border-x border-border/40">
    <!-- Header matching shadcn Mail h-13 -->
    <header class="h-13 shrink-0 border-b border-border/40 px-4 flex items-center justify-between bg-background sticky top-0 z-20">
      <div class="flex items-center gap-2">
        <span class="font-bold text-base tracking-tight">Cuan</span>
        <span class="text-muted-foreground/60">/</span>
        <h1 class="text-sm font-semibold">Inbox</h1>
      </div>
      <div class="flex items-center gap-1">
        <button type="button" class="inline-flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-accent/50 transition-colors">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15"/></svg>
        </button>
      </div>
    </header>

    <!-- Search & Filter Controls -->
    <div class="px-4 pt-3 pb-2 space-y-2 border-b border-border/40">
      <div class="relative w-full">
        <svg class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
        <input class="w-full h-9 rounded-md border border-border/50 bg-secondary/30 pl-9 pr-4 text-xs placeholder:text-muted-foreground focus:outline-none" placeholder="Search conversations…" value="" />
      </div>
      <div class="flex items-center gap-1">
        <button class="px-2.5 py-1 text-xs font-medium rounded-md border-b-2 border-foreground text-foreground">Semua</button>
        <button class="px-2.5 py-1 text-xs font-medium rounded-md text-muted-foreground hover:text-foreground">Asisten</button>
        <button class="px-2.5 py-1 text-xs font-medium rounded-md text-muted-foreground hover:text-foreground">Pelanggan</button>
      </div>
    </div>

    <!-- Conversation List with Things 3 Dynamic Indicators -->
    <div class="flex-1 divide-y divide-border/40 overflow-y-auto">

      <!-- ITEM 1: Task Tunggal Berjalan (Single running task: outline circle with subtle pulse) -->
      <div class="relative inbox-item-row hover:bg-accent/40 transition-colors">
        <button type="button" class="inbox-contact-row flex items-start gap-3 w-full min-h-[4.5rem] p-3 text-left border-0 bg-transparent">
          <div class="relative shrink-0 mt-0.5">
            <div class="h-9 w-9 rounded-full bg-secondary/80 flex items-center justify-center font-bold text-xs border border-border/50 text-foreground overflow-hidden">
              <svg class="h-4 w-4 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M12 8v8m-4-4h8"/></svg>
            </div>
          </div>
          <div class="flex-1 min-w-0 flex flex-col gap-1">
            <div class="flex items-center justify-between gap-1.5 pr-9">
              <div class="flex items-center gap-1.5 min-w-0">
                <span class="text-sm font-semibold text-foreground truncate">
                  Kerjakan mandiri file standalone cleanup-workflow.md lalu lampirkan hasilnya. I...
                </span>
              </div>
              <span class="text-xs shrink-0 tabular-nums text-muted-foreground">11:55</span>
            </div>
            <p class="text-sm text-muted-foreground dark:text-zinc-400 line-clamp-1 leading-normal">
              Siap, langsung ane delegasikan pembuatan file standalone cleanup-workflow.md ke task runner....
            </p>
          </div>
        </button>
        <!-- Dynamic Right Slot: Single Running Task Outline Circle with Subtle Pulse -->
        <div class="absolute right-0 top-0 z-10">
          <button type="button" data-task-progress-trigger data-task-progress="partial" aria-label="1 task in progress"
            class="inline-flex h-11 w-11 shrink-0 items-center justify-center bg-transparent text-[color:var(--status-working)] focus-visible:outline">
            <span aria-hidden="true" class="inline-flex h-4 w-4">
              <svg viewBox="0 0 16 16" class="h-full w-full task-pulse-subtle">
                <circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" stroke-width="1.5" class="opacity-80"/>
              </svg>
            </span>
          </button>
        </div>
      </div>

      <!-- ITEM 2: Beberapa Task (Ada yang Aktif) (Multi-task with 1 of 3 complete and 1 active: 1/3 pie SVG with subtle pulse) -->
      <div class="relative inbox-item-row hover:bg-accent/40 transition-colors">
        <button type="button" class="inbox-contact-row flex items-start gap-3 w-full min-h-[4.5rem] p-3 text-left border-0 bg-transparent">
          <div class="relative shrink-0 mt-0.5">
            <div class="h-9 w-9 rounded-full bg-secondary/80 flex items-center justify-center font-bold text-xs border border-border/50 text-foreground overflow-hidden">
              <svg class="h-4 w-4 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M9 12l2 2 4-4"/></svg>
            </div>
          </div>
          <div class="flex-1 min-w-0 flex flex-col gap-1">
            <div class="flex items-center justify-between gap-1.5 pr-9">
              <div class="flex items-center gap-1.5 min-w-0">
                <span class="text-sm font-medium text-foreground/90 truncate">
                  Rencana Arsitektur &amp; Sinkronisasi
                </span>
              </div>
              <span class="text-xs shrink-0 tabular-nums text-muted-foreground">10:45</span>
            </div>
            <p class="text-sm text-muted-foreground dark:text-zinc-400 line-clamp-1 leading-normal">
              Sprint Plan: 1 dari 3 tugas selesai, integrasi runner sedang dikerjakan...
            </p>
          </div>
        </button>
        <!-- Dynamic Right Slot: 1/3 Pie Progress SVG with Subtle Pulse -->
        <div class="absolute right-0 top-0 z-10">
          <button type="button" data-task-progress-trigger data-task-progress="partial" aria-label="1 of 3 tasks completed"
            class="inline-flex h-11 w-11 shrink-0 items-center justify-center bg-transparent text-[color:var(--status-working)] focus-visible:outline">
            <span aria-hidden="true" class="inline-flex h-4 w-4">
              <svg viewBox="0 0 16 16" class="h-full w-full task-pulse-subtle">
                <circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.65"/>
                <!-- 1/3 arc pie slice (approx 120deg: x=11.897, y=10.25) -->
                <path d="M 8 8 L 8 3.5 A 4.5 4.5 0 0 1 11.897 10.25 Z" fill="currentColor"/>
              </svg>
            </span>
          </button>
        </div>
      </div>

      <!-- ITEM 3: Semua Task Selesai (All tasks completed: static green checkmark CheckCircle2) -->
      <div class="relative inbox-item-row hover:bg-accent/40 transition-colors">
        <button type="button" class="inbox-contact-row flex items-start gap-3 w-full min-h-[4.5rem] p-3 text-left border-0 bg-transparent">
          <div class="relative shrink-0 mt-0.5">
            <div class="h-9 w-9 rounded-full bg-secondary/80 flex items-center justify-center font-bold text-xs border border-border/50 text-foreground overflow-hidden">
              <svg class="h-4 w-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
            </div>
          </div>
          <div class="flex-1 min-w-0 flex flex-col gap-1">
            <div class="flex items-center justify-between gap-1.5 pr-9">
              <div class="flex items-center gap-1.5 min-w-0">
                <span class="text-sm font-medium text-foreground/90 truncate">Winz 8</span>
              </div>
              <span class="text-xs shrink-0 tabular-nums text-muted-foreground">10:28</span>
            </div>
            <p class="text-sm text-muted-foreground dark:text-zinc-400 line-clamp-1 leading-normal">
              Halo Mas, Ane sudah bantu cek di sistem. Akun winz_8@ymail.com saat ini sudah terdaftar aktif...
            </p>
          </div>
        </button>
        <!-- Dynamic Right Slot: Static Green CheckCircle2 -->
        <div class="absolute right-0 top-0 z-10">
          <button type="button" data-task-progress-trigger data-task-progress="complete" aria-label="All tasks completed"
            class="inline-flex h-11 w-11 shrink-0 items-center justify-center bg-transparent text-[color:var(--status-success)] focus-visible:outline">
            <span aria-hidden="true" class="inline-flex h-4 w-4">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" class="h-full w-full">
                <circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>
              </svg>
            </span>
          </button>
        </div>
      </div>

      <!-- ITEM 4: Semua Task Selesai (Completed standalone task: static green checkmark CheckCircle2) -->
      <div class="relative inbox-item-row hover:bg-accent/40 transition-colors">
        <button type="button" class="inbox-contact-row flex items-start gap-3 w-full min-h-[4.5rem] p-3 text-left border-0 bg-transparent">
          <div class="relative shrink-0 mt-0.5">
            <div class="h-9 w-9 rounded-full bg-secondary/80 flex items-center justify-center font-bold text-xs border border-border/50 text-foreground overflow-hidden">
              <svg class="h-4 w-4 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M12 8v8m-4-4h8"/></svg>
            </div>
          </div>
          <div class="flex-1 min-w-0 flex flex-col gap-1">
            <div class="flex items-center justify-between gap-1.5 pr-9">
              <div class="flex items-center gap-1.5 min-w-0">
                <span class="text-sm font-medium text-foreground/90 truncate">
                  Kerjakan mandiri file standalone cleanup-workflow.md lalu lampirkan hasilnya. I...
                </span>
              </div>
              <span class="text-xs shrink-0 tabular-nums text-muted-foreground">09:55</span>
            </div>
            <p class="text-sm text-muted-foreground dark:text-zinc-400 line-clamp-1 leading-normal">
              Berkas cleanup-workflow.md sudah siap di ruang artefak privat dan terlampir sebagai unduhan....
            </p>
          </div>
        </button>
        <!-- Dynamic Right Slot: Static Green CheckCircle2 -->
        <div class="absolute right-0 top-0 z-10">
          <button type="button" data-task-progress-trigger data-task-progress="complete" aria-label="All tasks completed"
            class="inline-flex h-11 w-11 shrink-0 items-center justify-center bg-transparent text-[color:var(--status-success)] focus-visible:outline">
            <span aria-hidden="true" class="inline-flex h-4 w-4">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" class="h-full w-full">
                <circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>
              </svg>
            </span>
          </button>
        </div>
      </div>

      <!-- ITEM 5: Tanpa Task (No task: clean slot without icon, natural right alignment) -->
      <div class="relative inbox-item-row hover:bg-accent/40 transition-colors">
        <button type="button" class="inbox-contact-row flex items-start gap-3 w-full min-h-[4.5rem] p-3 text-left border-0 bg-transparent">
          <div class="relative shrink-0 mt-0.5">
            <div class="h-9 w-9 rounded-full bg-secondary/80 flex items-center justify-center font-bold text-xs border border-border/50 text-foreground overflow-hidden">
              <svg class="h-4 w-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
            </div>
          </div>
          <div class="flex-1 min-w-0 flex flex-col gap-1">
            <div class="flex items-center justify-between gap-1.5">
              <div class="flex items-center gap-1.5 min-w-0">
                <span class="text-sm font-medium text-foreground/90 truncate">Defri Putra</span>
              </div>
              <span class="text-xs shrink-0 tabular-nums text-muted-foreground">Yesterday</span>
            </div>
            <p class="text-sm text-muted-foreground dark:text-zinc-400 line-clamp-1 leading-normal">
              Iya kayake sob, aman semua.
            </p>
          </div>
        </button>
        <!-- No Task: Slot remains completely clean without icon -->
      </div>

    </div>
  </div>
</body>
</html>`
}

async function run() {
  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu'],
  })

  // 1. Mobile Viewport (390px x 844px)
  const mobileContext = await browser.newContext({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  const mobilePage = await mobileContext.newPage()
  await mobilePage.setContent(createInboxHtml(), { waitUntil: 'load' })
  await mobilePage.waitForTimeout(300)
  const mobilePath = path.join(artifactsDir, 'inbox-things3-mobile-390px.png')
  await mobilePage.screenshot({ path: mobilePath, fullPage: true })
  console.log('Saved mobile screenshot:', mobilePath)

  // 2. Desktop Viewport (1280px x 800px)
  const desktopContext = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  })
  const desktopPage = await desktopContext.newPage()
  await desktopPage.setContent(createInboxHtml(), { waitUntil: 'load' })
  await desktopPage.waitForTimeout(300)
  const desktopPath = path.join(artifactsDir, 'inbox-things3-desktop.png')
  await desktopPage.screenshot({ path: desktopPath, fullPage: true })
  console.log('Saved desktop screenshot:', desktopPath)

  await browser.close()
  console.log('Visual verification screenshots completed successfully.')
}

run().catch((err) => {
  console.error(err)
  process.exit(1)
})
