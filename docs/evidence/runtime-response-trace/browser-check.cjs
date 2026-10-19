// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/runtime-response-trace',
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
const trace = {
  request_id: '54d6f90e-5e40-4138-ba51-0d56042fdc4e',
  message_id: 'm1',
  agent_id: 'agent_good',
  state: 'failed',
  created_at: now,
  started_at: now,
  safe_error: 'Agen belum menghasilkan balasan. Pesan tersimpan.',
  steps: [
    { seq: 1, kind: 'accepted', text: 'Pesan diterima.', at: now },
    {
      seq: 2,
      kind: 'provider',
      text: 'Proses agen berjalan; menunggu respons provider.',
      at: now,
    },
    {
      seq: 3,
      kind: 'provider_result',
      text: 'Proses provider selesai: exit 0, durasi 7.4 dtk.',
      at: now,
    },
    {
      seq: 4,
      kind: 'result_validation',
      text: 'Provider tidak menghasilkan teks balasan.',
      at: now,
    },
  ],
}
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
  requests: [trace],
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
  trace.steps = [
    { seq: 1, kind: 'accepted', text: 'Pesan diterima.', at: now },
    { seq: 2, kind: 'runtime_init', text: 'Sesi runtime dimulai.', at: now },
    {
      seq: 3,
      kind: 'runtime_session',
      text: 'ID sesi runtime: 22222222-3333-4444-5555-666666666666',
      at: now,
    },
    {
      seq: 4,
      kind: 'runtime_provider_started',
      text: 'Provider menyiapkan respons.',
      at: now,
    },
    {
      seq: 5,
      kind: 'runtime_tool_started',
      text: 'Tool #2 · Baca file dimulai.',
      at: now,
    },
    {
      seq: 6,
      kind: 'runtime_tool_failed',
      text: 'Tool #2 · Baca file gagal (0.2 dtk).',
      at: now,
    },
    {
      seq: 7,
      kind: 'runtime_provider_failed',
      text: 'Tahap respons provider gagal (1.4 dtk).',
      at: now,
    },
    {
      seq: 8,
      kind: 'provider_result',
      text: 'Proses provider selesai: exit 0, durasi 7.4 dtk.',
      at: now,
    },
    {
      seq: 9,
      kind: 'result_validation',
      text: 'Provider tidak menghasilkan teks balasan.',
      at: now,
    },
  ]
  for (const width of [1440, 820, 390, 360]) {
    await page.setViewportSize({ width, height: 900 })
    await page.goto(base + '/inbox/room0')
    const panel = page.getByLabel('Status respons', { exact: true })
    await panel.getByText('Gagal', { exact: true }).waitFor()
    const response = panel.locator('details').first(),
      technical = panel.locator('details details')
    assert.equal(await response.getAttribute('open'), null)
    assert.equal(
      await panel.getByText(trace.safe_error, { exact: true }).isVisible(),
      false,
    )
    assert.equal(
      await panel.getByText(trace.request_id, { exact: true }).isVisible(),
      false,
    )
    assert((await panel.boundingBox()).height <= 56)
    await page.screenshot({ path: out + '/closed-' + width + '.png' })
    await response.locator(':scope > summary').click()
    await panel
      .getByRole('list', { name: 'Timeline respons', exact: true })
      .waitFor()
    assert.equal(await technical.getAttribute('open'), null)
    assert.equal(
      await panel.getByText(trace.request_id, { exact: true }).isVisible(),
      false,
    )
    assert.equal(
      await panel
        .getByText('Proses provider selesai: exit 0, durasi 7.4 dtk.', {
          exact: true,
        })
        .isVisible(),
      false,
    )
    await panel.getByText('Tool gagal', { exact: true }).waitFor()
    await panel.getByText('Provider gagal', { exact: true }).waitFor()
    assert.equal(
      await panel
        .getByText('22222222-3333-4444-5555-666666666666', { exact: true })
        .isVisible(),
      false,
    )
    await page.screenshot({ path: out + '/process-' + width + '.png' })
    await technical.locator('summary').click()
    await panel.getByText(trace.request_id, { exact: true }).waitFor()
    await panel
      .getByText('22222222-3333-4444-5555-666666666666', { exact: true })
      .waitFor()
    await panel
      .getByText('Proses provider selesai: exit 0, durasi 7.4 dtk.', {
        exact: true,
      })
      .waitFor()
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth,
      ),
      false,
    )
    await page.screenshot({ path: out + '/technical-' + width + '.png' })
    viewports.push({
      width,
      closedHeight: 44,
      nestedLayers: true,
      noOverflow: true,
    })
  }
  await page.evaluate(() =>
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: async (text) => {
          window.__traceCopied = text
        },
      },
    }),
  )
  await page
    .getByRole('button', { name: 'Salin detail teknis', exact: true })
    .click()
  await page.getByText('Detail tersalin.', { exact: true }).waitFor()
  const copy = JSON.parse(await page.evaluate(() => window.__traceCopied))
  assert.equal(copy.request_id, trace.request_id)
  assert.equal(copy.prompt, undefined)
  trace.steps = trace.steps.slice(0, 2)
  await page.reload()
  const panel = page.getByLabel('Status respons', { exact: true })
  await panel.locator('details > summary').first().click()
  await panel.locator('details details > summary').click()
  await panel
    .getByText(
      'Metadata proses provider belum tersedia untuk permintaan ini.',
      { exact: true },
    )
    .waitFor()
  for (const [state, label] of [
    ['completed', 'Selesai'],
    ['queued', 'Menunggu'],
    ['running', 'Memproses'],
    ['interrupted', 'Terputus'],
    ['cancelled', 'Dibatalkan'],
    ['cancelling', 'Menghentikan…'],
  ]) {
    trace.state = state
    trace.safe_error = ''
    await page.reload()
    await page
      .getByLabel('Status respons', { exact: true })
      .getByText(label, { exact: true })
      .waitFor()
    assert.equal(
      await page.getByRole('button', { name: /Coba lagi|Retry/ }).count(),
      0,
    )
  }
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
          'compact status summary',
          'streamed tool failure distinct from provider failure; session ID technical-only',
          'process timeline and error behind disclosure',
          'technical IDs/exit/copy in nested disclosure',
          'all request states',
          'legacy missing metadata',
          'no retry or provider calls',
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
    'PASS layered response trace:compact/process/technical at4 widths, copy and all request states; no replay',
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
