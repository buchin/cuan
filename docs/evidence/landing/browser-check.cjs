// Serves the exact production build; fixture API only, no provider/production writes.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'),
  path = require('path'),
  http = require('http'),
  assert = require('node:assert/strict')
const out = 'docs/evidence/landing',
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
const history=id=>{if(id.startsWith("topic_"))return {conversation_id:id,room_id:id,contact:contacts.find(c=>c.conversation_id===id),messages:chatAttachmentMessages.get(id)||((topicAttachments.get(id)||[]).length?[{id:1,uuid:'attachment-message',author_type:'user',sender_type:'visitor',content:'Attached reference',created_at:now,attachments:topicAttachments.get(id)}]:[]),requests:[],tasks:[]};const h=historyBase(id);h.requests[0]={...h.requests[0],message_id:'m1',reply_id:inboxPending?'':'m2',state:inboxPending?'running':'completed'};if(inboxPending)h.messages=h.messages.slice(0,1);h.tasks=id==='room0'?[tasks.find(t=>t.id==='linked_work')].filter(Boolean):[];return h}
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
let topicFail=true;const topicRequests=[];const uploaded=[],topicAttachments=new Map();const chatAttachmentMessages=new Map();
const profile={id:'fixture',username:'fixtureuser',display_name:'',email:'fixture@example.invalid'};
let preferences={approval:true,blocker:true,completed:true,customer:true,quiet:false,start_hour:22,end_hour:8,timezone:'UTC'},pushDevices=0;
const notifications=[{id:'approval-fixture',kind:'approval',title:'Approve fixture plan',href:'/inbox/room0',created_at:now,read:false},{id:'blocker-fixture',kind:'blocker',title:'Fixture needs attention',href:'/inbox/room1',created_at:now,read:false}];
const vapidKey=require('node:crypto').createECDH('prime256v1');vapidKey.generateKeys();
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
        file.endsWith('.webmanifest') ? 'application/manifest+json' : file.endsWith('.png') ? 'image/png' : file.endsWith('.js')
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
  browser = await chromium.launchPersistentContext(fs.mkdtempSync(path.join(require('node:os').homedir(),'.cache/cuan-pwa-fixture-')), {
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
  await page.addInitScript(()=>{
    const nativeUA=navigator.userAgent;Object.defineProperty(navigator,'userAgent',{get:()=>localStorage.getItem('fixture-ios')==='1'?'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)':nativeUA});
    window.addEventListener('beforeinstallprompt',e=>{if(!e.fixturePrompt)e.stopImmediatePropagation()},true);
    window.__installInvoked=0;
    window.__permissionRequests=0;window.__installInvoked=0;
    const sub=()=>({endpoint:'https://fcm.googleapis.com/fcm/send/browser-fixture',toJSON(){return{endpoint:this.endpoint,keys:{p256dh:'fixture-key',auth:'fixture-auth'}}},async unsubscribe(){localStorage.removeItem('fixture-push');return true}});
    PushManager.prototype.getSubscription=async()=>localStorage.getItem('fixture-push')?sub():null;
    PushManager.prototype.subscribe=async function(options){if(options.applicationServerKey.byteLength!==65)throw new Error('Invalid public key');localStorage.setItem('fixture-push','1');return sub()};
    Notification.requestPermission=async()=>{window.__permissionRequests++;return 'granted'};
  });
  await page.addInitScript(() =>
    localStorage.getItem('fixture-anonymous')==='1'?localStorage.removeItem('cuan_token'):localStorage.setItem('cuan_token', 'fixture-only'),
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

    if(p==='/api/v1/auth/me')return r.headers().authorization==='Bearer fixture-only'?json({owner:profile,user:profile}):json({error:'unauthorized'},401);
    if(p==='/api/v1/auth/login')return json({owner:profile,user:profile,token:'fixture-only'});
    if(p==='/api/v1/profile'){if(method==='PATCH')profile.display_name=r.postDataJSON().display_name;return json({display_name:profile.display_name})}
    if(p==='/api/v1/auth/username'){const body=r.postDataJSON();if(body.current_password!=='correct-password')return json({error:'Current password is incorrect.'},403);profile.username=body.username.toLowerCase();return json({username:profile.username})}
    if(p==='/api/v1/notifications/preferences'){if(method==='PATCH')preferences=r.postDataJSON();return json({preferences,devices:pushDevices})}
    if(p==='/api/v1/notifications/push'){if(method==='GET')return json({public_key:vapidKey.getPublicKey().toString('base64url')});pushDevices=method==='POST'?1:0;return json({ok:true})}
    if(p==='/api/v1/notifications'){if(method==='PATCH'){const body=r.postDataJSON();notifications.forEach(item=>{if(body.all||body.ids?.includes(item.id))item.read=true})}return json({items:notifications})}
    if(p==='/api/v1/chat/attachments'&&method==='POST'){
      assert.equal(r.headers().authorization,'Bearer fixture-only');
      const raw=r.postDataBuffer().toString('latin1'),name=raw.match(/filename="([^"]+)"/)[1];
      const a={id:'file_'+uploaded.length,name,type:name.endsWith('.png')?'image/png':'text/plain',size:25,url:'/api/v1/chat/attachments/file_'+uploaded.length};uploaded.push(a);await new Promise(resolve=>setTimeout(resolve,200));return json(a,201)
    }
    if(p.startsWith('/api/v1/chat/attachments/')&&method==='GET'){
      assert.equal(r.headers().authorization,'Bearer fixture-only');const a=uploaded.find(a=>a.url===p);assert(a);
      return route.fulfill({status:200,contentType:a.type,body:a.type==='image/png'?Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jN1sAAAAASUVORK5CYII=','base64'):Buffer.from('Fixture attachment text')})
    }
    if(method==='POST'&&/\/chat\/conversations\/[^/]+\/messages$/.test(p)){
      const data=r.postDataJSON(),room=p.split('/')[5];assert(data.attachment_ids.length);const attachments=data.attachment_ids.map(id=>uploaded.find(a=>a.id===id));chatAttachmentMessages.set(room,[{id:1,uuid:'sent_attachment',author_type:'user',sender_type:'visitor',content:data.message,attachments,created_at:now}]);return json({message:chatAttachmentMessages.get(room)[0]},202)
    }
    if(p==='/api/v1/studio/goals' && method==='POST'){
      const data=r.postDataJSON();topicRequests.push(data);topicAttachments.set("topic_"+data.request_id,(data.attachment_ids||[]).map(id=>uploaded.find(a=>a.id===id)));
      let topic=contacts.find(c=>c.conversation_id==='topic_'+data.request_id);
      if(!topic){topic={id:'topic_'+data.request_id,agent_id:'agent_good',agent_name:'@verified',type:'agent',name:data.goal,avatar:'',status:'idle',conversation_id:'topic_'+data.request_id,unread_count:0,updated_at:now};contacts.push(topic)}
      if(topicFail){topicFail=false;return json({error:'Fixture response lost. Retry safely.'},503)}
      return json({conversation_id:topic.conversation_id,status:'accepted'},202)
    }
    if(p==='/api/v1/chat/conversations'&&method==='POST'){
     const data=r.postDataJSON();topicRequests.push(data);topicAttachments.set("topic_"+data.request_id,(data.attachment_ids||[]).map(id=>uploaded.find(a=>a.id===id)));const a=agents.find(a=>a.id===data.agent_id);assert(a);let topic=contacts.find(c=>c.conversation_id==='topic_'+data.request_id);if(!topic){topic={id:'topic_'+data.request_id,agent_id:a.id,agent_name:a.name,type:'agent',name:'New conversation',avatar:'',status:'idle',conversation_id:'topic_'+data.request_id,unread_count:0,updated_at:now};contacts.push(topic)}
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
 await page.goto(base+'/studio');await page.getByRole('heading',{name:'What would you like to get done today?',exact:true}).waitFor();
 await page.evaluate(()=>{localStorage.setItem('fixture-anonymous','1');localStorage.removeItem('cuan_token')});
 for(const width of [360,390,820,1440])for(const theme of ['light','dark']){
  await page.setViewportSize({width,height:900});await page.evaluate(theme=>{localStorage.setItem('cuan-theme',theme);localStorage.setItem('cuan-language','en')},theme);await page.goto(base+'/');await page.getByRole('heading',{name:/Rest easy/}).waitFor();assert.equal(new URL(page.url()).pathname,'/');
  assert.equal(await page.getByRole('navigation',{name:'Main navigation',exact:true}).count(),0);assert.equal(await page.getByRole('link',{name:'Get started',exact:true}).count(),2);
  await page.getByRole('button',{name:'Work',exact:true}).click();await page.getByText('The work keeps moving.',{exact:true}).waitFor();await page.getByRole('button',{name:'Result',exact:true}).click();await page.getByText('A result you can check.',{exact:true}).waitFor();await page.getByRole('button',{name:'Plan',exact:true}).click();
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:out+'/landing-'+theme+'-'+width+'.png',fullPage:true});
  await page.getByRole('link',{name:'See how it works',exact:true}).click();await page.waitForURL(u=>u.hash==='#how-it-works');await page.getByRole('heading',{name:'A goal in. Work moving forward.',exact:true}).waitFor();
  await page.getByRole('link',{name:'Get started',exact:true}).first().click();await page.waitForURL(u=>u.pathname==='/login');await page.getByRole('heading',{name:'Sign in to Cuan',exact:true}).waitFor();await page.getByRole('link',{name:'Cuan home',exact:true}).click();await page.waitForURL(u=>u.pathname==='/');
  checks.push({width,theme,publicRootLanding:true,ctaToLogin:true,interactiveExample:true,noOverflow:true});
 }
 await page.evaluate(()=>localStorage.setItem('cuan-language','id'));await page.reload();await page.getByRole('heading',{name:/Tenang saja/}).waitFor();await page.screenshot({path:out+'/landing-id-mobile.png',fullPage:true});
 // Private deep links still require sign-in; root/login redirect for a signed-in owner.
 await page.goto(base+'/inbox/room0');await page.getByRole('heading',{name:'Sign in to Cuan',exact:true}).waitFor();await page.evaluate(()=>{localStorage.setItem('fixture-anonymous','0');localStorage.setItem('cuan-language','en')});await page.goto(base+'/');await page.waitForURL(u=>u.pathname==='/studio');await page.goto(base+'/login');await page.waitForURL(u=>u.pathname==='/studio');
 const goal=page.getByRole('textbox',{name:'Your goal',exact:true});await goal.fill('Keep my draft while installing');
 const install=()=>page.evaluate(()=>{const e=new Event('beforeinstallprompt');e.fixturePrompt=true;e.prompt=async()=>{window.__installInvoked++};e.userChoice=Promise.resolve({outcome:'accepted'});window.dispatchEvent(e)});
 await install();const nudge=page.getByRole('complementary',{name:'Install Cuan',exact:true});await nudge.waitFor();await nudge.getByRole('button',{name:'Dismiss install suggestion',exact:true}).click();assert.equal(await nudge.count(),0);await page.reload();await goal.waitFor();await install();assert.equal(await nudge.count(),0);assert.equal(await goal.inputValue(),'Keep my draft while installing');
 await page.evaluate(()=>localStorage.removeItem('cuan-install-dismissed-until'));await page.reload();await goal.waitFor();await install();await nudge.getByRole('button',{name:'Install Cuan',exact:true}).click();await page.waitForFunction(()=>window.__installInvoked===1);await page.evaluate(()=>window.dispatchEvent(new Event('appinstalled')));assert.equal(await nudge.count(),0);assert.equal(await goal.inputValue(),'Keep my draft while installing');
 await page.evaluate(()=>{localStorage.removeItem('cuan-install-dismissed-until');localStorage.setItem('fixture-ios','1')});await page.reload();await nudge.waitFor();await nudge.getByRole('button',{name:'Install Cuan',exact:true}).click();await page.getByRole('dialog',{name:'Install Cuan',exact:true}).getByText('In Safari, open Share → Add to Home Screen → Add.',{exact:true}).waitFor();await page.keyboard.press('Escape');

 // Installation remains discoverable independently of the Studio dismissal.
 await nudge.getByRole('link',{name:'Find this later in Settings → General.',exact:true}).click();await page.waitForURL(u=>u.pathname==='/settings'&&u.hash==='#install-cuan');
 const card=page.getByRole('region',{name:'Install Cuan',exact:true});await card.getByText('Tap Share, then Add to Home Screen.',{exact:true}).waitFor();
 await page.evaluate(()=>{localStorage.setItem('cuan-install-dismissed-until',String(Date.now()+604800000));localStorage.removeItem('fixture-ios')});await page.reload();await card.waitFor();
 for(const width of [390,1440]) {await page.setViewportSize({width,height:900});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await card.screenshot({path:out+'/settings-install-'+width+'.png'});}
 await install();await card.getByRole('button',{name:'Install Cuan',exact:true}).click();await page.waitForFunction(()=>window.__installInvoked===1);await page.evaluate(()=>window.dispatchEvent(new Event('appinstalled')));await card.getByText('Cuan is installed.',{exact:true}).waitFor();
 fs.writeFileSync(out+'/settings-install.json',JSON.stringify({permanentAfterDismissal:true,studioLink:true,iosSteps:true,nativePrompt:true,installedState:true,mobileAndDesktop:true,permissionRequests:0},null,2));
 assert.deepEqual(errors,[]);assert.deepEqual(unknown,[]);assert.equal(launches.length,0);assert.equal(await page.evaluate(()=>window.__permissionRequests),0);
 fs.writeFileSync(out+'/browser.json',JSON.stringify({fixture:true,checks,indonesian:true,privateRoutesProtected:true,authenticatedRootAndLoginToStudio:true,installNudgeDismissalAndPrompt:true,iosInstructions:true,draftPreserved:true,providerInvocations:0,errors,unknown},null,2)+'\n');console.log('PASS public landing, authenticated routing and install suggestion');
})().catch(async e=>{if(page)await page.screenshot({path:out+'/check-failure.png'});console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))})
