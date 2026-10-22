const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const http = require('http'),
  fs = require('fs'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/contacts-crm'
let browser, server
;(async () => {
  const calls = [],
    errors = []
  let fail = false
  server = http.createServer((req, res) => {
    if (req.url === '/chat/widget.js') {
      res.setHeader('content-type', 'text/javascript')
      return res.end(fs.readFileSync('web/dist/chat/widget.js'))
    }
    res.setHeader('content-type', 'text/html')
    res.end(
      '<!doctype html><html><body><h1>Widget integration fixture</h1><script>window.$crisp=[["set","user:email",["queued@example.invalid"]]]</script><script src="/chat/widget.js" data-product="Dojo" data-name="Fixture Customer"></script></body></html>',
    )
  })
  await new Promise((r) => server.listen(4182, '127.0.0.1', r))
  browser = await chromium.launch({
    headless: true,
    executablePath:
      '/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
    env: {
      ...process.env,
      LD_LIBRARY_PATH: [
        'usr/lib',
        'usr/lib/pulseaudio',
        'lib',
        'usr/lib/chromium',
      ]
        .map((p) => '/home/dev/.local/playwright-alpine/' + p)
        .join(':'),
    },
  })
  const page = await browser.newPage({ viewport: { width: 390, height: 844 } })
  page.on('pageerror', (e) => errors.push(e.message))
  page.setDefaultTimeout(5000)
  await page.route('**/api/chat/**', async (route) => {
    const req = route.request(),
      p = new URL(req.url()).pathname
    const body = req.method() === 'POST' ? req.postDataJSON() : null
    calls.push({ path: p, method: req.method(), body })
    const json = (data, status = 200) =>
      route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(data),
      })
    if (p === '/api/chat/session')
      return json({ session_uuid: 'session-fixture', messages: [] })
    if (p.endsWith('/page')) return json({ ok: true })
    if (req.method() === 'POST')
      return json(
        fail ? { error: 'unavailable' } : { ok: true },
        fail ? 503 : 200,
      )
    return json({ messages: [] })
  })
  await page.goto('http://127.0.0.1:4182')
  await page.waitForFunction(() => !!window.$dojoChat)
  assert.equal(calls.length, 0, 'passive page view created a contact')
  await page
    .getByRole('button', { name: 'Buka Live Chat', exact: true })
    .click()
  await page.waitForFunction(
    () =>
      localStorage.getItem(
        'dojo_chat_session_v2:http://127.0.0.1:4182:Dojo',
      ) === 'session-fixture',
  )
  const init = calls.find((c) => c.path === '/api/chat/session')
  assert.equal(init.body.email, 'queued@example.invalid')
  assert.equal(init.body.name, 'Fixture Customer')
  assert(init.body.guest_uuid)
  await page.evaluate(() =>
    window.$dojoChat.identify({
      email: 'updated@example.invalid',
      name: 'Updated',
    }),
  )
  assert.equal(
    calls.filter((c) => c.path === '/api/chat/session').at(-1).body.email,
    'updated@example.invalid',
  )
  await page.evaluate(() =>
    window.$crisp.push(['set', 'user:email', ['crisp@example.invalid']]),
  )
  await page.waitForTimeout(100)
  assert.equal(
    calls.filter((c) => c.path === '/api/chat/session').at(-1).body.email,
    'crisp@example.invalid',
  )
  const input = page.getByRole('textbox', { name: 'Pesan' })
  await input.fill('Message survives failure')
  fail = true
  await page.getByRole('button', { name: 'Kirim Pesan' }).click()
  await page.getByRole('alert').waitFor()
  assert.equal(await input.inputValue(), 'Message survives failure')
  fail = false
  await page.getByRole('button', { name: 'Kirim Pesan' }).click()
  await page.waitForFunction(
    () =>
      document
        .querySelector('#dojo-livechat-host')
        .shadowRoot.querySelector('textarea').value === '',
  )
  for (const width of [1440, 820, 390, 360]) {
    await page.setViewportSize({ width, height: 844 })
    const m = await page.evaluate(() => {
      const root = document.querySelector('#dojo-livechat-host').shadowRoot
      const w = root.querySelector('#window').getBoundingClientRect()
      const b = root.querySelector('#send-btn').getBoundingClientRect()
      return {
        left: w.left,
        right: w.right,
        width: innerWidth,
        send: b.width,
        font: getComputedStyle(root.querySelector('textarea')).fontSize,
      }
    })
    assert(m.left >= 0 && m.right <= m.width)
    assert.equal(m.send, 48)
    assert.equal(m.font, '16px')
    await page.screenshot({ path: out + '/widget-' + width + '.png' })
  }
  await input.focus()
  await page.keyboard.press('Escape')
  assert.equal(
    await page
      .getByRole('button', { name: 'Buka Live Chat' })
      .getAttribute('aria-expanded'),
    'false',
  )
  assert.deepEqual(errors, [])
  fs.writeFileSync(
    out + '/widget.json',
    JSON.stringify(
      {
        fixture: true,
        checks: [
          'no contact from passive view',
          'queued Crisp identity',
          'session response parsed',
          'identify API',
          'Crisp email update',
          'failed send keeps input and shows error',
          'retry sends',
          'desktop/tablet/mobile bounds',
          '16px input and 48px send',
          'Escape closes',
        ],
        providerInvocations: 0,
        errors,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(
    'PASS widget identity, initialization, retry, accessibility and responsive sizing',
  )
})()
  .catch((e) => {
    console.error(e)
    process.exitCode = 1
  })
  .finally(async () => {
    if (browser) await browser.close()
    if (server) await new Promise((r) => server.close(r))
  })
