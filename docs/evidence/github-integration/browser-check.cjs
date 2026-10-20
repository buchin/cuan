const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const out = 'docs/evidence/landing'", "const out = 'docs/evidence/github-integration'");
setup=setup.replace('const now = new Date().toISOString()',"let githubConnected=false, githubFailure=false;let repoPageFailure=false;const githubWrites=[];const now = new Date().toISOString()");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')",`
 if(p==='/api/v1/integrations/github'){
  if(method==='PUT'){githubWrites.push({method});if(githubFailure)return json({error:'fixture failure'},400);githubConnected=true;}
  if(method==='DELETE'){githubWrites.push({method});githubConnected=false;}
  return json({connected:githubConnected,account:githubConnected?{login:'fixture-owner',name:'Fixture Owner'}:undefined});
 }
 if(p==='/api/v1/integrations/github/repositories'){
  if(!githubConnected)return json({error:'connect first'},409);
  const page=Number(u.searchParams.get('page'));
  if(repoPageFailure)return json({error:'temporary failure'},503);
  return json({repositories:[{full_name:page===1?'fixture-owner/app':'fixture-owner/docs',default_branch:'trunk',permissions:{push:page===1}}],has_more:page===1});
 }
 if(method==='GET'&&p==='/api/v1/projects/p1')return json(projects[0]);
 if(method==='PATCH'&&p==='/api/v1/projects/p1/settings'){
  const data=r.postDataJSON();githubWrites.push({method,branch:data.default_branch,repo:data.repo_url});
  Object.assign(projects[0],data,{default_branch:data.default_branch||'trunk',updated_at:new Date().toISOString()});return json(projects[0]);
 }
    if(p==='/api/v1/provider-accounts')`);
setup+=`
 await page.goto(base+'/settings/integrations');
 await page.getByRole('link',{name:'GitHub →',exact:true}).click();
 await page.getByLabel('GitHub access token',{exact:true}).waitFor();
 await page.getByLabel('GitHub access token',{exact:true}).fill('fixture-not-a-real-token');
 githubFailure=true;await page.getByRole('button',{name:'Connect GitHub',exact:true}).click();
 await page.getByRole('alert').filter({hasText:'Could not connect'}).waitFor();
 assert.equal(await page.getByLabel('GitHub access token',{exact:true}).inputValue(),'fixture-not-a-real-token');
 githubFailure=false;await page.getByRole('button',{name:'Connect GitHub',exact:true}).click();
 await page.getByText('fixture-owner',{exact:true}).waitFor();
 assert.equal(await page.locator('input[type=password]').count(),0);
 const layouts=[];
 for(const theme of ['light','dark'])for(const width of [360,820,1440]){
  await page.setViewportSize({width,height:844});await page.evaluate(t=>localStorage.setItem('cuan-theme',t),theme);await page.reload();
  await page.getByText('fixture-owner',{exact:true}).waitFor();assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.screenshot({path:out+'/connection-'+theme+'-'+width+'.png'});layouts.push({theme,width,noOverflow:true});
 }
 await page.goto(base+'/projects/p1/settings');await page.getByText('Advanced',{exact:true}).click();
 await page.getByRole('button',{name:'Choose from GitHub',exact:true}).click();
 await page.getByLabel('Accessible repositories',{exact:true}).selectOption('fixture-owner/app');
 assert.equal(await page.getByLabel('Default branch',{exact:true}).inputValue(),'');
 repoPageFailure=true;await page.getByRole('button',{name:'Load more',exact:true}).click();
 await page.getByRole('alert').filter({hasText:'Could not load repositories'}).waitFor();
 assert.equal(await page.getByLabel('Accessible repositories',{exact:true}).locator('option').count(),2);
 repoPageFailure=false;await page.getByRole('button',{name:'Try again',exact:true}).click();
 await page.getByLabel('Accessible repositories',{exact:true}).selectOption('fixture-owner/docs');
 page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'Save changes',exact:true}).click();
 await page.getByText('Changes saved.',{exact:true}).waitFor();assert.equal(await page.getByLabel('Default branch',{exact:true}).inputValue(),'trunk');
 assert.equal(githubWrites.at(-1).branch,'');
 await page.setViewportSize({width:360,height:844});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:out+'/project-picker-360.png'});
 await page.goto(base+'/settings/integrations/github');page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'Disconnect',exact:true}).click();
 await page.getByLabel('GitHub access token',{exact:true}).waitFor();
 assert.equal(launches.length,0);assert.equal(composerSends.length,0);assert.deepEqual(errors,[]);
 fs.writeFileSync(out+'/browser.json',JSON.stringify({connectionFailureRetainsDraft:true,connectHidesToken:true,disconnect:true,pickerPagination:true,failedPageRetainsRows:true,defaultBranchFromGitHub:true,layouts,projectMobileNoOverflow:true,providerCalls:0,liveGitHubWrites:0},null,2));
 console.log('PASS GitHub connection, project picker, pagination recovery and responsive layouts');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
