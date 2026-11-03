// Browser render of the production build with fixture API; no provider writes.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-account-models'
let browser,server,fail=false,catalogCalls=0,longMessage=false
const task={id:'msg_reply',title:'Prepare the Studio guide',status:'blocked',source_message_id:'reply',project_id:'project',project_name:'Cuan fixture',summary:'Guide is ready for verification.',created_at:new Date().toISOString(),updated_at:new Date().toISOString(),latest_attempt:{id:'attempt',status:'canceled',error_message:'Cancelled'}}
const accounts=[{id:'signed',label:'Personal account',provider:'agy',credentials_present:true,state:'quota_limited',model_id:'saved-model',email:'owner@example.invalid'}, {id:'unsigned',label:'Not connected',provider:'agy',credentials_present:false,state:'login_required',model_id:''}]
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':p.endsWith('.woff2')?'font/woff2':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4182,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 const page=await browser.newPage();page.setDefaultTimeout(8000);const errors=[];page.on('pageerror',e=>errors.push(e.message))
 await page.addInitScript(()=>localStorage.setItem('cuan_token','fixture-only'))
 await page.route('**/api/**',route=>{const p=new URL(route.request().url()).pathname;const json=(body,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)})
 if(p==='/api/system/worker-health')return json({status:'ready',is_alive:true,running_count:0,queued_count:0})
 if(p==='/api/system/update')return json({current_version:'fixture',latest_version:'fixture',has_update:false,auto_update_enabled:false,auto_update_time:'02:00'})
 if(p==='/api/v1/tasks/msg_reply/run-agent/history')return json({history:[],live_session:task.latest_attempt.status==='running'?{session_id:'attempt',attempt_id:'attempt'}:null})
 if(p==='/api/v1/tasks/msg_reply')return json({...task,attempts:[task.latest_attempt],activities:[]})
 if(p==='/api/v1/contacts')return json({contacts:[]})
 if(p==='/api/v1/projects')return json({projects:[]})
 if(p==='/api/v1/item-preferences')return json({items:{}})
 if(p.endsWith('/auth/status'))return json({setup_required:false})
 if(p.endsWith('/auth/me'))return json({owner:{id:'fixture',username:'Owner',role:'owner'}})
 if(p==='/api/v1/provider-accounts')return json({accounts})
 if(p==='/api/v1/provider-accounts/signed/models'){catalogCalls++;return fail?json({error:'Fixture discovery unavailable'},409):json({models:[{id:'native-model',name:'Native model'}]})}
 if(p==='/api/v1/provider-accounts/signed/usage')return json({available:true,checked_at:'2026-11-03T15:00:00Z',models:[{model:'native-model',remaining_percent:42,reset_at:'2026-11-04T15:00:00Z'},{model:'retired-model',remaining_percent:99}]})
 if(p==='/api/v1/provider-accounts/unsigned/models')throw Error('unsigned catalog called')
 if(p==='/api/v1/ai/providers')return json({providers:[]})
 if(p==='/api/v1/chat/contacts')return json({contacts:[{id:'contact',type:'agent',name:'Bantu bikin panduan singkat',agent_name:'@cuan',product:'general',is_online:true,agent_id:'agent_cuan',conversation_id:'fixture-room',unread_count:0,status:'idle',updated_at:new Date().toISOString()},{id:'unread-fixture',type:'agent',name:'Obrolan belum dibaca',agent_name:'@cuan',agent_id:'agent_cuan',conversation_id:'unread-room',unread_count:2,last_message:'Hasil tersedia untuk diperiksa.',status:'idle',updated_at:new Date().toISOString()}]})
 if(/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){const now=new Date().toISOString();const data={conversation_id:'fixture-room',room_id:'fixture-room',contact:{id:'contact',type:'agent',name:'@cuan',agent_id:'agent_cuan',conversation_id:'fixture-room'},messages:[{id:1,uuid:'message',author_type:'user',sender_type:'visitor',content:'Create a useful guide',created_at:now},{id:2,uuid:'reply',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'I will prepare the agreed guide and report the verified result here.'+(longMessage?'\n\n'+Array.from({length:14},(_,i)=>'Bagian '+(i+1)+': Jelaskan hasil pekerjaan yang sudah disepakati dengan bahasa sederhana.').join('\n\n'):'')+'\n\n[View work](/tasks/msg_reply)',created_at:now},{id:3,uuid:'report',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'### Evaluasi Kriteria Selesai (Definition of Done)\n**Status:** TERKENDALA (Blocked)\n\n**Bukti yang Terverifikasi:**\n- [x] **Konteks**: Tersimpan.\n\n**Belum Terbukti / Gap yang Perlu Diperbaiki:**\n- [ ] **Dokumen**: Belum dibuat.\n\n**Tindakan:** Periksa hasil.',created_at:now},{id:4,uuid:'latest-reply',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'I will prepare the agreed guide and report the verified result here.'+(longMessage?'\n\n'+Array.from({length:14},(_,i)=>'Bagian '+(i+1)+': Jelaskan hasil pekerjaan yang sudah disepakati dengan bahasa sederhana.').join('\n\n'):'')+'\n\n[View work](/tasks/msg_reply)',created_at:now}],tasks:[task],requests:[{request_id:'request',message_id:'message',reply_id:'',agent_id:'agent_cuan',state:'queued',created_at:now,safe_error:'AI capacity is temporarily full. Your message is saved and will continue automatically.',steps:Array.from({length:100},(_,i)=>({seq:i+1,kind:'capacity_wait',text:'Recorded event '+i,at:now}))}]};return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(data)+'\n\n'}):json(data)}
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
  const trigger=page.getByLabel('Response status',{exact:true}).getByRole('button',{name:'Detail',exact:true});await trigger.waitFor();const before=await trigger.boundingBox()
  await trigger.click();const drawer=page.getByRole('dialog',{name:'Response details',exact:true});await drawer.waitFor()
  const rect=await drawer.boundingBox();assert.equal(Math.round(rect.width),width===390?390:480)
  await drawer.getByRole('list',{name:'Response timeline'}).waitFor();assert.equal(await drawer.locator('ol[aria-label="Response timeline"] > li').count(),100)
  await page.screenshot({path:out+'/response-details-'+width+'.png'})
  await drawer.getByRole('button',{name:'Close drawer',exact:true}).click();await drawer.waitFor({state:'hidden'});const after=await trigger.boundingBox();assert(Math.abs(before.y-after.y)<2)
  assert(await page.getByRole('button',{name:'Cancel queued reply',exact:true}).isVisible());await page.screenshot({path:out+'/response-compact-'+width+'.png'})
 }
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/settings')
  const nav=page.getByRole('navigation',{name:'Settings navigation',exact:true});await nav.waitFor()
  if(width===1440){const r=await nav.boundingBox();assert(r.width<=256);assert(await page.locator('[data-detail-layout]').isVisible());assert.equal(await nav.locator('[aria-current="page"]').innerText(),'General')}
  else assert(!(await page.locator('[data-detail-layout]').isVisible()))
  await page.screenshot({path:out+'/settings-master-'+width+'.png'})
  await nav.getByRole('link',{name:'Appearance',exact:true}).click();await page.getByRole('button',{name:'Dark',exact:true}).click()
  assert.equal(await nav.isVisible(),width===1440);assert.equal(new URL(page.url()).pathname,'/settings/appearance');if(width===1440)assert.equal(await nav.getByRole('link').count(),10);await page.screenshot({path:out+'/settings-detail-'+width+'.png'})
  await page.goto('http://127.0.0.1:4182/inbox/fixture-room');const mixed=page.locator('.wa-incoming').filter({hasText:'I will prepare the agreed guide'}).last();await mixed.waitFor()
  assert.equal(await mixed.locator('[data-testid="task-completion-card"], [data-card="task-card"]').count(),0);assert.equal(await page.locator('[data-card="task-card"]').count(),1);const row=mixed.locator('xpath=ancestor::*[@data-message-id][1]');assert(await row.locator('[data-testid="task-completion-card"], [data-card="task-card"]').count()>0);assert.equal(await row.getByText('Needs attention',{exact:true}).count(),0);assert(await row.getByText('Cancelled',{exact:true}).count()>0);const card=row.locator('[data-card="task-card"]');assert(await card.getByText('Cuan fixture',{exact:true}).isVisible());assert.equal(await card.getByRole('button',{name:'Detail',exact:true}).count(),1);assert.equal(await card.getByRole('button',{name:'Terminal',exact:true}).count(),0)
  const style=await mixed.evaluate(el=>({bg:getComputedStyle(el).backgroundColor,padding:parseFloat(getComputedStyle(el).paddingLeft)}));assert.notEqual(style.bg,'rgba(0, 0, 0, 0)');assert(style.padding>=12)
  assert.equal(await page.getByRole('button',{name:'Actions for Inbox',exact:true}).count(),0);await page.screenshot({path:out+'/chat-bubble-dark-'+width+'.png'});assert.equal(await page.getByText(/1\/1 Terpenuhi/).count(),0);await page.screenshot({path:out+'/review-evidence-'+width+'.png'});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
 }
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/contacts');await page.getByPlaceholder('Search contacts…').waitFor();assert.equal(await page.getByRole('button',{name:'Actions for Contacts',exact:true}).count(),0);await page.screenshot({path:out+'/contacts-clean-'+width+'.png'})
 }
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/inbox/fixture-room');if(width===390)await page.getByRole('button',{name:'Back to conversations',exact:true}).click();await page.locator('[data-testid=conversation-item]:visible').first().waitFor();const row=page.locator('[data-testid=conversation-item]:visible').first();assert.equal(await row.getByText('@cuan',{exact:true}).count(),0);assert.equal(await row.getByText('general',{exact:true}).count(),0);assert.equal(await row.getByLabel('Online',{exact:true}).count(),0);await page.screenshot({path:out+'/inbox-list-'+width+'.png'})
 }
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/inbox/fixture-room');const summary=page.getByTestId('chat-review-summary');await summary.waitFor();assert.equal(await page.locator('.status-report-card').count(),0);await summary.getByRole('button',{name:'Review details',exact:true}).click();const detail=page.getByRole('dialog',{name:'Review details',exact:true});await detail.waitFor();assert(await detail.getByText('Dokumen',{exact:false}).count()>0);await page.screenshot({path:out+'/review-drawer-'+width+'.png'});await detail.getByRole('button',{name:'Close drawer',exact:true}).click();await summary.scrollIntoViewIfNeeded();await page.screenshot({path:out+'/review-summary-'+width+'.png'})
 }
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/inbox/fixture-room');if(width===390)await page.getByRole('button',{name:'Back to conversations',exact:true}).click();await page.waitForTimeout(400);const account=page.locator('button[aria-label="Account menu"]:visible').first();await account.waitFor();const avatar=account.locator('[data-avatar]');assert.equal(Math.round((await avatar.boundingBox()).width),36);assert((await account.boundingBox()).height>=56);await account.click();await page.getByTitle('Account overview',{exact:true}).waitFor();await page.screenshot({path:out+'/account-menu-'+width+'.png'});await page.evaluate(()=>document.documentElement.classList.remove('dark'));await page.screenshot({path:out+'/account-menu-light-'+width+'.png'});await page.evaluate(()=>document.documentElement.classList.add('dark'));assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await account.click()
 }
 longMessage=true
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/inbox/fixture-room');const bubble=page.locator('.wa-incoming').filter({hasText:'I will prepare the agreed guide'}).last();const more=bubble.getByRole('button',{name:'Read more',exact:true});await more.waitFor();assert.equal(await more.getAttribute('aria-expanded'),'false');const card=page.locator('[data-card="task-card"]');assert.equal(await card.locator('footer').count(),1);assert.equal(await card.locator('button',{hasText:'Detail'}).count(),1);await more.scrollIntoViewIfNeeded();await page.screenshot({path:out+'/message-collapsed-'+width+'.png'});await more.click();const less=bubble.getByRole('button',{name:'Show less',exact:true});assert.equal(await less.getAttribute('aria-expanded'),'true');await less.scrollIntoViewIfNeeded();await page.screenshot({path:out+'/message-expanded-'+width+'.png'});await less.click();assert.equal(await more.getAttribute('aria-expanded'),'false')
 }
 longMessage=false
 for(const [status,attemptStatus] of [['queued','queued'],['done','succeeded']]){
  task.status=status;task.latest_attempt.status=attemptStatus;delete task.latest_attempt.error_message;if(status==='done'){task.latest_attempt.error_message='Lease expired after accepted result';task.summary='Guide delivered and accepted.'}
  for(const width of [390,1440]){await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/inbox/fixture-room');const card=page.locator('[data-card="task-card"]');await card.waitFor();assert.equal(await card.getByRole('button',{name:'Terminal',exact:true}).count(),0);if(status==='done'){assert(await card.getByText('Guide delivered and accepted.',{exact:true}).isVisible());assert.equal(await card.getByText('Lease expired after accepted result',{exact:true}).count(),0)}await card.scrollIntoViewIfNeeded();await page.screenshot({path:out+'/task-card-'+status+'-'+width+'.png'})}
 }
 task.status='doing';task.latest_attempt.status='running';const terminalInput=[];let terminalSocket,terminalConnections=0
 await page.routeWebSocket(/\/api\/v1\/tasks\/msg_reply\/terminal/,ws=>{terminalSocket=ws;terminalConnections++;ws.send(JSON.stringify({type:'connected',session_id:'attempt',mode:'interactive'}));ws.send(Buffer.from('Actual terminal fixture\r\n'));ws.onMessage(m=>{const data=JSON.parse(String(m));if(data.type==='stdin')terminalInput.push(data.data)})})
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/inbox/fixture-room')
  const card=page.locator('[data-card="task-card"]');await card.getByRole('button',{name:'Terminal',exact:true}).click()
  const drawer=page.getByTestId('inbox-task-drawer');await drawer.waitFor();await drawer.locator('.xterm-screen').waitFor()
  const input=drawer.locator('.xterm-helper-textarea');await input.focus();await page.keyboard.type('x');await page.waitForTimeout(100)
  const connectionsBefore=terminalConnections, deliveredBefore=terminalInput.length
  await terminalSocket.close({code:1012,reason:'fixture reconnect'})
  await page.waitForTimeout(100);await input.focus();await input.press('Control+l')
  for(let n=0;n<40&&(terminalConnections===connectionsBefore||terminalInput.length===deliveredBefore);n++)await page.waitForTimeout(100)
  assert(terminalConnections>connectionsBefore,'same-attempt reconnect must occur');assert.equal(terminalInput.slice(deliveredBefore).join(''),'\u000c','only unsent offline input should flush once')
  await page.screenshot({path:out+'/task-terminal-'+width+'.png'});await drawer.getByRole('button',{name:'Close drawer',exact:true}).click()
  await card.getByRole('button',{name:'Detail',exact:true}).click();await drawer.waitFor();assert.equal(await drawer.locator('.xterm-screen').count(),0);await drawer.getByRole('button',{name:'Close drawer',exact:true}).click()
 }
 assert(terminalInput.length>=2,'Terminal must transmit real stdin frames in the fixture')
 assert.deepEqual(errors,[]);fs.writeFileSync(out+'/result.json',JSON.stringify({passed:true,viewports:[390,1440],catalogCalls},null,2));console.log('PASS model picker browser checks: '+out)
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
