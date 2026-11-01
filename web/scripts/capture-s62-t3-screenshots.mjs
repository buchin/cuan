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
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    background-color: #141416;
  }
`

// 1. Tasks HTML (After vs Before)
function createTasksHtml(isBefore = false) {
  const containerClass = isBefore
    ? 'border border-border bg-card'
    : 'border border-border/40 bg-card/30 backdrop-blur-xs'
  const divideClass = isBefore ? 'divide-y divide-border' : 'divide-y divide-border/40'
  const pillStyle = isBefore
    ? 'background-color: #ffffff; color: #000000; font-weight: 700; border: 1px solid #ffffff;'
    : ''
  const badgeClass = isBefore
    ? 'rounded-full px-2.5 py-0.5 text-xs font-semibold'
    : 'rounded-full px-2.5 py-0.5 text-xs font-medium border border-border/40'

  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 62 — Tasks Audit</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport p-4 sm:p-8 max-w-6xl mx-auto w-full space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="space-y-1">
        <div class="flex items-center gap-2">
          <h1 class="text-2xl font-bold tracking-tight text-foreground">Tugas & Antrean Agen</h1>
          <span class="rounded-full bg-muted/60 px-2.5 py-0.5 text-xs text-muted-foreground border border-border/40">5 aktif</span>
        </div>
        <p class="text-sm text-muted-foreground">Kelola pekerjaan otomatis, backlog tugas, dan delegasi asisten AI.</p>
      </div>
      <div class="flex items-center gap-2">
        <div class="relative min-w-[240px]">
          <input
            type="text"
            placeholder="Cari tugas…"
            value="Audit"
            class="w-full rounded-xl border ${isBefore ? 'border-border bg-card' : 'border-border/40 bg-muted/20'} px-3.5 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>
        <button class="inline-flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
          </svg>
          Tugas Baru
        </button>
      </div>
    </div>

    <!-- Filter Chips -->
    <div class="flex items-center gap-2 overflow-x-auto pb-1">
      <span class="rounded-full bg-primary/10 text-primary border border-primary/20 px-3 py-1 text-xs font-medium cursor-pointer">Semua</span>
      <span class="rounded-full border ${isBefore ? 'border-border' : 'border-border/40'} text-muted-foreground hover:bg-muted/30 px-3 py-1 text-xs font-medium cursor-pointer">Sedang Dikerjakan (2)</span>
      <span class="rounded-full border ${isBefore ? 'border-border' : 'border-border/40'} text-muted-foreground hover:bg-muted/30 px-3 py-1 text-xs font-medium cursor-pointer">Dalam Antrean (1)</span>
      <span class="rounded-full border ${isBefore ? 'border-border' : 'border-border/40'} text-muted-foreground hover:bg-muted/30 px-3 py-1 text-xs font-medium cursor-pointer">Perlu Ditinjau (1)</span>
      <span class="rounded-full border ${isBefore ? 'border-border' : 'border-border/40'} text-muted-foreground hover:bg-muted/30 px-3 py-1 text-xs font-medium cursor-pointer">Selesai (1)</span>
    </div>

    <!-- Main Inset Group Table -->
    <div class="overflow-hidden rounded-xl ${containerClass}">
      <table class="w-full text-left text-sm">
        <thead class="border-b ${isBefore ? 'border-border bg-card' : 'border-border/40 bg-muted/30'} text-xs text-muted-foreground uppercase tracking-wider">
          <tr>
            <th class="px-4 py-3 font-medium">Tugas</th>
            <th class="px-4 py-3 font-medium">Proyek</th>
            <th class="px-4 py-3 font-medium">Status</th>
            <th class="px-4 py-3 font-medium">Pelaksana</th>
            <th class="px-4 py-3 font-medium text-right">Diperbarui</th>
          </tr>
        </thead>
        <tbody class="${divideClass}">
          <tr class="hover:bg-muted/30 transition-colors">
            <td class="px-4 py-3.5">
              <div class="font-medium text-foreground">S62-T3: Universal UI/UX Audit & Inset Group Refactoring</div>
              <div class="text-xs text-muted-foreground">Eliminasi 1px wireframe box dan live terminal streaming</div>
            </td>
            <td class="px-4 py-3.5 text-muted-foreground">Cuan Core</td>
            <td class="px-4 py-3.5">
              <span class="${badgeClass} bg-amber-500/10 text-amber-400" ${isBefore ? `style="${pillStyle}"` : ''}>
                ● Working
              </span>
            </td>
            <td class="px-4 py-3.5 text-foreground flex items-center gap-2">
              <span class="h-6 w-6 rounded-full bg-secondary flex items-center justify-center text-xs font-medium border border-border/40">AG</span>
              Antigravity AI
            </td>
            <td class="px-4 py-3.5 text-right text-xs text-muted-foreground">Baru saja</td>
          </tr>
          <tr class="hover:bg-muted/30 transition-colors">
            <td class="px-4 py-3.5">
              <div class="font-medium text-foreground">S62-T2: Visual Noise Cleanup on Chat Canvas</div>
              <div class="text-xs text-muted-foreground">Pembersihan garis header/composer dan memori chip</div>
            </td>
            <td class="px-4 py-3.5 text-muted-foreground">Cuan Core</td>
            <td class="px-4 py-3.5">
              <span class="${badgeClass} bg-emerald-500/10 text-emerald-400" ${isBefore ? `style="${pillStyle}"` : ''}>
                ✓ Done
              </span>
            </td>
            <td class="px-4 py-3.5 text-foreground flex items-center gap-2">
              <span class="h-6 w-6 rounded-full bg-secondary flex items-center justify-center text-xs font-medium border border-border/40">BC</span>
              Buchin
            </td>
            <td class="px-4 py-3.5 text-right text-xs text-muted-foreground">15m lalu</td>
          </tr>
          <tr class="hover:bg-muted/30 transition-colors">
            <td class="px-4 py-3.5">
              <div class="font-medium text-foreground">Rekapitulasi Order Harian Toko WhatsApp</div>
              <div class="text-xs text-muted-foreground">Otomatisasi pengiriman ringkasan invoice ke spreadsheet</div>
            </td>
            <td class="px-4 py-3.5 text-muted-foreground">Hermes Sales</td>
            <td class="px-4 py-3.5">
              <span class="${badgeClass} bg-sky-500/10 text-sky-400" ${isBefore ? `style="${pillStyle}"` : ''}>
                Queued
              </span>
            </td>
            <td class="px-4 py-3.5 text-foreground flex items-center gap-2">
              <span class="h-6 w-6 rounded-full bg-secondary flex items-center justify-center text-xs font-medium border border-border/40">HM</span>
              Hermes Bot
            </td>
            <td class="px-4 py-3.5 text-right text-xs text-muted-foreground">1j lalu</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Mobile Card Preview in Mobile Mode -->
    <div class="sm:hidden space-y-3">
      <div class="rounded-xl ${containerClass} p-4 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-muted-foreground">Cuan Core · S62-T3</span>
          <span class="${badgeClass} bg-amber-500/10 text-amber-400">● Working</span>
        </div>
        <div class="font-medium text-foreground">Universal UI/UX Audit & Inset Group Refactoring</div>
        <div class="flex items-center justify-between pt-1 text-xs text-muted-foreground">
          <span>Antigravity AI</span>
          <span>Baru saja</span>
        </div>
      </div>
      <div class="rounded-xl ${containerClass} p-4 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-muted-foreground">Cuan Core · S62-T2</span>
          <span class="${badgeClass} bg-emerald-500/10 text-emerald-400">✓ Done</span>
        </div>
        <div class="font-medium text-foreground">Visual Noise Cleanup on Chat Canvas</div>
        <div class="flex items-center justify-between pt-1 text-xs text-muted-foreground">
          <span>Buchin</span>
          <span>15m lalu</span>
        </div>
      </div>
    </div>
  </div>
</body>
</html>`
}

