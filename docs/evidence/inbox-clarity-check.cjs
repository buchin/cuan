// Actual frontend build; isolated API fixtures, no production writes.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const {routeAPI,task}=require('./customer-task-card-check.cjs')
const fs=require('fs'),http=require('http'),path=require('path'),assert=require('node:assert/strict')
const out='/home/dev/reports/cuan-pm-recovery/inbox-clarity',now=new Date().toISOString()
const ack='Work queued. I’ll report progress and the result here.'
let browser,server,page,onlyAck=false
Object.assign(task,{title:'Perbaiki navigasi',summary:'Memeriksa navigasi yang sudah disetujui.',status:'queued',launch:{state:'queued'},source_message_id:'reply',latest_attempt:{id:'attempt',status:'failed'}})
const contacts=[{id:'fixture',type:'agent',agent_id:'agent_cuan',agent_name:'@cuan',name:'Perbaikan navigasi',conversation_id:'fixture-room',last_message:'Hasil lama belum selesai.\n\n'+ack+'\n\n[View work](/tasks/msg_reply)',unread_count:2,queued_tasks:1,task_count:1,updated_at:now,last_message_at:now},{id:'issue',type:'agent',agent_id:'agent_cuan',agent_name:'@cuan',name:'Koneksi GitHub',conversation_id:'issue-room',last_message:'The work is saved, but could not start. GitHub access is unavailable.',failed_tasks:1,task_count:1,updated_at:now},{id:'customer',type:'customer',name:'Pelanggan',conversation_id:'customer-room',last_message:'Terima kasih.',has_messages:true,updated_at:now}]
async function api(route){
 const p=new URL(route.request().url()).pathname,json=body=>route.fulfill({contentType:'application/json',body:JSON.stringify(body)})
 if(p==='/api/v1/agents/auth-status')return json({runtime_prerequisites:{cli_available:true,sandbox_available:true,installation_supported:true}})
 if(p==='/api/v1/chat/contacts')return json({contacts})
 if(/^\/api\/v1\/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){
  const body={conversation_id:'fixture-room',room_id:'fixture-room',contact:contacts[0],tasks:[task],requests:[],messages:[{id:1,uuid:'user',author_type:'user',sender_type:'visitor',content:'Periksa navigasi yang sudah disetujui.',created_at:now},{id:2,uuid:'reply',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:(onlyAck?'':'Saya lanjutkan pemeriksaan navigasinya.\n\n')+ack+'\n\n[View work](/tasks/msg_reply)',created_at:now},{id:3,uuid:'failure',author_type:'system',sender_type:'operator',author_name:'@cuan',content:'The work is saved, but could not start. GitHub access is unavailable.\n\n[View work](/tasks/msg_reply)',created_at:now}]}
  return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(body)+'\n\n'}):json(body)
 }
 return routeAPI(route)
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let file=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(file))file=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(file))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4188,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [390,1440])for(const language of ['en','id']){
  const theme=language==='id'?'dark':'light',context=await browser.newContext({viewport:{width,height:width===390?844:900},isMobile:width===390,hasTouch:width===390})
  page=await context.newPage();page.setDefaultTimeout(15000)
  await page.addInitScript(({language,theme})=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-language',language);localStorage.setItem('cuan-theme',theme)}, {language,theme})
  await page.route('**/api/**',api)
  await page.goto('http://127.0.0.1:4188/inbox');const assistant=page.getByRole('tab',{name:language==='en'?'Assistant':'Asisten',exact:true});await assistant.waitFor()
  assert.equal(await assistant.innerText(),language==='en'?'Assistant':'Asisten');assert.equal(await page.getByRole('tab',{name:/^Agents/}).count(),0)
  await page.screenshot({path:`${out}/list-${language}-${width}.png`})
  onlyAck=false;await page.goto('http://127.0.0.1:4188/inbox/fixture-room');await page.locator('[data-card=task-card]').first().waitFor()
  await page.getByText('Saya lanjutkan pemeriksaan navigasinya.',{exact:true}).filter({visible:true}).waitFor()
  assert.equal(await page.getByText(ack,{exact:false}).filter({visible:true}).count(),0)
  await page.getByText('The work is saved, but could not start. GitHub access is unavailable.',{exact:true}).filter({visible:true}).waitFor()
  await page.screenshot({path:`${out}/chat-${language}-${width}.png`})
  onlyAck=true;await page.reload();await page.locator('[data-card=task-card]').first().waitFor()
  assert.equal(await page.getByText(ack,{exact:false}).filter({visible:true}).count(),0)
  await page.screenshot({path:`${out}/card-only-${language}-${width}.png`})
  await page.goto('http://127.0.0.1:4188/studio');const composer=page.locator('#studio-goal');await composer.waitFor()
  const top=await page.locator('[data-studio-goal-panel] h2').evaluate(el=>el.getBoundingClientRect().top)
  if(width===390)assert(top>180 && top<400,`Studio should be vertically centered:${top}`)
  assert.equal(await page.getByText(/Tell Cuan your goal|Ceritakan tujuan Anda/).count(),0)
  await composer.fill('Ide yang belum rapi');await page.screenshot({path:`${out}/studio-${language}-${width}.png`})
  if(width===390){await page.setViewportSize({width,height:500});assert(await composer.isVisible());assert((await composer.boundingBox()).y<200);assert.equal(await composer.inputValue(),'Ide yang belum rapi')}
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await context.close()
 }
 console.log('390/1440 English/Indonesian: singular Assistant tab, concise preview, launch boilerplate removed, actual failure retained, card-only message, Studio centered/draft preserved at short viewport; screenshots saved.')
})().catch(async e=>{console.error(e);if(page)await page.screenshot({path:out+'/error.png'}).catch(()=>{});process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
