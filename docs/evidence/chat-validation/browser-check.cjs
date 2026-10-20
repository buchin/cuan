const fs=require('fs'),path=require('path'),source=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8')
let setup=source.slice(0,source.indexOf(' const checks=[];'))
setup=setup.replace('const data = history(chat[1])',`const data = history(chat[1]);data.requests[0].steps=[...(data.requests[0].steps||[]),{kind:'runtime_tool_started',text:'Tool #2 · Read file started.',at:now},{kind:'runtime_session',text:'Runtime session ID: private-session',at:now}];`)
setup+=`
const checks=[];
for(const width of [390,1440]){
 await page.setViewportSize({width,height:844});await page.goto(base+'/inbox/room0');const activity=page.locator('[data-response-state="running"]');await activity.getByRole('status').filter({hasText:'Tool #2 · Read file started.'}).waitFor();assert(!(await activity.innerText()).includes('private-session'));assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.goto(base+'/studio');await page.locator('#studio-goal').fill('Keep this draft');await page.locator('a[aria-label="Go to Studio"]:visible').click();assert.equal(await page.locator('#studio-goal').inputValue(),'Keep this draft');await page.goto(base+'/contacts');await page.locator('a[aria-label="Go to Studio"]:visible').click();await page.waitForURL(u=>u.pathname==='/studio');assert.equal(await page.locator('#studio-goal').inputValue(),'Keep this draft');checks.push({width,latestSafeActivity:true,sessionIdExcluded:true,logoToStudio:true,draftPreserved:true});
}
fs.writeFileSync('docs/evidence/chat-validation/browser.json',JSON.stringify({checks,providerCalls:0},null,2));console.log('Latest activity and logo/draft checks passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'))
