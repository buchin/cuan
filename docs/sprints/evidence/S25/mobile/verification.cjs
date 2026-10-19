const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const fs=require('fs'),path=require('path'),assert=require('assert');
const root=process.cwd(), out=root+'/docs/sprints/evidence/S25/mobile';
const mode=process.argv[2]||'after', base=mode==='staging'?'https://cuan-s25-dojo.fural.space':'https://cuan.dojo.cc';
const dist=mode==='before'?process.env.CUAN_BEFORE_DIST:root+'/web/dist';
(async()=>{const b=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE,args:['--no-sandbox','--disable-dev-shm-usage']});const results=[];try{
for(const [w,h] of [[360,800],[390,844],[412,915],[800,360],[844,390],[915,412]]){
const ctx=await b.newContext({viewport:{width:w,height:h},isMobile:true,hasTouch:true});
await ctx.addInitScript(token=>localStorage.setItem('cuan_token',token),fs.readFileSync(process.env.CUAN_E2E_TOKEN_FILE,'utf8'));
const p=await ctx.newPage();p.setDefaultTimeout(7000);
if(!['production','staging'].includes(mode))await p.route(base+'/**',async r=>{const u=new URL(r.request().url());if(u.pathname.startsWith('/api/')||u.pathname.startsWith('/ws'))return r.continue();const f=path.join(dist,u.pathname.startsWith('/assets/')?u.pathname:'index.html');await r.fulfill({path:f})});
let room=JSON.parse(fs.readFileSync(root+'/docs/sprints/evidence/S25/production-smoke.json')).conversation_id;
if(mode==='staging'){const resp=await p.request.get(base+'/api/v1/chat/contacts',{headers:{Authorization:'Bearer '+fs.readFileSync(process.env.CUAN_E2E_TOKEN_FILE,'utf8')}});room=(await resp.json()).contacts.find(c=>c.name.includes('pin1')).conversation_id;}await p.goto(base+'/inbox/'+room);await p.locator('textarea').waitFor({state:'attached'});
if(!await p.locator('textarea').isVisible())await p.getByRole('button').filter({hasText:'@pin1'}).first().click();
await p.locator('textarea').waitFor();
for(const keyboard of [false,true]){
// Headless has no OS keyboard: resize the visible viewport with a focused textarea.
await p.setViewportSize({width:w,height:keyboard?Math.max(210,h-300):h});
if(keyboard)await p.locator('textarea').focus();else await p.locator('textarea').blur();
await p.locator('textarea').fill('Reachability check — unsent');await p.waitForTimeout(120);
const metrics=await p.evaluate(()=>{const t=document.querySelector('textarea'),c=t.parentElement.parentElement,btn=t.nextElementSibling,s=c.previousElementSibling;const rect=x=>{const r=x.getBoundingClientRect();return {top:r.top,bottom:r.bottom,left:r.left,right:r.right,height:r.height}};s.scrollTop=0;return {viewport:{width:innerWidth,height:innerHeight,visualHeight:visualViewport.height},documentWidth:document.documentElement.scrollWidth,composer:rect(c),input:rect(t),send:rect(btn),scroll:{client:s.clientHeight,total:s.scrollHeight,top:s.scrollTop},hit:document.elementFromPoint(btn.getBoundingClientRect().x+20,btn.getBoundingClientRect().y+20)===btn||btn.contains(document.elementFromPoint(btn.getBoundingClientRect().x+20,btn.getBoundingClientRect().y+20))}});
await p.waitForTimeout(3700);
metrics.scrollAfterPoll=await p.evaluate(()=>document.querySelector('textarea').parentElement.parentElement.previousElementSibling.scrollTop);
await p.evaluate(()=>{const s=document.querySelector('textarea').parentElement.parentElement.previousElementSibling;s.scrollTop=s.scrollHeight});
await p.screenshot({path:`${out}/${mode}-${w}x${h}-${keyboard?'keyboard-emulated':'closed'}.png`});
results.push({w,h,keyboardEmulated:keyboard,...metrics});fs.writeFileSync(`${out}/${mode}-checkpoint.json`,JSON.stringify(results,null,2));
if(mode!=='before'){assert(metrics.documentWidth<=w,'document width');assert(Math.abs(metrics.composer.bottom-metrics.viewport.visualHeight)<2,'composer anchored');assert(metrics.hit,'send reachable');assert(metrics.scroll.client>20,'message viewport');assert(metrics.scrollAfterPoll===0,'poll preserves scroll')}
}
await ctx.close();}
fs.writeFileSync(`${out}/${mode}.json`,JSON.stringify({mode,limits:'Chromium mobile CSS viewport + focused viewport resize; no physical Android/iOS keyboard/browser chrome or OS safe-area rendering. Before uses retained build with real production API; after uses candidate build with same API; production uses served assets. Draft never submitted.',results},null,2));console.log(mode+' '+results.length+' measured states');
}finally{await b.close()}})().catch(e=>{console.error(e);process.exitCode=1});
