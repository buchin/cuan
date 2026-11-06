// Operator smoke check: browse the configured Cuan account without sending chat.
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import { createRequire } from 'node:module'

if (process.env.CUAN_TASK_CAPABILITY_FILE || process.env.CUAN_TASK_CAPABILITY_JSON)
  throw new Error('Owner UI capture is unavailable in bound task sessions')
const [room, outputArgument] = process.argv.slice(2)
if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(room || '') || !outputArgument)
  throw new Error('Usage: node web/scripts/capture-production-room.mjs ROOM_ID NEW_PRIVATE_OUTPUT_DIRECTORY')
let api = process.env.CUAN_API_URL
let token = process.env.CUAN_AUTH_TOKEN || process.env.CUAN_API_TOKEN
for (const file of [path.join(os.homedir(), '.cuan/config.json'), path.join(os.homedir(), '.config/cuan/config.json')]) {
  try {
    const config = JSON.parse(await fs.readFile(file, 'utf8'))
    api ||= config.api_url
    token ||= config.auth_token
  } catch (error) {
    if (error.code !== 'ENOENT') throw new Error('Cuan CLI configuration is unreadable')
  }
}
if (!api || !token) throw new Error('Connect the Cuan CLI account before capturing private UI')
const base = new URL(api)
if (base.username || base.password || (base.protocol !== 'https:' && !(base.protocol === 'http:' && ['localhost', '127.0.0.1'].includes(base.hostname))))
  throw new Error('Use the configured HTTPS Cuan host or a local development host')
const output = path.resolve(outputArgument)
await fs.mkdir(output, { mode: 0o700 })
const require = createRequire(import.meta.url)
const { chromium } = require(process.env.CUAN_BROWSER_MODULE || path.join(os.homedir(), '.local/lib/node_modules/playwright'))
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.CUAN_CHROMIUM_EXECUTABLE,
  args: process.env.CUAN_BROWSER_NO_SANDBOX === '1' ? ['--no-sandbox'] : [],
})
try {
  const errors = []
  for (const width of [390, 1440]) {
    const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 1000 }, hasTouch: width === 390 })
    try {
      await context.addCookies([{ name: 'cuan_session', value: token, url: base.origin, httpOnly: true, secure: base.protocol === 'https:', sameSite: 'Lax' }])
      await context.addInitScript(({ origin, token }) => {
        if (location.origin === origin) localStorage.setItem('cuan_token', token)
      }, { origin: base.origin, token })
      const page = await context.newPage()
      page.on('pageerror', error => errors.push(error.message))
      // Inbox polls and streams live work; network idleness is not readiness.
      await page.goto(new URL(`/inbox/${room}`, base.origin).href, { waitUntil: 'domcontentloaded', timeout: 30000 })
      await page.locator('.inbox-message-list [data-message-id]').first().waitFor({ state: 'attached', timeout: 15000 })
      await page.locator('.inbox-message-list').waitFor({ state: 'visible' })
      await page.locator('textarea').first().waitFor({ state: 'visible', timeout: 15000 })
      await page.evaluate(() => document.fonts.ready.then(() => true))
      await page.screenshot({ path: path.join(output, `inbox-${width}.png`) })
    } finally { await context.close() }
  }
  await fs.writeFile(path.join(output, 'smoke.json'), JSON.stringify({ room, widths: [390, 1440], pageErrors: errors }), { mode: 0o600, flag: 'wx' })
  if (errors.length) throw new Error('Production UI reported browser errors; inspect private smoke.json')
  console.log('PASS: production Inbox loaded at 390px and 1440px with no browser page errors')
} finally { await browser.close() }
