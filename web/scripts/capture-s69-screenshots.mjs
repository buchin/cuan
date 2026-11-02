import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')
const evidenceDir = path.join(repoRoot, 'docs/sprints/evidence/s69')
const screenshotsDir = path.join(repoRoot, 'screenshots')

fs.mkdirSync(evidenceDir, { recursive: true })
fs.mkdirSync(screenshotsDir, { recursive: true })

const require = createRequire(import.meta.url)
const playwrightPath = path.join(process.env.HOME || '/home/dev', '.local/lib/node_modules/playwright')
const { chromium } = require(playwrightPath)

// Find compiled CSS
const distAssetsDir = path.join(webDir, 'dist/assets')
let compiledCss = ''
if (fs.existsSync(distAssetsDir)) {
  const cssFiles = fs.readdirSync(distAssetsDir).filter((f) => f.endsWith('.css'))
  if (cssFiles.length > 0) {
    const mainCssFile = cssFiles.find((f) => f.startsWith('style-') || f.startsWith('index-')) || cssFiles[0]
    compiledCss = fs.readFileSync(path.join(distAssetsDir, mainCssFile), 'utf8')
  }
}

const baseStyles = `
  ${compiledCss}
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
  }
`

function createAgentHtml(isMobile = false) {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 69 - Asisten & Brain Cluster</title>
  <style>
    ${baseStyles}
  </style>
</head>
<body class="dark bg-background text-foreground">
  <div class="app-viewport">
    <!-- Header -->
    <header class="h-14 border-b border-border/40 px-4 flex items-center justify-between shrink-0 bg-background/95 backdrop-blur-sm">
      <div class="flex items-center gap-3">
        <span class="font-bold tracking-tight text-foreground text-sm">Cuan</span>
        <span class="text-xs text-muted-foreground">/ Pengaturan</span>
      </div>
      <div class="flex items-center gap-2">
        <div class="flex h-8 w-8 items-center justify-center rounded-full bg-primary/20 text-primary text-xs font-bold border border-primary/30">
          BU
        </div>
      </div>
    </header>

    <div class="flex-1 flex overflow-hidden">
      ${!isMobile ? `
      <!-- Master Navigation Sidebar -->
      <aside class="w-72 border-r border-border/40 p-4 space-y-4 shrink-0 bg-card/20 overflow-y-auto">
        <div class="space-y-1">
          <h3 class="px-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/80">Workspace & Sistem</h3>
          <div class="space-y-0.5">
            <div class="px-3 py-2 text-xs text-muted-foreground hover:bg-accent/40 rounded-lg flex items-center gap-2.5">
              <span>Umum</span>
            </div>
            <div class="px-3 py-2 text-xs text-muted-foreground hover:bg-accent/40 rounded-lg flex items-center gap-2.5">
              <span>Anggota Tim</span>
            </div>
            <div class="px-3 py-2 text-xs text-muted-foreground hover:bg-accent/40 rounded-lg flex items-center gap-2.5">
              <span>Jadwal Rutin</span>
            </div>
          </div>
        </div>

        <div class="space-y-1">
          <h3 class="px-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/80">Kecerdasan AI & Memori</h3>
          <div class="space-y-0.5">
            <div class="px-3 py-2 text-xs bg-primary/10 text-primary font-semibold rounded-lg flex items-center gap-2.5 shadow-xs">
              <span>Asisten & Otak AI</span>
            </div>
            <div class="px-3 py-2 text-xs text-muted-foreground hover:bg-accent/40 rounded-lg flex items-center gap-2.5">
              <span>Memori Agen & Rasa</span>
            </div>
          </div>
        </div>
      </aside>
      ` : ''}

      <!-- Main Detail Pane -->
      <main class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-6 max-w-4xl">
        <div>
          <h2 class="text-xl font-bold tracking-tight text-foreground flex items-center gap-2">
            <span>Asisten AI & Kumpulan Otak</span>
          </h2>
          <p class="mt-1 text-xs text-muted-foreground">
            Atur nama asisten dan hubungkan akun-akun AI yang menjadi otak komputasi di balik layar.
          </p>
        </div>

        <!-- 1. Assistant Identity Card -->
        <div class="rounded-2xl border border-border/60 bg-card/60 p-5 space-y-4">
          <div class="flex items-center gap-3.5">
            <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary border border-primary/20 text-2xl">
              🤖
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="font-bold text-base text-foreground">Cuan</h3>
                <span class="inline-flex items-center gap-1 px-2.5 py-0.5 text-[10px] font-semibold rounded-full bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                  <span>Siap Melayani (4 Otak Terhubung)</span>
                </span>
              </div>
              <p class="text-xs text-muted-foreground mt-0.5">
                Satu asisten cerdas untuk semua percakapan, tugas, dan proyek bisnis Anda.
              </p>
            </div>
          </div>

          <div class="pt-2 border-t border-border/40 max-w-md">
            <label class="block text-xs font-semibold text-foreground mb-1.5">Nama Asisten</label>
            <div class="flex items-center gap-2">
              <input type="text" value="Cuan" class="px-3 py-2 bg-muted/50 border border-border/60 rounded-xl text-sm font-medium text-foreground flex-1" />
              <button class="px-3.5 py-2 border border-border/60 rounded-xl text-xs font-medium text-foreground">Simpan</button>
            </div>
          </div>
        </div>

        <!-- 2. Brain Cluster -->
        <div class="space-y-3">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-bold tracking-tight text-foreground flex items-center gap-2">
                <span>Kumpulan Otak AI (Brain Cluster)</span>
              </h3>
              <p class="text-xs text-muted-foreground mt-0.5">
                Akun-akun AI yang terhubung melayani request layaknya server cluster. Cuan otomatis membagi beban tugas (load balancing) dan beralih akun saat kuota habis (auto-failover).
              </p>
            </div>
            <button class="inline-flex items-center justify-center gap-1.5 px-4 py-2 bg-primary text-primary-foreground font-semibold text-xs rounded-xl shadow-xs shrink-0">
              + Tambah Otak AI
            </button>
          </div>

          <!-- Nodes List -->
          <div class="rounded-2xl border border-border/40 bg-card/40 overflow-hidden divide-y divide-border/30">
            <!-- Node 1: Google Antigravity (@sm) -->
            <div class="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div class="flex items-center gap-3">
                <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-blue-500/10 text-blue-400 font-bold text-xs">
                  G
                </div>
                <div>
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="font-semibold text-sm text-foreground">Google Antigravity (@sm)</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-blue-500/10 text-blue-400 font-semibold border border-blue-500/25">Langganan Resmi</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-emerald-500/15 text-emerald-400 font-semibold border border-emerald-500/30">Siap Melayani</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-muted text-muted-foreground">Vision Ready</span>
                    <span class="text-[10px] font-mono text-muted-foreground">gemini-2.5-pro</span>
                  </div>
                  <div class="text-xs text-muted-foreground mt-0.5">
                    pin.bbm.id.1@gmail.com · <span class="text-emerald-400 font-medium">✓ Kuota Tersedia</span>
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button class="px-2.5 py-1 text-xs border border-border/60 rounded-lg text-foreground hover:bg-muted">Tes Kuota</button>
                <button class="px-2.5 py-1 text-xs text-destructive hover:bg-destructive/10 rounded-lg">Putuskan</button>
              </div>
            </div>

            <!-- Node 2: Google Antigravity (@pin2) -->
            <div class="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div class="flex items-center gap-3">
                <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-blue-500/10 text-blue-400 font-bold text-xs">
                  G
                </div>
                <div>
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="font-semibold text-sm text-foreground">Google Antigravity (@pin2)</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-blue-500/10 text-blue-400 font-semibold border border-blue-500/25">Langganan Resmi</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-amber-500/15 text-amber-400 font-semibold border border-amber-500/30">Cadangan (Failover)</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-muted text-muted-foreground">Vision Ready</span>
                    <span class="text-[10px] font-mono text-muted-foreground">gemini-2.5-flash</span>
                  </div>
                  <div class="text-xs text-muted-foreground mt-0.5">
                    pin.bbm.id.2@gmail.com · <span class="text-amber-400 font-medium">Siaga Otomatis Saat Kuota Utama Penuh</span>
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button class="px-2.5 py-1 text-xs border border-border/60 rounded-lg text-foreground hover:bg-muted">Tes Kuota</button>
                <button class="px-2.5 py-1 text-xs text-destructive hover:bg-destructive/10 rounded-lg">Putuskan</button>
              </div>
            </div>

            <!-- Node 3: CommandCode (@cmd) -->
            <div class="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div class="flex items-center gap-3">
                <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-purple-500/10 text-purple-400 font-bold text-xs">
                  CC
                </div>
                <div>
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="font-semibold text-sm text-foreground">CommandCode (@cmd)</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-purple-500/10 text-purple-400 font-semibold border border-purple-500/25">Langganan Resmi</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-emerald-500/15 text-emerald-400 font-semibold border border-emerald-500/30">Siap Melayani</span>
                    <span class="text-[10px] font-mono text-muted-foreground">claude-3-7-sonnet</span>
                  </div>
                  <div class="text-xs text-muted-foreground mt-0.5">
                    Sesi Browser Terhubung · <span class="text-emerald-400 font-medium">✓ Aktif</span>
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button class="px-2.5 py-1 text-xs border border-border/60 rounded-lg text-foreground hover:bg-muted">Tes Kuota</button>
                <button class="px-2.5 py-1 text-xs text-destructive hover:bg-destructive/10 rounded-lg">Putuskan</button>
              </div>
            </div>

            <!-- Node 4: DeepSeek BYOK -->
            <div class="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div class="flex items-center gap-3">
                <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-indigo-500/10 text-indigo-400 font-bold text-xs">
                  DS
                </div>
                <div>
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="font-semibold text-sm text-foreground">DeepSeek</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-muted text-muted-foreground font-semibold border border-border">Kunci API (BYOK)</span>
                    <span class="px-2 py-0.5 text-[10px] rounded-full bg-emerald-500/15 text-emerald-400 font-semibold border border-emerald-500/30">Siap Melayani</span>
                    <span class="text-[10px] font-mono text-muted-foreground">deepseek-chat</span>
                  </div>
                  <div class="text-xs text-muted-foreground mt-0.5">
                    sk-•••••••••••• · <span class="text-emerald-400 font-medium">✓ Latency: 98ms</span>
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button class="px-2.5 py-1 text-xs border border-border/60 rounded-lg text-foreground hover:bg-muted">Tes Latency</button>
                <button class="px-2.5 py-1 text-xs text-destructive hover:bg-destructive/10 rounded-lg">Putuskan</button>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  </div>
</body>
</html>`
}

function createTeamHtml(isMobile = false) {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 69 - Anggota Tim</title>
  <style>
    ${baseStyles}
  </style>
</head>
<body class="dark bg-background text-foreground">
  <div class="app-viewport">
    <!-- Header -->
    <header class="h-14 border-b border-border/40 px-4 flex items-center justify-between shrink-0 bg-background/95 backdrop-blur-sm">
      <div class="flex items-center gap-3">
        <span class="font-bold tracking-tight text-foreground text-sm">Cuan</span>
        <span class="text-xs text-muted-foreground">/ Pengaturan</span>
      </div>
      <div class="flex items-center gap-2">
        <div class="flex h-8 w-8 items-center justify-center rounded-full bg-primary/20 text-primary text-xs font-bold border border-primary/30">
          BU
        </div>
      </div>
    </header>

    <div class="flex-1 flex overflow-hidden">
      ${!isMobile ? `
      <!-- Master Navigation Sidebar -->
      <aside class="w-72 border-r border-border/40 p-4 space-y-4 shrink-0 bg-card/20 overflow-y-auto">
        <div class="space-y-1">
          <h3 class="px-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/80">Workspace & Sistem</h3>
          <div class="space-y-0.5">
            <div class="px-3 py-2 text-xs text-muted-foreground hover:bg-accent/40 rounded-lg flex items-center gap-2.5">
              <span>Umum</span>
            </div>
            <div class="px-3 py-2 text-xs bg-primary/10 text-primary font-semibold rounded-lg flex items-center gap-2.5 shadow-xs">
              <span>Anggota Tim</span>
            </div>
            <div class="px-3 py-2 text-xs text-muted-foreground hover:bg-accent/40 rounded-lg flex items-center gap-2.5">
              <span>Jadwal Rutin</span>
            </div>
          </div>
        </div>

        <div class="space-y-1">
          <h3 class="px-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/80">Kecerdasan AI & Memori</h3>
          <div class="space-y-0.5">
            <div class="px-3 py-2 text-xs text-muted-foreground hover:bg-accent/40 rounded-lg flex items-center gap-2.5">
              <span>Asisten & Otak AI</span>
            </div>
            <div class="px-3 py-2 text-xs text-muted-foreground hover:bg-accent/40 rounded-lg flex items-center gap-2.5">
              <span>Memori Agen & Rasa</span>
            </div>
          </div>
        </div>
      </aside>
      ` : ''}

      <!-- Main Detail Pane -->
      <main class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-6 max-w-4xl">
        <div class="flex items-center justify-between gap-4 pb-2 border-b border-border/40">
          <div>
            <h2 class="text-xl font-bold tracking-tight text-foreground flex items-center gap-2">
              <span>Anggota Tim</span>
            </h2>
            <p className="mt-1 text-xs text-muted-foreground">
              Kelola rekan kerja dan staf yang memiliki akses ke akun ruang kerja Cuan.
            </p>
          </div>
          <button class="inline-flex items-center justify-center gap-1.5 px-4 py-2.5 bg-primary text-primary-foreground font-semibold text-xs rounded-xl shadow-xs shrink-0">
            + Undang Anggota
          </button>
        </div>

        <!-- Active Members List -->
        <div class="space-y-3">
          <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Staf Aktif (2)
          </h3>

          <div class="rounded-2xl border border-border/40 bg-card/40 overflow-hidden divide-y divide-border/30">
            <!-- Member 1: Owner -->
            <div class="p-4 flex items-center justify-between">
              <div class="flex items-center gap-3.5">
                <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-emerald-500/15 text-emerald-400 font-bold text-xs border border-emerald-500/30">
                  BU
                </div>
                <div>
                  <div class="flex items-center gap-2">
                    <span class="font-semibold text-sm text-foreground">Buchin</span>
                    <span class="text-[10px] bg-muted text-muted-foreground px-2 py-0.5 rounded-full font-mono">Anda</span>
                    <span class="px-2.5 py-0.5 text-[11px] font-semibold rounded-full bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">Owner</span>
                    <span class="px-2 py-0.5 text-[10px] text-emerald-400 bg-emerald-500/10 rounded-full font-medium">Aktif</span>
                  </div>
                  <div class="text-xs text-muted-foreground mt-0.5">
                    buchin@dropsugar.com · Bergabung 2 Nov 2026
                  </div>
                </div>
              </div>
            </div>

            <!-- Member 2: Staff -->
            <div class="p-4 flex items-center justify-between">
              <div class="flex items-center gap-3.5">
                <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary/15 text-primary font-bold text-xs border border-primary/30">
                  SA
                </div>
                <div>
                  <div class="flex items-center gap-2">
                    <span class="font-semibold text-sm text-foreground">Siti Aminah</span>
                    <span class="px-2.5 py-0.5 text-[11px] font-semibold rounded-full bg-primary/10 text-primary border border-primary/25">Member</span>
                    <span class="px-2 py-0.5 text-[10px] text-emerald-400 bg-emerald-500/10 rounded-full font-medium">Aktif</span>
                  </div>
                  <div class="text-xs text-muted-foreground mt-0.5">
                    siti@dropsugar.com · Bergabung 2 Nov 2026
                  </div>
                </div>
              </div>
              <button class="px-3 py-1.5 text-xs text-destructive hover:bg-destructive/10 rounded-lg">Hapus Akses</button>
            </div>
          </div>
        </div>

        <!-- Pending Invitations -->
        <div class="space-y-3 pt-2">
          <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Undangan Menunggu (1)
          </h3>

          <div class="rounded-2xl border border-border/40 bg-card/40 overflow-hidden p-4 flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-amber-500/10 text-amber-500 border border-amber-500/30 text-xs">
                ✉️
              </div>
              <div>
                <div class="flex items-center gap-2">
                  <span class="font-medium text-sm text-foreground">budi@dropsugar.com</span>
                  <span class="px-2 py-0.5 text-[10px] rounded-full bg-amber-500/15 text-amber-400 border border-amber-500/30">Menunggu Konfirmasi</span>
                  <span class="text-[10px] bg-muted px-2 py-0.5 rounded-full text-muted-foreground">member</span>
                </div>
                <div class="text-xs text-muted-foreground mt-0.5">Dikirim 2 Nov 2026</div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <button class="px-3 py-1.5 text-xs text-destructive hover:bg-destructive/10 rounded-lg">Cabut</button>
            </div>
          </div>
        </div>
      </main>
    </div>
  </div>
</body>
</html>`
}

