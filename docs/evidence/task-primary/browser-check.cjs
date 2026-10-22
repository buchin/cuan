// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/task-primary',
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





 const checks=[];
 const oldTime=new Date(Date.parse(now)-60000).toISOString();
 function attempt(id,taskId,number,status='succeeded'){return{id,task_id:taskId,agent_name:'agent_good',runner_id:'local_terminal',attempt_number:number,status,created_at:now,started_at:now,finished_at:status==='running'?undefined:now,exit_code:status==='running'?undefined:0}}
 function report(id,taskId,attemptId,summary,kind='implementation',at=now){return[{id:id+'_kind',task_id:taskId,attempt_id:attemptId,activity_type:'agent_result_kind',new_status:kind,message:'Result classification',created_at:at},{id,task_id:taskId,attempt_id:attemptId,activity_type:'agent_report',new_status:'review',message:['Agent report: review','','## Work summary',summary,'','## Verification','Actual fixture evidence.'].join(String.fromCharCode(10)),created_at:at}]}
 tasks[0]={...tasks[0],title:'Export customer contacts',instructions:'Export contacts as CSV.',status:'review',assigned_agent:'agent_good',summary:'Contacts export is ready.',complete_note:'Verified fixture CSV output.',dod_checklist:[{text:'Verify CSV output',completed:false}],attempts:[attempt('result_current','task_one',2),attempt('result_old','task_one',1)],activities:[...report('current','task_one','result_current','Contacts export is ready.'),...report('old','task_one','result_old','Earlier CSV result.','implementation',oldTime)]};tasks[0].latest_attempt=tasks[0].attempts[0];
 tasks[1]={...tasks[1],title:'Say hello',instructions:'Say hello',status:'review',assigned_agent:'agent_good',summary:'Hello! How can I help?',complete_note:'Answered directly; no files or tests needed.',dod_checklist:[],attempts:[attempt('answer_current','task_two',1)],activities:report('answer','task_two','answer_current','Hello! How can I help?','answer')};tasks[1].latest_attempt=tasks[1].attempts[0];
 tasks.push({id:'ready_task',title:'Add an export button',instructions:'Add an export button',project_id:'p1',status:'backlog',priority:'normal',created_at:now,updated_at:now,attempts:[],activities:[]});
 tasks.push({id:'live_task',title:'Check contact export',instructions:'Check contact export',status:'doing',assigned_agent:'agent_good',priority:'normal',created_at:now,updated_at:now,attempts:[attempt('live_current','live_task',1,'running')],activities:[]});tasks.at(-1).latest_attempt=tasks.at(-1).attempts[0];
 for(const width of [360,390,820,1440])for(const theme of ['light','dark']){
  await page.setViewportSize({width,height:900});await page.goto(base+'/settings/appearance');await page.getByRole('button',{name:theme==='light'?'Light':'Dark',exact:true}).click();
  await page.goto(base+'/tasks/task_one?q=export');await page.waitForURL(u=>u.pathname.endsWith('/sessions/result_current'));
  await page.getByRole('region',{name:'Session result',exact:true}).getByText('Contacts export is ready.',{exact:true}).waitFor();
  assert.equal(await page.locator('.xterm-screen').count(),0);
  await page.getByRole('region',{name:'Original request',exact:true}).getByText('Export contacts as CSV.',{exact:true}).waitFor();
  assert.equal(await page.getByRole('button',{name:'Accept',exact:true}).count(),1);
  assert.equal(await page.getByRole('dialog').count(),0);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.screenshot({path:out+'/result-'+theme+'-'+width+'.png'});
  await page.goto(base+'/tasks/task_two');await page.waitForURL(u=>u.pathname.endsWith('/sessions/answer_current'));
  await page.getByRole('heading',{name:'Answer',exact:true}).waitFor();
  assert.equal(await page.getByRole('button',{name:'Accept',exact:true}).count(),0);
  await page.getByRole('textbox',{name:'Follow-up instructions',exact:true}).waitFor();
  if(width===390||width===1440)await page.screenshot({path:out+'/answer-'+theme+'-'+width+'.png'});
  checks.push({width,theme,directLatestSession:true,requestAndResult:true,evidenceCollapsed:true,noTerminalUntilRequested:true,answerWithoutAccept:true,noOverflow:true});
 }
 assert.equal(launches.length,0);
 await page.setViewportSize({width:1440,height:900});
 await page.goto(base+'/tasks/task_one?q=export');await page.waitForURL(u=>u.pathname.endsWith('/sessions/result_current'));
 await page.getByRole('button',{name:'Accept',exact:true}).click();
 let modal=page.getByRole('dialog',{name:'Accept result',exact:true});
 assert.equal(await modal.getByRole('button',{name:'Accept result',exact:true}).isEnabled(),false);
 await page.getByRole('button',{name:'Close Accept result',exact:true}).click();
 await page.getByRole('button',{name:'Detail',exact:true}).click();
 await page.getByText('Agent checklist',{exact:true}).click();
 await page.getByRole('checkbox',{name:'Verify: Verify CSV output',exact:true}).click();await page.waitForFunction(()=>document.querySelector('input[aria-label="Verify: Verify CSV output"]')?.checked);
 await page.getByRole('button',{name:'Close Detail',exact:true}).click();
 await page.getByRole('button',{name:'Accept',exact:true}).click();
 modal=page.getByRole('dialog',{name:'Accept result',exact:true});
 await modal.getByRole('textbox',{name:'Acceptance evidence',exact:true}).fill('Verified the CSV matches the requested contacts and workspace.');
 await modal.getByRole('button',{name:'Accept result',exact:true}).click();
 await page.getByText('Result accepted.',{exact:true}).waitFor();
 assert.equal(tasks[0].status,'done');
 await page.getByRole('button',{name:'Detail',exact:true}).click();await page.getByRole('button',{name:'Open terminal',exact:true}).click();await page.getByText('Saved output',{exact:true}).waitFor();
 await page.getByRole('button',{name:'Back to result',exact:true}).click();
 await page.getByRole('button',{name:'Detail',exact:true}).click();
 await page.getByRole('combobox',{name:'Select session',exact:true}).selectOption('result_old');
 await page.getByText('Earlier CSV result.',{exact:true}).waitFor();
 assert.equal(await page.getByRole('button',{name:'Accept',exact:true}).count(),0);
 assert.equal(await page.getByRole('textbox',{name:'Follow-up instructions',exact:true}).count(),0);
 await page.getByRole('button',{name:'Detail',exact:true}).click();
 await page.getByRole('link',{name:/Open task details/}).click();
 await page.getByRole('dialog',{name:'Task detail',exact:true}).waitFor();
 await page.getByRole('button',{name:'Expand task view',exact:true}).click();
 await page.getByRole('button',{name:'Collapse task view',exact:true}).click();
 await page.getByRole('button',{name:'Close task details',exact:true}).click();
 await page.waitForURL(u=>u.pathname.endsWith('/sessions/result_old'));
 await page.getByText('Earlier CSV result.',{exact:true}).waitFor();
 await page.goto(base+'/tasks/ready_task?q=export');
 await page.getByRole('region',{name:'Task workspace',exact:true}).waitFor();
 assert.equal(launches.length,0);
 await page.getByRole('link',{name:'Detail',exact:true}).click();
 await page.getByRole('dialog',{name:'Task detail',exact:true}).waitFor();
 await page.getByRole('button',{name:'Close task details',exact:true}).click();
 await page.getByRole('region',{name:'Task workspace',exact:true}).waitFor();
 assert.equal(new URL(page.url()).searchParams.get('q'),'export');
 await page.getByRole('button',{name:'Start',exact:true}).click();
 await page.getByText('Starting session…',{exact:true}).waitFor();
 await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(launches.length,1);
 await page.goto(base+'/tasks/live_task');
 await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(launches.length,1);
 await page.getByRole('button',{name:'Enable input',exact:true}).click();await page.getByText('Connected · input enabled',{exact:true}).waitFor();
 await page.locator('.xterm-helper-textarea').pressSequentially('Check the CSV');assert(inputWrites.join('').includes('Check the CSV'));
 await page.reload();await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(launches.length,1);
 await page.goto(base+'/tasks/task_two');
 await page.getByRole('heading',{name:'Answer',exact:true}).waitFor();
 await page.getByRole('textbox',{name:'Follow-up instructions',exact:true}).fill('Explain the next step');
 await page.getByRole('button',{name:'Start follow-up session',exact:true}).click();
 await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(launches.length,2);assert.equal(launches.at(-1).revision,'Explain the next step');
 assert.equal(tasks[1].status,'doing');
 await page.goto(base+'/tasks/task_one/sessions/foreign-session');
 await page.getByText('Session not found for this task.',{exact:true}).waitFor();
 assert.equal(launches.length,2);
 await page.goto(base+'/tasks');await page.getByRole('button',{name:'New task',exact:true}).click();
 modal=page.getByRole('dialog',{name:'New task',exact:true});
 await modal.getByRole('combobox',{name:'Agent',exact:true}).selectOption('');
 await modal.getByRole('textbox',{name:'Task instructions',exact:true}).fill('Save for later');
 await modal.getByRole('button',{name:'Create task',exact:true}).click();
 await page.getByRole('region',{name:'Task workspace',exact:true}).waitFor();
 assert.equal(launches.length,2);
 await page.goto(base+'/tasks');await page.getByRole('button',{name:'New task',exact:true}).click();
 modal=page.getByRole('dialog',{name:'New task',exact:true});
 await modal.getByRole('combobox',{name:'Agent',exact:true}).selectOption('agent_good');
 await modal.getByRole('textbox',{name:'Task instructions',exact:true}).fill('Check the requested export');
 await modal.getByRole('button',{name:'Create task',exact:true}).click();
 await page.getByText('Starting session…',{exact:true}).waitFor();
 await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(launches.length,3);
 await page.reload();await page.getByText('Connected · read-only',{exact:true}).waitFor();assert.equal(launches.length,3);
 assert.deepEqual(errors,[]);assert.deepEqual(unknown,[]);
 fs.writeFileSync(out+'/browser.json',JSON.stringify({fixture:true,checks,explicitStartOnly:true,acceptanceGates:true,historySnapshot:true,readyTaskAndMetadataReturn:true,interactiveAttachOnly:true,followup:true,providerInvocations:0,errors,unknown},null,2)+String.fromCharCode(10));
 console.log('PASS task-primary workspace:8 responsive/theme checks, latest/history, ready/start, answer/result, acceptance gates and live input');
})().catch(async e=>{if(page)await page.screenshot({path:out+'/check-failure.png'});console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))})
