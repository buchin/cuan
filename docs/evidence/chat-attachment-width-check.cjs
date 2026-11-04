// Render the actual app using the shared customer fixture. No production writes.
const {routeAPI}=require('./customer-task-card-check.cjs')
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),http=require('http'),path=require('path'),assert=require('node:assert/strict')
const out=process.env.CUAN_VISUAL_OUTPUT||'/home/dev/reports/cuan-pm-recovery/chat-attachment-width'
let server,browser,page
;(async()=>{
 fs.mkdirSync(out,{recursive:true});server=http.createServer((req,res)=>{let p=path.join(process.cwd(),'web/dist',req.url.split('?')[0]);if(!path.extname(p))p=path.join(process.cwd(),'web/dist/index.html');try{res.setHeader('Content-Type',p.endsWith('.js')?'text/javascript':p.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(p))}catch{res.writeHead(404);res.end()}});await new Promise(r=>server.listen(4186,'127.0.0.1',r))
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
 for(const width of [360,390,1440])for(const theme of ['light','dark'])for(const standalone of [false,true]){
 const context=await browser.newContext({viewport:{width,height:900},isMobile:width<500,hasTouch:width<500});page=await context.newPage();page.setDefaultTimeout(15000)
 await page.addInitScript(theme=>{localStorage.setItem('cuan_token','fixture');localStorage.setItem('cuan-language','en');localStorage.setItem('cuan-theme',theme)},theme)
 await page.route('**/api/**',route=>routeAPI(route,{standalone,wideSpeech:true}))
 await page.goto('http://127.0.0.1:4186/inbox/fixture-room')
 const card=page.locator(standalone?'[data-testid=task-completion-card], [data-card=task-card]':'[data-message-id=reply] [data-card=task-card]').first();await card.waitFor();await card.scrollIntoViewIfNeeded()
 const geometry=await card.evaluate(el=>{const frame=el.closest('.inbox-attachment');if(!frame)throw Error('attachment has no shared width frame');const box=el.getBoundingClientRect(),parent=frame.parentElement.getBoundingClientRect();return {width:box.width,x:box.x,parentWidth:parent.width,parentX:parent.x}})
 assert(geometry.width<=geometry.parentWidth*.92+1,'task preview fills the conversation width');assert(Math.abs(geometry.x-geometry.parentX)<=1,'task preview lost incoming alignment');assert(geometry.width>=200,'task preview became unusably narrow')
 if(!standalone){const bubble=page.locator('[data-message-id=reply] .wa-message-bubble');const speech=await bubble.boundingBox();const box=await card.boundingBox();assert(Math.abs(speech.x-box.x)<=1);assert(Math.abs(speech.width-box.width)<=2,'task and long incoming speech use different width limits')}
 assert.equal(await card.locator('xpath=ancestor::*[contains(concat(" ",normalize-space(@class)," ")," wa-message-bubble ")]').count(),0,'preview nested inside a fake speech bubble')
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
 const detail=card.getByRole('button',{name:'Detail',exact:true});if(await detail.count()){await detail.scrollIntoViewIfNeeded();assert((await detail.boundingBox()).height>=44)}
 await page.screenshot({path:`${out}/${standalone?'standalone':'attached'}-${theme}-${width}.png`})
 await context.close()
 }
 console.log('Attached/standalone task previews360/390/1440 light/dark:92% incoming width boundary, left alignment, no nested speech surface,44px Detail and no overflow.')
})().catch(async e=>{console.error(e);if(page)await page.screenshot({path:out+'/error.png'});process.exitCode=1}).finally(async()=>{await browser?.close();server?.close()})