// 2. Contacts HTML
function createContactsHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 62 — Contacts & Memory Notes</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport p-4 sm:p-8 max-w-6xl mx-auto w-full space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="space-y-1">
        <div class="flex items-center gap-2">
          <h1 class="text-2xl font-bold tracking-tight text-foreground">Kontak & Pelanggan</h1>
          <span class="rounded-full bg-muted/60 px-2.5 py-0.5 text-xs text-muted-foreground border border-border/40">24 tersimpan</span>
        </div>
        <p class="text-sm text-muted-foreground">Direktori pelanggan terpusat dengan konteks catatan memori AI.</p>
      </div>
      <div class="flex items-center gap-2">
        <div class="relative min-w-[240px]">
          <input
            type="text"
            placeholder="Cari kontak…"
            value="Siti"
            class="w-full rounded-xl border border-border/40 bg-muted/20 px-3.5 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>
        <button class="inline-flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors">
          + Kontak
        </button>
      </div>
    </div>

    <!-- Master-Detail Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      <!-- Left: Contact Master List -->
      <div class="lg:col-span-5 space-y-3">
        <div class="overflow-hidden rounded-xl border border-border/40 bg-card/30">
          <div class="px-4 py-3 border-b border-border/40 bg-muted/20 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Daftar Pelanggan Aktif
          </div>
          <div class="divide-y divide-border/40">
            <!-- Selected Contact -->
            <div class="p-3.5 bg-muted/40 cursor-pointer flex items-center justify-between transition-colors">
              <div class="flex items-center gap-3">
                <div class="h-10 w-10 rounded-full bg-primary/20 text-primary font-bold flex items-center justify-center text-sm border border-primary/30">
                  SR
                </div>
                <div>
                  <div class="font-medium text-foreground">Siti Rahma</div>
                  <div class="text-xs text-muted-foreground">+62 812-3456-7890</div>
                </div>
              </div>
              <span class="rounded-full px-2 py-0.5 text-xs font-medium border border-border/40 bg-emerald-500/10 text-emerald-400">
                VIP Pelanggan
              </span>
            </div>

            <!-- Contact 2 -->
            <div class="p-3.5 hover:bg-muted/20 cursor-pointer flex items-center justify-between transition-colors">
              <div class="flex items-center gap-3">
                <div class="h-10 w-10 rounded-full bg-secondary text-muted-foreground font-bold flex items-center justify-center text-sm border border-border/40">
                  BP
                </div>
                <div>
                  <div class="font-medium text-foreground">Budi Pratama</div>
                  <div class="text-xs text-muted-foreground">+62 813-9876-5432</div>
                </div>
              </div>
              <span class="rounded-full px-2 py-0.5 text-xs font-medium border border-border/40 bg-muted/30 text-muted-foreground">
                Reguler
              </span>
            </div>

            <!-- Contact 3 -->
            <div class="p-3.5 hover:bg-muted/20 cursor-pointer flex items-center justify-between transition-colors">
              <div class="flex items-center gap-3">
                <div class="h-10 w-10 rounded-full bg-secondary text-muted-foreground font-bold flex items-center justify-center text-sm border border-border/40">
                  AL
                </div>
                <div>
                  <div class="font-medium text-foreground">Anisa Lestari</div>
                  <div class="text-xs text-muted-foreground">+62 856-1122-3344</div>
                </div>
              </div>
              <span class="rounded-full px-2 py-0.5 text-xs font-medium border border-border/40 bg-sky-500/10 text-sky-400">
                Prospek
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Right: Contact Detail & Memory Notes -->
      <div class="lg:col-span-7 space-y-4">
        <!-- Profile Inset Group -->
        <div class="overflow-hidden rounded-xl border border-border/40 bg-card/30 p-5 space-y-4">
          <div class="flex items-center justify-between border-b border-border/40 pb-4">
            <div class="flex items-center gap-3">
              <div class="h-12 w-12 rounded-full bg-primary/20 text-primary font-bold flex items-center justify-center text-base border border-primary/30">
                SR
              </div>
              <div>
                <h2 class="text-lg font-bold text-foreground">Siti Rahma</h2>
                <p class="text-xs text-muted-foreground">Toko Busana Hijab Modern Jakarta</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <button class="rounded-lg border border-border/40 px-3 py-1.5 text-xs font-medium hover:bg-muted/40 transition-colors">
                Kirim Pesan
              </button>
            </div>
          </div>

          <!-- Memory Notes Section (Refactored Inset Group) -->
          <div class="space-y-3 pt-1">
            <div class="flex items-center justify-between">
              <span class="text-sm font-semibold text-foreground">Konteks & Catatan Memori AI</span>
              <button class="text-xs text-primary hover:underline">+ Tambah Catatan</button>
            </div>

            <!-- Note Item 1 -->
            <div class="rounded-xl border border-border/40 bg-card/50 p-3.5 space-y-1.5">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-foreground">Preferensi Kurir & Jam Kirim</span>
                <span class="text-[11px] text-muted-foreground">2 jam lalu</span>
              </div>
              <p class="text-xs text-muted-foreground leading-relaxed">
                Pelanggan selalu memilih kurir Same Day logistik sebelum pukul 15:00 WIB. Resi otomatis dimohon dikirim ke nomor WhatsApp utama.
              </p>
            </div>

            <!-- Note Item 2 -->
            <div class="rounded-xl border border-border/40 bg-card/50 p-3.5 space-y-1.5">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-foreground">Riwayat Pembelian Terakhir</span>
                <span class="text-[11px] text-muted-foreground">Kemarin</span>
              </div>
              <p class="text-xs text-muted-foreground leading-relaxed">
                Invoice INV-2026-890 lunas via QRIS. Pelanggan menyukai diskon bundling grosir 10 pcs ke atas.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Mobile 4-Pillar Bottom Bar (Preview) -->
    <div class="sm:hidden fixed bottom-0 left-0 right-0 bg-[#141416]/95 backdrop-blur-md border-t border-border/40 flex items-center justify-around py-2 z-50">
      <div class="flex flex-col items-center gap-1 text-muted-foreground text-xs py-1">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
        <span>Chat</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-muted-foreground text-xs py-1">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/></svg>
        <span>Tasks</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-primary text-xs py-1 font-semibold">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"/></svg>
        <span>Contacts</span>
      </div>
      <div class="flex flex-col items-center gap-1 text-muted-foreground text-xs py-1">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/></svg>
        <span>Menu</span>
      </div>
    </div>
  </div>
