// Real React components with isolated API fixtures. Never customer/live evidence.
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import {createRequire} from 'node:module'
import {createServer} from 'vite'
const require=createRequire(import.meta.url)
const {chromium}=require(process.env.CUAN_BROWSER_MODULE||path.join(os.homedir(),'.local/lib/node_modules/playwright'))
const output=process.argv[2]
if(!output)throw Error('Supply a new private evidence directory')
await fs.mkdir(output,{recursive:true,mode:0o700})
const now='2026-11-06T05:00:00Z',room='work-fixture'
const contact={id:room,conversation_id:room,name:'Website pelanggan',type:'customer',email:'customer@example.test',unread_count:0,status:'online',last_message:'The work is queued waiting for the active project task to complete. [View work](/tasks/work)',updated_at:now,task_count:5,active_tasks:1}
const extraContacts=[
  {id:'c-fahmi',conversation_id:'c-fahmi',name:'fahmi putra',type:'customer',email:'fahmi@example.test',unread_count:0,status:'online',is_online:true,last_message:'Biar kami aktifkan',updated_at:now,task_count:0,active_tasks:0},
  {id:'c-kisworo',conversation_id:'c-kisworo',name:'kisworo izar',type:'customer',email:'kisworo@example.test',unread_count:0,status:'offline',last_message:'Customer reply draft: Halo sob! Akses...',updated_at:now,task_count:1,active_tasks:0,done_tasks:1,resolved:false},
  {id:'c-azizoke',conversation_id:'c-azizoke',name:'azizoke 2022-05-30 09:30:43',type:'customer',email:'azizoke@example.test',unread_count:0,status:'offline',last_message:'Email lama kenapa sob',updated_at:now,task_count:0,active_tasks:0},
  {id:'c-anon',conversation_id:'c-anon',name:'Visitor #3f8a91',type:'customer',email:'',unread_count:0,status:'offline',last_message:'Tanya produk kak',updated_at:now,task_count:0,active_tasks:0},
  {id:'c-agent',conversation_id:'c-agent',name:'Penyederhanaan Timeline Task Chat',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',last_message:'Termasuk posisi judul task lain sejajar itu...',updated_at:now,task_count:1,active_tasks:1},
  {id:'c-review',conversation_id:'c-review',name:'Peringatan Kapasitas Server',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',last_message:'Hasil optimasi selesai diperiksa.',updated_at:now,task_count:1,active_tasks:0,review_tasks:1},
  {id:'c-queued',conversation_id:'c-queued',name:'Pindahkan Tombol New Work',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',last_message:'The work is queued waiting for the active project task to complete.',updated_at:now,task_count:1,queued_tasks:1,active_tasks:0},
]
const contactsList=[contact,...extraContacts]
const agents=[{id:'agent_cuan',name:'@cuan',status:'active',enabled:true,harness:'agy',provider:'agy',supports_vision:true}]
const attempt={id:'attempt',task_id:'work',status:'running',agent_name:'agent_cuan',runner_id:'runner',attempt_number:2,created_at:now,started_at:now}
const baseTask={id:'work',title:'Perbaiki tampilan website pelanggan dan periksa hasilnya pada desktop maupun ponsel sebelum menyatakan pekerjaan selesai',status:'doing',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,source_message_id:'source',repo_url:'https://github.com/example/site',created_at:now,updated_at:now,latest_attempt:attempt,latest_progress:'Memeriksa tampilan mobile setelah perbaikan.',instructions:'Perbaiki tampilan pada desktop dan mobile. Verifikasi hasil sebenarnya.',summary:'Laporan sebelumnya belum diterima.',dod_checklist:[{text:'Tampilan stabil pada mobile dan desktop',completed:false}]}
const earlierTasks=[
  {id:'task-1',title:'Diagnosa Operasional Kendala Login Magic Link Nerd di Server Baru',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-40000).toISOString(),updated_at:now},
  {id:'task-2',title:'Remediasi dan Perbaikan Pengiriman Magic Link Login Nerd di Server Baru',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-30000).toISOString(),updated_at:now},
  {id:'task-3',title:'Sinkronisasi dan Deploy Release HEAD ke Production VPS',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-20000).toISOString(),updated_at:now},
  {id:'task-4',title:'Validasi Status dan Respons Server Baru Pasca Migrasi',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-10000).toISOString(),updated_at:now},
  {id:'task-review',title:'Optimasi & Pembersihan Ruang Penyimpanan',status:'review',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-5000).toISOString(),updated_at:now,latest_progress:'Menunggu pemeriksaan',summary:'Identified disk usage bottlenecks (89% used, 9.0 GiB free)...'},
]
const allTasks=[...earlierTasks,baseTask]
const activities=Array.from({length:30},(_,index)=>({id:'progress-'+index,task_id:'work',attempt_id:'attempt',activity_type:'agent_report',new_status:'progress',author:'agent_cuan',message:['Memeriksa sumber masalah.','Meninjau perubahan yang tersimpan.','Menguji tampilan pada browser.','Membandingkan hasil dengan permintaan awal.'][index%4],created_at:new Date(Date.parse(now)+index*1000).toISOString()}))
activities.push({id:'long-review',task_id:'work',attempt_id:'attempt',activity_type:'coordinator_review_assessment',author:'coordinator',message:JSON.stringify({assessment:{decision:'revise',finding:('Pemeriksaan hasil perlu membandingkan perilaku aplikasi dengan permintaan awal dan bukti yang tersimpan. ').repeat(12)+'Akhir laporan asli.'}}),created_at:new Date(Date.parse(now)+31000).toISOString()})
activities.push({id:'provider-note',task_id:'work',activity_type:'comment',author:'runner',message:'Provider preflight selected allowed backup account',created_at:now})
const fixtureTask={...baseTask,attempts:[attempt],activities}
const message=(id,text,extra={})=>({id,uuid:String(id),room_id:room,sender_type:'operator',author_type:'agent',author_id:'agent_cuan',author_name:'Cuan',message:text,content:text,is_internal:true,created_at:now,...extra})
const messages=[message('source','Tolong perbaiki tampilan website.',{author_type:'user',author_id:'operator_user'}),...Array.from({length:12},(_,i)=>message('chat-'+i,'Pembahasan kebutuhan yang tetap tersedia di percakapan.')),
 message('choices','Pilih langkah yang paling sesuai.',{prompt_choices:{question:'Bagaimana kita lanjutkan?',options:[{id:'continue',label:'Lanjutkan pemeriksaan'},{id:'adjust',label:'Sesuaikan kebutuhan'}],allow_other:true}}),
 message('reply-out','Sudut bubble chat diperbarui dengan lengkungan yang halus.',{sender_type:'operator',author_type:'operator',author_id:'operator_user',author_name:'You',is_internal:false}),
 message('old','Hasil perlu diperiksa lagi. [View work](/tasks/work)',{work_update:{task_id:'work',kind:'progress'}}),
 message('room-ref','Berikut ruang chat agen terkait: [Penyederhanaan Timeline Task Chat](/inbox/c-agent)'),
  message('reply-draft','Draf email balasan untuk pelanggan sudah disiapkan.',{reply_draft:'Halo sob,\n\nKendala login sudah diperbaiki dan sistem normal kembali. Silakan dicoba login ulang.',is_internal:true}),
 message('files','Files from this work session',{work_update:{task_id:'work',attempt_id:'attempt',kind:'files'},attachments:[{id:'file',name:'report.txt',type:'text/plain',size:10,url:'/api/v1/chat/attachments/file'}]}),
 message('proposal-pending','Siap, ane susun proposal untuk Riwayat & Log Pengiriman (Phase 3).\n\nDi fase ini, setiap sesi broadcast yang dikirim akan otomatis tercatat di menu Kontak—lengkap dengan waktu pengiriman, subjek pesan, jumlah penerima sukses, dan rincian error jika ada email yang gagal terkirim. Tampilannya dirancang minimalis dan rapi mengikuti standar Things 3 / Bear Notes agar mudah dipantau tanpa menambah beban visual.\n\nDetail rencana dan kriteria tuntasnya sudah ane cantumkan pada kartu persetujuan berikut. Silakan dikonfirmasi jika cakupannya sudah sesuai.',{
   author_type:'agent',
   is_internal:true,
   approval_plan:{
     title:'Phase 3: Riwayat & Log Pengiriman Broadcast Email CRM',
     description:'Problem Statement:\nSetelah Phase 1 (resolver audiens & preview) dan Phase 2 (draf pesan, batch sender, idempotency, dan RFC 8058) selesai terverifikasi, sistem belum memiliki pencatatan riwayat broadcast. Operator membutuhkan visibilitas riwayat pengiriman lampau beserta detail status keberhasilan, log kegagalan, dan metadata pengiriman untuk audit serta transparansi operasional.',
     project_name:'cuan'
   },
   approval_request_id:'plan-approve-1',
   plan_defer_request_id:'plan-defer-1',
   approval_accepted:false
 }),
 message('proposal-accepted','Berikut proposal rencana yang telah disetujui:',{
   author_type:'agent',
   is_internal:true,
   approval_plan:{
     title:'Penyelarasan Desain Kartu Approval Chat: Eliminasi Nested Box Cards',
     description:'Merapikan struktur visual komponen kartu proposal/approval di linimasa percakapan agar menyatu secara seamless dengan bubble pesan chat tanpa efek kotak dalam kotak.',
     project_name:'cuan'
   },
   approval_request_id:'plan-approve-2',
   plan_defer_request_id:'plan-defer-2',
   approval_accepted:true
 }),
 message('proposal-stale-done','Rilis hasil yang diperiksa belum tuntas. [View work](/tasks/task-1)',{
   author_type:'agent',
   is_internal:true,
   approval_plan:{
     title:'Lanjutkan pemeriksaan',
     description:'Lengkapi pemeriksaan agar hasil pekerjaan bisa dipastikan.',
     project_name:'Website',
     continuation:{task_id:'task-1',attempt_id:'attempt-old'}
   },
   approval_request_id:'plan-approve-stale-done',
   approval_accepted:false
 }),
 message('proposal-expired','Hasil pekerjaan sebelumnya memerlukan konfirmasi kelanjutan:',{
   author_type:'agent',
   is_internal:true,
   approval_plan:{
     title:'Lanjutkan pemeriksaan berkala',
     description:'Lengkapi pemeriksaan agar hasil pekerjaan bisa dipastikan.',
     project_name:'Website'
   },
   approval_request_id:'plan-approve-expired',
   approval_accepted:false
 })]
