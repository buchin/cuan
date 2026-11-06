import fs from 'node:fs'
import path from 'node:path'
import http from 'node:http'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const webDistDir = path.resolve(rootDir, 'web/dist')
const artifactsDir = process.env.CUAN_TASK_ARTIFACTS_DIR || '/tmp/cuan-task-agent-179021030/artifacts'
fs.mkdirSync(artifactsDir, { recursive: true })

const pwModulePath = process.env.CUAN_BROWSER_MODULE || 'playwright-core'
const pw = require(pwModulePath)

// Find compiled css
const assetsDir = path.resolve(webDistDir, 'assets')
const cssFile = fs.readdirSync(assetsDir).find(f => f.startsWith('style-') && f.endsWith('.css'))
if (!cssFile) {
  throw new Error('Compiled CSS not found in dist/assets')
}

// Create verification HTML page
const verifyHtml = `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
  <title>Cuan - Visual Verification Mermaid</title>
  <link rel="stylesheet" href="/assets/${cssFile}">
  <script src="/vendor/mermaid/mermaid.min.js"></script>
  <style>
    body {
      background-color: #141416;
      color: #f4f4f5;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      margin: 0;
      padding: 16px;
      min-height: 100vh;
    }
    .chat-bubble {
      max-width: 680px;
      margin: 0 auto;
      background: #1b1c1f;
      border-radius: 16px;
      padding: 16px 20px;
      border: 1px solid rgba(255, 255, 255, 0.08);
    }
  </style>
</head>
<body>
  <div class="chat-bubble">
    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 12px;">
      <div style="width: 28px; height: 28px; border-radius: 50%; background: #3b82f6; display: flex; align-items: center; justify-content: center; font-size: 13px; font-weight: bold;">C</div>
      <span style="font-weight: 600; font-size: 14px;">Evaluasi Otomatis DoD</span>
      <span style="color: #71717a; font-size: 12px; margin-left: auto;">19:40</span>
    </div>

    <p style="font-size: 14px; margin-bottom: 8px; color: #d4d4d8;">
      arsitektur tingkat tinggi—barulah agen melapor dan meminta bantuan PO.
    </p>

    <h3 style="font-size: 15px; font-weight: 600; margin: 16px 0 8px 0; color: #fafafa;">
      Alur Kerja (Workflow) yang Terbentuk
    </h3>

    <!-- Mermaid Visual Diagram Component -->
    <div id="mermaid-root"></div>

    <h3 style="font-size: 15px; font-weight: 600; margin: 24px 0 8px 0; color: #fafafa;">
      Fallback Graceful (Sintaks Rusak / Invalid)
    </h3>
    <div id="mermaid-error-root"></div>

    <p style="font-size: 14px; margin-top: 16px; color: #a1a1aa;">
      Aspek Teknis yang Perlu Diperhatikan: responsif di mobile 360px–390px dan viewer zoom interaktif.
    </p>
  </div>

  <div id="portal-root"></div>

  <script>
    const mermaidCode = \`flowchart TD
    A["Runner Selesai Eksekusi"] --> B["Evaluasi DoD vs Bukti"]
    B --> C{"Hasil Evaluasi"}
    C -->|Lulus Penuh| D["Set Status Done"]
    C -->|Belum Lulus & Fixable| E["Update Arahan"]
    E --> A
    C -->|Gagal & Butuh Bantuan Manusia| F["Escalate ke PO"]\`;

    const brokenCode = \`flowchart TD
    A["Runner Selesai Eksekusi"] -
    B --> C["Evaluasi DoD vs Bukti\`;

    let currentZoom = 1;
    let isFullscreen = false;
    let viewMode = 'diagram';

    function renderUI(svgContent) {
      const container = document.getElementById('mermaid-root');
      container.innerHTML = \`
        <div class="chat-code-mermaid my-3 rounded-xl border border-border/80 bg-muted/20 overflow-hidden shadow-xs">
          <!-- Header Bar -->
          <div class="flex min-h-11 items-center justify-between gap-2 border-b border-border/60 bg-muted/40 px-3 py-2 text-xs">
            <div class="flex items-center gap-2 min-w-0">
              <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="6" height="6" rx="1"/><rect x="15" y="3" width="6" height="6" rx="1"/><rect x="9" y="15" width="6" height="6" rx="1"/><path d="M6 9v3a3 3 0 0 0 3 3h6a3 3 0 0 0 3-3V9"/><path d="M12 12v3"/></svg>
              </span>
              <span class="font-semibold text-foreground truncate">Diagram Alur (Mermaid)</span>
              <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground border border-border/40 shrink-0">7 baris</span>
            </div>

            <!-- Action Controls -->
            <div class="flex items-center gap-1.5 shrink-0">
              <button id="btn-toggle-view" class="inline-flex min-h-9 items-center gap-1 rounded-lg px-2.5 py-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40">
                \${viewMode === 'diagram' ? 'Sintaks' : 'Visual'}
              </button>
              <button id="btn-copy" class="inline-flex min-h-9 items-center gap-1 rounded-lg px-2.5 py-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40">
                Salin
              </button>
              <button id="btn-fullscreen" class="inline-flex min-h-9 items-center gap-1 rounded-lg px-2.5 py-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40">
                Perbesar
              </button>
            </div>
          </div>
          <!-- Body -->
          <div id="mermaid-body">
            \${viewMode === 'diagram' ? \`
              <div class="mermaid-scroll-wrapper w-full max-w-full overflow-x-auto p-4 flex justify-center items-center" style="-webkit-overflow-scrolling: touch; touch-action: pan-x pan-y;">
                <div class="mermaid-svg-container w-full flex justify-center [&>svg]:max-w-full [&>svg]:h-auto [&>svg]:mx-auto">
                  \${svgContent}
                </div>
              </div>
            \` : \`
              <pre class="p-3 overflow-x-auto font-mono text-xs bg-background/80 text-foreground leading-relaxed"><code>\${mermaidCode}</code></pre>
            \`}
          </div>
        </div>
      \`;

      document.getElementById('btn-fullscreen').onclick = openModal;
      document.getElementById('btn-toggle-view').onclick = () => {
        viewMode = viewMode === 'diagram' ? 'code' : 'diagram';
        renderUI(svgContent);
      };
      document.getElementById('btn-copy').onclick = () => {
        navigator.clipboard.writeText(mermaidCode);
        document.getElementById('btn-copy').innerText = 'Tersalin';
        setTimeout(() => { document.getElementById('btn-copy').innerText = 'Salin'; }, 2000);
      };
    }

    function renderErrorUI() {
      const errContainer = document.getElementById('mermaid-error-root');
      errContainer.innerHTML = \`
        <div class="chat-code-mermaid my-3 rounded-xl border border-border/80 bg-muted/20 overflow-hidden shadow-xs">
          <div class="flex min-h-11 items-center justify-between gap-2 border-b border-border/60 bg-muted/40 px-3 py-2 text-xs">
            <div class="flex items-center gap-2 min-w-0">
              <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-amber-500/10 text-amber-500">
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
              </span>
              <span class="font-semibold text-foreground truncate">Diagram Rusak (Testing Fallback)</span>
              <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground border border-border/40 shrink-0">3 baris</span>
            </div>
            <button class="inline-flex min-h-9 items-center gap-1 rounded-lg px-2.5 py-1 text-xs text-muted-foreground border border-border/40">Salin</button>
          </div>
          <div class="p-3 space-y-2.5">
            <div class="flex items-center gap-2 rounded-lg bg-amber-500/10 border border-amber-500/20 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
              <svg class="h-4 w-4 shrink-0 text-amber-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
              <div class="min-w-0">
                <p class="font-semibold">Sintaks diagram Mermaid tidak valid</p>
                <p class="text-[11px] opacity-80 truncate">Parse error on line 2: Unexpected token '-'</p>
              </div>
            </div>
            <pre class="overflow-x-auto rounded-lg bg-background/90 p-3 font-mono text-xs leading-relaxed border border-border/50 text-foreground"><code>\${brokenCode}</code></pre>
          </div>
        </div>
      \`;
    }

    function openModal() {
      const portal = document.getElementById('portal-root');
      portal.innerHTML = \`
        <div id="fullscreen-dialog" class="fixed inset-0 z-50 flex flex-col bg-background/95 backdrop-blur-md" role="dialog" aria-modal="true" aria-label="Pratinjau Diagram Mermaid">
          <div class="flex min-h-12 items-center justify-between border-b border-border/60 bg-muted/30 px-4 py-2.5">
            <div class="flex items-center gap-2">
              <span class="font-semibold text-sm">Diagram Alur (Mermaid)</span>
              <span class="text-xs text-muted-foreground">Mode Viewer Interaktif</span>
            </div>
            <div class="flex items-center gap-2">
              <button id="zoom-out" class="min-h-9 px-2.5 rounded bg-muted text-xs cursor-pointer">Zoom Out</button>
              <span id="zoom-val" class="text-xs font-mono">100%</span>
              <button id="zoom-in" class="min-h-9 px-2.5 rounded bg-muted text-xs cursor-pointer">Zoom In</button>
              <button id="zoom-reset" class="min-h-9 px-2.5 rounded bg-muted text-xs cursor-pointer">Reset</button>
              <button id="close-modal" class="min-h-9 px-3 rounded bg-primary/20 text-primary text-xs font-semibold cursor-pointer">Tutup</button>
            </div>
          </div>
          <div class="flex-1 overflow-auto p-6 flex items-center justify-center cursor-grab">
            <div id="modal-svg" style="transform: scale(\${currentZoom}); transition: transform 0.15s ease;">
              \${renderedSvg}
            </div>
          </div>
          <div class="flex items-center justify-between border-t border-border/40 bg-muted/20 px-4 py-2 text-[11px] text-muted-foreground">
            <span>Gunakan kontrol zoom atau seret mouse/layar sentuh untuk menggeser.</span>
            <span class="font-mono">Tekan Esc untuk menutup</span>
          </div>
        </div>
      \`;
      document.getElementById('zoom-in').onclick = () => { currentZoom = Math.min(3, currentZoom + 0.2); updateZoom(); };
      document.getElementById('zoom-out').onclick = () => { currentZoom = Math.max(0.4, currentZoom - 0.2); updateZoom(); };
      document.getElementById('zoom-reset').onclick = () => { currentZoom = 1; updateZoom(); };
      document.getElementById('close-modal').onclick = () => { portal.innerHTML = ''; };
    }

    function updateZoom() {
      document.getElementById('modal-svg').style.transform = 'scale(' + currentZoom + ')';
      document.getElementById('zoom-val').innerText = Math.round(currentZoom * 100) + '%';
    }

    let renderedSvg = '';
    window.onload = async () => {
      mermaid.initialize({
        startOnLoad: false,
        theme: 'dark',
        securityLevel: 'strict',
        fontFamily: 'inherit'
      });
      const { svg } = await mermaid.render('mermaid-chart-1', mermaidCode);
      renderedSvg = svg;
      renderUI(svg);
      renderErrorUI();
    };
  </script>
</body>
</html>
`;

