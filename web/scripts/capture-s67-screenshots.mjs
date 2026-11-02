import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const require = createRequire(import.meta.url)
const playwrightPath = path.join(process.env.HOME || '/home/dev', '.local/lib/node_modules/playwright')
const { chromium } = require(playwrightPath)

const evidenceDir = path.join(__dirname, '../../docs/sprints/evidence/s67')
fs.mkdirSync(evidenceDir, { recursive: true })

let token = '3f349e777665c43db865008c45888fb4cd85fdfceb24ac655bd1dff02d9a899e'
try {
  const cuanCfg = JSON.parse(fs.readFileSync(path.join(process.env.HOME || '/home/dev', '.cuan/config.json'), 'utf8'))
  if (cuanCfg.token) token = cuanCfg.token
} catch (e) {}

const customerUrl = 'https://cuan.dojo.cc/inbox/5ea132cf-b001-4b48-ad16-8653b3f5f051'
const cuanAgentUrl = 'https://cuan.dojo.cc/inbox/88bd8608-a87d-4729-91d4-1f6c6997e375'

async function run() {
  const executablePath = path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium')
  const browser = await chromium.launch({
    headless: true,
    executablePath,
    env: {
      ...process.env,
      LD_LIBRARY_PATH:
        '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' +
        (process.env.LD_LIBRARY_PATH || ''),
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--disable-gpu'],
  })

  // 1. Desktop - Customer Room (5ea132cf)
  {
    const context = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      deviceScaleFactor: 2,
    })
    await context.addCookies([
      {
        name: 'cuan_session',
        value: token,
        domain: 'cuan.dojo.cc',
        path: '/',
        httpOnly: false,
        secure: true,
        sameSite: 'Lax',
      },
    ])
    const page = await context.newPage()
    await page.addInitScript((tok) => {
      localStorage.setItem('cuan_token', tok)
      localStorage.setItem('cuan_auth_token', tok)
    }, token)

    await page.goto(customerUrl, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForTimeout(4000)

    const p = path.join(evidenceDir, 's67-inbox-customer-desktop.png')
    await page.screenshot({ path: p, fullPage: false })
    console.log('Saved', p)
    await context.close()
  }

  // 2. Mobile 390px - Customer Room (5ea132cf)
  {
    const context = await browser.newContext({
      viewport: { width: 390, height: 844 },
      deviceScaleFactor: 2,
      isMobile: true,
      hasTouch: true,
    })
    await context.addCookies([
      {
        name: 'cuan_session',
        value: token,
        domain: 'cuan.dojo.cc',
        path: '/',
        httpOnly: false,
        secure: true,
        sameSite: 'Lax',
      },
    ])
    const page = await context.newPage()
    await page.addInitScript((tok) => {
      localStorage.setItem('cuan_token', tok)
      localStorage.setItem('cuan_auth_token', tok)
    }, token)

    await page.goto(customerUrl, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForTimeout(3000)
    // If mobile is still on contacts view, click the customer conversation
    const customerCard = page.locator('text=Visitor #5ea132').first()
    if (await customerCard.isVisible({ timeout: 5000 }).catch(() => false)) {
      await customerCard.click()
    }
    await page.waitForTimeout(3000)

    const p = path.join(evidenceDir, 's67-inbox-customer-mobile.png')
    await page.screenshot({ path: p, fullPage: false })
    console.log('Saved', p)
    await context.close()
  }

  // 3. Desktop - @cuan Agent Room
  {
    const context = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      deviceScaleFactor: 2,
    })
    await context.addCookies([
      {
        name: 'cuan_session',
        value: token,
        domain: 'cuan.dojo.cc',
        path: '/',
        httpOnly: false,
        secure: true,
        sameSite: 'Lax',
      },
    ])
    const page = await context.newPage()
    await page.addInitScript((tok) => {
      localStorage.setItem('cuan_token', tok)
      localStorage.setItem('cuan_auth_token', tok)
    }, token)

    await page.goto(cuanAgentUrl, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForTimeout(4000)

    const p = path.join(evidenceDir, 's67-inbox-cuan-desktop.png')
    await page.screenshot({ path: p, fullPage: false })
    console.log('Saved', p)
    await context.close()
  }

  // 4. Mobile 390px - @cuan Agent Room
  {
    const context = await browser.newContext({
      viewport: { width: 390, height: 844 },
      deviceScaleFactor: 2,
      isMobile: true,
      hasTouch: true,
    })
    await context.addCookies([
      {
        name: 'cuan_session',
        value: token,
        domain: 'cuan.dojo.cc',
        path: '/',
        httpOnly: false,
        secure: true,
        sameSite: 'Lax',
      },
    ])
    const page = await context.newPage()
    await page.addInitScript((tok) => {
      localStorage.setItem('cuan_token', tok)
      localStorage.setItem('cuan_auth_token', tok)
    }, token)

    await page.goto(cuanAgentUrl, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForTimeout(4000)

    const p = path.join(evidenceDir, 's67-inbox-cuan-mobile.png')
    await page.screenshot({ path: p, fullPage: false })
    console.log('Saved', p)
    await context.close()
  }

  await browser.close()
  console.log('All screenshots captured successfully!')
}

run().catch((err) => {
  console.error(err)
  process.exit(1)
})
