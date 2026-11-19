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
await fs.mkdir(output,{mode:0o700})
const now='2026-11-06T05:00:00Z',room='work-fixture'
const contact={id:room,conversation_id:room,name:'Website pelanggan',type:'customer',email:'customer@example.test',unread_count:0,status:'online',last_message:'Memeriksa tampilan mobile setelah perbaikan.',updated_at:now,task_count:1,active_tasks:1}
const agents=[{id:'agent_cuan',name:'@cuan',status:'active',enabled:true,harness:'agy',provider:'agy',supports_vision:true}]
const attempt={id:'attempt',task_id:'work',status:'running',agent_name:'agent_cuan',runner_id:'runner',attempt_number:2,created_at:now,started_at:now}
const baseTask={id:'work',title:'Perbaiki tampilan website pelanggan dan periksa hasilnya pada desktop maupun ponsel sebelum menyatakan pekerjaan selesai',status:'doing',priority:'normal',project_id:'project',project_name:'Website',source_room_id:room,source_message_id:'source',repo_url:'https://github.com/example/site',created_at:now,updated_at:now,latest_attempt:attempt,latest_progress:'Memeriksa tampilan mobile setelah perbaikan.',instructions:'Perbaiki tampilan pada desktop dan mobile. Verifikasi hasil sebenarnya.',summary:'Laporan sebelumnya belum diterima.',dod_checklist:[{text:'Tampilan stabil pada mobile dan desktop',completed:false}]}
const activities=Array.from({length:30},(_,index)=>({id:'progress-'+index,task_id:'work',attempt_id:'attempt',activity_type:'agent_report',new_status:'progress',author:'agent_cuan',message:['Memeriksa sumber masalah.','Meninjau perubahan yang tersimpan.','Menguji tampilan pada browser.','Membandingkan hasil dengan permintaan awal.'][index%4],created_at:new Date(Date.parse(now)+index*1000).toISOString()}))
activities.push({id:'long-review',task_id:'work',attempt_id:'attempt',activity_type:'coordinator_review_assessment',author:'coordinator',message:JSON.stringify({assessment:{decision:'revise',finding:('Pemeriksaan hasil perlu membandingkan perilaku aplikasi dengan permintaan awal dan bukti yang tersimpan. ').repeat(12)+'Akhir laporan asli.'}}),created_at:new Date(Date.parse(now)+31000).toISOString()})
activities.push({id:'provider-note',task_id:'work',activity_type:'comment',author:'runner',message:'Provider preflight selected allowed backup account',created_at:now})
const fixtureTask={...baseTask,attempts:[attempt],activities}
const message=(id,text,extra={})=>({id,uuid:String(id),room_id:room,sender_type:'operator',author_type:'agent',author_id:'agent_cuan',author_name:'Cuan',message:text,content:text,is_internal:true,created_at:now,...extra})
const messages=[message('source','Tolong perbaiki tampilan website.',{author_type:'user',author_id:'operator_user'}),...Array.from({length:12},(_,i)=>message('chat-'+i,'Pembahasan kebutuhan yang tetap tersedia di percakapan.')),
 message('choices','Pilih langkah yang paling sesuai.',{prompt_choices:{question:'Bagaimana kita lanjutkan?',options:[{id:'continue',label:'Lanjutkan pemeriksaan'},{id:'adjust',label:'Sesuaikan kebutuhan'}],allow_other:true}}),
 message('reply-out','Sudut bubble chat diperbarui dengan lengkungan yang halus.',{sender_type:'operator',author_type:'operator',author_id:'operator_user',author_name:'You',is_internal:false}),
 message('old','Hasil perlu diperiksa lagi. [View work](/tasks/work)',{work_update:{task_id:'work',kind:'progress'}}),
  message('reply-draft','Draf email balasan untuk pelanggan sudah disiapkan.',{reply_draft:'Halo sob,\n\nKendala login sudah diperbaiki dan sistem normal kembali. Silakan dicoba login ulang.',is_internal:true}),
 message('files','Files from this work session',{work_update:{task_id:'work',attempt_id:'attempt',kind:'files'},attachments:[{id:'file',name:'report.txt',type:'text/plain',size:10,url:'/api/v1/chat/attachments/file'}]})]