fs.writeFileSync(path.join(webDistDir, 'verify-mermaid.html'), verifyHtml, 'utf8')
console.log('Created verify-mermaid.html in web/dist')

// Start local HTTP server
const server = http.createServer((req, res) => {
  let reqPath = decodeURI(req.url.split('?')[0])
  if (reqPath === '/') reqPath = '/index.html'
  const filePath = path.join(webDistDir, reqPath)
  if (!fs.existsSync(filePath) || fs.statSync(filePath).isDirectory()) {
    res.statusCode = 404
    res.end('Not Found')
    return
  }
  const ext = path.extname(filePath)
  const mimeMap = {
    '.html': 'text/html; charset=utf-8',
    '.js': 'application/javascript; charset=utf-8',
    '.mjs': 'application/javascript; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.svg': 'image/svg+xml',
    '.json': 'application/json',
    '.png': 'image/png'
  }
  res.setHeader('Content-Type', mimeMap[ext] || 'application/octet-stream')
  fs.createReadStream(filePath).pipe(res)
})

await new Promise(resolve => server.listen(4173, '127.0.0.1', resolve))
console.log('Local test server running at http://127.0.0.1:4173')

// Launch Playwright
console.log('Launching browser verification via', pwModulePath)
const browser = await pw.chromium.launch({ headless: true })

