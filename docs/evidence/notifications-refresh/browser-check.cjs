const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const now = new Date().toISOString()", "let notificationReadFails=false; const now = new Date().toISOString()");
setup=setup.replace("if(p==='/api/v1/notifications'){if(method==='PATCH'){", "if(p==='/api/v1/notifications'){if(method==='PATCH'){if(notificationReadFails)return json({error:'Fixture read update failed'},503);");
setup+=`
const stamp=Date.now(), yesterday=new Date(stamp),old=new Date(stamp);yesterday.setDate(yesterday.getDate()-1);old.setDate(old.getDate()-12);
notifications.splice(0,notifications.length,
 {id:'reply',kind:'blocker',title:'Reply interrupted',context:'Website refresh',summary:'Your message is saved. Open the conversation for recovery options.',status:'Needs attention',action_label:'Check reply',href:'/inbox/room0',created_at:new Date(stamp-120000).toISOString(),read:false},
 {id:'proposal',kind:'approval',title:'Improve navigation and simplify the first-time workspace experience '+ 'long_reference_'.repeat(10),context:'Website project',status:'Needs approval',action_label:'Review proposal',href:'/inbox/room1',created_at:new Date(stamp-60000).toISOString(),read:false},
 {id:'approved',kind:'approval',title:'Update contact navigation',context:'CRM improvements',status:'Approved',resolved:true,action_label:'View conversation',href:'/inbox/room2',created_at:yesterday.toISOString(),read:false},
 {id:'customer',kind:'customer',title:'New customer message',context:'Alice Example',status:'Customer message',action_label:'View message',href:'/inbox/room0',created_at:old.toISOString(),read:true},
 {id:'done',kind:'completed',title:'Improve mobile navigation',context:'Website project',status:'Completed',action_label:'View work',href:'/tasks/task_one',created_at:new Date(stamp-300000).toISOString(),read:false}
);
const checks=[];
for(const width of [390,820,1440])for(const theme of ['light','dark']) {
 await page.setViewportSize({width,height:844});await page.goto(base+'/notifications');await page.evaluate(theme=>{localStorage.setItem('cuan-theme',theme);localStorage.setItem('cuan-language','en')},theme);await page.reload();
 await page.getByRole('heading',{name:'Today',exact:true}).waitFor();
 await page.getByRole('heading',{name:'Yesterday',exact:true}).waitFor();await page.getByRole('heading',{name:'Earlier',exact:true}).waitFor();
 await page.getByRole('link',{name:'Unread 3',exact:true}).waitFor();
 await page.getByText('Website refresh',{exact:true}).waitFor();await page.getByText('Check reply',{exact:true}).waitFor();
 assert.equal(await page.locator('time').count(),5);
 assert(await page.locator('time').first().getAttribute('title'));
 assert(!(/\\d{1,2}\\/\\d{1,2}\\//).test(await page.locator('time').first().innerText()));
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:'docs/evidence/notifications-refresh/'+theme+'-'+width+'.png'});
 await page.getByRole('link',{name:'Unread 3',exact:true}).click();
 await page.getByText('Update contact navigation',{exact:true}).waitFor({state:'hidden'});
 assert.equal(await page.getByText('Update contact navigation',{exact:true}).count(),0);
 assert.equal(await page.getByText('Alice Example',{exact:true}).count(),0);
 assert.equal(await page.locator('time').count(),3);
 checks.push({width,theme,contextVisible:true,relativeTimeWithFullTooltip:true,dayGroups:true,resolvedExcludedFromUnread:true,noHorizontalOverflow:true});
}
await page.getByRole('link',{name:'All',exact:true}).click();
notificationReadFails=true;await page.getByRole('link').filter({hasText:'Reply interrupted'}).click();await page.waitForURL(base+'/inbox/room0');
assert.equal(notifications.find(n=>n.id==='reply').read,false);
await page.goBack();await page.getByRole('alert').filter({hasText:'Could not mark notifications as read. Try again.'}).waitFor();
notificationReadFails=false;await page.getByRole('link').filter({hasText:'Reply interrupted'}).click();await page.waitForURL(base+'/inbox/room0');
await page.goBack();await page.getByRole('link',{name:'Unread 2',exact:true}).waitFor();
await page.evaluate(()=>localStorage.setItem('cuan-language','id'));await page.reload();await page.getByRole('heading',{name:'Hari ini',exact:true}).waitFor();await page.getByText('Periksa balasan',{exact:true}).waitFor();
await page.getByRole('button',{name:'Tindakan notifikasi',exact:true}).click();await page.getByRole('menuitem',{name:'Tandai semua dibaca',exact:true}).click();
await page.getByRole('link',{name:'Belum dibaca 0',exact:true}).waitFor();
assert.deepEqual(errors,[]);
fs.writeFileSync('docs/evidence/notifications-refresh/browser.json',JSON.stringify({checks,navigationSurvivesReadWriteFailure:true,readStateUpdatesOnSuccess:true,indonesianCopy:true,markAllRead:true,providerCalls:0},null,2));
console.log('Notifications compact/context/reconciliation browser checks passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
