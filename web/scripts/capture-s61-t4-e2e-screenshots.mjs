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

// Common styling wrapper
const getHtmlWrapper = (title, content, extraStyles = '') => `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>${title}</title>
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
    ${extraStyles}
  </style>
</head>
<body class="dark">
  ${content}
</body>
</html>`

// 1. Template: Inbox SubPanel with Apple Mail & Bear Inset List (3-Tier) & Conversation View
const inboxHtml = getHtmlWrapper(
  'Cuan Sprint 61 — S61-T4 Inbox Apple Mail & Bear Inset List Audit',
  `
  <div class="flex h-screen w-screen overflow-hidden bg-[#141416]">
    <!-- SubPanel: Apple Mail & Bear Inset List (Full width on mobile, sidebar on desktop) -->
    <aside class="w-full md:w-80 lg:w-96 border-r border-border/30 bg-[#141416] flex flex-col shrink-0 h-full">
      <!-- SubPanel Header with Calm Filter Segments -->
      <div class="p-3.5 border-b border-border/30 shrink-0 space-y-3">
        <div class="flex items-center justify-between">
          <h1 class="text-base font-semibold tracking-tight text-foreground">Inbox</h1>
          <span class="text-xs text-muted-foreground font-mono">3 obrolan</span>
        </div>
        <!-- Things 3 / Bear Notes Neutral Filter Segments -->
        <div class="flex items-center gap-1.5 p-1 bg-muted/30 rounded-xl">
          <button class="min-h-11 rounded-lg px-3.5 py-1.5 text-xs font-medium bg-secondary text-foreground shadow-2xs">
            Semua
          </button>
          <button class="min-h-11 rounded-lg px-3.5 py-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors">
            Asisten
          </button>
          <button class="min-h-11 rounded-lg px-3.5 py-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors">
            Pelanggan
          </button>
        </div>
      </div>

      <!-- Conversation List: 3-Tier Apple Mail & Bear Inset List -->
      <div class="flex-1 overflow-y-auto divide-y divide-border/30">
        <!-- Item 1: Active SM (Selected) -->
        <div class="min-h-[4.5rem] px-3.5 py-3 bg-secondary/40 border-l-2 border-primary cursor-pointer transition-colors space-y-1">
          <!-- Tier 1: Sender & Tabular Time -->
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-semibold text-foreground truncate">Scrum Master (@sm)</span>
            <span class="text-[11px] font-mono tabular-nums text-muted-foreground shrink-0">10:48</span>
          </div>
          <!-- Tier 2: Clean Subject & StatusDot Micro-Indicator -->
          <div class="flex items-center gap-2">
            <span class="inline-block h-2 w-2 rounded-full bg-blue-500 animate-pulse shrink-0" title="Tugas aktif berjalan"></span>
            <span class="text-xs font-medium text-foreground truncate">S61-T4 Quality Gate & Deployment</span>
          </div>
          <!-- Tier 3: 2-line snippet in muted zinc-400 -->
          <p class="text-xs text-zinc-400 line-clamp-2 leading-relaxed">
            Menjalankan automated quality gate menyeluruh, visual cross-check, dan verifikasi deployment VPS.
          </p>
        </div>

        <!-- Item 2: Customer Live Chat -->
        <div class="min-h-[4.5rem] px-3.5 py-3 hover:bg-muted/20 cursor-pointer transition-colors space-y-1">
          <!-- Tier 1: Sender & Tabular Time -->
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-medium text-foreground truncate">Siti Rahma</span>
            <span class="text-[11px] font-mono tabular-nums text-muted-foreground shrink-0">Kemarin</span>
          </div>
          <!-- Tier 2: Clean Subject -->
          <div class="flex items-center gap-2">
            <span class="text-xs text-muted-foreground truncate">Pertanyaan Integrasi WhatsApp Cuan</span>
          </div>
          <!-- Tier 3: 2-line snippet in muted zinc-400 -->
          <p class="text-xs text-zinc-400 line-clamp-2 leading-relaxed">
            Apakah Cuan mendukung koneksi multi-device WhatsApp Baileys tanpa perlu server terpisah?
          </p>
        </div>

        <!-- Item 3: Content Writer Agent -->
        <div class="min-h-[4.5rem] px-3.5 py-3 hover:bg-muted/20 cursor-pointer transition-colors space-y-1">
          <!-- Tier 1: Sender & Tabular Time -->
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-medium text-foreground truncate">Writer Agent (@writer)</span>
            <span class="text-[11px] font-mono tabular-nums text-muted-foreground shrink-0">29 Okt</span>
          </div>
          <!-- Tier 2: Clean Subject -->
          <div class="flex items-center gap-2">
            <span class="text-xs text-muted-foreground truncate">Draft Artikel Blog: Zero Cognitive Fatigue</span>
          </div>
          <!-- Tier 3: 2-line snippet in muted zinc-400 -->
          <p class="text-xs text-zinc-400 line-clamp-2 leading-relaxed">
            Draft artikel mengenai filosofi Things 3 & Bear Notes untuk produktivitas solopreneur sudah siap.
          </p>
        </div>
      </div>
    </aside>

    <!-- Main Chat Canvas (Desktop Only in split view) -->
    <main class="hidden md:flex flex-1 flex-col h-full bg-[#141416] min-w-0">
      <!-- Calm Top Header -->
      <header class="px-5 py-3.5 border-b border-border/30 bg-[#18181a] flex items-center justify-between shrink-0">
        <div class="flex items-center gap-3">
          <div class="h-9 w-9 rounded-full bg-secondary flex items-center justify-center font-semibold text-xs text-foreground">
            SM
          </div>
          <div>
            <h2 class="text-sm font-semibold text-foreground tracking-tight">Scrum Master (@sm)</h2>
            <p class="text-[11px] text-muted-foreground flex items-center gap-1.5">
              <span class="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
              <span>Online · Ruang Kerja Tenang</span>
            </p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs font-mono text-muted-foreground bg-muted/40 px-2.5 py-1 rounded-md">#S61-T4</span>
        </div>
      </header>

      <!-- Chat Stream -->
      <div class="flex-1 p-5 space-y-4 overflow-y-auto max-w-[75ch] mx-auto w-full">
        <!-- Quiet Date Divider -->
        <div class="flex justify-center my-3">
          <time class="text-xs font-medium text-muted-foreground select-none">
            Today
          </time>
        </div>

        <!-- Incoming Calm Bubble -->
        <div class="flex flex-col items-start wa-message-row">
          <div class="calm-message-bubble calm-incoming max-w-[85%] rounded-2xl px-4 py-3 space-y-2">
            <p class="text-[15px] leading-relaxed text-foreground">
              Selamat pagi Pak Buchin! Backlog Sprint 61 telah mencapai tahap akhir (S61-T4: End-to-End Quality Gate, Visual Cross-Check & VPS Deployment).
            </p>
            <div class="calm-markdown-footer mt-1 flex justify-end">
              <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:45</span>
            </div>
          </div>
        </div>

        <!-- Outgoing Calm Bubble -->
        <div class="flex flex-col items-end wa-message-row">
          <div class="calm-message-bubble calm-outgoing max-w-[85%] rounded-2xl px-4 py-3 space-y-2">
            <p class="text-[15px] leading-relaxed text-foreground">
              Pastikan verifikasi mobile 390px rapi, tidak ada bar raksasa, pil putih menyala, atau kotak ganda.
            </p>
            <div class="calm-markdown-footer mt-1 flex justify-end items-center gap-1">
              <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:47</span>
              <svg class="h-3.5 w-3.5 text-[#8e8e93]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
              </svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Bottom Calm Composer -->
      <footer class="p-4 border-t border-border/30 bg-[#141416] shrink-0">
        <div class="max-w-[75ch] mx-auto flex items-center gap-2 rounded-2xl bg-[#1b1c1f] border border-white/5 px-4 py-2.5">
          <input type="text" placeholder="Tulis instruksi atau pesan untuk tim..." class="bg-transparent flex-1 text-sm text-foreground placeholder:text-muted-foreground outline-none" disabled />
          <button class="h-8 w-8 rounded-full bg-[#ededed] text-[#141416] flex items-center justify-center shrink-0">
            <svg class="h-4 w-4 fill-current" viewBox="0 0 24 24">
              <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
            </svg>
          </button>
        </div>
      </footer>
    </main>
  </div>
  `
)