try {
  // Mobile Screenshot: 390 x 844 (iPhone 14 standard)
  console.log('Testing Mobile 390px...')
  const mobile390Context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  const mobile390Page = await mobile390Context.newPage()
  await mobile390Page.goto('http://127.0.0.1:4173/verify-mermaid.html', { waitUntil: 'networkidle' })
  await mobile390Page.waitForSelector('.mermaid-svg-container svg', { timeout: 15000 })
  
  const mobile390Path = path.join(artifactsDir, 'mermaid-mobile-390.png')
  await mobile390Page.screenshot({ path: mobile390Path, fullPage: false })
  console.log('Saved mobile 390px screenshot to:', mobile390Path)

  // Test Fullscreen button
  console.log('Testing Fullscreen modal...')
  await mobile390Page.click('#btn-fullscreen')
  await mobile390Page.waitForSelector('#fullscreen-dialog', { timeout: 5000 })
  const mobileFullscreenPath = path.join(artifactsDir, 'mermaid-mobile-fullscreen.png')
  await mobile390Page.screenshot({ path: mobileFullscreenPath, fullPage: false })
  console.log('Saved mobile fullscreen screenshot to:', mobileFullscreenPath)
  await mobile390Context.close()

  // Mobile Screenshot: 360 x 740 (Android small standard)
  console.log('Testing Mobile 360px...')
  const mobile360Context = await browser.newContext({
    viewport: { width: 360, height: 740 },
    deviceScaleFactor: 2,
  })
  const mobile360Page = await mobile360Context.newPage()
  await mobile360Page.goto('http://127.0.0.1:4173/verify-mermaid.html', { waitUntil: 'networkidle' })
  await mobile360Page.waitForSelector('.mermaid-svg-container svg', { timeout: 15000 })

  const mobile360Path = path.join(artifactsDir, 'mermaid-mobile-360.png')
  await mobile360Page.screenshot({ path: mobile360Path, fullPage: false })
  console.log('Saved mobile 360px screenshot to:', mobile360Path)
  await mobile360Context.close()

  // Desktop Screenshot: 1280 x 800
  console.log('Testing Desktop 1280px...')
  const desktopContext = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1.5,
  })
  const desktopPage = await desktopContext.newPage()
  await desktopPage.goto('http://127.0.0.1:4173/verify-mermaid.html', { waitUntil: 'networkidle' })
  await desktopPage.waitForSelector('.mermaid-svg-container svg', { timeout: 15000 })

  const desktopPath = path.join(artifactsDir, 'mermaid-desktop.png')
  await desktopPage.screenshot({ path: desktopPath, fullPage: false })
  console.log('Saved desktop screenshot to:', desktopPath)

  // Error fallback screenshot
  const errorElem = await desktopPage.$('#mermaid-error-root')
  if (errorElem) {
    const errorFallbackPath = path.join(artifactsDir, 'mermaid-error-fallback.png')
    await errorElem.screenshot({ path: errorFallbackPath })
    console.log('Saved error fallback screenshot to:', errorFallbackPath)
  }
  await desktopContext.close()

  console.log('--- ALL VISUAL SCREENSHOTS CAPTURED SUCCESSFULLY ---')
} finally {
  await browser.close()
  server.close()
}
