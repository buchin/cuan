// First-owner UI with fixture API; no real owner/account or production changes.
const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const now = new Date().toISOString()", "let ownerSetup=true, sessionActive=false, setupFails=false, loginFails=false, statusFails=false, setupConflict=false; let setupWrites=0; const now = new Date().toISOString()");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')return json({accounts:backupAccounts});", `
    if(p==='/api/v1/auth/status')return statusFails?json({error:'unavailable'},503):json({setup_required:ownerSetup,owner_configured:!ownerSetup});
    if(p==='/api/v1/auth/me')return sessionActive && r.headers().authorization==='Bearer fixture-only'?json({owner:profile,user:profile}):json({error:'unauthorized'},401);
    if(p==='/api/v1/auth/setup') {
      setupWrites++;
      if(setupConflict){ownerSetup=false;return json({error:'already configured'},403);}
      if(setupFails)return json({error:'Could not create account. Try again.'},503);
      ownerSetup=false;sessionActive=true;return json({token:'fixture-only',owner:profile},201);
    }
    if(p==='/api/v1/auth/login') {if(loginFails)return json({error:'Incorrect username or password.'},401);sessionActive=true;return json({token:'fixture-only',owner:profile});}
    if(p==='/api/v1/agents'&&method==='GET')return json({agents:[]});
    if(p==='/api/v1/provider-accounts')return json({accounts:[]});
`);
setup+=`
const checks=[];
for(const width of [390,820,1440]) {
 ownerSetup=true;sessionActive=false;
 await page.setViewportSize({width,height:844});await page.goto(base+'/');
 await page.getByRole('heading',{name:'Set up your workspace',exact:true}).waitFor();
 assert.equal(await page.getByText('Use default admin',{exact:true}).count(),0);
 await page.getByLabel('Email',{exact:true}).fill('owner@example.invalid');
 await page.getByLabel('Password',{exact:true}).fill('fixture-password-123');
 await page.getByRole('button',{name:'Show password',exact:true}).click();
 assert.equal(await page.getByLabel('Password',{exact:true}).getAttribute('type'),'text');
 await page.getByRole('button',{name:'Hide password',exact:true}).click();
 setupFails=true;await page.getByRole('button',{name:'Create account',exact:true}).click();
 await page.getByRole('alert').filter({hasText:'Could not create account. Try again.'}).waitFor();
 assert.equal(await page.getByLabel('Email',{exact:true}).inputValue(),'owner@example.invalid');
 assert.equal(await page.getByLabel('Password',{exact:true}).inputValue(),'fixture-password-123');
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:'docs/evidence/onboarding/owner-'+width+'.png'});
 setupFails=false;await page.getByRole('button',{name:'Create account',exact:true}).click();
 await page.waitForURL(base+'/studio');await page.getByRole('heading',{name:'Connect your AI to get started',exact:true}).waitFor();
 await page.reload();await page.getByRole('heading',{name:'Connect your AI to get started',exact:true}).waitFor();
 assert.equal(await page.getByRole('button',{name:'Create account',exact:true}).count(),0);
 checks.push({width,rootShowsOwnerSetup:true,explicitPassword:true,noSharedCredentials:true,failedSetupRetainsDraft:true,successfulSetupEntersAiOnboarding:true,sessionSurvivesReload:true,noOverflow:true});
}
ownerSetup=false;sessionActive=false;loginFails=true;await page.goto(base+'/login');
await page.getByLabel('Username or email',{exact:true}).fill('owner@example.invalid');
await page.getByLabel('Password',{exact:true}).fill('fixture-password-123');
await page.getByRole('button',{name:'Sign in',exact:true}).click();
await page.getByRole('alert').filter({hasText:'Incorrect username or password.'}).waitFor();
assert.equal(await page.getByLabel('Username or email',{exact:true}).inputValue(),'owner@example.invalid');
assert.equal(await page.getByLabel('Password',{exact:true}).inputValue(),'fixture-password-123');
loginFails=false;await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(base+'/studio');
// A stale first-run page must recover if another owner finishes setup first.
ownerSetup=true;sessionActive=false;setupConflict=true;await page.goto(base+'/login');
await page.getByLabel('Email',{exact:true}).fill('owner@example.invalid');await page.getByLabel('Password',{exact:true}).fill('fixture-password-123');
await page.getByRole('button',{name:'Create account',exact:true}).click();
await page.getByRole('alert').filter({hasText:'This workspace already has an owner. Sign in to continue.'}).waitFor();
await page.getByRole('button',{name:'Sign in',exact:true}).waitFor();setupConflict=false;
statusFails=true;await page.reload();await page.getByText('Could not check workspace setup. Check your connection and try again.',{exact:true}).waitFor();
statusFails=false;await page.getByRole('button',{name:'Try again',exact:true}).click();await page.getByRole('button',{name:'Sign in',exact:true}).waitFor();
assert.deepEqual(errors,[]);
fs.writeFileSync('docs/evidence/onboarding/owner-browser.json',JSON.stringify({checks,loginErrorRetainsDraft:true,concurrentOwnerSetupHandled:true,statusFailureRetry:true,setupWrites,realAccountsCreated:0,providerCalls:0},null,2));
console.log('Owner setup and sign-in browser checks passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
