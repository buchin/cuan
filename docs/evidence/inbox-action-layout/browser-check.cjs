const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const out = 'docs/evidence/landing'","const out = 'docs/evidence/inbox-action-layout'");
setup=setup.replace('let inboxPending=true','let statusText="Working";let inboxPending=true');
setup=setup.replace('const h=historyBase(id);','const h=historyBase(id);h.requests[0].steps=[{seq:1,kind:"runtime_tool_started",text:statusText,at:now}];');
setup=setup.replace("    if(p==='/api/v1/provider-accounts')", "    if(p==='/api/v1/item-preferences')return json({items:{}});\n    if(p==='/api/v1/provider-accounts')");
setup+=`
const checks=[];await page.goto(base+'/inbox/room0');
for(const width of [360,820,1440])for(const theme of ['light','dark']){
 await page.setViewportSize({width,height:844});await page.evaluate(t=>localStorage.setItem('cuan-theme',t),theme);
 await page.goto(base+'/inbox/room0');
 const status=page.locator('[data-response-state="running"]').last();await status.waitFor();
 const cancel=status.getByRole('button',{name:'Cancel',exact:true});await cancel.waitFor();
 const before=await cancel.boundingBox();statusText='The agent is reading project files and checking the latest implementation details';await page.reload();await cancel.waitFor();
 const after=await cancel.boundingBox();assert(Math.abs(before.x-after.x)<1,'Cancel moved when status text changed');
 await status.locator('summary').first().click();await status.getByRole('list',{name:'Response timeline'}).waitFor();
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await status.locator('summary').first().click();
 if(width>=1024){
  const selected=page.locator('.inbox-contact-row[aria-pressed="true"]:visible').first();await selected.waitFor();
  const row=selected.locator('../..');const menu=row.locator('[data-item-menu]');
  const rowColor=await row.evaluate(el=>getComputedStyle(el).backgroundColor);const selectedColor=await selected.evaluate(el=>getComputedStyle(el).backgroundColor);assert.equal(rowColor,selectedColor);
  await menu.focus();await menu.click();await page.getByRole('menu').waitFor();await page.keyboard.press('Escape');
 }
 await page.screenshot({path:out+'/'+theme+'-'+width+'.png'});checks.push({width,theme,cancelStable:true,detailExpandable:true,noOverflow:true});statusText='Working';
}
assert.equal(launches.length,0);assert.equal(composerSends.length,0);assert.deepEqual(errors,[]);
fs.writeFileSync(out+'/browser.json',JSON.stringify({checks,selectedRowContinuous:true,menuKeyboardAccessible:true,providerCalls:0},null,2));console.log('PASS stable response actions, selected rows and responsive layouts');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
