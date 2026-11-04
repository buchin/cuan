const taskPageFixture=require('./task-page-fixture.cjs')
// Real frontend build, isolated fixture APIs. No production messages/tasks are created.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-pm-recovery/task-row-menu',now=new Date().toISOString()
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
].map(c=>({...c,type:c.id==='contact'?'customer':'agent',agent_id:'agent_cuan',agent_name:'@cuan',unread_count:c.id==='complete'?7:0,status:'idle',updated_at:now}))
const task={id:'msg_reply',title:'Siapkan panduan pelanggan',status:'done',project_id:'large',project_name:'Cuan',summary:'Panduan selesai dan telah diverifikasi.',source_message_id:'reply',created_at:now,updated_at:now,latest_attempt:{id:'attempt',status:'succeeded'}}
const working=process.env.CUAN_CARD_STATUS==='working';if(working){task.status='doing';task.latest_attempt.status='running'}
const taskWrites=[];let deleted=false,cancelled=false;const activeTask={...task,id:"msg_active",title:"Active work",status:"doing",latest_attempt:{id:"active-attempt",status:"running"}};
let server,browser,debugPage
async function routeAPI(route){
 const p=new URL(route.request().url()).pathname,json=body=>route.fulfill({contentType:'application/json',body:JSON.stringify(taskPageFixture(route.request().url(),body))})
 if(p.endsWith('/auth/status'))return json({setup_required:false})
 if(p.endsWith('/auth/me'))return json({owner:{id:'fixture',username:'Owner',role:'owner'}})
 if(p==='/api/v1/projects')return json({projects})
 if(p.startsWith('/api/v1/projects/'))return json(projects.find(x=>x.id===p.split('/')[4])||{})
 if(p==='/api/v1/tasks')return json({tasks:[...(deleted?[]:[task]),{...activeTask,status:cancelled?'blocked':'doing',latest_attempt:{...activeTask.latest_attempt,status:cancelled?'canceled':'running'}}]})
 if(p==='/api/v1/tasks/msg_active/cancel'&&route.request().method()==='POST'){taskWrites.push('cancel');cancelled=true;return json({id:'active-attempt',status:'canceled'})}
 if(p==='/api/v1/tasks/msg_reply'&&route.request().method()==='DELETE'){taskWrites.push('delete');deleted=true;return json({})}
 if(p==='/api/v1/tasks/msg_reply')return json({...task,attempts:[task.latest_attempt],activities:[]})
 if(p.endsWith('/run-agent/history'))return json({history:[],live_session:working?{attempt_id:'attempt'}:null})
 if(p==='/api/v1/chat/contacts')return json({contacts})
 if(p==='/api/v1/contacts')return json({contacts:[]})
 if(p==='/api/v1/item-preferences')return json({items:{}})
 if(p==='/api/v1/agents')return json({agents:[{id:'agent_cuan',name:'@cuan',enabled:true}]})
 if(p==='/api/v1/notifications')return json({notifications:[],unread_count:0})
 if(p==='/api/system/worker-health')return json({status:'ready',is_alive:true,running_count:0,queued_count:0})
 if(p==='/api/system/update')return json({current_version:'fixture',latest_version:'fixture',has_update:false})
  if(/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){
   const data={conversation_id:'fixture-room',room_id:'fixture-room',contact:contacts[0],tasks:[task],messages:[{id:1,uuid:'message',author_type:'user',sender_type:'visitor',content:'Buatkan panduan untuk pelanggan.',created_at:now},{id:11,uuid:'user-followup',author_type:'user',sender_type:'visitor',content:'Gunakan bahasa yang sederhana.',created_at:now},{id:2,uuid:'reply',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'Panduan sudah selesai dan diperiksa.\n\n[View work](/tasks/msg_reply)',created_at:now},{id:3,uuid:'agent-followup',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'Silakan cek file hasilnya.',created_at:now},{id:4,uuid:'file-only',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'',message:'',attachments:[{id:'fixture-file',name:'panduan.md',type:'text/markdown',size:1024,url:'/api/v1/chat/attachments/private-file'}],created_at:now},{id:5,uuid:'user-later',author_type:'user',sender_type:'visitor',content:'Terima kasih, hasilnya sudah sesuai.',created_at:new Date(Date.parse(now)+360000).toISOString()} ,{id:6,uuid:'draft-reply',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'Kami sudah meninjau laporan Anda.',reply_draft:'Terima kasih sudah menunggu. Kami sedang memeriksa kendalanya.',created_at:now}],requests:[]}
   return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(data)+'\n\n'}):json(data)
  } return json({})
}
contacts[0].name='Heru Purwanto dengan nama yang panjang';contacts[0].email='herusip10.dengan.email.panjang@example.com'
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4183,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [390,1440])for(const theme of ['dark','light']){
 deleted=false;cancelled=false;taskWrites.length=0;
 const context=await browser.newContext({viewport:{width,height:900},isMobile:width===390,hasTouch:width===390}),page=await context.newPage();debugPage=page;
 await page.addInitScript(theme=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-theme',theme)},theme)
 await page.addInitScript(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.copiedFixtureEmail=text}}}));
 await page.route('**/api/**',routeAPI)
 await page.route('**/api/v1/chat/attachments/private-file',route=>route.fulfill({contentType:'text/markdown',body:'# Panduan privat\nIsi lampiran internal.'}))
 await page.goto('http://127.0.0.1:4183/inbox/fixture-room');const identity=page.locator('.wa-chat-header [data-avatar]');await identity.waitFor();const identityBox=await identity.boundingBox();assert(parseFloat(await identity.evaluate(el=>getComputedStyle(el).borderRadius))>=identityBox.width/2);await page.screenshot({path:out+'/avatars-'+theme+'-'+width+'.png'});
 await page.goto('http://127.0.0.1:4183/tasks');
 await page.goto('http://127.0.0.1:4183/tasks');const unread=page.getByTestId(width===390?'inbox-unread-badge-bottom':'inbox-unread-badge-rail');await unread.waitFor();const badgeBox=await unread.boundingBox();assert.equal(badgeBox.width,badgeBox.height);assert(parseFloat(await unread.evaluate(el=>getComputedStyle(el).borderRadius))>=badgeBox.height/2);const trigger=page.getByRole('button',{name:'Actions for '+task.title,exact:true}).filter({visible:true});await trigger.waitFor();assert((await trigger.boundingBox()).width>=44);await trigger.click();const menu=page.getByRole('menu',{name:'Actions for '+task.title,exact:true});await menu.waitFor();assert.equal(page.url(),'http://127.0.0.1:4183/tasks');assert.equal(await menu.getByRole('menuitem',{name:'Cancel task',exact:true}).count(),0);await page.screenshot({path:out+'/menu-'+theme+'-'+width+'.png'});await page.keyboard.press('Escape');await menu.waitFor({state:'detached'});assert(await trigger.evaluate(el=>el===document.activeElement));await page.keyboard.press('Enter');await menu.waitFor();await menu.getByRole('menuitem',{name:'Copy link',exact:true}).click();assert.equal(await page.evaluate(()=>window.copiedFixtureEmail),'http://127.0.0.1:4183/tasks/msg_reply');assert.equal(page.url(),'http://127.0.0.1:4183/tasks');await trigger.click();await menu.getByRole('menuitem',{name:'Delete task',exact:true}).click();const confirmation=page.getByRole('dialog',{name:'Delete task',exact:true});await confirmation.waitFor();assert.equal(taskWrites.length,0);await confirmation.getByRole('button',{name:'Keep task',exact:true}).click();await trigger.click();await page.locator('h1').first().click();await menu.waitFor({state:'detached'});await trigger.click();await menu.getByRole('menuitem',{name:'Delete task',exact:true}).click();await confirmation.getByRole('button',{name:'Delete task',exact:true}).click();await confirmation.waitFor({state:'detached'});assert.deepEqual(taskWrites,['delete']);const activeTrigger=page.getByRole('button',{name:'Actions for Active work',exact:true}).filter({visible:true});await activeTrigger.click();const activeMenu=page.getByRole('menu',{name:'Actions for Active work',exact:true});assert.equal(await activeMenu.getByRole('menuitem',{name:'Delete task',exact:true}).count(),0);await activeMenu.getByRole('menuitem',{name:'Cancel task',exact:true}).click();const cancelDialog=page.getByRole('dialog',{name:'Cancel task',exact:true});await cancelDialog.waitFor();await cancelDialog.getByRole('button',{name:'Cancel task',exact:true}).click();await cancelDialog.waitFor({state:'detached'});assert.deepEqual(taskWrites,['delete','cancel']);assert.equal(page.url(),'http://127.0.0.1:4183/tasks');
 await context.close()
 }
 console.log('Task row menus390/1440 light/dark: pointer, keyboard, copy, confirmation, delete and cancel verified without opening details.')
})().catch(async e=>{console.error(e);if(debugPage)await debugPage.screenshot({path:out+'/error.png'});process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
