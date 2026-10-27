// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/crm-patterns',
  base = 'http://127.0.0.1:4181'
const now = new Date().toISOString()
let contactsEmpty = false,
  taskSaveFail = true
const crm = [
  {
    id: 'crm1',
    name: 'Alice Example',
    email: 'alice@example.invalid',
    product: 'Dojo',
    conversation_id: 'room0',
    tags: ['VIP', 'Pelanggan', 'Aktif'],
    updated_at: now,
  },
  {
    id: 'crm2',
    name: 'Budi',
    email: '',
    product: 'TermAPI',
    conversation_id: 'room1',
    tags: ['Prospek'],
    updated_at: now,
  },
  {
    id: 'crm3',
    name: 'LongContactName'.repeat(12),
    email: 'long_email_'.repeat(10) + '@example.invalid',
    product: 'Dojo',
    conversation_id: 'room2',
    tags: [],
    updated_at: now,
  },
  {
    id: 'crm4',
    name: 'Kontak tag penuh',
    email: 'max@example.invalid',
    product: '',
    conversation_id: 'room3',
    tags: Array.from({ length: 20 }, (_, i) => 'Tag' + i),
    updated_at: now,
  },
]
let tagSaveFail = false,
  crmLoadFail = false

const longTask =
  'Perbaiki navigasi mobile dan pastikan konteks task mudah dibaca ' +
  'long_identifier_'.repeat(8)
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
    title: longTask,
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
let contactFail = false
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
      if (method === 'PATCH' && p.startsWith('/api/v1/contacts/')) {
        if (tagSaveFail) return json({ error: 'Fixture tag save failed' }, 503)
        const c = crm.find((c) => p.endsWith('/' + c.id))
        assert(c)
        c.tags = r.postDataJSON().tags
        return json({ tags: c.tags })
      }
      if (
        method === 'POST' &&
        /\/projects\/p[12]\/(archive|unarchive)$/.test(p)
      ) {
        const x = projects.find((x) => p.includes('/' + x.id + '/'))
        x.is_archived = p.endsWith('/archive')
        return json(x)
      }
      if (method === 'POST' && p === '/api/v1/tasks') {
        if (taskSaveFail)
          return json({ error: 'Fixture task save failed' }, 503)
        const data = r.postDataJSON()
        const task = {
          ...data,
          id: 'new_fixture',
          project_id: data.project_ids?.[0],
          created_at: now,
          updated_at: now,
        }
        tasks.push(task)
        return json(task)
      }
      if (method === 'PUT' && p === '/api/v1/tasks/new_fixture') {
        Object.assign(tasks[0], r.postDataJSON())
        return json(tasks[0])
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
    if (p === '/api/v1/projects') return json({ projects })
    if (p === '/api/v1/contacts')
      return crmLoadFail
        ? json({ error: 'Fixture contacts load failed' }, 503)
        : json({ contacts: contactsEmpty ? [] : crm })
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

  const checks = []
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
      await page.goto(base + '/contacts')
      const list = page.locator('[aria-label="Daftar kontak"]')
      const alice = list
        .getByRole('button')
        .filter({ has: page.getByText('Alice Example', { exact: true }) })
      await alice.waitFor()
      assert.equal(await list.getByRole('button').count(), 4)
      assert.equal(await alice.getByText('+1', { exact: true }).count(), 1)
      assert.equal(await alice.getByText('Aktif', { exact: true }).count(), 0)
      assert(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      )
      await page.screenshot({
        path: out + '/contacts-list-' + theme + '-' + width + '.png',
      })
      await alice.click()
      await page.waitForURL('**/contacts?contact=crm1')
      const detail = page.getByRole('complementary', {
        name: 'Detail kontak',
        exact: true,
      })
      await detail.waitFor()
      assert.equal(await list.isVisible(), width >= 1200)
      await detail
        .getByRole('link', { name: 'Buka percakapan', exact: true })
        .waitFor()
      assert.equal(
        await detail
          .getByRole('link', { name: 'Buka percakapan', exact: true })
          .getAttribute('href'),
        '/inbox/room0',
      )
      assert(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      )
      await page.screenshot({
        path: out + '/contacts-detail-' + theme + '-' + width + '.png',
      })
      checks.push({
        width,
        theme,
        mobileDetailFocused: width < 1200,
        desktopSplit: width >= 1200,
        noOverflow: true,
      })
    }
  let detail = page.getByRole('complementary', {
      name: 'Detail kontak',
      exact: true,
    }),
    form = detail.getByRole('form', { name: 'Kelola tag', exact: true })
  await form.getByRole('button', { name: 'Hapus tag VIP', exact: true }).click()
  await form
    .getByRole('textbox', { name: 'Tambah tag', exact: true })
    .fill('vIp')
  await form.getByRole('button', { name: 'Tambah', exact: true }).click()
  assert.equal(
    await form
      .getByRole('button', { name: 'Hapus tag vIp', exact: true })
      .count(),
    1,
  )
  await form
    .getByRole('textbox', { name: 'Tambah tag', exact: true })
    .fill('VIP')
  await form.getByRole('button', { name: 'Tambah', exact: true }).click()
  assert.equal(
    await form.getByRole('button', { name: /Hapus tag vip/i }).count(),
    1,
  )
  await form
    .getByRole('textbox', { name: 'Tambah tag', exact: true })
    .fill('x'.repeat(41))
  await form.getByRole('button', { name: 'Tambah', exact: true }).click()
  await form
    .getByRole('alert')
    .getByText('Tag maksimal 40 karakter.', { exact: true })
    .waitFor()
  assert.equal(writes.length, 0)
  await form.getByRole('textbox', { name: 'Tambah tag', exact: true }).fill('')
  await form.getByRole('button', { name: 'Prospek', exact: true }).click()
  await form
    .getByRole('button', { name: 'Hapus tag Prospek', exact: true })
    .waitFor()
  await detail
    .getByRole('button', { name: 'Kembali ke daftar kontak', exact: true })
    .click()
  await page
    .locator('[aria-label="Daftar kontak"]')
    .getByRole('button')
    .filter({ has: page.getByText('Alice Example', { exact: true }) })
    .click()
  detail = page.getByRole('complementary', {
    name: 'Detail kontak',
    exact: true,
  })
  form = detail.getByRole('form', { name: 'Kelola tag', exact: true })
  await form
    .getByRole('button', { name: 'Hapus tag Prospek', exact: true })
    .waitFor()
  assert.equal(writes.length, 0)
  tagSaveFail = true
  await form.getByRole('button', { name: 'Simpan tag', exact: true }).click()
  await form.getByRole('alert').waitFor()
  assert.equal(crm[0].tags.includes('Prospek'), false)
  tagSaveFail = false
  await form.getByRole('button', { name: 'Simpan tag', exact: true }).click()
  await form
    .getByRole('status')
    .getByText('Tag tersimpan.', { exact: true })
    .waitFor()
  assert(crm[0].tags.includes('Prospek'))
  await page.reload()
  await page
    .getByRole('button', { name: 'Hapus tag Prospek', exact: true })
    .waitFor()
  form = page.getByRole('form', { name: 'Kelola tag', exact: true })
  while (await form.getByRole('button', { name: /^Hapus tag / }).count())
    await form
      .getByRole('button', { name: /^Hapus tag / })
      .first()
      .click()
  await form.getByRole('button', { name: 'Simpan tag', exact: true }).click()
  await form.getByRole('status').waitFor()
  assert.deepEqual(crm[0].tags, [])
  await page.goto(base + '/contacts?contact=crm4')
  form = page.getByRole('form', { name: 'Kelola tag', exact: true })
  await form
    .getByRole('textbox', { name: 'Tambah tag', exact: true })
    .fill('21st')
  await form.getByRole('button', { name: 'Tambah', exact: true }).click()
  await form
    .getByRole('alert')
    .getByText('Maksimal 20 tag per kontak.', { exact: true })
    .waitFor()
  assert.equal(writes.length, 3)
  await form
    .getByRole('button', { name: 'Simpan tag', exact: true })
    .scrollIntoViewIfNeeded()
  const saveBox = await form
    .getByRole('button', { name: 'Simpan tag', exact: true })
    .boundingBox()
  const navBox = await page
    .getByRole('navigation', { name: 'Navigasi bawah', exact: true })
    .boundingBox()
  assert(saveBox.y + saveBox.height <= navBox.y)
  await page.goto(base + '/contacts?contact=missing')
  await page.getByText('Kontak tidak ditemukan', { exact: true }).waitFor()
  await page
    .getByRole('button', { name: 'Buka daftar kontak', exact: true })
    .click()
  await page
    .getByRole('combobox', { name: 'Filter sumber', exact: true })
    .selectOption('TermAPI')
  assert.equal(
    await page
      .locator('[aria-label="Daftar kontak"]')
      .getByRole('button')
      .count(),
    1,
  )
  await page
    .getByRole('combobox', { name: 'Filter tag', exact: true })
    .selectOption('Prospek')
  assert.equal(
    await page
      .locator('[aria-label="Daftar kontak"]')
      .getByRole('button')
      .count(),
    1,
  )
  await page
    .getByRole('textbox', { name: 'Cari kontak', exact: true })
    .fill('no match')
  await page
    .getByRole('button', { name: 'Hapus semua filter', exact: true })
    .click()
  assert.equal(
    await page
      .locator('[aria-label="Daftar kontak"]')
      .getByRole('button')
      .count(),
    4,
  )
  await page
    .locator('[aria-label="Daftar kontak"]')
    .getByRole('button')
    .filter({ has: page.getByText('Budi', { exact: true }) })
    .click()
  await page.goBack()
  await page.waitForURL('**/contacts')
  assert.equal(
    await page.locator('[aria-label="Daftar kontak"]').isVisible(),
    true,
  )
  crmLoadFail = true
  await page.reload()
  await page.getByRole('alert').waitFor()
  crmLoadFail = false
  await page.getByRole('button', { name: 'Coba lagi', exact: true }).click()
  await page.getByText('Alice Example', { exact: true }).waitFor()
  for (const route of ['tasks', 'projects', 'settings/agents']) {
    await page.goto(base + '/' + route)
    if (route === 'tasks') {
      await page
        .getByRole('combobox', { name: 'Filter project', exact: true })
        .selectOption('p2')
      await page
        .getByRole('textbox', { name: 'Search tasks', exact: true })
        .fill('none')
      await page
        .getByRole('button', { name: 'Clear filters', exact: true })
        .click()
      assert.equal(
        await page
          .getByRole('combobox', { name: 'Filter project', exact: true })
          .inputValue(),
        'all',
      )
      await page.getByRole('button', { name: longTask, exact: true }).waitFor()
    }
    if (route === 'projects') {
      await page.getByRole('button', { name: 'List view', exact: true }).click()
      await page.reload()
      assert.equal(
        await page
          .getByRole('button', { name: 'List view', exact: true })
          .getAttribute('aria-pressed'),
        'true',
      )
      await page
        .getByRole('searchbox', { name: 'Search projects', exact: true })
        .fill('Cuan')
      await page
        .getByRole('button', { name: 'Clear search', exact: true })
        .click()
    }
    if (route === 'settings/agents') {
      await page
        .getByRole('textbox', { name: 'Cari agen', exact: true })
        .fill('no match')
      await page
        .getByText('Tidak ada agen yang cocok.', { exact: true })
        .waitFor()
      await page
        .getByRole('button', { name: 'Tampilkan semua agen', exact: true })
        .click()
      assert.equal(
        await page
          .getByRole('list', { name: 'Daftar agen', exact: true })
          .getByRole('listitem')
          .count(),
        3,
      )
    }
    assert(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    )
    await page.screenshot({
      path: out + '/' + route.replace('/', '-') + '-390.png',
    })
  }
  await page.goto(base + '/projects/p1')
  await page
    .getByRole('textbox', { name: 'Search tasks', exact: true })
    .fill('none')
  await page.getByRole('button', { name: 'Clear filters', exact: true }).click()
  await page.getByRole('button', { name: longTask, exact: true }).waitFor()
  assert.equal(
    await page
      .getByRole('button', { name: 'Different project task', exact: true })
      .count(),
    0,
  )
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
      for (const route of ['tasks', 'projects', 'settings/agents']) {
        await page.goto(base + '/' + route)
        await page.locator('main .page-title').first().waitFor()
        assert(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
        )
        checks.push({ width, theme, route, noOverflow: true })
        if ([390, 1440].includes(width))
          await page.screenshot({
            path:
              out +
              '/patterns-' +
              route.replace('/', '-') +
              '-' +
              theme +
              '-' +
              width +
              '.png',
          })
      }
    }
  contactsEmpty = true
  await page.goto(base + '/contacts')
  await page.getByText('Belum ada kontak', { exact: true }).waitFor()
  await page
    .getByRole('link', { name: 'Hubungkan live chat', exact: true })
    .waitFor()
  assert.deepEqual(errors, [])
  assert.deepEqual(unknown, [])
  assert(
    writes.every(
      (w) => w.method === 'PATCH' && w.path === '/api/v1/contacts/crm1',
    ),
  )
  assert.equal(writes.length, 3)
  fs.writeFileSync(
    out + '/browser.json',
    JSON.stringify(
      {
        fixture: true,
        exactProductionBuild: true,
        checks,
        workflows: [
          'compact list, max2 tags',
          'mobile focused detail/Back and desktop split',
          'query deep link and reload',
          'chip add/remove, existing tags and case-insensitive duplicates',
          '40character and20tag validation',
          'draft preserved on Back, save failure retry, empty tags save',
          'source/tag/search filters and reset',
          'unknown contact, load error retry and empty state',
          'Tasks reset includes project',
          'Projects view preference and reset',
          'Agents search without provider probes',
        ],
        errors,
        writes,
        unknown,
        providerInvocations: 0,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(
    'PASS CRM patterns:32 page/theme/viewport checks, tag validation/save/retry, deep links/Back, source/tag filters, Tasks/Projects/Agents reset/search',
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
