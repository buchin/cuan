const taskPageFixture=require('./task-page-fixture.cjs')
// Render actual frontend with isolated APIs; never mutates production data.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),http=require('http'),path=require('path'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-pm-recovery/attempt-progress'
const now=new Date().toISOString()
let server,browser,page
const tasks=[{id:'queued',title:'Audit ulang navigasi',status:'queued',summary:'HASIL LAMA SELESAI',latest_attempt:{id:'new-queued',status:'queued'}},{id:'running',title:'Periksa hasil audit',status:'doing',latest_progress:'Memeriksa hasil terbaru',summary:'HASIL LAMA SELESAI',latest_attempt:{id:'new-running',status:'running'}}].map(t=>({...t,project_id:'project',project_name:'Cuan',created_at:now,updated_at:now}))
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4184,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [390,1440])for(const theme of ['light','dark']){
 const context=await browser.newContext({viewport:{width,height:900}});page=await context.newPage()
 await page.addInitScript(theme=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-theme',theme)},theme)
 await page.route('**/api/**',route=>{
 const p=new URL(route.request().url()).pathname;let body={}
 if(p.endsWith('/auth/status'))body={setup_required:false}
 if(p.endsWith('/auth/me'))body={owner:{id:'fixture',username:'Owner',role:'owner'}}
 if(p==='/api/v1/tasks')body={tasks}
 if(p.startsWith('/api/v1/tasks/'))body={...tasks.find(t=>p.endsWith('/'+t.id)),activities:[],attempts:[]}
 if(p==='/api/v1/projects')body={projects:[{id:'project',name:'Cuan',created_at:now,updated_at:now}]}
 if(p==='/api/v1/chat/contacts'||p==='/api/v1/contacts')body={contacts:[]}
 if(p==='/api/v1/agents')body={agents:[]}
 if(p==='/api/v1/notifications')body={notifications:[],unread_count:0}
 if(p==='/api/v1/item-preferences')body={items:{}}
 return route.fulfill({contentType:'application/json',body:JSON.stringify(taskPageFixture(route.request().url(),body))})
 })
 await page.goto('http://127.0.0.1:4184/tasks')
 await page.getByText('Audit ulang navigasi',{exact:true}).filter({visible:true}).waitFor()
 assert.equal(await page.getByText('HASIL LAMA SELESAI',{exact:false}).filter({visible:true}).count(),0)
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
 await page.screenshot({path:`${out}/tasks-${theme}-${width}.png`})
 await context.close()
 }
 console.log('Actual task list390/1440 light/dark: queued retry never presents historical completion summary; no overflow.')
})().catch(async e=>{console.error(e);if(page)await page.screenshot({path:out+'/error.png'});process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
