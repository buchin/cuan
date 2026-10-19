// Exact production build with isolated fixture API. No provider or product writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/inbox-list',
  base = 'http://127.0.0.1:4181',
  now = new Date().toISOString()
let browser,
  server,
  fail = false,
  empty = false
const errors = [],
  writes = [],
  unknown = []
const agents = [
  {
    id: 'agent_one',
    name: '@pin2',
    enabled: true,
    connection_state: 'unknown',
  },
  {
    id: 'agent_two',
    name: '@helper',
    enabled: true,
    connection_state: 'verified',
    verified_at: now,
  },
  {
    id: 'agent_login',
    name: '@login',
    enabled: true,
    connection_state: 'login_required',
  },
]
const contacts = [
  {
    id: 'contact_agent_one',
    conversation_id: 'room0',
    type: 'agent',
    name: '@pin2',
    last_message:
      '[@pin2] Siap! **Ringkas** dengan `kode`.\n```cuan-task\n{"title":"PRIVATE_METADATA"}\n```',
    last_message_at: now,
    unread_count: 3,
  },
  {
    id: 'contact_agent_two',
    conversation_id: 'room1',
    type: 'agent',
    name: '@helper',
    last_message: '[@helper] Cek [panduan](https://example.invalid).',
    last_message_at: now,
    unread_count: 0,
  },
  {
    id: 'contact_agent_login',
    conversation_id: 'room2',
    type: 'agent',
    name: '@login',
    last_message: 'Silakan lanjutkan.',
    last_message_at: now,
    unread_count: 0,
  },
  {
    id: 'customer1',
    conversation_id: 'room3',
    type: 'customer',
    name: 'Pelanggan baru',
    product: 'Dojo',
    last_message: 'Tanya paket langganan',
    last_message_at: now,
    unread_count: 2,
  },
].map((c) => ({ ...c, status: 'idle', updated_at: now }))
;(async () => {
  server = http.createServer((req, res) => {
    let file = path.join(process.cwd(), 'web/dist', req.url.split('?')[0])
    if (!path.extname(file))
      file = path.join(process.cwd(), 'web/dist/index.html')
    try {
      const data = fs.readFileSync(file)
      res.setHeader(
        'Content-Type',
        file.endsWith('.js')
          ? 'text/javascript'
          : file.endsWith('.css')
            ? 'text/css'
            : file.endsWith('.woff2')
              ? 'font/woff2'
              : 'text/html',
      )
      res.end(data)
    } catch {
      res.writeHead(404)
      res.end()
    }
  })
  await new Promise((r) => server.listen(4181, '127.0.0.1', r))
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
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  page.setDefaultTimeout(6000)
  page.on('pageerror', (e) => errors.push(e.message))
  await page.addInitScript(() =>
    localStorage.setItem('cuan_token', 'fixture-only'),
  )

  await page.route('**/api/**', async (route) => {
    const req = route.request(),
      p = new URL(req.url()).pathname
    const json = (data, status = 200) =>
      route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(data),
      })
    if (req.method() !== 'GET') {
      writes.push(p)
      return json({ error: 'Unexpected write' }, 400)
    }
    if (p.endsWith('/auth/status')) return json({ setup_required: false })
    if (p.endsWith('/auth/me'))
      return json({ owner: { id: 'fixture', username: 'Fixture' } })
    if (p === '/api/v1/agents') return json({ agents })
    if (p === '/api/v1/tasks') return json({ tasks: [] })
    if (p === '/api/v1/projects') return json({ projects: [] })
    if (p === '/api/v1/chat/contacts')
      return fail
        ? json({ error: 'Fixture unavailable' }, 503)
        : json({ contacts: empty ? [] : contacts })
    const match = p.match(
      /chat\/conversations\/(room[0-3])\/(messages|events)$/,
    )
    if (match) {
      const data = {
        conversation_id: match[1],
        contact: contacts.find((c) => c.conversation_id === match[1]),
        messages: [
          {
            id: 1,
            uuid: 'm1',
            author_type: 'user',
            content: 'Halo',
            created_at: now,
          },
        ],
        tasks: [],
        requests: [],
      }
      return match[2] === 'messages'
        ? json(data)
        : route.fulfill({
            status: 200,
            contentType: 'text/event-stream',
            body: 'event: snapshot\ndata: ' + JSON.stringify(data) + '\n\n',
          })
    }
    unknown.push(p)
    return json({ error: 'Not found' }, 404)
  })
  const measurements = []
  for (const width of [1440, 820, 390, 360])
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width, height: 900 })
      await page.goto(base + '/settings/appearance')
      await page
        .getByRole('button', {
          name: theme === 'light' ? 'Light' : 'Dark',
          exact: true,
        })
        .click()
      await page.goto(base + '/inbox')
      const list = page.locator('.inbox-list:visible')
      await list.locator('.inbox-contact-row').first().waitFor()
      assert.equal(await list.count(), 1)
      assert.equal(
        await list.getByText('Belum diperiksa', { exact: true }).count(),
        0,
      )
      assert.equal(
        await list.getByText('Terhubung', { exact: true }).count(),
        0,
      )
      assert.equal(
        await list.getByText('Perlu login', { exact: true }).count(),
        1,
      )
      assert.equal(
        await list
          .getByText('Siap! Ringkas dengan kode. Draft task', { exact: true })
          .count(),
        1,
      )
      assert.equal(
        await list.getByText('Cek panduan.', { exact: true }).count(),
        1,
      )
      assert(
        !(await list
          .textContent()
          .then(
            (s) => s.includes('PRIVATE_METADATA') || s.includes('[@pin2]'),
          )),
      )
      const row = list.locator('.inbox-contact-row').first()
      const type = await row.evaluate((e) => {
        const name = e.querySelector('.text-base'),
          preview = e.querySelector('p.text-sm'),
          time = e.querySelector('.tabular-nums')
        return {
          name: getComputedStyle(name).fontSize,
          preview: getComputedStyle(preview).fontSize,
          time: getComputedStyle(time).fontSize,
          height: e.getBoundingClientRect().height,
        }
      })
      assert.equal(type.name, '16px')
      assert.equal(type.preview, '14px')
      assert.equal(type.time, '12px')
      assert(type.height >= 80)
      for (const btn of await list.locator('button').all()) {
        const b = await btn.boundingBox()
        assert(b.height >= 44)
      }
      assert.equal(
        await list.getByText('Percakapan', { exact: true }).count(),
        0,
      )
      assert(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      )
      if (width < 768) {
        assert.equal(
          await page.locator('header:visible h1').textContent(),
          'Inbox',
        )
        assert.equal(
          (await page.locator('header:visible').boundingBox()).height,
          56,
        )
        await page
          .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
          .getByLabel('5 pesan belum dibaca', { exact: true })
          .waitFor()
      }
      await page.screenshot({
        path: out + '/list-' + theme + '-' + width + '.png',
      })
      measurements.push({ width, theme, typography: type, noOverflow: true })
    }
  let list = page.locator('.inbox-list:visible')
  await list.getByRole('button', { name: /^Agen/ }).click()
  assert.equal(await list.locator('.inbox-contact-row').count(), 3)
  assert.equal(
    await list
      .getByRole('button', { name: /^Agen/ })
      .getAttribute('aria-pressed'),
    'true',
  )
  await list.getByRole('button', { name: /^Pelanggan/ }).click()
  assert.equal(await list.locator('.inbox-contact-row').count(), 1)
  await list.getByRole('button', { name: 'Semua', exact: true }).click()
  await list
    .getByRole('textbox', { name: 'Cari percakapan', exact: true })
    .fill('helper')
  assert.equal(await list.locator('.inbox-contact-row').count(), 1)
  await list
    .getByRole('textbox', { name: 'Cari percakapan', exact: true })
    .fill('no-match')
  await list
    .getByText('Belum ada percakapan yang cocok.', { exact: true })
    .waitFor()
  await list
    .getByRole('textbox', { name: 'Cari percakapan', exact: true })
    .fill('')
  await list.locator('.inbox-contact-row').first().click()
  await page.getByRole('textbox', { name: 'Pesan', exact: true }).waitFor()
  await page
    .getByRole('button', { name: 'Kembali ke percakapan', exact: true })
    .click()
  await page.waitForURL('**/inbox')
  await page
    .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
    .waitFor()
  fail = true
  await page.reload()
  list = page.locator('.inbox-list:visible')
  await list.getByRole('alert').waitFor()
  fail = false
  await list.getByRole('button', { name: 'Coba lagi', exact: true }).click()
  await list.locator('.inbox-contact-row').first().waitFor()
  empty = true
  await page.reload()
  await list
    .getByText('Belum ada percakapan yang cocok.', { exact: true })
    .waitFor()
  assert.equal(
    await page
      .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
      .getByLabel(/pesan belum dibaca/)
      .count(),
    0,
  )
  assert.deepEqual(errors, [])
  assert.deepEqual(writes, [])
  assert.deepEqual(unknown, [])
  fs.writeFileSync(
    out + '/browser.json',
    JSON.stringify(
      {
        fixture: true,
        exactProductionBuild: true,
        measurements,
        checks: [
          'compact header',
          'normal statuses omitted',
          'blocking warnings retained',
          'plain preview without agent prefix or draft JSON',
          'search and filters',
          'real unread counts only',
          'Back restores navigation',
          'error/retry and empty state',
          '44px targets',
        ],
        errors,
        writes,
        unknown,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(
    'PASS Inbox list:8 viewport/theme combinations, typography, filters/search, warning/unread states, Back, error/retry',
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
