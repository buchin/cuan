// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/contacts-crm',
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
  await page.goto(base + '/team')
  await page.waitForURL('**/settings/agents')
  await page.getByRole('heading', { name: 'Agen', exact: true }).waitFor()
  assert.equal(
    await page
      .getByRole('navigation', { name: 'Navigasi utama', exact: true })
      .getByRole('button', { name: 'Team', exact: true })
      .count(),
    0,
  )
  assert.equal(
    await page
      .getByRole('navigation', { name: 'Navigasi utama', exact: true })
      .getByRole('button', { name: 'Contacts', exact: true })
      .count(),
    1,
  )
  await page.getByRole('button', { name: 'Tambah agen', exact: true }).click()
  await page.getByRole('dialog', { name: 'Tambah agen' }).waitFor()
  await page.keyboard.press('Escape')
  await page.getByRole('dialog').waitFor({ state: 'detached' })
  assert.equal(
    writes.length,
    0,
    'Opening settings/add modal must not start OAuth or verify provider',
  )
  // Pin project settings to URL; reload works without previous project selection.
  await page.goto(base + '/settings')
  await page.getByRole('link', { name: 'Cuan fixture', exact: true }).click()
  await page.waitForURL('**/projects/p1/settings')
  await page.getByRole('heading', { name: 'Settings: Cuan fixture' }).waitFor()
  assert.equal(
    await page.getByRole('button', { name: /Simpan|Delete Project/ }).count(),
    0,
  )
  assert.equal(await page.locator('input').count(), 0)
  await page.reload()
  await page.getByRole('heading', { name: 'Settings: Cuan fixture' }).waitFor()
  // Archive then restore using supported endpoints; fixture data only.
  page.once('dialog', (d) => d.accept())
  await page
    .getByRole('button', { name: 'Arsipkan project', exact: true })
    .click()
  await page
    .getByRole('status')
    .getByText('Project telah diarsipkan.')
    .waitFor()
  await page.goto(base + '/projects')
  await page.getByRole('button', { name: /Archived \(2\)/ }).click()
  page.once('dialog', (d) => d.accept())
  await page.getByTitle('Unarchive', { exact: true }).first().click()
  await page.getByRole('button', { name: /Active Projects \(1\)/ }).waitFor()
  // Project board actually scopes tasks; main list is separate.
  await page.goto(base + '/projects/p1')
  await page
    .getByRole('button', { name: 'Readability fixture task', exact: true })
    .waitFor()
  assert.equal(
    await page
      .getByRole('button', { name: 'Different project task', exact: true })
      .count(),
    0,
  )
  // Task detail is a real modal and returns to its source project.
  await page
    .getByRole('button', { name: 'Readability fixture task', exact: true })
    .click()
  const taskDialog = page.getByRole('dialog', {
    name: 'Task detail',
    exact: true,
  })
  await taskDialog.waitFor()
  for (let i = 0; i < 20; i++) {
    await page.keyboard.press('Tab')
    assert(
      await page.evaluate(() => !!document.activeElement.closest('dialog')),
      'focus escaped task detail',
    )
  }
  await page.keyboard.press('Escape')
  await page.waitForURL('**/projects/p1')
  await page
    .getByRole('button', { name: 'Readability fixture task', exact: true })
    .waitFor()
  for (const width of [1440, 1280, 1024, 820, 768, 390, 360]) {
    await page.setViewportSize({ width, height: 900 })
    for (const route of [
      'inbox',
      'tasks',
      'projects',
      'settings',
      'settings/agents',
    ]) {
      await page.goto(base + '/' + route)
      await page.waitForTimeout(160)
      assert(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
        'page overflow ' + width + ' ' + route,
      )
      const sizes = await page.evaluate(() => ({
        nav: document
          .querySelector('.primary-navigation')
          .getBoundingClientRect().width,
        main: document
          .querySelector('.app-shell > main')
          .getBoundingClientRect().width,
        context:
          document
            .querySelector('.conversation-sidebar')
            ?.getBoundingClientRect().width || 0,
      }))
      if (width >= 768 && width < 1200) {
        assert.equal(sizes.nav, 64)
        assert.equal(sizes.context, 0)
        assert.equal(sizes.main, width - 64)
      }
      viewports.push({ width, route, ...sizes, noOverflow: true })
      if (
        (width === 820 && route === 'inbox') ||
        (width === 1440 && ['inbox', 'settings/agents'].includes(route)) ||
        (width === 390 && ['projects', 'settings/agents'].includes(route))
      )
        await page.screenshot({
          path: out + '/' + route.replace('/', '-') + '-' + width + '.png',
        })
    }
  }
  // Mobile: list -> chat -> settings for the correct agent -> return to same chat.
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(base + '/inbox')
  await page
    .getByRole('button')
    .filter({ has: page.getByText('@verified', { exact: true }) })
    .click()
  await page.getByRole('textbox', { name: 'Pesan', exact: true }).waitFor()
  assert.equal(await page.locator('.chat-markdown').count(), 1)
  assert.equal(
    await page
      .locator('.chat-markdown')
      .evaluate((e) => getComputedStyle(e).fontSize),
    '16px',
  )
  assert.equal(
    await page
      .getByRole('textbox', { name: 'Pesan', exact: true })
      .evaluate((e) => getComputedStyle(e).fontSize),
    '16px',
  )
  assert.equal(
    await page
      .getByRole('button', { name: 'Kirim pesan', exact: true })
      .evaluate((e) => e.getBoundingClientRect().height),
    48,
  )
  await page.getByRole('link', { name: 'Kelola koneksi agen' }).click()
  assert(page.url().includes('agent=agent_good'))
  await page.getByRole('link', { name: '← Kembali ke percakapan' }).click()
  await page.waitForURL('**/inbox/room0')
  await page.getByRole('textbox', { name: 'Pesan', exact: true }).waitFor()
  await page.screenshot({ path: out + '/chat-390.png' })
  await page.getByTitle('Kembali ke percakapan').click()
  await page.waitForURL('**/inbox')
  await page
    .getByRole('textbox', { name: 'Pesan', exact: true })
    .waitFor({ state: 'detached' })
  await page.getByRole('textbox', { name: 'Cari percakapan' }).waitFor()
  const list = page.locator('main')
  assert.equal(await list.getByText('Terhubung', { exact: true }).count(), 1)
  assert.equal(
    await list.getByText('Belum diperiksa', { exact: true }).count(),
    1,
  )
  assert.equal(await list.getByText('Perlu login', { exact: true }).count(), 1)
  assert.equal(await list.getByText('Connected', { exact: true }).count(), 0)
  // Drawer is absent when closed, traps focus and restores focus on Escape/backdrop/navigation.
  await page.goto(base + '/tasks')
  const trigger = page.getByRole('button', { name: 'Open Navigation Drawer' })
  assert.equal(await page.locator('dialog').count(), 0)
  await trigger.click()
  const dialog = page.getByRole('dialog', { name: 'Navigasi', exact: true })
  await dialog.waitFor()
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press('Tab')
    assert(
      await page.evaluate(() => !!document.activeElement.closest('dialog')),
      'focus escaped drawer',
    )
  }
  await page.keyboard.press('Escape')
  await dialog.waitFor({ state: 'detached' })
  assert(
    await trigger.evaluate((e) => document.activeElement === e),
    'focus not restored',
  )
  await trigger.click()
  await page.mouse.click(380, 300)
  await dialog.waitFor({ state: 'detached' })
  await trigger.click()
  await dialog.getByRole('button', { name: 'Settings', exact: true }).click()
  await page.waitForURL('**/settings')
  assert.equal(await page.locator('dialog').count(), 0)
  await page.goto(base + '/tasks')
  await trigger.click()
  await page.setViewportSize({ width: 820, height: 1000 })
  await page.getByRole('dialog').waitFor({ state: 'detached' })
  // Navigation collapse preference and theme preference survive reload.
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.getByRole('button', { name: 'Ciutkan navigasi' }).click()
  await page.reload()
  assert.equal(
    await page
      .locator('.primary-navigation')
      .evaluate((e) => e.getBoundingClientRect().width),
    64,
  )
  await page.getByRole('button', { name: 'Perluas navigasi' }).click()
  await page
    .getByRole('button', { name: 'Ganti tema', exact: true })
    .first()
    .click()
  await page.reload()
  assert(
    await page.locator('html').evaluate((e) => e.classList.contains('dark')),
  )
  await page.screenshot({ path: out + '/tasks-dark-1440.png' })
  contactFail = true
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto(base + '/inbox')
  await page
    .getByRole('alert')
    .getByText('Kontak belum dapat dimuat. Periksa koneksi dan coba lagi.')
    .waitFor()
  contactFail = false
  await page.getByRole('button', { name: 'Coba lagi', exact: true }).click()
  await page.locator('main').getByText('@verified', { exact: true }).waitFor()
  assert.equal(errors.length, 0)
  assert.equal(
    unknown.length,
    0,
    'unhandled API requests: ' + unknown.join(','),
  )
  assert(
    writes.every(
      (w) =>
        (w.method === 'POST' &&
          /\/projects\/p[12]\/(archive|unarchive)$/.test(w.path)) ||
        (w.method === 'PATCH' && w.path === '/api/v1/contacts/crm1'),
    ),
  )
  // CRM directory, persisted tags fixture, search, filter, states and chat navigation.
  for (const width of [1440, 1024, 820, 768, 390, 360]) {
    await page.setViewportSize({ width, height: 900 })
    await page.goto(base + '/contacts')
    await page.getByRole('button', { name: /Pelanggan Satu/ }).waitFor()
    await page.getByRole('button', { name: /Pelanggan Satu/ }).click()
    await page.getByLabel('Tag', { exact: true }).waitFor()
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth,
      ),
      false,
    )
    await page.screenshot({ path: out + '/contacts-' + width + '.png' })
    viewports.push({ width, route: 'contacts', noOverflow: true })
  }
  await page.getByLabel('Tag', { exact: true }).fill('VIP, Pelanggan aktif')
  await page.getByRole('button', { name: 'Simpan tag', exact: true }).click()
  await page.waitForFunction(
    () =>
      document.querySelector('#contact-tags')?.value === 'VIP, Pelanggan aktif',
  )
  await page.reload()
  await page.getByRole('button', { name: /Pelanggan Satu/ }).click()
  assert.equal(
    await page.getByLabel('Tag', { exact: true }).inputValue(),
    'VIP, Pelanggan aktif',
  )
  await page.getByLabel('Cari kontak').fill('one@example.invalid')
  assert.equal(
    await page.getByRole('button', { name: /Pelanggan Dua/ }).count(),
    0,
  )
  await page.getByLabel('Cari kontak').fill('')
  await page.getByLabel('Filter tag').selectOption('Pelanggan aktif')
  assert.equal(
    await page.getByRole('button', { name: /Pelanggan Dua/ }).count(),
    0,
  )
  await page.getByRole('link', { name: 'Buka percakapan' }).click()
  await page.waitForURL('**/inbox/room0')
  crmFail = true
  await page.goto(base + '/contacts')
  await page.getByRole('alert').waitFor()
  crmFail = false
  await page.getByRole('button', { name: 'Coba lagi', exact: true }).click()
  await page.getByRole('button', { name: /Pelanggan Satu/ }).waitFor()
  crmEmpty = true
  await page.reload()
  await page.getByRole('heading', { name: 'Belum ada kontak' }).waitFor()
  crmEmpty = false
  assert.deepEqual(errors, [])
  fs.writeFileSync(
    out + '/browser.json',
    JSON.stringify(
      {
        fixture: true,
        exactProductionBuild: true,
        providerInvocations: 0,
        contactsCRM: true,
        viewports,
        checks: [
          'CRM directory/search/tag filter/save/reload/chat link/empty/error/retry',
          'legacy Team redirect',
          'Settings→Agents without auth probes',
          'project URL settings/reload',
          'honest repository read-only details',
          'archive/restore contract',
          'project-scoped tasks',
          'task modal focus/Escape/source return',
          'mobile list/chat/return',
          'factual agent status',
          '16px message/composer and 48px send',
          'closed drawer unmounted',
          'native focus trap/Escape/backdrop/focus restore',
          'drawer closes on breakpoint',
          'navigation preference',
          'dark persistence',
          'contacts fetch failure and retry',
        ],
        writes,
        unknown,
        errors,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(
    'PASS: responsive UI regression and Contacts directory/filter/tags/reload/chat/empty/error/retry',
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
