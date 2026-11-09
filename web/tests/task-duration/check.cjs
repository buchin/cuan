// Exact built app, isolated API fixture. No production/task writes.
const fs = require('fs'), path = require('path')
let setup = fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0]
setup = setup.replace("const out = 'docs/evidence/landing'", "const out = '/home/dev/artifacts/cuan-task-duration'").replaceAll('4181','4188')
setup = setup.replace("    if(p==='/api/v1/profile')", "    if(p==='/api/v1/item-preferences')return json({items:{}});\n    if(p==='/api/v1/profile')")
setup = setup.replace('const h=historyBase(id);', 'const h=historyBase(id);h.requests=[];h.tasks=tasks;h.messages=[];return h;')
setup = setup.replace("{tasks},listError?503:200", "{tasks,total:tasks.length,scope_total:tasks.length,counts:{all:tasks.length,active:1,done:1},page:1,page_size:25},listError?503:200")
setup += `
const epoch=Date.parse(now),ago=minutes=>new Date(epoch-minutes*60000).toISOString();
tasks.splice(0,tasks.length,
 {id:'task_one',project_id:'p1',title:'Siapkan panduan kerja',status:'done',priority:'normal',created_at:ago(60),completed_at:ago(53),updated_at:now,summary:'Panduan sudah diperiksa dan siap digunakan.',instructions:'Buat panduan kerja.',attempts:[],activities:[]},
 {id:'task_two',project_id:'p1',title:'Periksa hasil terbaru',status:'review',priority:'normal',review_state:'running',created_at:ago(5),updated_at:now,summary:'Hasil sedang diperiksa.',latest_attempt:{id:'verify',task_id:'task_two',status:'succeeded',created_at:ago(1),finished_at:now},attempts:[],activities:[]}
);
const checks=[];
for(const width of [390,1440])for(const theme of ['light','dark']){
 await page.setViewportSize({width,height:900});
 await page.goto(base+'/tasks');
 await page.evaluate(t=>{localStorage.setItem('cuan-theme',t);localStorage.setItem('cuan-language','id')},theme);
 await page.reload();
 await page.locator('[data-task-duration]').first().waitFor();
 assert(await page.locator('[data-task-duration][aria-label="Selesai dalam 7 menit"]').count()>0);
 assert(await page.locator('[data-task-duration][aria-label="Durasi 5 menit"]').count()>0);
 if(width===1440)assert.equal(await page.locator('thead th').count(),await page.locator('tbody tr').first().locator('td').count(),'Table headers must match data columns');
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:out+'/'+theme+'-'+width+'-list.png'});
 await page.goto(base+'/tasks/task_one');
 await page.getByText('Selesai dalam 7 menit',{exact:true}).waitFor();
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:out+'/'+theme+'-'+width+'-detail.png'});
 checks.push({width,theme,completedDuration:true,wholeTaskReviewDuration:true,noOverflow:true});
}
await page.goto(base+'/inbox/room0');
await page.locator('.composer-tasks .composer-mode-tab').last().click();
await page.getByRole('button',{name:/Siapkan panduan kerja/}).click();
await page.getByText('Selesai dalam 7 menit',{exact:true}).waitFor();
await page.screenshot({path:out+'/inbox-1440.png'});
await page.setViewportSize({width:390,height:900});
await page.screenshot({path:out+'/inbox-390.png'});
assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
await page.setViewportSize({width:1440,height:900});
await page.evaluate(()=>localStorage.setItem('cuan-language','en'));
tasks[0].created_at=ago(4380);tasks[0].completed_at=ago(55);
await page.goto(base+'/tasks/task_one');
await page.getByText('Completed in 3 days',{exact:true}).waitFor();
await page.screenshot({path:out+'/english-long-duration.png'});
tasks[0].created_at=ago(60);tasks[0].completed_at=ago(53);
await page.evaluate(()=>localStorage.setItem('cuan-language','id'));
await page.clock.install({time:new Date(epoch)});
await page.goto(base+'/tasks');
await page.locator('[data-task-duration][aria-label="Selesai dalam 7 menit"]').waitFor();
await page.clock.fastForward(120000);
await page.locator('[data-task-duration][aria-label="Durasi 7 menit"]').waitFor();
assert(await page.locator('[data-task-duration][aria-label="Selesai dalam 7 menit"]').count()>0,'Done timer must remain frozen');
tasks[0].completed_at=undefined;
await page.reload();
await page.getByText('Siapkan panduan kerja',{exact:true}).first().waitFor();
assert.equal(await page.locator('[data-task-duration][aria-label^="Selesai"]').count(),0,'Unknown completion must not use last edit');
assert.equal(writes.length,0);assert.equal(composerSends.length,0);assert.equal(launches.length,0);assert.deepEqual(errors,[]);
fs.writeFileSync(out+'/checks.json',JSON.stringify(checks,null,2));console.log('PASS: actual app task durations,390/1440 light/dark, frozen Done, live minute refresh, unknown history');
})().catch(async e=>{console.error(e);if(page)console.error((await page.locator('body').innerText()).slice(0,2500));console.error(unknown);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'))
