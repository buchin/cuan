// Production bundle with synthetic API data only. No provider calls.
const fs=require('fs'),path=require('path')
const source=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8')
let setup=source.slice(0,source.indexOf(' const checks=[];'))
setup=setup.replace("const now = new Date().toISOString()",`const now = new Date().toISOString();let globalFail=false;const searchNote={id:'memory-budi',kind:'note',body:'His child likes dinosaurs. Follow up about cycling.',recorded_by:'You',source_room_id:'',source_message_id:'',created_at:'2020-01-01T00:00:00Z',updated_at:'2020-01-01T00:00:00Z'};`)
setup=setup.replace("    if(p==='/api/v1/auth/me')",`    if(p==='/api/v1/contacts/search'){
      const q=(new URL(r.url()).searchParams.get('q')||'').toLowerCase();if(q==='slow')await new Promise(resolve=>setTimeout(resolve,650));
      const direct=crm.filter(c=>(c.name+' '+c.email+' '+c.product+' '+c.tags.join(' ')).toLowerCase().includes(q));const note=q==='dinosaur'||q==='dinosaurs'?[{...crm[1],search_note:{id:searchNote.id,snippet:searchNote.body}}]:[];
      return json({contacts:[...direct,...note.filter(c=>!direct.some(d=>d.id===c.id))],has_more:false})
    }
    if(p==='/api/v1/contacts/crm2/memory')return json({items:[searchNote],has_more:false});
    if(p==='/api/v1/search'){
      if(globalFail)return json({error:'temporary fixture error'},503);
      const q=(new URL(r.url()).searchParams.get('q')||'');const found=q.toLowerCase().includes('dinosaur');
      return json({groups:[
        {kind:'contacts',items:found?[{id:'crm2',title:'Budi',snippet:searchNote.body,url:'/contacts?contact=crm2'}]:[],has_more:false},
        {kind:'notes',items:found?[{id:searchNote.id,title:'Budi — saved note',snippet:searchNote.body,url:'/contacts?contact=crm2&note_query=dinosaur&note=memory-budi'}]:[],has_more:false},
        {kind:'conversations',items:found?[{id:'room0',title:'Dinosaur conversation',snippet:'Networking follow-up',url:'/inbox/room0'}]:[],has_more:false},
        {kind:'projects',items:found?[{id:'p1',title:'Dinosaur project',snippet:'Project context',url:'/projects/p1/tasks'}]:[],has_more:false},
        {kind:'tasks',items:found?[{id:'task_one',title:'Dinosaur task',snippet:'Task context',url:'/projects/p1/tasks/task_one'}]:[],has_more:false}
      ]})
    }
    if(p==='/api/v1/auth/me')`)
setup+=`
const checks=[],errors=[];page.on('pageerror',e=>errors.push(e.message));
for(const width of [360,820,1440])for(const theme of ['light','dark']){
 await page.setViewportSize({width,height:844});await page.goto(base+'/contacts');await page.evaluate(theme=>{localStorage.setItem('cuan-theme',theme);localStorage.setItem('cuan-language','en')},theme);await page.reload();
 const sidebar=page.getByRole('complementary',{name:'Contact list',exact:true}),search=page.getByRole('textbox',{name:'Search contacts and notes',exact:true});await search.fill('dinosaur');const result=sidebar.getByRole('button',{name:/Budi/});await result.waitFor();assert.equal(await result.count(),1);assert((await result.innerText()).includes('His child likes dinosaurs'));await result.click();await page.locator('[data-search-match="true"]').waitFor();assert.equal(await page.getByRole('textbox',{name:'Search notes',exact:true}).inputValue(),'dinosaur');
 await page.getByRole('button',{name:'Search Cuan',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Search Cuan',exact:true});await dialog.waitFor();const field=dialog.getByRole('searchbox',{name:'Search anything',exact:true});await field.fill('dinosaur');await dialog.getByRole('link',{name:/Dinosaur project/}).waitFor();assert.equal(await dialog.locator('[data-search-result]').count(),5);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:'docs/evidence/universal-search/search-'+theme+'-'+width+'.png'});
 await dialog.getByRole('region',{name:'Notes',exact:true}).getByRole('link').click();await dialog.waitFor({state:'detached'});await page.locator('[data-search-match="true"]').waitFor();checks.push({width,theme,noteSidebarSearch:true,deduplicatedContact:true,matchingNoteOpened:true,globalGroups:true,noOverflow:true});
}
await page.setViewportSize({width:1440,height:900});await page.goto(base+'/contacts');const search=page.getByRole('textbox',{name:'Search contacts and notes',exact:true});await search.fill('slow');await page.waitForTimeout(300);await search.fill('dinosaur');await page.getByRole('complementary',{name:'Contact list',exact:true}).getByRole('button',{name:/Budi/}).waitFor();await page.waitForTimeout(750);assert.equal(await page.getByRole('complementary',{name:'Contact list',exact:true}).getByRole('button',{name:/Budi/}).count(),1);
await page.keyboard.press('Control+k');const dialog=page.getByRole('dialog',{name:'Search Cuan',exact:true});await dialog.waitFor();const field=dialog.getByRole('searchbox',{name:'Search anything',exact:true});globalFail=true;await field.fill('dinosaur');await dialog.getByRole('alert').waitFor();globalFail=false;await dialog.getByRole('button',{name:'Try again',exact:true}).click();await dialog.locator('[data-search-result]').first().waitFor();await field.focus();await page.keyboard.press('ArrowDown');assert(await dialog.locator('[data-search-result]').first().evaluate(el=>el===document.activeElement));await field.focus();await page.keyboard.press('Escape');await dialog.waitFor({state:'detached'});await page.keyboard.press('Control+k');await dialog.waitFor();await dialog.getByRole('searchbox').fill('dinosaur');await dialog.getByRole('link',{name:/Dinosaur project/}).click();await page.getByRole('region',{name:'Project tasks',exact:true}).waitFor();
assert.deepEqual(errors,[]);fs.writeFileSync('docs/evidence/universal-search/browser.json',JSON.stringify({checks,staleSearchIgnored:true,keyboardShortcut:true,arrowNavigation:true,escapeClose:true,retry:true,resultNavigation:true,pageErrors:errors,providerCalls:0},null,2));console.log('Universal/sidebar search passed: six responsive/theme cases, note navigation, keyboard, retry and stale-response guards.');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'))
