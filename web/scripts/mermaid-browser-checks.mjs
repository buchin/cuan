import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'
export const flowSource =
  'flowchart TD\n  A[Ide] --> B[Diskusi]\n  B --> C[Kerjakan]\n  C --> D[Hasil terverifikasi]'
export async function runMermaidChecks(base, output) {
  await fs.mkdir(output, { recursive: true })
  const require = createRequire(import.meta.url)
  const { chromium } = require(
    process.env.CUAN_PLAYWRIGHT_MODULE ||
      process.env.CUAN_BROWSER_MODULE ||
      path.join(os.homedir(), '.local/lib/node_modules/playwright'),
  )
  const browser = process.env.CUAN_REMOTE_BROWSER_URL
    ? await chromium.connectOverCDP(process.env.CUAN_REMOTE_BROWSER_URL)
    : await chromium.launch({
        headless: true,
        executablePath: process.env.CUAN_CHROMIUM_EXECUTABLE,
        args: process.env.CUAN_BROWSER_NO_SANDBOX === '1' ? ['--no-sandbox'] : [],
      })
  const source = flowSource
  try {
    const errors = [],
      external = []
    for (const width of [390, 1440]) {
      const context = await browser.newContext({
        viewport: { width, height: width === 390 ? 844 : 1000 },
        hasTouch: width === 390,
      })
      await context.grantPermissions(['clipboard-read', 'clipboard-write'])
      await context.addInitScript(() =>
        localStorage.setItem('cuan-language', 'id'),
      )
      const page = await context.newPage()
      page.on('pageerror', (error) => errors.push(error.message))
      page.on('request', (request) => {
        if (
          /^https?:/.test(request.url()) &&
          !request.url().startsWith('http://127.0.0.1:')
        )
          external.push(request.url())
      })
      const requests = []
      page.on('request', (request) => requests.push(request.url()))
      await page.goto(base + '?kind=none')
      await page.getByText('Obrolan tanpa diagram.').waitFor()
      assert(
        !requests.some((url) =>
          /assets\/mermaid-|\.vite\/deps\/mermaid/.test(url),
        ),
        'Plain messages must not download Mermaid',
      )
      for (const theme of ['light', 'dark']) {
        await page.emulateMedia({ colorScheme: theme })
        await page.goto(base)
        const image = page.locator('[data-mermaid-diagram] img').first()
        await image.waitFor()
        await page.waitForFunction(() => {
          const image = document.querySelector('[data-mermaid-diagram] img')
          return image?.complete && image.naturalWidth > 0
        })
        assert(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
          'Diagram overflowed the page',
        )
        const svg = decodeURIComponent(
          (await image.getAttribute('src')).split(',').slice(1).join(','),
        )
        const labels = await page.evaluate((text) => {
          const svg = new DOMParser().parseFromString(text, 'image/svg+xml')
          return [...svg.querySelectorAll('text, foreignObject')]
            .map((node) => node.textContent)
            .join(' ')
            .replace(/\s+/g, '')
        }, svg)
        assert(
          svg.includes('<svg') && labels.includes('Hasilterverifikasi'),
          'SVG lost its readable labels',
        )
        await page.screenshot({
          path: path.join(output, `mermaid-${width}-${theme}.png`),
          fullPage: true,
        })
        await page
          .getByRole('button', { name: 'Lihat kode diagram', exact: true })
          .click()
        assert.equal(
          await page.getByLabel('Kode diagram', { exact: true }).innerText(),
          source,
        )
        await page
          .getByRole('button', { name: 'Lihat kode diagram', exact: true })
          .click()
        await page
          .getByRole('button', { name: 'Salin kode diagram', exact: true })
          .click()
        assert.equal(
          await page.evaluate(() => navigator.clipboard.readText()),
          source,
        )
        const expand = page.getByRole('button', {
          name: 'Perbesar diagram',
          exact: true,
        })
        await expand.click()
        const dialog = page.getByRole('dialog')
        await dialog.waitFor()
        const box = await dialog.boundingBox()
        assert(
          box && box.x >= 0 && box.width <= width + 1,
          'Viewer controls left the viewport',
        )
        for (let step = 0; step < 6; step++)
          await dialog
            .getByRole('button', { name: 'Perbesar', exact: true })
            .click()
        const pan = dialog.getByLabel('Geser diagram', { exact: true })
        assert(
          await pan.evaluate(
            (node) =>
              node.scrollWidth > node.clientWidth ||
              node.scrollHeight > node.clientHeight,
          ),
          'Zoom did not expand the diagram',
        )
        await pan.evaluate((node) => {
          node.scrollLeft = 120
          node.scrollTop = 120
        })
        await page.waitForTimeout(250)
        await page.screenshot({
          path: path.join(output, `mermaid-viewer-${width}-${theme}.png`),
        })
        assert(
          await pan.evaluate(
            (node) => node.scrollLeft > 0 || node.scrollTop > 0,
          ),
          'Viewer cannot pan',
        )
        const beforeDrag = await pan.evaluate((node) => node.scrollTop)
        const area = await pan.boundingBox()
        await page.mouse.move(area.x + 100, area.y + 150)
        await page.mouse.down()
        await page.mouse.move(area.x + 100, area.y + 90)
        await page.mouse.up()
        assert(
          (await pan.evaluate((node) => node.scrollTop)) > beforeDrag,
          'Mouse drag did not pan the diagram',
        )
        await page.screenshot({
          path: path.join(output, `mermaid-viewer-${width}-${theme}.png`),
        })
        await page.keyboard.press('Escape')
        await dialog.waitFor({ state: 'detached' })
        await page.waitForFunction(
          () =>
            document.activeElement?.getAttribute('aria-label') ===
            'Perbesar diagram',
        )
      }
      await page.goto(base)
      await page.locator('[data-mermaid-diagram] img').waitFor()
      await page.evaluate(() => {
        navigator.clipboard.writeText = async () => {
          throw new Error('Fixture clipboard denied')
        }
      })
      await page
        .getByRole('button', { name: 'Salin kode diagram', exact: true })
        .click()
      await page
        .getByText('Tidak dapat menyalin. Pilih dan salin kode diagram.', {
          exact: true,
        })
        .waitFor()
      await page.getByLabel('Kode diagram', { exact: true }).waitFor()
      await page.goto(base + '?kind=sequence&preview=1')
      await page.locator('[data-mermaid-diagram] img').waitFor()
      for (const kind of ['invalid', 'oversized']) {
        await page.goto(base + '?kind=' + kind)
        await page
          .getByText('Diagram tidak dapat ditampilkan.', { exact: true })
          .waitFor()
        await page.getByLabel('Kode diagram', { exact: true }).waitFor()
        await page
          .getByText('Hasilnya dilaporkan di obrolan ini.', { exact: true })
          .waitFor()
        assert(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
        )
        if (kind === 'invalid')
          await page.screenshot({
            path: path.join(output, `mermaid-invalid-${width}.png`),
            fullPage: true,
          })
      }
      for (const attack of [
        'attack',
        'remoteimage',
        'escapedimage',
        'remotestyle',
      ]) {
        await page.goto(base + '?kind=' + attack)
        await page
          .locator('[data-mermaid-diagram] img, [data-mermaid-diagram] pre')
          .first()
          .waitFor()
        assert.equal(
          await page.evaluate(() => window.__diagramInjected),
          undefined,
        )
      }
      await context.close()
    }
    assert.deepEqual(external, [], 'Diagram rendered external resources')
    assert.deepEqual(errors, [], 'Uncaught browser errors')
    console.log(
      'PASS: lazy loading, SVG, themes, 390px/desktop, source/copy, zoom/pan, Escape/focus, preview, invalid/oversized and untrusted content.',
    )
  } finally {
    await browser.close()
  }
}
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  if (!process.argv[2] || !process.argv[3])
    throw new Error('Provide fixture URL and screenshot directory')
  await runMermaidChecks(process.argv[2], process.argv[3])
}
