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
`

function createMobileAgentsHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Settings - Agents (Mobile 390px)</title>
  <style>${baseStyles}</style>
</head>
<body class="dark bg-background text-foreground min-h-screen">
  <div class="max-w-[390px] mx-auto min-h-screen flex flex-col bg-background">
    <!-- Mobile App Header: Clean, No noisy + button -->
    <header class="h-14 shrink-0 border-b border-border/40 px-4 flex items-center justify-between bg-card/60 backdrop-blur">
      <div class="flex items-center gap-2">
        <div class="h-6 w-6 rounded-full border-2 border-foreground flex items-center justify-center">
          <div class="h-2 w-2 rounded-full bg-foreground"></div>
        </div>
        <h1 class="text-base font-semibold">Agents</h1>
      </div>
      <div class="flex items-center gap-2">
        <button class="h-9 w-9 text-muted-foreground flex items-center justify-center text-lg">⋮</button>
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="flex-1 p-4 space-y-5 overflow-y-auto">
      <!-- Clean Single Mobile Back Button (No double arrow < ←) -->
      <div>
        <a href="#" class="inline-flex min-h-11 items-center gap-1.5 rounded-md text-sm font-medium text-muted-foreground hover:text-foreground transition-colors">
          <svg class="h-4 w-4 shrink-0" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
          <span>Kembali ke Pengaturan</span>
        </a>
      </div>

      <!-- Tabs: Asisten AI (1) vs Anggota Tim (1) -->
      <div class="flex border-b border-border/40" role="tablist">
        <button class="flex items-center gap-2 px-4 py-3 text-sm font-semibold border-b-2 border-primary text-primary">
          <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>
          <span>Asisten AI</span>
          <span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">1</span>
        </button>
        <button class="flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 border-transparent text-muted-foreground hover:text-foreground">
          <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
          <span>Anggota Tim</span>
          <span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">1</span>
        </button>
      </div>

      <!-- 1. Primary Assistant Profile Card (Single Agent Facade @cuan) -->
      <div class="rounded-2xl border border-border/40 bg-card/30 p-5 space-y-4">
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-3.5 min-w-0">
            <div class="relative shrink-0">
              <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary border border-primary/20 text-2xl font-semibold">
                <svg class="h-7 w-7" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>
              </div>
              <div class="absolute -bottom-1 -right-1">
                <span class="relative flex h-3 w-3 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 space-y-0.5">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-base font-bold text-foreground truncate">@cuan</h3>
                <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                  <span>Aktif & Siap</span>
                </span>
              </div>
              <p class="text-xs text-muted-foreground truncate">Cuan Universal Assistant</p>
            </div>
          </div>

          <button class="inline-flex min-h-11 items-center gap-1.5 rounded-xl border border-border bg-background px-3 py-1.5 text-xs font-medium text-foreground hover:bg-muted shrink-0">
            <svg class="h-3.5 w-3.5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/></svg>
            <span>Edit</span>
          </button>
        </div>

        <!-- Persona / Prompt -->
        <div class="space-y-1 pt-2 border-t border-border/40">
          <span class="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider block">Peran & Instruksi Kerja</span>
          <div class="p-3 rounded-xl border border-border/30 bg-muted/20 text-xs text-foreground/90 leading-relaxed">
            Asisten bisnis terpadu untuk memandu proyek, merespons percakapan pelanggan, dan mengeksekusi otomatisasi operasional.
          </div>
        </div>

        <!-- Model Chips -->
        <div class="space-y-1.5 pt-2 border-t border-border/40">
          <div class="flex items-center justify-between">
            <span class="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider block">Model AI & Konfigurasi</span>
            <span class="text-[11px] text-muted-foreground hover:text-foreground font-medium">Detail Teknis ›</span>
          </div>
          <div class="flex flex-wrap items-center gap-1.5">
            <span class="inline-flex items-center gap-1 rounded-md bg-muted/60 px-2.5 py-1 text-[11px] font-medium border border-border/50">
              <span class="text-muted-foreground">Provider:</span>
              <span class="font-semibold text-foreground">Google AI</span>
            </span>
            <span class="inline-flex items-center gap-1 rounded-md bg-muted/60 px-2.5 py-1 text-[11px] font-medium border border-border/50">
              <span class="text-muted-foreground">Model:</span>
              <span class="font-semibold text-foreground">gemini-1.5-pro</span>
              <span class="inline-flex items-center gap-0.5 px-1 rounded text-[9px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">Vision</span>
            </span>
            <span class="inline-flex items-center gap-1 rounded-md bg-muted/60 px-2.5 py-1 text-[11px] font-medium border border-border/50">
              <span class="text-muted-foreground">Verified:</span>
              <span class="font-semibold text-foreground">Just now</span>
            </span>
          </div>
        </div>
      </div>

      <!-- 2. Worker Pool Accordion (Deep Layer via Progressive Disclosure) -->
      <details class="group rounded-2xl border border-border/40 bg-card/20 p-4 transition-colors" open>
        <summary class="flex min-h-11 cursor-pointer items-center justify-between text-sm font-medium text-foreground select-none">
          <div class="flex items-center gap-2.5">
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-muted text-muted-foreground shrink-0">
              <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="16" height="16" x="4" y="4" rx="2"/><rect width="6" height="6" x="9" y="9" rx="1"/><path d="M15 2v2"/><path d="M15 20v2"/><path d="M2 15h2"/><path d="M2 9h2"/><path d="M20 15h2"/><path d="M20 9h2"/><path d="M9 2v2"/><path d="M9 20v2"/></svg>
            </div>
            <div class="min-w-0">
              <span class="font-semibold text-foreground text-xs sm:text-sm">Mesin Eksekusi (Worker Pool)</span>
              <p class="text-[11px] text-muted-foreground font-normal">4 worker internal siap untuk coding & vision</p>
            </div>
          </div>
          <span class="text-xs text-muted-foreground transition-transform group-open:rotate-90 shrink-0">▾</span>
        </summary>

        <div class="mt-3 pt-3 border-t border-border/40 space-y-2.5">
          <p class="text-[11px] text-muted-foreground leading-relaxed">
            Akun worker internal digunakan secara otonom oleh @cuan di balik layar untuk mengeksekusi tugas khusus dan auto-failover.
          </p>

          <div class="rounded-xl border border-border/40 bg-card/40 overflow-hidden divide-y divide-border/40">
            <div class="flex items-center justify-between p-3 text-xs hover:bg-muted/30 cursor-pointer transition-colors">
              <div class="flex items-center gap-2.5">
                <div class="h-7 w-7 rounded-lg bg-muted flex items-center justify-center text-xs">🤖</div>
                <div>
                  <span class="font-medium text-foreground">@dev2</span>
                  <span class="text-[10px] text-muted-foreground font-mono ml-1.5">agy</span>
                </div>
              </div>
              <div class="flex items-center gap-1.5 text-muted-foreground text-[11px]">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                <span>Worker · Detail ›</span>
              </div>
            </div>

            <div class="flex items-center justify-between p-3 text-xs hover:bg-muted/30 cursor-pointer transition-colors">
              <div class="flex items-center gap-2.5">
                <div class="h-7 w-7 rounded-lg bg-muted flex items-center justify-center text-xs">🤖</div>
                <div>
                  <span class="font-medium text-foreground">@dm</span>
                  <span class="text-[10px] text-muted-foreground font-mono ml-1.5">agy</span>
                </div>
              </div>
              <div class="flex items-center gap-1.5 text-muted-foreground text-[11px]">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                <span>Worker · Detail ›</span>
              </div>
            </div>

            <div class="flex items-center justify-between p-3 text-xs hover:bg-muted/30 cursor-pointer transition-colors">
              <div class="flex items-center gap-2.5">
                <div class="h-7 w-7 rounded-lg bg-muted flex items-center justify-center text-xs">🤖</div>
                <div>
                  <span class="font-medium text-foreground">@pin5</span>
                  <span class="text-[10px] text-muted-foreground font-mono ml-1.5">commandcode</span>
                </div>
              </div>
              <div class="flex items-center gap-1.5 text-muted-foreground text-[11px]">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                <span>Worker · Detail ›</span>
              </div>
            </div>

            <div class="flex items-center justify-between p-3 text-xs hover:bg-muted/30 cursor-pointer transition-colors">
              <div class="flex items-center gap-2.5">
                <div class="h-7 w-7 rounded-lg bg-muted flex items-center justify-center text-xs">🤖</div>
                <div>
                  <span class="font-medium text-foreground">@tim5</span>
                  <span class="text-[10px] text-muted-foreground font-mono ml-1.5">commandcode</span>
                </div>
              </div>
              <div class="flex items-center gap-1.5 text-muted-foreground text-[11px]">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                <span>Worker · Detail ›</span>
              </div>
            </div>
          </div>

          <div class="pt-1 flex items-center justify-between text-[11px] text-muted-foreground">
            <span>Butuh menambah engine AI?</span>
            <a href="#" class="text-primary hover:underline font-medium">Buka Penyedia AI →</a>
          </div>
        </div>
      </details>
    </main>

    <!-- Mobile Bottom Navigation: 5 Pillars -->
    <nav class="h-16 shrink-0 border-t border-border/40 bg-card/80 backdrop-blur grid grid-cols-5 items-center px-1">
      <div class="flex flex-col items-center justify-center gap-1 text-muted-foreground">
        <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
        <span class="text-[10px]">Chat</span>
      </div>
      <div class="flex flex-col items-center justify-center gap-1 text-muted-foreground">
        <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>
        <span class="text-[10px]">Tasks</span>
      </div>
      <div class="flex flex-col items-center justify-center gap-1 text-muted-foreground">
        <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7h-7L10 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2z"/></svg>
        <span class="text-[10px]">Projects</span>
      </div>
      <div class="flex flex-col items-center justify-center gap-1 text-muted-foreground">
        <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
        <span class="text-[10px]">Contacts</span>
      </div>
      <div class="flex flex-col items-center justify-center gap-1 text-primary font-medium">
        <div class="h-5 w-5 rounded-full bg-primary/20 text-primary flex items-center justify-center text-[10px] font-bold">BU</div>
        <span class="text-[10px]">Account</span>
      </div>
    </nav>
  </div>
</body>
</html>`
}

function createDesktopAgentsHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Settings - Agents (Desktop 1280px)</title>
  <style>${baseStyles}</style>
</head>
<body class="dark bg-background text-foreground min-h-screen">
  <div class="min-h-screen flex bg-background">
    <!-- Desktop Left Nav Sidebar -->
    <aside class="w-64 border-r border-border/40 p-4 space-y-4 shrink-0 bg-card/20">
      <div class="flex items-center gap-2 mb-6">
        <div class="h-7 w-7 rounded-lg bg-primary/20 text-primary flex items-center justify-center font-bold text-sm">C</div>
        <span class="font-bold text-base tracking-tight">cuan</span>
      </div>

      <div class="space-y-1 pt-2">
        <div class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground px-3 py-1">Workspace & Sistem</div>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Umum</a>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Jadwal Rutin</a>
        
        <div class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground px-3 py-1 pt-3">Kecerdasan AI</div>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Penyedia AI</a>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Memori Agen</a>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm bg-primary/10 text-primary font-medium">Agen & Tim</a>
        
        <div class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground px-3 py-1 pt-3">Konektivitas</div>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Aplikasi & Kemampuan</a>
      </div>
    </aside>

    <!-- Main Content Area: Focused Single Agent Canvas (max-w-3xl) -->
    <main class="flex-1 p-8 overflow-y-auto">
      <div class="max-w-3xl mx-auto space-y-6">
        <header class="pb-3 border-b border-border/40 flex items-center justify-between">
          <div>
            <h2 class="text-xl font-bold tracking-tight text-foreground">Asisten & Tim</h2>
            <p class="text-sm text-muted-foreground mt-0.5">Kelola asisten AI cerdas Anda dan kolaborator tim ruang kerja.</p>
          </div>
        </header>

        <!-- Tabs: Asisten AI (1) vs Anggota Tim (1) -->
        <div class="flex border-b border-border/40" role="tablist">
          <button class="flex items-center gap-2 px-4 py-3 text-sm font-semibold border-b-2 border-primary text-primary">
            <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>
            <span>Asisten AI</span>
            <span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">1</span>
          </button>
          <button class="flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 border-transparent text-muted-foreground hover:text-foreground">
            <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
            <span>Anggota Tim</span>
            <span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">1</span>
          </button>
        </div>

        <!-- 1. Primary Assistant Profile Card (Single Agent Facade @cuan) -->
        <div class="rounded-2xl border border-border/40 bg-card/30 p-6 space-y-5 transition-colors">
          <div class="flex items-center justify-between gap-4">
            <div class="flex items-center gap-4 min-w-0">
              <div class="relative shrink-0">
                <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 text-primary border border-primary/20 text-3xl font-semibold">
                  <svg class="h-8 w-8" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>
                </div>
                <div class="absolute -bottom-1 -right-1">
                  <span class="relative flex h-3.5 w-3.5 shrink-0 items-center justify-center">
                    <span class="relative inline-flex rounded-full h-3.5 w-3.5 bg-emerald-500"></span>
                  </span>
                </div>
              </div>
              <div class="min-w-0 space-y-1">
                <div class="flex items-center gap-2.5">
                  <h3 class="text-xl font-bold tracking-tight text-foreground">@cuan</h3>
                  <span class="font-mono text-xs text-muted-foreground">@cuan</span>
                  <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                    <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                    <span>Aktif & Siap Melayani</span>
                  </span>
                </div>
                <p class="text-sm text-muted-foreground">Cuan Universal Executive Assistant</p>
              </div>
            </div>

            <button class="inline-flex min-h-11 items-center gap-2 rounded-xl border border-border bg-background px-4 py-2 text-sm font-medium text-foreground hover:bg-muted active:scale-[0.98] transition-colors shrink-0">
              <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/></svg>
              <span>Edit Profil Asisten</span>
            </button>
          </div>

          <!-- Persona & Instruksi Kerja -->
          <div class="space-y-1.5 pt-3 border-t border-border/40">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider block">Peran & Instruksi Kerja</span>
            <div class="p-3.5 rounded-xl border border-border/30 bg-muted/20 text-sm text-foreground/90 leading-relaxed">
              Asisten bisnis terpadu untuk memandu proyek, merespons percakapan pelanggan, dan mengeksekusi otomatisasi operasional.
            </div>
          </div>

          <!-- Model AI & Status Mesin -->
          <div class="space-y-2 pt-3 border-t border-border/40">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider block">Model AI & Konfigurasi</span>
              <span class="text-xs text-muted-foreground hover:text-foreground font-medium cursor-pointer">Detail Teknis & Kuota ›</span>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <span class="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-xs font-medium border border-border/50">
                <span class="text-muted-foreground">Provider:</span>
                <span class="font-semibold text-foreground">Google AI</span>
              </span>
              <span class="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-xs font-medium border border-border/50">
                <span class="text-muted-foreground">Model:</span>
                <span class="font-semibold text-foreground">gemini-1.5-pro</span>
                <span class="inline-flex items-center gap-0.5 px-1 py-0.2 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">Vision</span>
              </span>
              <span class="inline-flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-xs font-medium border border-border/50">
                <span class="text-muted-foreground">Verified:</span>
                <span class="font-semibold text-foreground font-mono">Just now</span>
              </span>
            </div>
          </div>
        </div>

        <!-- 2. Worker Pool Accordion (Deep Layer via Progressive Disclosure) -->
        <details class="group rounded-2xl border border-border/40 bg-card/20 p-5 transition-colors">
          <summary class="flex min-h-11 cursor-pointer items-center justify-between text-sm font-medium text-foreground select-none">
            <div class="flex items-center gap-3">
              <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-muted text-muted-foreground shrink-0">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="16" height="16" x="4" y="4" rx="2"/><rect width="6" height="6" x="9" y="9" rx="1"/><path d="M15 2v2"/><path d="M15 20v2"/><path d="M2 15h2"/><path d="M2 9h2"/><path d="M20 15h2"/><path d="M20 9h2"/><path d="M9 2v2"/><path d="M9 20v2"/></svg>
              </div>
              <div>
                <span class="font-semibold text-foreground">Mesin Eksekusi Latar Belakang (Worker Pool)</span>
                <p class="text-xs text-muted-foreground font-normal mt-0.5">4 worker internal siap untuk coding & vision otomatis</p>
              </div>
            </div>
            <span class="text-xs text-muted-foreground transition-transform group-open:rotate-90">▸</span>
          </summary>

          <div class="mt-4 pt-3 border-t border-border/40 space-y-3">
            <p class="text-xs text-muted-foreground leading-relaxed">
              Akun-akun worker internal ini digunakan secara otonom oleh @cuan di balik layar untuk mengeksekusi tugas khusus (coding, terminal, vision) dan auto-failover saat kuota menipis.
            </p>

            <div class="rounded-xl border border-border/40 bg-card/40 overflow-hidden divide-y divide-border/40">
              <div class="flex items-center justify-between p-3.5 text-sm hover:bg-muted/30 cursor-pointer transition-colors">
                <div class="flex items-center gap-3">
                  <div class="h-8 w-8 rounded-lg bg-muted flex items-center justify-center text-sm">🤖</div>
                  <div>
                    <span class="font-medium text-foreground">@dev2</span>
                    <span class="text-xs text-muted-foreground font-mono ml-2">agy</span>
                  </div>
                </div>
                <div class="flex items-center gap-2 text-muted-foreground text-xs">
                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                  <span>Worker · Detail ›</span>
                </div>
              </div>

              <div class="flex items-center justify-between p-3.5 text-sm hover:bg-muted/30 cursor-pointer transition-colors">
                <div class="flex items-center gap-3">
                  <div class="h-8 w-8 rounded-lg bg-muted flex items-center justify-center text-sm">🤖</div>
                  <div>
                    <span class="font-medium text-foreground">@dm</span>
                    <span class="text-xs text-muted-foreground font-mono ml-2">agy</span>
                  </div>
                </div>
                <div class="flex items-center gap-2 text-muted-foreground text-xs">
                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                  <span>Worker · Detail ›</span>
                </div>
              </div>

              <div class="flex items-center justify-between p-3.5 text-sm hover:bg-muted/30 cursor-pointer transition-colors">
                <div class="flex items-center gap-3">
                  <div class="h-8 w-8 rounded-lg bg-muted flex items-center justify-center text-sm">🤖</div>
                  <div>
                    <span class="font-medium text-foreground">@pin5</span>
                    <span class="text-xs text-muted-foreground font-mono ml-2">commandcode</span>
                  </div>
                </div>
                <div class="flex items-center gap-2 text-muted-foreground text-xs">
                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                  <span>Worker · Detail ›</span>
                </div>
              </div>

              <div class="flex items-center justify-between p-3.5 text-sm hover:bg-muted/30 cursor-pointer transition-colors">
                <div class="flex items-center gap-3">
                  <div class="h-8 w-8 rounded-lg bg-muted flex items-center justify-center text-sm">🤖</div>
                  <div>
                    <span class="font-medium text-foreground">@tim5</span>
                    <span class="text-xs text-muted-foreground font-mono ml-2">commandcode</span>
                  </div>
                </div>
                <div class="flex items-center gap-2 text-muted-foreground text-xs">
                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                  <span>Worker · Detail ›</span>
                </div>
              </div>
            </div>

            <div class="pt-2 flex items-center justify-between text-xs text-muted-foreground">
              <span>Butuh menambah engine AI atau kuota baru?</span>
              <a href="#" class="text-primary hover:underline font-medium">Buka Penyedia AI →</a>
            </div>
          </div>
        </details>
      </div>
    </main>
  </div>
</body>
</html>`
}

async function capture() {
  const browser = await chromium.launch({
    headless: true,
    executablePath: path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium'),
    env: {
      ...process.env,
      LD_LIBRARY_PATH:
        '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' +
        (process.env.LD_LIBRARY_PATH || ''),
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox'],
  })

  // 1. Mobile viewport (390 x 844)
  const pageMobile = await browser.newPage({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  await pageMobile.setContent(createMobileAgentsHtml())
  const mobileScreenshotPath = path.join(screenshotsDir, 'agents-standardized-mobile-390px.png')
  await pageMobile.screenshot({
    path: mobileScreenshotPath,
    fullPage: false,
  })

  // 2. Desktop viewport (1280 x 800)
  const pageDesktop = await browser.newPage({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  })
  await pageDesktop.setContent(createDesktopAgentsHtml())
  const desktopScreenshotPath = path.join(screenshotsDir, 'agents-standardized-desktop.png')
  await pageDesktop.screenshot({
    path: desktopScreenshotPath,
    fullPage: false,
  })

  await browser.close()
  console.log('Screenshots captured successfully:', {
    mobileScreenshotPath,
    desktopScreenshotPath,
  })
}

capture().catch((err) => {
  console.error(err)
  process.exit(1)
})
