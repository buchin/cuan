// Reuse the isolated app/API fixture, replacing only its scenario runner.
const fs=require('fs'),path=require('path')
const source=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8')
let setup=source.slice(0,source.indexOf(' const checks=[];'))
setup=setup.replace("    if(p==='/api/v1/auth/me')",`    if(method==='GET'&&p==='/api/v1/chat/conversations/room0/messages')return json(perfHistory);
    if(p==='/api/v1/auth/me')`)
setup=setup.replace('const data = history(chat[1])',"const data = chat[1] === 'room0' ? perfHistory : history(chat[1])")
setup=setup.replace("const now = new Date().toISOString()",`const now = new Date().toISOString();const perfHistory={room_id:'room0',conversation_id:'room0',messages:[],requests:[],tasks:[]};
for(let i=0;i<200;i++){const isAgent=i%2===1;perfHistory.messages.push({id:i+1,uuid:'perf-'+i,room_id:'room0',author_type:isAgent?'agent':'user',sender_type:isAgent?'operator':'visitor',author_name:isAgent?'@verified':'Owner',content:isAgent?'### Performance reply '+i+'\\n\\nA useful answer with **formatting**.\\n\\n- Read the project\\n- Verify the outcome\\n\\n\\x60\\x60\\x60go\\npackage main\\n\\x60\\x60\\x60':'Performance question '+i,created_at:now});if(isAgent)perfHistory.requests.push({request_id:'perf-request-'+i,message_id:'perf-'+(i-1),reply_id:'perf-'+i,agent_id:'agent_good',state:'completed',started_at:now,created_at:now,steps:Array.from({length:32},(_,j)=>({seq:j+1,kind:j===31?'provider_result':'runtime_tool_finished',text:j===31?'Provider process finished: exit 0.':'Read file completed.',at:now}))});}`)
setup+=`
const cdp=await page.context().newCDPSession(page);await cdp.send('Performance.enable');
const start=Date.now();await page.goto(base+'/inbox/room0');await page.getByRole('heading',{name:'Performance reply 199',exact:true}).waitFor();const ready=Date.now()-start;
const nodes=await cdp.send('Memory.getDOMCounters'),before=(await cdp.send('Performance.getMetrics')).metrics;await new Promise(r=>setTimeout(r,3300));const after=(await cdp.send('Performance.getMetrics')).metrics;
const metric=(list,name)=>list.find(x=>x.name===name)?.value||0;
const report={syntheticMessages:200,syntheticRequests:100,stepsPerRequest:32,historyBytes:Buffer.byteLength(JSON.stringify(perfHistory)),readyMs:ready,domNodes:nodes.nodes,renderedMessages:await page.locator('.wa-message-bubble').count(),collapsedTimelineNodes:await page.locator('[aria-label="Response timeline"] li').count(),idleScriptMs:(metric(after,'ScriptDuration')-metric(before,'ScriptDuration'))*1000,idleTaskMs:(metric(after,'TaskDuration')-metric(before,'TaskDuration'))*1000,jsHeapBytes:metric(after,'JSHeapUsedSize'),providerCalls:0};
const assert=require('assert/strict');
if(process.env.CUAN_PERF_LABEL==='after'){
 assert.equal(report.renderedMessages,50);assert.equal(report.collapsedTimelineNodes,0);
 const detail=page.locator('details[class*="group/response"]').first();
 await detail.locator('summary').first().click();await page.locator('[aria-label="Response timeline"] li').first().waitFor();
 assert.equal(await page.locator('[aria-label="Response timeline"] li').count(),31);
 await detail.locator('summary').first().click();await page.waitForTimeout(100);assert.equal(await page.locator('[aria-label="Response timeline"] li').count(),0);
 const list=page.locator('.inbox-message-list');await list.evaluate(el=>el.scrollTop=0);
 const beforeTop=await page.getByText('Performance question 150',{exact:true}).boundingBox();
 await page.getByRole('button',{name:'Load older messages',exact:true}).click();
 assert.equal(await page.locator('.wa-message-bubble').count(),100);
 const afterTop=await page.getByText('Performance question 150',{exact:true}).boundingBox();
 assert(Math.abs(beforeTop.y-afterTop.y)<3,'older history must preserve viewport anchor');
 await page.getByRole('button',{name:'Load older messages',exact:true}).click();await page.getByRole('button',{name:'Load older messages',exact:true}).click();
 assert.equal(await page.locator('.wa-message-bubble').count(),200);assert.equal(await page.getByRole('button',{name:'Load older messages',exact:true}).count(),0);
 report.regressions={lazyDetail:true,olderHistory:true,scrollAnchor:true};
 report.otherPages=[];
 for(const route of ['/contacts','/projects','/notifications','/settings']){
  await page.goto(base+route);await page.waitForTimeout(350);
  const first=(await cdp.send('Performance.getMetrics')).metrics;await page.waitForTimeout(1200);const last=(await cdp.send('Performance.getMetrics')).metrics;
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  report.otherPages.push({route,idleScriptMs:(metric(last,'ScriptDuration')-metric(first,'ScriptDuration'))*1000});
 }

}
fs.writeFileSync('docs/evidence/chat-performance/'+(process.env.CUAN_PERF_LABEL||'current')+'.json',JSON.stringify(report,null,2));console.log(JSON.stringify(report));
})().catch(async e=>{if(page){console.error((await page.locator('body').innerText()).slice(-1800));await page.screenshot({path:'docs/evidence/chat-performance/debug.png'})}console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`
// Evaluate with the fixture's require/filename environment, without writing copies.
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'))
