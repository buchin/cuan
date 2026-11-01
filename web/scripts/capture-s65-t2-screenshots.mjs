import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')
const screenshotsDir = path.join(repoRoot, 'screenshots')
const reportsDir = path.join(repoRoot, 'docs/reports')

if (!fs.existsSync(screenshotsDir)) {
  fs.mkdirSync(screenshotsDir, { recursive: true })
}
if (!fs.existsSync(reportsDir)) {
  fs.mkdirSync(reportsDir, { recursive: true })
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

function createTimelineHtml(isMobile = false) {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>S65-T2: Hybrid Actionable Task Card & 1-Click Actionable Draft</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport flex flex-col h-full bg-[#141416]">
    <!-- Top Calm Header -->
    <header class="h-14 shrink-0 border-b border-border/30 bg-[#18181a] px-4 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="h-8 w-8 rounded-full bg-secondary flex items-center justify-center font-semibold text-xs text-foreground">
          WP
        </div>
        <div>
          <h2 class="text-sm font-semibold text-foreground tracking-tight">WhatsApp Support Room</h2>
          <p class="text-[11px] text-muted-foreground flex items-center gap-1.5">
            <span class="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
            <span>Online • Pelanggan: +62 812-3456-7890</span>
          </p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs px-2.5 py-1 rounded-full bg-primary/10 text-primary font-medium">Sprint 65</span>
      </div>
    </header>

    <!-- Chat Message Stream -->
    <div class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-5 max-w-4xl mx-auto w-full">
      
      <!-- Visitor Message -->
      <div class="flex items-start gap-3">
        <div class="h-7 w-7 rounded-full bg-muted flex items-center justify-center text-[11px] font-medium shrink-0">
          WA
        </div>
        <div class="max-w-[85%] rounded-2xl rounded-tl-sm bg-[#1e1e22] border border-border/30 p-3.5 text-sm text-foreground">
          <div class="text-[11px] font-medium text-muted-foreground mb-1">Pengunjung (+62 812-3456-7890)</div>
          Halo kak, mau tanya apakah paket tahunan Cuan ada promo diskon khusus untuk pendaftaran bulan ini?
        </div>
      </div>

      <!-- Assistant / S65-T2 Hybrid Actionable Task Card -->
      <div class="flex items-start gap-3">
        <div class="h-7 w-7 rounded-full bg-primary/20 text-primary flex items-center justify-center text-[11px] font-semibold shrink-0">
          AI
        </div>
        <div class="flex-1 max-w-[92%] sm:max-w-2xl space-y-3">
          
          <!-- S65-T2 Task Completion Card -->
          <div data-testid="task-completion-card" class="rounded-xl border border-border/50 bg-card/60 p-4 space-y-3.5 transition-colors">
            
            <!-- Calm Header with Live Terminal & Retry buttons -->
            <div class="flex items-center justify-between gap-2">
              <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-medium bg-emerald-500/10 text-emerald-400">
                <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <span>Selesai</span>
              </span>

              <!-- Actionable Header Buttons -->
              <div class="flex items-center gap-1.5">
                <button
                  type="button"
                  data-testid="task-card-terminal-btn"
                  class="min-h-[44px] min-w-[44px] px-3 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors"
                >
                  <svg class="h-3.5 w-3.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                  </svg>
                  <span>Live Terminal</span>
                </button>

                <button
                  type="button"
                  data-testid="task-card-retry-btn"
                  class="min-h-[44px] min-w-[44px] px-3 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors"
                >
                  <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                  <span>Coba Lagi</span>
                </button>
              </div>
            </div>

            <!-- Task Title & Summary -->
            <div class="space-y-1">
              <h3 class="text-sm font-semibold text-foreground tracking-tight">
                Pengecekan promo tahunan & kalkulasi diskon voucher
              </h3>
              <p class="text-xs text-muted-foreground leading-relaxed">
                Database pricing diverifikasi. Paket tahunan aktif dengan promo diskon 20% kode <code class="px-1 py-0.5 rounded bg-muted/60 font-mono text-[11px]">ANNUAL20</code>.
              </p>
            </div>

            <!-- S65-T2 1-Click Actionable Visitor Reply Draft Section -->
            <div data-testid="task-card-draft-section" class="rounded-xl border border-border/40 bg-muted/20 p-3 sm:p-4 space-y-2.5">
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-1.5 text-xs font-medium text-foreground">
                  <svg class="h-3.5 w-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 10h.01M12 10h.01M16 10h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                  </svg>
                  <span>Draf Balasan Pengunjung</span>
                </div>
                <button
                  type="button"
                  data-testid="task-card-use-draft-btn"
                  class="min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 bg-primary/10 text-primary hover:bg-primary/20 transition-colors shadow-sm cursor-pointer"
                >
                  <svg class="h-3.5 w-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                  </svg>
                  <span>Gunakan Draf di Input</span>
                </button>
              </div>

              <!-- Draft Text Bubble -->
              <div class="text-xs text-foreground/90 bg-background/60 border border-border/30 rounded-lg p-3 leading-relaxed whitespace-pre-wrap font-sans">Halo kak! Betul sekali, untuk paket tahunan kami ada diskon 20% langsung menggunakan kode voucher ANNUAL20 saat checkout. Apakah ada bantuan lain yang dibutuhkan untuk proses aktivasi akunnya?</div>
            </div>

            <!-- Parked Secondary Details Link (SlideDrawer handoff) -->
            <div class="pt-1">
              <button
                type="button"
                data-testid="task-card-diagnostic-link"
                class="min-h-[44px] text-xs font-medium text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 transition-colors cursor-pointer group"
              >
                <span>Detail Diagnostik & Status Deploy</span>
                <span class="group-hover:translate-x-0.5 transition-transform text-muted-foreground/70">→</span>
              </button>
            </div>

          </div>

        </div>
      </div>

    </div>

    <!-- Calm Composer with 1-Click Loaded Draft Text -->
    <div class="p-3 sm:p-4 border-t border-border/30 bg-[#161618] shrink-0">
      <div class="max-w-4xl mx-auto flex items-end gap-2 bg-[#1e1e22] border border-border/40 rounded-2xl p-2 px-3 shadow-inner">
        <textarea
          rows="${isMobile ? 3 : 2}"
          class="flex-1 bg-transparent border-0 text-sm text-foreground focus:outline-none resize-none placeholder:text-muted-foreground"
          placeholder="Tulis pesan balasan..."
        >Halo kak! Betul sekali, untuk paket tahunan kami ada diskon 20% langsung menggunakan kode voucher ANNUAL20 saat checkout. Apakah ada bantuan lain yang dibutuhkan untuk proses aktivasi akunnya?</textarea>
        <button
          type="button"
          class="h-10 w-10 rounded-xl bg-primary text-primary-foreground flex items-center justify-center shrink-0 hover:opacity-90 transition-opacity"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
          </svg>
        </button>
      </div>
      <div class="max-w-4xl mx-auto flex items-center justify-between text-[11px] text-muted-foreground mt-2 px-1">
        <span>Tekan Enter untuk mengirim atau edit draf di atas</span>
        <span class="text-emerald-400">✓ Draf terisi otomatis dari Task Card</span>
      </div>
    </div>
  </div>
</body>
</html>`
}

function createDrawerHtml(activeTab = 'details', isMobile = false) {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>S65-T2: SlideDrawer Diagnostic & Live Terminal Tabs</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport flex flex-row h-full bg-[#141416]">
    
    <!-- Background Timeline (Desktop shows side-by-side, Mobile hides chat behind drawer) -->
    <div class="${isMobile ? 'hidden' : 'flex-1'} flex flex-col h-full border-r border-border/30 opacity-40 pointer-events-none">
      <header class="h-14 border-b border-border/30 bg-[#18181a] px-4 flex items-center">
        <span class="text-sm font-semibold">WhatsApp Support Room</span>
      </header>
      <div class="p-6 space-y-4">
        <div class="p-4 rounded-xl border border-border/40 bg-card/60">
          <div class="h-4 w-40 bg-muted/40 rounded mb-2"></div>
          <div class="h-3 w-60 bg-muted/20 rounded"></div>
        </div>
      </div>
    </div>

    <!-- SlideDrawer Component -->
    <div class="${isMobile ? 'w-full' : 'w-[480px]'} flex flex-col h-full bg-[#18181b] shadow-2xl z-50">
      
      <!-- Drawer Header -->
      <div class="p-4 border-b border-border/40 flex items-center justify-between shrink-0 bg-[#1a1a1e]">
        <div class="flex items-center gap-2">
          <div class="h-7 w-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
          </div>
          <span class="text-sm font-semibold text-foreground">Detail Diagnostik Task</span>
        </div>
        <button type="button" class="min-h-[44px] min-w-[44px] inline-flex items-center justify-center rounded-lg hover:bg-muted/40 text-muted-foreground hover:text-foreground">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- S65-T2 SlideDrawer Tabs -->
      <div class="flex border-b border-border/40 bg-[#151518] px-2 pt-1 shrink-0">
        <button
          type="button"
          class="min-h-[44px] px-4 py-2 text-xs font-medium border-b-2 flex items-center gap-2 transition-colors ${
            activeTab === 'details'
              ? 'border-primary text-foreground bg-muted/20'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }"
        >
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <span>Detail Diagnostik & Status Deploy</span>
        </button>

        <button
          type="button"
          class="min-h-[44px] px-4 py-2 text-xs font-medium border-b-2 flex items-center gap-2 transition-colors ${
            activeTab === 'terminal'
              ? 'border-primary text-foreground bg-muted/20'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }"
        >
          <svg class="h-3.5 w-3.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          <span>Live Terminal</span>
        </button>
      </div>

      <!-- Drawer Content -->
      <div class="flex-1 overflow-y-auto p-4 sm:p-5 space-y-4">
        ${
          activeTab === 'details'
            ? `
          <!-- Status Deploy Section (Conditional Deploy Indicator) -->
          <div class="rounded-xl border border-border/40 bg-muted/20 p-3.5 space-y-2">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-foreground flex items-center gap-1.5">
                <svg class="h-3.5 w-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
                <span>Status Deploy</span>
              </span>
              <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-muted/60 text-muted-foreground">
                <svg class="h-3 w-3 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
                <span>Deploy VPS Tidak Diperlukan</span>
              </span>
            </div>
            <p class="text-[11px] text-muted-foreground">
              Tugas ini merupakan investigasi konfigurasi/analisis tanpa perubahan bundle runtime pada server VPS.
            </p>
          </div>

          <!-- Parked Definition of Done (DoD) Criteria -->
          <div class="rounded-xl border border-border/40 bg-muted/20 p-3.5 space-y-2.5">
            <div class="flex items-center gap-1.5 text-xs font-semibold text-foreground">
              <svg class="h-3.5 w-3.5 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
              </svg>
              <span>Kriteria Definition of Done (DoD)</span>
            </div>
            <div class="space-y-1.5">
              <div class="flex items-start gap-2 text-xs text-foreground/90">
                <span class="text-emerald-400 mt-0.5">✓</span>
                <span>Analisis database kupon promo tahunan berhasil diverifikasi</span>
              </div>
              <div class="flex items-start gap-2 text-xs text-foreground/90">
                <span class="text-emerald-400 mt-0.5">✓</span>
                <span>Draf balasan ramah pengunjung diformulasikan sesuai format CS WhatsApp</span>
              </div>
              <div class="flex items-start gap-2 text-xs text-foreground/90">
                <span class="text-emerald-400 mt-0.5">✓</span>
                <span>Zero Visual Noise: tabel mentah diparkir ke SlideDrawer agar stream bersih</span>
              </div>
              <div class="flex items-start gap-2 text-xs text-foreground/90">
                <span class="text-emerald-400 mt-0.5">✓</span>
                <span>Aksi 1-klik muat draf ke composer terintegrasi dengan autofokus</span>
              </div>
            </div>
          </div>

          <!-- Git & Task Execution Metadata -->
          <div class="rounded-xl border border-border/40 bg-muted/20 p-3.5 space-y-2">
            <span class="text-xs font-semibold text-foreground">Metadata Eksekusi</span>
            <div class="flex flex-wrap items-center gap-2 pt-1">
              <span class="inline-flex items-center gap-1 font-mono text-[11px] text-muted-foreground bg-muted/40 px-2 py-0.5 rounded">
                <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
                <span>Agent: @sm</span>
              </span>
              <span class="inline-flex items-center gap-1 font-mono text-[11px] text-muted-foreground bg-muted/40 px-2 py-0.5 rounded">
                <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 20l4-16m2 16l4-16M6 9h14M4 15h14" />
                </svg>
                <span>feature/s65-t2-hybrid-actionable-task-card</span>
              </span>
            </div>
          </div>
          `
            : `
          <!-- Live Terminal Tab Content (xterm output stream) -->
          <div class="flex-1 h-full min-h-[400px] rounded-xl border border-border/40 bg-[#0d0d0e] p-3 font-mono text-xs text-emerald-400 space-y-1.5 shadow-inner">
            <div class="text-muted-foreground text-[11px] pb-2 border-b border-white/10 flex items-center justify-between">
              <span>task-session: uryzvnstzqtlzpfjhlzdzncn</span>
              <span class="text-emerald-400 flex items-center gap-1">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
                Connected
              </span>
            </div>
            <div class="text-zinc-400">$ fural-agent task-stream uryzvnstzqtlzpfjhlzdzncn</div>
            <div class="text-zinc-300">[18:25:01] Initializing runner workspace...</div>
            <div class="text-zinc-300">[18:25:03] Running discount matrix calculation...</div>
            <div class="text-zinc-300">[18:25:05] Query matched promo code: <span class="text-amber-300">ANNUAL20</span> (Discount: 20%)</div>
            <div class="text-emerald-400">[18:25:08] Envelope generated with visitor reply draft.</div>
            <div class="text-emerald-300">[18:25:10] Execution finished cleanly. Exit code: 0</div>
            <div class="text-zinc-500 pt-3 flex items-center gap-2">
              <span class="animate-pulse">▌</span>
              <span>Live stream completed</span>
            </div>
          </div>
          `
        }
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
  
  // 1. Desktop: Hybrid Actionable Task Card & 1-Click Draft in Timeline
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } })
    await page.setContent(createTimelineHtml(false), { waitUntil: 'load' })
    await page.screenshot({ path: path.join(screenshotsDir, 's65-t2-hybrid-task-card-desktop.png') })
    await page.close()
  }

  // 2. Mobile: Hybrid Actionable Task Card (390px)
  {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } })
    await page.setContent(createTimelineHtml(true), { waitUntil: 'load' })
    await page.screenshot({ path: path.join(screenshotsDir, 's65-t2-hybrid-task-card-mobile.png') })
    await page.close()
  }

  // 3. Desktop: SlideDrawer Diagnostic & Status Deploy Tab
  {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } })
    await page.setContent(createDrawerHtml('details', false), { waitUntil: 'load' })
    await page.screenshot({ path: path.join(screenshotsDir, 's65-t2-slidedrawer-diagnostic-desktop.png') })
    await page.close()
  }

  // 4. Mobile: SlideDrawer Live Terminal Tab (390px)
  {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } })
    await page.setContent(createDrawerHtml('terminal', true), { waitUntil: 'load' })
    await page.screenshot({ path: path.join(screenshotsDir, 's65-t2-slidedrawer-terminal-mobile.png') })
    await page.close()
  }

  await browser.close()
  console.log('Successfully captured all S65-T2 screenshots.')
}

capture().catch((err) => {
  console.error('Error capturing screenshots:', err)
  process.exit(1)
})
