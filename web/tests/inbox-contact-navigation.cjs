const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert=require('node:assert/strict');
(async()=>{

 const now=new Date().toISOString();const room='11111111-1111-4111-8111-111111111111',id='22222222-2222-4222-8222-222222222222';
 const crm={id,name:'Suprlearningcenter',email:'customer@example.invalid',product:'Dojo',conversation_id:room,tags:[],updated_at:now};
 const contact={...crm,id:'cust_'+room,contact_id:id,type:'customer',avatar:'👤',status:'offline',updated_at:now,unread_count:0,has_messages:true,last_message:'Halo om',last_message_at:now};
 for(const width of [390,1440]){
 const browser=await chromium.launch({headless:true,...(process.env.CHROMIUM_EXECUTABLE ? {executablePath:process.env.CHROMIUM_EXECUTABLE} : {})});
  const page=await browser.newPage({viewport:{width,height:900}});const errors=[];page.on('pageerror',e=>errors.push(e.stack));
  await page.addInitScript(()=>{localStorage.setItem('cuan_token','local-visual-fixture');localStorage.setItem('theme','dark')});
  await page.route('**/api/**',async route=>{
   const p=new URL(route.request().url()).pathname;if(!p.startsWith('/api/'))return route.continue();
   if(p.endsWith('/events'))return route.fulfill({contentType:'text/event-stream',body:': local fixture\n\n'});
   let data={};
   if(p==='/api/v1/auth/status')data={setup_required:false};
   else if(p==='/api/v1/auth/me')data={user:{id:'owner',email:'owner@example.invalid',name:'Owner',role:'owner'}};
   else if(p==='/api/v1/item-preferences')data={items:{}};
   else if(p==='/api/v1/chat/contacts')data={contacts:[contact]};
   else if(p.includes('/chat/conversations/')&&p.endsWith('/messages'))data={conversation_id:room,contact,messages:[{id:'message-one',author_type:'user',author_name:crm.name,content:'Halo om',created_at:now,is_internal:false}],tasks:[],requests:[]};
   else if(p==='/api/v1/contacts')data={contacts:[crm]};
   else if(p.includes('/memory'))data={items:[],memories:[]};
   else if(p==='/api/v1/notifications')data={items:[]};
   else if(p.includes('/projects'))data={projects:[]};
   else if(p.includes('/agents'))data={agents:[]};
   else if(p.includes('/tasks'))data={tasks:[]};
   else if(p.includes('/archive'))data={items:[]};
   else if(p.includes('read'))data={read_state:{},states:{},last_read:{}};
   await route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  });
  await page.goto((process.env.CUAN_VISUAL_URL || 'http://127.0.0.1:5182')+'/inbox/'+room);
  
  const link=page.getByRole('link',{name:'View contact details'});await link.waitFor({timeout:7000});assert.equal(await link.getAttribute('href'),'/contacts?contact='+id);
  await page.screenshot({path:`/tmp/cuan-contact-header-${width}.png`});
  await link.click();await page.waitForURL('**/contacts?contact='+id);await page.getByText('Notes & about',{exact:true}).waitFor();await page.getByRole('heading',{name:crm.name,exact:true}).waitFor();
  assert.equal(await page.getByText('Contact not found',{exact:true}).count(),0);
  await page.getByText(crm.email,{exact:true}).last().waitFor();
  await page.screenshot({path:`/tmp/cuan-contact-open-${width}.png`});
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  assert.deepEqual(errors,[]);await browser.close();console.log(width+': canonical contact navigation passed');
 }
})().catch(e=>{console.error(e);process.exit(1)});
