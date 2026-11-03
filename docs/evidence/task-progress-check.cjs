// Real production bundle with isolated fixture APIs; never writes to production.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs = require('fs'), path = require('path'), http = require('http'), assert = require('node:assert/strict')
const out = process.env.CUAN_VISUAL_OUTPUT || '/home/dev/reports/cuan-task-progress'
const now = new Date().toISOString()
const projects = [
 { id:'large',name:'Cuan — banyak pekerjaan',repo_url:'https://github.com/buchin/cuan',task_progress:{total:5002,done:1001} },
 { id:'complete',name:'Panduan penggunaan',repo_url:'',task_progress:{total:5,done:5} },
 { id:'waiting',name:'Rencana konten',repo_url:'',task_progress:{total:3,done:0} },
 { id:'empty',name:'Ide baru',repo_url:'',task_progress:{total:0,done:0} },
].map(p=>({...p,project_type:'custom',is_archived:false,created_at:now,updated_at:now,metadata:{}}))
const contacts = [
 {id:'contact',conversation_id:'fixture-room',name:'Siapkan panduan untuk pelanggan',task_count:7002,done_tasks:1001,canceled_tasks:2000,active_tasks:1,last_message:'Draf sudah tersedia. Saya lanjutkan pekerjaan berikutnya.'},
 {id:'complete',conversation_id:'complete-room',name:'Panduan penggunaan',task_count:5,done_tasks:5,last_message:'Panduan sudah diperiksa dan siap digunakan.'},
 {id:'waiting',conversation_id:'waiting-room',name:'Rencana konten',task_count:3,done_tasks:0,review_tasks:1,last_message:'Hasil pertama sedang diperiksa.'},
 {id:'canceled',conversation_id:'canceled-room',name:'Rencana yang dibatalkan',task_count:2,done_tasks:0,canceled_tasks:2,last_message:'Rencana dibatalkan.'},
 {id:'empty',conversation_id:'empty-room',name:'Diskusi ide baru',task_count:0,done_tasks:0,last_message:'Mari bahas kebutuhan Anda.'},
 {id:'unknown',conversation_id:'unknown-room',name:'Status belum tersedia',last_message:'Percakapan tanpa data progres.'},
].map(c=>({...c,type:'agent',agent_id:'agent_cuan',agent_name:'@cuan',unread_count:0,status:'idle',updated_at:now}))
let task={id:'msg_reply',title:'Siapkan panduan pelanggan',status:'done',project_id:'large',project_name:'Cuan',summary:'Panduan selesai dan telah diverifikasi.',source_message_id:'reply',created_at:now,updated_at:now,latest_attempt:{id:'attempt',status:'succeeded'}}
let server,browser,debugPage
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':p.endsWith('.woff2')?'font/woff2':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4183,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 const page=await browser.newPage();debugPage=page;page.setDefaultTimeout(10000);const errors=[];page.on('pageerror',e=>errors.push(e.message))
 await page.addInitScript(()=>localStorage.setItem('cuan_token','fixture-only'))
 await page.route('**/api/**',route=>{
  const p=new URL(route.request().url()).pathname,json=(body)=>route.fulfill({contentType:'application/json',body:JSON.stringify(body)})
  if(p.endsWith('/auth/status'))return json({setup_required:false})
  if(p.endsWith('/auth/me'))return json({owner:{id:'fixture',username:'Owner',role:'owner'}})
  if(p==='/api/v1/projects')return json({projects})
  if(p.startsWith('/api/v1/projects/'))return json(projects.find(x=>x.id===p.split('/')[4])||{})
  if(p==='/api/v1/tasks')return json({tasks:[task]})
  if(p==='/api/v1/tasks/msg_reply')return json({...task,attempts:[task.latest_attempt],activities:[]})
  if(p.endsWith('/run-agent/history'))return json({history:[],live_session:null})
  if(p==='/api/v1/chat/contacts')return json({contacts})
  if(p==='/api/v1/contacts')return json({contacts:[]})
  if(p==='/api/v1/item-preferences')return json({items:{}})
  if(p==='/api/v1/agents')return json({agents:[{id:'agent_cuan',name:'@cuan',enabled:true}]})
  if(p==='/api/v1/notifications')return json({notifications:[],unread_count:0})
  if(p==='/api/system/worker-health')return json({status:'ready',is_alive:true,running_count:0,queued_count:0})
  if(p==='/api/system/update')return json({current_version:'fixture',latest_version:'fixture',has_update:false})
  if(/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){
   const data={conversation_id:'fixture-room',room_id:'fixture-room',contact:contacts[0],tasks:[task],messages:[{id:1,uuid:'message',author_type:'user',sender_type:'visitor',content:'Buatkan panduan untuk pelanggan.',created_at:now},{id:11,uuid:'user-followup',author_type:'user',sender_type:'visitor',content:'Gunakan bahasa yang sederhana.',created_at:now},{id:2,uuid:'reply',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'Panduan sudah selesai dan diperiksa.\n\n[View work](/tasks/msg_reply)',created_at:now},{id:3,uuid:'agent-followup',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'Silakan cek file hasilnya.',created_at:now},{id:4,uuid:'file-only',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'',message:'',attachments:[{id:'fixture-file',name:'panduan.pdf',type:'application/pdf',size:1024,url:'/fixtures/panduan.pdf'}],created_at:now},{id:5,uuid:'user-later',author_type:'user',sender_type:'visitor',content:'Terima kasih, hasilnya sudah sesuai.',created_at:new Date(Date.parse(now)+360000).toISOString()}],requests:[]}
   return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(data)+'\n\n'}):json(data)
  }
  return json({})
 })
 for(const theme of ['dark','light'])for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4183/inbox/fixture-room')
  await page.evaluate(theme=>{document.documentElement.classList.toggle('dark',theme==='dark');localStorage.setItem('cuan-theme',theme)},theme)
  if(width===390)await page.getByRole('button',{name:'Back to conversations',exact:true}).click()
  const rows=page.locator('[data-testid=conversation-item]:visible');await rows.first().waitFor()
  const partial=rows.filter({hasText:'Siapkan panduan untuk pelanggan'}).locator('[data-task-progress]')
  // Opening a room updates its live counters from the actual one-task fixture.
  assert(await partial.count()===1)
  const full=rows.filter({hasText:'Panduan penggunaan'}).locator('[data-task-progress]')
  assert.equal(await full.getAttribute('data-task-progress'),'complete');assert.equal(await full.getAttribute('aria-label'),'5 of 5 tasks completed')
  for(const name of ['Rencana yang dibatalkan','Diskusi ide baru','Status belum tersedia'])assert.equal(await rows.filter({hasText:name}).locator('[data-task-progress]').count(),0)
  const review=rows.filter({hasText:'Rencana konten'}).locator('[data-task-progress]');assert.equal(await review.getAttribute('aria-label'),'0 of 3 tasks completed');assert.equal(await review.locator('path').count(),0)
  await page.screenshot({path:`${out}/inbox-${theme}-${width}.png`})
  await page.goto('http://127.0.0.1:4183/projects');await page.getByRole('link',{name:/Cuan — banyak pekerjaan/}).waitFor()
  const large=page.getByRole('link',{name:/Cuan — banyak pekerjaan/}).locator('[data-task-progress]')
  assert.equal(await large.getAttribute('aria-label'),'1,001 of 5,002 tasks completed');assert.equal(await large.locator('path').count(),1)
  assert.equal(await page.getByRole('link',{name:/Ide baru/}).locator('[data-task-progress]').count(),0)
  assert.equal(await large.evaluate(el=>el.getBoundingClientRect().width),16)
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
  await page.screenshot({path:`${out}/projects-${theme}-${width}.png`})
  await page.goto('http://127.0.0.1:4183/inbox/fixture-room');const card=page.locator('[data-card=task-card]');await card.waitFor()
  const check=card.locator('[data-task-status=Completed] svg');assert((await check.getAttribute('class')).includes('--status-success'))
  assert.equal(await page.locator('[data-message-id=user-followup] [data-bubble-tail]').count(),0)
  assert.equal(await page.locator('[data-message-id=agent-followup] [data-bubble-tail]').count(),0)
  assert.equal(await page.locator('[data-message-id=file-only] [data-bubble-tail]').count(),0)
  assert.equal(await page.locator('[data-message-id=user-later] [data-bubble-tail=outgoing]').count(),1)
  assert.equal(await page.locator('[data-card] [data-bubble-tail]').count(),0)
  const tail=await page.locator('[data-message-id=reply] [data-bubble-tail=incoming]').evaluate(el=>{const s=getComputedStyle(el,'::before');return {display:s.display,width:s.width,bg:s.backgroundColor,bubble:getComputedStyle(el).backgroundColor}})
  assert.equal(tail.display,'block');assert.equal(tail.width,'8px');assert.equal(tail.bg,tail.bubble)
  await card.scrollIntoViewIfNeeded();await page.screenshot({path:`${out}/completed-${theme}-${width}.png`})
 }
 // Localized exact counts, no rounded100% before the last accepted task.
 await page.evaluate(()=>localStorage.setItem('cuan-language','id'));await page.goto('http://127.0.0.1:4183/projects');await page.getByLabel('1.001 dari 5.002 tugas selesai',{exact:true}).waitFor()
 projects[0].task_progress.done=5002;await page.getByLabel('5.002 dari 5.002 tugas selesai',{exact:true}).waitFor();assert.equal(await page.getByLabel('5.002 dari 5.002 tugas selesai',{exact:true}).getAttribute('data-task-progress'),'complete')
 assert.deepEqual(errors,[]);console.log('Progress rendering passed:390/1440 light/dark, large counts, empty/canceled/unknown, review, completion and Indonesian labels.')
})().catch(async e=>{console.error(e);if(debugPage){await debugPage.screenshot({path:out+"/failure.png"});fs.writeFileSync(out+"/failure.txt",await debugPage.locator("body").innerText())}process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)server.close()})
