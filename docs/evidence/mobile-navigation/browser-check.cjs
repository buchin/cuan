// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/mobile-navigation',
  base = 'http://127.0.0.1:4181'
const now = new Date().toISOString()
const projects = [
  {
    id: 'p1',
    name: 'Cuan fixture',
    project_type: 'custom',
    repo_url: 'https://github.com/example/cuan',
    default_branch: 'main',
    is_archived: false,
    created_at: now,
    updated_at: now,
  },
  {
    id: 'p2',
    name: 'Archived fixture',
    project_type: 'custom',
    repo_url: '',
    is_archived: true,
    created_at: now,
    updated_at: now,
  },
]
const agents = [
  {
    id: 'agent_good',
    name: '@verified',
    harness: 'agy',
    enabled: true,
    connection_state: 'verified',
    verified_at: now,
    last_checked_at: now,
  },
  {
    id: 'agent_old',
    name: '@expired',
    harness: 'agy',
    enabled: true,
    connection_state: 'verified',
    verified_at: new Date(Date.now() - 600000).toISOString(),
  },
  {
    id: 'agent_login',
    name: '@login',
    harness: 'agy',
    enabled: true,
    connection_state: 'login_required',
  },
]
const contacts = agents.map((a, i) => ({
  id: 'contact_' + a.id,
  type: 'agent',
  name: a.name,
  avatar: '',
  conversation_id: 'room' + i,
  status: 'idle',
  unread_count: 0,
  last_message: 'Latest fixture message',
  last_message_at: now,
  updated_at: now,
}))
const tasks = [
  {
    id: 'task_one',
    project_id: 'p1',
    title: 'Readability fixture task',
    status: 'backlog',
    priority: 'normal',
    assigned_agent: 'agent_good',
    instructions: 'Fixture instructions',
    created_at: now,
    updated_at: now,
  },
  {
    id: 'task_two',
    project_id: 'p2',
    title: 'Different project task',
    status: 'review',
    priority: 'low',
    created_at: now,
    updated_at: now,
  },
]
const markdown =
  '## Fixture answer\n\nA **useful** answer with `inline code`.\n\n1. Read the project\n2. Recommend next steps\n\n```go\npackage main\n```\n\n| Item | Result |\n| --- | --- |\n| UI | Verified |'
const history = (id) => ({
  conversation_id: id,
  room_id: id,
  contact: contacts.find((c) => c.conversation_id === id),
  messages: [
    {
      id: 1,
      uuid: 'm1',
      author_type: 'user',
      sender_type: 'visitor',
      content: 'Read project fixture',
      created_at: now,
    },
    {
      id: 2,
      uuid: 'm2',
      author_type: 'agent',
      sender_type: 'operator',
      author_name: '@verified',
      content: markdown,
      created_at: now,
    },
  ],
  requests: [],
  tasks: [],
})
let browser, server
let contactFail = false,
  crmFail = false,
  crmEmpty = false
const crm = [
  {
    id: 'crm1',
    name: 'Pelanggan Satu',
    email: 'one@example.invalid',
    product: 'Dojo',
    tags: ['VIP'],
    conversation_id: 'room0',
    updated_at: now,
  },
  {
    id: 'crm2',
    name: 'Pelanggan Dua',
    email: '',
    product: 'TermAPI',
    tags: [],
    conversation_id: 'room1',
    updated_at: now,
  },
]

const writes = [],
  unknown = [],
  errors = [],
  viewports = []
