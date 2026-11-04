// Render the real build; fixture-only API, no production writes.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const {routeAPI}=require('./customer-task-card-check.cjs')
const fs=require('fs'),http=require('http'),path=require('path'),assert=require('node:assert/strict')
const out='/home/dev/reports/cuan-pm-recovery/retry-clarity',now=new Date().toISOString()
let browser,server,page,action='check_status',retryCalls=0,requestState='failed',rejectRetry=false
const contact={id:'fixture',type:'agent',agent_id:'agent_cuan',agent_name:'@cuan',name:'What next untuk Cuan?',conversation_id:'fixture-room',updated_at:now}
async function api(route){
 const p=new URL(route.request().url()).pathname,json=(body,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)})
 if(p==='/api/v1/chat/contacts')return json({contacts:[contact]})
 if(p.endsWith('/requests/request/retry')){retryCalls++;if(!rejectRetry){requestState='queued';return json({request:{state:'queued'}},202)}return json({error:'This attempt cannot be safely repeated. Check its status and details.'},409)}
 if(/^\/api\/v1\/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){
  const body={conversation_id:'fixture-room',room_id:'fixture-room',contact,tasks:[],requests:[{request_id:'request',message_id:'user',reply_id:'reply',state:requestState,recovery_action:action,safe_error:'The provider reached a usage limit after tool activity that cannot be safely repeated.',created_at:now,deadline:now,steps:[{seq:1,kind:'runtime_tool_started',text:'Inspecting saved work',at:now},{seq:2,kind:'provider_cooldown',text:'Provider usage limit',at:now}]}],messages:[{id:1,uuid:'user',author_type:'user',sender_type:'visitor',content:'What next untuk Cuan?',created_at:now}]}
  return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(body)+'\n\n'}):json(body)
 }
 return routeAPI(route)
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let file=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(file))file=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(file))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4194,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [390,1440])for(const language of ['en','id']){
  const context=await browser.newContext({viewport:{width,height:844},isMobile:width===390,hasTouch:width===390})
  page=await context.newPage();page.setDefaultTimeout(12000)
  await page.addInitScript(({language})=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-language',language);localStorage.setItem('cuan-theme',language==='id'?'dark':'light')},{language})
  await page.route('**/api/**',api)
  action='check_status';requestState='failed';rejectRetry=false;await page.goto('http://127.0.0.1:4194/inbox/fixture-room');const status=page.locator('[data-response-state="failed"]');await status.waitFor()
  const retry=language==='en'?'Try again':'Coba lagi',waiting=language==='en'?'Waiting for AI capacity':'Menunggu kapasitas AI'
  await status.getByRole('button',{name:retry,exact:true}).waitFor()
  assert.equal(await status.getByText(waiting,{exact:true}).count(),0)
  
  await page.screenshot({path:`${out}/inspect-${language}-${width}.png`})
  await status.getByRole('button',{name:retry,exact:true}).click();await page.locator('[data-response-state=queued]').waitFor();assert.equal(await page.getByRole('alert').count(),0);
  action='check_usage';requestState='failed';rejectRetry=true;await page.reload();await status.waitFor();await status.getByRole('button',{name:retry,exact:true}).waitFor();await status.getByText(waiting,{exact:true}).waitFor()
  await status.getByRole('button',{name:retry,exact:true}).click()
  await page.getByRole('dialog').waitFor()
  assert.equal(await status.getByRole('button',{name:retry,exact:true}).count(),0)
  assert.equal(await page.getByRole('alert').count(),0)
  await page.screenshot({path:`${out}/race-${language}-${width}.png`})
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await context.close()
 }
 assert.equal(retryCalls,8);console.log('PASS: failed reply retry reconciles unknown state; quota and409 races at390/1440 English/Indonesian')
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