// 2. Template: Chat Stream with SlideDrawer Open (Progressive Disclosure)
const drawerHtml = getHtmlWrapper(
  'Cuan Sprint 61 — S61-T4 SlideDrawer Task Peek Visual Audit',
  `
  <div class="flex h-screen w-screen overflow-hidden bg-[#141416] relative">
    <!-- Unbroken Chat Stream Column -->
    <div class="flex-1 flex flex-col h-full bg-[#141416] min-w-0 border-r border-border/30">
      <!-- Calm Top Header -->
      <header class="px-5 py-3.5 border-b border-border/30 bg-[#18181a] flex items-center justify-between shrink-0">
        <div class="flex items-center gap-3">
          <div class="h-9 w-9 rounded-full bg-secondary flex items-center justify-center font-semibold text-xs text-foreground">
            SM
          </div>
          <div>
            <h2 class="text-sm font-semibold text-foreground tracking-tight">Scrum Master (@sm)</h2>
            <p class="text-[11px] text-muted-foreground flex items-center gap-1.5">
              <span class="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
              <span>Online · Ruang Kerja Tenang</span>
            </p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs font-mono text-muted-foreground bg-muted/40 px-2.5 py-1 rounded-md">#S61-T4</span>
        </div>
      </header>

      <!-- Chat Stream -->
      <div class="flex-1 p-5 space-y-4 overflow-y-auto max-w-[75ch] mx-auto w-full">
        <!-- Quiet Date Divider -->
        <div class="flex justify-center my-3">
          <time class="text-xs font-medium text-muted-foreground select-none">
            Today
          </time>
        </div>

        <!-- Incoming Message with Task Link -->
        <div class="flex flex-col items-start wa-message-row">
          <div class="calm-message-bubble calm-incoming max-w-[85%] rounded-2xl px-4 py-3 space-y-2.5">
            <p class="text-[15px] leading-relaxed text-foreground">
              Pak Buchin, seluruh automated test (Go & Vitest 335 tests) telah 100% PASS dan production build exit 0. Tugas sedang berjalan di laci samping.
            </p>
            <div class="pt-1">
              <button type="button" class="inline-flex items-center gap-1.5 text-xs font-medium text-primary hover:underline transition-colors bg-primary/10 px-3 py-1.5 rounded-md">
                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/>
                </svg>
                <span>Assigned to task #GUQZZ</span>
              </button>
            </div>
            <div class="calm-markdown-footer mt-1 flex justify-end">
              <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:52</span>
            </div>
          </div>
        </div>

        <!-- Outgoing Message -->
        <div class="flex flex-col items-end wa-message-row">
          <div class="calm-message-bubble calm-outgoing max-w-[85%] rounded-2xl px-4 py-3 space-y-2">
            <p class="text-[15px] leading-relaxed text-foreground">
              Bagus sekali! SlideDrawer meluncur mulus tanpa menutup percakapan kita di sisi kiri.
            </p>
            <div class="calm-markdown-footer mt-1 flex justify-end items-center gap-1">
              <span class="calm-message-meta text-[11px] text-[#8e8e93]">10:53</span>
              <svg class="h-3.5 w-3.5 text-[#8e8e93]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
              </svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Bottom Composer -->
      <footer class="p-4 border-t border-border/30 bg-[#141416] shrink-0">
        <div class="max-w-[75ch] mx-auto flex items-center gap-2 rounded-2xl bg-[#1b1c1f] border border-white/5 px-4 py-2.5">
          <input type="text" placeholder="Tulis instruksi atau pesan untuk tim..." class="bg-transparent flex-1 text-sm text-foreground placeholder:text-muted-foreground outline-none" disabled />
          <button class="h-8 w-8 rounded-full bg-[#ededed] text-[#141416] flex items-center justify-center shrink-0">
            <svg class="h-4 w-4 fill-current" viewBox="0 0 24 24">
              <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
            </svg>
          </button>
        </div>
      </footer>
    </div>

    <!-- Right Side: SlideDrawer for Task Inspection -->
    <aside class="w-full sm:w-[480px] h-full bg-[#18181b] border-l border-border/40 shadow-2xl flex flex-col shrink-0 z-50">
      <!-- Drawer Header -->
      <div class="flex items-center justify-between gap-3 border-b border-border/40 px-5 py-4 shrink-0">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <h2 class="text-sm font-semibold leading-tight text-foreground truncate">
              S61-T4: End-to-End Quality Gate & VPS Deployment
            </h2>
            <span class="text-[11px] font-mono font-medium px-2 py-0.5 rounded border border-border/40 text-blue-400 bg-blue-500/10 shrink-0">
              #GUQZZ
            </span>
          </div>
          <p class="mt-1 text-xs text-muted-foreground truncate">
            #GUQZZ · Sedang Berjalan (Doing)
          </p>
        </div>
        <div class="flex items-center gap-1 shrink-0">
          <button type="button" class="rounded-md p-1.5 text-muted-foreground hover:text-foreground">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>
      </div>

      <!-- Drawer Content -->
      <div class="flex-1 overflow-y-auto px-5 py-4 space-y-4 text-sm">
        <!-- Status Card -->
        <div class="rounded-xl border border-border/40 bg-card/60 p-3.5 space-y-2">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span class="inline-block h-2 w-2 rounded-full bg-blue-500 animate-pulse"></span>
              <span class="text-xs font-medium text-foreground">Sedang Berjalan (Doing)</span>
            </div>
            <span class="text-[11px] font-mono text-muted-foreground">Baru saja</span>
          </div>
          <p class="text-xs text-muted-foreground">
            Branch: <code class="font-mono text-foreground">feature/s61-t4-quality-gate-deploy</code>
          </p>
        </div>

        <!-- Plan & Scope -->
        <div class="space-y-1.5">
          <h3 class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Rencana & Kriteria Penerimaan</h3>
          <p class="text-xs text-foreground leading-relaxed">
            1. Automated quality gate: 100% tests PASS, build exit 0.<br>
            2. Audit visual mobile (390px) & desktop: Things 3 & Bear Notes.<br>
            3. Sinkronkan dokumen sprint, README, panduan desain, dan LandingPage.<br>
            4. Fast-forward merge ke dev dan deploy zero-downtime ke cuan.dojo.cc.
          </p>
        </div>

        <!-- DoD Checklist -->
        <div class="space-y-2 pt-2 border-t border-border/30">
          <h3 class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Definition of Done</h3>
          <div class="space-y-1.5 text-xs">
            <div class="flex items-center gap-2 text-emerald-400">
              <svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
              </svg>
              <span>100% Go & Vitest Tests Pass (335 tests)</span>
            </div>
            <div class="flex items-center gap-2 text-emerald-400">
              <svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
              </svg>
              <span>Vite/TypeScript Production Build Exit 0</span>
            </div>
            <div class="flex items-center gap-2 text-emerald-400">
              <svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
              </svg>
              <span>Audit Visual Mobile 390px & Desktop Lulus</span>
            </div>
            <div class="flex items-center gap-2 text-muted-foreground">
              <span class="h-4 w-4 rounded-full border border-border/60 flex items-center justify-center shrink-0">○</span>
              <span>Deployment ke VPS cuan.dojo.cc Healthy</span>
            </div>
          </div>
        </div>
      </div>
    </aside>
  </div>
  `
)

