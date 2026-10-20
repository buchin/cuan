const fs=require('fs'),path=require('path');
let setup=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8').split(' const checks=[];')[0];
setup+=`
const checks=[];
tasks[0].status='doing';tasks[0].execution_status='running';tasks[1].status='blocked';tasks[1].project_id='p1';
for(const theme of ['light','dark']) for(const width of [360,390,820,1440]) {
 await page.setViewportSize({width,height:844});
 await page.goto(base+'/studio');await page.evaluate(theme=>{localStorage.setItem('cuan-theme',theme);localStorage.setItem('cuan-language','en')},theme);
 await page.goto(base+'/projects');
 const row=page.getByRole('link',{name:/Cuan fixture/});await row.waitFor();
 assert((await row.innerText()).includes('2 tasks'));
 assert((await row.innerText()).includes('1 working'));
 assert((await row.innerText()).includes('1 blocked'));
 const search=page.getByRole('textbox',{name:'Search projects',exact:true});
 const searchBox=await search.locator('..').boundingBox();assert(searchBox.height<=42);
 const add=page.getByRole('button',{name:'New project',exact:true});const box=await add.boundingBox();assert(box.height>=40&&box.height<=44);
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:'docs/evidence/project-list/projects-'+theme+'-'+width+'.png'});
 await search.fill('not-a-project');await page.getByText('No matching projects.').waitFor();
 await search.fill('Cuan');await row.waitFor();await search.fill('');
 await page.getByRole('button',{name:/Archived/}).click();assert.equal(await row.count(),0);
 await page.getByRole('button',{name:/Active/}).click();await row.waitFor();
 await row.click();await page.getByRole('button',{name:'New task',exact:true}).waitFor();
 checks.push({width,theme,compactSearch:true,workingAndBlockedCounts:true,archiveFilter:true,projectNavigation:true,noHorizontalOverflow:true,createButtonHeight:box.height});
}
assert.deepEqual(errors,[]);assert.equal(launches.length,0);
fs.writeFileSync('docs/evidence/project-list/browser.json',JSON.stringify({checks,providerCalls:0},null,2));
console.log('PASS project list responsive interactions');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`;
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'));
