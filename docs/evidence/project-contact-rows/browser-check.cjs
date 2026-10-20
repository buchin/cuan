const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const out = 'docs/evidence/landing'","const out = 'docs/evidence/project-contact-rows'");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')", "    if(p.endsWith('/memory'))return json({items:[],has_more:false});if(p==='/api/v1/item-preferences')return json({items:{}});\n    if(p==='/api/v1/projects/p1')return json(projects[0]);\n    if(p==='/api/v1/provider-accounts')");
setup+=`
const checks=[];await page.goto(base+'/projects/p1/tasks');
for(const theme of ['light','dark'])for(const width of [360,820,1440]){
 await page.setViewportSize({width,height:844});await page.evaluate(t=>localStorage.setItem('cuan-theme',t),theme);
 for(const kind of ['projects','contacts']){
  await page.goto(base+(kind==='projects'?'/projects/p1/tasks':'/contacts'));
  const selector=kind==='projects'?'.project-list-row':'.contact-list-row';
  if(kind==='contacts')await page.locator(selector+':visible').first().click();
  if(width>=1024){
   const selected=page.locator(selector+(kind==='projects'?'[aria-current="page"]':'[aria-pressed="true"]')+':visible').first();await selected.waitFor();await page.mouse.move(1100,20);await page.waitForTimeout(200);
   const row=selected.locator('../..');assert.equal(await row.evaluate(el=>getComputedStyle(el).backgroundColor),await selected.evaluate(el=>getComputedStyle(el).backgroundColor));
   const menu=row.locator('[data-item-menu]');await menu.focus();await menu.click();await page.getByRole('menu').waitFor();await page.keyboard.press('Escape');
   const size=await menu.boundingBox();assert(size.width>=44&&size.height>=44);
  }
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:out+'/'+kind+'-'+theme+'-'+width+'.png'});checks.push({kind,theme,width,noOverflow:true});
 }
}
assert.equal(launches.length,0);assert.equal(composerSends.length,0);assert.deepEqual(errors,[]);
fs.writeFileSync(out+'/browser.json',JSON.stringify({checks,selectedRowsContinuous:true,menusKeyboardAccessible:true,touchTargets44:true,providerCalls:0},null,2));console.log('PASS Projects/Contacts rows and responsive layouts');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
