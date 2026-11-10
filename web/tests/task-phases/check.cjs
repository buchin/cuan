const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright')
const assert=require('node:assert/strict')
;(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE,args:['--single-process','--no-zygote','--disable-gpu']})
 try{
  const page=await browser.newPage()
  await page.addInitScript(()=>localStorage.setItem('cuan-language','id'))
  await page.route('**/api/v1/tasks/release/run-agent/history',r=>r.fulfill({json:{history:[]}}))
  for(const width of [390,1440]){
   await page.setViewportSize({width,height:950})
   await page.goto((process.env.CUAN_VISUAL_URL||'http://127.0.0.1:5174')+'/tests/task-phases/fixture.html')
   const rows=page.locator('[data-conversation-row]')
   assert.equal(await rows.count(),4)
   assert.equal(await rows.nth(0).locator('[data-status-dot="working"]').count(),1,'queued work must stay visible and pulse')
   assert.equal(await rows.nth(1).locator('[data-status-dot="working"]').count(),1,'checks retain ownership pulse')
   assert.equal(await rows.nth(2).locator('[data-status-dot="working"]').count(),0,'owner action stays static')
   await page.locator('.composer-tasks').getByRole('button',{name:/Tugas/}).click()
   await page.getByText('Menunggu rilis',{exact:true}).waitFor()
   assert.equal(await page.getByRole('button',{name:'Terminal langsung',exact:true}).count(),0)
   await page.screenshot({path:`/tmp/cuan-task-phases-${width}.png`})
   await page.getByRole('button',{name:'Deploy now',exact:true}).click()
   await page.getByText('Merilis',{exact:true}).waitFor()
   await page.screenshot({path:`/tmp/cuan-task-deploying-${width}.png`})
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
  }
 }finally{await browser.close()}
 console.log('390/1440px: visible queue/check pulse, shared solid dot, static owner state, real release labels and no phantom terminal passed')
})().catch(e=>{console.error(e);process.exit(1)})
