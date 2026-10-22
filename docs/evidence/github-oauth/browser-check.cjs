const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const out = 'docs/evidence/landing'","const out = 'docs/evidence/github-oauth'");
setup=setup.replace('const now = new Date().toISOString()',"let oauthAvailable=true,oauthCheckFail=false,oauthStartFail=false,connected=false;const now = new Date().toISOString()");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')",`
 if(p==='/api/v1/integrations/github/oauth'){
  if(method==='GET')return oauthCheckFail?json({error:'fixture failure'},503):json({configured:oauthAvailable});
  if(oauthStartFail)return json({error:'fixture failure'},503);
  assert.equal(r.postDataJSON().return_to,'/tasks/task_one');return json({authorize_url:'https://github.com/login/oauth/authorize?client_id=fixture&state=fixture-state'});
 }
 if(p==='/api/v1/integrations/github'){
  if(method==='PUT'){assert.equal(r.postDataJSON().token,'fixture-manual-token');connected=true;}
  return json({connected,account:connected?{login:'fixture-owner'}:undefined});
 }
    if(p==='/api/v1/provider-accounts')`);
setup+=`
await page.route('https://github.com/login/oauth/authorize**',route=>route.fulfill({contentType:'text/html',body:'<h1>Fixture GitHub authorization</h1>'}));
const target=base+'/settings/integrations/github?return_to='+encodeURIComponent('/tasks/task_one');const checks=[];
for(const width of [360,820,1440]){
 await page.setViewportSize({width,height:844});await page.goto(target);
 const connect=page.getByRole('button',{name:'Connect GitHub',exact:true});await connect.waitFor();await connect.isEnabled();
 assert.equal(await page.getByLabel('GitHub access token',{exact:true}).isVisible(),false);
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:out+'/primary-'+width+'.png'});checks.push({width,tokenHiddenByDefault:true,noOverflow:true});
}
oauthCheckFail=true;await page.reload();await page.getByText('Could not check GitHub sign-in availability.',{exact:true}).waitFor();oauthCheckFail=false;await page.getByRole('button',{name:'Check again',exact:true}).click();
await page.getByRole('button',{name:'Connect GitHub',exact:true}).waitFor();
oauthStartFail=true;await page.getByRole('button',{name:'Connect GitHub',exact:true}).click();await page.getByRole('alert').filter({hasText:'Could not start'}).waitFor();oauthStartFail=false;
await page.getByRole('button',{name:'Connect GitHub',exact:true}).click();await page.waitForURL('https://github.com/login/oauth/authorize**');await page.getByRole('heading',{name:'Fixture GitHub authorization'}).waitFor();
oauthAvailable=false;await page.goto(target);await page.getByText('GitHub sign-in needs to be configured by this installation’s administrator.',{exact:true}).waitFor();assert(await page.getByRole('button',{name:'Connect GitHub',exact:true}).isDisabled());
await page.getByText('Advanced',{exact:true}).click();await page.getByLabel('GitHub access token',{exact:true}).fill('fixture-manual-token');await page.getByRole('button',{name:'Connect with token',exact:true}).click();
await page.getByRole('link',{name:'Back to task',exact:true}).waitFor();assert((await page.getByRole('link',{name:'Back to task',exact:true}).getAttribute('href'))==='/tasks/task_one');
assert.equal(launches.length,0);assert.equal(composerSends.length,0);assert.deepEqual(errors,[]);
fs.writeFileSync(out+'/browser.json',JSON.stringify({checks,oauthNavigation:true,availabilityRetry:true,startFailureRetry:true,unconfiguredHonestState:true,manualAdvancedWorks:true,sameTaskReturn:true,liveGitHubCalls:0,providerCalls:0},null,2));console.log('PASS OAuth primary action, retry and Advanced token');
})().catch(async e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
