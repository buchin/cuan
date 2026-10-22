// Exact production build, fixture API only. No real tasks or provider calls.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/chat-task-draft',
  base = 'http://127.0.0.1:4181',
  now = new Date().toISOString()
let browser,
  server,
  created = null,
  saveFail = false,
  loadFail = false,
  showDraft = true
const errors = [],
  unknown = [],
  writes = []
const projects = [
  { id: 'p1', name: 'Cuan', is_archived: false },
  { id: 'p2', name: 'Archived', is_archived: true },
  { id: 'p3', name: 'Other', is_archived: false },
]
const contact = {
  id: 'c1',
  name: '@fixture',
  type: 'agent',
  conversation_id: 'room0',
  status: 'idle',
  updated_at: now,
}
const draft = {
  title: 'Rapikan navigasi mobile',
  description:
    'Tujuan: navigasi chat mudah dipakai.\nScope: header dan Back.\nKriteria selesai: tidak overflow pada 360px.',
  project_id: 'p1',
}
const history = () => ({
  conversation_id: 'room0',
  room_id: 'room0',
  contact,
  messages: [
    {
      id: 1,
      uuid: 'm1',
      author_type: 'user',
      content: 'Bikinin task dari pembahasan tadi',
      created_at: now,
    },
    {
      id: 2,
      uuid: 'm2',
      author_type: 'agent',
      content: showDraft
        ? 'Ini draft dari pembahasan kita, belum tersimpan.'
        : 'Halo, ada yang bisa saya bantu?',
      task_draft: showDraft ? draft : undefined,
      created_at: now,
    },
  ],
  requests: [],
  tasks: created ? [created] : [],
})
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
      writes.push({ method: req.method(), path: p, body: req.postDataJSON() })
      if (
        req.method() === 'POST' &&
        p === '/api/v1/chat/conversations/room0/messages/m2/task'
      ) {
        if (saveFail) return json({ error: 'Fixture save unavailable' }, 503)
        created = created || {
          id: 'msg_m2',
          source_message_id: 'm2',
          source_room_id: 'room0',
          status: 'backlog',
          ...req.postDataJSON(),
        }
        return json({ task: created })
      }
      unknown.push(req.method() + ':' + p)
      return json({ error: 'Unexpected write' }, 400)
    }
    if (p.endsWith('/auth/status')) return json({ setup_required: false })
    if (p.endsWith('/auth/me'))
      return json({ owner: { id: 'fixture', username: 'Fixture' } })
    if (p === '/api/v1/projects')
      return loadFail
        ? json({ error: 'Fixture load unavailable' }, 503)
        : json({ projects })
    if (p === '/api/v1/agents') return json({ agents: [] })
    if (p === '/api/v1/tasks') return json({ tasks: created ? [created] : [] })
    if (p === '/api/v1/tasks/msg_m2')
      return created ? json(created) : json({ error: 'Not found' }, 404)
    if (p === '/api/v1/chat/contacts') return json({ contacts: [contact] })
    if (p.endsWith('/messages')) return json(history())
    if (p.endsWith('/events'))
      return route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: 'event: snapshot\ndata: ' + JSON.stringify(history()) + '\n\n',
      })
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
      await page.goto(base + '/inbox/room0')
      const card = page.getByRole('form', { name: 'Draft task', exact: true })
      await card.waitFor()
      await page
        .getByRole('button', { name: 'Buat task', exact: true })
        .waitFor({ state: 'visible' })
      await page.waitForFunction(
        () =>
          !document.querySelector(
            'form[aria-label="Draft task"] button[type]',
          ) ||
          !document.querySelector('form[aria-label="Draft task"] select')
            .disabled,
      )
      assert.equal(
        await card.getByLabel('Judul task', { exact: true }).inputValue(),
        draft.title,
      )
      assert.equal(
        await card.getByLabel('Deskripsi', { exact: true }).inputValue(),
        draft.description,
      )
      assert.equal(
        await card.getByLabel('Project', { exact: true }).inputValue(),
        'p1',
      )
      assert.equal(
        await card
          .getByRole('option', { name: 'Archived', exact: true })
          .count(),
        0,
      )
      assert.equal(
        await page
          .getByRole('button', { name: 'Buat / lihat task', exact: true })
          .count(),
        0,
      )
      assert.equal(writes.length, 0)
      assert(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      )
      await card.scrollIntoViewIfNeeded()
      await page.screenshot({
        path: out + '/draft-' + theme + '-' + width + '.png',
      })
      measurements.push({
        width,
        theme,
        editableDraft: true,
        noOverflow: true,
        noAutoCreate: true,
      })
    }
  let card = page.getByRole('form', { name: 'Draft task', exact: true })
  await card
    .getByLabel('Judul task', { exact: true })
    .fill('Judul yang sudah diedit')
  await card
    .getByLabel('Deskripsi', { exact: true })
    .fill('Scope diedit. Kriteria selesai: Back kembali ke Inbox.')
  await card.getByLabel('Project', { exact: true }).selectOption('p3')
  saveFail = true
  await card.getByRole('button', { name: 'Buat task', exact: true }).click()
  await card.getByRole('alert').waitFor()
  assert.equal(created, null)
  assert.equal(
    await card.getByLabel('Judul task', { exact: true }).inputValue(),
    'Judul yang sudah diedit',
  )
  saveFail = false
  await card.getByRole('button', { name: 'Buat task', exact: true }).click()
  await page
    .getByRole('link', {
      name: 'Buka task: Judul yang sudah diedit',
      exact: true,
    })
    .waitFor()
  assert.equal(created.project_id, 'p3')
  assert.equal(
    created.description,
    'Scope diedit. Kriteria selesai: Back kembali ke Inbox.',
  )
  assert.equal(created.status, 'backlog')
  await page.reload()
  await page
    .getByRole('link', {
      name: 'Buka task: Judul yang sudah diedit',
      exact: true,
    })
    .waitFor()
  assert.equal(
    await page.getByRole('button', { name: 'Buat task', exact: true }).count(),
    0,
  )
  assert.equal(writes.length, 2)
  await page.screenshot({ path: out + '/saved-360.png' })
  created = null
  loadFail = true
  await page.reload()
  card = page.getByRole('form', { name: 'Draft task', exact: true })
  await card.getByRole('alert').waitFor()
  assert.equal(
    await card
      .getByRole('button', { name: 'Buat task', exact: true })
      .isEnabled(),
    false,
  )
  loadFail = false
  await card.getByRole('button', { name: 'Coba lagi', exact: true }).click()
  await page.waitForFunction(
    () =>
      !document.querySelector('form[aria-label="Draft task"] select').disabled,
  )
  showDraft = false
  await page.reload()
  await page
    .getByText('Halo, ada yang bisa saya bantu?', { exact: true })
    .waitFor()
  assert.equal(
    await page.getByRole('form', { name: 'Draft task', exact: true }).count(),
    0,
  )
  assert.equal(
    await page
      .getByRole('button', { name: 'Buat / lihat task', exact: true })
      .count(),
    0,
  )
  assert.equal(writes.length, 2)
  assert.deepEqual(errors, [])
  assert.deepEqual(unknown, [])
  fs.writeFileSync(
    out + '/browser.json',
    JSON.stringify(
      {
        fixture: true,
        exactProductionBuild: true,
        measurements,
        checks: [
          'no task buttons on ordinary messages',
          'editable title/description/project',
          'archived project excluded',
          'no writes before confirmation',
          'save failure preserves edits',
          'confirmed task in backlog',
          'reload links existing task',
          'load failure retry',
          'no execution calls',
        ],
        writes,
        errors,
        unknown,
        providerInvocations: 0,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(
    'PASS chat task draft:8 viewport/theme combinations, confirmation/retry/reload, no auto create or execute',
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
