const fs=require('fs'),path=require('path'),source=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8');
let setup=source.slice(0,source.indexOf(' const checks=[];'));
setup=setup.replace('let inboxPending=true',`let planAccepted=false, planDeferred=false;const approvalCalls=[];let inboxPending=true`);
setup=setup.replace("    if(method==='POST'&&/\\/chat\\/conversations",`    if(method==='POST' && /\\/chat\\/conversations\\/[^/]+\\/messages$/.test(p)) {
      const data=r.postDataJSON();approvalCalls.push(data);if(data.plan_action==='defer'){planDeferred=true}else{planAccepted=true};return json({message:{uuid:'approved-input'}},202)
    }
    if(method==='POST'&&/\\/chat\\/conversations`);
setup=setup.replace('const data = history(chat[1])',`const data = history(chat[1]);
data.messages=[{id:1,uuid:'plan-message',author_type:'agent',sender_type:'operator',author_name:'Cuan',content:'Here is the proposed audit.',created_at:now,approval_title:'Verify navigation',approval_plan:{title:'Verify navigation',description:'Read-only navigation audit. Report evidence. Do not modify or deploy.',project_id:'p1'},approval_request_id:'91000000-0000-4000-8000-000000000011',plan_deferred:planDeferred,plan_defer_request_id:'91000000-0000-4000-8000-000000000012',approval_accepted:planAccepted}];data.tasks=[];
data.requests=planAccepted?[{request_id:'91000000-0000-4000-8000-000000000011',state:'completed',message_id:'approved-input',reply_id:'approval-reply',created_at:now,steps:[]}]:[];if(planDeferred)data.requests.push({request_id:'91000000-0000-4000-8000-000000000012',state:'completed',message_id:'deferred-input',reply_id:'deferred-reply',created_at:now,steps:[]});`);
setup+=`
const checks=[];
await page.addInitScript(()=>{Object.defineProperty(navigator,'languages',{get:()=>['id-ID','en-US']})});
await page.goto(base+'/inbox/room0');assert.equal(await page.evaluate(()=>document.documentElement.lang),'en');
await page.evaluate(()=>localStorage.setItem('cuan-language','id'));await page.reload();assert.equal(await page.evaluate(()=>document.documentElement.lang),'id');
await page.evaluate(()=>localStorage.setItem('cuan-language','auto'));await page.reload();assert.equal(await page.evaluate(()=>document.documentElement.lang),'id');
await page.evaluate(()=>localStorage.setItem('cuan-language','en'));await page.reload();
for(const width of [390,820,1440]){
 planAccepted=false;planDeferred=false;await page.setViewportSize({width,height:844});await page.goto(base+'/inbox/room0');
 const approve=page.getByRole('button',{name:'Approve and start',exact:true});await approve.waitFor();
 await page.getByText('Read-only navigation audit. Report evidence. Do not modify or deploy.',{exact:true}).waitFor();
 const input=page.locator('textarea').first();await input.fill('Keep this unsent draft');
 await page.getByRole('button',{name:'Change plan',exact:true}).click();
 assert((await input.inputValue()).startsWith('Keep this unsent draft'));
 assert((await input.inputValue()).includes('Changes to this plan — Verify navigation:'));
 assert(await input.evaluate(el=>el===document.activeElement));
 assert((await approve.boundingBox()).height>=44);
 await page.screenshot({path:'docs/evidence/plan-approval/choices-'+width+'.png'});
 await input.fill('Keep this unsent draft');
 await page.getByRole('button',{name:'Not now',exact:true}).click();
 await page.getByRole('button',{name:'Saved for later',exact:true}).waitFor();
 assert(await page.getByRole('button',{name:'Saved for later',exact:true}).isDisabled());
 assert.equal(approvalCalls.at(-1).plan_action,'defer');
 assert.equal(approvalCalls.at(-1).request_id,'91000000-0000-4000-8000-000000000012');
 assert.equal(await input.inputValue(),'Keep this unsent draft');

 await approve.click();await page.getByRole('button',{name:'Approved',exact:true}).waitFor();
 assert(await page.getByRole('button',{name:'Approved',exact:true}).isDisabled());
 assert.equal(await input.inputValue(),'Keep this unsent draft');
 assert.equal(approvalCalls.at(-1).approve_message_id,'plan-message');
 assert.equal(approvalCalls.at(-1).request_id,'91000000-0000-4000-8000-000000000011');
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:'docs/evidence/plan-approval/approval-'+width+'.png'});
 checks.push({width,exactScopeVisible:true,changePlanPreservesDraftAndFocus:true,deferralAction:true,minimumTouchHeight44:true,approveAction:true,stableRequestId:true,approvedButtonDisabled:true,draftPreserved:true,noHorizontalOverflow:true});
}
assert.deepEqual(errors,[]);
fs.writeFileSync('docs/evidence/plan-approval/browser.json',JSON.stringify({checks,englishDefaultOnIndonesianBrowser:true,explicitLanguageAndAutoPreserved:true,providerCalls:0},null,2));
console.log('Plan approval browser fixtures passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
