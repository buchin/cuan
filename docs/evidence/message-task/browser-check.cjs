// Real Chromium + served assets + real fixture backend. No route/response mocks.
// The preview backend is explicitly fake-provider, temporary-HOME and in-memory.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const crypto = require('node:crypto')
const base = process.env.CUAN_BROWSER_BASE || 'https://cuan-message-lifecycle-dojo.fural.space'
const out = 'docs/evidence/message-task'
;(async () => {
 const browser = await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 let debugPage
 const result = { fixtureProvider:true, responseMocks:false, assetOverlay:false, errors:[], viewports:[], checks:[] }
 try {
  const context = await browser.newContext({viewport:{width:360,height:844},isMobile:true,hasTouch:true})
  const page = await context.newPage(); debugPage=page; page.setDefaultTimeout(12000)
  page.on('pageerror',e=>result.errors.push(e.message))
  await page.goto(base+'/inbox',{waitUntil:'domcontentloaded'})
  await page.locator('input[type=text]').first().fill('admin'); await page.locator('input[type=password]').fill('whosyourdaddy'); await page.locator('button[type=submit]').click()
  await page.getByRole('textbox',{name:'Pesan',exact:true}).waitFor()
  result.assets=[]
  for(const file of fs.readdirSync('web/dist/assets').filter(f=>/\.(js|css)$/.test(f))){const r=await page.request.get(base+'/assets/'+file);assert.equal(r.status(),200);const actual=await r.body();const expected=fs.readFileSync('web/dist/assets/'+file);assert(actual.equals(expected),'served asset differs: '+file);result.assets.push({file,bytes:actual.length,sha256:crypto.createHash('sha256').update(actual).digest('hex')})}
  const token = await page.evaluate(()=>localStorage.getItem('cuan_token'))
  const api = async(path,method='GET',body) => {
   const response=await page.request.fetch(base+path,{method,headers:{Authorization:'Bearer '+token},data:body})
   assert(response.ok(),`${method} ${path}: ${response.status()}`); return response.json()
  }
  const contacts=await api('/api/v1/chat/contacts'); const conv=contacts.contacts[0].conversation_id
  const stream=await page.evaluate(async id=>{const c=new AbortController();const t=setTimeout(()=>c.abort(),5000);const start=performance.now();try{const r=await fetch('/api/v1/chat/conversations/'+id+'/events',{headers:{Authorization:'Bearer '+localStorage.getItem('cuan_token')},signal:c.signal});const p=await r.body.getReader().read();return{status:r.status,bytes:p.value.length,firstChunkMs:performance.now()-start}}finally{clearTimeout(t);c.abort()}},conv)
  assert.equal(stream.status,200);assert(stream.bytes>0);result.stream=stream
  const nonce=Date.now();const first='FIXTURE first '+nonce;const second='FIXTURE queued follow-up '+nonce
  const send=async text=>{await page.getByRole('textbox',{name:'Pesan',exact:true}).fill(text);const response=page.waitForResponse(r=>r.url().endsWith('/messages')&&r.request().method()==='POST');await page.getByRole('button',{name:'Kirim pesan',exact:true}).click();const r=await response;assert.equal(r.status(),202);return (await r.json()).request}
  const one=await send(first);const two=await send(second)
  const card=text=>page.locator('.inbox-message-list > div').filter({has:page.getByText(text,{exact:true})}).first()
  await card(first).getByText('Agen sedang menyiapkan balasan',{exact:true}).waitFor()
  await card(second).getByText('Pesan diterima · menunggu giliran',{exact:true}).waitFor()
  await page.screenshot({path:out+'/running-360.png'})
  await card(second).getByRole('button',{name:'Batalkan',exact:true}).click();await card(second).getByText('Dibatalkan',{exact:true}).waitFor()
  await card(first).getByText('Balasan tersedia',{exact:true}).waitFor()
  const before=await api('/api/v1/chat/conversations/'+conv+'/messages')
  const same=await api('/api/v1/chat/conversations/'+conv+'/messages','POST',{request_id:one.request_id,message:first})
  assert.equal(same.request.reply_id,one.reply_id)
  await card(first).getByRole('button',{name:'Buat / lihat task',exact:true}).click()
  await card(first).getByLabel('Judul task').fill('Task from fixture '+nonce)
  await card(first).getByRole('button',{name:'Simpan task',exact:true}).click()
  await card(first).getByRole('link',{name:/Task:|Buka task dan sesi/}).waitFor()
  const captured=await api('/api/v1/tasks/msg_'+one.message_id)
  assert.equal(captured.status,'backlog');assert.equal(captured.source_message_id,one.message_id);assert.equal(captured.source_room_id,conv)
  await page.reload({waitUntil:'domcontentloaded'});await page.getByRole('textbox',{name:'Pesan',exact:true}).waitFor()
  await card(first).getByRole('link',{name:/Task:/}).waitFor()
  const after=await api('/api/v1/chat/conversations/'+conv+'/messages');assert.equal(after.messages.length,before.messages.length);assert.equal(after.requests.find(q=>q.request_id===two.request_id).state,'cancelled')
  for(const width of [360,390,1280]){await page.setViewportSize({width,height:844});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`overflow ${width}`);await page.screenshot({path:out+'/inbox-'+width+'.png'});result.viewports.push({width,height:844,noOverflow:true})}
  await card(first).getByRole('link',{name:/Task:/}).click();await page.getByRole('button',{name:'Agent Terminal',exact:true}).waitFor()
  const runResponse=page.waitForResponse(r=>r.url().endsWith('/run-agent')&&r.request().method()==='POST')
  await page.getByRole('button',{name:'Run Agent',exact:true}).click();assert.equal((await runResponse).status(),200)
  await page.getByText('Running',{exact:true}).first().waitFor()
  await page.getByText(/^connected$/i).waitFor()
  const running=await api('/api/v1/tasks/'+captured.id);assert.equal(running.latest_attempt.status,'running')
  const duplicate=await page.request.post(base+'/api/v1/tasks/'+captured.id+'/run-agent',{headers:{Authorization:'Bearer '+token},data:{agent_id:'agent_lifecycle'}});assert.equal(duplicate.status(),409)
  await page.getByText('Process finished · review required',{exact:true}).waitFor()
  const done=await api('/api/v1/tasks/'+captured.id);assert.equal(done.status,'review');assert.equal(done.latest_attempt.status,'succeeded');assert.equal(done.attempts.length,1)
  await page.getByText('Saved attempt output',{exact:true}).click();await page.getByText('Demo fixture: pesan diproses. Ini bukan respons provider nyata.',{exact:false}).last().waitFor()
  const logs=await api('/api/v1/tasks/'+captured.id+'/attempts/'+done.latest_attempt.id+'/logs');assert(logs.logs.some(l=>l.line.includes('Demo fixture')))
  await page.screenshot({path:out+'/task-review-1280.png'})
  // A second deliberate run can be cancelled; its late exit must not become success.
  const rerunResponse=page.waitForResponse(r=>r.url().endsWith('/run-agent')&&r.request().method()==='POST')
  await page.getByRole('button',{name:'Run Agent',exact:true}).click();assert.equal((await rerunResponse).status(),200);await page.getByRole('button',{name:'Cancel run',exact:true}).waitFor();await page.getByRole('button',{name:'Cancel run',exact:true}).click()
  await page.getByText('Cancelled',{exact:true}).waitFor();const cancelled=await api('/api/v1/tasks/'+captured.id);assert.equal(cancelled.status,'blocked');assert.equal(cancelled.latest_attempt.status,'canceled');assert.equal(cancelled.attempts.length,2)
  await page.goto(base+'/inbox/'+conv,{waitUntil:'domcontentloaded'});await page.getByRole('textbox',{name:'Pesan',exact:true}).waitFor();await card(first).getByRole('link',{name:/blocked/}).waitFor()
  result.checks=['live SSE','FIFO follow-up','queued cancellation','same-key reconciliation','refresh preserves messages','source task linkage','canonical task capture','single active session','durable attempt success requires review','retained output','task cancellation stays cancelled','task status returns to chat']
  assert.equal(result.errors.length,0)
  fs.writeFileSync(out+'/browser-results.json',JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result))
  await context.close()
 } catch(e) { if(debugPage){await debugPage.screenshot({path:out+'/browser-failure.png'});fs.writeFileSync(out+'/browser-failure.txt',e.message+'\n'+await debugPage.locator('body').innerText())} throw e } finally {await browser.close()}
})().catch(e=>{console.error(e.message);process.exitCode=1})
