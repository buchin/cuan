// Visual verification of InboxActivity dynamic rotating status text
import fsSync from 'node:fs'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const playwrightPath = process.env.CUAN_BROWSER_MODULE ||
  (fsSync.existsSync(path.join(os.homedir(), '.local/node_modules/playwright'))
    ? path.join(os.homedir(), '.local/node_modules/playwright')
    : path.join(os.homedir(), '.local/lib/node_modules/playwright'))
const { chromium } = require(playwrightPath)

const output = process.argv[2] || process.env.CUAN_TASK_ARTIFACTS_DIR
if (!output) throw new Error('Supply an output directory')
await fs.mkdir(output, { recursive: true, mode: 0o700 })

const webRoot = path.resolve('web')
process.chdir(webRoot)

const server = await createServer({
  root: webRoot,
  server: { host: '127.0.0.1', port: 0 },
  plugins: [{
    name: 'inbox-activity-html',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (req.url === '/' || req.url.startsWith('/activity-fixture')) {
          res.setHeader('Content-Type', 'text/html')
          res.end(await server.transformIndexHtml('/', '<!DOCTYPE html><html class="dark"><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body class="bg-background text-foreground"><div id="root"></div><script type="module" src="/tests/inbox-activity-fixture.tsx"></script></body></html>'))
        } else next()
      })
    }
  }]
})

await server.listen()
const origin = server.resolvedUrls.local[0]
let browser

try {
  browser = await chromium.launch({
    headless: true,
    executablePath: process.env.CUAN_CHROMIUM_EXECUTABLE,
    args: process.env.CUAN_BROWSER_NO_SANDBOX === '1' ? ['--no-sandbox'] : [],
  })

  // Test Mobile (390px) and Desktop (1440px)
  for (const width of [390, 1440]) {
    const isMobile = width === 390
    const context = await browser.newContext({
      viewport: { width, height: isMobile ? 844 : 900 },
      hasTouch: isMobile,
    })

    // 1. Dark Mode (Matching user's screenshot environment)
    const page = await context.newPage()
    await page.goto(new URL('/activity-fixture', origin).href, { waitUntil: 'networkidle' })
    await page.evaluate(() => document.documentElement.classList.add('dark'))

    const activityTextEl = page.locator('[data-testid="inbox-activity-text"]')
    await activityTextEl.waitFor({ state: 'visible', timeout: 5000 })

    const typingDotsEl = page.locator('[data-testid="inbox-activity-typing-dots"]')
    await typingDotsEl.waitFor({ state: 'visible', timeout: 5000 })
    assert.ok(await typingDotsEl.isVisible(), 'Typing dots indicator must be visible')

    // Initial phrase must be Exploring workspace…
    const initialText = await activityTextEl.innerText()
    console.log(`[${width}px Dark] Initial text: "${initialText}"`)
    assert.equal(initialText, 'Exploring workspace…', 'Initial activity text must be Exploring workspace…')

    const boxBefore = await activityTextEl.boundingBox()

    // Take screenshot of Initial State (Exploring workspace…)
    const initialCaptureName = `inbox-activity-exploring-workspace-${width}-dark.png`
    await page.screenshot({ path: path.join(output, initialCaptureName) })
    await page.screenshot({ path: path.join(output, `inbox-activity-thinking-${width}-dark.png`) })
    console.log(`Saved: ${initialCaptureName}`)

    // Wait 3.8s for rotation to trigger Analyzing tasks…
    console.log(`Waiting for rotation at ${width}px...`)
    await page.waitForTimeout(3800)

    const rotatedText = await activityTextEl.innerText()
    console.log(`[${width}px Dark] Rotated text: "${rotatedText}"`)
    assert.equal(rotatedText, 'Analyzing tasks…', 'After rotation, activity text must be Analyzing tasks…')

    const boxAfter = await activityTextEl.boundingBox()
    // Verify zero layout shift: y position must be identical
    assert.ok(
      Math.abs(boxBefore.y - boxAfter.y) < 1.0,
      `Zero vertical layout shift: before y=${boxBefore.y}, after y=${boxAfter.y}`,
    )

    // Take screenshot of Rotated State (Analyzing tasks…)
    const rotatedCaptureName = `inbox-activity-analyzing-tasks-${width}-dark.png`
    await page.screenshot({ path: path.join(output, rotatedCaptureName) })
    await page.screenshot({ path: path.join(output, `inbox-activity-analyzing-${width}-dark.png`) })
    console.log(`Saved: ${rotatedCaptureName}`)

    // 2. Light Mode screenshot for contrast and low visual noise
    await page.evaluate(() => document.documentElement.classList.remove('dark'))
    await page.waitForTimeout(100)
    const lightCaptureName = `inbox-activity-${width}-light.png`
    await page.screenshot({ path: path.join(output, lightCaptureName) })
    console.log(`Saved: ${lightCaptureName}`)

    // 3. Trigger Real Tool Step
    const toggleBtn = page.locator('#btn-toggle-step')
    await toggleBtn.click()
    await page.waitForTimeout(150)

    const stepText = await activityTextEl.innerText()
    console.log(`[${width}px] Tool Step text: "${stepText}"`)
    assert.ok(
      stepText.includes('Membaca berkas konfigurasi proyek'),
      `Must display actual tool activity text, got "${stepText}"`,
    )

    const stepCaptureName = `inbox-activity-step-${width}.png`
    await page.screenshot({ path: path.join(output, stepCaptureName) })
    console.log(`Saved: ${stepCaptureName}`)

    await context.close()
  }

  console.log('Visual verification completed successfully!')
} finally {
  await browser?.close()
  await server.close()
}
