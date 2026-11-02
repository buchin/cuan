import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const require = createRequire(import.meta.url)
const playwrightPath = path.join(process.env.HOME || '/home/dev', '.local/lib/node_modules/playwright')
const { chromium } = require(playwrightPath)

const artifactsDir = '/home/dev/.gemini/antigravity-cli/brain/13cac881-bec7-40cd-ad84-386d334fb3f2'
const token = '3f349e777665c43db865008c45888fb4cd85fdfceb24ac655bd1dff02d9a899e'
const targetUrl = 'https://cuan.dojo.cc/inbox/3cf4a1f7-e13a-4ad5-a40b-9c6168459075'

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

  // 1. Desktop
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
    // Also inject localStorage token
    await page.addInitScript((tok) => {
      localStorage.setItem('cuan_token', tok)
      localStorage.setItem('cuan_auth_token', tok)
    }, token)

    await page.goto(targetUrl, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForTimeout(4000)

    const desktopPath = path.join(artifactsDir, 'production-room-desktop.png')
    await page.screenshot({ path: desktopPath, fullPage: false })
    console.log('Saved desktop screenshot to', desktopPath)
    await context.close()
  }

  // 2. Mobile 390px
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

    await page.goto(targetUrl, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await page.waitForTimeout(4000)

    const mobilePath = path.join(artifactsDir, 'production-room-mobile-390px.png')
    await page.screenshot({ path: mobilePath, fullPage: false })
    console.log('Saved mobile screenshot to', mobilePath)
    await context.close()
  }

  await browser.close()
}

run().catch((err) => {
  console.error(err)
  process.exit(1)
})
