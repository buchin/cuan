const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({headless:true,args:['--single-process','--no-zygote','--disable-gpu'],...(process.env.CHROMIUM_EXECUTABLE?{executablePath:process.env.CHROMIUM_EXECUTABLE}:{})});
 try {
 const page=await browser.newPage();page.setDefaultTimeout(15000);page.setDefaultNavigationTimeout(15000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
 const baseUrl=process.env.CUAN_VISUAL_URL||'http://127.0.0.1:5174';
 await page.addInitScript(()=>localStorage.setItem('cuan-language','en'));
 await page.route('**/api/v1/item-preferences?kind=conversation',r=>r.fulfill({json:{items:{}}}));
 await page.route('**/api/v1/chat/conversations/*/resolution',r=>{const id=r.request().url().split('/').at(-2);const body=r.request().postDataJSON();return r.fulfill({json:{id,resolved:body.resolved,resolved_at:body.resolved?body.observed_at:null}})});
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:850});await page.goto(`${baseUrl}/tests/conversation-resolution/fixture.html`);
  await page.getByRole('tab',{name:'Unresolved',exact:true}).waitFor({timeout:15000});
  assert.equal(await page.getByTestId('conversation-item').count(),3);
  await page.getByRole('tab',{name:'All',exact:true}).click();assert.equal(await page.getByTestId('conversation-item').count(),4);
  await page.getByRole('button',{name:'Resolved conversations',exact:true}).click();assert.equal(await page.getByTestId('conversation-item').count(),1);
  await page.getByRole('tab',{name:'Unresolved',exact:true}).click();
  await page.screenshot({path:`/tmp/cuan-resolution-${width}.png`});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'horizontal overflow');
  page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Resolve conversations',exact:true}).click();
  await page.waitForFunction(()=>document.querySelectorAll('[data-testid=conversation-item]').length===1);
  if(width===1440){await page.getByRole('button',{name:'Tasks',exact:false}).click();await page.screenshot({path:'/tmp/cuan-resolution-panel-1440.png'});await page.getByRole('button',{name:'Reopen conversation',exact:true}).first().click();await page.waitForFunction(()=>document.querySelectorAll('[data-testid=conversation-item]').length===2)}
 }
 await page.setViewportSize({width:390,height:850});await page.goto(`${baseUrl}/tests/conversation-resolution/fixture.html?chat`);
 await page.getByRole('button',{name:/Tasks/}).click();assert.equal(await page.locator('.composer-tasks').getByRole('button',{name:/Resolve conversation|Reopen conversation/}).count(),0);assert.equal(await page.getByRole('button',{name:'Resolve conversation',exact:true}).count(),1);await page.screenshot({path:'/tmp/cuan-resolution-panel-390.png'});
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 assert.deepEqual(errors,[]);console.log('Resolved/unresolved filtering, bulk completion, reopen and 390/1440px layout pass.');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
