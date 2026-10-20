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
await page.goto(base+'/inbox/room0?message=perf-10');await page.locator('[data-search-message="true"]').waitFor();
const target=await page.locator('[data-search-message="true"]').boundingBox(),list=await page.locator('.inbox-message-list').boundingBox();require('assert/strict')(target.y>=list.y&&target.y<list.y+list.height,'matched old message must be in viewport');
require('assert/strict')(await page.locator('.wa-message-bubble').count()>50,'old target must be mounted');console.log('Old message deep-link and scroll anchor passed.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'))
