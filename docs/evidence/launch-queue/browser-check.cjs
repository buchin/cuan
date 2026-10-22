const fs=require('fs'),path=require('path'),source=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8');
let setup=source.slice(0,source.indexOf(' const checks=[];'));
setup=setup.replace(" const p=u.pathname,r=route.request(),body=method==='GET'||method==='DELETE'?{}:r.postDataJSON()||{}",` const p=u.pathname,r=route.request(),body=method==='GET'||method==='DELETE'?{}:r.postDataJSON()||{}
 if(p==='/api/v1/tasks/task_one/cancel'&&method==='POST'){tasks[0].launch.state='cancelled';tasks[0].status='blocked';return json({status:'cancelled'}),true}`);
setup+=`
const checks=[];
for(const width of [390,820,1440]){
 tasks[0].launch={state:'queued',available_at:now,safe_error:'All AI session slots are in use.'};tasks[0].attempts=[];delete tasks[0].latest_attempt;tasks[0].status='backlog';
 await page.setViewportSize({width,height:844});await page.goto(base+'/tasks/task_one/sessions/latest');
 await page.getByText('Waiting for an available AI session. Cuan will start this work automatically.',{exact:true}).waitFor();
 const cancel=page.getByRole('button',{name:'Cancel waiting work',exact:true});await cancel.waitFor();
 assert.equal(await page.getByRole('button',{name:'Start',exact:true}).count(),0);
 assert.equal(await page.getByRole('combobox',{name:'Agent',exact:true}).count(),0);
 assert.equal(launches.length,0);
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:'docs/evidence/launch-queue/waiting-'+width+'.png'});
 await cancel.click();await page.getByText('The waiting work was cancelled.',{exact:true}).waitFor();
 assert.equal(launches.length,0);
 checks.push({width,automaticWaiting:true,cancelWaitingWork:true,noAgentSelection:true,noManualLaunch:true,noHorizontalOverflow:true});
}
assert.deepEqual(errors,[]);
fs.writeFileSync('docs/evidence/launch-queue/browser.json',JSON.stringify({checks,providerCalls:0},null,2));
console.log('Waiting launch browser fixture passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