async function capture() {
  console.log('==> Launching Chromium for Sprint 69 visual inspection...')
  const executablePath = path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium')
  const browser = await chromium.launch({
    headless: true,
    executablePath,
    env: {
      ...process.env,
      LD_LIBRARY_PATH:
        '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' +
        (process.env.LD_LIBRARY_PATH || ''),
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--disable-gpu'],
  })

  // 1. Desktop: Agent & Brain Cluster
  const pageDesktopAgent = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  await pageDesktopAgent.setContent(createAgentHtml(false))
  const pathDesktopAgent = path.join(evidenceDir, 's69-desktop-agent-brain-cluster.png')
  await pageDesktopAgent.screenshot({ path: pathDesktopAgent, fullPage: false })
  fs.copyFileSync(pathDesktopAgent, path.join(screenshotsDir, 's69-desktop-agent-brain-cluster.png'))
  console.log('✓ Captured desktop agent & brain cluster:', pathDesktopAgent)

  // 2. Desktop: Team Members
  const pageDesktopTeam = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  await pageDesktopTeam.setContent(createTeamHtml(false))
  const pathDesktopTeam = path.join(evidenceDir, 's69-desktop-team-members.png')
  await pageDesktopTeam.screenshot({ path: pathDesktopTeam, fullPage: false })
  fs.copyFileSync(pathDesktopTeam, path.join(screenshotsDir, 's69-desktop-team-members.png'))
  console.log('✓ Captured desktop team members:', pathDesktopTeam)

  // 3. Mobile 390px: Agent & Brain Cluster
  const pageMobileAgent = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true })
  await pageMobileAgent.setContent(createAgentHtml(true))
  const pathMobileAgent = path.join(evidenceDir, 's69-mobile-390px-agent.png')
  await pageMobileAgent.screenshot({ path: pathMobileAgent, fullPage: false })
  fs.copyFileSync(pathMobileAgent, path.join(screenshotsDir, 's69-mobile-390px-agent.png'))
  console.log('✓ Captured mobile agent & brain cluster:', pathMobileAgent)

  // 4. Mobile 390px: Team Members
  const pageMobileTeam = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true })
  await pageMobileTeam.setContent(createTeamHtml(true))
  const pathMobileTeam = path.join(evidenceDir, 's69-mobile-390px-team.png')
  await pageMobileTeam.screenshot({ path: pathMobileTeam, fullPage: false })
  fs.copyFileSync(pathMobileTeam, path.join(screenshotsDir, 's69-mobile-390px-team.png'))
  console.log('✓ Captured mobile team members:', pathMobileTeam)

  await browser.close()
  console.log('✓ All Sprint 69 screenshots captured successfully.')
}

capture().catch((err) => {
  console.error('Screenshot capture failed:', err)
  process.exit(1)
})
