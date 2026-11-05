const taskPageFixture=require('/home/dev/repos/cuan/docs/evidence/task-page-fixture.cjs')
// Real frontend build, isolated fixture APIs. No production messages/tasks are created.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-pm-recovery/task-attention',now=new Date().toISOString()
const projects=[
 {id:'large',name:'Cuan — banyak pekerjaan',repo_url:'https://github.com/buchin/cuan',task_progress:{total:5002,done:1001,canceled:2000,active:1000,queued:1000,review:1001,blocked:1000}},
 {id:'complete',name:'Panduan penggunaan',repo_url:'',task_progress:{total:5,done:5,canceled:0,active:0,queued:0,review:0,blocked:0}},
 {id:'mixed',name:'Pekerjaan tersisa selesai',repo_url:'',task_progress:{total:2,done:2,canceled:2,active:0,queued:0,review:0,blocked:0}},
 {id:'waiting',name:'Rencana konten',repo_url:'',task_progress:{total:3,done:0}},
 {id:'empty',name:'Ide baru',repo_url:'',task_progress:{total:0,done:0}},
].map(p=>({...p,project_type:'custom',is_archived:false,created_at:now,updated_at:now,metadata:{}}))
const proposal={title:'Next step',description:'Run the missing checks against the saved work and report the verified result.',project_id:'large',continuation:{task_id:'msg_reply',attempt_id:'attempt'}}
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
let server,browser,debugPage
async function routeAPI(route,{standalone=false,wideSpeech=false}={}){
 const p=new URL(route.request().url()).pathname,json=body=>route.fulfill({contentType:'application/json',body:JSON.stringify(taskPageFixture(route.request().url(),body))})
 if(p.endsWith('/auth/status'))return json({setup_required:false})
 if(p.endsWith('/auth/me'))return json({owner:{id:'fixture',username:'Owner',role:'owner'}})
 if(p==='/api/v1/projects')return json({projects})
 if(p.startsWith('/api/v1/projects/'))return json(projects.find(x=>x.id===p.split('/')[4])||{})
 if(p==='/api/v1/tasks')return json({tasks:[task]})
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
   const data={conversation_id:'fixture-room',room_id:'fixture-room',contact:contacts[0],tasks:[task],messages:[{id:1,uuid:'message',author_type:'user',sender_type:'visitor',content:'Buatkan panduan untuk pelanggan.',created_at:now},{id:11,uuid:'user-followup',author_type:'user',sender_type:'visitor',content:'Gunakan bahasa yang sederhana.',created_at:now},{id:2,uuid:'reply',approval_plan:proposal,approval_request_id:'approval-fixture',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'Panduan sudah selesai dan diperiksa.\n\n[View work](/tasks/msg_reply)',created_at:now},{id:3,uuid:'agent-followup',is_internal:!!process.env.CUAN_CHECK_PROMPT_CHOICES,prompt_choices:process.env.CUAN_CHECK_PROMPT_CHOICES?{question:'Pilih nama repositori',options:[{id:'guardian',label:'Pakai laravel-traffic-guardian',variant:'primary'},{id:'botbuster',label:'Pakai laravel-botbuster'}],allow_other:true}:undefined,author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'Silakan cek file hasilnya.',created_at:now},{id:4,uuid:'file-only',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'',message:'',attachments:[{id:'fixture-file',name:'panduan.md',type:'text/markdown',size:1024,url:'/api/v1/chat/attachments/private-file'}],created_at:now},{id:5,uuid:'user-later',author_type:'user',sender_type:'visitor',content:'Terima kasih, hasilnya sudah sesuai.',created_at:new Date(Date.parse(now)+360000).toISOString()} ,{id:6,uuid:'draft-reply',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'Kami sudah meninjau laporan Anda.',reply_draft:'Terima kasih sudah menunggu. Kami sedang memeriksa kendalanya.',created_at:now}],requests:[]}
   if(wideSpeech){const speech=data.messages.find(message=>message.uuid==='reply');if(speech)speech.content='Laporan pekerjaan yang telah diperiksa bersama tim. '.repeat(20)+'\n\n[View work](/tasks/msg_reply)'}
   if(standalone)data.messages=data.messages.filter(message=>message.uuid!=='reply')
   return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(data)+'\n\n'}):json(data)
  } return json({})
}
task.status='blocked';task.latest_attempt.status='failed';task.latest_attempt.error_message='Execution timed out after adaptive timeout (9m57.192853126s)';
const writes=[];
async function attentionAPI(route){if(route.request().method()==='POST'&&/\/(messages|run-agent|steer|retry)$/.test(new URL(route.request().url()).pathname))writes.push({url:route.request().url(),body:route.request().postDataJSON()});return routeAPI(route)}
(async()=>{
 fs.mkdirSync(out,{recursive:true});server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript; charset=utf-8':p.endsWith('.css')?'text/css; charset=utf-8':'text/html; charset=utf-8');try{res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}});await new Promise(r=>server.listen(4183,'127.0.0.1',r));
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}});
 for(const width of [390,1440])for(const theme of ['light','dark']){
 const ctx=await browser.newContext({viewport:{width,height:900}}),page=await ctx.newPage();page.setDefaultTimeout(8000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.addInitScript(theme=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-theme',theme)},theme);await page.route('**/api/**',attentionAPI);await page.goto('http://127.0.0.1:4183/inbox/fixture-room');
 if(process.env.CUAN_CHECK_PROMPT_CHOICES){
 writes.length=0;const choices=page.getByTestId('prompt-choice-chips');await choices.waitFor();await choices.scrollIntoViewIfNeeded();const input=page.locator('[data-primary-composer]');await input.fill('My unsent preference.');await choices.getByTestId('prompt-choice-other').click();assert.equal(await input.inputValue(),'My unsent preference.');assert.equal(await input.evaluate(el=>el===document.activeElement),true);assert.equal(await page.locator('#compose-public-tab').getAttribute('aria-selected'),'false');assert.deepEqual(writes,[]);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'horizontal page overflow');await page.screenshot({path:out+'/choices-page-'+theme+'-'+width+'.png'});await choices.screenshot({path:out+'/choices-'+theme+'-'+width+'.png'});await choices.getByTestId('prompt-choice-guardian').click();await page.waitForTimeout(150);assert.equal(writes.length,1);assert.equal(writes[0].body.message,'@cuan Pakai laravel-traffic-guardian');assert(!writes[0].body.approve_message_id);assert.equal(await input.inputValue(),'My unsent preference.');assert.deepEqual(errors,[]);await ctx.close();continue;
 }
 writes.length=0;const card=page.locator('[data-card=task-card]');await card.waitFor();await card.scrollIntoViewIfNeeded();assert.equal(await card.locator('.animate-pulse').count(),0);assert(!await card.innerText().then(s=>s.includes('9m57.')),'technical timeout leaked into card summary');
 await card.getByText('Work stopped before verification finished.',{exact:true}).waitFor();await page.screenshot({path:out+'/attention-'+theme+'-'+width+'.png'});
 const input=page.locator('[data-primary-composer]');await input.fill('My unsent note.');await card.locator('[data-task-other]').click();const draft=await input.inputValue();assert(draft.startsWith('My unsent note.\n\n'),'unsent draft overwritten');assert(draft.includes('Changes to this plan'));assert(draft.includes(task.title));assert.equal(await page.locator('#compose-panel').count(),1);assert.equal(await page.locator('#compose-public-tab').getAttribute('aria-selected'),'false');assert.equal(await page.locator('[role=tab]').filter({hasText:'Whisper'}).getAttribute('aria-selected'),'true');assert.deepEqual(writes,[],'discussion click sent a message or reran task');
 await page.locator('[data-inbox-composer]').screenshot({path:out+'/draft-'+theme+'-'+width+'.png'});
 const detail=await card.locator('[data-task-detail]').boundingBox(),approve=await card.locator('[data-task-approve]').boundingBox();assert(detail.x+detail.width<approve.x,'Detail must stay left of actions');await card.locator('[data-task-approve]').click();await page.waitForTimeout(100);assert.equal(writes.length,1,'approval must send exactly one owner decision');assert.equal(writes[0].body.approve_message_id,'reply');assert.equal(writes[0].body.request_id,'approval-fixture');assert.equal(await input.inputValue(),draft,'approval erased unsent draft');writes.length=0;await input.fill('');
 for(const state of ['queued','running']){task.review_state=state;await page.reload();await card.locator('[data-task-status="'+(state==='queued'?'Waiting for review':'Checking result')+'"]').waitFor();assert.equal(await card.locator('[data-task-approve]').count(),0,'active/pending review asked owner to intervene');assert.equal(await card.locator('.animate-pulse').count()>0,state==='running')}
 task.review_state=undefined;await ctx.close();assert.deepEqual(errors,[]);
 }
 console.log(process.env.CUAN_CHECK_PROMPT_CHOICES?'390/1440 light/dark choices: Others focuses internal composer and preserves draft; named choice sends one chat message without executable approval.':'390/1440 light/dark: exact recommendation, Detail left, Others preserves internal draft without sending, Approve sends one persisted-plan decision; active/queued reviews hide owner intervention.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()});