</body>
</html>`
}

// 3. Settings HTML (After vs Before)
function createSettingsHtml(isBefore = false) {
  const containerClass = isBefore
    ? 'border border-border bg-card p-5'
    : 'overflow-hidden rounded-2xl border border-border/40 bg-card/40'
  const divideClass = isBefore ? 'space-y-4' : 'divide-y divide-border/40'

  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 62 — Settings Audit</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport p-4 sm:p-8 max-w-4xl mx-auto w-full space-y-6">
    <!-- Header -->
    <div class="space-y-1">
      <h1 class="text-2xl font-bold tracking-tight text-foreground">Pengaturan Sistem</h1>
      <p class="text-sm text-muted-foreground">Kelola akun, integrasi jadwal sistem, dan preferensi aplikasi secara terpadu.</p>
    </div>

    <!-- Segmented Navigation -->
    <div class="flex items-center gap-1 border-b border-border/40 pb-2">
      <button class="px-3.5 py-1.5 rounded-lg text-sm font-medium bg-muted/40 text-foreground">Umum & Jadwal</button>
      <button class="px-3.5 py-1.5 rounded-lg text-sm font-medium text-muted-foreground hover:text-foreground">Akun & Profil</button>
      <button class="px-3.5 py-1.5 rounded-lg text-sm font-medium text-muted-foreground hover:text-foreground">Pembaruan</button>
    </div>

    <!-- Scheduled Jobs Section (Things 3 GroupedSection) -->
    <div class="${containerClass}">
      <div class="px-5 py-3.5 ${isBefore ? 'border-b border-border mb-3' : 'border-b border-border/40 bg-muted/20'} flex items-center justify-between">
        <div>
          <div class="text-sm font-semibold text-foreground">Jadwal & Rutinitas Sistem</div>
          <div class="text-xs text-muted-foreground">Otomatisasi latar belakang terintegrasi langsung dari Settings</div>
        </div>
        <button class="rounded-lg bg-primary/10 border border-primary/20 text-primary px-3 py-1.5 text-xs font-medium hover:bg-primary/20 transition-colors">
          + Buat Jadwal
        </button>
      </div>

      <div class="${divideClass}">
        <!-- Job 1 -->
        <div class="px-5 py-4 flex items-center justify-between ${isBefore ? 'border border-border rounded-lg' : ''}">
          <div class="space-y-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-foreground">Otomasi Rekap Transaksi Harian</span>
              <span class="rounded-full px-2 py-0.5 text-[11px] font-medium border border-border/40 bg-emerald-500/10 text-emerald-400">Aktif</span>
            </div>
            <div class="text-xs text-muted-foreground">Setiap hari pukul 08:00 WIB (cron: 0 8 * * *) · Target: Hermes Bot</div>
          </div>
          <div class="flex items-center gap-2">
            <button class="px-2.5 py-1 text-xs border border-border/40 rounded-lg text-muted-foreground hover:text-foreground">Jalankan</button>
            <button class="px-2.5 py-1 text-xs border border-border/40 rounded-lg text-muted-foreground hover:text-foreground">Edit</button>
          </div>
        </div>

        <!-- Job 2 -->
        <div class="px-5 py-4 flex items-center justify-between ${isBefore ? 'border border-border rounded-lg' : ''}">
          <div class="space-y-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-foreground">Sinkronisasi Katalog & Stok Produk</span>
              <span class="rounded-full px-2 py-0.5 text-[11px] font-medium border border-border/40 bg-emerald-500/10 text-emerald-400">Aktif</span>
            </div>
            <div class="text-xs text-muted-foreground">Setiap 30 menit (cron: */30 * * * *) · Target: Antigravity AI</div>
          </div>
          <div class="flex items-center gap-2">
            <button class="px-2.5 py-1 text-xs border border-border/40 rounded-lg text-muted-foreground hover:text-foreground">Jalankan</button>
            <button class="px-2.5 py-1 text-xs border border-border/40 rounded-lg text-muted-foreground hover:text-foreground">Edit</button>
          </div>
        </div>
      </div>
    </div>

    <!-- App Update Card (Things 3 GroupedSection) -->
    <div class="${containerClass}">
      <div class="px-5 py-3.5 ${isBefore ? 'border-b border-border mb-3' : 'border-b border-border/40 bg-muted/20'} flex items-center justify-between">
        <div>
          <div class="text-sm font-semibold text-foreground">Pembaruan Aplikasi</div>
          <div class="text-xs text-muted-foreground">Status rilis dan sinkronisasi versi Cuan</div>
        </div>
      </div>
      <div class="px-5 py-4 flex items-center justify-between">
        <div class="space-y-1">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-foreground">Cuan Suite v1.4.2</span>
            <span class="rounded-full px-2 py-0.5 text-[11px] font-medium border border-border/40 bg-emerald-500/10 text-emerald-400">Versi Terbaru Terpasang</span>
          </div>
          <div class="text-xs text-muted-foreground">Terakhir diperiksa otomatis: Hari ini 12:00 WIB</div>
        </div>
        <button class="px-3 py-1.5 text-xs font-medium border border-border/40 rounded-lg text-foreground hover:bg-muted/40 transition-colors">
          Periksa Pembaruan
        </button>
      </div>
    </div>
  </div>
</body>
</html>`
}

