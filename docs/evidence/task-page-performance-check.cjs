// Real build + isolated API fixture: bounded requests, server-driven filters,
// paging, search races and route bootstrapping. Never touches production data.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),http=require('http'),path=require('path'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-pm-recovery/task-page-performance',now=Date.now()
const tasks=Array.from({length:111},(_,i)=>({id:'task_'+String(i).padStart(4,'0'),title:'Work '+String(i).padStart(4,'0'),project_id:'project',project_name:'Cuan',status:['todo','doing','done'][i%3],priority:i%2?'normal':'high',created_at:new Date(now+i*1000).toISOString(),updated_at:new Date(now+i*1000).toISOString(),summary:'Approved work',latest_attempt:{id:'attempt_'+i,status:['queued','running','succeeded'][i%3]}}))
const project={id:'project',name:'Cuan',project_type:'custom',repo_url:'https://github.com/buchin/cuan',created_at:new Date(now).toISOString(),updated_at:new Date(now).toISOString(),task_progress:{total:111,done:37,active:37,queued:37},metadata:{}}
const calls=[]
let server,browser,page,failSearch=''
async function routeAPI(route){const req=route.request(),url=new URL(req.url()),p=url.pathname;calls.push({p,q:Object.fromEntries(url.searchParams),method:req.method()});const json=body=>route.fulfill({contentType:'application/json',body:JSON.stringify(body)})
 if(p.endsWith('/auth/status'))return json({setup_required:false})
 if(p.endsWith('/auth/me'))return json({owner:{id:'fixture',username:'Owner',role:'owner'}})
 if(p==='/api/v1/tasks'){
  assert.equal(url.searchParams.get('view'),'board','page eagerly fetched the full legacy task list')
  const size=Number(url.searchParams.get('page_size')||25);assert(size<=25)
  const counts={all:111,active:74,completed:37,attention:0,work_active:74,review_or_blocked:0,todo:37,doing:37,done:37}
  let rows=tasks.slice();const q=url.searchParams.get('search')||'',status=url.searchParams.get('status')||'',priority=url.searchParams.get('priority')||'',sort=url.searchParams.get('sort')||'updated'
  if(failSearch&&q===failSearch){failSearch='';return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Fixture unavailable'})})}
  if(q==='slow')await new Promise(r=>setTimeout(r,500))
  if(q)rows=rows.filter(t=>(t.title+' '+t.id).toLowerCase().includes(q.toLowerCase()))
  if(status&&status!=='all')rows=rows.filter(t=>['active','work_active'].includes(status)?t.status!=='done':status==='completed'?t.status==='done':t.status===status)
  if(priority&&priority!=='all')rows=rows.filter(t=>t.priority===priority)
  rows.sort((a,b)=>sort==='title'?a.title.localeCompare(b.title):b.updated_at.localeCompare(a.updated_at))
  const total=rows.length,num=Math.min(Number(url.searchParams.get('page')||1),Math.max(1,Math.ceil(total/size)));return json({tasks:rows.slice((num-1)*size,num*size),total,scope_total:111,counts,page:num,page_size:size})
 }
 if(p==='/api/v1/projects')return json({projects:[project]})
 if(p==='/api/v1/projects/project')return json(project)
 if(p==='/api/v1/agents')return json({agents:[{id:'agent_cuan',name:'@cuan',enabled:true,harness:'claude',provider:'claude',status:'ready'}]})
 if(p==='/api/v1/contacts'||p==='/api/v1/chat/contacts')return json({contacts:[],has_more:false})
 if(p==='/api/v1/item-preferences')return json({items:{}})
 if(p==='/api/v1/notifications')return json({notifications:[],unread_count:0})
 if(p==='/api/system/worker-health')return json({status:'ready',is_alive:true,running_count:0,queued_count:0})
 return json({})
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true});server=http.createServer((req,res)=>{let file=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(file))file=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(file))}catch{res.writeHead(404);res.end()}});await new Promise(r=>server.listen(4185,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [390,1440])for(const theme of ['light','dark']){
 const context=await browser.newContext({viewport:{width,height:900},isMobile:width===390,hasTouch:width===390});page=await context.newPage();page.setDefaultTimeout(15000)
 await page.addInitScript(theme=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-theme',theme);localStorage.setItem('cuan-language','en')},theme);await page.route('**/api/**',routeAPI)
 await page.goto('http://127.0.0.1:4185/tasks');await page.getByText('Work 0110',{exact:true}).filter({visible:true}).waitFor()
 assert.equal(await page.locator('tbody tr').count(),width===390?0:25,'hidden alternate layout was mounted');
 if(theme==='light'){await page.waitForTimeout(3400);assert.equal(calls.filter(c=>/^\/api\/v1\/tasks\/task_/.test(c.p)).length,0,'individual task cards polled details')}
 const visibleActions=()=>page.getByRole('button',{name:/^Actions for Work/}).filter({visible:true});assert.equal(await visibleActions().count(),25)
 await page.screenshot({path:`${out}/tasks-${theme}-${width}.png`});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
 await page.getByRole('button',{name:'Next page',exact:true}).filter({visible:true}).click();await page.getByText('Work 0085',{exact:true}).filter({visible:true}).waitFor();assert.equal(await visibleActions().count(),25)
 const search=page.getByRole('searchbox',{name:'Search tasks',exact:true});await search.fill('Work 0001');await page.getByText('Work 0001',{exact:true}).filter({visible:true}).waitFor();assert.equal(await visibleActions().count(),1);assert.equal(await page.getByRole('button',{name:'Next page',exact:true}).filter({visible:true}).count(),0)
 await search.fill('slow');await page.waitForTimeout(270);await search.fill('Work 0002');await page.getByText('Work 0002',{exact:true}).filter({visible:true}).waitFor();await page.waitForTimeout(550);assert.equal(await visibleActions().count(),1)
 await search.fill('');await page.getByText('Work 0110',{exact:true}).filter({visible:true}).waitFor()
 const doneResponse=page.waitForResponse(response=>new URL(response.url()).pathname==='/api/v1/tasks'&&new URL(response.url()).searchParams.get('status')==='done')
 await page.getByRole('tab',{name:/^(Done|Selesai)/}).filter({visible:true}).click();await doneResponse;await page.getByText('Work 0109',{exact:true}).waitFor({state:'hidden'});assert.equal(await visibleActions().count(),25)
 await page.getByRole('tab',{name:/^(All|Semua)/}).filter({visible:true}).click();await page.getByText('Work 0109',{exact:true}).filter({visible:true}).waitFor()
 failSearch='Work 0003';await search.fill('Work 0003');await page.getByRole('alert').filter({hasText:'Could not load tasks'}).waitFor();assert.equal(await page.getByText('No matching tasks.',{exact:true}).count(),0)
 await page.getByRole('button',{name:'Try again',exact:true}).click();await page.getByText('Work 0003',{exact:true}).filter({visible:true}).waitFor()

 await page.goto('http://127.0.0.1:4185/projects/project/tasks');await page.getByText('Work 0110',{exact:true}).filter({visible:true}).waitFor();await page.getByRole('button',{name:'Next page',exact:true}).filter({visible:true}).click();await page.getByText('Work 0085',{exact:true}).filter({visible:true}).waitFor();if(theme==='light'){await page.waitForTimeout(3400);assert.equal(calls.filter(c=>/^\/api\/v1\/tasks\/task_/.test(c.p)).length,0,'project cards polled details')}await page.screenshot({path:`${out}/project-${theme}-${width}.png`});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
 await context.close()
 }
 for(const route of ['/studio','/inbox','/projects','/contacts','/settings']){
 const context=await browser.newContext({viewport:{width:1440,height:900}});page=await context.newPage();await page.addInitScript(()=>localStorage.setItem('cuan_token','fixture'));await page.route('**/api/**',routeAPI);const start=calls.length;await page.goto('http://127.0.0.1:4185'+route);await page.locator('main').first().waitFor();await page.waitForTimeout(400);assert.equal(calls.slice(start).filter(r=>r.p==='/api/v1/tasks').length,0,route+' eagerly fetched tasks');await context.close()
 }
 fs.writeFileSync(out+'/requests.json',JSON.stringify(calls,null,2));console.log('390/1440 light/dark:25 rows, next page, remote search, stale-response cancellation, Project paging; five unrelated routes make zero task-list requests.')
})().catch(async e=>{console.error(e);if(page)await page.screenshot({path:out+'/error.png'});process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
