// Browser render of the production build with fixture API; no provider writes.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-pm-recovery/terminal-redraw-browser'
let browser,server,fail=false,catalogCalls=0,longMessage=false
const task={id:'msg_reply',title:'Prepare the Studio guide',status:'blocked',source_message_id:'reply',project_id:'project',project_name:'Cuan fixture',summary:'Guide is ready for verification.',created_at:new Date().toISOString(),updated_at:new Date().toISOString(),latest_attempt:{id:'attempt',status:'canceled',error_message:'Cancelled'}}
const accounts=[{id:'signed',label:'Personal account',provider:'agy',credentials_present:true,state:'quota_limited',model_id:'saved-model',email:'owner@example.invalid'}, {id:'unsigned',label:'Not connected',provider:'agy',credentials_present:false,state:'login_required',model_id:''}]
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':p.endsWith('.woff2')?'font/woff2':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4182,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 const page=await browser.newPage({hasTouch:true});page.setDefaultTimeout(8000);const errors=[];page.on('pageerror',e=>errors.push(e.message))
 await page.addInitScript(()=>{
 localStorage.setItem('cuan_token','fixture-only');window.__terminalResizeObservers=[];
 const Original=window.ResizeObserver;
 window.ResizeObserver=class extends Original {constructor(callback){super(callback);this.callback=callback}observe(target,options){super.observe(target,options);if(target.querySelector('.xterm'))window.__terminalResizeObservers.push(()=>this.callback([],this))}};
})
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
 task.status='doing';task.latest_attempt.status='running';const terminalInput=[],terminalResizes=[],outputCredits=[];let terminalSocket,terminalConnections=0
 await page.routeWebSocket(/\/api\/v1\/tasks\/msg_reply\/terminal/,ws=>{terminalSocket=ws;terminalConnections++;ws.send(JSON.stringify({type:'connected',session_id:'attempt',mode:'interactive',output_ack:true}));ws.send(Buffer.from('Actual terminal fixture\r\n'));ws.onMessage(m=>{const data=JSON.parse(String(m));if(data.type==='output_ack')outputCredits.push({connection:terminalConnections,...data});if(data.type==='stdin')terminalInput.push(data.data);if(data.type==='resize')terminalResizes.push({connection:terminalConnections,...data})})})
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/inbox/fixture-room')
  const card=page.locator('[data-card="task-card"]');await card.getByRole('button',{name:'Terminal',exact:true}).click()
  const drawer=page.getByTestId('inbox-task-drawer');await drawer.waitFor();await drawer.locator('.xterm-screen').waitFor()
  const input=drawer.locator('.xterm-helper-textarea');await input.focus();await page.keyboard.type('x');await page.waitForTimeout(100)
  await input.press('Enter');await page.waitForTimeout(80);
  const imeStart=terminalInput.length;
  for(const value of ['j','ji','jiu','jiuu','jiuuh']){
    await input.evaluate((el,value)=>{el.value=value;el.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:value}));},value);
    terminalSocket.send(Buffer.from('\x1b[2K\rWorking output update\r\n'));
    await page.waitForTimeout(80);
  }
  assert.equal(terminalInput.slice(imeStart).join(''),'jiuuh','IME resends while output streams must deliver one word');
  await page.waitForTimeout(600);
  const currentConnection=terminalConnections;
  const initialResizes=terminalResizes.filter(x=>x.connection===currentConnection);
  assert.equal(initialResizes.length,1,'initial attach must settle on one PTY grid: '+JSON.stringify(initialResizes));
  const credits=outputCredits.filter(x=>x.connection===currentConnection);assert(credits.length>=1,'xterm must acknowledge parsed frames');assert.deepEqual(credits.map(x=>x.sequence),credits.map((_,i)=>i+1),'output credits preserve frame order');
  const beforeNoop=terminalResizes.length;
  await page.evaluate(()=>{for(let n=0;n<20;n++)window.__terminalResizeObservers.forEach(poke=>poke())});
  await page.waitForTimeout(450);
  assert.equal(terminalResizes.length,beforeNoop,'unchanged observer notifications must not resize the PTY');
  await input.evaluate(el=>el.value='pending SwiftKey context');
  terminalSocket.send(JSON.stringify({type:'connected',session_id:'attempt',mode:'interactive'}));
  await page.waitForTimeout(150);
  assert.equal(await input.inputValue(),'pending SwiftKey context','duplicate handshake must not reset the input');
  assert(await drawer.locator('.xterm-screen').isVisible());
  const beforeKeyboard=terminalResizes.length;
  for(const height of [810,760,700,620,560]){await page.setViewportSize({width,height});await page.waitForTimeout(15)}
  await page.waitForTimeout(500);
  assert(terminalResizes.length-beforeKeyboard<=1,'keyboard animation must coalesce into one final PTY resize');
  assert.equal(await input.inputValue(),'pending SwiftKey context','output and resizing must retain IME context');
  const connectionsBefore=terminalConnections, deliveredBefore=terminalInput.length
  await terminalSocket.close({code:1012,reason:'fixture reconnect'})
  await page.waitForTimeout(100);await input.focus();await input.press('Control+l')
  for(let n=0;n<40&&(terminalConnections===connectionsBefore||terminalInput.length===deliveredBefore);n++)await page.waitForTimeout(100)
  assert(terminalConnections>connectionsBefore,'same-attempt reconnect must occur');assert.equal(terminalInput.slice(deliveredBefore).join(''),'\u000c','only unsent offline input should flush once')
  await page.screenshot({path:out+'/task-terminal-'+width+'.png'});await drawer.getByRole('button',{name:'Close drawer',exact:true}).click()
  await card.getByRole('button',{name:'Detail',exact:true}).click();await drawer.waitFor();assert.equal(await drawer.locator('.xterm-screen').count(),0);await drawer.getByRole('button',{name:'Close drawer',exact:true}).click()
 }
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:900});await page.goto('http://127.0.0.1:4182/tasks/msg_reply/sessions/attempt');
  const composer=page.locator('#terminal_composer_input');await composer.waitFor();await composer.scrollIntoViewIfNeeded();
  const box=await composer.boundingBox();const attachment=page.getByRole('button',{name:'Add attachment',exact:true});const attachBox=await attachment.boundingBox();
  assert(Math.abs((box.y+box.height/2)-(attachBox.y+attachBox.height/2))<8,'input and attachment share one compact row');
  assert(box.height<=44,'empty terminal composer matches Inbox scale');
  await composer.fill('Inspect current progress');await page.screenshot({path:out+'/terminal-composer-'+width+'.png'});
  if(width===390){
    const keys=page.getByRole('toolbar',{name:'Terminal Mobile Toolbar'});
    assert.equal(await keys.count(),0,'accessory keys stay hidden without software keyboard');
    await composer.focus();await page.setViewportSize({width,height:500});await keys.waitFor();
    assert.equal(await page.getByRole('button',{name:'Toggle terminal keys',exact:true}).count(),0);
    assert.equal(await composer.inputValue(),'Inspect current progress');
    await page.screenshot({path:out+'/terminal-composer-keyboard-390.png'});
    await page.setViewportSize({width,height:900});await keys.waitFor({state:'detached'});
  }

  await composer.fill('');
 }
 assert(terminalInput.length>=2,'Terminal must transmit real stdin frames in the fixture')
 assert.deepEqual(errors,[]);fs.writeFileSync(out+'/result.json',JSON.stringify({passed:true,viewports:[390,1440],catalogCalls},null,2));console.log('PASS terminal grid stability, IME, streaming output and reconnect: '+out)
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
