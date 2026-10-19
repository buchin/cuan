// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/tasks-usability',
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
    tags: ['VIP', "Customers", 'Aktif'],
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
  requests: [{request_id:'fixture-request',message_id:'m2',agent_id:'agent_good',state:'failed',created_at:now,started_at:now,safe_error:'The provider could not reply. Your message is saved.',steps:[{seq:1,kind:'accepted',text:'Message received.',at:now},{seq:2,kind:'runtime_tool_failed',text:'Tool #2 · Read file failed.',at:now},{seq:3,kind:'provider_result',text:'Provider process finished: exit 1.',at:now}]}],
  tasks: [],
})
// Stateful isolated task API. Backend/PTY invariants are separately exercised by Go tests.
tasks[0]={...tasks[0],title:'Improve task workflow and keep long titles readable on every screen',instructions:'## Human instructions\n\nKeep the original requirements.\n\n- Verify mobile and desktop\n- Preserve saved evidence\n\n```go\npackage main\n```',description:undefined,status:'backlog',summary:'',complete_note:'',dod_checklist:[{text:'Verify the result',completed:false}],attempts:[],activities:[],assignee_id:'agent_good'}
tasks[1]={...tasks[1],title:'Review the saved result',status:'review',summary:'A completed implementation.',complete_note:'',dod_checklist:[],attempts:[{id:'old_attempt',task_id:'task_two',agent_name:'agent_good',runner_id:'local_terminal',attempt_number:1,status:'succeeded',created_at:now,started_at:now,finished_at:now,exit_code:0}],activities:[]};tasks[1].latest_attempt=tasks[1].attempts[0]
let failSave=false,failLaunch=false,failCreate=false,listError=false,version=0
const launches=[],createRequests=[],runKeys=new Map(),createdKeys=new Map(),terminalConnections=[]
const bump=t=>{t.updated_at=new Date(Date.parse(now)+(++version)*1000).toISOString()}
const logRows=Array.from({length:280},(_,i)=>({id:i+1,stream:'stdout',line:i===41?'verification-needle:PASS':`Fixture output line ${i+1}`,logged_at:now}))
async function workspaceAPI(route,json,u,method){
 const p=u.pathname,r=route.request(),body=method==='GET'||method==='DELETE'?{}:r.postDataJSON()||{}
 if(method!=='GET')writes.push({method,path:p})
 if(p==='/api/v1/tasks'){
  if(method==='GET'){await json(listError?{error:'Fixture task list unavailable'}:{tasks},listError?503:200);return true}
  if(method==='POST'){createRequests.push(body);let t=createdKeys.get(body.request_id);if(!t){t={id:createdKeys.size?'created_task_'+createdKeys.size:'created_task',title:body.title,instructions:body.instructions,priority:body.priority,status:body.status||'backlog',project_id:body.project_ids?.[0]||'',project_ids:body.project_ids||[],summary:'',complete_note:'',dod_checklist:body.dod_checklist||[],created_at:now,updated_at:now,attempts:[],activities:[]};tasks.push(t);createdKeys.set(body.request_id,t)}if(failCreate){failCreate=false;await json({error:'Fixture create response lost'},503)}else await json(t,201);return true}
 }
 const match=p.match(/^\/api\/v1\/tasks\/([^/]+)(.*)$/);if(!match)return false
 const t=tasks.find(t=>t.id===match[1]),tail=match[2];if(!t){await json({error:'Task not found'},404);return true}
 if(!tail&&method==='GET'){await json({...t,attempts:t.attempts||[],activities:t.activities||[]});return true}
 if(!tail&&method==='PUT'){if(failSave){await json({error:'Fixture save failed; draft is retained'},503);return true}if(body.expected_updated_at&&body.expected_updated_at!==t.updated_at){await json({error:'Task changed. Reload the saved task'},409);return true}if(body.status==='done'&&(!(body.complete_note||t.complete_note)?.trim()||(body.dod_checklist||t.dod_checklist||[]).some(i=>!i.completed))){await json({error:'Verification is required'},400);return true}for(const k of ['title','instructions','status','priority','summary','complete_note','dod_checklist','project_ids'])if(body[k]!==undefined)t[k]=body[k];if(body.project_ids)t.project_id=body.project_ids[0]||'';if(body.assigned_agent!==undefined)t.assignee_id=body.assigned_agent;bump(t);await json(t);return true}
 if(!tail&&method==='DELETE'){tasks.splice(tasks.indexOf(t),1);await json({id:t.id});return true}
 if(tail==='/comments'&&method==='POST'){const a={id:'comment_'+version,activity_type:'comment',author:'Owner',message:body.message,created_at:now};t.activities.push(a);await json(a,201);return true}
 if(tail==='/run-agent/history'){const a=t.latest_attempt;const live=a&&['running','queued'].includes(a.status)?{session_id:'live_'+a.id,attempt_id:a.id,agent_id:a.agent_name,started_at:a.created_at}:null;await json({history:(t.attempts||[]).map(a=>({attempt_id:a.id,session_id:'live_'+a.id,status:a.status})),live_session:live});return true}
 if(tail.includes('/logs')){await json({logs:logRows.filter(l=>l.id>Number(u.searchParams.get('after_id')||0)).slice(0,250)});return true}
 if(tail==='/run-agent'&&method==='POST'){launches.push(body);const replay=runKeys.get(body.request_id);if(replay){await json({status:replay.status,attempt:replay,replayed:true,session_id:'live_'+replay.id});return true}const a={id:'attempt_'+version,task_id:t.id,runner_id:'local_terminal',agent_name:body.agent_id,status:'running',attempt_number:(t.attempts?.length||0)+1,created_at:now,started_at:now};t.attempts=[a,...t.attempts||[]];t.latest_attempt=a;t.status='doing';bump(t);runKeys.set(body.request_id,a);if(body.revision)t.activities.push({id:'revision_'+version,activity_type:'comment',author:'Owner',message:'Revision requested:\n'+body.revision,created_at:now});if(failLaunch){failLaunch=false;await json({error:'Fixture launch response lost'},503)}else await json({status:'running',attempt:a,session_id:'live_'+a.id});return true}
 if(tail==='/cancel'&&method==='POST'){t.latest_attempt.status='canceled';t.latest_attempt.exit_code=130;t.latest_attempt.finished_at=now;t.status='blocked';bump(t);await json(t.latest_attempt);return true}
 return false
}
let browser, server, page
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
  page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  page.setDefaultTimeout(6000)
  page.on('pageerror', (e) => errors.push(e.message))
  await page.routeWebSocket(/\/api\/v1\/tasks\/[^/]+\/terminal/, ws=>{const url=new URL(ws.url());assert(!url.searchParams.has('token'));assert.equal(url.searchParams.get('attach'),'1');terminalConnections.push(url.searchParams.get('session_id'));ws.send(JSON.stringify({type:'connected',session_id:url.searchParams.get('session_id'),mode:'watch'}));ws.send(Buffer.from('Live fixture output\r\nverification-needle:PASS\r\n'));ws.onMessage(message=>{const d=JSON.parse(String(message));if(d.type==='mode')ws.send(JSON.stringify({type:'status',mode:d.mode}));if(d.type==='stdin')ws.send(Buffer.from('ACK:'+d.data))})})
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
    if(p.startsWith('/api/v1/tasks')){const handled=await workspaceAPI(route,json,u,method);if(handled)return}
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
 const first=tasks[0],second=tasks[1]
 const openFirst=async()=>{await page.getByText(first.title,{exact:true}).locator('visible=true').first().click();await page.getByRole('dialog',{name:'Task detail',exact:true}).waitFor();await page.getByRole('button',{name:'Edit',exact:true}).waitFor({state:'visible'});await page.waitForFunction(()=>!document.querySelector('dialog[aria-label="Task detail"] button[disabled] svg.lucide-pencil'))}
 for(const width of [360,390,820,1440])for(const theme of ['light','dark']){
  await page.setViewportSize({width,height:900});await page.goto(base+'/settings/appearance');await page.getByRole('button',{name:theme==='light'?'Light':'Dark',exact:true}).click();await page.goto(base+'/tasks');await page.getByText(first.title,{exact:true}).locator('visible=true').first().waitFor();assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:out+'/list-'+theme+'-'+width+'.png'});if(width<640){assert((await page.getByRole('textbox',{name:'Search tasks',exact:true}).boundingBox()).width>=200);await page.getByRole('button',{name:'Sort tasks',exact:true}).click();await page.getByRole('radio',{name:'Highest priority',exact:true}).click();assert(page.url().includes('sort=priority'));await page.getByRole('combobox',{name:'Task status filter',exact:true}).selectOption('review');await page.getByText(second.title,{exact:true}).locator('visible=true').first().waitFor();await page.getByRole('combobox',{name:'Task status filter',exact:true}).selectOption('all');}await page.getByRole('button',{name:'New task',exact:true}).click();let composer=page.getByRole('dialog',{name:'What should we work on?',exact:true});assert.equal(await composer.locator('textarea').count(),1);assert.equal(await composer.getByRole('textbox',{name:'Task title',exact:true}).count(),0);assert.equal(await composer.getByRole('combobox',{name:'Project',exact:true}).inputValue(),'');assert(await composer.getByRole('button',{name:'Create task',exact:true}).isDisabled());await page.screenshot({path:out+'/create-'+theme+'-'+width+'.png'});await page.keyboard.press('Escape');await openFirst();const d=page.getByRole('dialog',{name:'Task detail',exact:true});await d.getByRole('heading',{name:first.title,exact:true}).waitFor();assert.equal(await d.locator('textarea').count(),0);assert(await d.evaluate(e=>e.scrollWidth<=e.clientWidth));await d.getByRole('heading',{name:'Human instructions',exact:true,level:3}).waitFor();assert.equal(await d.getByRole('heading',{name:first.title,exact:true}).evaluate(e=>getComputedStyle(e).whiteSpace),'normal');await page.screenshot({path:out+'/detail-'+theme+'-'+width+'.png'});for(let n=0;n<12;n++){await page.keyboard.press('Tab');assert(await d.evaluate(e=>e.contains(document.activeElement)))}await page.keyboard.press('Escape');await d.waitFor({state:'detached'});checks.push({width,theme,readFirst:true,wrappedTitle:true,noOverflow:true,focusTrap:true})
 }
 assert.equal(launches.length,0);assert.equal(terminalConnections.length,0)
 // Project-scoped creation defaults and unsaved composer protection.
 await page.goto(base+'/projects/p1');await page.getByRole('button',{name:'New task',exact:true}).click();let projectComposer=page.getByRole('dialog',{name:'What should we work on?',exact:true});assert.equal(await projectComposer.getByRole('combobox',{name:'Project',exact:true}).inputValue(),'p1');await projectComposer.getByRole('textbox',{name:'Task instructions',exact:true}).fill('Keep this unsaved prompt');page.once('dialog',d=>d.dismiss());await page.keyboard.press('Escape');assert.equal(await projectComposer.getByRole('textbox',{name:'Task instructions',exact:true}).inputValue(),'Keep this unsaved prompt');page.once('dialog',d=>d.accept());await page.keyboard.press('Escape');await projectComposer.waitFor({state:'detached'});
 // Inline edits never open task details or launch execution; failures remain visible.
 await page.setViewportSize({width:1440,height:900});await page.goto(base+'/tasks');let rowStatus=page.getByRole('combobox',{name:'Change status for '+first.title,exact:true});failSave=true;await rowStatus.selectOption('todo');await page.getByRole('alert').filter({hasText:'Fixture save failed'}).waitFor();assert.equal(first.status,'backlog');assert.equal(await page.getByRole('dialog').count(),0);await page.getByRole('button',{name:'Dismiss',exact:true}).click();failSave=false;await rowStatus.selectOption('todo');await page.waitForFunction(()=>document.querySelector('select[aria-label^="Change status for Improve"]')?.value==='todo');assert.equal(first.status,'todo');await rowStatus.selectOption('backlog');await page.waitForFunction(()=>document.querySelector('select[aria-label^="Change status for Improve"]')?.value==='backlog');let assignment=page.getByRole('combobox',{name:'Change assignee for '+first.title,exact:true});await assignment.selectOption('');await page.waitForFunction(()=>document.querySelector('select[aria-label^="Change assignee for Improve"]')?.value==='');await assignment.selectOption('agent_good');await page.waitForFunction(()=>document.querySelector('select[aria-label^="Change assignee for Improve"]')?.value==='agent_good');assert.equal(launches.length,0);
 await page.getByRole('button',{name:'Filters',exact:true}).click();await page.getByRole('combobox',{name:'Filter priority',exact:true}).selectOption('normal');await page.getByRole('button',{name:/^Show .*tasks?$/}).click();await page.getByRole('button',{name:'Remove Priority: normal filter',exact:true}).click();assert(!page.url().includes('priority='));

 // URL filters, sorting, reload, and return to the same view.
 await page.setViewportSize({width:1440,height:900});await page.goto(base+'/tasks');await page.getByRole('textbox',{name:'Search tasks',exact:true}).fill('Improve task workflow');await page.getByRole('combobox',{name:'Sort tasks',exact:true}).selectOption('priority');assert(page.url().includes('q=Improve'));await page.reload();assert.equal(await page.getByRole('textbox',{name:'Search tasks',exact:true}).inputValue(),'Improve task workflow');await openFirst();await page.getByRole('button',{name:'Expand task view',exact:true}).click();assert(page.url().includes('view=full'));await page.getByRole('button',{name:'Collapse task view',exact:true}).waitFor();assert.equal(Math.round((await page.getByRole('dialog',{name:'Task detail',exact:true}).boundingBox()).width),1440);await page.getByRole('button',{name:'Close task details',exact:true}).click();await page.waitForURL(new RegExp('/tasks[?]'));assert(page.url().includes('sort=priority'));assert(!page.url().includes('view=full'))
 await page.getByRole('button',{name:'Filters',exact:true}).click();await page.getByRole('combobox',{name:'Filter priority',exact:true}).selectOption('urgent');await page.getByRole('button',{name:'Show 0 tasks',exact:true}).click();await page.getByText('No matching tasks',{exact:true}).waitFor();await page.getByRole('button',{name:'Clear filters',exact:true}).first().click();await openFirst()
 // Retain failed edits, protect unsaved close, and persist independent fields.
 await page.getByRole('button',{name:'Edit',exact:true}).click();await page.getByRole('textbox',{name:'Work summary',exact:true}).fill('Implemented a readable task workspace.');await page.getByRole('textbox',{name:'Completion notes',exact:true}).fill('Verified the fixture at360px and1440px.');failSave=true;await page.getByRole('button',{name:'Save changes',exact:true}).click();await page.getByRole('alert').filter({hasText:'Fixture save failed'}).waitFor();assert.equal(await page.getByRole('textbox',{name:'Work summary',exact:true}).inputValue(),'Implemented a readable task workspace.');page.once('dialog',d=>d.dismiss());await page.getByRole('button',{name:'Close task details',exact:true}).click();await page.getByRole('textbox',{name:'Work summary',exact:true}).waitFor();failSave=false;await page.getByRole('button',{name:'Save changes',exact:true}).click();await page.getByText('Task changes saved.',{exact:true}).waitFor();assert(first.instructions.includes('Keep the original requirements.'));assert.equal(first.summary,'Implemented a readable task workspace.')
 // Unknown launch outcome: same durable key after reload, one session.
 await page.getByRole('button',{name:'Start',exact:true}).click();let modal=page.getByRole('dialog',{name:'Start task',exact:true});await modal.getByRole('combobox',{name:'Execution agent',exact:true}).selectOption('agent_good');failLaunch=true;await modal.getByRole('button',{name:'Start session',exact:true}).click();await page.getByRole('button',{name:'Check launch request',exact:true}).waitFor();assert.equal(first.attempts.length,1);const originalRequest=launches[0].request_id;await page.reload();await page.getByRole('button',{name:'Check launch request',exact:true}).click();await page.getByRole('region',{name:'Session terminal',exact:true}).waitFor();await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(launches.length,2);assert.equal(launches[1].request_id,originalRequest);assert.equal(first.attempts.length,1)
 // Input is explicitly enabled; reconnect only opens the same process.
 await page.getByRole('button',{name:'Enable input',exact:true}).click();await page.getByText('Connected · input enabled',{exact:true}).waitFor();const before=launches.length;await page.getByRole('button',{name:'Reconnect terminal',exact:true}).click();await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(launches.length,before);assert(terminalConnections.every(id=>id==='live_'+first.latest_attempt.id));await page.getByRole('textbox',{name:'Search terminal output',exact:true}).fill('verification-needle');await page.getByText('1 of 1',{exact:true}).waitFor();await page.getByRole('button',{name:'Copy output',exact:true}).click();await page.getByText('Output copied.',{exact:true}).waitFor();const download=page.waitForEvent('download');await page.getByRole('button',{name:'Download output',exact:true}).click();const file=await download;await file.saveAs(out+'/fixture-output.txt');assert(fs.readFileSync(out+'/fixture-output.txt','utf8').includes('verification-needle'))
 await page.screenshot({path:out+'/live-terminal-desktop.png'});await page.setViewportSize({width:390,height:900});assert(await page.getByRole('dialog',{name:'Task detail',exact:true}).evaluate(e=>e.scrollWidth<=e.clientWidth));await page.screenshot({path:out+'/live-terminal-mobile.png'})
 // Cancellation is confirmed, retains output and exposes an explicit retry.
 await page.getByRole('button',{name:'Cancel session',exact:true}).click();modal=page.getByRole('dialog',{name:'Cancel session?',exact:true});await modal.getByRole('button',{name:'Cancel session',exact:true}).click();await page.getByRole('button',{name:'Retry',exact:true}).waitFor();await page.getByText('Saved output',{exact:true}).waitFor();await page.getByRole('textbox',{name:'Search terminal output',exact:true}).fill('Fixture output line 280');await page.getByText('1 of 1',{exact:true}).waitFor();assert.equal(first.latest_attempt.status,'canceled');await page.getByRole('button',{name:'Close task details',exact:true}).click()
 // Review requires evidence; rejection starts a revision with saved feedback.
 await page.getByText(second.title,{exact:true}).locator('visible=true').first().click();await page.getByRole('button',{name:'Approve result',exact:true}).click();modal=page.getByRole('dialog',{name:'Approve task result',exact:true});assert(await modal.getByRole('button',{name:'Approve and mark Done',exact:true}).isDisabled());await page.keyboard.press('Escape');await page.getByRole('button',{name:'Request changes',exact:true}).click();modal=page.getByRole('dialog',{name:'Request changes',exact:true});await modal.getByRole('combobox',{name:'Execution agent',exact:true}).selectOption('agent_good');assert(await modal.getByRole('button',{name:'Start session',exact:true}).isDisabled());await modal.getByRole('textbox',{name:'Revision feedback',exact:true}).fill('Fix the remaining mobile issue and verify on a390px viewport.');await modal.getByRole('button',{name:'Start session',exact:true}).click();assert.equal(second.attempts.length,2);assert(second.activities.some(a=>a.message.includes('remaining mobile issue')));second.latest_attempt.status='succeeded';second.latest_attempt.exit_code=0;second.latest_attempt.finished_at=now;second.status='review';bump(second);await page.getByRole('button',{name:'Reconnect terminal',exact:true}).click();await page.getByRole('button',{name:'Approve result',exact:true}).waitFor();await page.getByRole('button',{name:'Overview',exact:true}).click();await page.getByRole('button',{name:'Approve result',exact:true}).click();modal=page.getByRole('dialog',{name:'Approve task result',exact:true});await modal.getByRole('textbox',{name:'Approval evidence',exact:true}).fill('Verified the output and mobile layout; no remaining issue.');await modal.getByRole('button',{name:'Approve and mark Done',exact:true}).click();await page.getByRole('button',{name:'Reopen task',exact:true}).waitFor();assert.equal(second.status,'done')
 // Activity comments survive reload; deletion requires explicit confirmation.
 await page.getByRole('button',{name:'Activity',exact:true}).click();await page.getByRole('textbox',{name:'Add a comment',exact:true}).fill('Review evidence recorded.');await page.getByRole('button',{name:'Send comment',exact:true}).click();await page.getByText('Review evidence recorded.',{exact:true}).waitFor();await page.reload();await page.getByRole('button',{name:'Activity',exact:true}).click();await page.getByText('Review evidence recorded.',{exact:true}).waitFor();await page.getByRole('button',{name:'Task actions',exact:true}).click();await page.getByRole('button',{name:'Delete task',exact:true}).click();modal=page.getByRole('dialog',{name:'Delete task?',exact:true});const deletes=writes.filter(w=>w.method==='DELETE').length;await modal.getByRole('button',{name:'Keep task',exact:true}).click();assert.equal(writes.filter(w=>w.method==='DELETE').length,deletes);await page.getByRole('button',{name:'Task actions',exact:true}).click();await page.getByRole('button',{name:'Delete task',exact:true}).click();await page.getByRole('dialog',{name:'Delete task?',exact:true}).getByRole('button',{name:'Delete task',exact:true}).click();await page.waitForURL(u=>u.pathname==='/tasks');assert(!tasks.some(t=>t.id===second.id))
 // Create retries retain the same request, then open the persisted task.
 await page.getByRole('button',{name:'New task',exact:true}).click();modal=page.getByRole('dialog',{name:'What should we work on?',exact:true});assert.equal(await modal.getByRole('textbox',{name:'Task title',exact:true}).count(),0);await modal.getByRole('textbox',{name:'Task instructions',exact:true}).fill('Create with acceptance criteria\nKeep the full prompt as human instructions.');failCreate=true;await modal.getByRole('button',{name:'Create task',exact:true}).click();await modal.getByRole('alert').waitFor();await page.reload();await page.getByRole('button',{name:'New task',exact:true}).click();modal=page.getByRole('dialog',{name:'What should we work on?',exact:true});assert((await modal.getByRole('textbox',{name:'Task instructions',exact:true}).inputValue()).startsWith('Create with acceptance criteria'));await modal.getByRole('button',{name:'Create task',exact:true}).click();await page.waitForURL(u=>u.pathname==='/tasks/created_task');await page.getByRole('heading',{name:'Create with acceptance criteria',exact:true}).waitFor();assert.equal(createRequests.length,2);assert.equal(createRequests[0].request_id,createRequests[1].request_id);assert.equal(tasks.filter(t=>t.id==='created_task').length,1);assert.equal(createdKeys.values().next().value.dod_checklist.length,0);assert.equal(createdKeys.values().next().value.instructions,'Create with acceptance criteria\nKeep the full prompt as human instructions.')

 // Explicit Create & start launches once; a lost response opens durable recovery.
 await page.getByRole('button',{name:'Close task details',exact:true}).click();await page.getByRole('button',{name:'New task',exact:true}).click();modal=page.getByRole('dialog',{name:'What should we work on?',exact:true});await modal.getByRole('textbox',{name:'Task instructions',exact:true}).fill('Delegate this task to the selected agent');await modal.getByRole('combobox',{name:'Agent',exact:true}).selectOption('agent_good');assert.equal((await modal.getByRole('button',{name:'Create task',exact:true}).innerText()).trim(),'Create & start');const priorLaunchCount=launches.length;failLaunch=true;await modal.getByRole('button',{name:'Create task',exact:true}).click();await page.getByRole('button',{name:'Check launch request',exact:true}).waitFor();const delegated=tasks.find(t=>t.title==='Delegate this task to the selected agent');assert.equal(delegated.attempts.length,1);assert.equal(launches.length,priorLaunchCount+1);const delegatedRequest=launches[launches.length-1].request_id;await page.reload();await page.getByRole('button',{name:'Check launch request',exact:true}).click();await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(delegated.attempts.length,1);assert.equal(launches[launches.length-1].request_id,delegatedRequest);
 assert.deepEqual(errors,[]);assert.deepEqual(unknown,[]);fs.writeFileSync(out+'/browser.json',JSON.stringify({fixture:true,exactProductionBuild:true,checks,workflows:['URL filters/sort/reload and return context','read-first responsive detail and focus trap','expanded task view','failed-save draft and unsaved-close protection','independent human/work/result fields','unknown launch recovery same ID after reload','one session and explicit input mode','attach-only reconnect','terminal follow/search/copy/download','saved output pagination','cancel and retained output','revision with concrete feedback','review approval evidence','durable comments','confirmed deletion','idempotent create from one prompt','simple prompt composer and first-line title','project-scoped default and discard protection','explicit create/start and uncertain launch recovery','inline status/assignee and save-error recovery','active-filter chip removal','mobile sort and visible status filter'],writes,launchRequests:launches.length,terminalConnections:terminalConnections.length,errors,unknown,providerInvocations:0},null,2)+'\n');console.log('PASS Tasks workspace fixture:8 responsive/theme combinations and complete editing, launch/recovery, terminal, revision/review, comments, delete and create workflows')
})().catch(async e=>{if(page)await page.screenshot({path:out+'/check-failure.png'});console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))})
