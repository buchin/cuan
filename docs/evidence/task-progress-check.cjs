// Real frontend build, isolated fixture APIs. No production messages/tasks are created.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-inbox-noise',now=new Date().toISOString()
const projects=[
 {id:'large',name:'Cuan — banyak pekerjaan',repo_url:'https://github.com/buchin/cuan',task_progress:{total:5002,done:1001,canceled:2000,active:1000,queued:1000,review:1001,blocked:1000}},
 {id:'complete',name:'Panduan penggunaan',repo_url:'',task_progress:{total:5,done:5,canceled:0,active:0,queued:0,review:0,blocked:0}},
 {id:'mixed',name:'Pekerjaan tersisa selesai',repo_url:'',task_progress:{total:2,done:2,canceled:2,active:0,queued:0,review:0,blocked:0}},
 {id:'waiting',name:'Rencana konten',repo_url:'',task_progress:{total:3,done:0}},
 {id:'empty',name:'Ide baru',repo_url:'',task_progress:{total:0,done:0}},
].map(p=>({...p,project_type:'custom',is_archived:false,created_at:now,updated_at:now,metadata:{}}))
const report=status=>'### Evaluasi Kriteria Selesai (Definition of Done)\n**Status:** '+status+'\nBukti yang terverifikasi: dokumen tersedia.'
const contacts=[
 {id:'contact',conversation_id:'fixture-room',name:'Siapkan panduan untuk pelanggan',task_count:7002,done_tasks:1001,canceled_tasks:2000,active_tasks:1,queued_tasks:1,review_tasks:1,failed_tasks:1,last_message:'Draf sudah tersedia. Saya lanjutkan pekerjaan berikutnya.'},
 {id:'complete',conversation_id:'complete-room',name:'Panduan penggunaan',task_count:5,done_tasks:5,last_message:report('LULUS')},
 {id:'mixed',conversation_id:'mixed-room',name:'Pekerjaan tersisa selesai',task_count:4,done_tasks:2,canceled_tasks:2,last_message:report('LULUS')},
 {id:'problem',conversation_id:'problem-room',name:'Hasil perlu diperbaiki',task_count:1,done_tasks:0,failed_tasks:1,last_message:report('BELUM LULUS (Perlu revisi)')},
 {id:'problemplain',conversation_id:'plain-problem-room',name:'Koneksi perlu diperiksa',task_count:1,done_tasks:0,failed_tasks:1,last_message:'Saya sudah memeriksa koneksinya.'},
 {id:'waiting',conversation_id:'waiting-room',name:'Rencana konten',task_count:3,done_tasks:0,review_tasks:1,last_message:report('PERLU DIREVIU')},
 {id:'canceled',conversation_id:'canceled-room',name:'Pekerjaan dibatalkan',task_count:1,done_tasks:0,canceled_tasks:1,review_tasks:0,last_message:'Pekerjaan dibatalkan. Hasil dan kriteria selesai belum diverifikasi.'},
 {id:'single',conversation_id:'single-room',name:'Satu tugas selesai',task_count:1,done_tasks:1,last_message:report('LULUS')},
 {id:'empty',conversation_id:'empty-room',name:'Diskusi ide baru',task_count:0,done_tasks:0,last_message:'Mari bahas kebutuhan Anda.'},
 {id:'unknown',conversation_id:'unknown-room',name:'Status belum tersedia',last_message:'Percakapan tanpa data progres.'},
].map(c=>({...c,type:'agent',agent_id:'agent_cuan',agent_name:'@cuan',unread_count:c.id==='problemplain'?3:c.id==='mixed'?2:0,status:'idle',updated_at:now}))
const task={id:'msg_reply',title:'Siapkan panduan pelanggan',status:'done',project_id:'large',project_name:'Cuan',summary:'Panduan selesai dan telah diverifikasi.',source_message_id:'reply',created_at:now,updated_at:now,latest_attempt:{id:'attempt',status:'succeeded'}}
let server,browser,debugPage
async function routeAPI(route){
 const p=new URL(route.request().url()).pathname,json=body=>route.fulfill({contentType:'application/json',body:JSON.stringify(body)})
 if(p.endsWith('/auth/status'))return json({setup_required:false})
 if(p.endsWith('/auth/me'))return json({owner:{id:'fixture',username:'Owner',role:'owner'}})
 if(p==='/api/v1/projects')return json({projects})
 if(p.startsWith('/api/v1/projects/'))return json(projects.find(x=>x.id===p.split('/')[4])||{})
 if(p==='/api/v1/tasks')return json({tasks:[task]})
 if(p==='/api/v1/tasks/msg_reply')return json({...task,attempts:[task.latest_attempt],activities:[]})
 if(p.endsWith('/run-agent/history'))return json({history:[],live_session:null})
 if(p==='/api/v1/chat/contacts')return json({contacts})
 if(p==='/api/v1/contacts')return json({contacts:[]})
 if(p==='/api/v1/item-preferences')return json({items:{}})
 if(p==='/api/v1/agents')return json({agents:[{id:'agent_cuan',name:'@cuan',enabled:true}]})
 if(p==='/api/v1/notifications')return json({notifications:[],unread_count:0})
 if(p==='/api/system/worker-health')return json({status:'ready',is_alive:true,running_count:0,queued_count:0})
 if(p==='/api/system/update')return json({current_version:'fixture',latest_version:'fixture',has_update:false})
  if(/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){
   const data={conversation_id:'fixture-room',room_id:'fixture-room',contact:contacts[0],tasks:[task],messages:[{id:1,uuid:'message',author_type:'user',sender_type:'visitor',content:'Buatkan panduan untuk pelanggan.',created_at:now},{id:11,uuid:'user-followup',author_type:'user',sender_type:'visitor',content:'Gunakan bahasa yang sederhana.',created_at:now},{id:2,uuid:'reply',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'Panduan sudah selesai dan diperiksa.\n\n[View work](/tasks/msg_reply)',created_at:now},{id:3,uuid:'agent-followup',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'Silakan cek file hasilnya.',created_at:now},{id:4,uuid:'file-only',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'',message:'',attachments:[{id:'fixture-file',name:'panduan.pdf',type:'application/pdf',size:1024,url:'/fixtures/panduan.pdf'}],created_at:now},{id:5,uuid:'user-later',author_type:'user',sender_type:'visitor',content:'Terima kasih, hasilnya sudah sesuai.',created_at:new Date(Date.parse(now)+360000).toISOString()}],requests:[]}
   return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(data)+'\n\n'}):json(data)
  } return json({})
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':p.endsWith('.woff2')?'font/woff2':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4183,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 const errors=[]
 for(const theme of ['dark','light'])for(const width of [390,1440]){
  const context=await browser.newContext({viewport:{width,height:900},isMobile:width===390,hasTouch:width===390})
  const page=await context.newPage();debugPage=page;page.setDefaultTimeout(10000);page.on('pageerror',e=>errors.push(e.message))
  await page.addInitScript(theme=>{localStorage.setItem('cuan_token','fixture-only');localStorage.setItem('cuan-theme',theme)},theme)
  await page.route('**/api/**',routeAPI)
  await page.goto('http://127.0.0.1:4183/inbox/fixture-room')
  if(width===390)await page.getByRole('button',{name:'Back to conversations',exact:true}).click()
  const rows=page.locator('[data-conversation-row]:visible'),row=name=>rows.filter({hasText:name})
  await rows.first().waitFor()
  const avatar=rows.first().locator('[data-avatar]');assert.equal(await avatar.evaluate(el=>getComputedStyle(el).borderRadius),'0px')
  const account=page.locator('button[aria-label="Account menu"]:visible').first();const accountAvatar=account.locator('[data-avatar]');assert.equal(await accountAvatar.evaluate(el=>getComputedStyle(el).borderRadius),'0px');assert.equal(Math.round((await accountAvatar.boundingBox()).width),36)
  await account.click();const menuAvatar=page.locator('[data-account-popover] [data-avatar]');await menuAvatar.waitFor();assert.equal(await menuAvatar.evaluate(el=>getComputedStyle(el).borderRadius),'0px');assert.equal(Math.round((await menuAvatar.boundingBox()).width),40)
  await page.screenshot({path:`${out}/square-account-${theme}-${width}.png`});await account.click()

  const initialURL=page.url()
  const progressRow=rows.first(),progressBounds=await progressRow.locator('[data-task-progress]').boundingBox(),rowBounds=await progressRow.boundingBox();assert(progressBounds.x>=rowBounds.x+rowBounds.width-48,'pie stays in top-right action area');assert.equal(await page.locator('[data-unread-dot]').count(),0,'unread uses typography, not a second circle');
  const largeInbox=rows.first().locator('[data-task-progress]');assert.equal(await largeInbox.getAttribute('data-task-progress'),'partial')
  assert.equal(await row('Panduan penggunaan').locator('[data-task-progress]').getAttribute('data-task-progress'),'complete')
  const mixed=row('Pekerjaan tersisa selesai').locator('[data-task-progress]');assert.equal(await mixed.getAttribute('data-task-progress'),'complete-with-cancellations');assert((await mixed.getAttribute('class')).includes('text-muted-foreground'))
  for(const name of ['Hasil perlu diperbaiki','Rencana konten','Pekerjaan dibatalkan','Satu tugas selesai','Diskusi ide baru','Status belum tersedia'])assert.equal(await row(name).locator('[data-task-progress]').count(),0)
  assert(await row('Hasil perlu diperbaiki').getByTestId('task-badge-attention').isVisible(),'zero progress must retain a visible issue marker')
  assert(await row('Hasil perlu diperbaiki').getByText('The result needs attention.',{exact:true}).isVisible())
  assert(await row('Panduan penggunaan').getByText('Work completed and verified.',{exact:true}).isVisible())
  assert.equal(await row('Pekerjaan dibatalkan').getByTestId('task-badge-review').count(),0)
  assert(await row('Pekerjaan dibatalkan').getByText('Work canceled. Saved work is still available.',{exact:true}).isVisible())
  assert(await row('Rencana konten').getByTestId('task-badge-review').isVisible())
  assert(await row('Rencana konten').getByText('Result ready for review.',{exact:true}).isVisible())
  assert.equal(await row('Rencana konten').getByTestId('task-badge-attention').count(),0)
  assert(await row('Koneksi perlu diperiksa').getByTestId('task-badge-attention').isVisible())
  assert.equal(await row('Koneksi perlu diperiksa').getByText('3 unread messages',{exact:true}).count(),1);assert.equal(await row('Panduan penggunaan').locator('[data-unread-count]').count(),0);assert(Number(await row('Koneksi perlu diperiksa').getByText('Koneksi perlu diperiksa',{exact:true}).evaluate(el=>getComputedStyle(el).fontWeight))>=600)
  assert.equal(await row('Koneksi perlu diperiksa').locator('[data-task-progress]').count(),0)
  assert(await row('Koneksi perlu diperiksa').getByText('Saya sudah memeriksa koneksinya.',{exact:true}).isVisible())
  const menu=rows.first().locator('xpath=ancestor::*[@data-item-actions-quiet][1]').locator('[data-item-menu]');assert.equal(await menu.evaluate(el=>getComputedStyle(el).visibility),'hidden')
  if(width===1440){await rows.first().hover();assert.equal(await menu.evaluate(el=>getComputedStyle(el).visibility),'visible');await page.mouse.move(0,0)}
  else {
   const touch=row('Hasil perlu diperbaiki').getByTestId('conversation-item')
   await touch.dispatchEvent('pointerdown',{pointerType:'touch',pointerId:1,clientX:150,clientY:400,bubbles:true});await page.waitForTimeout(650);await page.getByRole('menu').waitFor();await touch.dispatchEvent('pointerup',{pointerType:'touch',pointerId:1,bubbles:true});await touch.dispatchEvent('click',{bubbles:true})
   assert.equal(page.url(),initialURL);await page.keyboard.press('Escape');await page.getByRole('menu').waitFor({state:'detached'})
  }
  await page.evaluate(()=>document.activeElement?.blur());await page.screenshot({path:`${out}/inbox-${theme}-${width}.png`})
  await mixed.click();const detail=page.getByRole('dialog',{name:'Task progress',exact:true});await detail.waitFor();assert(await detail.getByText('All remaining work completed',{exact:true}).isVisible());assert(await detail.getByText('Canceled',{exact:true}).isVisible());assert.equal(page.url(),initialURL)
  const rect=await detail.boundingBox();assert(rect.x>=0&&rect.x+rect.width<=width)
  assert((await mixed.boundingBox()).width>=44);assert.equal(await mixed.locator('span').first().evaluate(el=>el.getBoundingClientRect().width),16)
  await page.screenshot({path:`${out}/breakdown-${theme}-${width}.png`});await page.keyboard.press('Escape');await detail.waitFor({state:'detached'});assert.equal(await row('Pekerjaan tersisa selesai').getByText('2 unread messages',{exact:true}).count(),1,'progress inspection must not mark conversation read')
  if(width===1440)await page.mouse.move(0,0)
  await largeInbox.focus();await page.keyboard.press('Enter');await detail.waitFor();assert(await detail.getByText('Working',{exact:true}).isVisible());await page.keyboard.press('Escape');await detail.waitFor({state:'detached'})
  await page.goto('http://127.0.0.1:4183/projects');const project=page.locator('[data-project-row]').filter({hasText:'Cuan — banyak pekerjaan'});await project.waitFor();const large=project.locator('[data-task-progress]')
  assert.equal(await large.getAttribute('aria-label'),'1,001 of 5,002 tasks completed · 2,000 canceled')
  assert.equal(await page.locator('[data-project-row]').filter({hasText:'Rencana konten'}).locator('[data-task-progress]').count(),0)
  assert.equal(await page.locator('button button,a button').count(),0,'progress triggers must be siblings of navigation buttons/links')
  if(width===1440)await page.mouse.move(0,0)
  await page.screenshot({path:`${out}/projects-${theme}-${width}.png`})
  const projectURL=page.url();await large.click();await detail.waitFor();assert(await detail.getByText('In review',{exact:true}).isVisible());assert(await detail.getByText('Needs attention',{exact:true}).isVisible());assert.equal(page.url(),projectURL);await detail.getByRole('button',{name:'Close task progress',exact:true}).click();await detail.waitFor({state:'detached'})
  await project.getByRole('link').click();assert(page.url().includes('/projects/large/settings'))
  await page.goto('http://127.0.0.1:4183/inbox/fixture-room');const card=page.locator('[data-card=task-card]');await card.waitFor();assert((await card.locator('[data-task-status=Completed] svg').getAttribute('class')).includes('--status-success'))
  for(const id of ['user-followup','agent-followup','file-only'])assert.equal(await page.locator(`[data-message-id=${id}] [data-bubble-tail]`).count(),0)
  assert.equal(await page.locator('[data-message-id=user-later] [data-bubble-tail=outgoing]').count(),1);assert.equal(await page.locator('[data-card] [data-bubble-tail]').count(),0)
  await card.scrollIntoViewIfNeeded();await page.screenshot({path:`${out}/completed-${theme}-${width}.png`})
  if(theme==='light'&&width===1440){
   await page.evaluate(()=>localStorage.setItem('cuan-language','id'));await page.goto('http://127.0.0.1:4183/projects');await page.getByLabel('1.001 dari 5.002 tugas selesai · 2.000 dibatalkan',{exact:true}).waitFor()
   Object.assign(projects[0].task_progress,{done:5002,active:0,queued:0,review:0,blocked:0});const finished=page.getByLabel('5.002 dari 5.002 tugas selesai · 2.000 dibatalkan',{exact:true});await finished.waitFor();assert.equal(await finished.getAttribute('data-task-progress'),'complete-with-cancellations')
  }
  await context.close();debugPage=null
 }
 assert.deepEqual(errors,[]);console.log('Passed390/1440 light/dark: zero/single suppression, visible issue markers, truthful previews, neutral cancellations, tap/hover/keyboard breakdown,44px targets, long-press menus, navigation isolation, exact counts, live refresh, localized labels and speech grouping.')
})().catch(async e=>{console.error(e);if(debugPage){await debugPage.screenshot({path:out+'/failure.png'});fs.writeFileSync(out+'/failure.txt',await debugPage.locator('body').innerText())}process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)server.close()})
