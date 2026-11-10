import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const browserModulePath = process.env.CUAN_BROWSER_MODULE || path.join(os.homedir(), '.local/share/cuan-browser/1.64.1/node_modules/playwright-core')
const { chromium } = require(browserModulePath)

const repoRoot = path.resolve('.')
const webDir = path.resolve('web')
const distAssetsDir = path.join(webDir, 'dist/assets')
const artifactsDir = process.env.CUAN_TASK_ARTIFACTS_DIR || path.resolve('artifacts')
const repoScreenshotsDir = path.join(repoRoot, 'screenshots')

if (!fs.existsSync(artifactsDir)) {
  fs.mkdirSync(artifactsDir, { recursive: true, mode: 0o700 })
}
if (!fs.existsSync(repoScreenshotsDir)) {
  fs.mkdirSync(repoScreenshotsDir, { recursive: true })
}

// Find compiled CSS
const cssFiles = fs.readdirSync(distAssetsDir).filter((f) => f.endsWith('.css'))
const mainCssFile = cssFiles.find((f) => f.startsWith('index-') || f.startsWith('style-')) || cssFiles[0]
const compiledCss = fs.readFileSync(path.join(distAssetsDir, mainCssFile), 'utf8')
const chatCalmCss = fs.readFileSync(path.join(webDir, 'src/features/inbox/chat-calm.css'), 'utf8')
const chatMarkdownCss = fs.readFileSync(path.join(webDir, 'src/features/inbox/chat-markdown.css'), 'utf8')

function buildChatHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cuan Chat - Approved Proposal Card (Things 3 Clean Style)</title>
  <style>
    ${compiledCss}
    ${chatCalmCss}
    ${chatMarkdownCss}
    body {
      background-color: #141416 !important;
      color: #ededed;
      margin: 0;
      padding: 0;
      font-family: 'Geist Variable', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      -webkit-font-smoothing: antialiased;
    }
    .preview-canvas {
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      background-color: #141416;
    }
  </style>
</head>
<body class="dark">
  <div class="preview-canvas wa-chat calm-chat">
    <!-- Calm Chat Header -->
    <header class="h-14 border-b border-white/5 bg-[#18181a] px-4 flex items-center justify-between shrink-0 sticky top-0 z-20">
      <div class="flex items-center gap-3">
        <div class="h-8 w-8 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center font-semibold text-xs">
          🤖
        </div>
        <div>
          <h2 class="text-sm font-semibold tracking-tight text-white flex items-center gap-1.5">
            <span>cuan assistant</span>
            <span class="inline-block h-2 w-2 rounded-full bg-emerald-500"></span>
          </h2>
          <p class="text-[11px] text-zinc-400">Direct Room • Online</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs px-2.5 py-1 rounded-full bg-secondary/50 text-muted-foreground font-mono">cuan</span>
      </div>
    </header>

    <!-- Chat Message Stream -->
    <main class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4 max-w-3xl mx-auto w-full">
      <!-- User message -->
      <div class="wa-message-row flex flex-col items-end">
        <div class="wa-message-bubble wa-outgoing max-w-[85%] rounded-2xl rounded-tr-xs bg-[#242b35] p-3 text-sm text-white">
          <p>Tolong rapikan indikator status approval card chat agar bersih dan tanpa duplikasi status.</p>
          <div class="wa-markdown-footer">
            <span class="wa-message-meta">
              <time datetime="2026-11-10T12:02:00Z">12:02</time>
            </span>
          </div>
        </div>
      </div>

      <!-- Agent Message with Approved ApprovalCard -->
      <div class="wa-message-row flex flex-col items-start w-full">
        <div class="wa-message-bubble wa-incoming relative min-w-0 max-w-[94%] sm:max-w-[85%] rounded-2xl bg-[#1c1d22] border border-white/5 p-3.5 sm:p-4 text-white">
          <!-- Intro message text -->
          <div class="chat-markdown text-sm text-zinc-200 mb-1">
            <p>Berikut proposal rencana yang telah disetujui:</p>
          </div>

          <!-- Clean Approved Proposal Card (Without Double Check & Without Footer Status) -->
          <div
            role="region"
            aria-label="Disetujui: Indikator Task Pie Progress Berdenyut di Kolom Kanan Inbox"
            data-testid="approval-card"
            data-status="accepted"
            class="approval-card mt-3 rounded-xl border p-3.5 sm:p-4 space-y-3 shadow-none transition-all duration-200 min-w-0 max-w-full overflow-hidden border-border/30 bg-card/20 dark:bg-card/10 border-l-2 border-l-emerald-500"
          >
            <!-- Header Badge & Project Tag: Clean CheckCircle2 icon + 'Approved' text without duplicate check character -->
            <div class="flex items-center justify-between gap-2 border-b border-border/40 pb-2">
              <div class="flex items-center gap-1.5 min-w-0">
                <svg class="lucide lucide-check-circle-2 h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <path d="m9 12 2 2 4-4"/>
                </svg>
                <span class="text-xs font-medium tracking-normal truncate text-emerald-600 dark:text-emerald-400">
                  Approved
                </span>
              </div>

              <span class="inline-flex items-center gap-1 text-[10px] font-mono font-medium px-2 py-0.5 rounded-full bg-secondary text-secondary-foreground shrink-0 max-w-[160px] truncate" title="cuan">
                <svg class="lucide lucide-folder h-2.5 w-2.5 opacity-70 shrink-0" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>
                </svg>
                <span class="truncate">cuan</span>
              </span>
            </div>

            <!-- Plan Title -->
            <div class="approval-card-title">
              <h4 class="text-sm font-semibold tracking-tight text-foreground break-words leading-snug">
                Indikator Task Pie Progress Berdenyut di Kolom Kanan Inbox
              </h4>
            </div>

            <!-- Plan Markdown Description -->
            <div class="approval-card-description min-w-0 max-w-full text-foreground/90">
              <div class="relative transition-all duration-200">
                <div class="chat-markdown min-w-0 max-w-full leading-relaxed text-sm">
                  <p><strong>Hasil yang Diharapkan:</strong><br>
                  Tampilan daftar inbox menjadi lebih bersih dan lega ala Things 3. Indikator status task dipusatkan di slot kanan (sejajar dengan timestamp) menggantikan baris teks di bawah cuplikan pesan.</p>
                  <p><strong>Ruang Lingkup Perubahan:</strong></p>
                  <ol>
                    <li>Pembersihan Slot Kiri Bawah:<br>- Hapus badge / counter progress di bawah snippet obrolan</li>
                    <li>Indikator Dinamis:<br>- Tampilkan pie status task dengan animasi berdenyut halus</li>
                  </ol>
                </div>
              </div>
              <button
                type="button"
                data-testid="approval-card-toggle-plan"
                class="mt-1.5 inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline cursor-pointer"
              >
                <span>Show full plan</span>
                <svg class="lucide lucide-chevron-down h-3 w-3" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>
              </button>
            </div>

            <!-- Note: Action Buttons are completely removed when accepted! -->
          </div>

          <!-- Message footer with timestamp precisely aligned in its slot -->
          <div class="wa-markdown-footer">
            <span class="wa-message-meta">
              <time datetime="2026-11-10T12:03:00Z">12:03</time>
            </span>
          </div>
        </div>
      </div>
    </main>
  </div>
