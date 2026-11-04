// Real frontend, isolated API fixtures; no production writes.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const {routeAPI,task}=require('./customer-task-card-check.cjs')
const fs=require('fs'),http=require('http'),path=require('path'),assert=require('node:assert/strict')
const out=process.env.CUAN_REVIEW_EVIDENCE_DIR||'/home/dev/reports/cuan-pm-recovery/review-outcome',now=new Date().toISOString()
const finding=process.env.CUAN_REVIEW_FINDING||''
const verifying=process.env.CUAN_REVIEW_VERIFYING==='1'
const stopped=verifying?'Cuan menyiapkan verifikasi tambahan pada hasil yang sama.':'Pekerjaan belum lolos verifikasi. Batas perbaikan otomatis sudah tercapai; hasil sementara dan bukti tetap tersimpan.'
const outcome=[finding,stopped].filter(Boolean).join('\n\n')
Object.assign(task,{title:'Perbaiki navigasi',summary:'Verifikasi belum selesai.',status:verifying?'queued':'blocked',source_message_id:'reply',launch:{state:verifying?'queued':'stopped'},latest_attempt:{id:'attempt',status:finding?'succeeded':'failed'}})
const contact={id:'fixture',type:'agent',agent_id:'agent_cuan',agent_name:'@cuan',name:'Perbaikan navigasi',conversation_id:'fixture-room',last_message:outcome,failed_tasks:verifying?0:1,task_count:1,updated_at:now}
let browser,server
async function api(route){
 const p=new URL(route.request().url()).pathname,json=body=>route.fulfill({contentType:'application/json',body:JSON.stringify(body)})
 if(p==='/api/v1/chat/contacts')return json({contacts:[contact]})
 if(/^\/api\/v1\/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){
  const body={conversation_id:'fixture-room',room_id:'fixture-room',contact,tasks:[task],requests:[],messages:[{id:1,uuid:'user',author_type:'user',sender_type:'visitor',content:'Perbaiki navigasi yang sudah disetujui.',created_at:now},{id:2,uuid:'reply',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:outcome+'\n\n[View work](/tasks/msg_reply)',created_at:now}]}
  return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(body)+'\n\n'}):json(body)
 }
 return routeAPI(route)
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let file=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(file))file=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(file))}catch{res.writeHead(404);res.end()}})
 await new Promise(resolve=>server.listen(4191,'127.0.0.1',resolve))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [390,1440]){
  const context=await browser.newContext({viewport:{width,height:width===390?844:900},isMobile:width===390,hasTouch:width===390})
  const page=await context.newPage();page.setDefaultTimeout(15000)
  await page.addInitScript(({theme})=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-language','id');localStorage.setItem('cuan-theme',theme)},{theme:width===390?'dark':'light'})
  await page.route('**/api/**',api)
  await page.goto('http://127.0.0.1:4191/inbox/fixture-room')
  await page.getByText(stopped,{exact:true}).filter({visible:true}).waitFor()
  if(finding)await page.getByText(finding,{exact:true}).filter({visible:true}).waitFor()
  await page.locator('[data-card=task-card]').first().waitFor()
  assert.equal(await page.getByText(/akan melanjutkan|I will continue/).count(),0)
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
  await page.screenshot({path:`${out}/blocked-review-${width}.png`})
  await context.close()
 }
 console.log('390/1440: actual review outcome and task card visible without unsupported continuation promise or horizontal overflow.')
})().catch(error=>{console.error(error);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
