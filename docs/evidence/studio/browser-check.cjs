// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = process.env.CUAN_VISUAL_OUTPUT || '/home/dev/reports/cuan-studio-simple',
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
let inboxPending=true
const historyBase = (id) => ({
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
const history=id=>{if(id.startsWith("topic_"))return {conversation_id:id,room_id:id,contact:contacts.find(c=>c.conversation_id===id),messages:[],requests:[],tasks:[]};const h=historyBase(id);h.requests[0]={...h.requests[0],message_id:'m1',reply_id:inboxPending?'':'m2',state:inboxPending?'running':'completed'};if(inboxPending)h.messages=h.messages.slice(0,1);h.tasks=id==='room0'?[tasks.find(t=>t.id==='linked_work')].filter(Boolean):[];return h}
// Stateful isolated task API. Backend/PTY invariants are separately exercised by Go tests.
tasks[0]={...tasks[0],title:'Improve task workflow and keep long titles readable on every screen',instructions:'## Human instructions\n\nKeep the original requirements.\n\n- Verify mobile and desktop\n- Preserve saved evidence\n\n```go\npackage main\n```',description:undefined,status:'backlog',summary:'',complete_note:'',dod_checklist:[{text:'Verify the result',completed:false}],attempts:[],activities:[],assignee_id:'agent_good'}
tasks[1]={...tasks[1],title:'Review the saved result',status:'review',summary:'A completed implementation.',complete_note:'',dod_checklist:[],attempts:[{id:'old_attempt',task_id:'task_two',agent_name:'agent_good',runner_id:'local_terminal',attempt_number:1,status:'succeeded',created_at:now,started_at:now,finished_at:now,exit_code:0}],activities:[]};tasks[1].latest_attempt=tasks[1].attempts[0]
let failSave=false,failLaunch=false,failCreate=false,listError=false,version=0
const launches=[],createRequests=[],runKeys=new Map(),createdKeys=new Map(),terminalConnections=[],inputWrites=[]
const bump=t=>{t.updated_at=new Date(Date.parse(now)+(++version)*1000).toISOString()}
const logRows=Array.from({length:280},(_,i)=>({id:i+1,stream:'stdout',line:i===41?'verification-needle:PASS':`Fixture output line ${i+1}`,logged_at:now}))
async function workspaceAPI(route,json,u,method){
 const p=u.pathname,r=route.request(),body=method==='GET'||method==='DELETE'?{}:r.postDataJSON()||{}
 if(method!=='GET')writes.push({method,path:p})
 if(p==='/api/v1/tasks'){
  if(method==='GET'){await json(listError?{error:'Fixture task list unavailable'}:{tasks},listError?503:200);return true}
  if(method==='POST'){createRequests.push(body);let t=createdKeys.get(body.request_id);if(!t){t={id:createdKeys.size?'created_task_'+createdKeys.size:'created_task',assigned_agent:body.assigned_agent,title:body.title||body.instructions.trim().split('\n')[0],instructions:body.instructions,priority:body.priority,status:body.status||'backlog',project_id:body.project_ids?.[0]||'',project_ids:body.project_ids||[],summary:'',complete_note:'',dod_checklist:body.dod_checklist||[],created_at:now,updated_at:now,attempts:[],activities:[]};tasks.push(t);createdKeys.set(body.request_id,t)}if(failCreate){failCreate=false;await json({error:'Fixture create response lost'},503)}else await json(t,201);return true}
 }
 const match=p.match(/^\/api\/v1\/tasks\/([^/]+)(.*)$/);if(!match)return false
 const t=tasks.find(t=>t.id===match[1]),tail=match[2];if(!t){await json({error:'Task not found'},404);return true}
 if(!tail&&method==='GET'){await json({...t,attempts:t.attempts||[],activities:t.activities||[]});return true}
 if(!tail&&method==='PUT'){if(failSave){await json({error:'Fixture save failed; draft is retained'},503);return true}if(body.expected_updated_at&&body.expected_updated_at!==t.updated_at){await json({error:'Task changed. Reload the saved task'},409);return true}if(body.status==='done'&&(!(body.complete_note||t.complete_note)?.trim()||(body.dod_checklist||t.dod_checklist||[]).some(i=>!i.completed))){await json({error:'Verification is required'},400);return true}for(const k of ['title','instructions','status','priority','summary','complete_note','dod_checklist','project_ids'])if(body[k]!==undefined)t[k]=body[k];if(body.project_ids)t.project_id=body.project_ids[0]||'';if(body.assigned_agent!==undefined)t.assignee_id=body.assigned_agent;bump(t);await json(t);return true}
 if(!tail&&method==='DELETE'){tasks.splice(tasks.indexOf(t),1);await json({id:t.id});return true}
 if(tail==='/comments'&&method==='POST'){const a={id:'comment_'+version,activity_type:'comment',author:'Owner',message:body.message,created_at:now};t.activities.push(a);await json(a,201);return true}
 if(tail==='/run-agent/history'){const a=t.latest_attempt;const live=a&&['running','queued'].includes(a.status)?{session_id:'live_'+a.id,attempt_id:a.id,agent_id:a.agent_name,started_at:a.created_at}:null;await json({history:(t.attempts||[]).map(a=>({attempt_id:a.id,session_id:'live_'+a.id,status:a.status})),live_session:live});return true}
 if(tail.includes('/logs')){await json({logs:logRows.filter(l=>l.id>Number(u.searchParams.get('after_id')||0)).slice(0,250)});return true}
 if(tail==='/run-agent'&&method==='POST'){launches.push(body);const replay=runKeys.get(body.request_id);if(replay){await json({status:replay.status,attempt:replay,replayed:true,session_id:'live_'+replay.id});return true}const a={id:'attempt_'+version,task_id:t.id,runner_id:'local_terminal',agent_name:body.agent_id,status:'running',attempt_number:(t.attempts?.length||0)+1,created_at:now,started_at:now};t.attempts=[a,...t.attempts||[]];t.latest_attempt=a;t.status='doing';bump(t);runKeys.set(body.request_id,a);if(body.revision)t.activities.push({id:'revision_'+version,activity_type:'comment',author:'Owner',message:'Revision requested:\n'+body.revision,created_at:now});await new Promise(resolve=>setTimeout(resolve,600));if(failLaunch){failLaunch=false;await json({error:'Fixture launch response lost'},503)}else await json({status:'running',attempt:a,session_id:'live_'+a.id});return true}
 if(tail==='/cancel'&&method==='POST'){t.latest_attempt.status='canceled';t.latest_attempt.exit_code=130;t.latest_attempt.finished_at=now;t.status='blocked';bump(t);await json(t.latest_attempt);return true}
 return false
}
let browser, server, page
let contactFail = false
let topicFail=true;const topicRequests=[];
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
  await page.routeWebSocket(/\/api\/v1\/tasks\/[^/]+\/terminal/, ws=>{const url=new URL(ws.url());assert(!url.searchParams.has('token'));assert.equal(url.searchParams.get('attach'),'1');terminalConnections.push(url.searchParams.get('session_id'));ws.send(JSON.stringify({type:'connected',session_id:url.searchParams.get('session_id'),mode:'watch'}));ws.send(Buffer.from('Live fixture output\r\nverification-needle:PASS\r\n'));ws.onMessage(message=>{const d=JSON.parse(String(message));if(d.type==='mode')ws.send(JSON.stringify({type:'status',mode:d.mode}));if(d.type==='stdin'){inputWrites.push(d.data);ws.send(Buffer.from('ACK:'+d.data))}})})
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

    if(p==='/api/v1/notifications') return json({notifications:[],unread_count:0})
    if(p==='/api/v1/item-preferences') return json({items:{}})
    if(p==='/api/system/worker-health') return json({status:'ready',is_alive:true,queued_count:0,running_count:0,queue_depth:0})
    if(p==='/api/system/update') return json({current_version:'fixture',latest_version:'fixture',has_update:false,auto_update_enabled:false,auto_update_time:'02:00'})
    if(p==='/api/v1/studio/goals' && method==='POST'){
      const data=r.postDataJSON();topicRequests.push(data);
      let topic=contacts.find(c=>c.conversation_id==='topic_'+data.request_id);
      if(!topic){topic={id:'topic_'+data.request_id,agent_id:'agent_good',agent_name:'@verified',type:'agent',name:data.goal,avatar:'',status:'idle',conversation_id:'topic_'+data.request_id,unread_count:0,updated_at:now};contacts.push(topic)}
      if(topicFail){topicFail=false;return json({error:'Fixture response lost. Retry safely.'},503)}
      return json({conversation_id:topic.conversation_id,status:'accepted'},202)
    }
    if(p==='/api/v1/chat/conversations'&&method==='POST'){
     const data=r.postDataJSON();topicRequests.push(data);const a=agents.find(a=>a.id===data.agent_id);assert(a);let topic=contacts.find(c=>c.conversation_id==='topic_'+data.request_id);if(!topic){topic={id:'topic_'+data.request_id,agent_id:a.id,agent_name:a.name,type:'agent',name:'New conversation',avatar:'',status:'idle',conversation_id:'topic_'+data.request_id,unread_count:0,updated_at:now};contacts.push(topic)}
     if(topicFail){topicFail=false;return json({error:'Fixture topic response lost. Retry safely.'},503)}return json(topic)
    }
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
    if (p==='/api/v1/agents/auth-status') return json({runtime_prerequisites:{cli_available:true,sandbox_available:true,installation_supported:true}})
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
 for(const width of [390,1440])for(const theme of ['light','dark']){
  await page.setViewportSize({width,height:900});await page.goto(base+'/settings/appearance');await page.getByRole('button',{name:theme==='light'?'Light':'Dark',exact:true}).click();
  await page.goto(base+'/studio');await page.waitForURL(u=>u.pathname==='/studio');
  const goal=page.getByRole('textbox',{name:'Your goal',exact:true});const submit=page.getByRole('button',{name:'Send',exact:true});await goal.waitFor();
  assert(await submit.isDisabled());assert.equal(await page.getByRole('combobox').count(),0);
  assert.equal(await goal.evaluate(el=>getComputedStyle(el).fontSize),'16px');
  const text='Organize a useful goal '+width+' '+theme;await goal.fill(text);await page.reload();await goal.waitFor();assert.equal(await goal.inputValue(),text);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));if(width===390){const nav=page.getByRole('navigation',{name:'Bottom navigation'});assert.deepEqual(await nav.locator(':scope > button, :scope > div').allTextContents(),['Studio','Inbox','Tasks','Projects','FIAccount']);assert.equal(await nav.locator('[aria-current=page]').getAttribute('data-testid'),'bottom-nav-studio');}
  assert.equal(await page.locator('[data-studio-goal-panel] aside').count(),0);
  await page.screenshot({path:out+'/studio-'+theme+'-'+width+'.png'});
  await submit.click();if(topicRequests.length===1){await page.getByRole('alert').waitFor();await submit.click();assert.equal(topicRequests[0].request_id,topicRequests[1].request_id)}
  await page.waitForURL(u=>u.pathname.startsWith('/inbox/topic_'));
  await page.goto(base+'/inbox');await page.getByRole('button',{name:'New message',exact:true}).click();await page.waitForURL(u=>u.pathname==='/studio');
  await goal.waitFor();assert.equal(await goal.inputValue(),'');
  checks.push({width,theme,defaultStudio:true,draftRetained:true,inboxNewToStudio:true,noSelectors:true,noOverflow:true});
 }
 await page.setViewportSize({width:390,height:844});
 await page.goto(base+'/tasks');await page.locator('[data-page-header]').first().waitFor();
 assert.equal(await page.locator('[data-page-header] .lucide-square-check-big').count(),1);
 await page.screenshot({path:out+'/tasks-mobile.png'});
 await page.getByTestId('bottom-nav-user-button').click();
 await page.getByRole('button',{name:'Contacts',exact:true}).click();await page.waitForURL(u=>u.pathname==='/contacts');
 await page.screenshot({path:out+'/contacts-mobile.png'});
 await page.getByTestId('bottom-nav-user-button').click();await page.getByTestId('navuser-settings').click();await page.waitForURL(u=>u.pathname==='/settings');await page.getByRole('heading',{name:'Settings',exact:true}).first().waitFor();
 await page.screenshot({path:out+'/settings-mobile.png'});
 await page.goto(base+'/studio');await page.getByRole('textbox',{name:'Your goal',exact:true}).waitFor();await page.evaluate(()=>localStorage.setItem('cuan-language','id'));await page.reload();await page.getByPlaceholder('Tulis aja yang lagi kepikiran…').waitFor();await page.screenshot({path:out+'/studio-empty-id-mobile.png'});
 assert.deepEqual(errors,[]);assert.deepEqual(unknown,[]);assert.equal(launches.length,0);
 fs.writeFileSync(out+'/browser.json',JSON.stringify({fixture:true,checks,idempotentGoalRetry:true,providerInvocations:0,errors,unknown},null,2)+'\n');console.log('PASS Studio responsive navigation, drafts, retry and accepted conversation');
})().catch(async e=>{if(page)await page.screenshot({path:out+'/check-failure.png'});console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))})
