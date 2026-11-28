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
  {id:'c-agent',conversation_id:'c-agent',name:'Penyederhanaan Timeline Task Chat',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',last_message:'Termasuk posisi judul task lain sejajar itu...',updated_at:now,task_count:1,active_tasks:1,active_substage:'Checking result'},
  {id:'c-review',conversation_id:'c-review',name:'Peringatan Kapasitas Server',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',last_message:'Hasil optimasi selesai diperiksa.',updated_at:now,task_count:1,active_tasks:0,review_tasks:1},
  {id:'c-queued',conversation_id:'c-queued',name:'Pindahkan Tombol New Work',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',last_message:'The work is queued waiting for the active project task to complete.',updated_at:now,task_count:1,queued_tasks:1,active_tasks:0},
  {id:'c-queued-review',conversation_id:'c-queued-review',name:'Perbaiki Validasi Respons Review',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',last_message:'Pekerjaan masuk dalam antrean',updated_at:now,task_count:2,queued_tasks:1,review_tasks:1,active_tasks:0,active_substage:'Queued'},
  {id:'c-sys1',conversation_id:'c-sys1',name:'Rotasi Dinamis Kata Status Kerja Asisten',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',resolved:true,only_system_notices:true,last_message:'Sistem baru saja diperbarui. Pekerjaan Anda tersimpan aman dan dilanjutkan secara otomatis...',updated_at:now,task_count:1,active_tasks:0,done_tasks:1},
  {id:'c-sys2',conversation_id:'c-sys2',name:'Penyelarasan Desain UI Chat Reference',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',resolved:true,only_system_notices:true,last_message:'Sistem baru saja diperbarui. Pekerjaan Anda tersimpan aman dan dilanjutkan secara otomatis...',updated_at:now,task_count:1,active_tasks:0,done_tasks:1},
  {id:'c-sys3',conversation_id:'c-sys3',name:'Standarisasi Warna & Ikon Status Task',type:'agent',agent_id:'agent_cuan',avatar:'🤖',unread_count:0,status:'online',resolved:true,only_system_notices:true,last_message:'Sistem baru saja diperbarui. Pekerjaan Anda tersimpan aman dan dilanjutkan secara otomatis...',updated_at:now,task_count:1,active_tasks:0,done_tasks:1},
]
const contactsList=[contact,...extraContacts]
const agents=[{id:'agent_cuan',name:'@cuan',status:'active',enabled:true,harness:'agy',provider:'agy',supports_vision:true}]
const attempt={id:'attempt',task_id:'work',status:'running',agent_name:'agent_cuan',runner_id:'runner',attempt_number:2,created_at:now,started_at:now}
const baseTask={id:'work',title:'Perbaiki tampilan website pelanggan dan periksa hasilnya pada desktop maupun ponsel sebelum menyatakan pekerjaan selesai',status:'doing',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,source_message_id:'source',repo_url:'https://github.com/example/site',created_at:now,updated_at:now,latest_attempt:attempt,latest_progress:'Memeriksa tampilan mobile setelah perbaikan.',instructions:'Perbaiki tampilan pada desktop dan mobile. Verifikasi hasil sebenarnya.',summary:'Laporan sebelumnya belum diterima.',dod_checklist:[{text:'Tampilan stabil pada mobile dan desktop',completed:false}]}
const earlierTasks=[
  {id:'task-1',title:'Diagnosa Operasional Kendala Login Magic Link Nerd di Server Baru',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-40000).toISOString(),updated_at:now},
  {id:'task-2',title:'Remediasi dan Perbaikan Pengiriman Magic Link Login Nerd di Server Baru',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-30000).toISOString(),updated_at:now},
  {id:'task-3',title:'Sinkronisasi dan Deploy Release HEAD ke Production VPS',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-20000).toISOString(),updated_at:now,complete_note:'Ringkasan rilis:\n\n- Sinkronisasi branch utama dan verifikasi build aset produksi selesai.\n- Migrasi skema database berjalan tanpa downtime.\n- Health check endpoint `/healthz` mengembalikan status 200 OK.'},
  {id:'task-4',title:'Validasi Status dan Respons Server Baru Pasca Migrasi',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-10000).toISOString(),updated_at:now,complete_note:'Pemeriksaan pasca migrasi:\n\n- Verifikasi DNS, sertifikat TLS, dan reverse proxy Nginx.\n- Pengujian login magic link, pengiriman email SMTP, dan antrean background worker.\n- Pemantauan metrik CPU, memori, serta log aplikasi selama 15 menit tanpa error.'},
  {id:'task-review',title:'Optimasi & Pembersihan Ruang Penyimpanan',status:'review',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,created_at:new Date(Date.parse(now)-5000).toISOString(),updated_at:now,latest_progress:'Menunggu pemeriksaan',summary:'Identified disk usage bottlenecks (89% used, 9.0 GiB free)...'},
  {id:'task-attention',title:'Investigasi dan perbaiki pemutaran video dracin',status:'blocked',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,latest_attempt:{id:'attempt-attention',task_id:'task-attention',status:'failed',created_at:new Date(Date.parse(now)-4000).toISOString()},created_at:new Date(Date.parse(now)-4000).toISOString(),updated_at:now,summary:'Pemutaran video langsung terhalang karena layanan API upstream mengembalikan error 502.'},
  {
    id:'task-done-live',
    title:'Investigasi & Perbaiki Pembacaan Lampiran Gambar di Chat',
    status:'done',
    priority:'normal',
    project_id:'project',
    project_name:'Website',
    source_room_id:room,
    commit_sha:'9c4d3f7466565284dbfa786e6a8c46ac93cab45c',
    summary:'Pembacaan lampiran gambar di chat (termasuk jalur coba lagi) sudah diperbaiki dan diverifikasi.',
    complete_note:'Independent verification of commit 9c4d3f7466565284dbfa786e6a8c46ac93cab45c (read-only attempt): ran targeted Go tests which passed in 2.716s (saved in session:targeted_verification_tests.txt). Remaining gap: changes are committed locally at 9c4d3f7466565284dbfa786e6a8c46ac93cab45c and not yet pushed/merged to branch dev.\n\n🚀 Perubahan telah diverifikasi aktif pada rilis live (9c4d3f74).',
    created_at:new Date(Date.parse(now)-2500).toISOString(),
    completed_at:now,
    updated_at:now,
  },
]
const allTasks=[...earlierTasks,baseTask]
let workTaskOverride=null
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
 message('gallery-4-images','Tangkapan layar hasil pengujian UI:',{
   author_type:'agent',
   is_internal:false,
   attachments:[
     {id:'img-1',name:'preview-1.png',type:'image/png',size:24000,url:'/api/v1/chat/attachments/img-1'},
     {id:'img-2',name:'preview-2.png',type:'image/png',size:24000,url:'/api/v1/chat/attachments/img-2'},
     {id:'img-3',name:'preview-3.png',type:'image/png',size:24000,url:'/api/v1/chat/attachments/img-3'},
     {id:'img-4',name:'preview-4.png',type:'image/png',size:24000,url:'/api/v1/chat/attachments/img-4'},
   ]
 }),
 message('gallery-mixed','Berikut hasil desain dan dokumen spesifikasi:',{
   author_type:'agent',
   is_internal:false,
   attachments:[
     {id:'img-a',name:'desain-desktop.png',type:'image/png',size:15000,url:'/api/v1/chat/attachments/img-a'},
     {id:'img-b',name:'desain-mobile.png',type:'image/png',size:18000,url:'/api/v1/chat/attachments/img-b'},
     {id:'doc-1',name:'spesifikasi.pdf',type:'application/pdf',size:45000,url:'/api/v1/chat/attachments/doc-1'},
   ]
 }),
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
  const active=location.pathname.startsWith('/inbox/c-agent')?(contactsList.find(c=>c.id==='c-agent')||contact):contact
  return React.createElement(InboxView,{contacts:contactsList,agents,activeContact:active,initialMode:'chat'})
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
 if(req.url.startsWith('/inbox/work-fixture')||req.url.startsWith('/inbox/c-agent')||req.url.startsWith('/tasks/work/sessions/')||req.url.startsWith('/studio')){res.setHeader('Content-Type','text/html');res.end(await server.transformIndexHtml(req.url,'<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/__work_entry.jsx"></script></body></html>'))}
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
  const fixtureMessages=messages.map(m=>({...m}))
  const fixtureCannedResponses=[
   {id:'canned-halo',shortcut:'halo',title:'Salam Pembuka',content:'Halo! Ada yang bisa kami bantu?',category:'greeting',created_at:now,updated_at:now}
  ]
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
   else if(/\/conversations\/[^/]+\/messages\/[^/]+$/.test(pathname)&&route.request().method()==='PATCH'){
     const msgId=pathname.split('/').pop()
     let body={}
     try{body=route.request().postDataJSON()||{}}catch{}
     const newText=(body?.content||body?.message||'').trim()
     const target=fixtureMessages.find(m=>m.uuid===msgId||String(m.id)===msgId)
     if(!target)return route.fulfill({status:404,contentType:'application/json',body:JSON.stringify({error:'message not found'})})
     target.content=newText
     target.message=newText
     target.updated_at=new Date(Date.parse(now)+60000).toISOString()
     target.edited=true
     return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(target)})
   }
   else if(/\/conversations\/[^/]+\/messages\/[^/]+$/.test(pathname)&&route.request().method()==='DELETE'){
     const msgId=pathname.split('/').pop()
     const idx=fixtureMessages.findIndex(m=>m.uuid===msgId||String(m.id)===msgId)
     if(idx<0)return route.fulfill({status:404,contentType:'application/json',body:JSON.stringify({error:'message not found'})})
     fixtureMessages.splice(idx,1)
     return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({status:'deleted',id:msgId})})
   }
   else if(pathname.endsWith('/messages')){
     const match=pathname.match(/\/conversations\/([^/]+)\/messages/)
     const reqId=match?match[1]:null
     const matchedContact=reqId?(contactsList.find(c=>c.id===reqId||c.conversation_id===reqId)||contact):contact
     const activeRequests=reqId==='c-agent'?[{request_id:'req-typing-dots',message_id:'source',agent_id:'agent_cuan',state:'running',created_at:now,started_at:now,steps:[]}]:[]
     data={messages:fixtureMessages,tasks:workTaskOverride?[...earlierTasks,workTaskOverride]:allTasks,contact:matchedContact,requests:activeRequests}
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
   else if(pathname==='/api/v1/tasks/task-done-live'){
    const t0=new Date(Date.parse(now)-25000).toISOString()
    const t1=new Date(Date.parse(now)-20000).toISOString()
    const t2=new Date(Date.parse(now)-15000).toISOString()
    const t3=new Date(Date.parse(now)-10000).toISOString()
    const t4=new Date(Date.parse(now)-5000).toISOString()
    data={
      id:'task-done-live',title:'Investigasi & Perbaiki Pembacaan Lampiran Gambar di Chat',status:'done',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,
      commit_sha:'9c4d3f7466565284dbfa786e6a8c46ac93cab45c',
      created_at:t0,completed_at:now,updated_at:now,
      summary:'Pembacaan lampiran gambar di chat (termasuk jalur coba lagi) sudah diperbaiki dan diverifikasi.',
      complete_note:'Independent verification of commit 9c4d3f7466565284dbfa786e6a8c46ac93cab45c (read-only attempt): ran targeted Go tests which passed in 2.716s (saved in session:targeted_verification_tests.txt). Remaining gap: changes are committed locally at 9c4d3f7466565284dbfa786e6a8c46ac93cab45c and not yet pushed/merged to branch dev.\n\n🚀 Perubahan telah diverifikasi aktif pada rilis live (9c4d3f74).',
      latest_attempt:{id:'att-release',task_id:'task-done-live',kind:'native_release',status:'succeeded',agent_name:'agent_cuan',runner_id:'runner',started_at:t4,finished_at:now},
      attempts:[
        {id:'att-exec',task_id:'task-done-live',kind:'execution',status:'succeeded',attempt_number:1,agent_name:'agent_cuan',runner_id:'runner',started_at:t0,finished_at:t1},
        {id:'att-verify',task_id:'task-done-live',kind:'verification',status:'succeeded',attempt_number:2,agent_name:'agent_cuan',runner_id:'runner',started_at:t2,finished_at:t3},
        {id:'att-release',task_id:'task-done-live',kind:'native_release',status:'succeeded',attempt_number:3,agent_name:'agent_cuan',runner_id:'runner',started_at:t4,finished_at:now},
      ],
      activities:[
        {id:'done-act-1',task_id:'task-done-live',activity_type:'task_created',author:'operator',message:'Task created',created_at:t0},
        {id:'done-act-2',task_id:'task-done-live',attempt_id:'att-verify',activity_type:'runner_verification',author:'runner',message:'go test ./internal/server/... ok in 2.716s',created_at:t3},
        {id:'done-act-3',task_id:'task-done-live',attempt_id:'att-release',activity_type:'deployment_verified',author:'cuan-deploy',message:JSON.stringify({commit:'9c4d3f7466565284dbfa786e6a8c46ac93cab45c'}),created_at:now},
      ]
    }
   }
   else if(pathname.endsWith('/run-agent/history'))data={history:[],live_session:sessionStatus==='running'?{session_id:'fixture-session',attempt_id:'attempt',agent_id:'agent_cuan',started_at:now}:null}
   else if(pathname.endsWith('/logs'))data={logs:[{id:1,line:'Isolated terminal fixture output',stream:'stdout',logged_at:now}]}
   else if(pathname.includes('/agents'))data=pathname==='/api/v1/agents'?{agents}:agents[0]
   else if(pathname.includes('preferences'))data={items:{}}
   else if(pathname.includes('/chat/attachments/'))return route.fulfill({status:200,contentType:'image/png',body:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==','base64')})
   else if(pathname.endsWith('/projects'))data={projects:[]}
   else if(pathname.endsWith('/canned-responses')){
    const q=(new URL(route.request().url()).searchParams.get('q')||'').trim().toLowerCase()
    const filtered=q?fixtureCannedResponses.filter(c=>c.shortcut.toLowerCase().includes(q)||c.title.toLowerCase().includes(q)||c.content.toLowerCase().includes(q)):fixtureCannedResponses
    return route.fulfill({status:200,contentType:'application/json',headers:{'Cache-Control':'no-store'},body:JSON.stringify({canned_responses:filtered,total:filtered.length})})
   }
   else if(pathname.includes('/email')&&route.request().method()==='POST')return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({status:'success',email_id:'re_mock_fixture_123',delivery_status:'sent',recipient:'customer@example.test',message:'Email balasan berhasil dikirim'})})
   else if(pathname.endsWith('/cancel')&&route.request().method()==='POST')return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({status:'cancelled'})})
   else if(pathname.endsWith('/files')&&route.request().method()==='POST')return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({files:[{filename:'screenshot.png',path:'./screenshot.png',size:2048}]})})
   else if(pathname.endsWith('/tasks'))data={tasks:allTasks}
   else if(pathname.endsWith('/stream')||pathname.endsWith('/events')){
    const sseMatch=pathname.match(/\/conversations\/([^/]+)\//)
    const sseConvId=sseMatch?sseMatch[1]:null
    const sseRequests=sseConvId==='c-agent'?[{request_id:'req-typing-dots',message_id:'source',agent_id:'agent_cuan',state:'running',created_at:now,started_at:now,steps:[]}]:[]
    const snap={messages:fixtureMessages,tasks:workTaskOverride?[...earlierTasks,workTaskOverride]:allTasks,requests:sseRequests,system_notices:[{id:'system-notice-1',message:'Sistem baru saja diperbarui. Pekerjaan Anda tersimpan aman dan dilanjutkan secara otomatis...',created_at:now}]}
    return route.fulfill({status:200,contentType:'text/event-stream',body:`event: snapshot\ndata: ${JSON.stringify(snap)}\n\n`})
   }
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
    const inactiveBottomProjects = bottomNav.locator('[data-testid="bottom-nav-projects"]')
    await inactiveBottomProjects.waitFor({ state: 'visible' })
    assert.equal(await inactiveBottomProjects.getAttribute('aria-current'), null, 'Mobile projects must be inactive')
    await bottomNav.screenshot({ path: path.join(output, 'mobile-navigation-active-inbox.png') })
    await inactiveBottomProjects.hover()
    await page.waitForTimeout(100)
    await bottomNav.screenshot({ path: path.join(output, 'mobile-navigation-hover.png') })
  }
  if(width>=1200){
    const desktopNav = page.locator('nav.primary-navigation')
    await desktopNav.waitFor({ state: 'visible' })
    const activeInbox = desktopNav.locator('.nav-action[aria-current="page"]')
    await activeInbox.waitFor({ state: 'visible' })
    assert.equal(await activeInbox.getAttribute('data-nav-tone'), 'inbox', 'Active nav item must be inbox')
    assert.equal(await desktopNav.locator('.nav-action[data-nav-tone="tasks"]').count(), 0, 'Tasks must be removed from desktop primary navigation')
    const inactiveProjects = desktopNav.locator('.nav-action[data-nav-tone="projects"]')
    await inactiveProjects.waitFor({ state: 'visible' })
    assert.equal(await inactiveProjects.getAttribute('aria-current'), null, 'Projects must be inactive')
    await desktopNav.screenshot({ path: path.join(output, 'desktop-navigation-active-inbox.png') })
    await inactiveProjects.hover()
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
      assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Memeriksa hasil")').count()>=1,'Mobile list must show "Memeriksa hasil" for active substage matching task card')
      assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Siap ditinjau")').count()>=1,'Mobile list must show "Siap ditinjau" for review task')
      assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Menunggu antrean")').count()>=1,'Mobile list must show "Menunggu antrean" for queued task')
      const mobileQueuedItem = page.locator('[data-testid="conversation-item"]:has-text("Menunggu antrean")').first()
      assert.ok(await mobileQueuedItem.locator('[data-testid="workflow-status-indicator"] svg.lucide-clock3').count() >= 1, 'Mobile queued item must show neutral clock icon')
      assert.equal(await mobileQueuedItem.locator('circle[stroke-dasharray="2.5 1.5"]').count(), 0, 'Mobile queued item must not show dashed amber ring')
      const mobileMixedItem = page.locator('[data-testid="conversation-item"]:has-text("Perbaiki Validasi Respons Review")').first()
      assert.ok(await mobileMixedItem.locator(':text("Menunggu antrean")').count() >= 1, 'Mixed queued+review item must show "Menunggu antrean"')
      assert.ok(await mobileMixedItem.locator('[data-testid="workflow-status-indicator"] svg.lucide-clock3').count() >= 1, 'Mixed queued+review item must show neutral clock icon')
      assert.equal(await mobileMixedItem.locator('circle[stroke-dasharray="2.5 1.5"]').count(), 0, 'Mixed queued+review item must not show dashed amber ring')
      assert.equal(await mobileMixedItem.locator('svg.lucide-eye').count(), 0, 'Mixed queued+review item must not show purple eye icon')
      const mobileReviewItem = page.locator('[data-testid="conversation-item"]:has-text("Peringatan Kapasitas Server")').first()
      assert.ok(await mobileReviewItem.locator(':text("Siap ditinjau")').count() >= 1, 'Review-only item must show "Siap ditinjau"')
      assert.ok(await mobileReviewItem.locator('svg.lucide-eye').count() >= 1, 'Review-only item must show eye icon')
      assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Biar kami aktifkan")').count()>=1,'Mobile list must fall back to last_message for idle contact')
      // Verify system lifecycle notice unread prevention:
      const bottomNav = page.locator('nav.bottom-navigation')
      assert.equal(await bottomNav.locator('[data-testid="inbox-unread-badge-bottom"]').count(), 0, 'Mobile bottom nav must not show unread badge for system notices')
      const unresolvedTab = page.locator('button:has-text("Belum tuntas"), button:has-text("Unresolved")').first()
      if (await unresolvedTab.count() > 0 && await unresolvedTab.isVisible()) {
        await unresolvedTab.click()
        await page.waitForTimeout(200)
        assert.equal(await page.locator('[data-testid="conversation-item"]:has-text("Rotasi Dinamis Kata Status")').count(), 0, 'Belum tuntas tab must exclude system notice conversations')
        await page.screenshot({ path: path.join(output, 'inbox-mobile-unresolved-filtered-390.png') })
        const allTab = page.locator('button:has-text("Semua"), button:has-text("All")').first()
        if (await allTab.count() > 0 && await allTab.isVisible()) {
          await allTab.click()
          await page.waitForTimeout(200)
        }
      }
      const sysNoticeItem = page.locator('[data-testid="conversation-item"]:has-text("Rotasi Dinamis Kata Status")').first()
      if (await sysNoticeItem.count() > 0) {
        assert.equal(await sysNoticeItem.locator('[data-testid="unread-micro-dot"]').count(), 0, 'System notice contact must not have unread micro dot')
      }
      await page.screenshot({path:path.join(output,'inbox-mobile-contacts-390-light.png')})
      await page.evaluate(()=>document.documentElement.classList.add('dark'))
      await page.screenshot({path:path.join(output,'inbox-mobile-contacts-390-dark.png')})
      await page.evaluate(()=>document.documentElement.classList.remove('dark'))
      await page.locator('[data-testid="conversation-item"]:visible').first().click()
      await page.waitForTimeout(300)
    }
    const bottomNav = page.locator('.bottom-navigation')
    await bottomNav.waitFor({ state: 'visible' })
    assert.equal(await bottomNav.locator('[data-testid="bottom-nav-tasks"]').count(), 0, 'Tasks must be removed from mobile bottom navigation')
    const navItems = await page.$$eval('.bottom-navigation > button, .bottom-navigation > div', els =>
      els.map(el => ({
        testId: el.getAttribute('data-testid'),
        label: (el.textContent || '').trim(),
        x: el.getBoundingClientRect().x,
        width: el.getBoundingClientRect().width,
        height: el.getBoundingClientRect().height,
      }))
    )
    assert.equal(navItems.length, 5, 'Bottom navigation must have exactly 5 slots')
    assert.equal(navItems[0].testId, 'bottom-nav-inbox', 'Slot 1 must be Inbox')
    assert.equal(navItems[1].testId, 'bottom-nav-projects', 'Slot 2 must be Projects')
    assert.equal(navItems[2].testId, 'bottom-nav-studio', 'Slot 3 must be New work (+)')
    assert.equal(navItems[3].testId, 'bottom-nav-contacts', 'Slot 4 must be Contacts')
    assert.equal(navItems[4].testId, 'bottom-nav-settings', 'Slot 5 must be Settings')
    assert.deepEqual(navItems.map(i => i.label), ['Inbox', 'Project', 'New work', 'Kontak', 'Pengaturan'], 'Mobile bottom nav labels must be consistent in Indonesian')
    assert.ok(navItems[0].x < navItems[1].x, 'Inbox must be before Projects')
    assert.ok(navItems[1].x < navItems[2].x, 'Projects must be before New work (+)')
    assert.ok(navItems[2].x < navItems[3].x, 'New work (+) must be before Contacts')
    assert.ok(navItems[3].x < navItems[4].x, 'Contacts must be before Settings')
    for (const item of navItems) {
      assert.ok(item.height >= 56, `Touch target height must be >= 56px (min-h-14), got ${item.height}`)
    }
    const studioAccent = bottomNav.locator('[data-testid="bottom-nav-studio"] [data-action-trigger="true"]')
    await studioAccent.waitFor({ state: 'visible' })
    const accentBox = await studioAccent.boundingBox()
    assert.ok(accentBox && accentBox.width >= 28 && accentBox.width <= 36, `Studio circle accent width should be between 28px and 36px, got ${accentBox?.width}`)
    assert.ok(accentBox && accentBox.height >= 28 && accentBox.height <= 36, `Studio circle accent height should be between 28px and 36px, got ${accentBox?.height}`)
    const borderRadius = await studioAccent.evaluate(el => window.getComputedStyle(el).borderRadius)
    assert.ok(borderRadius.includes('9999px') || parseInt(borderRadius) >= 14, `Studio accent must be circular, got ${borderRadius}`)
    await page.screenshot({ path: path.join(output, 'mobile-390-bottom-nav-symmetrical.png') })
  }
  if(width>=1200){
    const desktopNav = page.locator('nav.primary-navigation')
    assert.ok(await page.locator('[data-avatar="true"]:has-text("FP")').count()>=1,'Desktop sidebar must display FP monogram')
    assert.ok(await page.locator('[data-avatar="true"]:has-text("KI")').count()>=1,'Desktop sidebar must display KI monogram')
    assert.ok(await page.locator('[data-avatar="true"]:has-text("AZ")').count()>=1,'Desktop sidebar must display AZ monogram')
    assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Sedang dikerjakan")').count()>=1,'Desktop list must show "Sedang dikerjakan" for active running task')
    assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Menunggu antrean")').count()>=1,'Desktop list must show "Menunggu antrean" for queued task')
    const desktopQueuedItem = page.locator('[data-testid="conversation-item"]:has-text("Menunggu antrean")').first()
    assert.ok(await desktopQueuedItem.locator('[data-testid="workflow-status-indicator"] svg.lucide-clock3').count() >= 1, 'Desktop queued item must show neutral clock icon')
    assert.equal(await desktopQueuedItem.locator('circle[stroke-dasharray="2.5 1.5"]').count(), 0, 'Desktop queued item must not show dashed amber ring')
    assert.ok(await page.locator('[data-testid="conversation-item"]:has-text("Biar kami aktifkan")').count()>=1,'Desktop list must fall back to last_message for idle contact')
    assert.equal(await desktopNav.locator('[data-testid="inbox-unread-badge-rail"]').count(), 0, 'Desktop navigation rail must have no unread badge for system notices')
    await page.screenshot({path:path.join(output,`inbox-desktop-monogram-${width}.png`)})
  }
  // Tasks live in the composer disclosure, not as duplicate chat cards.
  const tasks=page.locator('.composer-tasks > div > button[aria-expanded]')
  await tasks.waitFor({state:'visible'})
  assert.equal(await tasks.getAttribute('aria-expanded'),'false')
  assert.equal(await tasks.getAttribute('data-status'),'working','Header bar badge must reflect latest running task even when an older task needs attention')
  assert.equal(await tasks.locator('[data-status-dot="working"]').count(),1,'Header bar must show working dot next to running latest task title')
  assert.equal(await tasks.locator('svg.lucide-circle-alert, svg.lucide-alert-circle').count(),0,'Header bar must not show red alert icon when latest task is working')
  const assignedBtn=page.locator('[data-testid="assigned-task-btn"]').first()
  await assignedBtn.waitFor({state:'visible'})
  assert.equal(await assignedBtn.locator('[data-status-dot="working"]').count(),1,'Assigned task button must show pulsing working dot while task is running')
  assert.ok((await assignedBtn.innerText()).includes('Sedang dikerjakan tim'),'Assigned task button must show Sedang dikerjakan tim while running')
  await assignedBtn.scrollIntoViewIfNeeded()
  await page.screenshot({path:path.join(output,`assigned-task-running-${width}.png`)})
  assert.equal(await page.locator('.inbox-message-list [data-card="task-card"]').count(),0)
  assert.equal(await page.locator('.inbox-message-list').getByText('Sistem baru saja diperbarui').count(), 0, 'System update notice must NOT appear as a chat bubble in conversation timeline')
  const systemToast = page.locator('[data-testid="system-update-toast"]')
  await systemToast.waitFor({ state: 'visible', timeout: 5000 })
  assert.ok((await systemToast.innerText()).includes('Sistem baru saja diperbarui'), 'System update notice must appear as a temporary toast')
  const toastBox = await systemToast.boundingBox()
  assert.ok(toastBox && toastBox.x >= 0 && toastBox.x + toastBox.width <= width, `System toast must fit within ${width}px viewport`)
  await page.screenshot({path:path.join(output,`inbox-timeline-notice-${width}.png`)})
  if (width === 390) {
    await systemToast.waitFor({ state: 'hidden', timeout: 8000 })
  }
  const collapsedChevronBox=await tasks.locator('svg').last().boundingBox()
  await page.screenshot({path:path.join(output,`composer-tasks-collapsed-${width}.png`)})
  const initialChatScroll=await page.locator('.inbox-message-list').evaluate(el=>el.scrollTop)
  await tasks.click()
  assert.equal(await tasks.getAttribute('aria-expanded'),'true')
  const afterOpenChatScroll=await page.locator('.inbox-message-list').evaluate(el=>el.scrollTop)
  assert.ok(Math.abs(afterOpenChatScroll - initialChatScroll) <= 2, 'Main chat history scroll must remain undisturbed')
  const drawer = page.locator('[data-testid="composer-tasks-drawer"]')
  await drawer.waitFor({state:'visible'})
  const panelEl=page.locator('.composer-tasks-panel')
  const card=page.locator('.composer-tasks-panel [data-timeline-item="work"]')
  try{await card.waitFor({state:'visible',timeout:20000})}catch(error){await page.screenshot({path:path.join(output,'failed-'+width+'.png')});console.log(JSON.stringify({errors,body:(await page.locator('body').innerText()).slice(0,1500)}));throw error}
  assert.equal(await card.count(),1)
  const cardBox=await card.boundingBox()
  const panelBox=await panelEl.boundingBox()
  const drawerBox = await drawer.boundingBox()
  assert.ok(cardBox && panelBox && drawerBox)
  assert.ok(cardBox.y >= panelBox.y - 2, 'Newest task must be at top within visible panel height')
  assert.ok(cardBox.y + cardBox.height <= panelBox.y + panelBox.height + 2, 'Newest task must not be clipped')
  const collapseBtn=page.locator('[data-testid="composer-tasks-collapse"]')
  await collapseBtn.waitFor({state:'visible'})
  if (width >= 1200) {
    const chatBox = await page.locator('.inbox-message-list').boundingBox()
    assert.ok(drawerBox.x > chatBox.x, 'Drawer opens as right panel on desktop without layout shift on chat')
  } else if (width === 390) {
    assert.equal(Math.round(drawerBox.width), 390, 'Drawer opens as full-width sheet on mobile')
  }
  const reviewCard = page.locator('.composer-tasks-panel [data-timeline-item="task-review"]')
  await reviewCard.waitFor({state:'visible'})
  assert.equal(await reviewCard.locator('svg.lucide-eye').count(), 1, 'Task in review must display Eye icon')
  const attentionCard = page.locator('.composer-tasks-panel [data-timeline-item="task-attention"]')
  await attentionCard.waitFor({state:'visible'})
  assert.equal(await attentionCard.locator('svg.lucide-circle-alert, svg.lucide-alert-circle').count(), 1, 'Stopped/failed task in drawer must display AlertCircle icon')
  const attentionText = await attentionCard.innerText()
  assert.ok(!attentionText.includes('Sedang dikerjakan'), 'Stopped/failed task with red ! icon must not show Sedang dikerjakan label')
  assert.ok(attentionText.includes('Belum selesai'), 'Stopped/failed task must show Belum selesai label matching its icon')
  const doneLiveCard = page.locator('.composer-tasks-panel [data-timeline-item="task-done-live"]')
  await doneLiveCard.waitFor({state:'visible'})
  const doneLiveText = await doneLiveCard.innerText()
  assert.ok(!doneLiveText.includes('not yet pushed/merged'), 'Done live task in Task obrolan must not show stale pre-publication gap text')
  assert.ok(doneLiveText.includes('Pembacaan lampiran gambar di chat'), 'Done live task in Task obrolan must show clean summary')
  assert.ok(doneLiveText.includes('Perubahan telah diverifikasi aktif pada rilis live'), 'Done live task in Task obrolan must show live release confirmation')
  assert.equal(await reviewCard.locator('svg.lucide-check-check').count(), 0, 'Task in review must not display double checkmark')
  assert.equal(await page.locator('.composer-tasks-panel svg.lucide-check-check').count(), 0, 'No task in review should use check-check')
  const connectorMetrics = await page.evaluate(() => {
    const item = document.querySelector('.composer-tasks-panel [data-timeline-item]')
    const connector = item?.querySelector('[data-timeline-connector]')
    const iconWrap = item?.querySelector('span.h-4.w-4')
    if (!connector || !iconWrap) return null
    const cRect = connector.getBoundingClientRect()
    const iRect = iconWrap.getBoundingClientRect()
    const style = getComputedStyle(connector)
    return {
      width: style.width,
      rectWidth: cRect.width,
      connectorCenterX: cRect.left + cRect.width / 2,
      iconCenterX: iRect.left + iRect.width / 2,
      backgroundColor: style.backgroundColor,
    }
  })
  assert.ok(connectorMetrics, 'Timeline connector and status icon must exist in ComposerTasks')
  assert.equal(connectorMetrics.width, '1px', `Timeline connector must be 1px hairline, got ${connectorMetrics.width}`)
  assert.ok(
    Math.abs(connectorMetrics.connectorCenterX - connectorMetrics.iconCenterX) <= 1,
    `Timeline connector center (${connectorMetrics.connectorCenterX}) must align with status icon center (${connectorMetrics.iconCenterX})`
  )
  await page.screenshot({path:path.join(output,`composer-tasks-expanded-${width}.png`)})
  if (width === 1440 || width === 390) {
    await page.evaluate(() => document.documentElement.classList.add('dark'))
    await page.waitForTimeout(100)
    const darkConnector = await page.evaluate(() => {
      const connector = document.querySelector('.composer-tasks-panel [data-timeline-connector]')
      if (!connector) return null
      const style = getComputedStyle(connector)
      return { width: style.width, backgroundColor: style.backgroundColor }
    })
    assert.ok(darkConnector && darkConnector.width === '1px', `Dark mode timeline connector must be 1px, got ${darkConnector?.width}`)
    await page.screenshot({path:path.join(output,`composer-tasks-expanded-dark-${width}.png`)})
    await page.evaluate(() => document.documentElement.classList.remove('dark'))
  }
  const drawerScroll = drawer.locator('[data-drawer-scroll]')
  await drawerScroll.waitFor({ state: 'visible' })
  const scrollMetrics = await page.evaluate(() => {
    const scroller = document.querySelector('[data-testid="composer-tasks-drawer"] [data-drawer-scroll]')
    const inner = document.querySelector('.composer-tasks-panel')
    if (!scroller || !inner) return null
    const scrollerStyle = getComputedStyle(scroller)
    const innerStyle = getComputedStyle(inner)
    return {
      initialTop: scroller.scrollTop,
      scrollHeight: scroller.scrollHeight,
      clientHeight: scroller.clientHeight,
      scrollerOverflowY: scrollerStyle.overflowY,
      innerOverflowY: innerStyle.overflowY,
    }
  })
  assert.ok(scrollMetrics, 'Drawer scroll container and inner panel must exist')
  assert.equal(scrollMetrics.initialTop, 0, 'Drawer scroll must start at top (0)')
  assert.ok(scrollMetrics.scrollHeight > scrollMetrics.clientHeight, `Drawer must be scrollable at width ${width} (${scrollMetrics.scrollHeight} > ${scrollMetrics.clientHeight})`)
  assert.ok(['auto', 'scroll'].includes(scrollMetrics.scrollerOverflowY), 'SlideDrawer body must own overflow-y')
  assert.ok(!['auto', 'scroll'].includes(scrollMetrics.innerOverflowY), 'composer-tasks-panel must not create a nested scroll container')
  await drawerScroll.evaluate((el) => { el.scrollTop = el.scrollHeight })
  await page.waitForTimeout(100)
  const bottomScroll = await drawerScroll.evaluate((el) => ({
    scrollTop: el.scrollTop,
    clientHeight: el.clientHeight,
    scrollHeight: el.scrollHeight,
  }))
  assert.ok(bottomScroll.scrollTop > 0, `Drawer must scroll down at width ${width}`)
  assert.ok(bottomScroll.scrollTop + bottomScroll.clientHeight >= bottomScroll.scrollHeight - 2, `Drawer must reach bottom at width ${width}`)
  await page.screenshot({path:path.join(output,`composer-tasks-scrolled-bottom-${width}.png`)})
  await drawerScroll.evaluate((el) => { el.scrollTop = 0 })
  assert.equal(await drawerScroll.evaluate((el) => el.scrollTop), 0, 'Drawer must scroll back to top cleanly')
  assert.equal(await page.getByText('Files from this work session',{exact:true}).count(),0)
  assert.equal(await card.locator('[data-timeline-trigger]').count(), 0, 'Linear progress timeline eliminates nested accordion triggers')
  assert.equal(await page.locator('[data-testid="composer-tasks-back"]').count(), 0, 'Inline back button must be eliminated')
  await card.getByText(baseTask.latest_progress,{exact:true}).waitFor()
  // Open Rincian for task-done-live to verify permanent Pengerjaan, Verifikasi, and Push & Rilis Live timeline items
  await doneLiveCard.getByRole('button', { name: /Rincian|Details/i }).click()
  const donePanel = page.locator('[data-testid="inbox-work-panel"]')
  await donePanel.waitFor({ state: 'visible' })
  await donePanel.locator('[data-work-timeline]').waitFor({ state: 'visible' })
  const donePanelText = await donePanel.innerText()
  assert.ok(!donePanelText.includes('not yet pushed/merged'), 'Rincian page must not show stale pre-publication gap text')
  assert.ok(donePanelText.includes('Pengerjaan selesai') || donePanelText.includes('Execution finished'), 'Rincian timeline must show permanent Pengerjaan stage')
  assert.ok(donePanelText.includes('Verifikasi selesai') || donePanelText.includes('Verification finished'), 'Rincian timeline must show permanent Verifikasi stage')
  assert.ok(donePanelText.includes('Push & rilis live selesai') || donePanelText.includes('Push & live release finished'), 'Rincian timeline must show permanent Push & Rilis Live stage')
  await page.screenshot({ path: path.join(output, `rincian-done-live-${width}.png`) })
  await donePanel.locator('[data-work-back]').first().click()
  await donePanel.waitFor({ state: 'hidden' })
  if (await collapseBtn.isVisible()) {
    await collapseBtn.click()
    await drawer.waitFor({state:'hidden'})
  }
  assert.equal(await tasks.getAttribute('aria-expanded'),'false')
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
  const choices=page.getByTestId('prompt-choice-chips')
  await choices.scrollIntoViewIfNeeded()
  await choices.waitFor({state:'visible'})
  assert.equal(await choices.getByRole('button',{name:'Lanjutkan pemeriksaan',exact:true}).count(),1)
  assert.equal(await choices.getByRole('button',{name:'Sesuaikan kebutuhan',exact:true}).count(),1)
  for(const button of await choices.getByRole('button').all()) {
   assert.ok((await button.boundingBox()).height>=44)
   const style=await button.evaluate(el=>({radius:getComputedStyle(el).borderRadius,background:getComputedStyle(el).backgroundColor}))
   assert.equal(style.radius,'0px')
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

  // Multi-image gallery verification (DoD 1, 2, 3, 4)
  const gallery = page.locator('[data-testid="attachment-gallery"]')
  await gallery.first().waitFor({state:'visible',timeout:10000})
  assert.ok(await gallery.count() >= 2, 'AttachmentGallery must be rendered for multi-image messages')
  assert.equal(await page.locator('[data-testid="attachment-gallery"][data-image-count="4"]').count(), 1, '4-image gallery must be rendered in 2x2 grid')
  assert.equal(await page.locator('[data-testid="attachment-gallery"][data-image-count="2"]').count(), 1, '2-image gallery must be rendered in 2-column grid')

  // Verify non-image document alongside gallery
  const docAttachment = page.locator('.inbox-message-list button[aria-label*="spesifikasi.pdf"]')
  await docAttachment.waitFor({state:'visible'})
  assert.equal(await docAttachment.count(), 1, 'Non-image doc must remain accessible alongside gallery')
  const mixedGallery = page.locator('[data-testid="attachment-gallery"][data-image-count="2"]')
  await mixedGallery.scrollIntoViewIfNeeded()
  await page.screenshot({path:path.join(output,`inbox-gallery-mixed-${width}.png`)})

  // Verify tap/click on image cell opens FilePreviewModal
  const galleryCell = page.locator('[data-testid="attachment-gallery"] button').first()
  await galleryCell.scrollIntoViewIfNeeded()
  await page.screenshot({path:path.join(output,`inbox-gallery-grid-${width}.png`)})
  await galleryCell.click()
  const previewDialog = page.locator('dialog[open]')
  await previewDialog.waitFor({state:'visible',timeout:5000})
  assert.equal(await previewDialog.count(), 1, 'Clicking gallery cell must open FilePreviewModal')
  await page.screenshot({path:path.join(output,`inbox-gallery-preview-modal-${width}.png`)})
  const closeBtn = previewDialog.locator('button[aria-label="Tutup"], button[aria-label="Close"]').first()
  await closeBtn.click()
  await previewDialog.waitFor({state:'hidden'})

  // Frame the actual choices with the work preview in native Inbox captures.
  await choices.scrollIntoViewIfNeeded()
  const previousScroll=await list.evaluate(el=>el.scrollTop)
  await page.screenshot({path:path.join(output,`inbox-${width}.png`)})
  if (!(await card.isVisible())) {
    await tasks.click()
    await card.waitFor({state:'visible'})
  }
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
  if (!(await card.isVisible())) {
    await tasks.click()
    await card.waitFor({state:'visible'})
  }
  await card.getByRole('button',{name:/^(Details|Detail|Rincian)$/}).click()
  await panel.waitFor({state:'visible'})
  await panel.locator('[data-work-back]').click()
  await panel.waitFor({state:'detached'})
  if (!(await reviewCard.isVisible())) {
    await tasks.click()
    await reviewCard.waitFor({state:'visible'})
  }
  const reviewDetailsBtn = reviewCard.getByRole('button',{name:/^(Details|Detail|Rincian)$/})
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
  // Verify real-time status transition on assigned-task-btn from running (pulsing) to done (calm)
  workTaskOverride={...baseTask,status:'done',latest_attempt:{...attempt,status:'succeeded',finished_at:now}}
  await page.evaluate(()=>window.dispatchEvent(new Event('focus')))
  await page.waitForFunction(()=>{
    const btn=document.querySelector('[data-testid="assigned-task-btn"]')
    return btn && !btn.querySelector('[data-status-dot="working"]') && btn.textContent?.includes('Selesai')
  },null,{timeout:7000})
  assert.equal(await assignedBtn.locator('[data-status-dot="working"]').count(),0,'Pulsing dot must stop when task finishes')
  assert.equal(await assignedBtn.locator('svg.lucide-circle-check, svg.lucide-check-circle-2').count(),1,'Completed assigned task button must show calm check icon')
  await assignedBtn.scrollIntoViewIfNeeded()
  await page.screenshot({path:path.join(output,`assigned-task-done-${width}.png`)})
  workTaskOverride=null
  await page.evaluate(()=>window.dispatchEvent(new Event('focus')))
  await page.waitForFunction(()=>{
    const btn=document.querySelector('[data-testid="assigned-task-btn"]')
    return btn && Boolean(btn.querySelector('[data-status-dot="working"]'))
  },null,{timeout:7000})
  if (!(await card.isVisible())) {
    await tasks.click()
    await card.waitFor({state:'visible'})
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
  // Verify TerminalComposer Quick Reply button beside Mic/Send, popover, search, insert without auto-send, and `/` slash commands
  const qrTrigger = page.locator('[data-testid="terminal-quick-reply-trigger"]')
  await qrTrigger.waitFor({state:'visible',timeout:5000})
  const qrInput = page.locator('#terminal_composer_input')
  const qrSendBtn = page.locator('[aria-label="Terminal input composer"] button.composer-send')
  const [qrTriggerBox, qrInputBox, qrSendBox] = await Promise.all([
    qrTrigger.boundingBox(),
    qrInput.boundingBox(),
    qrSendBtn.boundingBox(),
  ])
  assert.ok(qrTriggerBox && qrInputBox && qrSendBox, 'TerminalComposer controls must have bounding boxes')
  assert.ok(qrInputBox.width >= 150, `Terminal input width (${qrInputBox.width}px) must stay comfortable on ${width}px`)
  assert.ok(qrInputBox.x + qrInputBox.width <= qrTriggerBox.x + 2, 'Quick reply button must be to the right of input')
  assert.ok(qrTriggerBox.x + qrTriggerBox.width <= qrSendBox.x + 2, 'Quick reply button must be immediately beside Mic/Send button without overlap')
  await qrTrigger.click()
  const qrPopover = page.locator('[data-testid="terminal-quick-reply-popover"]')
  await qrPopover.waitFor({state:'visible',timeout:5000})
  await page.screenshot({path:path.join(output,`terminal-quick-reply-popover-${width}.png`)})
  await page.locator('[data-testid="terminal-quick-reply-search"]').fill('test')
  const beforeQrText = await qrInput.inputValue()
  const qrFirstItem = page.locator('[data-testid="terminal-quick-reply-item"]').first()
  await qrFirstItem.waitFor({state:'visible',timeout:5000})
  const expectedText = await qrFirstItem.locator('p').innerText()
  await qrFirstItem.click()
  await qrPopover.waitFor({state:'hidden',timeout:5000})
  const expectedAfterQr = beforeQrText.trim() ? `${beforeQrText.trimEnd()} ${expectedText}` : expectedText
  assert.equal(await qrInput.inputValue(), expectedAfterQr, 'Selecting a quick reply must insert its text without auto-sending')
  const isFocused = await qrInput.evaluate((el) => document.activeElement === el)
  assert.equal(isFocused, true, 'Selecting a quick reply must focus the terminal input')
  // Verify `/` slash commands still open without conflict, then restore
  await qrInput.fill('/')
  await page.getByText('Slash Commands', {exact:true}).waitFor({state:'visible',timeout:5000})
  await qrInput.fill(beforeQrText)
  // Test Stop/Cancel session modal
  const cancelButton = page.locator("button[aria-label=\"Stop session\"], button[aria-label=\"Cancel session\"], button[aria-label=\"Hentikan sesi\"]")
  await cancelButton.waitFor({state: "visible", timeout: 5000})
  await cancelButton.click()
  const dialog = page.locator("dialog[open]")
  await dialog.waitFor({state: "visible", timeout: 5000})
  const modalHeading = dialog.locator("h2")
  await modalHeading.waitFor({state: "visible", timeout: 5000})
  const keepRunning = dialog.locator("button.action-secondary")
  const confirmCancel = dialog.locator("button.action-primary")
  assert.ok(await keepRunning.isVisible(), "Keep running button must be visible")
  assert.ok(await confirmCancel.isVisible(), "Stop/Cancel session button must be visible")
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

  // Verify circular TypingDots & bubble-contained InboxActivity (light & dark mode)
  await page.goto(new URL('/inbox/c-agent', origin).href, { waitUntil: 'domcontentloaded' })
  const activityBubble = page.locator('[data-response-state="running"].wa-message-bubble.wa-incoming[data-bubble-tail="incoming"]')
  await activityBubble.waitFor({ state: 'visible', timeout: 10000 })
  await activityBubble.scrollIntoViewIfNeeded()
  const typingDots = activityBubble.getByTestId('inbox-activity-typing-dots')
  await typingDots.waitFor({ state: 'visible', timeout: 5000 })
  const stopReplyBtn = activityBubble.getByRole('button', { name: /Hentikan respons|Stop reply/i })
  await stopReplyBtn.waitFor({ state: 'visible', timeout: 5000 })
  const detailBtn = activityBubble.getByRole('button', { name: /Detail/i })
  await detailBtn.waitFor({ state: 'visible', timeout: 5000 })
  const activityTextEl = activityBubble.getByTestId('inbox-activity-text')
  await activityTextEl.waitFor({ state: 'visible', timeout: 5000 })
  const bubbleBox = await activityBubble.boundingBox()
  assert.ok(bubbleBox && bubbleBox.width >= 270, `Assistant progress bubble width must be >= 270px, got ${bubbleBox?.width}`)
  if (width >= 1200) {
    assert.ok(bubbleBox.width < width * 0.85, `Assistant progress bubble on desktop must not span full width, got ${bubbleBox.width}px`)
  }
  for (const [name, loc] of [['typingDots', typingDots], ['activityText', activityTextEl], ['detailBtn', detailBtn], ['stopReplyBtn', stopReplyBtn]]) {
    const elBox = await loc.boundingBox()
    assert.ok(
      elBox &&
        elBox.x >= bubbleBox.x - 1 &&
        elBox.x + elBox.width <= bubbleBox.x + bubbleBox.width + 1 &&
        elBox.y >= bubbleBox.y - 1 &&
        elBox.y + elBox.height <= bubbleBox.y + bubbleBox.height + 1,
      `${name} must be contained inside assistant chat bubble`,
    )
  }
  const dotStyles = await typingDots.locator('span').evaluateAll(els =>
    els.map(el => {
      const s = getComputedStyle(el)
      return {
        borderRadius: s.borderRadius,
        width: s.width,
        height: s.height,
        animationName: s.animationName,
      }
    })
  )
  assert.equal(dotStyles.length, 3, 'TypingDots must render 3 dot spans')
  for (const d of dotStyles) {
    assert.equal(d.borderRadius, '9999px', `Typing dot must be circular (9999px), got ${d.borderRadius}`)
    assert.equal(d.width, d.height, `Typing dot must be equal width and height, got ${d.width}x${d.height}`)
  }
  await page.evaluate(() => document.documentElement.classList.remove('dark'))
  await page.waitForTimeout(100)
  await page.screenshot({ path: path.join(output, `typing-dots-${width}-light.png`) })
  await page.evaluate(() => document.documentElement.classList.add('dark'))
  await page.waitForTimeout(100)
  await page.screenshot({ path: path.join(output, `typing-dots-${width}-dark.png`) })

  // Verify message context menu UI, Edit message, Delete message, and non-owner restrictions
  await page.evaluate(() => document.documentElement.classList.remove('dark'))
  await page.goto(new URL('/inbox/work-fixture', origin).href, { waitUntil: 'domcontentloaded' })
  const userMsgRow = page.locator('[data-message-id="reply-out"]')
  await userMsgRow.waitFor({ state: 'visible', timeout: 10000 })
  await userMsgRow.scrollIntoViewIfNeeded()

  // Open context menu on user-owned message (via right-click / contextmenu so it works identically on desktop & touch)
  await userMsgRow.locator('.wa-message-bubble').click({ button: 'right' })
  const menuPanel = page.locator('[data-item-menu-panel][role="menu"]')
  await menuPanel.waitFor({ state: 'visible', timeout: 5000 })
  const menuMetrics = await menuPanel.evaluate(el => {
    const s = getComputedStyle(el)
    const items = Array.from(el.querySelectorAll('[role="menuitem"]'))
    const firstItemRect = items[0]?.getBoundingClientRect()
    return {
      borderRadius: s.borderRadius,
      boxShadow: s.boxShadow,
      itemHeight: firstItemRect ? firstItemRect.height : 0,
      labels: items.map(i => i.textContent?.trim() || ''),
    }
  })
  assert.notEqual(menuMetrics.borderRadius, '0px', `Context menu must have rounded corners, got ${menuMetrics.borderRadius}`)
  assert.notEqual(menuMetrics.boxShadow, 'none', 'Context menu must have elevation shadow')
  assert.ok(menuMetrics.itemHeight >= 34 && menuMetrics.itemHeight <= 42, `Context menu item height should be compact & touch-friendly (34-42px), got ${menuMetrics.itemHeight}px`)
  assert.ok(menuMetrics.labels.includes('Edit pesan'), `User message menu must include 'Edit pesan', got ${JSON.stringify(menuMetrics.labels)}`)
  assert.ok(menuMetrics.labels.includes('Hapus pesan'), `User message menu must include 'Hapus pesan', got ${JSON.stringify(menuMetrics.labels)}`)
  await page.screenshot({ path: path.join(output, `message-context-menu-${width}.png`) })

  // Edit user message
  await menuPanel.getByRole('menuitem', { name: 'Edit pesan' }).click()
  const editTextarea = userMsgRow.locator('[data-message-edit-form] textarea')
  await editTextarea.waitFor({ state: 'visible', timeout: 5000 })
  await editTextarea.fill('Sudut bubble chat diperbarui dan telah diedit.')
  await userMsgRow.locator('[data-message-edit-form]').getByRole('button', { name: /Simpan|Save/i }).click()
  await userMsgRow.getByText('Sudut bubble chat diperbarui dan telah diedit.').waitFor({ state: 'visible', timeout: 5000 })
  await userMsgRow.getByText('(diedit)').waitFor({ state: 'visible', timeout: 5000 })

  // Verify edit persists across reload
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.locator('[data-message-id="reply-out"]').getByText('Sudut bubble chat diperbarui dan telah diedit.').waitFor({ state: 'visible', timeout: 10000 })
  await page.locator('[data-message-id="reply-out"]').getByText('(diedit)').waitFor({ state: 'visible', timeout: 5000 })

  // Verify agent and system messages do NOT show Edit pesan / Hapus pesan
  for (const nonOwnerId of ['proposal-pending', 'room-ref']) {
    const nonOwnerRow = page.locator(`[data-message-id="${nonOwnerId}"]`)
    await nonOwnerRow.scrollIntoViewIfNeeded()
    await nonOwnerRow.locator('.wa-message-bubble').click({ button: 'right' })
    await menuPanel.waitFor({ state: 'visible', timeout: 5000 })
    const nonOwnerLabels = await menuPanel.locator('[role="menuitem"]').allTextContents()
    assert.ok(!nonOwnerLabels.includes('Edit pesan'), `Non-owner message ${nonOwnerId} must NOT have Edit pesan`)
    assert.ok(!nonOwnerLabels.includes('Hapus pesan'), `Non-owner message ${nonOwnerId} must NOT have Hapus pesan`)
    await page.keyboard.press('Escape')
    await menuPanel.waitFor({ state: 'hidden', timeout: 5000 })
  }

  // Delete user message with confirmation and verify persistence across reload
  const userMsgAfterReload = page.locator('[data-message-id="reply-out"]')
  await userMsgAfterReload.scrollIntoViewIfNeeded()
  await userMsgAfterReload.locator('.wa-message-bubble').click({ button: 'right' })
  await menuPanel.waitFor({ state: 'visible', timeout: 5000 })
  await menuPanel.getByRole('menuitem', { name: 'Hapus pesan' }).click()
  const confirmDeleteBtn = page.locator('[data-confirm-delete-message]')
  await confirmDeleteBtn.waitFor({ state: 'visible', timeout: 5000 })
  await confirmDeleteBtn.click()
  await userMsgAfterReload.waitFor({ state: 'detached', timeout: 5000 })
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.locator('[data-message-id="proposal-pending"]').waitFor({ state: 'visible', timeout: 10000 })
  assert.equal(await page.locator('[data-message-id="reply-out"]').count(), 0, 'Deleted message must remain deleted after reload')

  // Verify Quick Reply (Canned Responses) Zap picker & cross-device slash autocomplete sync on mobile & desktop
  const cannedTrigger = page.locator('#compose-panel button[aria-haspopup="dialog"]')
  await cannedTrigger.waitFor({ state: 'visible', timeout: 5000 })
  await cannedTrigger.click()
  await page.getByText('/halo').waitFor({ state: 'visible', timeout: 5000 })
  await page.screenshot({ path: path.join(output, `quick-reply-popover-${width}.png`) })
  await page.keyboard.press('Escape')
  await page.getByText('/halo').waitFor({ state: 'hidden', timeout: 5000 })

  // Simulate adding a template from another device (e.g. phone) while this tab stays open
  fixtureCannedResponses.unshift({
    id: 'canned-promo-hp',
    shortcut: 'promo-hp',
    title: 'Promo dari HP',
    content: 'Halo! Ini promo terbaru dari HP.',
    category: 'sales',
    created_at: now,
    updated_at: new Date(Date.parse(now) + 120000).toISOString(),
  })
  const quickReplyComposer = page.locator('[data-primary-composer]')
  await quickReplyComposer.fill('/promo')
  const slashMatchBtn = page.locator('#compose-panel button', { hasText: '/promo-hp' })
  await slashMatchBtn.waitFor({ state: 'visible', timeout: 5000 })
  await page.screenshot({ path: path.join(output, `quick-reply-slash-${width}.png`) })
  await slashMatchBtn.click()
  assert.equal(await quickReplyComposer.inputValue(), 'Halo! Ini promo terbaru dari HP.', 'Selecting synced slash template must fill composer')
  await quickReplyComposer.fill('')

  // Simulate another template added while tab was unfocused, then tab regains focus
  fixtureCannedResponses.unshift({
    id: 'canned-followup',
    shortcut: 'followup',
    title: 'Follow Up Pelanggan',
    content: 'Apakah ada kendala lain yang bisa kami bantu?',
    category: 'support',
    created_at: now,
    updated_at: new Date(Date.parse(now) + 180000).toISOString(),
  })
  await page.evaluate(() => window.dispatchEvent(new Event('focus')))
  await cannedTrigger.click()
  await page.getByText('/followup').waitFor({ state: 'visible', timeout: 5000 })
  await page.keyboard.press('Escape')

  checks.push({width,composer_task_disclosure:true,composer_task_autoscroll:true,last_task_visible:true,no_chat_task_card:true,session_loading_running_completed:true,back_preserves_draft_and_scroll:true,full_terminal_height:terminalBox.height,panel_width:box.width,composer_alignment:true,multiline_autoexpand:true,studio_autoexpand:true,typing_dots_circular:true,message_context_menu_ui:menuMetrics,message_edit_and_delete:true,canned_responses_cross_device_sync:true})
  await context.close()
  await browser.close()
  browser=null
 }
 await fs.writeFile(path.join(output,'verification.json'),JSON.stringify({source:'Actual application React components; isolated API fixtures, not live task evidence',checks},null,2))
 console.log(JSON.stringify(checks))
}finally{await browser?.close();await server.close()}
