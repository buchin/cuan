import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const webDistDir = path.resolve(rootDir, 'web/dist')
const artifactsDir = process.env.CUAN_TASK_ARTIFACTS_DIR || '/tmp'

const pwModulePath = process.env.CUAN_BROWSER_MODULE || 'playwright-core'
const pw = require(pwModulePath)

// Find compiled css
const assetsDir = path.resolve(webDistDir, 'assets')
const cssFile = fs.readdirSync(assetsDir).find(f => f.startsWith('style-') && f.endsWith('.css'))
if (!cssFile) {
  throw new Error('Compiled CSS not found in dist/assets')
}

// Create standalone verification page in dist
const verifyHtml = `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
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

    let currentZoom = 1;
    let isFullscreen = false;
    let viewMode = 'diagram';

    function renderUI(svgContent, isError, errorMsg) {
      const container = document.getElementById('mermaid-root');
      container.innerHTML = \`
        <div class="chat-code-mermaid my-3 rounded-xl border border-border/80 bg-muted/20 overflow-hidden shadow-xs">
          <!-- Header -->
          <div class="flex min-h-11 items-center justify-between gap-2 border-b border-border/60 bg-muted/40 px-3.5 py-2 text-xs">
            <div class="flex items-center gap-2 min-w-0">
              <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="6" height="6" rx="1"/><rect x="15" y="3" width="6" height="6" rx="1"/><rect x="9" y="15" width="6" height="6" rx="1"/><path d="M6 9v3a3 3 0 0 0 3 3h6a3 3 0 0 0 3-3V9"/><path d="M12 12v3"/></svg>
              </span>
              <span class="font-semibold text-foreground truncate">Diagram Alur (Mermaid)</span>
              <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground border border-border/40 shrink-0">7 baris</span>
            </div>
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
              <div class="mermaid-scroll-wrapper w-full max-w-full overflow-x-auto p-4 flex justify-center items-center" style="-webkit-overflow-scrolling: touch;">
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
        renderUI(svgContent, isError, errorMsg);
      };
      document.getElementById('btn-copy').onclick = () => {
        navigator.clipboard.writeText(mermaidCode);
        document.getElementById('btn-copy').innerText = 'Tersalin';
        setTimeout(() => { document.getElementById('btn-copy').innerText = 'Salin'; }, 2000);
      };
    }

    function openModal() {
      const portal = document.getElementById('portal-root');
      portal.innerHTML = \`
        <div id="fullscreen-dialog" class="fixed inset-0 z-50 flex flex-col bg-background/95 backdrop-blur-md" role="dialog" aria-modal="true">
          <div class="flex min-h-12 items-center justify-between border-b border-border/60 bg-muted/30 px-4 py-2.5">
            <div class="flex items-center gap-2">
              <span class="font-semibold text-sm">Diagram Alur (Mermaid)</span>
              <span class="text-xs text-muted-foreground">Viewer Interaktif</span>
            </div>
            <div class="flex items-center gap-2">
              <button id="zoom-out" class="min-h-9 px-2.5 rounded bg-muted text-xs">Zoom Out</button>
              <span id="zoom-val" class="text-xs font-mono">100%</span>
              <button id="zoom-in" class="min-h-9 px-2.5 rounded bg-muted text-xs">Zoom In</button>
              <button id="zoom-reset" class="min-h-9 px-2.5 rounded bg-muted text-xs">Reset</button>
              <button id="close-modal" class="min-h-9 px-3 rounded bg-primary/20 text-primary text-xs font-semibold">Tutup</button>
            </div>
          </div>
          <div class="flex-1 overflow-auto p-6 flex items-center justify-center">
            <div id="modal-svg" style="transform: scale(\${currentZoom}); transition: transform 0.15s ease;">
              \${renderedSvg}
            </div>
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
      renderUI(svg, false, null);
    };
  </script>
</body>
</html>
`;

fs.writeFileSync(path.join(webDistDir, 'verify-mermaid.html'), verifyHtml, 'utf8')
console.log('Created verify-mermaid.html in web/dist')

// Launch Playwright
console.log('Launching browser verification via', pwModulePath)
const browser = await pw.chromium.launch({ headless: true })

try {
  // Mobile Screenshot: 390 x 844 (iPhone 14 standard)
  const mobileContext = await browser.newContext({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
  })
  const mobilePage = await mobileContext.newPage()
  await mobilePage.goto('http://127.0.0.1:4173/verify-mermaid.html', { waitUntil: 'networkidle' })
  await mobilePage.waitForSelector('.mermaid-svg-container svg', { timeout: 10000 })
  
  const mobilePath = path.join(artifactsDir, 'mobile.png')
  await mobilePage.screenshot({ path: mobilePath, fullPage: false })
  console.log('Saved mobile screenshot to:', mobilePath)

  // Test Fullscreen button
  await mobilePage.click('#btn-fullscreen')
  await mobilePage.waitForSelector('#fullscreen-dialog', { timeout: 5000 })
  const mobileFullscreenPath = path.join(artifactsDir, 'mobile-fullscreen.png')
  await mobilePage.screenshot({ path: mobileFullscreenPath, fullPage: false })
  console.log('Saved mobile fullscreen screenshot to:', mobileFullscreenPath)
  await mobileContext.close()

  // Desktop Screenshot: 1280 x 800
  const desktopContext = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1.5,
  })
  const desktopPage = await desktopContext.newPage()
  await desktopPage.goto('http://127.0.0.1:4173/verify-mermaid.html', { waitUntil: 'networkidle' })
  await desktopPage.waitForSelector('.mermaid-svg-container svg', { timeout: 10000 })

  const desktopPath = path.join(artifactsDir, 'desktop.png')
  await desktopPage.screenshot({ path: desktopPath, fullPage: false })
  console.log('Saved desktop screenshot to:', desktopPath)
  await desktopContext.close()

  console.log('--- ALL VISUAL SCREENSHOTS CAPTURED SUCCESSFULLY ---')
} finally {
  await browser.close()
}
