const taskPageFixture=require('/home/dev/repos/cuan/docs/evidence/task-page-fixture.cjs')
// Real frontend build, isolated fixture APIs. No production messages/tasks are created.
const { chromium } = require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-pm-recovery/read-receipts',now=new Date().toISOString()
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
   const data={conversation_id:'fixture-room',room_id:'fixture-room',contact:contacts[0],tasks:[task],messages:[{id:1,uuid:'message',author_type:'user',sender_type:'visitor',content:'Buatkan panduan untuk pelanggan.',created_at:now},{id:11,uuid:'user-followup',author_type:'user',sender_type:'visitor',content:'Gunakan bahasa yang sederhana.',created_at:now},{id:2,uuid:'reply',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'Panduan sudah selesai dan diperiksa.\n\n[View work](/tasks/msg_reply)',created_at:now},{id:3,uuid:'agent-followup',author_type:'agent',sender_type:'operator',author_name:'@cuan',content:'Silakan cek file hasilnya.',created_at:now},{id:4,uuid:'file-only',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'',message:'',attachments:[{id:'fixture-file',name:'panduan.md',type:'text/markdown',size:1024,url:'/api/v1/chat/attachments/private-file'}],created_at:now},{id:5,uuid:'user-later',author_type:'user',sender_type:'visitor',content:'Terima kasih, hasilnya sudah sesuai.',created_at:new Date(Date.parse(now)+360000).toISOString()} ,{id:6,uuid:'draft-reply',author_type:'agent',is_internal:true,sender_type:'operator',author_name:'@cuan',content:'Kami sudah meninjau laporan Anda.',reply_draft:'Terima kasih sudah menunggu. Kami sedang memeriksa kendalanya.',created_at:now}],requests:[]}
   if(wideSpeech){const speech=data.messages.find(message=>message.uuid==='reply');if(speech)speech.content='Laporan pekerjaan yang telah diperiksa bersama tim. '.repeat(20)+'\n\n[View work](/tasks/msg_reply)'}
   if(standalone)data.messages=data.messages.filter(message=>message.uuid!=='reply')
   return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(data)+'\n\n'}):json(data)
  } return json({})
}
const receiptMessages=[
 {id:1,uuid:'visitor',sender_type:'visitor',author_type:'user',author_id:'visitor',author_name:'Customer',content:'Terima kasih, apakah konfigurasi sudah selesai?',created_at:now,email_delivery:{status:'delivered',recipient_type:'operator'}},
 ...['saved','delivered','read'].map((state,i)=>({id:i+2,uuid:state,sender_type:'operator',author_type:'user',author_id:'operator_user',author_name:'You',content:['Konfigurasi sudah selesai.','Sertifikat HTTPS sudah aktif.','Silakan coba kembali, lalu kabari kami jika masih ada kendala.'][i],created_at:now,...(state!=='saved'?{delivered_at:now}:{}),...(state==='read'?{read_confirmed_at:now}:{})})),
 {id:6,uuid:'whisper',sender_type:'operator',author_type:'user',author_id:'operator_user',author_name:'You',is_internal:true,content:'@cuan pantau hasilnya',created_at:now,delivered_at:now,read_confirmed_at:now}
];
const acknowledgements=[];
async function receiptAPI(route){
 const p=new URL(route.request().url()).pathname;
 if(p.endsWith('/receipts')){acknowledgements.push(route.request().postDataJSON());return route.fulfill({json:{ok:true}})}
 if(/chat\/conversations\/fixture-room\/(messages|events)$/.test(p)){
 const data={conversation_id:'fixture-room',room_id:'fixture-room',contact:contacts[0],tasks:[],messages:receiptMessages,requests:[]};
 return p.endsWith('/events')?route.fulfill({contentType:'text/event-stream',body:'event: snapshot\ndata: '+JSON.stringify(data)+'\n\n'}):route.fulfill({json:data});
 }
 if(p.endsWith('/settings/resend'))return route.fulfill({json:{settings:{has_api_key:true,has_webhook_secret:true,from_name:'Support',from_email:'support@example.invalid',notification_email:'team@example.invalid',enabled:true,idle_threshold_minutes:5,validation_status:'valid'}}});
 return routeAPI(route);
}
(async()=>{
 fs.mkdirSync(out,{recursive:true});server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':'text/html');try{res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}});await new Promise(r=>server.listen(4183,'127.0.0.1',r));
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}});
 for(const width of [390,1440])for(const theme of ['light','dark']){
 const ctx=await browser.newContext({viewport:{width,height:900}}),page=await ctx.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.addInitScript(theme=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-theme',theme)},theme);await page.route('**/api/**',receiptAPI);await page.goto('http://127.0.0.1:4183/inbox/fixture-room');
 await page.locator('[data-message-id=read]').getByRole('img',{name:'Read in chat',exact:true}).waitFor();
 await page.locator('[data-message-id=delivered]').getByRole('img',{name:'Delivered to chat',exact:true}).waitFor();
 await page.locator('[data-message-id=saved]').getByRole('img',{name:'Message saved',exact:true}).waitFor();
 assert.equal(await page.locator('[data-message-id=whisper]').getByRole('img',{name:'Read in chat'}).count(),0);
 await page.locator('[data-message-id=visitor]').scrollIntoViewIfNeeded();await page.waitForTimeout(250);assert(acknowledgements.some(a=>a.read_ids.includes('visitor')),'visible customer message acknowledged');
 await page.screenshot({path:out+'/inbox-'+theme+'-'+width+'.png'});assert.deepEqual(errors,[]);
 await page.goto('http://127.0.0.1:4183/settings/integrations/resend');await page.getByText('Email delivery tracking',{exact:true}).click();await page.locator('#resend-webhook-secret').waitFor();await page.locator('#resend-webhook-secret').scrollIntoViewIfNeeded();await page.screenshot({path:out+'/settings-'+theme+'-'+width+'.png'});
 await ctx.close();
 }
 console.log('Inbox saved/delivered/read and internal-message isolation; mobile/desktop light/dark and integration settings rendered.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()});
