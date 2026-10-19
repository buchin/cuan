// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/context-new',
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


 const checks=[]
 const routes=[['inbox','New message','New message'],['contacts','New contact','New contact'],['tasks','New task','New task'],['projects','New project','New project'],['settings','New','New'],['settings/agents','New agent','Tambah agen'],['projects/p1','New task','New task'],['inbox/room0','New message','New message']]
 for(const width of [1440,820,390,360])for(const theme of ['light','dark']){
  await page.setViewportSize({width,height:900});await page.goto(base+'/settings/appearance');await page.getByRole('button',{name:theme==='light'?'Light':'Dark',exact:true}).click()
  for(const [route,action,title] of routes){
   await page.goto(base+'/'+route);if(route==='inbox/room0')await page.getByRole('textbox',{name:'Pesan',exact:true}).waitFor();else await page.locator('.app-page-header').getByRole('heading',{level:1}).waitFor()
   const button=page.getByRole('button',{name:action,exact:true});await button.waitFor();assert.equal(await button.count(),1);const b=await button.boundingBox();assert(b.x+b.width/2>width/2&&b.height>=44);assert.equal((await button.innerText()).trim(),action)
   assert.equal(await page.getByRole('navigation',{name:'Navigasi utama',exact:true}).getByRole('button',{name:'New project',exact:true}).count(),0)
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));if(width===390&&theme==='dark')await page.screenshot({path:out+'/page-'+route.replaceAll('/','-')+'-390.png'})
   await button.click();const dialog=page.getByRole('dialog',{name:title,exact:true});await dialog.waitFor();assert(await dialog.evaluate(e=>e.scrollWidth<=e.clientWidth))
   if(route==='projects/p1')assert.equal(await dialog.getByLabel('Project',{exact:true}).inputValue(),'p1')
   if(width===390&&theme==='dark')await page.screenshot({path:out+'/dialog-'+route.replaceAll('/','-')+'-390.png'})
   for(let i=0;i<10;i++){await page.keyboard.press('Tab');assert(await dialog.evaluate(e=>e.contains(document.activeElement)))}
   await page.keyboard.press('Escape');await dialog.waitFor({state:'detached'});checks.push({width,theme,route,action,oneNew:true,visibleContextLabel:true,dialog:true,noOverflow:true})
  }
 }
 assert.equal(writes.length,0)
 await page.setViewportSize({width:390,height:900});await page.goto(base+'/projects');await page.getByRole('button',{name:'Menu akun',exact:true}).click();let menu=page.getByRole('region',{name:'Menu akun',exact:true});await menu.getByRole('button',{name:'Settings',exact:true}).waitFor();await page.keyboard.press('Escape');await menu.waitFor({state:'detached'});assert(await page.getByRole('button',{name:'Menu akun',exact:true}).evaluate(e=>e===document.activeElement));await page.getByRole('button',{name:'Menu akun',exact:true}).click();await page.getByRole('region',{name:'Menu akun',exact:true}).getByRole('button',{name:'Settings',exact:true}).click();await page.waitForURL('**/settings')
 await page.goto(base+'/inbox/room0');await page.getByRole('textbox',{name:'Pesan',exact:true}).fill('Draft for verified only');await page.getByRole('button',{name:'New message',exact:true}).click();let chooser=page.getByRole('dialog',{name:'New message',exact:true});await chooser.getByRole('textbox',{name:'Cari penerima',exact:true}).fill('@expired');await chooser.getByRole('button').filter({has:page.getByText('@expired',{exact:true})}).click();await page.waitForURL('**/inbox/room1');await page.waitForFunction(()=>document.querySelector('textarea[aria-label="Pesan"]')?.value==='');await page.getByRole('button',{name:'New message',exact:true}).click();chooser=page.getByRole('dialog',{name:'New message',exact:true});await chooser.getByRole('button').filter({has:page.getByText('@verified',{exact:true})}).click();await page.waitForURL('**/inbox/room0');await page.waitForFunction(()=>document.querySelector('textarea[aria-label="Pesan"]')?.value==='Draft for verified only');assert.equal(writes.length,0)
 await page.goto(base+'/contacts');await page.getByRole('button',{name:'New contact',exact:true}).click();let form=page.getByRole('dialog',{name:'New contact',exact:true});await form.getByLabel('Nama',{exact:true}).fill('Manual contact fixture');await form.getByLabel('Email (opsional)',{exact:true}).fill('manual@example.invalid');await form.getByRole('button',{name:'Simpan kontak',exact:true}).click();await form.getByRole('alert').waitFor();assert.equal(await form.getByLabel('Nama',{exact:true}).inputValue(),'Manual contact fixture');await form.getByRole('button',{name:'Simpan kontak',exact:true}).click();await form.waitFor({state:'detached'});assert.equal(manualRequests.length,2);assert.equal(manualRequests[0].request_id,manualRequests[1].request_id);await page.waitForURL('**/contacts?contact='+manualRequests[0].request_id);const detail=page.getByRole('complementary',{name:'Detail kontak',exact:true});await detail.getByText(/Kontak manual belum terhubung/).waitFor();assert.equal(await detail.getByRole('link',{name:'Buka percakapan',exact:true}).count(),0);await page.screenshot({path:out+'/manual-created-390.png'});assert.equal(crm.filter(c=>c.is_manual).length,1)
 await page.goto(base+'/inbox');await page.getByRole('button',{name:'New message',exact:true}).click();chooser=page.getByRole('dialog',{name:'New message',exact:true});await chooser.getByRole('textbox',{name:'Cari penerima',exact:true}).fill('Manual contact fixture');await chooser.getByText('Penerima tidak ditemukan.',{exact:true}).waitFor();await page.keyboard.press('Escape')
 assert.deepEqual(errors,[]);assert.deepEqual(unknown,[]);assert.equal(writes.length,2);assert(writes.every(w=>w.method==='POST'&&w.path==='/api/v1/contacts'))
 fs.writeFileSync(out+'/browser.json',JSON.stringify({fixture:true,exactProductionBuild:true,checks,workflows:['one contextual New in header','no sidebar or body duplicates','all create dialogs focus/keyboard/44px targets','project-scoped new task','avatar Settings mobile and Escape','recipient chooser sends nothing','draft isolation between recipients','manual create lost response retry same ID','manual contact no messaging channel or recipient entry'],errors,unknown,writes,providerInvocations:0},null,2)+'\n');console.log('PASS context New:64 viewport/theme/route checks, avatar Settings, recipient draft isolation and manual contact retry')
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))})
