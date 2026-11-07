// Isolated real React components. This is UI evidence, never live task evidence.
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
import {createRequire} from 'node:module'
import {createServer} from 'vite'
const require=createRequire(import.meta.url)
const {chromium}=require(process.env.CUAN_BROWSER_MODULE||path.join(os.homedir(),'.local/lib/node_modules/playwright'))
const output=process.argv[2]
if(!output)throw Error('Supply a new private evidence directory')
await fs.mkdir(output,{mode:0o700})
const finding='Navigasi dari kartu percakapan ke obrolan yang tepat belum terverifikasi pada ponsel dan desktop.'
const now='2026-11-07T02:00:00Z'
const task={id:'review-fixture',project_id:'project',project_name:'Website',title:'Periksa kartu percakapan dan navigasinya',status:'blocked',priority:'normal',review_finding:finding,created_at:now,updated_at:now,latest_attempt:{id:'attempt',task_id:'review-fixture',status:'succeeded',runner_id:'fixture',agent_name:'agent_cuan',created_at:now,finished_at:now}}
const entry=`import React,{useState} from 'react';
import{createRoot}from'react-dom/client';import{BrowserRouter}from'react-router-dom';
import{LanguageProvider}from'/src/context/LanguageContext.tsx';import{AuthProvider}from'/src/context/AuthContext.tsx';
import{AppShell}from'/src/components/layout/AppShell.tsx';
import{InboxTaskAction}from'/src/features/inbox/InboxTaskAction.tsx';
import'/src/index.css';import'/src/features/inbox/chat-calm.css';
function Scene(){const[task,setTask]=useState(${JSON.stringify(task)});window.setFixtureTask=setTask;return React.createElement('main',{className:'inbox-message-list flex-1 overflow-auto p-4 md:p-8'},React.createElement(InboxTaskAction,{conversationId:'fixture',messageId:'message',linkedTask:task}))}
createRoot(document.getElementById('root')).render(React.createElement(BrowserRouter,null,React.createElement(LanguageProvider,null,React.createElement(AuthProvider,null,React.createElement(AppShell,{currentTab:'inbox',onSelectTab:()=>{}},React.createElement(Scene))))));`
const webRoot=path.resolve('web')
process.chdir(webRoot)
const server=await createServer({root:webRoot,envDir:path.resolve(output),cacheDir:path.join(path.resolve(output),'vite-cache'),server:{host:'127.0.0.1',port:0},plugins:[{name:'review-finding-fixture',enforce:'pre',resolveId(id){if(id==='/__finding.jsx')return'\0finding.jsx'},load(id){if(id==='\0finding.jsx')return entry},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url.startsWith('/review-finding')){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml(req.url,'<html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="root"></div><script type="module" src="/__finding.jsx"></script></body></html>'))}else next()})}}]})
await server.listen()
const browser=await chromium.launchPersistentContext(path.join(output,'profile'),{headless:true,executablePath:process.env.CUAN_CHROMIUM_EXECUTABLE,args:process.env.CUAN_BROWSER_NO_SANDBOX==='1'?['--no-sandbox']:[]})
const checks=[]
try{
 for(const width of[390,1440]){
  const context=browser
  await context.addInitScript(()=>localStorage.setItem('cuan-language','id'))
  const page=await context.newPage(),errors=[]
  await page.setViewportSize({width,height:width===390?844:1000})
  page.on('pageerror',error=>errors.push(error.message))
  page.on('console',message=>{if(message.type()==='error')errors.push(message.text())})
  await page.route('**/api/**',route=>new URL(route.request().url()).pathname.startsWith('/api/')?route.fulfill({contentType:'application/json',body:'{}'}):route.continue())
  await page.goto(new URL('/review-finding',server.resolvedUrls.local[0]).href,{waitUntil:'networkidle'})
  const card=page.locator('[data-card="task-card"]')
  try{await card.getByText(finding,{exact:true}).waitFor()}catch(error){await page.screenshot({path:path.join(output,'failed-'+width+'.png')});console.log(JSON.stringify({errors,body:await page.locator('body').innerText(),html:(await page.content()).slice(0,1800)}));throw error}
  assert.equal(await card.getByText('A verified result is still missing.',{exact:true}).count(),0)
  assert.equal(await card.locator('[data-task-approve]').count(),0)
  assert.equal(await card.locator('[data-task-other]').count(),0)
  assert.ok(await card.locator('[data-task-detail]').isVisible())
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
  await page.screenshot({path:path.join(output,`finding-${width}.png`)})
  await page.evaluate(task=>window.setFixtureTask({...task,review_state:'running'}),task)
  await card.getByText('Memeriksa hasil',{exact:true}).waitFor()
  assert.equal(await card.getByText(finding,{exact:true}).count(),0)
  await page.evaluate(task=>window.setFixtureTask({...task,review_finding:undefined}),task)
  await card.getByText('Hasil yang terverifikasi belum tersedia.',{exact:true}).waitFor()
  assert.equal(await card.getByText(finding,{exact:true}).count(),0)
  assert.deepEqual(errors,[])
  checks.push({width,findingVisible:true,activeReviewHidesFinding:true,removedFindingClears:true,consoleErrors:errors})
  await page.close()
 }
 await fs.writeFile(path.join(output,'checks.json'),JSON.stringify(checks,null,2))
}finally{await browser.close();await server.close()}
console.log(JSON.stringify(checks))
