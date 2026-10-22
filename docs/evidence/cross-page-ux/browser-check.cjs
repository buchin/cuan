// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/cross-page-ux',
  base = 'http://127.0.0.1:4181'
const now = new Date().toISOString()
let contactsEmpty = false,
  taskSaveFail = true
const crm = [
  {
    id: 'crm1',
    name: 'Pelanggan satu',
    email: 'one@example.invalid',
    product: 'Dojo',
    conversation_id: 'room0',
    tags: ['VIP'],
    updated_at: now,
  },
]
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
      return json({ contacts: contactsEmpty ? [] : crm })
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
      for (const route of [
        'inbox',
        'contacts',
        'tasks',
        'projects',
        'settings',
        'settings/agents',
        'settings/integrations/live-chat',
        'settings/appearance',
      ]) {
        await page.goto(base + '/' + route)
        await page.locator('main h1').first().waitFor()
        if (route === 'contacts')
          await page.getByText('Pelanggan satu', { exact: true }).waitFor()
        if (route === 'tasks')
          await page
            .getByRole('button', { name: longTask, exact: true })
            .waitFor()
        if (route === 'projects')
          await page
            .getByRole('button', { name: 'Cuan fixture', exact: true })
            .waitFor()
        assert(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
          'overflow ' + route + ' ' + width,
        )
        if (width < 768 && route === 'inbox') {
          const logo = page
            .locator('header:visible')
            .getByRole('img', { name: 'Logo Cuan — uang kepeng', exact: true })
          assert.equal((await logo.boundingBox()).width, 24)
          assert.equal(
            (await page.locator('header:visible').boundingBox()).height,
            56,
          )
        }
        if (route === 'contacts') {
          assert.equal(
            await page
              .getByRole('heading', { name: 'Contacts', exact: true })
              .evaluate((e) => getComputedStyle(e).fontSize),
            width >= 1200 ? '24px' : '20px',
          )
          const section = page.getByRole('region', {
            name: 'Contacts',
            exact: true,
          })
          assert.equal(
            await section.evaluate((e) => getComputedStyle(e).paddingLeft),
            '0px',
          )
        }
        if (route === 'tasks' && width < 640) {
          assert.equal(
            await page
              .getByRole('columnheader', { name: 'Code', exact: true })
              .isVisible(),
            false,
          )
          const title = page.getByRole('button', {
            name: longTask,
            exact: true,
          })
          assert.equal(
            await title.evaluate((e) => getComputedStyle(e).fontSize),
            '16px',
          )
          const b = await title.boundingBox()
          assert(b.width > 150)
        }
        if (route.startsWith('settings')) {
          const tabs = page.getByRole('navigation', {
            name: 'Bagian pengaturan',
            exact: true,
          })
          const boxes = await Promise.all(
            (await tabs.getByRole('link').all()).map((e) => e.boundingBox()),
          )
          assert(boxes.every((b) => b.height >= 44))
          assert(
            boxes.every((b) => Math.abs(b.y - boxes[0].y) < 1),
            'settings tabs wrapped',
          )
        }
        if (route === 'projects') {
          for (const label of ['Active Projects (1)', 'Archived (1)'])
            assert(
              (
                await page
                  .getByRole('button', { name: label, exact: true })
                  .boundingBox()
              ).height >= 44,
            )
        }
        checks.push({ width, theme, route, noOverflow: true })
        if (
          [390, 1440].includes(width) &&
          ['inbox', 'contacts', 'tasks', 'projects', 'settings'].includes(route)
        )
          await page.screenshot({
            path: out + '/after-' + route + '-' + theme + '-' + width + '.png',
          })
      }
    }
  await page.setViewportSize({ width: 360, height: 900 })
  await page.goto(base + '/projects')
  await page
    .getByRole('searchbox', { name: 'Search projects', exact: true })
    .fill('not found fixture')
  await page
    .getByText('No projects match your search', { exact: true })
    .waitFor()
  assert.equal(
    await page.getByText('No active projects yet', { exact: true }).count(),
    0,
  )
  await page.getByRole('button', { name: 'Clear search', exact: true }).click()
  await page
    .getByRole('button', { name: 'Cuan fixture', exact: true })
    .waitFor()
  await page.getByRole('button', { name: 'List view', exact: true }).click()
  const project = page.getByRole('button', { name: /Cuan fixture.*github/ })
  await project.focus()
  await page.keyboard.press('Enter')
  await page.waitForURL('**/projects/p1')
  await page.getByRole('button', { name: longTask, exact: true }).waitFor()
  assert.equal(
    await page
      .getByRole('button', { name: 'Different project task', exact: true })
      .count(),
    0,
  )
  await page.getByRole('button', { name: longTask, exact: true }).click()
  await page.getByRole('dialog', { name: 'Task detail', exact: true }).waitFor()
  await page.keyboard.press('Escape')
  await page.waitForURL('**/projects/p1')
  await page.goto(base + '/tasks')
  await page
    .getByRole('textbox', { name: 'Search tasks', exact: true })
    .fill('no match')
  await page.getByRole('button', { name: 'Clear filters', exact: true }).click()
  await page.getByRole('button', { name: longTask, exact: true }).waitFor()
  tasks.splice(0)
  await page.reload()
  await page.getByText('No tasks yet', { exact: true }).waitFor()
  await page
    .getByRole('button', { name: 'Create first task', exact: true })
    .click()
  const createDialog = page.getByRole('dialog', {
    name: 'New task',
    exact: true,
  })
  await createDialog.waitFor()
  for (let i = 0; i < 14; i++) {
    await page.keyboard.press('Tab')
    assert(
      await page.evaluate(() => !!document.activeElement.closest('dialog')),
    )
  }
  for (const width of [360, 390, 820, 1440]) {
    await page.setViewportSize({ width, height: 900 })
    assert(await createDialog.evaluate((e) => e.scrollWidth <= e.clientWidth))
    for (const control of await createDialog
      .locator('input,select,button')
      .all()) {
      assert((await control.boundingBox()).height >= 44)
    }
    await page.screenshot({ path: out + '/new-task-' + width + '.png' })
  }
  await page.keyboard.press('Escape')
  await createDialog.waitFor({ state: 'detached' })
  assert(
    await page
      .getByRole('button', { name: 'Create first task', exact: true })
      .evaluate((e) => e === document.activeElement),
  )
  await page
    .getByRole('button', { name: 'Create first task', exact: true })
    .click()
  await createDialog
    .getByLabel('Judul task', { exact: true })
    .fill('Fixture task from accessible form')
  assert.equal(
    await createDialog
      .getByLabel('Project', { exact: true })
      .getByRole('option', { name: 'Archived fixture', exact: true })
      .count(),
    0,
  )
  await createDialog
    .getByRole('button', { name: 'Buat task', exact: true })
    .click()
  await createDialog.getByRole('alert').waitFor()
  assert.equal(
    await createDialog.getByLabel('Judul task', { exact: true }).inputValue(),
    'Fixture task from accessible form',
  )
  taskSaveFail = false
  await createDialog
    .getByRole('button', { name: 'Buat task', exact: true })
    .click()
  await createDialog.waitFor({ state: 'detached' })
  await page
    .getByRole('button', {
      name: 'Fixture task from accessible form',
      exact: true,
    })
    .waitFor()
  assert.equal(tasks[0].status, 'todo')
  await page.setViewportSize({ width: 360, height: 900 })
  await page.goto(base + '/contacts')
  await page.getByText('Pelanggan satu', { exact: true }).click()
  await page
    .getByRole('complementary', { name: 'Detail kontak', exact: true })
    .waitFor()
  await page
    .getByRole('button', { name: 'Kembali ke daftar kontak', exact: true })
    .click()
  contactsEmpty = true
  await page.reload()
  await page.getByText('Belum ada kontak', { exact: true }).waitFor()
  assert.equal(
    await page
      .getByRole('textbox', { name: 'Cari kontak', exact: true })
      .count(),
    0,
  )
  await page
    .getByRole('link', { name: 'Hubungkan live chat', exact: true })
    .click()
  await page.waitForURL('**/settings/integrations/live-chat')
  assert.deepEqual(errors, [])
  assert.deepEqual(unknown, [])
  assert.equal(writes.length, 3)
  assert(
    writes.every((w) =>
      ['/api/v1/tasks', '/api/v1/tasks/new_fixture'].includes(w.path),
    ),
  )
  fs.writeFileSync(
    out + '/browser.json',
    JSON.stringify(
      {
        fixture: true,
        exactProductionBuild: true,
        checks,
        workflows: [
          'Inbox mobile logo24px without taller header',
          'Contacts consistent padding/title and empty CTA',
          'Tasks long title readable on mobile, scoped board and modal',
          'Tasks empty CTA and filter recovery',
          'Projects accurate search empty state and keyboard list navigation',
          'Settings single-row scrollable44px links',
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
    'PASS cross-page UX: ' +
      checks.length +
      ' viewport/theme/route checks, task modal/empty/filter, project search/keyboard, Contacts detail/empty CTA',
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
