const fs=require('fs'),path=require('path'),source=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8');
let setup=source.slice(0,source.indexOf(' const checks=[];'));
setup=setup.replace('let inboxPending=true',`let recoveryAction='try_again', retryAccepted=false, loseReply=true, retryCalls=0;const retrySourceIds=[];let inboxPending=true`);
setup=setup.replace("    if(method==='POST'&&/\\/chat\\/conversations",`    if(method==='POST' && /requests\\/[^/]+\\/retry$/.test(p)) {
      retryCalls++;retrySourceIds.push(p);retryAccepted=true;
      await new Promise(resolve=>setTimeout(resolve,150));
      if(loseReply){loseReply=false;return json({error:'Fixture reply lost'},503)}
      return json({request:{request_id:'retry-child'}},200)
    }
    if(method==='POST'&&/\\/chat\\/conversations`);
setup=setup.replace('const data = history(chat[1])',`const data = history(chat[1]);
data.messages=data.messages.slice(0,1);data.tasks=[];
const old={request_id:'retry-parent',message_id:'m1',reply_id:'r1',agent_id:'agent_good',state:recoveryAction==='check_status'?'interrupted':'failed',created_at:now,steps:[{seq:1,kind:'provider_result',text:'Provider process finished: exit 0, duration 40 s.',at:new Date(Date.parse(now)+40000).toISOString()}],safe_error:'Fixture failure',recovery_action:recoveryAction};
data.requests=retryAccepted?[old,{...old,request_id:'retry-child',reply_id:'r2',state:'queued',recovery_action:undefined,created_at:new Date(Date.parse(now)+1000).toISOString()}]:[old];`);
setup+=`
const checks=[];
for(const width of [390,820,1440]){
 await page.setViewportSize({width,height:844});retryAccepted=false;loseReply=true;recoveryAction='try_again';
 await page.goto(base+'/inbox/room0');
 const retry=page.getByRole('button',{name:'Try again',exact:true});await retry.waitFor();
 await retry.click();await page.getByRole('alert').filter({hasText:'Could not confirm recovery'}).waitFor();
 await retry.click();await page.locator('[data-response-state="queued"]').waitFor();
 assert.equal(await page.getByRole('button',{name:'Try again',exact:true}).count(),0);
 assert.equal(await page.getByText('Read project fixture',{exact:true}).count(),1);
 assert.equal(new Set(retrySourceIds.slice(-2)).size,1);
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 const before=retryCalls;retryAccepted=false;recoveryAction='check_status';await page.reload();
 await page.getByRole('button',{name:'Check status',exact:true}).click();assert.equal(retryCalls,before);assert(!(await page.locator('[data-response-state=interrupted]').innerText()).includes('40 s'));
 assert.equal(await page.getByRole('button',{name:'Try again',exact:true}).count(),0);
 recoveryAction='recovering';await page.reload();
 await page.getByRole('status').filter({hasText:'Recovering reply…'}).waitFor();
 assert.equal(await page.getByRole('button',{name:'Try again',exact:true}).count(),0);
 assert.equal(await page.getByRole('button',{name:'Check status',exact:true}).count(),0);
 recoveryAction='check_usage';await page.reload();await page.getByRole('link',{name:'Check usage',exact:true}).waitFor();
 assert.equal(await page.getByRole('link',{name:'Check usage',exact:true}).getAttribute('href'),'/settings/agents');
 checks.push({width,lostResponseSafeRetry:true,sameSourceId:true,oneUserMessage:true,unknownOutcomeOnlyChecksStatus:true,quotaLinksToSettings:true,automaticRecoveryNeedsNoClick:true,noHorizontalOverflow:true});
}
assert.deepEqual(errors,[]);
fs.writeFileSync('docs/evidence/chat-retry/browser.json',JSON.stringify({checks,providerCalls:0},null,2));
console.log('Chat recovery browser fixtures passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