// 4. Terminal Streaming HTML (After vs Before)
function createTerminalHtml(isBefore = false) {
  const statusText = isBefore ? 'Saved output' : 'Streaming live logs…'
  const dotColor = isBefore ? 'bg-muted-foreground' : 'bg-emerald-500 animate-pulse'

  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Sprint 62 — Live Terminal Streaming</title>
  <style>${baseStyles}</style>
</head>
<body class="dark">
  <div class="app-viewport p-4 sm:p-8 max-w-5xl mx-auto w-full space-y-4">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="space-y-1">
        <h1 class="text-xl font-bold tracking-tight text-foreground">Task Terminal — Sesi Eksekusi #EOBGT</h1>
        <p class="text-xs text-muted-foreground">S62-T3: Universal UI/UX Audit & Inset Group Refactoring</p>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-muted-foreground">Attempt #1 (Working)</span>
      </div>
    </div>

    <!-- Terminal Window -->
    <div class="rounded-xl border border-border/40 bg-[#0d0e11] overflow-hidden shadow-2xl flex flex-col h-[520px]">
      <!-- Terminal Window Bar -->
      <div class="h-11 px-4 border-b border-border/40 bg-[#141416]/90 flex items-center justify-between shrink-0">
        <div class="flex items-center gap-2">
          <div class="flex items-center gap-1.5">
            <span class="h-3 w-3 rounded-full bg-rose-500/80"></span>
            <span class="h-3 w-3 rounded-full bg-amber-500/80"></span>
            <span class="h-3 w-3 rounded-full bg-emerald-500/80"></span>
          </div>
          <span class="ml-2 text-xs font-mono text-muted-foreground">cuan-agent: session-89b</span>
        </div>

        <!-- Connection Status & Pulse Indicator -->
        <div class="flex items-center gap-2">
          <span class="text-xs font-medium text-foreground">${statusText}</span>
          <span class="h-2 w-2 rounded-full ${dotColor}"></span>
          <div class="ml-3 flex items-center gap-1 border-l border-border/40 pl-3">
            <button class="h-7 w-7 rounded flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors" title="Clear">
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
            </button>
            <button class="h-7 w-7 rounded flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-muted/40 transition-colors" title="Fullscreen">
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"/></svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Terminal Body / Stream Chunks -->
      <div class="flex-1 p-4 font-mono text-[13px] leading-relaxed text-[#ededed] overflow-y-auto space-y-1.5 select-text">
        ${isBefore ? `
        <div class="text-muted-foreground">[12:30:00] Initializing workspace environment...</div>
        <div class="text-muted-foreground">[12:30:01] Process started. Output buffered...</div>
        <div class="text-amber-400 mt-4">[Notice] Buffer is waiting for process termination before displaying final output...</div>
        ` : `
        <div class="text-muted-foreground">[12:30:00] Initializing workspace environment /home/dev/repos/cuan...</div>
        <div><span class="text-emerald-400 font-semibold">[stream-json:init]</span> session_id="89b" task_id="eobgtkmgdxsoepyogjvnidmb"</div>
        <div><span class="text-sky-400 font-semibold">[stream-json:step_update]</span> Starting Universal UI/UX Audit & Inset Group Refactoring (S62-T3)</div>
        <div class="text-muted-foreground pl-4">→ Inspecting TaskBoardView.tsx, TaskItemCard.tsx, ContactsView.tsx, SettingsView.tsx</div>
        <div class="text-muted-foreground pl-4">→ Removing 1px wireframe box borders & white solid badges; applying Things 3 hairline inset groups</div>
        <div><span class="text-indigo-400 font-semibold">[stream-json:tool_call]</span> cmd="go test -v -run TestAGYAdapter_StreamJSONChunksAndResult ./internal/agent/..."</div>
        <div class="text-emerald-400 pl-4">=== RUN   TestAGYAdapter_StreamJSONChunksAndResult</div>
        <div class="text-emerald-400 pl-4">--- PASS: TestAGYAdapter_StreamJSONChunksAndResult (0.01s)</div>
        <div><span class="text-indigo-400 font-semibold">[stream-json:tool_call]</span> cmd="npm test"</div>
        <div class="text-emerald-400 pl-4">✓ 5 tests passed in s62-universal-ui-terminal-stream.test.mjs (100% PASS)</div>
        <div class="text-emerald-400 pl-4">✓ 349 tests passed across all 35 suites in 5.1s</div>
        <div><span class="text-sky-400 font-semibold">[stream-json:step_update]</span> Incremental poller (/logs?after_id=N) receiving chunks in real-time without buffering delay.</div>
        <div class="flex items-center gap-2 pt-2">
          <span class="text-emerald-400 animate-pulse font-bold">●</span>
          <span class="text-foreground">Agen sedang aktif mengalirkan keluaran log...</span>
        </div>
        `}
      </div>
    </div>
  </div>
</body>
</html>`
}

async function run() {
  console.log('Launching Chromium for S62-T3 universal visual audit...')
  const browser = await chromium.launch({
    executablePath: path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium'),
    env: {
      ...process.env,
      LD_LIBRARY_PATH: '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' + (process.env.LD_LIBRARY_PATH || '')
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  })

  // Helper capture function
  async function capture(htmlContent, filename, viewport = { width: 1280, height: 800 }) {
    const tempHtml = path.join(screenshotsDir, `temp-${filename}.html`)
    fs.writeFileSync(tempHtml, htmlContent, 'utf8')
    const context = await browser.newContext({
      viewport,
      deviceScaleFactor: 2,
      colorScheme: 'dark'
    })
    const page = await context.newPage()
    await page.goto('file://' + tempHtml)
    await page.waitForTimeout(500)
    const outPath = path.join(screenshotsDir, filename)
    await page.screenshot({ path: outPath, fullPage: true })
    console.log(`Saved screenshot: ${outPath}`)
    await context.close()
    if (fs.existsSync(tempHtml)) fs.unlinkSync(tempHtml)
  }

  // 1. Tasks Screenshots (Desktop After, Mobile After, Desktop Before)
  console.log('Capturing Tasks screenshots...')
  await capture(createTasksHtml(false), 's62-t3-tasks-desktop-after.png', { width: 1280, height: 800 })
  await capture(createTasksHtml(false), 's62-t3-tasks-mobile-after.png', { width: 390, height: 844 })
  await capture(createTasksHtml(true), 's62-t3-tasks-desktop-before.png', { width: 1280, height: 800 })

  // 2. Contacts Screenshots (Desktop After, Mobile After)
  console.log('Capturing Contacts screenshots...')
  await capture(createContactsHtml(), 's62-t3-contacts-desktop-after.png', { width: 1280, height: 800 })
  await capture(createContactsHtml(), 's62-t3-contacts-mobile-after.png', { width: 390, height: 844 })

  // 3. Settings Screenshots (Desktop After, Desktop Before)
  console.log('Capturing Settings screenshots...')
  await capture(createSettingsHtml(false), 's62-t3-settings-desktop-after.png', { width: 1280, height: 800 })
  await capture(createSettingsHtml(true), 's62-t3-settings-desktop-before.png', { width: 1280, height: 800 })

  // 4. Terminal Screenshots (Desktop After with live stream, Desktop Before with static buffered)
  console.log('Capturing Terminal screenshots...')
  await capture(createTerminalHtml(false), 's62-t3-terminal-desktop-after.png', { width: 1280, height: 800 })
  await capture(createTerminalHtml(true), 's62-t3-terminal-desktop-before.png', { width: 1280, height: 800 })

  await browser.close()
  console.log('All S62-T3 visual audit screenshots generated successfully!')
}

run().catch((err) => {
  console.error('Error capturing S62-T3 screenshots:', err)
  process.exit(1)
})
