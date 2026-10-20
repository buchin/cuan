const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const out = 'docs/evidence/landing'","const out = 'docs/evidence/integration-cards'");
setup=setup.replace('const now = new Date().toISOString()',"let connected=false,configured=true,connectionFailure=false;const now = new Date().toISOString()");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')",`
 if(p==='/api/v1/integrations/github/oauth')return json({configured});
 if(p==='/api/v1/integrations/github')return connectionFailure?json({error:'fixture failure'},503):json({connected,account:connected?{login:'fixture-owner'}:undefined});
    if(p==='/api/v1/provider-accounts')`);
setup+=`
await page.goto(base+'/settings/integrations');const checks=[];
for(const theme of ['light','dark'])for(const width of [360,820,1440]){
 await page.setViewportSize({width,height:844});await page.evaluate(t=>localStorage.setItem('cuan-theme',t),theme);await page.reload();
 const github=page.getByLabel('GitHub integration card',{exact:true}),live=page.getByLabel('Live Chat integration card',{exact:true});
 await github.getByText('Not connected',{exact:true}).waitFor();await live.getByRole('link',{name:'Set up Live Chat',exact:true}).waitFor();
 const left=await github.boundingBox(),right=await live.boundingBox();assert(width>=768?Math.abs(left.y-right.y)<1:right.y>left.y);
 assert.equal(await live.getByText('Connected',{exact:true}).count(),0);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:out+'/overview-'+theme+'-'+width+'.png'});checks.push({width,theme,noOverflow:true,responsiveCards:true});
}
connected=true;await page.reload();const github=page.getByLabel('GitHub integration card',{exact:true});await github.getByText('fixture-owner',{exact:true}).waitFor();await github.getByRole('link',{name:'Manage',exact:true}).click();await page.waitForURL('**/settings/integrations/github');await page.getByText('fixture-owner',{exact:true}).waitFor();
await page.getByRole('link',{name:'← Integrations',exact:true}).click();configured=false;connected=false;await page.reload();await github.getByText('Setup needed',{exact:true}).waitFor();
connectionFailure=true;await page.reload();await github.getByText('Status unavailable',{exact:true}).waitFor();connectionFailure=false;await github.getByRole('button',{name:'Try again',exact:true}).click();await github.getByText('Setup needed',{exact:true}).waitFor();
await page.getByLabel('Live Chat integration card',{exact:true}).getByRole('link',{name:'Set up Live Chat',exact:true}).click();await page.waitForURL('**/settings/integrations/live-chat');await page.getByRole('button',{name:'Copy widget code',exact:true}).waitFor();
assert.equal(launches.length,0);assert.equal(composerSends.length,0);assert.deepEqual(errors,[]);
fs.writeFileSync(out+'/browser.json',JSON.stringify({checks,connectedManage:true,unconfiguredSetup:true,statusRetry:true,liveChatDoesNotClaimConnected:true,detailNavigation:true,providerCalls:0},null,2));console.log('PASS integration cards, states, retry and navigation');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
