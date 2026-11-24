import fs from 'node:fs/promises'
import fsSync from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import { createRequire } from 'node:module'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const playwrightPath =
  process.env.CUAN_BROWSER_MODULE ||
  ['/home/ubuntu/.local/node_modules/playwright', path.join(os.homedir(), '.local/node_modules/playwright')].find((p) => fsSync.existsSync(p)) ||
  'playwright'
const { chromium } = require(playwrightPath)

const outputDir = path.resolve(process.argv[2] || '/tmp/add-brain-ui')
await fs.mkdir(outputDir, { recursive: true })

const entry = `
import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { LanguageProvider } from '/src/context/LanguageContext.tsx';
import { AddBrainModal } from '/src/features/agents/AddBrainModal.tsx';
import '/src/index.css';

function Fixture() {
  const [open, setOpen] = useState(true);
  return React.createElement(
    'div',
    { className: 'min-h-screen bg-muted/20 p-8 flex flex-col items-center justify-center' },
    React.createElement(
      'button',
      {
        onClick: () => setOpen(true),
        className: 'action-primary px-4 py-2 rounded-xl text-sm font-medium mb-4'
      },
      'Buka Modal'
    ),
    React.createElement(AddBrainModal, {
      open: open,
      onClose: () => setOpen(false),
      onBrainAdded: () => {},
      existingProviders: []
    })
  );
}

createRoot(document.getElementById('root')).render(
  React.createElement(LanguageProvider, null, React.createElement(Fixture))
);
`


const webRoot = fsSync.existsSync(path.resolve('web')) ? path.resolve('web') : path.resolve('.')
const distAssets = path.join(webRoot, 'dist/assets')
const cssFiles = (await fs.readdir(distAssets)).filter((f) => f.endsWith('.css'))
const mainCss = cssFiles.find((f) => f.startsWith('style-')) || cssFiles[0]
const compiledCss = await fs.readFile(path.join(distAssets, mainCss), 'utf8')

const server = await createServer({
  root: webRoot,
  server: { host: '127.0.0.1', port: 0 },
  plugins: [
    {
      name: 'add-brain-fixture',
      enforce: 'pre',
      resolveId(id) {
        if (id === '/__add_brain_entry.jsx') return '\0add-brain-fixture-entry.jsx'
      },
      load(id) {
        if (id === '\0add-brain-fixture-entry.jsx') return entry
      },
      configureServer(server) {
        server.middlewares.use(async (req, res, next) => {
          if (req.url.startsWith('/api/v1/ai/providers/models')) {
            res.setHeader('Content-Type', 'application/json')
            res.end(
              JSON.stringify({
                provider: 'claude',
                models: [
                  { id: 'claude-sonnet-5-5', name: 'Claude Sonnet 5.5', input_token_limit: 200000 },
                  { id: 'claude-opus-5-5', name: 'Claude Opus 5.5', input_token_limit: 200000 },
                  { id: 'claude-fable-5-2', name: 'Claude Fable 5.2', input_token_limit: 200000 },
                  { id: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6', input_token_limit: 200000 },
                ],
              })
            )
            return
          }
          if (req.url === '/' || req.url.startsWith('/?')) {
            res.setHeader('Content-Type', 'text/html')
            res.end(
              await server.transformIndexHtml(
                req.url,
                `<!DOCTYPE html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><style>${compiledCss}</style></head><body class="bg-background text-foreground"><div id="root"></div><script type="module" src="/__add_brain_entry.jsx"></script></body></html>`
              )
            )
            return
          }
          next()
        })
      },
    },
  ],
})

await server.listen()
const origin = server.resolvedUrls.local[0]
let browser

try {
  browser = await chromium.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox'],
  })

  // 1. Desktop 1280px - Subscription Mode under Claude Code
  const contextDesktop = await browser.newContext({
    viewport: { width: 1280, height: 860 },
  })
  await contextDesktop.addInitScript(() => {
    localStorage.setItem('cuan-language', 'id')
    localStorage.setItem('cuan-theme', 'light')
  })
  const pageDesktop = await contextDesktop.newPage()
  await pageDesktop.goto(origin)
  await pageDesktop.waitForSelector('[data-testid="add-brain-content"]', { timeout: 10000 })
  await pageDesktop.waitForTimeout(600)

  await pageDesktop.screenshot({
    path: path.join(outputDir, 'desktop-claude-subscription.png'),
    fullPage: false,
  })
  console.log('Saved desktop-claude-subscription.png')

  // Switch to BYOK mode under Claude Code
  const byokTabBtn = pageDesktop.locator('button:has-text("Kunci API")')
  await byokTabBtn.click()
  await pageDesktop.waitForTimeout(400)
  await pageDesktop.screenshot({
    path: path.join(outputDir, 'desktop-claude-byok.png'),
    fullPage: false,
  })
  console.log('Saved desktop-claude-byok.png')

  // Click on OpenCode engine card
  const openCodeBtn = pageDesktop.locator('button:has-text("OpenCode")')
  await openCodeBtn.click()
  await pageDesktop.waitForTimeout(400)
  await pageDesktop.screenshot({
    path: path.join(outputDir, 'desktop-opencode-byok.png'),
    fullPage: false,
  })
  console.log('Saved desktop-opencode-byok.png')

  await contextDesktop.close()

  // 2. Mobile 390px
  const contextMobile = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
  })
  await contextMobile.addInitScript(() => {
    localStorage.setItem('cuan-language', 'id')
    localStorage.setItem('cuan-theme', 'light')
  })
  const pageMobile = await contextMobile.newPage()
  await pageMobile.goto(origin)
  await pageMobile.waitForSelector('[data-testid="add-brain-content"]', { timeout: 10000 })
  await pageMobile.waitForTimeout(600)

  await pageMobile.screenshot({
    path: path.join(outputDir, 'mobile-claude-subscription.png'),
    fullPage: false,
  })
  console.log('Saved mobile-claude-subscription.png')

  await contextMobile.close()

  // 3. Desktop Dark Mode - Subscription Mode
  const contextDark = await browser.newContext({
    viewport: { width: 1280, height: 860 },
  })
  await contextDark.addInitScript(() => {
    localStorage.setItem('cuan-language', 'id')
    localStorage.setItem('cuan-theme', 'dark')
  })
  const pageDark = await contextDark.newPage()
  await pageDark.goto(origin)
  await pageDark.waitForSelector('[data-testid="add-brain-content"]', { timeout: 10000 })
  await pageDark.evaluate(() => {
    document.documentElement.classList.add('dark')
    document.body.classList.add('dark')
  })
  await pageDark.waitForTimeout(600)

  await pageDark.screenshot({
    path: path.join(outputDir, 'desktop-subscription-dark.png'),
    fullPage: false,
  })
  console.log('Saved desktop-subscription-dark.png')

  await contextDark.close()

  console.log('All verification screenshots captured successfully.')
} finally {
  if (browser) await browser.close()
  await server.close()
}
