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
    <!-- Mobile App Header -->
    <header class="h-14 shrink-0 border-b border-border/40 px-4 flex items-center justify-between bg-card/60 backdrop-blur">
      <div class="flex items-center gap-2">
        <div class="h-6 w-6 rounded-full border-2 border-foreground flex items-center justify-center">
          <div class="h-2 w-2 rounded-full bg-foreground"></div>
        </div>
        <h1 class="text-base font-semibold">Agents</h1>
      </div>
      <div class="flex items-center gap-2">
        <button class="h-9 w-9 rounded-lg bg-primary/10 text-primary flex items-center justify-center text-lg font-medium">+</button>
        <button class="h-9 w-9 text-muted-foreground flex items-center justify-center text-lg">⋮</button>
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="flex-1 p-4 space-y-4 overflow-y-auto">
      <!-- Unified Single Mobile Back Button (Clean, No Pill, No Double Stack) -->
      <div>
        <a href="#" class="inline-flex min-h-11 items-center gap-1.5 rounded-md text-sm font-medium text-muted-foreground hover:text-foreground transition-colors">
          <svg class="h-4 w-4 shrink-0" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
          <span>Kembali ke percakapan</span>
        </a>
      </div>

      <!-- Tabs: Asisten AI (8) vs Anggota Tim (1) -->
      <div class="flex border-b border-border/60" role="tablist">
        <button class="flex items-center gap-2 px-4 py-2.5 text-sm font-semibold border-b-2 border-primary text-primary">
          <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>
          <span>Asisten AI</span>
          <span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">8</span>
        </button>
        <button class="flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 border-transparent text-muted-foreground hover:text-foreground">
          <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
          <span>Anggota Tim</span>
          <span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">1</span>
        </button>
      </div>

      <!-- Calm Search Bar -->
      <div class="relative min-w-0 flex-1">
        <svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
        <input class="field-control pl-9 w-full" placeholder="Cari nama atau tipe agen" value="" readonly />
      </div>

      <!-- HairlineList Container: Zero harsh box frames, clean subtle hairline separators -->
      <div class="rounded-xl border border-border/40 bg-card/40 overflow-hidden transition-colors">
        <div role="list" class="w-full divide-y divide-border/40">
          
          <!-- Agent 1: @support (Single title, no duplicate, calm trailing menu) -->
          <div role="listitem" class="group flex items-center gap-3 py-3 px-3 sm:px-4 min-h-[44px] text-sm transition-colors cursor-pointer hover:bg-muted/30">
            <div class="relative shrink-0">
              <div class="flex h-10 w-10 sm:h-11 sm:w-11 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 text-base font-semibold">
                <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>
              </div>
              <div class="absolute -bottom-0.5 -right-0.5">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-foreground tracking-tight">
                <h3 class="break-words text-sm sm:text-base font-semibold text-foreground">@support</h3>
              </div>
              <div class="truncate text-xs text-muted-foreground mt-0.5">
                Customer Support & Helpdesk
              </div>
            </div>
            <div class="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
              <button class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
              </button>
            </div>
          </div>

          <!-- Agent 2: @dev2 (Clean single title, no @dev2 @dev2 duplicate glitch) -->
          <div role="listitem" class="group flex items-center gap-3 py-3 px-3 sm:px-4 min-h-[44px] text-sm transition-colors cursor-pointer hover:bg-muted/30">
            <div class="relative shrink-0">
              <div class="flex h-10 w-10 sm:h-11 sm:w-11 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 text-base font-semibold">
                🤖
              </div>
              <div class="absolute -bottom-0.5 -right-0.5">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-foreground tracking-tight">
                <h3 class="break-words text-sm sm:text-base font-semibold text-foreground">@dev2</h3>
              </div>
              <div class="truncate text-xs text-muted-foreground mt-0.5">
                Development Team (Backend Runner)
              </div>
            </div>
            <div class="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
              <button class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
              </button>
            </div>
          </div>

          <!-- Agent 3: @dev (Clean single title) -->
          <div role="listitem" class="group flex items-center gap-3 py-3 px-3 sm:px-4 min-h-[44px] text-sm transition-colors cursor-pointer hover:bg-muted/30">
            <div class="relative shrink-0">
              <div class="flex h-10 w-10 sm:h-11 sm:w-11 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 text-base font-semibold">
                🤖
              </div>
              <div class="absolute -bottom-0.5 -right-0.5">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-foreground tracking-tight">
                <h3 class="break-words text-sm sm:text-base font-semibold text-foreground">@dev</h3>
              </div>
              <div class="truncate text-xs text-muted-foreground mt-0.5">
                Development Team (Frontend Architect)
              </div>
            </div>
            <div class="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
              <button class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
              </button>
            </div>
          </div>

          <!-- Agent 4: @dm (Clean single title) -->
          <div role="listitem" class="group flex items-center gap-3 py-3 px-3 sm:px-4 min-h-[44px] text-sm transition-colors cursor-pointer hover:bg-muted/30">
            <div class="relative shrink-0">
              <div class="flex h-10 w-10 sm:h-11 sm:w-11 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 text-base font-semibold">
                🤖
              </div>
              <div class="absolute -bottom-0.5 -right-0.5">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-foreground tracking-tight">
                <h3 class="break-words text-sm sm:text-base font-semibold text-foreground">@dm</h3>
              </div>
              <div class="truncate text-xs text-muted-foreground mt-0.5">
                Delivery Manager & Sprint Lead
              </div>
            </div>
            <div class="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
              <button class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
              </button>
            </div>
          </div>

        </div>
      </div>
    </main>

    <!-- Bottom Navigation Bar -->
    <nav class="h-16 border-t border-border/40 bg-card/60 backdrop-blur px-6 flex items-center justify-between text-muted-foreground">
      <div class="flex flex-col items-center gap-1 text-xs">
        <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/></svg>
        <span>Chat</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-xs">
        <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 11 3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>
        <span>Tasks</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-xs">
        <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M7 7h10"/><path d="M7 12h10"/><path d="M7 17h10"/></svg>
        <span>Projects</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-xs">
        <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/></svg>
        <span>Contacts</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-xs text-foreground font-semibold">
        <div class="h-5 w-5 rounded-full bg-primary/20 text-primary flex items-center justify-center text-[10px] font-bold">BU</div>
        <span>Account</span>
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
    <!-- Desktop Left Nav Sidebar (Master Navigation) -->
    <aside class="w-64 border-r border-border/40 p-4 space-y-4 shrink-0 bg-card/20">
      <div class="flex items-center gap-2 mb-6">
        <div class="h-7 w-7 rounded-lg bg-primary/20 text-primary flex items-center justify-center font-bold text-sm">C</div>
        <span class="font-bold text-base tracking-tight">Cuan Workspace</span>
      </div>

      <!-- SafeReturn link in sidebar -->
      <a href="#" class="inline-flex min-h-11 items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors w-full">
        <svg class="h-4 w-4 shrink-0" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>
        <span>Kembali ke percakapan</span>
      </a>

      <div class="space-y-1 pt-2">
        <div class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground px-3 py-1">Pengaturan</div>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Umum</a>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm bg-primary/10 text-primary font-medium">Asisten & Tim</a>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">AI Providers</a>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Memori Agen</a>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Jadwal & Cron</a>
        <a href="#" class="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-accent/50">Integrasi</a>
      </div>
    </aside>

    <!-- Main Detail Panel -->
    <main class="flex-1 p-8 overflow-y-auto max-w-4xl space-y-6">
      <header class="pb-3 border-b border-border/40 flex items-center justify-between">
        <div>
          <h2 class="text-xl font-bold tracking-tight text-foreground">Asisten & Tim</h2>
          <p class="text-sm text-muted-foreground mt-0.5">Kelola agen AI, routing model, dan staf ruang kerja Anda.</p>
        </div>
        <button class="inline-flex items-center gap-2 rounded-lg bg-primary px-3.5 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
          <span>+ Tambah Agen</span>
        </button>
      </header>

      <!-- Tabs: Asisten AI (8) vs Anggota Tim (1) -->
      <div class="flex border-b border-border/60" role="tablist">
        <button class="flex items-center gap-2 px-4 py-2.5 text-sm font-semibold border-b-2 border-primary text-primary">
          <span>Asisten AI</span>
          <span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">8</span>
        </button>
        <button class="flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 border-transparent text-muted-foreground hover:text-foreground">
          <span>Anggota Tim</span>
          <span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground font-mono">1</span>
        </button>
      </div>

      <!-- Search Bar -->
      <div class="relative min-w-0 max-w-md">
        <svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
        <input class="field-control pl-9 w-full" placeholder="Cari nama atau tipe agen" value="" readonly />
      </div>

      <!-- HairlineList Container Desktop (Includes Direct Pencil and AgentActions) -->
      <div class="rounded-xl border border-border/40 bg-card/40 overflow-hidden transition-colors">
        <div role="list" class="w-full divide-y divide-border/40">
          
          <!-- Agent 1: @support -->
          <div role="listitem" class="group flex items-center gap-3 py-3 px-4 min-h-[44px] text-sm transition-colors cursor-pointer hover:bg-muted/30">
            <div class="relative shrink-0">
              <div class="flex h-11 w-11 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 text-base font-semibold">
                <svg class="h-5 w-5" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>
              </div>
              <div class="absolute -bottom-0.5 -right-0.5">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-foreground tracking-tight">
                <h3 class="break-words text-base font-semibold text-foreground">@support</h3>
              </div>
              <div class="truncate text-xs text-muted-foreground mt-0.5">
                Customer Support & Helpdesk
              </div>
            </div>
            <div class="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
              <button class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors" title="Edit @support">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/></svg>
              </button>
              <button class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
              </button>
            </div>
          </div>

          <!-- Agent 2: @dev2 -->
          <div role="listitem" class="group flex items-center gap-3 py-3 px-4 min-h-[44px] text-sm transition-colors cursor-pointer hover:bg-muted/30">
            <div class="relative shrink-0">
              <div class="flex h-11 w-11 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 text-base font-semibold">
                🤖
              </div>
              <div class="absolute -bottom-0.5 -right-0.5">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-foreground tracking-tight">
                <h3 class="break-words text-base font-semibold text-foreground">@dev2</h3>
              </div>
              <div class="truncate text-xs text-muted-foreground mt-0.5">
                Development Team (Backend Runner)
              </div>
            </div>
            <div class="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
              <button class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors" title="Edit @dev2">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/></svg>
              </button>
              <button class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
              </button>
            </div>
          </div>

          <!-- Agent 3: @dev -->
          <div role="listitem" class="group flex items-center gap-3 py-3 px-4 min-h-[44px] text-sm transition-colors cursor-pointer hover:bg-muted/30">
            <div class="relative shrink-0">
              <div class="flex h-11 w-11 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 text-base font-semibold">
                🤖
              </div>
              <div class="absolute -bottom-0.5 -right-0.5">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-foreground tracking-tight">
                <h3 class="break-words text-base font-semibold text-foreground">@dev</h3>
              </div>
              <div class="truncate text-xs text-muted-foreground mt-0.5">
                Development Team (Frontend Architect)
              </div>
            </div>
            <div class="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
              <button class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors" title="Edit @dev">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/></svg>
              </button>
              <button class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
              </button>
            </div>
          </div>

          <!-- Agent 4: @dm -->
          <div role="listitem" class="group flex items-center gap-3 py-3 px-4 min-h-[44px] text-sm transition-colors cursor-pointer hover:bg-muted/30">
            <div class="relative shrink-0">
              <div class="flex h-11 w-11 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20 text-base font-semibold">
                🤖
              </div>
              <div class="absolute -bottom-0.5 -right-0.5">
                <span class="relative flex h-2 w-2 shrink-0 items-center justify-center">
                  <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              </div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-foreground tracking-tight">
                <h3 class="break-words text-base font-semibold text-foreground">@dm</h3>
              </div>
              <div class="truncate text-xs text-muted-foreground mt-0.5">
                Delivery Manager & Sprint Lead
              </div>
            </div>
            <div class="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
              <button class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors" title="Edit @dm">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/></svg>
              </button>
              <button class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors">
                <svg class="h-4 w-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></svg>
              </button>
            </div>
          </div>

        </div>
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