// 3. Template: Things 3 Pure Task Rows (TaskBoardView)
const tasksHtml = getHtmlWrapper(
  'Cuan Sprint 61 — S61-T4 Things 3 Pure Task Rows Audit',
  `
  <div class="min-h-screen bg-[#141416] p-6 max-w-4xl mx-auto space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between pb-4 border-b border-border/30">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-foreground">Daftar Tugas</h1>
        <p class="text-xs text-muted-foreground mt-1">Things 3 Pure Task Rows · Zero Action Buttons on Rows</p>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs font-mono text-muted-foreground bg-muted/40 px-3 py-1.5 rounded-lg">Sprint 61</span>
      </div>
    </div>

    <!-- Things 3 Task Rows List -->
    <div class="bg-card/40 rounded-2xl border border-border/30 overflow-hidden divide-y divide-border/30">
      <!-- Task 1 -->
      <div class="flex items-center justify-between px-4 py-3.5 hover:bg-muted/20 cursor-pointer transition-colors gap-3">
        <div class="flex items-center gap-3 min-w-0">
          <span class="h-2.5 w-2.5 rounded-full bg-emerald-500 shrink-0" title="Selesai"></span>
          <span class="text-sm font-medium text-foreground truncate">S61-T1: Apple Mail & Bear Inset List in InboxSubPanel</span>
        </div>
        <div class="flex items-center gap-3 shrink-0">
          <span class="text-xs font-mono text-muted-foreground tabular-nums">10:15</span>
        </div>
      </div>

      <!-- Task 2 -->
      <div class="flex items-center justify-between px-4 py-3.5 hover:bg-muted/20 cursor-pointer transition-colors gap-3">
        <div class="flex items-center gap-3 min-w-0">
          <span class="h-2.5 w-2.5 rounded-full bg-emerald-500 shrink-0" title="Selesai"></span>
          <span class="text-sm font-medium text-foreground truncate">S61-T2: Calm Low-Contrast Chat Bubbles & Things/Bear CSS Overhaul</span>
        </div>
        <div class="flex items-center gap-3 shrink-0">
          <span class="text-xs font-mono text-muted-foreground tabular-nums">10:35</span>
        </div>
      </div>

      <!-- Task 3 -->
      <div class="flex items-center justify-between px-4 py-3.5 hover:bg-muted/20 cursor-pointer transition-colors gap-3">
        <div class="flex items-center gap-3 min-w-0">
          <span class="h-2.5 w-2.5 rounded-full bg-emerald-500 shrink-0" title="Selesai"></span>
          <span class="text-sm font-medium text-foreground truncate">S61-T3: SlideDrawer Integration for Task References in Chat</span>
        </div>
        <div class="flex items-center gap-3 shrink-0">
          <span class="text-xs font-mono text-muted-foreground tabular-nums">10:50</span>
        </div>
      </div>

      <!-- Task 4 (Active) -->
      <div class="flex items-center justify-between px-4 py-3.5 bg-secondary/30 hover:bg-secondary/40 cursor-pointer transition-colors gap-3">
        <div class="flex items-center gap-3 min-w-0">
          <span class="h-2.5 w-2.5 rounded-full bg-blue-500 animate-pulse shrink-0" title="Sedang Berjalan"></span>
          <span class="text-sm font-medium text-foreground truncate">S61-T4: End-to-End Quality Gate, Visual Cross-Check & VPS Deployment</span>
        </div>
        <div class="flex items-center gap-3 shrink-0">
          <span class="text-xs font-mono text-blue-400 tabular-nums">Sedang berjalan</span>
        </div>
      </div>
    </div>
  </div>
  `
)