const entry=`import React from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter,useLocation} from 'react-router-dom';
import {LanguageProvider} from '/src/context/LanguageContext.tsx';
import {AuthProvider} from '/src/context/AuthContext.tsx';
import {AppShell} from '/src/components/layout/AppShell.tsx';
import {InboxView} from '/src/features/inbox/InboxView.tsx';
import {TaskSessionPage} from '/src/features/tasks/TaskSessionPage.tsx';
import {InboxSubPanel} from '/src/features/inbox/InboxSubPanel.tsx';
import '/src/index.css';
const contact=${JSON.stringify(contact)},agents=${JSON.stringify(agents)};
const noop=()=>{};
function Scene(){const location=useLocation();return location.pathname.startsWith('/tasks/work/sessions/')?React.createElement(TaskSessionPage,{taskId:'work',sessionId:location.pathname.split('/')[4]||'latest'}):React.createElement(InboxView,{contacts:[contact],agents,activeContact:contact,initialMode:'chat'})}
createRoot(document.getElementById('root')).render(React.createElement(BrowserRouter,null,React.createElement(LanguageProvider,null,React.createElement(AuthProvider,null,React.createElement(AppShell,{currentTab:'inbox',onSelectTab:noop,inboxSubPanel:React.createElement(InboxSubPanel,{contacts:[contact],agents,activeContactId:contact.id,onSelectContact:noop,selectedTab:'all',searchQuery:'',onSearchChange:noop,onTabChange:noop,onNewStudio:noop,onNewMessage:noop})},React.createElement(Scene))))));`
const webRoot=path.resolve('web')
process.chdir(webRoot)
const server=await createServer({root:webRoot,envDir:path.resolve(output),cacheDir:path.join(path.resolve(output),'vite-cache'),server:{host:'127.0.0.1',port:0},plugins:[{name:'work-fixture',enforce:'pre',transform(code,id){if(process.env.CUAN_HOOK_REGRESSION_PROBE==='1'&&id.split('?')[0].endsWith('/TaskSessionPage.tsx')){if(!code.includes('  if (!detail) {'))throw Error('Hook regression probe target missing');return code.replace('  if (!detail) {','  if (detail) useRef(null)\n  if (!detail) {')}},resolveId(id){if(id==='/__work_entry.jsx')return '\0work-fixture-entry.jsx'},load(id){if(id==='\0work-fixture-entry.jsx')return entry},configureServer(server){server.middlewares.use(async(req,res,next)=>{
 if(req.url.startsWith('/inbox/work-fixture')||req.url.startsWith('/tasks/work/sessions/')){res.setHeader('Content-Type','text/html');res.end(await server.transformIndexHtml(req.url,'<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/__work_entry.jsx"></script></body></html>'))}
 else next()
})}}]})
await server.listen()
const origin=server.resolvedUrls.local[0]
let browser
const checks=[]
try{
 for(const width of (process.env.CUAN_WORKFLOW_NATIVE==='1'?[390,1440]:[390,1440,1920])){
  browser=await chromium.launch({headless:true,executablePath:process.env.CUAN_CHROMIUM_EXECUTABLE,args:process.env.CUAN_BROWSER_NO_SANDBOX==='1'?['--no-sandbox']:[]})
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
   else if(pathname.endsWith('/messages'))data={messages,tasks:[baseTask],contact,requests:[]}
   else if(pathname==='/api/v1/tasks/work'){
    if(page.url().includes('/sessions/')&&sessionLoading){sessionLoading=false;await new Promise(resolve=>{releaseSession=resolve})}
    data=sessionStatus==='running'?fixtureTask:{...fixtureTask,status:'review',latest_attempt:{...attempt,status:'succeeded',finished_at:now},attempts:[{...attempt,status:'succeeded',finished_at:now}]}
   }
   else if(pathname.endsWith('/run-agent/history'))data={history:[],live_session:sessionStatus==='running'?{session_id:'fixture-session',attempt_id:'attempt',agent_id:'agent_cuan',started_at:now}:null}
   else if(pathname.endsWith('/logs'))data={logs:[{id:1,line:'Isolated terminal fixture output',stream:'stdout',logged_at:now}]}
   else if(pathname.includes('/agents'))data=pathname==='/api/v1/agents'?{agents}:agents[0]
   else if(pathname.includes('preferences'))data={items:{}}
   else if(pathname.endsWith('/projects'))data={projects:[]}
   else if(pathname.includes('/email')&&route.request().method()==='POST')return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({status:'success',email_id:'re_mock_fixture_123',delivery_status:'sent',recipient:'customer@example.test',message:'Email balasan berhasil dikirim'})})
   else if(pathname.endsWith('/files')&&route.request().method()==='POST')return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({files:[{filename:'screenshot.png',path:'./screenshot.png',size:2048}]})})
   else if(pathname.endsWith('/tasks'))data={tasks:[baseTask]}
   else if(pathname.endsWith('/stream')||pathname.endsWith('/events'))return route.fulfill({status:200,contentType:'text/event-stream',body:''})
   return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)})
  })
  await page.goto(new URL('/inbox/work-fixture',origin).href,{waitUntil:'domcontentloaded'})
  await page.evaluate(dark=>document.documentElement.classList.toggle('dark',dark),width===1920)
  // Tasks live in the composer disclosure, not as duplicate chat cards.
  const tasks=page.locator('.composer-tasks > div > button[aria-expanded]')
  await tasks.waitFor({state:'visible'})
  assert.equal(await tasks.getAttribute('aria-expanded'),'false')
  assert.equal(await page.locator('.inbox-message-list [data-card="task-card"]').count(),0)
  await tasks.click()
  assert.equal(await tasks.getAttribute('aria-expanded'),'true')
  const card=page.locator('.composer-tasks-panel [data-timeline-item="work"]')
  try{await card.waitFor({state:'visible',timeout:20000})}catch(error){await page.screenshot({path:path.join(output,'failed-'+width+'.png')});console.log(JSON.stringify({errors,body:(await page.locator('body').innerText()).slice(0,1500)}));throw error}
  assert.equal(await card.count(),1)
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
  sessionStatus='succeeded'
  await page.locator('[aria-label="Session result"]').waitFor({state:'visible',timeout:10000})
  await page.screenshot({path:path.join(output,`session-result-${width}.png`)})
  assert.equal(await page.getByText('Something went wrong',{exact:true}).count(),0)
  assert.deepEqual(errors,[])
  checks.push({width,composer_task_disclosure:true,no_chat_task_card:true,session_loading_running_completed:true,back_preserves_draft_and_scroll:true,full_terminal_height:terminalBox.height,panel_width:box.width})
  await context.close()
  await browser.close()
  browser=null
 }
 await fs.writeFile(path.join(output,'verification.json'),JSON.stringify({source:'Actual application React components; isolated API fixtures, not live task evidence',checks},null,2))
 console.log(JSON.stringify(checks))
}finally{await browser?.close();await server.close()}
