const {chromium}=require('/home/dev/.local/lib/node_modules/playwright');
const http=require('http'),fs=require('fs'),assert=require('node:assert/strict');
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-pm-recovery/read-receipts';let server,browser;
(async()=>{
 fs.mkdirSync(out,{recursive:true});server=http.createServer((req,res)=>{if(req.url==='/chat/widget.js'){res.setHeader('content-type','text/javascript; charset=utf-8');return res.end(fs.readFileSync('web/dist/chat/widget.js'))}res.setHeader('content-type','text/html; charset=utf-8');res.end('<!doctype html><html><body><h1>Customer support</h1><script src="/chat/widget.js" data-product="Receipt fixture"></script></body></html>')});await new Promise(r=>server.listen(4182,'127.0.0.1',r));
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}});
 for(const width of [390,1440]){
  const ctx=await browser.newContext({viewport:{width,height:900}}),page=await ctx.newPage();page.setDefaultTimeout(7000);const acknowledgements=[],errors=[];page.on('pageerror',e=>errors.push(e.message));
  const messages=Array.from({length:30},(_,i)=>({id:i+1,uuid:'operator-'+i,sender_type:'operator',author_type:'user',author_name:'Support',message:'Update '+(i+1)+': Your request is being checked by our team.',created_at:new Date().toISOString()}));
  messages.push({id:31,uuid:'visitor-reply',sender_type:'visitor',message:'Thank you for the update.',created_at:new Date().toISOString(),delivered_at:new Date().toISOString(),read_confirmed_at:new Date().toISOString()});
  let failedOnce=false;
  await page.route('**/api/chat/**',async route=>{
   const p=new URL(route.request().url()).pathname,body=route.request().method()==='POST'?route.request().postDataJSON():null;
   if(p.endsWith('/session'))return route.fulfill({json:{session_uuid:'fixture-session'}});
   if(p.endsWith('/receipts')){acknowledgements.push(body);if(!failedOnce){failedOnce=true;return route.fulfill({status:503,json:{error:'transient'}})}return route.fulfill({json:{ok:true}})}
   if(p.endsWith('/page'))return route.fulfill({json:{ok:true}});
   const after=Number(new URL(route.request().url()).searchParams.get('after_id')||0);return route.fulfill({json:{messages:messages.filter(m=>m.id>after),receipts:[]}});
  });
  await page.goto('http://127.0.0.1:4182');await page.waitForTimeout(200);assert.equal(acknowledgements.length,0,'closed uninitialized widget acknowledged messages');
  await page.locator('#dojo-livechat-host').locator('.dojo-launcher').click();await page.waitForTimeout(3900);
  assert(acknowledgements.some(a=>a.delivered_ids.includes('operator-0')),'browser delivery missing');assert(!acknowledgements.some(a=>a.read_ids.includes('operator-0')),'offscreen history marked read');assert(acknowledgements.some(a=>a.read_ids.includes('operator-29')),'visible latest message not read');assert(acknowledgements.length>=2,'transient acknowledgement not retried');
  await page.evaluate(()=>{window.fixtureVisibility='hidden';Object.defineProperty(document,'visibilityState',{configurable:true,get:()=>window.fixtureVisibility});document.dispatchEvent(new Event('visibilitychange'))});
  messages.push({id:32,uuid:'hidden-new',sender_type:'operator',message:'New update while hidden',created_at:new Date().toISOString()});await page.waitForTimeout(3900);assert(!acknowledgements.some(a=>a.read_ids.includes('hidden-new')),'hidden document marked read');
  await page.evaluate(()=>{window.fixtureVisibility='visible';document.dispatchEvent(new Event('visibilitychange'))});await page.waitForTimeout(300);assert(acknowledgements.some(a=>a.read_ids.includes('hidden-new')),'returning to visible viewport did not acknowledge');
  await page.locator('#dojo-livechat-host').locator('.dojo-launcher').click();messages.push({id:33,uuid:'closed-new',sender_type:'operator',message:'Update while closed',created_at:new Date().toISOString()});await page.waitForTimeout(3900);assert(acknowledgements.some(a=>a.delivered_ids.includes('closed-new')));assert(!acknowledgements.some(a=>a.read_ids.includes('closed-new')),'closed widget marked read');
  await page.locator('#dojo-livechat-host').locator('.dojo-launcher').click();await page.waitForTimeout(300);assert(acknowledgements.some(a=>a.read_ids.includes('closed-new')));assert.equal(await page.locator('#dojo-livechat-host').getByLabel('Read in chat',{exact:true}).count(),1);await page.screenshot({path:out+'/widget-'+width+'.png'});assert.deepEqual(errors,[]);await ctx.close();
 }
 console.log('Widget 390/1440: offscreen, closed and hidden isolation; visibility return; delivery retry after 503.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()});
