// Exact built assets with fixture APIs; no real OAuth, provider inference or production writes.
const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const now = new Date().toISOString()", "let runtimeMode='available'; let onboardingFail=false; const now = new Date().toISOString()");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')return json({accounts:backupAccounts});", `
    if(p==='/api/v1/agents/auth-status')return runtimeMode==='error'?json({error:'fixture unavailable'},503):json({runtime_prerequisites:{cli_available:runtimeMode!=='missing',sandbox_available:true,installation_supported:runtimeMode!=='unsupported',execution_verified:false}});
    if(p==='/api/v1/agents' && method==='GET')return onboardingFail?json({error:'fixture unavailable'},503):json({agents});
    if(p==='/api/v1/provider-accounts')return method==='POST'?json({account:{id:'acct_onboard'}},201):json({accounts:[]});
    if(p==='/api/v1/provider-accounts/acct_onboard/auth-init')return json({auth_url:'https://example.invalid/oauth',session_id:'fixture-session'});
    if(p==='/api/v1/provider-accounts/acct_onboard/auth-code')return json({account:{id:'acct_onboard',credentials_present:true,state:'verified'}});
    if(p==='/api/v1/agents/connect-cli') {const body=r.postDataJSON();agents.push({id:'agent_dev',name:body.name,harness:'agy',provider:'agy',enabled:true,status:'active',connection_state:'unknown'});return json({agent:agents[0]},201);}
`);
setup+=`
const checks=[];
topicFail=false;
for(const theme of ['light','dark']) for(const width of [360,390,820,1440]) {
 await page.goto(base+'/studio');await page.evaluate(theme=>localStorage.setItem('cuan-theme',theme),theme);
 agents.splice(0); tasks.splice(0); projects.splice(0,projects.length,{id:'default',name:'Default',repo_url:'',project_type:'custom',default_branch:'main',created_at:now});
 await page.setViewportSize({width,height:844});await page.goto(base+'/studio');
 await page.getByText('Write your goal, then connect your AI to get started. Your draft will stay here.',{exact:true}).waitFor();
 const goal=page.getByRole('textbox',{name:'Your goal',exact:true});await goal.fill('Prepare my first project. Do not deploy.');
 assert.equal(await page.getByRole('button',{name:'Connect Google',exact:true}).isEnabled(),true);
 assert.equal(topicRequests.length,0);
 await page.screenshot({path:'docs/evidence/direct-ai-connect/connect-'+theme+'-'+width+'.png'});
 await page.getByRole('button',{name:'Connect Google',exact:true}).click();
 await page.getByRole('dialog',{name:'Connect Google',exact:true}).waitFor();
 assert.equal(await page.getByLabel('Agent name',{exact:true}).count(),0);
 assert.equal(new URL(page.url()).pathname,'/studio');
 await page.getByLabel('2. Paste the code from Google', {exact:true}).fill('fixture-code');
 await page.getByRole('button',{name:'Confirm sign-in',exact:true}).click();
 await page.waitForURL(base+'/studio');
 await page.getByRole('button',{name:'Get it done',exact:true}).waitFor();
 assert.equal(await goal.inputValue(),'Prepare my first project. Do not deploy.');
 await page.waitForFunction(()=>Array.from(document.querySelectorAll('button')).some(b=>b.textContent.trim()==='Get it done'&&!b.disabled));
 assert.equal(await page.getByRole('button',{name:'Get it done',exact:true}).isEnabled(),true);
 assert.equal(topicRequests.length,0);
 await page.screenshot({path:'docs/evidence/direct-ai-connect/ready-'+theme+'-'+width+'.png'});
 await page.getByRole('button',{name:'Connect a project (optional)',exact:true}).click();
 await page.getByRole('dialog').waitFor();
 await page.getByRole('button',{name:'Cancel',exact:true}).click();
 assert.equal(await goal.inputValue(),'Prepare my first project. Do not deploy.');
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 checks.push({theme,width,noExecutionBeforeSubmission:true,directGoogleConnection:true,noAgentNameForm:true,connectFlowReturnsToStudio:true,draftRetained:true,projectOptional:true,noOverflow:true});
}
await page.setViewportSize({width:390,height:844});
// A failed account fetch must not be mistaken for an empty roster.
onboardingFail=true;await page.reload();await page.getByText('Could not check your setup. Your goal is kept.',{exact:true}).waitFor();
assert.equal(await page.getByRole('heading',{name:'Connect your AI to get started',exact:true}).count(),0);
onboardingFail=false;await page.getByRole('button',{name:'Try again',exact:true}).click();await page.getByRole('button',{name:'Get it done',exact:true}).waitFor();
// Runtime missing/unsupported and transport errors must preserve drafts and block submission.
for(const mode of ['missing','unsupported','error']) {
 runtimeMode=mode;await page.reload();
 await page.getByText(mode==='missing'?'The AI runtime is not installed yet. Your goal is saved.':mode==='unsupported'?'This installation does not yet support AI execution. Your goal is saved.':'Could not check your setup. Your goal is kept.',{exact:true}).waitFor();
 assert.equal(await page.getByRole('button',{name:'Get it done',exact:true}).isDisabled(),true);
 assert.equal(await page.getByRole('textbox',{name:'Your goal',exact:true}).inputValue(),'Prepare my first project. Do not deploy.');
 assert.equal(topicRequests.length,0);
 await page.screenshot({path:'docs/evidence/direct-ai-connect/'+mode+'.png'});
 runtimeMode='available';await page.getByRole('button',{name:mode==='error'?'Try again':'Check again',exact:true}).click();
 await page.getByRole('button',{name:'Get it done',exact:true}).waitFor();
 await page.waitForFunction(()=>Array.from(document.querySelectorAll('button')).some(b=>b.textContent.trim()==='Get it done'&&!b.disabled));
 assert.equal(await page.getByRole('button',{name:'Get it done',exact:true}).isEnabled(),true);
}
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
fs.writeFileSync('docs/evidence/direct-ai-connect/browser.json',JSON.stringify({checks,accountFetchFailureHandled:true,existingWorkspaceUnchanged:true,oneSubmissionAfterSetup:true,realProviderCalls:0},null,2));
console.log('Onboarding browser checks passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
