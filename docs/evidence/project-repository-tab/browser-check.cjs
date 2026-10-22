const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup=setup.replace("const now = new Date().toISOString()", "let projectSettingsFail=false;let projectConflict=false;const settingsWrites=[];let settingsFail=false;let preferenceFail=false; const itemPrefs={conversation:{},contact:{}}; const preferenceWrites=[]; const now = new Date().toISOString()");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')", `
    if(method==='GET' && /^\\/api\\/v1\\/projects\\/p[12]$/.test(p))return json(projects.find(project=>project.id===p.split('/')[4]));
    if(method==='PATCH' && p.endsWith('/settings') && p.startsWith('/api/v1/projects/')){
     const data=r.postDataJSON();settingsWrites.push(data);
     if(projectSettingsFail)return json({error:'Fixture project save failed'},503);
     if(projectConflict)return json({error:'Project changed elsewhere. Reload before saving.'},409);
     const item=projects.find(project=>project.id===p.split('/')[4]);
     Object.assign(item,{name:data.name,repo_url:data.repo_url,default_branch:data.default_branch,updated_at:new Date().toISOString(),metadata:{...(item.metadata||{}),cuan_settings:{goal:data.goal,instructions:data.instructions}}});
     return json(item);
    }
    if(settingsFail && method==='PATCH' && ['/api/v1/profile','/api/v1/notifications/preferences'].includes(p))return json({error:'Fixture save failed'},503);
    if(p==='/api/v1/item-preferences'){
      const kind=u.searchParams.get('kind');
      if(method==='PATCH'){
        const body=r.postDataJSON();preferenceWrites.push(body);
        if(preferenceFail)return json({error:'fixture preference unavailable'},503);
        const old=itemPrefs[kind][body.id]||{};
        if(body.archived!==undefined)old.archived_at=body.archived?new Date().toISOString():'';
        if(body.read!==undefined){old.unread=!body.read;if(body.read)old.read_at=body.seen_at||new Date().toISOString()}
        itemPrefs[kind][body.id]=old;
      }
      return json({items:itemPrefs[kind]});
    }
    if(p==='/api/v1/agents/auth-status')return json({runtime_prerequisites:{cli_available:true,sandbox_available:true,installation_supported:true}});
    if(p==='/api/v1/provider-accounts')`,1);
setup=setup.replaceAll("process.cwd()", "(process.env.CUAN_RELEASE_ROOT || process.cwd())");
setup=setup.replace("    if(p==='/api/v1/provider-accounts')", `
    if(p==='/api/v1/integrations/github/repositories')return json({repositories:[{full_name:'example/new-project',default_branch:'trunk',permissions:{push:true}}],has_more:false});
    if(p==='/api/v1/projects/default')return json(projects.find(p=>p.id==='default'));
    if(p==='/api/v1/provider-accounts')`);
setup+=`
const output='docs/evidence/project-repository-tab';fs.mkdirSync(output,{recursive:true});const checks=[];await page.goto(base+'/projects');
projects.push({id:'default',name:'Default',repo_url:'',project_type:'custom',created_at:now,updated_at:now});
await page.goto(base+'/projects');await page.getByRole('img',{name:'GitHub repository',exact:true}).waitFor();await page.getByRole('img',{name:'Project without a repository',exact:true}).waitFor();await page.setViewportSize({width:390,height:900});await page.screenshot({path:output+'/project-icons-390.png'});
for(const theme of ['light','dark'])for(const width of [390,820,1440]){
 await page.setViewportSize({width,height:900});await page.evaluate(value=>localStorage.setItem('cuan-theme',value),theme);
 await page.goto(base+'/projects/p1/settings');await page.getByLabel('Project name',{exact:true}).waitFor();
 assert.equal(await page.getByLabel('Repository URL',{exact:true}).count(),0);
 const nav=page.getByRole('navigation',{name:'Project navigation',exact:true});
 await nav.getByRole('link',{name:'Repository',exact:true}).click();
 await page.getByRole('form',{name:'Project repository',exact:true}).waitFor();
 assert(page.url().endsWith('/projects/p1/repository'));assert.equal(await nav.getByRole('link',{name:'Repository',exact:true}).getAttribute('aria-current'),'page');
 assert.equal(await nav.locator('[aria-current=page]').count(),1);assert.equal(await page.getByLabel('Goal',{exact:true}).count(),0);
 assert.equal(await page.getByLabel('Default branch',{exact:true}).isVisible(),false);
 await page.getByText('example/cuan',{exact:true}).waitFor();
 await page.getByRole('button',{name:'Choose from GitHub',exact:true}).waitFor();
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.waitForTimeout(180);await page.screenshot({path:output+'/repository-'+theme+'-'+width+'.png'});
 await page.reload();await page.getByRole('form',{name:'Project repository',exact:true}).waitFor();
 checks.push({theme,width,separateTab:true,advancedCollapsed:true,directReload:true,noOverflow:true});
}
await page.getByRole('navigation',{name:'Project navigation',exact:true}).getByRole('link',{name:'Settings',exact:true}).click();
await page.getByLabel('Goal',{exact:true}).fill('Retained draft across tabs');
await page.getByRole('navigation',{name:'Project navigation',exact:true}).getByRole('link',{name:'Repository',exact:true}).click();
await page.getByText('Unsaved changes',{exact:true}).waitFor();
await page.getByRole('navigation',{name:'Project navigation',exact:true}).getByRole('link',{name:'Settings',exact:true}).click();
assert.equal(await page.getByLabel('Goal',{exact:true}).inputValue(),'Retained draft across tabs');
await page.getByRole('button',{name:'Discard changes',exact:true}).click();
await page.getByRole('navigation',{name:'Project navigation',exact:true}).getByRole('link',{name:'Repository',exact:true}).click();
await page.getByRole('button',{name:'Choose from GitHub',exact:true}).click();await page.getByLabel('Accessible repositories',{exact:true}).selectOption('example/new-project');
await page.getByText('Selected repository',{exact:true}).waitFor();await page.getByText('Advanced',{exact:true}).click();
assert.equal(await page.getByLabel('Default branch',{exact:true}).inputValue(),'');
projectSettingsFail=true;page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Save changes',exact:true}).click();await page.getByRole('alert').filter({hasText:'Fixture project save failed'}).waitFor();
assert.equal(await page.getByLabel('Repository URL',{exact:true}).inputValue(),'https://github.com/example/new-project');
projectSettingsFail=false;page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Save changes',exact:true}).click();await page.getByText('Changes saved.',{exact:true}).waitFor();
assert.equal(settingsWrites.at(-1).default_branch,'');assert.equal(settingsWrites.at(-1).repo_url,'https://github.com/example/new-project');
await page.goto(base+'/projects/default/settings');await page.getByLabel('Project name',{exact:true}).waitFor();assert.equal(await page.getByRole('navigation',{name:'Project navigation',exact:true}).getByRole('link',{name:'Repository',exact:true}).count(),0);
assert.equal(launches.length,0);assert.equal(composerSends.length,0);assert.deepEqual(errors,[]);
const proof={checks,repositoryIcons:true,draftPreservedAcrossTabs:true,gitHubPicker:true,repositoryDefaultBranch:true,failedSaveRetainsSelection:true,confirmedSave:true,defaultHasNoRepositoryTab:true,providerCalls:0,api:'fixture'};
fs.writeFileSync(output+'/browser.json',JSON.stringify(proof,null,2));console.log(JSON.stringify(proof));
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
