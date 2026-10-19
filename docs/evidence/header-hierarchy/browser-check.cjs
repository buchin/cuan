// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/header-hierarchy',
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
  const routes = [
    ['inbox', 'Inbox', 0],
    ['contacts', 'Contacts', 0],
    ['tasks', 'Tasks', 0],
    ['projects', 'Projects', 0],
    ['settings', 'Settings', 0],
    ['settings/agents', 'Agents', 1],
    ['settings/integrations', 'Integrations', 1],
    ['settings/appearance', 'Tampilan', 1],
    ['settings/integrations/live-chat', 'Live Chat', 2],
    ['projects/p1', 'Cuan fixture', 1],
    ['projects/p1/settings', 'Settings', 2],
  ]
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
      for (const [route, title, parentCount] of routes) {
        await page.goto(base + '/' + route)
        const header = page.locator('.app-page-header:visible')
        await header
          .getByRole('heading', { level: 1, name: title, exact: true })
          .waitFor()
        assert.equal(
          await page.locator('main').getByRole('heading', { level: 1 }).count(),
          1,
        )
        assert.equal(
          await page
            .locator('main')
            .getByRole('heading', { name: title, exact: true })
            .count(),
          1,
        )
        assert.equal(
          await header
            .getByRole('heading', { level: 1 })
            .evaluate((e) => getComputedStyle(e).fontSize),
          '20px',
        )
        assert.equal((await header.boundingBox()).height, width < 768 ? 56 : 64)
        const logo = header.getByRole('img', {
          name: 'Logo Cuan — uang kepeng',
          exact: true,
        })
        assert.equal(await logo.count(), width < 768 ? 1 : 0)
        if (width < 768) assert.equal((await logo.boundingBox()).width, 24)
        const crumbs = header.getByRole('navigation', {
          name: 'Hierarki halaman',
          exact: true,
        })
        assert.equal(
          await crumbs.count(),
          width >= 768 && parentCount > 0 ? 1 : 0,
        )
        if (width >= 768 && parentCount > 0)
          assert.equal(await crumbs.getByRole('link').count(), parentCount)
        assert(!(await header.innerText()).includes('cuan'))
        assert(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
        )
        checks.push({
          width,
          theme,
          route,
          title,
          oneHeading: true,
          fontSize: 20,
          primaryBreadcrumbsAbsent: parentCount === 0,
          noOverflow: true,
        })
        if (theme === 'dark' && [390, 1440].includes(width))
          await page.screenshot({
            path:
              out +
              '/header-' +
              route.replaceAll('/', '-') +
              '-' +
              width +
              '.png',
          })
      }
    }
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto(base + '/projects/p1/settings')
  let crumbs = page.getByRole('navigation', {
    name: 'Hierarki halaman',
    exact: true,
  })
  await crumbs.getByRole('link', { name: 'Cuan fixture', exact: true }).click()
  await page.waitForURL('**/projects/p1')
  await page
    .getByRole('navigation', { name: 'Hierarki halaman', exact: true })
    .getByRole('link', { name: 'Projects', exact: true })
    .click()
  await page.waitForURL('**/projects')
  await page.getByRole('navigation',{name:'Hierarki halaman',exact:true}).waitFor({state:'detached'})
  assert.equal(
    await page
      .getByRole('navigation', { name: 'Hierarki halaman', exact: true })
      .count(),
    0,
  )
  await page.goto(base + '/settings/integrations/live-chat')
  await page
    .getByRole('navigation', { name: 'Hierarki halaman', exact: true })
    .getByRole('link', { name: 'Integrations', exact: true })
    .click()
  await page.waitForURL('**/settings/integrations')
  await page
    .getByRole('navigation', { name: 'Hierarki halaman', exact: true })
    .getByRole('link', { name: 'Settings', exact: true })
    .click()
  await page.waitForURL('**/settings')
  await page.setViewportSize({ width: 360, height: 900 })
  await page.goto(base + '/projects/p1/settings')
  await page
    .getByRole('link', { name: '← Kembali ke project', exact: true })
    .click()
  await page.waitForURL('**/projects/p1')
  await page
    .getByRole('button', { name: '← Semua project', exact: true })
    .click()
  await page.waitForURL('**/projects')
  await page.goto(base + '/contacts?contact=crm1')
  await page
    .getByRole('complementary', { name: 'Detail kontak', exact: true })
    .waitFor()
  assert.equal(
    await page.locator('main').getByRole('heading', { level: 1 }).count(),
    1,
  )
  await page
    .getByRole('button', { name: 'Kembali ke daftar kontak', exact: true })
    .click()
  await page.waitForURL('**/contacts')
  assert.deepEqual(errors, [])
  assert.deepEqual(unknown, [])
  assert.deepEqual(writes, [])
  fs.writeFileSync(
    out + '/browser.json',
    JSON.stringify(
      {
        fixture: true,
        exactProductionBuild: true,
        checks,
        workflows: [
          'one page title across all viewports',
          'logo mobile/sidebar desktop',
          'no primary breadcrumbs',
          'useful project and settings parent links',
          'mobile parent Back controls',
          'Contacts detail retains one h1',
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
    'PASS header hierarchy:88 route/viewport/theme checks; one title, branding, detail breadcrumbs and parent navigation',
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
