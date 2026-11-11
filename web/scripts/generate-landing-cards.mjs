import fs from 'node:fs/promises'
import path from 'node:path'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
// Try playwright on mac, fallback to VPS cuan-browser
let playwright
try {
  playwright = require('playwright')
} catch {
  playwright = require('/home/ubuntu/.local/share/cuan-browser/1.64.1/node_modules/playwright-core')
}

const { chromium } = playwright

async function generate() {
  const browser = await chromium.launch({ headless: true })
  const page = await browser.newPage({
    viewport: { width: 1200, height: 800 },
    deviceScaleFactor: 2, // High DPI Retina
  })

  const html = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Archivo:wght@600;700&family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
  <script src="https://cdn.tailwindcss.com"></script>
  <script>
    tailwind.config = {
      darkMode: 'class',
      theme: {
        extend: {
          fontFamily: {
            sans: ['Inter', 'sans-serif'],
            display: ['Archivo', 'sans-serif'],
          }
        }
      }
    }
  </script>
  <style>
    body { font-family: 'Inter', sans-serif; background: #0c0a09; }
  </style>
</head>
<body class="p-10 flex flex-wrap gap-8 items-start justify-center">

  <!-- CARD 1: CHAT -->
  <div id="card-chat" class="w-[360px] rounded-xl border border-slate-200/90 bg-white p-5 shadow-sm text-slate-800 antialiased">
    <div class="flex items-center justify-between border-b border-slate-100 pb-3 mb-3.5">
      <div class="flex items-center gap-2">
        <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
        <span class="font-display font-bold text-slate-900 text-sm">@cuan</span>
        <span class="text-[11px] text-slate-400">AI Agent</span>
      </div>
      <span class="text-[10px] text-slate-400 font-mono">Baru saja</span>
    </div>

    <div class="space-y-3 text-xs">
      <!-- User Bubble -->
      <div class="rounded-lg bg-slate-50/90 p-3 border border-slate-100">
        <div class="text-[10px] font-semibold text-slate-500 mb-1 flex items-center gap-1.5">
          <span class="h-1.5 w-1.5 rounded-full bg-blue-500"></span>
          Anda (Owner)
        </div>
        <div class="text-slate-800 leading-relaxed font-normal">
          &ldquo;Tolong hubungkan webhook form order ke bot WhatsApp notifikasi admin.&rdquo;
        </div>
      </div>

      <!-- Agent Bubble -->
      <div class="rounded-lg bg-blue-50/40 p-3 border border-blue-100/70">
        <div class="text-[10px] font-semibold text-blue-600 mb-1 flex items-center gap-1.5">
          <span class="h-1.5 w-1.5 rounded-full bg-blue-600"></span>
          @cuan
        </div>
        <div class="text-slate-700 leading-relaxed font-normal">
          Siap! Konteks repositori dan webhook terbaca. Saya siapkan <strong>Approval Card</strong> rencana kerja dan verifikasinya sebelum menyentuh server.
        </div>
      </div>
    </div>

    <!-- Quick action chips -->
    <div class="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between">
      <span class="text-[10px] text-slate-400">Pilihan cepat:</span>
      <div class="flex gap-1.5">
        <span class="rounded-md bg-blue-50 border border-blue-200/60 px-2 py-0.5 text-[10px] font-medium text-blue-700">✓ Lanjutkan</span>
        <span class="rounded-md bg-slate-100 px-2 py-0.5 text-[10px] font-medium text-slate-600">Sesuaikan</span>
      </div>
    </div>
  </div>


  <!-- CARD 2: APPROVAL CARD -->
  <div id="card-approve" class="w-[360px] rounded-xl border border-slate-200/90 bg-white p-5 shadow-sm text-slate-800 antialiased">
    <div class="flex items-center justify-between border-b border-slate-100 pb-3 mb-3.5">
      <div class="flex items-center gap-2">
        <span class="h-2 w-2 rounded-full bg-amber-500"></span>
        <span class="font-display font-bold text-slate-900 text-sm">Approval Card</span>
      </div>
      <span class="rounded-full bg-amber-50 border border-amber-200/70 px-2 py-0.5 text-[10px] font-medium text-amber-800">Persetujuan Anda</span>
    </div>

    <div class="space-y-3 text-xs">
      <div>
        <div class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider">Cakupan Kerja</div>
        <div class="font-semibold text-slate-900 mt-0.5">Integrasi Webhook Form & Bot WhatsApp</div>
      </div>

      <div class="rounded-lg bg-slate-50/90 p-3 border border-slate-100 space-y-1.5">
        <div class="text-[10px] font-semibold text-slate-500 mb-1">Kriteria Verifikasi (DoD):</div>
        <div class="flex items-center gap-2 text-[11px] text-slate-700">
          <span class="text-emerald-600 font-bold">✓</span>
          <span>Endpoint webhook menerima payload order</span>
        </div>
        <div class="flex items-center gap-2 text-[11px] text-slate-700">
          <span class="text-emerald-600 font-bold">✓</span>
          <span>Format pesan WhatsApp order terstruktur</span>
        </div>
        <div class="flex items-center gap-2 text-[11px] text-slate-700">
          <span class="text-emerald-600 font-bold">✓</span>
          <span>Simulasi uji kirim notifikasi 100% sukses</span>
        </div>
      </div>
    </div>

    <div class="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between gap-2">
      <button class="w-full rounded-lg bg-blue-600 py-2 px-3 text-center text-xs font-medium text-white shadow-xs hover:bg-blue-700 transition">
        Setujui Rencana & Jalankan →
      </button>
    </div>
  </div>


  <!-- CARD 3: VERIFIED RESULT -->
  <div id="card-done" class="w-[360px] rounded-xl border border-slate-200/90 bg-white p-5 shadow-sm text-slate-800 antialiased">
    <div class="flex items-center justify-between border-b border-slate-100 pb-3 mb-3.5">
      <div class="flex items-center gap-2">
        <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
        <span class="font-display font-bold text-slate-900 text-sm">Status: Selesai</span>
      </div>
      <span class="rounded-full bg-emerald-50 border border-emerald-200/70 px-2.5 py-0.5 text-[10px] font-medium text-emerald-800">✓ 100% Teruji</span>
    </div>

    <div class="space-y-3 text-xs">
      <div>
        <div class="text-[10px] font-semibold text-slate-400 uppercase tracking-wider">Hasil Eksekusi</div>
        <div class="font-semibold text-slate-900 mt-0.5">Webhook Form & WhatsApp Berhasil Aktif</div>
      </div>

      <div class="rounded-lg bg-emerald-50/30 p-3 border border-emerald-100/60 space-y-1.5">
        <div class="text-[10px] font-semibold text-emerald-800 mb-1">Bukti Kualitas (Quality Gate):</div>
        <div class="flex items-center gap-2 text-[11px] text-slate-700">
          <span class="text-emerald-600 font-bold">✓</span>
          <span>3 pengujian otomatis lulus tanpa error</span>
        </div>
        <div class="flex items-center gap-2 text-[11px] text-slate-700">
          <span class="text-emerald-600 font-bold">✓</span>
          <span>Bukti visual tangkapan layar terverifikasi</span>
        </div>
        <div class="flex items-center gap-2 text-[11px] text-slate-700">
          <span class="text-emerald-600 font-bold">✓</span>
          <span>Deploy aman ke VPS tanpa downtime</span>
        </div>
      </div>
    </div>

    <div class="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between">
      <span class="text-[10px] text-slate-400 font-mono">ID: #msg_e87e92</span>
      <span class="text-[11px] font-medium text-blue-600 hover:text-blue-700 cursor-pointer">Lihat Laporan &rarr;</span>
    </div>
  </div>

</body>
</html>
  `

  await page.setContent(html, { waitUntil: 'networkidle' })

  const outDir = path.resolve('web/public/landing')
  await fs.mkdir(outDir, { recursive: true })

  // Capture each card
  const cardChat = await page.$('#card-chat')
  await cardChat.screenshot({ path: path.join(outDir, 'step-chat.png') })

  const cardApprove = await page.$('#card-approve')
  await cardApprove.screenshot({ path: path.join(outDir, 'step-approve.png') })

  const cardDone = await page.$('#card-done')
  await cardDone.screenshot({ path: path.join(outDir, 'step-done.png') })

  await browser.close()
  console.log('Successfully generated step-chat.png, step-approve.png, step-done.png!')
}

generate().catch(err => {
  console.error(err)
  process.exit(1)
})
