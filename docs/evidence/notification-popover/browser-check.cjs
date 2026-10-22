const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const now = new Date().toISOString()", "let notificationReadFails=false; const now = new Date().toISOString()");
setup=setup.replace("if(p==='/api/v1/notifications'){if(method==='PATCH'){", "if(p==='/api/v1/notifications'){if(method==='PATCH'){if(notificationReadFails)return json({error:'Fixture write failure'},503);");
setup+=`
notifications.splice(0,notifications.length,
 {id:'attention',kind:'blocker',title:'Check the interrupted reply',context:'Website project',summary:'Your message is saved.',status:'Needs attention',href:'/inbox/room0',created_at:now,read:false},
 {id:'done',kind:'completed',title:'Improve mobile navigation',context:'Website project',status:'Completed',href:'/tasks/task_one',created_at:now,read:false},
 {id:'handled',kind:'approval',title:'Approved scope',context:'Website project',resolved:true,status:'Approved',href:'/inbox/room1',created_at:now,read:false});
const checks=[];
for(const width of [390,820,1440])for(const theme of ['light','dark']) {
 await page.setViewportSize({width,height:844});await page.goto(base+'/projects');await page.evaluate(value=>{localStorage.setItem('cuan-theme',value);localStorage.setItem('cuan-language','en')},theme);await page.reload();
 const trigger=page.getByRole('button',{name:'Notifications',exact:true});await trigger.click();
 if(width<768) {
  await page.waitForURL(base+'/notifications');await page.getByRole('button',{name:'Mark all as read',exact:true}).waitFor();
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.screenshot({path:'docs/evidence/notification-popover/'+theme+'-'+width+'.png'});
  await page.getByRole('button',{name:'Back',exact:true}).click();await page.waitForURL(base+'/projects');
  checks.push({width,theme,directNotificationPage:true,directMarkAllRead:true,backReturnsToOrigin:true,noOverflow:true});continue;
 }
 const panel=page.getByRole('dialog',{name:'Notifications',exact:true});await panel.waitFor();
 assert.equal(new URL(page.url()).pathname,'/projects');assert.equal(await panel.locator('li').count(),3);
 assert(await panel.getByRole('button',{name:'Mark all as read',exact:true}).isEnabled());
 const bounds=await panel.boundingBox();assert(bounds.x>=0&&bounds.x+bounds.width<=width+1&&bounds.y>=0&&bounds.y+bounds.height<=845);
 assert(width<768 ? bounds.y+bounds.height>=843 : bounds.y<100);
 await page.screenshot({path:'docs/evidence/notification-popover/'+theme+'-'+width+'.png'});
 await page.keyboard.press('Escape');await panel.waitFor({state:'hidden'});assert(await trigger.evaluate(el=>document.activeElement===el));
 checks.push({width,theme,preservesCurrentPage:true,placementCorrect:true,directMarkAllRead:true,escapeRestoresFocus:true,noOverflow:true});
}
const trigger=page.getByRole('button',{name:'Notifications',exact:true});await trigger.click();const panel=page.getByRole('dialog',{name:'Notifications',exact:true});await panel.waitFor();
notificationReadFails=true;await panel.getByRole('button',{name:'Mark all as read',exact:true}).click();await panel.getByRole('alert').waitFor();assert(notifications.some(n=>!n.read&&!n.resolved));
notificationReadFails=false;await panel.getByRole('button',{name:'Mark all as read',exact:true}).click();await page.waitForFunction(()=>document.querySelector('[role="dialog"] button')?.disabled===true);assert(notifications.every(n=>n.read));
await panel.getByRole('link',{name:'View all notifications',exact:true}).click();await page.waitForURL(base+'/notifications');await panel.waitFor({state:'hidden'});
assert(await page.getByRole('button',{name:'Mark all as read',exact:true}).isDisabled());
await page.getByRole('button',{name:'Back',exact:true}).click();await page.waitForURL(base+'/projects');
notifications.forEach(n=>n.read=false);await trigger.click();await panel.getByRole('button',{name:'Mark all as read',exact:true}).waitFor({state:'visible'});await page.waitForFunction(()=>!document.querySelector('[role="dialog"] button')?.disabled);
notificationReadFails=true;await panel.getByRole('link').filter({hasText:'Check the interrupted reply'}).click();await page.waitForURL(base+'/inbox/room0');await panel.waitFor({state:'hidden'});assert.equal(notifications[0].read,false);
await page.goBack();await page.waitForURL(base+'/projects');
await page.evaluate(()=>localStorage.setItem('cuan-language','id'));await page.reload();await page.getByRole('button',{name:'Notifikasi',exact:true}).click();await page.getByRole('button',{name:'Tutup notifikasi',exact:true}).waitFor();await page.getByRole('link',{name:'Lihat semua notifikasi',exact:true}).waitFor();
assert.deepEqual(errors,[]);
fs.writeFileSync('docs/evidence/notification-popover/browser.json',JSON.stringify({checks,markFailureRetainsUnread:true,markSuccessUpdatesBadge:true,readFailureDoesNotBlockNavigation:true,viewAllReturnsToOrigin:true,indonesianLabels:true,providerCalls:0},null,2));console.log('Notification popover browser checks passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
