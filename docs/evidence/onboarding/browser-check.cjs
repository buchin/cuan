// Exact built assets with fixture APIs; no real OAuth, provider inference or production writes.
const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const now = new Date().toISOString()", "let onboardingFail=false; const now = new Date().toISOString()");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')return json({accounts:backupAccounts});", `
    if(p==='/api/v1/agents' && method==='GET')return onboardingFail?json({error:'fixture unavailable'},503):json({agents});
    if(p==='/api/v1/provider-accounts')return method==='POST'?json({account:{id:'acct_onboard'}},201):json({accounts:[]});
    if(p==='/api/v1/provider-accounts/acct_onboard/auth-init')return json({auth_url:'https://example.invalid/oauth',session_id:'fixture-session'});
    if(p==='/api/v1/provider-accounts/acct_onboard/auth-code')return json({account:{id:'acct_onboard',credentials_present:true,state:'verified'}});
    if(p==='/api/v1/agents/connect-cli') {const body=r.postDataJSON();agents.push({id:'agent_dev',name:body.name,harness:'agy',provider:'agy',enabled:true,status:'active',connection_state:'unknown'});return json({agent:agents[0]},201);}
`);
setup+=`
const checks=[];
topicFail=false;
for(const width of [390,820,1440]) {
 agents.splice(0); tasks.splice(0); projects.splice(0,projects.length,{id:'default',name:'Default',repo_url:'',project_type:'custom',default_branch:'main',created_at:now});
 await page.setViewportSize({width,height:844});await page.goto(base+'/studio');
 await page.getByRole('heading',{name:'Connect your AI to get started',exact:true}).waitFor();
 const goal=page.getByRole('textbox',{name:'Your goal',exact:true});await goal.fill('Prepare my first project. Do not deploy.');
 assert.equal(await page.getByRole('button',{name:'Get it done',exact:true}).isEnabled(),false);
 assert.equal(topicRequests.length,0);
 await page.screenshot({path:'docs/evidence/onboarding/connect-'+width+'.png'});
 await page.getByRole('button',{name:'Connect your AI',exact:true}).click();
 await page.getByRole('dialog',{name:'Add agent',exact:true}).waitFor();
 await page.getByRole('button',{name:'Connect Google',exact:true}).click();
 await page.getByLabel('2. Paste the code from Google', {exact:true}).fill('fixture-code');
 await page.getByRole('button',{name:'Confirm sign-in',exact:true}).click();
 await page.waitForURL(base+'/studio');
 await page.getByRole('heading',{name:'Start with your first goal',exact:true}).waitFor();
 assert.equal(await goal.inputValue(),'Prepare my first project. Do not deploy.');
 assert.equal(await page.getByRole('button',{name:'Get it done',exact:true}).isEnabled(),true);
 assert.equal(topicRequests.length,0);
 await page.screenshot({path:'docs/evidence/onboarding/ready-'+width+'.png'});
 await page.getByRole('button',{name:'Connect a project (optional)',exact:true}).click();
 await page.getByRole('dialog').waitFor();
 await page.getByRole('button',{name:'Cancel',exact:true}).click();
 assert.equal(await goal.inputValue(),'Prepare my first project. Do not deploy.');
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 checks.push({width,noExecutionBeforeSubmission:true,connectFlowReturnsToStudio:true,draftRetained:true,projectOptional:true,noOverflow:true});
}
// A failed account fetch must not be mistaken for an empty roster.
onboardingFail=true;await page.reload();await page.getByText('Could not check your AI connections.',{exact:true}).waitFor();
assert.equal(await page.getByRole('heading',{name:'Connect your AI to get started',exact:true}).count(),0);
onboardingFail=false;await page.getByRole('button',{name:'Try again',exact:true}).click();await page.getByRole('button',{name:'Get it done',exact:true}).waitFor();
// Returning workspaces keep the ordinary Studio with no first-project prompt.
tasks.push({id:'old-task',title:'Existing work',status:'done',project_id:'default'});await page.reload();
await page.getByRole('button',{name:'Get it done',exact:true}).waitFor();
assert.equal(await page.getByRole('heading',{name:'Start with your first goal',exact:true}).count(),0);
await page.getByRole('button',{name:'Get it done',exact:true}).click();
await page.waitForURL(url=>url.pathname.startsWith('/inbox/'));
assert.equal(topicRequests.length,1);
assert.equal(topicRequests[0].goal,'Prepare my first project. Do not deploy.');
await page.reload();assert.equal(topicRequests.length,1);
assert.deepEqual(errors,[]);
fs.writeFileSync('docs/evidence/onboarding/browser.json',JSON.stringify({checks,accountFetchFailureHandled:true,existingWorkspaceUnchanged:true,oneSubmissionAfterSetup:true,realProviderCalls:0},null,2));
console.log('Onboarding browser checks passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
