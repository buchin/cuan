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
setup+=`
const checks=[];
await page.setViewportSize({width:390,height:844});await page.goto(base+'/inbox');
const menuButton=page.getByRole('button',{name:'Actions for @verified',exact:true});
await menuButton.waitFor();
await menuButton.click();await page.getByRole('menuitem',{name:'Mark as unread',exact:true}).click();
await page.locator('[aria-label="1 unread messages"]:visible').waitFor();
const row=page.getByRole('button').filter({has:page.locator('[aria-label="1 unread messages"]:visible')});
assert((await row.locator('span.text-foreground').first().getAttribute('class')).includes('font-semibold'));
await page.screenshot({path:'docs/evidence/item-actions/unread-390.png'});
await page.reload();await page.locator('[aria-label="1 unread messages"]:visible').waitFor();
await menuButton.click();await page.getByRole('menuitem',{name:'Mark as read',exact:true}).click();
await page.locator('[aria-label="1 unread messages"]:visible').waitFor({state:'detached'});
await page.screenshot({path:'docs/evidence/item-actions/read-390.png'});
await menuButton.click();await page.getByRole('menuitem',{name:'Archive conversation',exact:true}).click();
await menuButton.waitFor({state:'detached'});await page.getByRole('button',{name:'Undo',exact:true}).click();await menuButton.waitFor();
await menuButton.click();await page.getByRole('menuitem',{name:'Archive conversation',exact:true}).click();await page.reload();
await page.getByRole('button',{name:'Archived conversations',exact:true}).click();await menuButton.waitFor();
await menuButton.click();await page.getByRole('menuitem',{name:'Restore conversation',exact:true}).click();
await page.getByRole('button',{name:'Back to Inbox',exact:true}).click();await menuButton.waitFor();
preferenceFail=true;await menuButton.click();await page.getByRole('menuitem',{name:'Archive conversation',exact:true}).click();
await page.getByText('Could not complete this action. Try again.',{exact:true}).waitFor();assert(await menuButton.isVisible());preferenceFail=false;
// Real touch input rather than synthetic pointer events.
const cdp=await browser.newCDPSession(page);await cdp.send('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:1});
const bounds=await menuButton.locator('..').boundingBox();const x=bounds.x+100,y=bounds.y+30;
await cdp.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x,y}]});
await page.waitForTimeout(650);await cdp.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
await page.getByRole('menu').waitFor();await page.keyboard.press('Escape');assert.equal(await menuButton.getAttribute('aria-expanded'),'false');
await menuButton.focus();await page.keyboard.press('Shift+F10');await page.getByRole('menu').waitFor();await page.keyboard.press('ArrowDown');assert.equal(await page.getByRole('menuitem',{name:'Mark as read',exact:true}).count(),0);
await page.keyboard.press('Escape');
await cdp.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:x+120,y}]});
await cdp.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:x+20,y:y+2}]});
await page.getByText('Archive',{exact:true}).waitFor();await page.screenshot({path:'docs/evidence/item-actions/swipe-preview-390.png'});
await cdp.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
await page.getByRole('button',{name:'Undo',exact:true}).waitFor();await menuButton.waitFor({state:'detached'});
await page.getByRole('button',{name:'Undo',exact:true}).click();await menuButton.waitFor();

// A short swipe and a vertical gesture must never archive or open a menu.
const countBefore=preferenceWrites.length;
await cdp.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:x+100,y}]});
await cdp.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:x+70,y:y+1}]});
await cdp.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
assert(await menuButton.isVisible());assert.equal(preferenceWrites.length,countBefore);
await cdp.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x,y}]});
await cdp.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:x+1,y:y+30}]});
await page.waitForTimeout(650);await cdp.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
assert.equal(await page.getByRole('menu').count(),0);assert.equal(preferenceWrites.length,countBefore);
await cdp.detach();
for(const theme of ['light','dark'])for(const width of [360,820,1440]){
 await page.setViewportSize({width,height:844});await page.evaluate(theme=>localStorage.setItem('cuan-theme',theme),theme);await page.goto(base+'/inbox');
 await menuButton.click();await page.getByRole('menu').waitFor();assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:'docs/evidence/item-actions/inbox-'+theme+'-'+width+'.png'});await page.keyboard.press('Escape');
 checks.push({theme,width,visibleMenu:true,noOverflow:true});
}
await page.setViewportSize({width:390,height:844});await page.goto(base+'/contacts');
const contactMenu=page.getByRole('button',{name:'Actions for Alice Example',exact:true});await contactMenu.click();await page.getByRole('menuitem',{name:'Archive contact',exact:true}).click();await contactMenu.waitFor({state:'detached'});
await page.getByRole('button',{name:'Archived contacts',exact:true}).click();await contactMenu.waitFor();await contactMenu.click();await page.getByRole('menuitem',{name:'Restore contact',exact:true}).click();
await page.getByRole('button',{name:'Active contacts',exact:true}).click();await contactMenu.waitFor();
await page.goto(base+'/projects');await page.getByRole('button',{name:'Actions for Cuan fixture',exact:true}).click();await page.getByRole('menuitem',{name:'Open project',exact:true}).click();await page.getByRole('button',{name:'New task',exact:true}).waitFor();
await page.getByRole('button',{name:'Actions for '+tasks[0].title,exact:true}).click();await page.getByRole('menuitem',{name:'Open task',exact:true}).click();assert(page.url().includes('/tasks/task_one'));
await page.goto(base+'/notifications');const notif=page.getByRole('button',{name:/Actions for/}).first();await notif.click();await page.getByRole('menuitem',{name:'Mark as read',exact:true}).waitFor();

await page.keyboard.press('Escape');
inboxPending=false;await page.goto(base+'/inbox/room0');
await page.getByRole('button',{name:'Actions for Message 2',exact:true}).waitFor();
await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.__copied=text}}}));
await page.getByRole('button',{name:'Actions for Message 2',exact:true}).click();
await page.getByRole('menuitem',{name:'Copy message',exact:true}).click();assert.equal(await page.evaluate(()=>window.__copied),markdown);
await page.getByRole('button',{name:'Actions for Message 2',exact:true}).click();await page.getByRole('menuitem',{name:'Quote message',exact:true}).click();
assert((await page.getByRole('textbox',{name:'Message',exact:true}).inputValue()).startsWith('> ## Fixture answer'));
await page.screenshot({path:'docs/evidence/item-actions/message-390.png'});
tasks[0].status='doing';tasks[0].execution_status='idle';await page.goto(base+'/projects/p1/tasks');await page.getByText('Not running',{exact:true}).waitFor();
await page.goto(base+'/settings/agents');await page.getByText('Sign-in required',{exact:true}).waitFor();
await page.getByText('Connection status, remaining quota and work progress are separate. A connected account can still have limited quota.',{exact:true}).waitFor();
await page.goto(base+'/settings/profile');const name=page.getByLabel('Display name',{exact:true});
await name.waitFor();assert(await page.getByRole('button',{name:'Save display name',exact:true}).isDisabled());
await name.fill('New fixture name');await page.getByText('Unsaved changes',{exact:true}).waitFor();
settingsFail=true;await page.getByRole('button',{name:'Save display name',exact:true}).click();await page.getByText('Fixture save failed',{exact:true}).waitFor();assert.equal(await name.inputValue(),'New fixture name');await page.getByText('Unsaved changes',{exact:true}).waitFor();settingsFail=false;
await page.getByRole('button',{name:'Save display name',exact:true}).click();await page.getByText('Display name saved.',{exact:true}).waitFor();
assert(await page.getByRole('button',{name:'Save display name',exact:true}).isDisabled());
await page.goto(base+'/settings/notifications');const save=page.getByRole('button',{name:'Save preferences',exact:true});await save.waitFor();assert(await save.isDisabled());
await page.locator('form input[type=checkbox]').first().click();await page.getByText('Unsaved changes',{exact:true}).waitFor();
await save.click();await page.getByText('All changes saved',{exact:true}).waitFor();assert(await save.isDisabled());

await page.goto(base+'/projects/p1/settings');
const settingsForm=page.getByRole('form',{name:'Project settings',exact:true});await settingsForm.waitFor();
assert.equal(await page.getByRole('button',{name:'New task',exact:true}).count(),0);
const saveProject=settingsForm.getByRole('button',{name:'Save changes',exact:true});
assert(await saveProject.isDisabled());
assert.equal(await page.getByLabel('Repository URL',{exact:true}).isVisible(),false);
await page.getByLabel('Project name',{exact:true}).fill('Renamed fixture project');
await page.getByLabel('Goal',{exact:true}).fill('Deliver a useful business outcome.');
await page.getByLabel('Instructions for Cuan',{exact:true}).fill('Keep approved scope and report verification.');
projectSettingsFail=true;await saveProject.click();await page.getByText('Fixture project save failed',{exact:true}).waitFor();
assert.equal(await page.getByLabel('Project name',{exact:true}).inputValue(),'Renamed fixture project');
projectSettingsFail=false;await saveProject.click();await page.getByText('Changes saved.',{exact:true}).waitFor();assert(await saveProject.isDisabled());
await page.reload();await page.getByLabel('Project name',{exact:true}).waitFor();assert.equal(await page.getByLabel('Goal',{exact:true}).inputValue(),'Deliver a useful business outcome.');
await page.getByLabel('Project name',{exact:true}).fill('Conflict draft');
projectConflict=true;await saveProject.click();await page.getByRole('alert').filter({hasText:'Project changed elsewhere. Reload before saving.'}).waitFor();
assert.equal(await page.getByLabel('Project name',{exact:true}).inputValue(),'Conflict draft');
projectConflict=false;
page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'Reload saved settings',exact:true}).click();
await page.waitForFunction(()=>document.querySelector('#project-name')?.value==='Renamed fixture project');
assert.equal(await page.getByLabel('Project name',{exact:true}).inputValue(),'Renamed fixture project');
await page.getByText('Advanced',{exact:true}).click();await page.getByLabel('Default branch',{exact:true}).fill('next');
page.once('dialog',dialog=>dialog.accept());await saveProject.click();await page.getByText('Changes saved.',{exact:true}).waitFor();assert.equal(settingsWrites.at(-1).default_branch,'next');
for(const width of [360,390,820,1440])for(const theme of ['light','dark']){
 await page.setViewportSize({width,height:844});await page.evaluate(value=>localStorage.setItem('cuan-theme',value),theme);
 await page.goto(base+'/projects/p1/settings');await page.getByLabel('Goal',{exact:true}).waitFor();
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:'docs/evidence/settings-panel/settings-'+theme+'-'+width+'.png'});
}
delete tasks[0].description;
tasks[0].instructions='Owner request for snapshot fixture'+'\\n\\nProject guidance snapshot (context only; does not expand task authorization):\\n'+'\\x60\\x60\\x60cuan-project-context\\n'+JSON.stringify({project_id:'p1',workspace_revision:'initial',goal:'Internal snapshot fixture'})+'\\n\\x60\\x60\\x60';
await page.goto(base+'/projects/p1/tasks/task_one');
await page.getByText('Owner request for snapshot fixture',{exact:true}).first().waitFor();
assert.equal(await page.getByText('Internal snapshot fixture',{exact:false}).count(),0);

await page.setViewportSize({width:1440,height:900});
await page.goto(base+'/projects/p1/tasks');
const sidebar=page.getByRole('complementary',{name:'Project list',exact:true});await sidebar.waitFor();
const taskSidebar=await sidebar.boundingBox();
await page.goto(base+'/projects/p1/settings');
await page.getByRole('form',{name:'Project settings',exact:true}).waitFor();
const settingsSidebar=await sidebar.boundingBox();
assert.equal(taskSidebar.x,settingsSidebar.x);assert.equal(taskSidebar.width,settingsSidebar.width);
const formBox=await page.getByRole('form',{name:'Project settings',exact:true}).boundingBox();
assert(formBox.x>=settingsSidebar.x+settingsSidebar.width);
assert.equal(await page.getByRole('link',{name:'← Back to project',exact:true}).count(),0);
await page.getByRole('link',{name:'Tasks',exact:true}).click();
await page.waitForURL('**/projects/p1/tasks');
fs.writeFileSync('docs/evidence/settings-panel/browser.json',JSON.stringify({editableNameGoalInstructions:true,advancedCollapsed:true,saveDisabledUntilDirty:true,failedSaveRetainsDraft:true,saveReload:true,conflictPreservesDraftAndReloadsExplicitly:true,connectionChangeConfirmation:true,noNewTaskInSettings:true,responsiveWidths:[360,390,820,1440],themes:['light','dark'],noOverflow:true,providerCalls:0},null,2));
assert.equal(launches.length,0);assert.equal(composerSends.length,0);assert.deepEqual(errors,[]);
fs.writeFileSync('docs/evidence/item-actions/browser.json',JSON.stringify({checks,touchLongPress:true,touchSwipeArchive:true,verticalScrollCancelsLongPress:true,shortSwipeDoesNotArchive:true,keyboardMenu:true,readUnreadReload:true,archiveUndo:true,failedArchiveRetainsRow:true,contactArchiveRestore:true,projectTaskNavigation:true,notificationMenu:true,messageCopyQuote:true,truthfulIdleTask:true,agentConnectionExplanation:true,settingsDirtySavedStates:true,failedSaveRetainsDraft:true,noTaskLaunches:true,providerCalls:0},null,2));
console.log('PASS cross-page item actions, real touch, read/unread and archive persistence');
})().catch(async e=>{if(page){await page.screenshot({path:'docs/evidence/item-actions/failure.png'});console.error(await page.locator('body').innerText())};console.error(JSON.stringify({itemPrefs,preferenceWrites}));console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
