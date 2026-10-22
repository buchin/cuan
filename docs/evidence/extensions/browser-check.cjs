const {chromium}=require('/home/dev/.local/lib/node_modules/playwright');
const {spawn}=require('child_process'),fs=require('fs'),assert=require('node:assert/strict'),path=require('path');
const base='http://127.0.0.1:4183';let server,browser;
(async()=>{
 server=spawn(process.execPath,[path.resolve('web/node_modules/vite/bin/vite.js'),'--host','127.0.0.1','--port','4183','--strictPort'],{cwd:path.resolve('web'),stdio:'ignore'});
 for(let i=0;i<60;i++){try{if((await fetch(base+'/tests/extensions/fixture.html')).ok)break}catch{};await new Promise(r=>setTimeout(r,100));if(i===59)throw Error('Fixture Vite startup failed')}
 browser=await chromium.launch({headless:true,executablePath:'/home/dev/.local/playwright-alpine/usr/lib/chromium/chromium',args:['--no-sandbox','--disable-dev-shm-usage'],env:{...process.env,LD_LIBRARY_PATH:['usr/lib','usr/lib/pulseaudio','lib','usr/lib/chromium'].map(p=>'/home/dev/.local/playwright-alpine/'+p).join(':')}});
 const page=await browser.newPage();page.setDefaultTimeout(7000);
 page.on('pageerror',e=>console.error('Fixture page error:',e.message));page.on('console',m=>{if(m.type()==='error')console.error(m.text())});
 const checks=[];
 for(const width of [390,1440]){
  await page.setViewportSize({width,height:844});await page.goto(base+'/tests/extensions/fixture.html');await page.waitForTimeout(300);console.log((await page.locator('body').innerText()).slice(0,1600));
  const markdown=page.getByRole('region',{name:'Markdown fixture'}).or(page.locator('section[aria-label="Markdown fixture"]'));
  await markdown.getByText(':cuan:',{exact:true}).waitFor();assert.equal(await page.getByRole('note').count(),0);
  await page.getByRole('button',{name:'Enable example',exact:true}).click();await page.getByRole('note').waitFor();
  await markdown.getByText('Cuan',{exact:true}).waitFor();assert.equal(await page.getByRole('note').textContent(),'Hello <b>reader</b>');
  assert(await markdown.innerText().then(t=>t.includes('Unsupported format remains readable')));
  assert.equal(await page.evaluate(()=>window.fixtureInjected),undefined);
  const project=page.locator('section[aria-label="Project fixture"]');
  await project.getByRole('button',{name:'Kanban',exact:true}).click();
  for(const [label,status] of [['Ready','backlog'],['In progress','doing'],['Result ready','review'],['Needs attention','blocked'],['Completed','done']]){
   const column=project.locator('section').filter({has:page.getByRole('heading',{name:label,exact:true})});
   assert.equal(await column.getByRole('button',{name:status+' task '+status,exact:true}).count(),1)
  }
  await page.getByRole('button',{name:'Toggle loading',exact:true}).click();await project.getByRole('status').waitFor();
  assert.equal(await project.getByRole('button',{name:/blocked task/}).count(),0);
  await page.getByRole('button',{name:'Toggle loading',exact:true}).click();
  await page.getByRole('button',{name:'Break renderer',exact:true}).click();
  await markdown.locator('pre').filter({hasText:'Renderer original content'}).waitFor();
  await page.getByRole('button',{name:'Break parser',exact:true}).click();
  await markdown.getByText(':cuan:',{exact:false}).waitFor();
  assert((await markdown.innerText()).includes('Renderer original content'));
  await page.getByRole('button',{name:'Disable parser',exact:true}).click();await page.getByRole('note').waitFor();
  await page.getByRole('button',{name:'Disable example',exact:true}).click();await markdown.getByText(':cuan:',{exact:true}).waitFor();assert.equal(await page.getByRole('note').count(),0);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  checks.push({width,disabledDefault:true,parserAndRenderer:true,safeRawText:true,unknownFallback:true,rendererFailureFallback:true,parserFailureFallback:true,correctKanbanStatuses:true,loadingState:true,disableRestoresCore:true,noOverflow:true});
 }
 fs.writeFileSync('docs/evidence/extensions/browser.json',JSON.stringify({checks,providerCalls:0},null,2));console.log('Extension parser/renderer/view browser checks passed.');
})().catch(e=>{console.error(e.message);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server){server.kill('SIGTERM');await new Promise(r=>server.once('exit',r))}});
