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
activities.push({id:'provider-note',task_id:'work',activity_type:'comment',author:'runner',message:'Provider preflight selected allowed backup account',created_at:now})
const fixtureTask={...baseTask,attempts:[attempt],activities}
const message=(id,text,extra={})=>({id,uuid:String(id),room_id:room,sender_type:'operator',author_type:'agent',author_id:'agent_cuan',author_name:'Cuan',message:text,content:text,is_internal:true,created_at:now,...extra})
const messages=[message('source','Tolong perbaiki tampilan website.',{author_type:'user',author_id:'operator_user'}),...Array.from({length:12},(_,i)=>message('chat-'+i,'Pembahasan kebutuhan yang tetap tersedia di percakapan.')),
 message('old','Hasil perlu diperiksa lagi. [View work](/tasks/work)',{work_update:{task_id:'work',kind:'progress'}}),
 message('files','Files from this work session',{work_update:{task_id:'work',attempt_id:'attempt',kind:'files'},attachments:[{id:'file',name:'report.txt',type:'text/plain',size:10,url:'/api/v1/chat/attachments/file'}]})]
const entry=`import React from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter} from 'react-router-dom';
import {LanguageProvider} from '/src/context/LanguageContext.tsx';
import {AuthProvider} from '/src/context/AuthContext.tsx';
import {AppShell} from '/src/components/layout/AppShell.tsx';
import {InboxView} from '/src/features/inbox/InboxView.tsx';
import {InboxSubPanel} from '/src/features/inbox/InboxSubPanel.tsx';
import '/src/index.css';
const contact=${JSON.stringify(contact)},agents=${JSON.stringify(agents)};
const noop=()=>{};
createRoot(document.getElementById('root')).render(React.createElement(BrowserRouter,null,React.createElement(LanguageProvider,null,React.createElement(AuthProvider,null,React.createElement(AppShell,{currentTab:'inbox',onSelectTab:noop,inboxSubPanel:React.createElement(InboxSubPanel,{contacts:[contact],agents,activeContactId:contact.id,onSelectContact:noop,selectedTab:'all',searchQuery:'',onSearchChange:noop,onTabChange:noop,onNewStudio:noop,onNewMessage:noop})},React.createElement(InboxView,{contacts:[contact],agents,activeContact:contact,initialMode:'chat'}))))));`
const webRoot=path.resolve('web')
process.chdir(webRoot)
const server=await createServer({root:webRoot,server:{host:'127.0.0.1',port:0},plugins:[{name:'work-fixture',resolveId(id){if(id==='/__work_entry.jsx')return '\0work-fixture-entry.jsx'},load(id){if(id==='\0work-fixture-entry.jsx')return entry},configureServer(server){server.middlewares.use(async(req,res,next)=>{
 if(req.url.startsWith('/inbox/work-fixture')){res.setHeader('Content-Type','text/html');res.end(await server.transformIndexHtml(req.url,'<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/__work_entry.jsx"></script></body></html>'))}
 else next()
})}}]})
await server.listen()
const origin=server.resolvedUrls.local[0]
const browser=await chromium.launch({headless:true,executablePath:process.env.CUAN_CHROMIUM_EXECUTABLE,args:process.env.CUAN_BROWSER_NO_SANDBOX==='1'?['--no-sandbox']:[]})
const checks=[]
try{
 for(const width of [390,1440,1920]){
  const context=await browser.newContext({viewport:{width,height:width===390?844:1000},hasTouch:width===390})
  await context.addInitScript(()=>{localStorage.setItem('cuan-language','id');localStorage.setItem('cuan_token','isolated-fixture')})
  const page=await context.newPage(),errors=[]
  page.on('pageerror',error=>errors.push(error.stack || error.message))
  page.on('console',message=>{if(message.type()==='error')console.log(message.text())})
  await page.route('**/api/**',async route=>{
   const pathname=new URL(route.request().url()).pathname
   if(!pathname.startsWith('/api/'))return route.continue()
   let data={}
   if(pathname.endsWith('/auth/status'))data={setup_required:false}
   else if(pathname.endsWith('/auth/me'))data={user:{id:'owner',email:'owner@example.test',name:'Owner',role:'owner'}}
   else if(pathname.endsWith('/messages'))data={messages,tasks:[baseTask],contact,requests:[]}
   else if(pathname==='/api/v1/tasks/work')data=fixtureTask
   else if(pathname.endsWith('/run-agent/history'))data={history:[],live_session:{session_id:'fixture-session',attempt_id:'attempt',agent_id:'agent_cuan',started_at:now}}
   else if(pathname.endsWith('/logs'))data={logs:[{id:1,line:'Isolated terminal fixture output',stream:'stdout',logged_at:now}]}
   else if(pathname.includes('/agents'))data=pathname==='/api/v1/agents'?{agents}:agents[0]
   else if(pathname.includes('preferences'))data={items:{}}
   else if(pathname.endsWith('/projects'))data={projects:[]}
   else if(pathname.endsWith('/tasks'))data={tasks:[baseTask]}
   else if(pathname.endsWith('/stream')||pathname.endsWith('/events'))return route.fulfill({status:200,contentType:'text/event-stream',body:''})
   return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)})
  })
  await page.goto(new URL('/inbox/work-fixture',origin).href,{waitUntil:'domcontentloaded'})
  await page.evaluate(dark=>document.documentElement.classList.toggle('dark',dark),width===1920)
  const card=page.locator('[data-card="task-card"]')
  try{await card.waitFor({state:'visible',timeout:20000})}catch(error){await page.screenshot({path:path.join(output,'failed-'+width+'.png')});console.log(JSON.stringify({errors,body:(await page.locator('body').innerText()).slice(0,1500)}));throw error}
  assert.equal(await card.count(),1)
  assert.equal(await page.getByText('Files from this work session',{exact:true}).count(),0)
  await card.locator('[data-task-latest-update]').waitFor()
  const composer=page.locator('[data-primary-composer]')
  await composer.fill('Draft tetap tersimpan')
  const list=page.locator('.inbox-message-list')
  await list.evaluate(el=>{el.scrollTop=Math.max(0,el.scrollHeight-el.clientHeight-40)})
  await card.scrollIntoViewIfNeeded()
  const previousScroll=await list.evaluate(el=>el.scrollTop)
  await card.locator('[data-task-detail]').click()
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
  const scroll=panel.locator('[data-work-scroll]')
  await page.waitForFunction(()=>document.querySelector('[data-work-scroll]').scrollTop>0,null,{timeout:5000})
  await scroll.evaluate(el=>{el.scrollTop=0;el.dispatchEvent(new Event('scroll'))})
  await panel.getByRole('button',{name:'Progres terbaru'}).waitFor()
  await page.screenshot({path:path.join(output,`detail-${width}.png`)})
  await page.goBack()
  await panel.waitFor({state:'detached'})
  assert.equal(await composer.inputValue(),'Draft tetap tersimpan')
  await page.waitForFunction(expected=>Math.abs(document.querySelector('.inbox-message-list').scrollTop-expected)<5,previousScroll,{timeout:2000})
  await card.locator('[data-task-detail]').click()
  await panel.waitFor({state:'visible'})
  await panel.locator('[data-work-back]').click()
  await panel.waitFor({state:'detached'})
  await card.getByRole('button',{name:'Terminal',exact:true}).waitFor({state:'visible'})
  await card.getByRole('button',{name:'Terminal',exact:true}).click()
  await panel.locator('[data-work-terminal]').waitFor({state:'visible'})
  assert.equal(await panel.locator('[data-work-timeline]').count(),0)
  await panel.locator('.xterm').waitFor({state:'visible'})
  const terminalBox=await panel.locator('[data-work-terminal]').boundingBox()
  assert.ok(terminalBox.height>650)
  await page.screenshot({path:path.join(output,`terminal-${width}.png`)})
  assert.deepEqual(errors,[])
  checks.push({width,one_card:true,back_preserves_draft_and_scroll:true,full_terminal_height:terminalBox.height,panel_width:box.width})
  await context.close()
 }
 await fs.writeFile(path.join(output,'verification.json'),JSON.stringify({source:'Actual application React components; isolated API fixtures, not live task evidence',checks},null,2))
 console.log(JSON.stringify(checks))
}finally{await browser.close();await server.close()}
