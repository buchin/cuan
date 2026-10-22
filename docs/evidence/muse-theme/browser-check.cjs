// Isolated Vite/Chromium integration fixture: real InboxView, mocked history;
// no provider, production writes, or persisted product test route.
const {chromium}=require('/home/dev/.local/lib/node_modules/playwright')
const fs=require('fs'),path=require('path'),assert=require('node:assert/strict'),{spawnSync}=require('child_process'),http=require('http')
const initialCode = JSON.stringify('```js\nconst partial = 1'), finalCode = JSON.stringify('```js\nconst partial = 1\n```\n\n**Finished**')
const root=process.cwd(),temp=path.join(root,'web/.markdown-check'),out=path.join(root,'docs/evidence/muse-theme')
const message=['## Sprint recommendation','','A **useful** response with *emphasis*, ~~old scope~~, and `inline code`.','','1. Inspect the repository','   - Check **nested** evidence','2. Propose the sprint','','> Based on current evidence.','','| Feature | Outcome | Reference |','| --- | --- | --- |','| Very long feature column | Readable on mobile | '+ 'reference-'.repeat(30)+' |','','```go','package main','// '+ 'long-line-'.repeat(50),'func main() {}','```','','[Safe link](https://example.com/path) [Bad](javascript:alert%281%29) [Local file](file:///etc/passwd) [Data](data:text/html,bad) [Protocol relative](//tracker.invalid/path)','','<script>window.markdownXss = true</script>','<img src=x onerror="window.markdownXss=true">','','![Remote image](https://tracker.invalid/pixel)',''].join('\n')
let server,browser
;(async()=>{
fs.mkdirSync(temp,{recursive:true});fs.mkdirSync(out,{recursive:true})
fs.writeFileSync(path.join(temp,'index.html'),'<html><head></head><body><div id="root"></div><script type="module" src="./check.tsx"></script></body></html>')
fs.writeFileSync(path.join(temp,'check.tsx'),`import React from 'react';import {createRoot} from 'react-dom/client';import {MemoryRouter} from 'react-router-dom';import {InboxView} from '../src/features/inbox/InboxView';import {ChatMarkdown} from '../src/features/inbox/ChatMarkdown';import '../src/index.css';const contact={id:'fixture',type:'agent',name:'@fixture',status:'online',conversation_id:'fixture'};function App(){const [text,setText]=React.useState(${initialCode});return <MemoryRouter><main style={{height:'100vh',minWidth:0}}><InboxView contacts={[contact]} activeContact={contact}/></main><section id="partial"><ChatMarkdown content={text}/><button onClick={()=>setText(${finalCode})}>Finish snapshot</button></section></MemoryRouter>}createRoot(document.getElementById('root')!).render(<App/>);`)
const build=spawnSync(process.execPath,['--input-type=module','-e',`import {build} from 'vite';await build({configFile:false,root:${JSON.stringify(temp)},build:{outDir:'dist',emptyOutDir:true},logLevel:'error'})`],{cwd:path.join(root,'web'),encoding:'utf8',timeout:30000});if(build.status!==0)throw Error('Fixture build failed: '+build.stderr.slice(-2000))
server=http.createServer((req,res)=>{const file=path.join(temp,'dist',req.url==='/'?'index.html':req.url.split('?')[0]);try{const data=fs.readFileSync(file);res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':file.endsWith('.woff2')?'font/woff2':'text/html');res.end(data)}catch{res.writeHead(404);res.end()}});await new Promise(r=>server.listen(4179,'127.0.0.1',r))
browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}})
const context=await browser.newContext({viewport:{width:390,height:844},permissions:['clipboard-read','clipboard-write']});const page=await context.newPage();page.setDefaultTimeout(8000)
const source=JSON.parse(fs.readFileSync('docs/evidence/muse-theme/source.json'));const themeChecks=[];const errors=[],external=[];page.on('console',m=>{if(m.type()==='error')console.error('CONSOLE',m.text())});page.on('requestfailed',r=>console.error('REQUEST',r.url(),r.failure()));page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(!r.url().startsWith('http://127.0.0.1:4179')&&!r.url().startsWith('ws:'))external.push(r.url())})
await page.route('**/api/**',route=>{
const url=route.request().url();if(!new URL(url).pathname.startsWith('/api/'))return route.continue();if(url.endsWith('/events'))return route.fulfill({status:200,contentType:'text/event-stream',body:': fixture\n\n'})
return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({messages:[{id:1,uuid:'agent-msg',author_type:'agent',author_name:'@fixture',content:message,created_at:new Date().toISOString()},{id:2,uuid:'user-msg',author_type:'user',content:'**User input stays literal**',created_at:new Date().toISOString()}],requests:[],tasks:[]})})})
await page.goto('http://127.0.0.1:4179/');const body=page.locator('.inbox-message-list .chat-markdown');await body.locator('h2').waitFor()
assert.equal(await body.locator('h2').innerText(),'Sprint recommendation');assert.equal(await body.locator('strong').first().innerText(),'useful');assert(await body.locator('ol > li ul > li strong').count());assert.equal(await body.locator('table tbody tr').count(),1)
assert.equal(await body.locator('script,img,iframe').count(),0);assert.equal(await page.evaluate(()=>window.markdownXss),undefined)
assert.equal(await body.locator('a').count(),1);assert.equal(await body.locator('a').getAttribute('rel'),'noopener noreferrer');assert.equal(await body.locator('a').getAttribute('target'),'_blank')
assert.equal(await page.getByText('**User input stays literal**',{exact:true}).count(),1)
await body.getByRole('button',{name:'Copy code'}).click();assert((await page.evaluate(()=>navigator.clipboard.readText())).startsWith('package main\n'));await body.getByText('Copied',{exact:true}).waitFor()
// Exercise fallback failure explicitly; never falsely report success.
await page.evaluate(()=>{Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async()=>{throw Error('denied')}}});document.execCommand=()=>false})
await body.getByRole('button',{name:'Copy code'}).click();await body.getByRole('status').getByText('Could not copy. Select the code and copy manually.').waitFor()
const partial=page.locator('#partial');assert.equal(await partial.locator('pre code').innerText(),'const partial = 1');await partial.getByRole('button',{name:'Finish snapshot'}).click();await partial.locator('strong').getByText('Finished',{exact:true}).waitFor()
// Verify every official color token, including alpha utility support and fonts.
for(const mode of ['light','dark']){
 await page.evaluate(mode=>document.documentElement.classList.toggle('dark',mode==='dark'),mode);
 const actual=await page.evaluate(()=>{const css=getComputedStyle(document.documentElement);return Object.fromEntries([...css].filter(x=>x.startsWith('--')).map(k=>[k,css.getPropertyValue(k).trim()]))});
 for(const [key,value] of Object.entries(source.cssVars[mode])){
  if(!value.startsWith('oklch('))continue;
  const token=key==='sidebar'?'sidebar-background':key;
  const values=x=>x.split(/\s+/).map(Number);
  assert.deepEqual(values(actual['--'+token]),values(value.slice(6,-1)),mode+' '+token);
 }
 assert.equal(actual['--radius'],'1.25rem');themeChecks.push({mode,allOfficialColorTokensMatch:true,radius:'1.25rem'});
}
await page.evaluate(()=>document.documentElement.classList.remove('dark'));
await page.evaluate(()=>document.fonts.ready);const fonts=await page.evaluate(()=>({body:getComputedStyle(document.body).fontFamily,loaded:[...document.fonts].filter(f=>f.status==='loaded').map(f=>f.family)}));assert(fonts.body.includes('Geist Variable'));assert(fonts.loaded.some(x=>x.includes('Geist Variable')));assert(fonts.loaded.some(x=>x.includes('Geist Mono Variable')));
const alpha=await page.evaluate(()=>{const x=document.createElement('div');x.className='bg-primary/10';document.body.append(x);const color=getComputedStyle(x).backgroundColor;x.remove();return color});assert(alpha.includes('0.1'),alpha);
const widths=[];for(const width of [360,390,1280]){await page.setViewportSize({width,height:844});await page.waitForTimeout(100);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'page overflow '+width);const overflow=await body.locator('.chat-table').evaluate(x=>({scroll:x.scrollWidth,client:x.clientWidth}));assert(overflow.scroll>overflow.client,'wide table should scroll');widths.push({width,noPageOverflow:true,tableScrolls:true});await page.screenshot({path:path.join(out,'fixture-'+width+'.png')})}
await page.evaluate(()=>document.documentElement.classList.add('dark'));await page.waitForTimeout(300);await page.screenshot({path:path.join(out,'fixture-dark.png')})
assert.equal(errors.length,0);assert.equal(external.length,0)
fs.writeFileSync(path.join(out,'browser.json'),JSON.stringify({fixture:true,mockedHistory:true,providerInvoked:false,checks:['real InboxView integration','nested Markdown/GFM','user input remains literal','unsafe links and HTML blocked','remote images not loaded','exact clipboard content','clipboard denial shows error','incomplete fence and snapshot update','light/dark'],themeName:source.name,themeChecks,fonts,alphaColor:alpha,widths,javascriptErrors:errors,externalRequests:external},null,2)+'\n')
console.log('PASS: exact Muse light/dark tokens, radius, local fonts, alpha utilities and Markdown/mobile regression')
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r));fs.rmSync(temp,{recursive:true,force:true})})