const entry=`import React from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter,useLocation} from 'react-router-dom';
import {LanguageProvider} from '/src/context/LanguageContext.tsx';
import {AuthProvider} from '/src/context/AuthContext.tsx';
import {AppShell} from '/src/components/layout/AppShell.tsx';
import {InboxView} from '/src/features/inbox/InboxView.tsx';
import {TaskSessionPage} from '/src/features/tasks/TaskSessionPage.tsx';
import {InboxSubPanel} from '/src/features/inbox/InboxSubPanel.tsx';
import {StudioView} from '/src/features/inbox/StudioView.tsx';
import '/src/index.css';
const contact=${JSON.stringify(contact)},agents=${JSON.stringify(agents)},contactsList=${JSON.stringify(contactsList)};
const noop=()=>{};
function Scene(){
  const location=useLocation();
  if(location.pathname.startsWith('/tasks/work/sessions/')){
    return React.createElement(TaskSessionPage,{taskId:'work',sessionId:location.pathname.split('/')[4]||'latest'})
  }
  if(location.pathname.startsWith('/studio')){
    return React.createElement('div',{className:'h-full flex-1 overflow-y-auto px-3 py-6 sm:px-6 md:px-8 bg-background'},React.createElement(StudioView,{agentSetup:'ready',hasProject:true,firstGoal:false,onAccepted:noop}))
  }
  return React.createElement(InboxView,{contacts:contactsList,agents,activeContact:contact,initialMode:'chat'})
}
function AppFixture(){
  const location=useLocation();
  const currentTab=location.pathname.startsWith('/studio')?'studio':'inbox';
  return React.createElement(AppShell,{currentTab,onSelectTab:noop,inboxSubPanel:React.createElement(InboxSubPanel,{contacts:contactsList,agents,activeContactId:contact.id,onSelectContact:noop,selectedTab:'all',searchQuery:'',onSearchChange:noop,onTabChange:noop,onNewStudio:noop,onNewMessage:noop})},React.createElement(Scene));
}
createRoot(document.getElementById('root')).render(React.createElement(BrowserRouter,null,React.createElement(LanguageProvider,null,React.createElement(AuthProvider,null,React.createElement(AppFixture)))));`
const webRoot=path.resolve('web')
process.chdir(webRoot)
const server=await createServer({root:webRoot,envDir:path.resolve(output),cacheDir:path.join(path.resolve(output),'vite-cache'),server:{host:'127.0.0.1',port:0},plugins:[{name:'work-fixture',enforce:'pre',transform(code,id){if(process.env.CUAN_HOOK_REGRESSION_PROBE==='1'&&id.split('?')[0].endsWith('/TaskSessionPage.tsx')){if(!code.includes('  if (!detail) {'))throw Error('Hook regression probe target missing');return code.replace('  if (!detail) {','  if (detail) useRef(null)\n  if (!detail) {')}},resolveId(id){if(id==='/__work_entry.jsx')return '\0work-fixture-entry.jsx'},load(id){if(id==='\0work-fixture-entry.jsx')return entry},configureServer(server){server.middlewares.use(async(req,res,next)=>{
 if(req.url.startsWith('/inbox/work-fixture')||req.url.startsWith('/tasks/work/sessions/')||req.url.startsWith('/studio')){res.setHeader('Content-Type','text/html');res.end(await server.transformIndexHtml(req.url,'<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/__work_entry.jsx"></script></body></html>'))}
 else next()
})}}]})
await server.listen()
const origin=server.resolvedUrls.local[0]
let browser
const checks=[]
try{
 for(const width of (process.env.CUAN_WORKFLOW_NATIVE==='1'?[390,1440]:[390,1440,1920])){
  browser=process.env.CUAN_REMOTE_BROWSER_URL
    ? await chromium.connectOverCDP(process.env.CUAN_REMOTE_BROWSER_URL)
    : await chromium.launch({headless:true,executablePath:process.env.CUAN_CHROMIUM_EXECUTABLE,args:process.env.CUAN_BROWSER_NO_SANDBOX==='1'?['--no-sandbox']:[]})
  const context=await browser.newContext({viewport:{width,height:width===390?844:1000},hasTouch:width===390})
  await context.addInitScript(()=>{localStorage.setItem('cuan-language','id');localStorage.setItem('cuan_token','isolated-fixture')})
  const page=await context.newPage(),errors=[]
  await page.routeWebSocket('**/api/v1/tasks/*/terminal?*', socket=>{
   socket.send(JSON.stringify({type:'connected',session_id:'fixture-session',mode:'interactive',output_ack:true}))
   socket.send(Buffer.from('Isolated terminal fixture output\r\nApplication workflow verification ready.\r\n'))
  })
  let releaseSession,sessionLoading=true,sessionStatus='running'
  page.on('pageerror',error=>errors.push(error.stack || error.message))
  page.on('console',message=>{if(message.type()==='error')console.log(message.text())})
  await page.route('**/api/**',async route=>{
   const pathname=new URL(route.request().url()).pathname
   if(!pathname.startsWith('/api/'))return route.continue()
   let data={}
   if(pathname.endsWith('/auth/status'))data={setup_required:false}
   else if(pathname.endsWith('/auth/me'))data={user:{id:'owner',email:'owner@example.test',name:'Owner',role:'owner'}}
   else if(pathname.endsWith('/messages')&&route.request().method()==='POST'){
     let body={}
     try{body=route.request().postDataJSON()||{}}catch{}
     if(body?.approve_message_id==='proposal-expired'){
       return route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({error:'This recommendation is no longer current. Check the latest task result before approving.'})})
     }
     return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({ok:true})})
   }
   else if(pathname.endsWith('/messages')){
     const match=pathname.match(/\/conversations\/([^/]+)\/messages/)
     const reqId=match?match[1]:null
     const matchedContact=reqId?(contactsList.find(c=>c.id===reqId||c.conversation_id===reqId)||contact):contact
     data={messages,tasks:allTasks,contact:matchedContact,requests:[]}
   }
   else if(pathname==='/api/v1/tasks/work'){
    if(page.url().includes('/sessions/')&&sessionLoading){sessionLoading=false;await new Promise(resolve=>{releaseSession=resolve})}
    data=sessionStatus==='running'?fixtureTask:{...fixtureTask,status:'review',latest_attempt:{...attempt,status:'succeeded',finished_at:now},attempts:[{...attempt,status:'succeeded',finished_at:now}]}
   }
   else if(pathname==='/api/v1/tasks/task-review'){
    data={
      id:'task-review',title:'Optimasi & Pembersihan Ruang Penyimpanan',status:'review',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:now,updated_at:now,
      latest_attempt:{id:'attempt-review',task_id:'task-review',status:'succeeded',agent_name:'agent_cuan',runner_id:'runner',finished_at:now},
      latest_progress:'Menunggu pemeriksaan',summary:'Pembersihan cache berhasil dieksekusi.',
      attempts:[{id:'attempt-review',task_id:'task-review',status:'succeeded',started_at:now,finished_at:now}],
      activities:[
        {id:'act-1',task_id:'task-review',activity_type:'created',message:'Work created',created_at:now},
        {id:'act-2',task_id:'task-review',activity_type:'agent_report',new_status:'review',message:'Eksekusi selesai, menunggu pemeriksaan',created_at:now}
      ]
    }
   }
   else if(pathname.endsWith('/run-agent/history'))data={history:[],live_session:sessionStatus==='running'?{session_id:'fixture-session',attempt_id:'attempt',agent_id:'agent_cuan',started_at:now}:null}
   else if(pathname.endsWith('/logs'))data={logs:[{id:1,line:'Isolated terminal fixture output',stream:'stdout',logged_at:now}]}
   else if(pathname.includes('/agents'))data=pathname==='/api/v1/agents'?{agents}:agents[0]
   else if(pathname.includes('preferences'))data={items:{}}
   else if(pathname.endsWith('/projects'))data={projects:[]}
   else if(pathname.includes('/email')&&route.request().method()==='POST')return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({status:'success',email_id:'re_mock_fixture_123',delivery_status:'sent',recipient:'customer@example.test',message:'Email balasan berhasil dikirim'})})
   else if(pathname.endsWith('/cancel')&&route.request().method()==='POST')return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({status:'cancelled'})})
   else if(pathname.endsWith('/files')&&route.request().method()==='POST')return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({files:[{filename:'screenshot.png',path:'./screenshot.png',size:2048}]})})
   else if(pathname.endsWith('/tasks'))data={tasks:allTasks}
   else if(pathname.endsWith('/stream')||pathname.endsWith('/events'))return route.fulfill({status:200,contentType:'text/event-stream',body:''})
   return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)})
  })
  await page.goto(new URL('/inbox/work-fixture',origin).href,{waitUntil:'domcontentloaded'})
  await page.evaluate(dark=>document.documentElement.classList.toggle('dark',dark),width===1920)
  // Verify unified calm navigation styling (Things 3 & Bear Notes)
  if(width===390){
    const bottomNav = page.locator('nav.bottom-navigation')
    await bottomNav.waitFor({ state: 'visible' })
    const activeBottomInbox = bottomNav.locator('[data-testid="bottom-nav-inbox"][aria-current="page"]')
    await activeBottomInbox.waitFor({ state: 'visible' })
    const inactiveBottomTasks = bottomNav.locator('[data-testid="bottom-nav-tasks"]')
    await inactiveBottomTasks.waitFor({ state: 'visible' })
    assert.equal(await inactiveBottomTasks.getAttribute('aria-current'), null, 'Mobile tasks must be inactive')
    await bottomNav.screenshot({ path: path.join(output, 'mobile-navigation-active-inbox.png') })
    await inactiveBottomTasks.hover()
    await page.waitForTimeout(100)
    await bottomNav.screenshot({ path: path.join(output, 'mobile-navigation-hover.png') })
  }
  if(width>=1200){
    const desktopNav = page.locator('nav.primary-navigation')
    await desktopNav.waitFor({ state: 'visible' })
    const activeInbox = desktopNav.locator('.nav-action[aria-current="page"]')
    await activeInbox.waitFor({ state: 'visible' })
    assert.equal(await activeInbox.getAttribute('data-nav-tone'), 'inbox', 'Active nav item must be inbox')
    const inactiveTasks = desktopNav.locator('.nav-action[data-nav-tone="tasks"]')
    await inactiveTasks.waitFor({ state: 'visible' })
    assert.equal(await inactiveTasks.getAttribute('aria-current'), null, 'Tasks must be inactive')
    await desktopNav.screenshot({ path: path.join(output, 'desktop-navigation-active-inbox.png') })
    await inactiveTasks.hover()
    await page.waitForTimeout(100)
    await desktopNav.screenshot({ path: path.join(output, 'desktop-navigation-hover.png') })
  }
  // Verify avatar monograms in Inbox header and contact list
  if(width===390){
    const backBtn=page.locator('button[aria-label="Kembali ke percakapan"], button[aria-label="Back to conversations"]').first()
    if(await backBtn.count()>0 && await backBtn.isVisible()){
      await backBtn.click()
      await page.waitForTimeout(300)
      assert.ok(await page.locator('[data-avatar="true"]:has-text("FP")').count()>=1,'Mobile contacts must display FP monogram')
      assert.ok(await page.locator('[data-avatar="true"]:has-text("KI")').count()>=1,'Mobile contacts must display KI monogram')
      assert.ok(await page.locator('[data-avatar="true"]:has-text("AZ")').count()>=1,'Mobile contacts must display AZ monogram')
      assert.ok(await page.locator('[data-avatar="true"]:has-text("WP")').count()>=1,'Mobile contacts must display WP monogram')
      assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Sedang dikerjakan")').count()>=1,'Mobile list must show "Sedang dikerjakan" for active running task')
      assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Menunggu antrean")').count()>=1,'Mobile list must show "Menunggu antrean" for queued task')
      assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Biar kami aktifkan")').count()>=1,'Mobile list must fall back to last_message for idle contact')
      await page.screenshot({path:path.join(output,'inbox-mobile-contacts-390-light.png')})
      await page.evaluate(()=>document.documentElement.classList.add('dark'))
      await page.screenshot({path:path.join(output,'inbox-mobile-contacts-390-dark.png')})
      await page.evaluate(()=>document.documentElement.classList.remove('dark'))
      await page.locator('[data-testid="conversation-item"]:visible').first().click()
      await page.waitForTimeout(300)
    }
  }
  if(width>=1200){
    assert.ok(await page.locator('[data-avatar="true"]:has-text("FP")').count()>=1,'Desktop sidebar must display FP monogram')
    assert.ok(await page.locator('[data-avatar="true"]:has-text("KI")').count()>=1,'Desktop sidebar must display KI monogram')
    assert.ok(await page.locator('[data-avatar="true"]:has-text("AZ")').count()>=1,'Desktop sidebar must display AZ monogram')
    assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Sedang dikerjakan")').count()>=1,'Desktop list must show "Sedang dikerjakan" for active running task')
    assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Menunggu antrean")').count()>=1,'Desktop list must show "Menunggu antrean" for queued task')
    assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Biar kami aktifkan")').count()>=1,'Desktop list must fall back to last_message for idle contact')
    await page.screenshot({path:path.join(output,`inbox-desktop-monogram-${width}.png`)})
  }
  // Tasks live in the composer disclosure, not as duplicate chat cards.
  const tasks=page.locator('.composer-tasks > div > button[aria-expanded]')
  await tasks.waitFor({state:'visible'})
  assert.equal(await tasks.getAttribute('aria-expanded'),'false')
  assert.equal(await page.locator('.inbox-message-list [data-card="task-card"]').count(),0)
  const collapsedChevronBox=await tasks.locator('svg').last().boundingBox()
  await page.screenshot({path:path.join(output,`composer-tasks-collapsed-${width}.png`)})
  const initialChatScroll=await page.locator('.inbox-message-list').evaluate(el=>el.scrollTop)
  await tasks.click()
  assert.equal(await tasks.getAttribute('aria-expanded'),'true')
  const afterOpenChatScroll=await page.locator('.inbox-message-list').evaluate(el=>el.scrollTop)
  assert.equal(afterOpenChatScroll,initialChatScroll,'Main chat history scroll must remain undisturbed')
  const panelEl=page.locator('.composer-tasks-panel')
  const card=page.locator('.composer-tasks-panel [data-timeline-item="work"]')
  try{await card.waitFor({state:'visible',timeout:20000})}catch(error){await page.screenshot({path:path.join(output,'failed-'+width+'.png')});console.log(JSON.stringify({errors,body:(await page.locator('body').innerText()).slice(0,1500)}));throw error}
  assert.equal(await card.count(),1)
  const cardBox=await card.boundingBox()
  const panelBox=await panelEl.boundingBox()
  assert.ok(cardBox && panelBox)
  assert.ok(cardBox.y >= panelBox.y - 2, 'Newest task must be at top within visible panel height')
  assert.ok(cardBox.y + cardBox.height <= panelBox.y + panelBox.height + 2, 'Newest task must not be clipped')
  const collapseBtn=page.locator('[data-testid="composer-tasks-collapse"]')
  await collapseBtn.waitFor({state:'visible'})
  const expandedChevronBox=await collapseBtn.locator('svg').boundingBox()
  if(collapsedChevronBox && expandedChevronBox){
    assert.ok(Math.abs(collapsedChevronBox.x - expandedChevronBox.x) <= 3, `Zero layout shift horizontal: expected ~${collapsedChevronBox.x}, got ${expandedChevronBox.x}`)
    assert.ok(Math.abs(collapsedChevronBox.y - expandedChevronBox.y) <= 3, `Zero layout shift vertical: expected ~${collapsedChevronBox.y}, got ${expandedChevronBox.y}`)
  }
  const reviewCard = page.locator('.composer-tasks-panel [data-timeline-item="task-review"]')
  await reviewCard.waitFor({state:'visible'})
  assert.equal(await reviewCard.locator('svg.lucide-eye').count(), 1, 'Task in review must display Eye icon')
  assert.equal(await reviewCard.locator('svg.lucide-check-check').count(), 0, 'Task in review must not display double checkmark')
  assert.equal(await page.locator('.composer-tasks-panel svg.lucide-check-check').count(), 0, 'No task in review should use check-check')
  await page.screenshot({path:path.join(output,`composer-tasks-expanded-${width}.png`)})
  assert.equal(await page.getByText('Files from this work session',{exact:true}).count(),0)
  assert.equal(await card.locator('[data-timeline-trigger]').getAttribute('aria-expanded'),'true')
  await card.getByText(baseTask.latest_progress,{exact:true}).waitFor()
  const composer=page.locator('[data-primary-composer]')
  // Verify elimination of composer mode tabs (DoD 1)
  assert.equal(await page.locator('#compose-public-tab').count(), 0)
  assert.equal(await page.locator('#compose-internal-tab').count(), 0)

  // Verify auto-whisper on @agent mention and instant visual feedback (DoD 2, 3, 4)
  await composer.fill('@cuan tolong periksa laporan ini')
  const whisperButton = page.locator('button[aria-label*="whisper" i], button[aria-label*="bisikan" i]')
  await whisperButton.waitFor({state:'visible'})
  assert.equal(await whisperButton.count(), 1)
  assert.equal(await page.getByText('Bisikkan ke @cuan', { exact: false }).count(), 1)
  await page.screenshot({path:path.join(output,`inbox-whisper-${width}.png`)})

  // Verify deletion of mention restores public reply mode instantly
  await composer.fill('Draft tetap tersimpan')
  const sendButton = page.locator('button[aria-label*="Kirim pesan" i], button[aria-label*="Send message" i]')
  await sendButton.waitFor({state:'visible'})
  assert.equal(await sendButton.count(), 1)
  assert.equal(await whisperButton.count(), 0)
  await page.screenshot({path:path.join(output,`inbox-composer-${width}.png`)})

  // Verify alignment of composer container with chat stream (DoD 1)
  const streamConstraint = page.locator('.inbox-message-list .calm-stream-constraint')
  const composerConstraint = page.locator('[data-inbox-composer] .calm-stream-constraint')
  await streamConstraint.waitFor({ state: 'visible' })
  await composerConstraint.waitFor({ state: 'visible' })
  const streamBox = await streamConstraint.boundingBox()
  const composerBox = await composerConstraint.boundingBox()
  assert.ok(streamBox && composerBox)
  assert.equal(Math.round(composerBox.x), Math.round(streamBox.x), `Composer must horizontally align with chat stream at width ${width}`)
  assert.equal(Math.round(composerBox.width), Math.round(streamBox.width), `Composer width must match chat stream width at width ${width}`)

  // Verify multi-line auto-expand beyond 140px without clipping (DoD 2)
  const multilineText = 'Baris 1: Instruksi pekerjaan panjang\nBaris 2: Detail implementasi yang rapi\nBaris 3: Memastikan auto-expand mulus\nBaris 4: Tanpa visual clipping\nBaris 5: Area pengetikan tetap proporsional\nBaris 6: Nyaman dibaca dan diketik'
  await composer.fill(multilineText)
  const multilineHeight = await composer.evaluate(el => el.getBoundingClientRect().height)
  assert.ok(multilineHeight > 140, `Composer textarea must expand beyond 140px for multi-line text (observed ${multilineHeight}px)`)
  assert.ok(multilineHeight <= 200, `Composer textarea must respect maximum cap of 200px (observed ${multilineHeight}px)`)
  await page.screenshot({ path: path.join(output, `inbox-composer-expanded-${width}.png`) })
  // Restore initial draft
  await composer.fill('Draft tetap tersimpan')
  const list=page.locator('.inbox-message-list')
  await list.evaluate(el=>{el.scrollTop=Math.max(0,el.scrollHeight-el.clientHeight-40)})
  await card.scrollIntoViewIfNeeded()
  const choices=page.getByTestId('prompt-choice-chips')
  await choices.waitFor({state:'visible'})
  assert.equal(await choices.getByRole('button',{name:'Lanjutkan pemeriksaan',exact:true}).count(),1)
  assert.equal(await choices.getByRole('button',{name:'Sesuaikan kebutuhan',exact:true}).count(),1)
  for(const button of await choices.getByRole('button').all()) {
   assert.ok((await button.boundingBox()).height>=44)
   const style=await button.evaluate(el=>({radius:getComputedStyle(el).borderRadius,background:getComputedStyle(el).backgroundColor}))
   assert.equal(style.radius,'12px')
   assert.ok(style.background==='rgba(0, 0, 0, 0)' || style.background==='transparent',JSON.stringify(style))
  }
  const incomingBubble=page.locator('.wa-message-bubble.wa-incoming').first()
  await incomingBubble.waitFor({state:'visible'})
  const incomingRadius=await incomingBubble.evaluate(el=>({
    tl:getComputedStyle(el).borderTopLeftRadius,
    tr:getComputedStyle(el).borderTopRightRadius,
    br:getComputedStyle(el).borderBottomRightRadius,
    bl:getComputedStyle(el).borderBottomLeftRadius,
  }))
  assert.equal(incomingRadius.tr,'16px')
  assert.equal(incomingRadius.br,'16px')
  assert.equal(incomingRadius.bl,'16px')

  const outgoingBubble=page.locator('.wa-message-bubble.wa-outgoing').first()
  if(await outgoingBubble.count()>0){
    const outgoingRadius=await outgoingBubble.evaluate(el=>({
      tl:getComputedStyle(el).borderTopLeftRadius,
      tr:getComputedStyle(el).borderTopRightRadius,
      br:getComputedStyle(el).borderBottomRightRadius,
      bl:getComputedStyle(el).borderBottomLeftRadius,
    }))
    assert.equal(outgoingRadius.tl,'16px')
    assert.equal(outgoingRadius.bl,'16px')
    assert.equal(outgoingRadius.br,'16px')
  }
  const draftCard = page.locator('[data-card="reply-draft"]')
  await draftCard.waitFor({state:'visible'})
  assert.equal(await draftCard.count(), 1)
  const sendEmailBtn = draftCard.locator('[data-send-email-action]')
  await sendEmailBtn.waitFor({state:'visible'})
  assert.equal(await sendEmailBtn.count(), 1)
  assert.ok((await sendEmailBtn.boundingBox()).height >= 44)
  await draftCard.scrollIntoViewIfNeeded()
  await page.screenshot({path:path.join(output,`customer-reply-card-${width}.png`)})
  await sendEmailBtn.click()
  const sentBadge = draftCard.locator('[data-email-sent-badge]')
  await sentBadge.waitFor({state:'visible'})
  assert.equal(await sentBadge.count(), 1)
  await page.screenshot({path:path.join(output,`customer-reply-sent-${width}.png`)})

  // Verify Approval Card Seamless Integration (No Nested Box Cards)
  const pendingCard = page.locator('[data-testid="approval-card"][data-status="pending"]').filter({hasText:'Phase 3'}).first()
  await pendingCard.waitFor({state:'visible'})
  assert.equal(await pendingCard.count(), 1)
  // Verify no nested calm-message-bubble inside wa-message-bubble
  assert.equal(await page.locator('.wa-message-bubble .calm-message-bubble').count(), 0, 'No nested bubble inside message bubble')
  // Verify ApprovalCard is borderless/transparent (no stacked box card)
  const pendingClass = await pendingCard.getAttribute('class')
  assert.ok(pendingClass.includes('border-0') && pendingClass.includes('border-l-2') && pendingClass.includes('bg-transparent'), 'Approval card must be border-0 border-l-2 bg-transparent')
  assert.ok(!pendingClass.includes('bg-card'), 'Approval card must eliminate stacked bg-card')
  // Check action button touch target (min-h-11)
  const approveBtn = pendingCard.getByRole('button', {name: /Approve|Setujui/i})
  assert.ok((await approveBtn.boundingBox()).height >= 44, 'Approve button must meet 44px touch target')

  await pendingCard.scrollIntoViewIfNeeded()
  await page.screenshot({path:path.join(output,`approval-card-pending-${width}.png`)})

  const acceptedCard = page.locator('[data-testid="approval-card"][data-status="accepted"]')
  await acceptedCard.waitFor({state:'visible'})
  await acceptedCard.scrollIntoViewIfNeeded()
  await page.screenshot({path:path.join(output,`approval-card-accepted-${width}.png`)})

  // Verify Stale Approval Card Auto-Dismissal:
  // Continuation card for task-1 (already marked done in earlierTasks) must NOT be rendered.
  assert.equal(
    await page.locator('[data-testid="approval-card"]').filter({hasText:'Lanjutkan pemeriksaan'}).filter({hasNotText:'berkala'}).count(),
    0,
    'Stale continuation approval card for completed task must be dismissed'
  )

  // Verify Graceful Stale Handling on Click without Raw English Error:
  const expiredCard = page.locator('[data-testid="approval-card"]').filter({hasText:'Lanjutkan pemeriksaan berkala'})
  if(await expiredCard.count() > 0) {
    const approveExpiredBtn = expiredCard.getByRole('button', {name: /Approve|Setujui/i})
    await approveExpiredBtn.click()
    const staleNotice = page.getByText('Rekomendasi persetujuan ini sudah tidak berlaku karena status tugas telah diperbarui.')
    await staleNotice.waitFor({state:'visible'})
    assert.equal(await staleNotice.count(), 1, 'Friendly stale notice must be visible')
    assert.equal(await page.getByText(/continuationDescriptionMatches/i).count(), 0, 'Raw continuationDescriptionMatches must not be displayed')
    assert.equal(await page.getByText(/is no longer current/i).count(), 0, 'Raw English message must not be displayed')
    await page.screenshot({path:path.join(output,`approval-card-stale-notice-${width}.png`)})
  }

  // Verify RoomCard / Chat reference card calm styling & low visual noise (DoD)
  const roomCard = page.locator('.inbox-message-list [data-card="room-card"]').first()
  await roomCard.waitFor({state:'visible',timeout:10000})
  assert.ok(await roomCard.count() >= 1, 'RoomCard must be visible in chat stream')
  const roomCardText = await roomCard.innerText()
  assert.ok(!roomCardText.includes('#'), 'RoomCard surface must be free of hash IDs')
  assert.ok(roomCardText.includes('Penyederhanaan Timeline Task Chat'), 'Room title must be displayed as primary anchor')
  assert.ok(!roomCardText.includes('Peserta:'), 'RoomCard must eliminate redundant Peserta: label')
  assert.ok(await roomCard.getByRole('button', {name: /Buka ruang|Open room/i}).count() >= 1, 'Open room button must be present')
  assert.ok(await roomCard.getByRole('button', {name: /Salin tautan|Copy link/i}).count() >= 1, 'Copy link button must be present')
  const openRoomBtn = roomCard.getByRole('button', {name: /Buka ruang|Open room/i})
  assert.ok((await openRoomBtn.boundingBox()).height >= 44, 'Open room button must meet 44px touch target')
  const copyLinkBtn = roomCard.getByRole('button', {name: /Salin tautan|Copy link/i})
  assert.ok((await copyLinkBtn.boundingBox()).height >= 44, 'Copy link button must meet 44px touch target')
  await roomCard.scrollIntoViewIfNeeded()
  await roomCard.screenshot({path:path.join(output,`room-card-${width}.png`)})

  // Frame the actual choices with the work preview in native Inbox captures.
  await choices.scrollIntoViewIfNeeded()
  const previousScroll=await list.evaluate(el=>el.scrollTop)
  await page.screenshot({path:path.join(output,`inbox-${width}.png`)})
  await card.getByRole('button',{name:/^(Details|Detail|Rincian)$/}).click()
  const panel=page.getByTestId('inbox-work-panel')
  await panel.locator('[data-work-timeline]').waitFor({state:'visible'})
  assert.equal(await panel.locator('h2').count(),1)
  const heading=panel.locator('[data-work-title]')
  assert.equal(await heading.getAttribute('aria-expanded'),'false')
  await heading.click();assert.equal(await heading.getAttribute('aria-expanded'),'true')
  await heading.click()
  assert.equal(await panel.getByText('Provider preflight selected allowed backup account',{exact:true}).isVisible(),false)
  assert.equal(await panel.locator('[data-work-terminal]').count(),0)
  const box=await panel.boundingBox()
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
  if(width===390){assert.equal(Math.round(box.width),390);await panel.getByRole('button',{name:'Kembali ke percakapan'}).waitFor()}
  else if(width===1920){assert.ok(box.x>1000);assert.ok(await list.isVisible())}
  else assert.equal(await list.isVisible(),false)
  const dots=panel.locator('[data-status-dot]')
  assert.equal(await dots.filter({has:page.locator('.animate-ping')}).count(),1)
  const report=panel.locator('[data-work-event="long-review"]')
  const expand=report.locator('[data-timeline-trigger]')
  assert.equal(await expand.getAttribute('aria-expanded'),'true')
  await expand.click()
  assert.equal(await expand.getAttribute('aria-expanded'),'false')
  await expand.click()
  assert.equal(await expand.getAttribute('aria-expanded'),'true')
  assert.deepEqual(await report.locator('.chat-markdown').first().evaluate(el=>({size:getComputedStyle(el).fontSize,line:getComputedStyle(el).lineHeight})),{size:'14px',line:'22px'})
  await report.scrollIntoViewIfNeeded()
  await page.screenshot({path:path.join(output,`report-${width}.png`)})
  const scroll=panel.locator('[data-work-scroll]')
  await page.waitForFunction(()=>document.querySelector('[data-work-scroll]').scrollTop>0,null,{timeout:5000})
  await scroll.evaluate(el=>{el.scrollTop=0;el.dispatchEvent(new Event('scroll'))})
  await panel.getByRole('button',{name:'Progres terbaru'}).waitFor()
  await page.screenshot({path:path.join(output,`detail-${width}.png`)})
  await page.goBack()
  await panel.waitFor({state:'detached'})
  assert.equal(await composer.inputValue(),'Draft tetap tersimpan')
  await page.waitForFunction(expected=>Math.abs(document.querySelector('.inbox-message-list').scrollTop-expected)<5,previousScroll,{timeout:2000})
  await card.getByRole('button',{name:/^(Details|Detail|Rincian)$/}).click()
  await panel.waitFor({state:'visible'})
  await panel.locator('[data-work-back]').click()
  await panel.waitFor({state:'detached'})
  const reviewAccordionTrigger = page.locator('.composer-tasks-panel [data-timeline-item="task-review"] [data-timeline-trigger]')
  if(await reviewAccordionTrigger.count() > 0 && await reviewAccordionTrigger.isVisible()){
    if(await reviewAccordionTrigger.getAttribute('aria-expanded') === 'false'){
      await reviewAccordionTrigger.click()
    }
    const reviewDetailsBtn = page.locator('.composer-tasks-panel [data-timeline-item="task-review"]').getByRole('button',{name:/^(Details|Detail|Rincian)$/})
    if(await reviewDetailsBtn.count() > 0 && await reviewDetailsBtn.isVisible()){
      await reviewDetailsBtn.click()
      await panel.waitFor({state:'visible'})
      const reviewDot = panel.locator('[data-status-dot="review"]')
      await reviewDot.waitFor({state:'visible', timeout: 5000})
      assert.ok(await reviewDot.count() >= 1, 'TaskTimeline must show review status dot')
      await page.screenshot({path:path.join(output,`timeline-review-${width}.png`)})
      await panel.locator('[data-work-back]').click()
      await panel.waitFor({state:'detached'})
    }
  }
  await card.getByRole('button',{name:/terminal/i}).waitFor({state:'visible'})
  await card.getByRole('button',{name:/terminal/i}).click()
  await panel.locator('[data-work-terminal]').waitFor({state:'visible'})
  assert.equal(await panel.locator('[data-work-timeline]').count(),0)
  await panel.locator('.xterm').waitFor({state:'visible'})
  const terminalBox=await panel.locator('[data-work-terminal]').boundingBox()
  assert.ok(terminalBox.height>650)
  await page.locator('.xterm-screen').getByText('Application workflow verification ready.',{exact:false}).waitFor()
  await page.screenshot({path:path.join(output,`terminal-${width}.png`)})
  // Test native terminal drag & drop file upload
  const terminalElement=panel.locator('[aria-label="Terminal sesi"]')
  await terminalElement.evaluate(el=>{
    const dt=new DataTransfer()
    dt.items.add(new File(['mock-bytes'],'screenshot.png',{type:'image/png'}))
    el.dispatchEvent(new DragEvent('dragenter',{bubbles:true,cancelable:true,dataTransfer:dt}))
  })
  const dragOverlay=panel.locator('[role="region"][aria-label="Lepaskan file untuk upload ke workspace"]')
  await dragOverlay.waitFor({state:'visible',timeout:5000})
  assert.equal(await dragOverlay.isVisible(),true)
  await page.screenshot({path:path.join(output,`terminal-drag-${width}.png`)})
  await terminalElement.evaluate(el=>{
    const dt=new DataTransfer()
    dt.items.add(new File(['mock-bytes'],'screenshot.png',{type:'image/png'}))
    el.dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:dt}))
  })
  await page.waitForTimeout(600)
  await page.screenshot({path:path.join(output,`terminal-dropped-${width}.png`)})
  // Exercise the actual session component across the state transition which
  // exposes conditional hook errors. No Owner credentials or backend writes.
  await page.goto(new URL('/tasks/work/sessions/latest',origin).href,{waitUntil:'domcontentloaded'})
  await page.locator('[role="status"] h3').waitFor({state:'visible',timeout:10000})
  assert.equal(typeof releaseSession,'function')
  const rendered=Promise.race([page.locator('[aria-label="Task session"]').waitFor({state:'visible',timeout:15000}),new Promise((_,reject)=>page.once('pageerror',reject))])
  releaseSession()
  await rendered
  await page.locator('.xterm').waitFor({state:'visible',timeout:15000})
  await page.locator('.xterm-screen').getByText('Application workflow verification ready.',{exact:false}).waitFor()
  await page.screenshot({path:path.join(output,`session-running-${width}.png`)})
  // Test Cancel session modal
  const cancelButton = page.locator("button[aria-label=\"Cancel session\"]")
  await cancelButton.waitFor({state: "visible", timeout: 5000})
  await cancelButton.click()
  const dialog = page.locator("dialog[open]:has-text(\"Cancel session?\")")
  await dialog.waitFor({state: "visible", timeout: 5000})
  const modalHeading = dialog.locator("h2", { hasText: "Cancel session?" })
  await modalHeading.waitFor({state: "visible", timeout: 5000})
  const modalBody = dialog.getByText("Stop the active process? Saved output and existing file changes are retained.")
  await modalBody.waitFor({state: "visible", timeout: 5000})
  const keepRunning = dialog.locator("button.action-secondary", { hasText: "Keep running" })
  const confirmCancel = dialog.locator("button.action-primary", { hasText: "Cancel session" })
  assert.ok(await keepRunning.isVisible(), "Keep running button must be visible")
  assert.ok(await confirmCancel.isVisible(), "Cancel session button must be visible")
  const artifactName = width === 390 ? "mobile.png" : "desktop.png"
  await page.screenshot({path: path.join(output, artifactName)})
  await page.screenshot({path: path.join(output, `cancel-modal-${width}.png`)})
  await keepRunning.click()
  await dialog.waitFor({state: "hidden", timeout: 5000})
  sessionStatus='succeeded'
  await page.locator('[aria-label="Session result"]').waitFor({state:'visible',timeout:10000})
  await page.screenshot({path:path.join(output,`session-result-${width}.png`)})
  assert.equal(await page.getByText('Something went wrong',{exact:true}).count(),0)
  assert.deepEqual(errors,[])

  // Verify StudioView canvas and auto-expand at /studio
  await page.goto(new URL('/studio', origin).href, { waitUntil: 'domcontentloaded' })
  const studioGoal = page.locator('#studio-goal')
  await studioGoal.waitFor({ state: 'visible' })
  const initialStudioHeight = await studioGoal.evaluate(el => el.getBoundingClientRect().height)
  assert.ok(initialStudioHeight >= 104, 'Studio goal textarea must have minimum 104px height')
  if(width===390){
    const bottomNav = page.locator('nav.bottom-navigation')
    await bottomNav.waitFor({ state: 'visible' })
    const activeBottomStudio = bottomNav.locator('[data-testid="bottom-nav-studio"][aria-current="page"]')
    await activeBottomStudio.waitFor({ state: 'visible' })
    await bottomNav.screenshot({ path: path.join(output, 'mobile-navigation-active-studio.png') })
  }
  if(width>=1200){
    const desktopNav = page.locator('nav.primary-navigation')
    await desktopNav.waitFor({ state: 'visible' })
    const activeStudio = desktopNav.locator('.nav-action[aria-current="page"]')
    await activeStudio.waitFor({ state: 'visible' })
    assert.equal(await activeStudio.getAttribute('data-nav-tone'), 'studio', 'Active nav item must be studio')
    await desktopNav.screenshot({ path: path.join(output, 'desktop-navigation-active-studio.png') })
  }
  await page.screenshot({ path: path.join(output, `studio-${width}.png`) })

  await studioGoal.fill('Target pekerjaan baru:\n1. Integrasi API\n2. Penyesuaian layout\n3. Verifikasi otomatis\n4. Dokumentasi lengkap\n5. Pengujian di perangkat nyata')
  const expandedStudioHeight = await studioGoal.evaluate(el => el.getBoundingClientRect().height)
  assert.ok(expandedStudioHeight > initialStudioHeight, 'Studio goal textarea must smoothly auto-expand when typing multiple lines')
  await page.screenshot({ path: path.join(output, `studio-expanded-${width}.png`) })

  checks.push({width,composer_task_disclosure:true,composer_task_autoscroll:true,last_task_visible:true,no_chat_task_card:true,session_loading_running_completed:true,back_preserves_draft_and_scroll:true,full_terminal_height:terminalBox.height,panel_width:box.width,composer_alignment:true,multiline_autoexpand:true,studio_autoexpand:true})
  await context.close()
  await browser.close()
  browser=null
 }
 await fs.writeFile(path.join(output,'verification.json'),JSON.stringify({source:'Actual application React components; isolated API fixtures, not live task evidence',checks},null,2))
 console.log(JSON.stringify(checks))
}finally{await browser?.close();await server.close()}