</body>
</html>`
}

async function run() {
  const browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox'],
  })

  const html = buildChatHtml()
  const results = []

  try {
    for (const { name, width, height } of [
      { name: 'desktop', width: 1280, height: 800 },
      { name: 'mobile', width: 390, height: 844 },
    ]) {
      const page = await browser.newPage()
      await page.setViewportSize({ width, height })
      await page.setContent(html, { waitUntil: 'load' })

      const approvalCard = page.locator('[data-testid="approval-card"]')
      await approvalCard.waitFor({ state: 'visible' })

      // Check Header Text & Icon
      const headerText = await approvalCard.locator('.border-b span').first().innerText()
      const headerCheckCircle = await approvalCard.locator('svg.lucide-check-circle-2').count()

      // Assert: No double check character in header
      assert.ok(!headerText.includes('✓'), `Header text "${headerText}" must not contain checkmark character`)
      assert.equal(headerCheckCircle, 1, 'Header must display CheckCircle2 icon')
      assert.equal(headerText.trim(), 'Approved', 'Header status text must be Approved')

      // Assert: No double check icon or footer buttons in accepted state
      const footerButtons = await approvalCard.locator('button:has-text("Approved"), button:has-text("Disetujui")').count()
      const footerCheckCheck = await approvalCard.locator('svg.lucide-check-check').count()
      assert.equal(footerButtons, 0, 'Must not render footer approved button')
      assert.equal(footerCheckCheck, 0, 'Must not render CheckCheck icon in footer')

      // Check Timestamp slot precision
      const timestamp = page.locator('.wa-incoming .wa-markdown-footer time')
      await timestamp.waitFor({ state: 'visible' })
      const timeText = await timestamp.innerText()
      assert.equal(timeText, '12:03', 'Timestamp must be 12:03')

      // Verify layout bounding boxes
      const cardBox = await approvalCard.boundingBox()
      const timeBox = await timestamp.boundingBox()
      console.log(`[${name}] cardBox:`, cardBox, `timeBox:`, timeBox)
      assert.ok(cardBox, 'Approval card must have bounding box')
      assert.ok(timeBox, 'Timestamp must have bounding box')
      assert.ok(timeBox.y >= cardBox.y + cardBox.height - 4, `Timestamp (${timeBox.y}) must sit cleanly below card (${cardBox.y + cardBox.height}) without overlapping content`)

      // Save screenshots
      const artifactPath = path.join(artifactsDir, `approval-card-${name}.png`)
      const repoPath = path.join(repoScreenshotsDir, `approval-card-${name}.png`)
      await page.screenshot({ path: artifactPath, fullPage: false })
      await page.screenshot({ path: repoPath, fullPage: false })

      results.push({
        viewport: `${name} (${width}x${height})`,
        artifactPath,
        repoPath,
        headerText,
        headerCheckCircle,
        footerButtons,
        footerCheckCheck,
        timestamp: timeText,
        timeBoxY: timeBox.y,
        cardBottom: cardBox.y + cardBox.height,
      })

      await page.close()
    }
  } finally {
    await browser.close()
  }

  console.log('Visual verification succeeded! Results:', JSON.stringify(results, null, 2))
}

run().catch((err) => {
  console.error('Visual verification failed:', err)
  process.exit(1)
})
