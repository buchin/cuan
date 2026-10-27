const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const fs=require('fs'),path=require('path'),assert=require('assert');
const root=process.cwd(), out=root+'/docs/sprints/evidence/S25/mobile';
const mode=process.argv[2]||'after', base='https://cuan.dojo.cc';
const dist=mode==='before'?process.env.CUAN_BEFORE_DIST:root+'/web/dist';
(async()=>{const b=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE,args:['--no-sandbox','--disable-dev-shm-usage']});const results=[];try{
for(const [w,h] of [[390,844]]){
const ctx=await b.newContext({viewport:{width:w,height:h},isMobile:true,hasTouch:true});
await ctx.addInitScript(token=>localStorage.setItem('cuan_token',token),fs.readFileSync(process.env.CUAN_E2E_TOKEN_FILE,'utf8'));
const p=await ctx.newPage();p.setDefaultTimeout(7000);
if(mode!=='production')await p.route(base+'/**',async r=>{const u=new URL(r.request().url());if(u.pathname.startsWith('/api/')||u.pathname.startsWith('/ws'))return r.continue();const f=path.join(dist,u.pathname.startsWith('/assets/')?u.pathname:'index.html');await r.fulfill({path:f})});
const room=JSON.parse(fs.readFileSync(root+'/docs/sprints/evidence/S25/production-smoke.json')).conversation_id;
await p.goto(base+'/inbox/'+room);await p.locator('textarea').waitFor({state:'attached'});
if(!await p.locator('textarea').isVisible())await p.getByRole('button').filter({hasText:'@pin1'}).first().click();
await p.locator('textarea').waitFor();

const checks=[];
await p.locator('textarea').focus();
await p.evaluate(()=>{
  Object.defineProperty(visualViewport,'height',{configurable:true,get:()=>500});
  Object.defineProperty(visualViewport,'offsetTop',{configurable:true,get:()=>20});
  visualViewport.dispatchEvent(new Event('resize'));
});
await p.waitForTimeout(100);
const overlay=await p.locator('[data-inbox-composer]').boundingBox();
assert(Math.abs(overlay.y+overlay.height-520)<2,'visual viewport height+offset');
checks.push({case:'Synthetic visualViewport resize/offset (layout viewport unchanged)',composer:overlay,visibleBottom:520});
await p.evaluate(()=>{delete visualViewport.height;delete visualViewport.offsetTop;visualViewport.dispatchEvent(new Event('resize'))});
await p.locator('textarea').fill(Array(20).fill('Long draft must scroll internally').join('\n'));
const long=await p.locator('textarea').evaluate(t=>({client:t.clientHeight,scroll:t.scrollHeight,max:getComputedStyle(t).maxHeight}));
assert(long.scroll>long.client,'long draft internal scroll');checks.push({case:'long unsent draft',...long});
await p.locator('[data-inbox-composer]').evaluate(c=>c.style.paddingBottom='34px');
const safe=await p.locator('[data-inbox-composer]').boundingBox();const button=await p.locator('textarea + button').boundingBox();
assert(Math.abs(safe.y+safe.height-844)<2);assert(button.y+button.height<=810);
checks.push({case:'Synthetic 34px safe-area padding (not OS env)',composer:safe,button});
await p.screenshot({path:out+'/production-safe-area-long-draft.png'});
await p.locator('button[title="Kembali ke Daftar Kontak"]').click();
await p.locator('textarea').waitFor({state:'hidden'});
checks.push({case:'back to contacts',passed:true});
await p.getByRole('button').filter({hasText:'@pin1'}).first().click();await p.locator('textarea').waitFor();
checks.push({case:'reopen contact',passed:true});
fs.writeFileSync(out+'/production-additional.json',JSON.stringify(checks,null,2));
await ctx.close();}
console.log('additional viewport/safe-area/draft/navigation checks pass');
}finally{await b.close()}})().catch(e=>{console.error(e.message);process.exitCode=1});
