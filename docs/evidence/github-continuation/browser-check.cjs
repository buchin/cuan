const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const out = 'docs/evidence/landing'","const out = 'docs/evidence/github-continuation'");
setup=setup.replace('const now = new Date().toISOString()',"let githubConnected=false;const now = new Date().toISOString()");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')",`if(p==='/api/v1/integrations/github'){if(method==='PUT')githubConnected=true;return json({connected:githubConnected,account:githubConnected?{login:'fixture-owner'}:undefined});}
 if(p==='/api/v1/item-preferences')return json({items:{}});
    if(p==='/api/v1/provider-accounts')`);
setup+=`
tasks[0].status='blocked';tasks[0].activities=[{id:'required',activity_type:'github_connection_required',message:'Connect GitHub',created_at:now}];
const checks=[];
for(const width of [360,820,1440]){
 githubConnected=false;await page.setViewportSize({width,height:844});await page.goto(base+'/tasks/task_one');
 const notice=page.getByLabel('GitHub connection required',{exact:true});await notice.waitFor();
 await notice.getByRole('link',{name:'Connect GitHub',exact:true}).click();
 await page.getByLabel('GitHub access token',{exact:true}).fill('fixture-only-token');
 await page.getByRole('button',{name:'Connect GitHub',exact:true}).click();
 await page.getByRole('link',{name:'Back to task',exact:true}).click();await page.waitForURL('**/tasks/task_one');await notice.waitFor();
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:out+'/task-'+width+'.png'});checks.push({width,sameTaskReturn:true,noOverflow:true});
}
await page.goto(base+'/settings/integrations/github?return_to='+encodeURIComponent('https://example.invalid/escape'));
await page.getByText('fixture-owner',{exact:true}).waitFor();assert.equal(await page.getByRole('link',{name:'Back to task',exact:true}).count(),0);
tasks[0].activities.push({id:'resolved',activity_type:'github_connection_resolved',message:'Saved',created_at:new Date(Date.parse(now)+1000).toISOString()});await page.goto(base+'/tasks/task_one');
await page.getByText(tasks[0].title,{exact:true}).first().waitFor();assert.equal(await page.getByLabel('GitHub connection required',{exact:true}).count(),0);
assert.equal(launches.length,0);assert.equal(composerSends.length,0);assert.deepEqual(errors,[]);
fs.writeFileSync(out+'/browser.json',JSON.stringify({checks,externalReturnRejected:true,resolvedHidesCTA:true,automaticAgentLaunchesInFixture:0,providerCalls:0},null,2));console.log('PASS task GitHub connection CTA and return');
})().catch(async e=>{console.error(await page.locator('body').innerText());console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
