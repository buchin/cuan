// Actual build with isolated fixtures: no real invitations are created.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const {routeAPI}=require('./customer-task-card-check.cjs')
const fs=require('fs'),path=require('path'),http=require('http'),assert=require('node:assert/strict')
const out='/home/dev/reports/cuan-pm-recovery/team-invite';let browser,server
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}})
 await new Promise(r=>server.listen(4186,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [390,1440])for(const role of ['owner','member']){
  const context=await browser.newContext({viewport:{width,height:900}}),page=await context.newPage();let posts=0
  await page.addInitScript(()=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-language','id');localStorage.setItem('cuan-theme','light')})
  await page.route('**/api/**',async route=>{
   const p=new URL(route.request().url()).pathname,json=x=>route.fulfill({contentType:'application/json',body:JSON.stringify(x)})
   if(p.endsWith('/auth/me'))return json({owner:{id:'fixture',username:'Owner',email:'owner@example.com',role}})
   if(p==='/api/v1/team/members')return json({members:[{id:'fixture',name:'Buchin',email:'owner@example.com',role:'owner',is_active:true,created_at:new Date().toISOString()}],invitations:[]})
   if(p==='/api/v1/team/invites'){assert.equal(role,'owner');assert.deepEqual(route.request().postDataJSON(),{email:'new@example.com',role:'member'});posts++;return json({invitation:{id:'invite',email:'new@example.com',role:'member'},invite_url:'https://example.test/invite?token=fixture',token:'fixture'})}
   return routeAPI(route)
  })
  await page.goto('http://127.0.0.1:4186/settings/team');await page.getByRole('list',{name:'Daftar Anggota Tim',exact:true}).waitFor()
  const invite=page.getByRole('button',{name:'Undang Anggota',exact:true})
  assert.equal(await invite.count(),role==='owner'?1:0)
  if(role==='owner'){
   assert((await invite.boundingBox()).height>=44)
   await page.screenshot({path:`${out}/team-${width}.png`})
   await invite.click();await page.getByRole('dialog').waitFor();await page.locator('#invite-email').fill('new@example.com')
   await page.getByRole('button',{name:'Buat Tautan Undangan',exact:true}).click();await page.locator('#generated-invite-url').waitFor();assert.equal(posts,1)
   assert.equal(await page.locator('#generated-invite-url').inputValue(),'https://example.test/invite?token=fixture')
   await page.screenshot({path:`${out}/invite-${width}.png`})
  }
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await context.close()
 }
 console.log('390/1440: Settings Team retains Owner invite action; member cannot invite; default Member invitation returns shareable link; no overflow.')
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