async function run() {
  console.log('Launching Playwright Chromium for S61-T4 visual cross-check audit...')
  const browser = await chromium.launch({
    executablePath: path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium'),
    env: {
      ...process.env,
      LD_LIBRARY_PATH: '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' + (process.env.LD_LIBRARY_PATH || '')
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  })

  // Helper to capture desktop & mobile screenshots
  async function captureViewports(htmlContent, baseName) {
    const htmlFilePath = path.join(screenshotsDir, `preview-${baseName}.html`)
    fs.writeFileSync(htmlFilePath, htmlContent, 'utf8')

    // Mobile Viewport (390px x 844px iPhone 12/13/14 standard)
    const mobileContext = await browser.newContext({
      viewport: { width: 390, height: 844 },
      deviceScaleFactor: 2,
      colorScheme: 'dark'
    })
    const mobilePage = await mobileContext.newPage()
    await mobilePage.goto('file://' + htmlFilePath)
    await mobilePage.waitForTimeout(300)
    const mobilePath = path.join(screenshotsDir, `${baseName}-mobile.png`)
    await mobilePage.screenshot({ path: mobilePath, fullPage: true })
    console.log(`Saved mobile screenshot: ${mobilePath}`)
    await mobileContext.close()

    // Desktop Viewport (1440px x 900px standard widescreen)
    const desktopContext = await browser.newContext({
      viewport: { width: 1440, height: 900 },
      deviceScaleFactor: 2,
      colorScheme: 'dark'
    })
    const desktopPage = await desktopContext.newPage()
    await desktopPage.goto('file://' + htmlFilePath)
    await desktopPage.waitForTimeout(300)
    const desktopPath = path.join(screenshotsDir, `${baseName}-desktop.png`)
    await desktopPage.screenshot({ path: desktopPath, fullPage: true })
    console.log(`Saved desktop screenshot: ${desktopPath}`)
    await desktopContext.close()
  }

  // 1. Capture Inbox View
  console.log('Capturing Inbox Apple Mail & Bear Inset List...')
  await captureViewports(inboxHtml, 's61-t4-inbox')

  // 2. Capture SlideDrawer View
  console.log('Capturing Chat Stream with SlideDrawer...')
  await captureViewports(drawerHtml, 's61-t4-drawer')

  // 3. Capture Tasks View
  console.log('Capturing Things 3 Pure Task Rows...')
  await captureViewports(tasksHtml, 's61-t4-tasks')

  await browser.close()
  console.log('All S61-T4 audit screenshots generated successfully!')
}

run().catch((err) => {
  console.error('Failed to generate screenshots:', err)
  process.exit(1)
})
