// Browser render of the production build with fixture API; no provider writes.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-account-models'
let browser,server,fail=false,catalogCalls=0
const accounts=[{id:'signed',label:'Personal account',provider:'agy',credentials_present:true,state:'quota_limited',model_id:'saved-model',email:'owner@example.invalid'}, {id:'unsigned',label:'Not connected',provider:'agy',credentials_present:false,state:'login_required',model_id:''}]
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':p.endsWith('.woff2')?'font/woff2':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4182,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 const page=await browser.newPage();page.setDefaultTimeout(8000);const errors=[];page.on('pageerror',e=>errors.push(e.message))
 await page.addInitScript(()=>localStorage.setItem('cuan_token','fixture-only'))
 await page.route('**/api/**',route=>{const p=new URL(route.request().url()).pathname;const json=(body,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)})
 if(p==='/api/v1/item-preferences')return json({items:{}})
 if(p.endsWith('/auth/status'))return json({setup_required:false})
 if(p.endsWith('/auth/me'))return json({owner:{id:'fixture',username:'Owner',role:'owner'}})
 if(p==='/api/v1/provider-accounts')return json({accounts})
 if(p==='/api/v1/provider-accounts/signed/models'){catalogCalls++;return fail?json({error:'Fixture discovery unavailable'},409):json({models:[{id:'native-model',name:'Native model'}]})}
 if(p==='/api/v1/provider-accounts/signed/usage')return json({available:true,checked_at:'2026-11-03T15:00:00Z',models:[{model:'native-model',remaining_percent:42,reset_at:'2026-11-04T15:00:00Z'},{model:'retired-model',remaining_percent:99}]})
 if(p==='/api/v1/provider-accounts/unsigned/models')throw Error('unsigned catalog called')
 if(p==='/api/v1/ai/providers')return json({providers:[]})
 if(p==='/api/v1/chat/contacts')return json({contacts:[{id:'contact',type:'agent',name:'@cuan',agent_id:'agent_cuan',conversation_id:'fixture-room',unread_count:0,status:'idle',updated_at:new Date().toISOString()}]})
 if(/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){const now=new Date().toISOString();const data={conversation_id:'fixture-room',room_id:'fixture-room',contact:{id:'contact',type:'agent',name:'@cuan',agent_id:'agent_cuan',conversation_id:'fixture-room'},messages:[{id:1,uuid:'message',author_type:'user',sender_type:'visitor',content:'Create a useful guide',created_at:now}],tasks:[],requests:[{request_id:'request',message_id:'message',reply_id:'',agent_id:'agent_cuan',state:'queued',created_at:now,safe_error:'AI capacity is temporarily full. Your message is saved and will continue automatically.',steps:Array.from({length:100},(_,i)=>({seq:i+1,kind:'capacity_wait',text:'Recorded event '+i,at:now}))}]};return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(data)+'\n\n'}):json(data)}
 if(p==='/api/v1/agents')return json({agents:[{id:'agent_cuan',name:'@cuan',enabled:true}]})
 if(p==='/api/v1/notifications')return json({notifications:[],unread_count:0})
 return json({})
 })
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/settings/agent')
  await page.getByRole('button',{name:'Edit brain: Personal account',exact:true}).click()
  const select=page.getByLabel('Preferred model',{exact:true});await select.waitFor();await page.waitForFunction(()=>{const s=document.querySelector('#brain-model');return s&&!s.disabled})
  assert.equal(await select.inputValue(),'saved-model');assert.equal(await select.locator('option').count(),3)
  await page.getByRole('meter').waitFor();assert.equal(await page.getByRole('meter').getAttribute('aria-valuenow'),'42');assert.equal(await page.getByText('retired-model',{exact:true}).count(),0);await page.screenshot({path:out+'/models-'+width+'.png'})
  await select.selectOption('native-model');assert.equal(await select.inputValue(),'native-model')
  page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Close drawer',exact:true}).click()
  fail=true;await page.getByRole('button',{name:'Edit brain: Personal account',exact:true}).click();await page.getByText('Could not load models. Your saved selection is preserved.',{exact:true}).waitFor()
  assert.equal(await select.inputValue(),'saved-model');assert(await select.isDisabled());await page.screenshot({path:out+'/models-error-'+width+'.png'})
  fail=false;await page.getByRole('button',{name:'Try again',exact:true}).click();await page.waitForFunction(()=>!document.querySelector('#brain-model').disabled);assert.equal(await select.inputValue(),'saved-model')
  await page.getByRole('button',{name:'Close drawer',exact:true}).click();const before=catalogCalls
  await page.getByRole('button',{name:'Edit brain: Not connected',exact:true}).click();await page.getByText('Connect this account before choosing a model.',{exact:true}).waitFor();assert(await select.isDisabled());assert.equal(catalogCalls,before)
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:out+'/models-login-'+width+'.png'});await page.getByRole('button',{name:'Close drawer',exact:true}).click()
 }
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/inbox/fixture-room')
  const trigger=page.getByRole('button',{name:'Detail',exact:true});await trigger.waitFor();const before=await trigger.boundingBox()
  await trigger.click();const drawer=page.getByRole('dialog',{name:'Response details',exact:true});await drawer.waitFor()
  const rect=await drawer.boundingBox();assert.equal(Math.round(rect.width),width===390?390:480)
  await drawer.getByRole('list',{name:'Response timeline'}).waitFor();assert.equal(await drawer.locator('ol[aria-label="Response timeline"] > li').count(),100)
  await page.screenshot({path:out+'/response-details-'+width+'.png'})
  await drawer.getByRole('button',{name:'Close drawer',exact:true}).click();await drawer.waitFor({state:'hidden'});const after=await trigger.boundingBox();assert(Math.abs(before.y-after.y)<2)
  assert(await page.getByRole('button',{name:'Cancel queued reply',exact:true}).isVisible());await page.screenshot({path:out+'/response-compact-'+width+'.png'})
 }
 assert.deepEqual(errors,[]);fs.writeFileSync(out+'/result.json',JSON.stringify({passed:true,viewports:[390,1440],catalogCalls},null,2));console.log('PASS model picker browser checks: '+out)
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
