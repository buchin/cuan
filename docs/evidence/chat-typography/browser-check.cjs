// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/chat-typography',
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
let manualResponseFail=true
const manualRequests=[]
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
      if(method==='POST' && p==='/api/v1/contacts'){
        const data=r.postDataJSON();manualRequests.push(data)
        let c=crm.find(c=>c.id===data.request_id)
        if(!c){c={id:data.request_id,name:data.name,email:data.email,product:'Manual',conversation_id:'manual_'+data.request_id,tags:[],updated_at:now,is_manual:true};crm.push(c)}
        if(manualResponseFail){manualResponseFail=false;return json({error:'Fixture response lost; retry safely'},503)}
        return json({contact:c})
      }

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



const checks=[];
for(const width of [360,390,768,820,1440])for(const theme of ['light','dark']){
 await page.setViewportSize({width,height:900});await page.goto(base+'/settings/appearance');await page.getByRole('button',{name:theme==='light'?'Light':'Dark',exact:true}).click();await page.goto(base+'/inbox/room0');const md=page.locator('.chat-markdown').first();await md.waitFor();const actual=await md.evaluate(e=>({font:getComputedStyle(e).fontSize,line:getComputedStyle(e).lineHeight}));assert.equal(actual.font,width<768?'16px':'15px');assert.equal(parseFloat(actual.line),parseFloat(actual.font)*1.55);const plain=page.locator('.wa-plain-text').first();assert.equal(await plain.evaluate(e=>getComputedStyle(e).fontSize),actual.font);await md.evaluate(e=>{e.innerHTML='<h1>Hasil pembahasan</h1><p>Ringkasan yang tetap nyaman dibaca.</p><h2>Rekomendasi</h2><ul><li>Prioritas pertama<ul><li>Rincian lanjutan</li></ul></li><li>Prioritas kedua</li></ul><h3>Langkah berikutnya</h3><p>'+('Pesan panjang untuk memastikan pembungkusan teks. '.repeat(20))+'</p><pre><code>'+('long_code_'.repeat(40))+'</code></pre>'});for(const [tag,size] of [['h1','18px'],['h2','17px'],['h3','16px']])assert.equal(await md.locator(tag).evaluate(e=>getComputedStyle(e).fontSize),size);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));assert.equal(await page.getByRole('textbox',{name:'Pesan',exact:true}).evaluate(e=>getComputedStyle(e).fontSize),'16px');await page.screenshot({path:out+'/chat-'+theme+'-'+width+'.png'});checks.push({width,theme,...actual,headingSizes:[18,17,16],noPageOverflow:true,composer16:true});
}
assert.deepEqual(errors,[]);assert.equal(writes.length,0);fs.writeFileSync(out+'/browser.json',JSON.stringify({checks,fixture:true,productWrites:0,providerInvocations:0},null,2)+'\n');console.log('PASS typography:10 viewport/theme combinations, body/headings/composer sizes and long content overflow');
})().catch(e=>{console.error(e.message);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))})
