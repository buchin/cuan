// Real frontend and isolated owner API fixtures. No visitor message is sent.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const {routeAPI,task}=require('./customer-task-card-check.cjs')
const fs=require('fs'),http=require('http'),path=require('path'),assert=require('node:assert/strict')
const out=process.env.CUAN_REPLY_EVIDENCE_DIR||'/home/dev/reports/cuan-pm-recovery/customer-reply-ui'
const now=new Date().toISOString(),draft='Halo Pak Agus, hasil pemeriksaan menunjukkan kendala pada paket CLI dan pemeriksaan versi PHP. PHP 8.3 didukung, sedangkan PHP 8.2 belum didukung. Investigasi sudah selesai; perbaikan paket belum dirilis.'
const contact={id:'customer',conversation_id:'fixture-room',type:'customer',name:'Agus',email:'customer@example.com',task_count:1,done_tasks:1,unread_count:0,last_message:'Draf balasan tersedia.',updated_at:now}
let browser,server,debugPage,writes=0
async function api(route){
 const req=route.request(),p=new URL(req.url()).pathname,json=body=>route.fulfill({contentType:'application/json',body:JSON.stringify(body)})
 if(!['GET','HEAD'].includes(req.method()) && p.endsWith('/messages')){writes++;return json({})}
 if(p==='/api/v1/chat/contacts')return json({contacts:[contact]})
 if(/^\/api\/v1\/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){
  const body={conversation_id:'fixture-room',room_id:'fixture-room',contact,tasks:[task],requests:[],messages:[
   {id:1,uuid:'source',author_type:'user',content:'@cuan tolong investigasi kendala pelanggan.',is_internal:true,created_at:now},
   {id:2,uuid:'reply',author_type:'agent',author_name:'@cuan',is_internal:true,content:'Investigasi selesai. Panduan tersedia dalam laporan.\n\n[View work](/tasks/msg_reply)',created_at:now},
   {id:3,uuid:'prepared-reply',author_type:'agent',author_name:'@cuan',is_internal:true,content:'[View work](/tasks/msg_reply)',reply_draft:draft,created_at:now}
  ]}
  return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(body)+'\n\n'}):json(body)
 }
 return routeAPI(route)
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let file=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(file))file=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(file))}catch{res.writeHead(404);res.end()}})
 await new Promise(resolve=>server.listen(4192,'127.0.0.1',resolve))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [390,1440]){
  const context=await browser.newContext({viewport:{width,height:width===390?844:1000},isMobile:width===390,hasTouch:width===390}),page=await context.newPage();debugPage=page;page.setDefaultTimeout(10000)
  await page.addInitScript(theme=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-language','en');localStorage.setItem('cuan-theme',theme)},width===390?'dark':'light')
  await page.route('**/api/**',api)
  await page.goto('http://127.0.0.1:4192/inbox/fixture-room')
  const card=page.locator('[data-card=reply-draft]');await card.waitFor();await card.scrollIntoViewIfNeeded()
  assert.equal(await card.count(),1);assert(await card.locator('[data-internal-visibility]').count()>0)
  const use=card.getByRole('button',{name:'Review & Use',exact:true});assert((await use.boundingBox()).height>=44)
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
  await page.screenshot({path:out+'/draft-'+width+'.png'})
  const before=writes;await use.click();assert.equal(await page.locator('[data-primary-composer]').inputValue(),draft)
  assert.equal(writes,before,'review only fills composer; it must not send a customer message')
  await context.close()
 }
 console.log('390/1440: one internal customer draft; Review & Use fills composer without sending.')
})().catch(async error=>{console.error(error);if(debugPage){await debugPage.screenshot({path:out+'/failure.png'});console.error((await debugPage.locator('body').innerText()).slice(-2500))}process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
