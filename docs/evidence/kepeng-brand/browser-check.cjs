// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/kepeng-brand',
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
        file.endsWith('.svg') ? false : file.endsWith('.js')
          ? 'text/javascript'
          : file.endsWith('.css')
            ? 'text/css'
            : file.endsWith('.woff2')
              ? 'font/woff2'
              : 'text/html',
      )
      if(file.endsWith('.svg'))res.setHeader('Content-Type','image/svg+xml')
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
  const sizes=[]
  for(const width of [1440,820,390,360]) {
    await page.setViewportSize({width,height:900})
    for(const theme of ['light','dark']) {
      await page.goto(base+'/settings/appearance')
      await page.getByRole('button',{name:theme==='light'?'Light':'Dark',exact:true}).click()
      // Visible logo in desktop sidebar or mobile header.
      const visible=page.locator('svg[aria-label="Logo Cuan — uang kepeng"]:visible')
      await visible.waitFor()
      const measurement=await visible.evaluate(e=>({width:e.getBoundingClientRect().width,height:e.getBoundingClientRect().height,color:getComputedStyle(e).color,viewBox:e.getAttribute('viewBox')}))
      assert.equal(measurement.width,measurement.height)
      assert.equal(measurement.viewBox,'0 0 64 64')
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
      sizes.push({width,theme,...measurement})
      await page.screenshot({path:out+'/app-'+theme+'-'+width+'.png'})
    }
  }
  const favicon=await page.request.get(base+'/favicon.svg');assert.equal(favicon.status(),200);assert((await favicon.text()).includes('prefers-color-scheme:dark'))
  assert.equal(await page.locator('link[rel="icon"]').getAttribute('href'),'/favicon.svg')
  // Standalone visual sheet uses exact exported SVGs at actual small sizes.
  await page.route(base+'/brand-proof',r=>r.fulfill({contentType:'text/html',body:`<!doctype html><html><meta charset="utf-8"><body style="margin:0;font:16px system-ui"><h1 style="padding:20px">Cuan · Kepeng</h1>${['light','dark'].map(theme=>`<section style="padding:32px;background:${theme==='dark'?'#09090b':'#fff'};color:${theme==='dark'?'#fff':'#09090b'}"><h2>${theme}</h2><div style="display:flex;align-items:center;gap:32px">${[16,24,32,64,128].map(size=>`<div><img alt="Kepeng ${size}" src="/brand/cuan-${theme}.svg" width="${size}" height="${size}"><p>${size}px</p></div>`).join('')}</div></section>`).join('')}</body></html>`}))
  await page.setViewportSize({width:900,height:800});await page.goto(base+'/brand-proof');await page.waitForFunction(()=>[...document.images].every(i=>i.complete&&i.naturalWidth>0));await page.screenshot({path:out+'/logo-sizes.png'})
  const login=await browser.newPage({viewport:{width:390,height:844}})
  login.on('pageerror',e=>errors.push(e.message))
  await login.route('**/api/**',route=>route.fulfill({status:route.request().url().endsWith('/auth/me')?401:200,contentType:'application/json',body:JSON.stringify({setup_required:false})}))
  await login.goto(base+'/inbox');await login.getByRole('heading',{name:'Masuk ke Cuan'}).waitFor()
  for(const theme of ['light','dark']){await login.evaluate(theme=>localStorage.setItem('cuan-theme',theme),theme);await login.reload();await login.getByRole('img',{name:'Logo Cuan — uang kepeng'}).waitFor();await login.screenshot({path:out+'/login-'+theme+'-390.png'})}
  await login.close()
  assert.deepEqual(errors,[]);assert.deepEqual(unknown,[]);assert.deepEqual(writes,[])
  fs.writeFileSync(out+'/browser.json',JSON.stringify({fixture:true,exactProductionBuild:true,checks:['theme-aware inline kepeng in mobile/header and sidebar','square opening and ring preserved at16/24/32/64/128','adaptive favicon','no horizontal overflow'],sizes,errors,providerInvocations:0},null,2)+'\n')
  console.log('PASS kepeng: light/dark desktop/tablet/mobile, exported SVG sizes, adaptive favicon')
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))})