;(async () => {
  fs.mkdirSync(out, { recursive: true })
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
    const r = route.request(),
      u = new URL(r.url()),
      p = u.pathname,
      method = r.method()
    const json = (data, status = 200) =>
      route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(data),
      })
    if (method !== 'GET') {
      writes.push({ method, path: p })
      if (method === 'PATCH' && p === '/api/v1/contacts/crm1') {
        const tags = r.postDataJSON().tags
        crm[0].tags = tags
        return json({ tags })
      }
      if (
        method === 'POST' &&
        /\/projects\/p[12]\/(archive|unarchive)$/.test(p)
      ) {
        const x = projects.find((x) => p.includes('/' + x.id + '/'))
        x.is_archived = p.endsWith('/archive')
        return json(x)
      }
      return json({ error: 'Unexpected fixture write' }, 400)
    }
    if (p.endsWith('/auth/status')) return json({ setup_required: false })
    if (p.endsWith('/auth/me'))
      return json({
        owner: {
          id: 'fixture',
          username: 'Fixture user',
          email: 'fixture@example.invalid',
        },
      })
    if (p === '/api/v1/contacts')
      return json(
        crmFail
          ? { error: 'Kontak belum dapat dimuat' }
          : { contacts: crmEmpty ? [] : crm },
        crmFail ? 503 : 200,
      )
    if (p === '/api/v1/projects') return json({ projects })
    if (p === '/api/v1/agents') return json({ agents })
    if (p === '/api/v1/tasks') return json({ tasks })
    if (p === '/api/v1/chat/contacts')
      return json(
        contactFail ? { error: 'fixture failure' } : { contacts },
        contactFail ? 503 : 200,
      )
    const chat = p.match(/\/chat\/conversations\/([^/]+)\/(messages|events)$/)
    if (chat) {
      const data = history(chat[1])
      return chat[2] === 'messages'
        ? json(data)
        : route.fulfill({
            status: 200,
            contentType: 'text/event-stream',
            body: 'event: snapshot\ndata: ' + JSON.stringify(data) + '\n\n',
          })
    }
    if (p === '/api/v1/tasks/task_one') return json(tasks[0])
    if (p.includes('/api/v1/tasks/msg_'))
      return json({ error: 'not found' }, 404)
    unknown.push(p)
    return json({ error: 'No fixture route' }, 404)
  })
  for (const width of [1440, 1200, 1024, 820, 768, 767, 390, 360]) {
    await page.setViewportSize({ width, height: 900 })
    for (const route of [
      'inbox',
      'contacts',
      'tasks',
      'projects',
      'settings',
      'settings/integrations/live-chat',
    ]) {
      await page.goto(base + '/' + route)
      await page.locator('.primary-navigation').waitFor({ state: 'attached' })
      assert.equal(
        await page
          .getByRole('button', { name: 'Open Navigation Drawer' })
          .count(),
        0,
      )
      assert.equal(
        await page
          .getByRole('dialog', { name: 'Navigasi', exact: true })
          .count(),
        0,
      )
      assert.equal(
        await page.evaluate(
          () => document.documentElement.scrollWidth > innerWidth,
        ),
        false,
      )
      if (width < 768) {
        assert(
          await page
            .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
            .isVisible(),
        )
        assert.equal(
          await page
            .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
            .getByRole('button')
            .count(),
          4,
        )
        assert(
          await page
            .getByRole('button', { name: 'Settings', exact: true })
            .isVisible(),
        )
        assert(
          await page
            .getByRole('button', { name: 'Menu akun', exact: true })
            .isVisible(),
        )
        assert.equal(
          await page.locator('.primary-navigation').isVisible(),
          false,
        )
      } else assert(await page.locator('.primary-navigation').isVisible())
      viewports.push({ width, route, noDrawer: true, noOverflow: true })
      if (width === 390 && ['inbox', 'settings'].includes(route))
        await page.screenshot({ path: out + '/' + route + '-390.png' })
    }
  }
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(base + '/inbox')
  for (const label of ['Contacts', 'Tasks', 'Projects', 'Inbox']) {
    await page
      .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
      .getByRole('button', { name: label, exact: true })
      .click()
    await page.waitForURL('**/' + label.toLowerCase())
    await page.waitForFunction(
      (label) =>
        [...document.querySelectorAll('.bottom-navigation button')].some(
          (b) =>
            b.textContent.trim() === label &&
            b.getAttribute('aria-current') === 'page',
        ),
      label,
    )
    assert.equal(
      await page
        .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
        .getByRole('button', { name: label, exact: true })
        .getAttribute('aria-current'),
      'page',
    )
  }
  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await page.waitForURL('**/settings')
  await page.getByRole('button', { name: 'Menu akun', exact: true }).click()
  await page
    .getByRole('button', { name: 'Profil & Ganti Password', exact: true })
    .waitFor()
  await page.mouse.click(20, 100)
  await page
    .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
    .getByRole('button', { name: 'Inbox', exact: true })
    .click()
  await page
    .getByRole('button')
    .filter({ has: page.getByText('@verified', { exact: true }) })
    .click()
  await page.getByRole('textbox', { name: 'Pesan', exact: true }).waitFor()
  assert.equal(await page.locator('.bottom-navigation').count(), 0)
  await page.screenshot({ path: out + '/chat-390.png' })
  await page
    .getByRole('button', { name: 'Kembali ke percakapan', exact: true })
    .click()
  await page.waitForURL('**/inbox')
  assert(
    await page
      .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
      .isVisible(),
  )
  await page.setViewportSize({ width: 820, height: 1000 })
  assert(await page.locator('.primary-navigation').isVisible())
  await page.setViewportSize({ width: 390, height: 844 })
  assert.equal(await page.getByRole('dialog').count(), 0)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.getByRole('button', { name: 'Ciutkan navigasi' }).click()
  await page.reload()
  await page.locator('.primary-navigation').waitFor()
  assert.equal(
    await page
      .locator('.primary-navigation')
      .evaluate((e) => e.getBoundingClientRect().width),
    64,
  )
  assert.deepEqual(errors, [])
  assert.deepEqual(unknown, [])
  assert.deepEqual(writes, [])
  fs.writeFileSync(
    out + '/browser.json',
    JSON.stringify(
      {
        fixture: true,
        exactProductionBuild: true,
        viewports,
        checks: [
          'no mobile hamburger or navigation drawer',
          'four bottom tabs navigate with current state',
          'Settings and account header accessible',
          'chat hides bottom navigation, Back restores list and navigation',
          'tablet sidebar and desktop collapse retained',
          'resize creates no drawer or focus trap',
        ],
        errors,
        unknown,
        writes,
        providerInvocations: 0,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(
    'PASS:48 route/viewport checks; bottom tabs, Settings/account, chat Back and desktop/tablet sidebar',
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
