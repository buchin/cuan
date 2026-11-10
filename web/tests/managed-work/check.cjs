const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
;(async () => {
 const browser = await chromium.launch({headless:true, executablePath:process.env.CHROMIUM_EXECUTABLE, args:['--single-process','--no-zygote','--disable-gpu']})
 try {
  const page = await browser.newPage()
  await page.addInitScript(() => localStorage.setItem('cuan-language','id'))
  const url = process.env.CUAN_VISUAL_URL || 'http://127.0.0.1:5174'
  for (const width of [390,1440]) {
   await page.setViewportSize({width,height:850})
   await page.goto(url+'/tests/composer-tasks/fixture.html')
   await page.getByRole('button',{name:'Work paused',exact:true}).click()
   await page.getByRole('button',{name:/Tugas/}).click()
   assert.equal(await page.getByRole('button',{name:/Lanjutkan|Resume|Retry|Coba lagi/}).count(),0)
   assert.equal(await page.locator('.composer-tasks').getByText('Sedang dikerjakan',{exact:true}).count(),3)
   assert.equal(await page.locator('.composer-tasks-panel [data-status-dot="working"]').count(),3,'Internal recovery remains Cuan-owned')
   await page.getByRole('button',{name:'Needs owner',exact:true}).click()
   assert.equal(await page.locator('.composer-tasks-panel [data-status-dot="working"]').count(),0)
   await page.getByRole('button',{name:'Work paused',exact:true}).click()
   await page.getByLabel('Message').fill('Masukan user tetap tersimpan')
   await page.screenshot({path:`/tmp/cuan-managed-work-${width}.png`})
   await page.route('**/api/v1/tasks/paused',route=>route.fulfill({json:{id:'paused',title:'Verifikasi hasil perbaikan rilis',status:'blocked',source_room_id:'room',review_action:'continue',latest_attempt:{id:'finished',status:'failed'},attempts:[],activities:[],created_at:'2026-11-10T08:00:00Z'}}))
   await page.goto(url+'/tests/managed-work/fixture.html')
   await page.getByTestId('inbox-work-panel').waitFor()
   await page.getByText('Belum ada progres tercatat.').waitFor()
   assert.equal(await page.getByRole('button',{name:/Lanjutkan|Resume|Retry|Coba lagi/}).count(),0)
   await page.screenshot({path:`/tmp/cuan-managed-detail-${width}.png`})
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
  }
 } finally { await browser.close() }
 console.log('390px/desktop: no managed Resume/Retry, work ownership status, reachable composer/details passed')
})().catch(error=>{console.error(error);process.exit(1)})
