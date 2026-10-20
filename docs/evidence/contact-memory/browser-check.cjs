// Exact production build, isolated API fixture; no real inference or writes.
const fs=require('fs'),path=require('path')
const source=fs.readFileSync('docs/evidence/landing/browser-check.cjs','utf8')
let setup=source.slice(0,source.indexOf(' const checks=[];'))
setup=setup.replace("const now = new Date().toISOString()",`const now = new Date().toISOString();let memoryFail=false;let memoryItems=Array.from({length:61},(_,i)=>({id:'note-'+i,kind:i===0?'fact':'note',body:i===0?'Her child likes dinosaurs':'Meeting note '+i,recorded_by:'You',source_room_id:i===0?'room0':'',source_message_id:i===0?'m1':'',created_at:now,updated_at:now}));`)
setup=setup.replace("    if(p==='/api/v1/auth/me')",`    const memory=p.match(/^\\/api\\/v1\\/contacts\\/([^/]+)\\/memory(?:\\/([^/]+))?$/);
    if(memory){
      if(memoryFail)return json({error:'Could not save note. Try again.'},503);
      const id=memory[2];
      if(method==='GET'){const q=(new URL(r.url()).searchParams.get('q')||'').toLowerCase(),offset=Number(new URL(r.url()).searchParams.get('offset')||0);const filtered=memoryItems.filter(x=>x.body.toLowerCase().includes(q));return json({items:filtered.slice(offset,offset+50),has_more:filtered.length>offset+50})}
      if(method==='POST'){const b=r.postDataJSON();const item={...b,id:b.request_id,recorded_by:'You',source_room_id:'',source_message_id:'',created_at:now,updated_at:now};memoryItems.unshift(item);return json(item,201)}
      if(method==='PATCH'){const b=r.postDataJSON();memoryItems=memoryItems.map(x=>x.id===id?{...x,...b}:x);return json({ok:true})}
      if(method==='DELETE'){memoryItems=memoryItems.filter(x=>x.id!==id);return json({ok:true})}
    }
    if(p==='/api/v1/auth/me')`)
setup+=`
const results=[];const browserErrors=[];page.on('pageerror',e=>browserErrors.push(e.message));
for(const width of [360,820,1440])for(const theme of ['light','dark']){
 await page.setViewportSize({width,height:844});await page.goto(base+'/projects');await page.evaluate(theme=>{localStorage.setItem('cuan-theme',theme);localStorage.setItem('cuan-language','en')},theme);await page.reload();
 const list=page.getByRole('complementary',{name:'Project list',exact:true});await list.getByRole('link',{name:/Cuan fixture/}).click();await page.getByRole('region',{name:'Project tasks',exact:true}).waitFor();
 assert.equal(await page.getByRole('link',{name:/Review the saved result/}).count(),0);
 await page.screenshot({path:'docs/evidence/contact-memory/projects-'+theme+'-'+width+'.png'});
 if(width===1440){assert(await list.isVisible());await list.getByRole('button',{name:'Archived',exact:true}).click();await list.getByRole('link',{name:/Archived fixture/}).click();await page.getByRole('link',{name:/Review the saved result/}).waitFor()}
 else {assert(!(await list.isVisible()));await page.getByRole('link',{name:'Back to projects',exact:true}).click();await list.waitFor({state:'visible'})}
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.goto(base+'/contacts?contact=crm1');await page.getByRole('heading',{name:'Notes & about',exact:true}).waitFor();await page.getByText('Her child likes dinosaurs',{exact:true}).waitFor();assert.equal(await page.getByRole('form',{name:'Manage tags'}).isVisible(),false);
 await page.getByRole('button',{name:'Load older notes',exact:true}).click();await page.getByText('Meeting note 60',{exact:true}).waitFor();
 await page.getByRole('textbox',{name:'Search notes',exact:true}).fill('dinosaurs');await page.waitForTimeout(350);await page.getByText('Her child likes dinosaurs',{exact:true}).waitFor();assert.equal(await page.getByText('Meeting note 60',{exact:true}).count(),0);
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await page.screenshot({path:'docs/evidence/contact-memory/contact-'+theme+'-'+width+'.png'});
 results.push({width,theme,projectScopedTasks:true,responsiveProjectPicker:true,contactReadFirst:true,olderNotes:true,search:true,noOverflow:true});
}
await page.setViewportSize({width:1440,height:900});await page.goto(base+'/contacts?contact=crm1');await page.getByRole('button',{name:'Add note',exact:true}).click();const body=page.getByRole('textbox',{name:'What would you like to remember?',exact:true});await body.fill('Networking follow-up about cycling');memoryFail=true;await page.getByRole('button',{name:'Save',exact:true}).click();await page.getByRole('alert').filter({hasText:'Could not save note. Try again.'}).waitFor();assert.equal(await body.inputValue(),'Networking follow-up about cycling');memoryFail=false;await page.getByRole('button',{name:'Save',exact:true}).click();await page.getByText('Networking follow-up about cycling',{exact:true}).waitFor();
const card=page.getByRole('article').filter({hasText:'Networking follow-up about cycling'});await card.getByRole('button',{name:'Edit note',exact:true}).click();await body.fill('Updated networking note');await page.getByRole('button',{name:'Save',exact:true}).click();await page.getByText('Updated networking note',{exact:true}).waitFor();const updated=page.getByRole('article').filter({hasText:'Updated networking note'});await updated.getByRole('button',{name:'Delete note',exact:true}).click();await updated.getByRole('group',{name:'Confirm deletion',exact:true}).getByRole('button',{name:'Delete note',exact:true}).click();await page.getByText('Updated networking note',{exact:true}).waitFor({state:'detached'});
assert.deepEqual(browserErrors,[]);fs.writeFileSync('docs/evidence/contact-memory/browser.json',JSON.stringify({results,createEditDelete:true,failedSaveRetainsDraft:true,pageErrors:browserErrors,providerCalls:0},null,2));console.log('Contact memory and project navigation browser checks passed: six responsive/theme cases and CRUD/error recovery.');
})().catch(async e=>{if(page){console.error((await page.locator('body').innerText()).slice(-1800));await page.screenshot({path:'docs/evidence/contact-memory/debug.png'})}console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`
new Function('require','process','__dirname',setup)(require,process,path.resolve('docs/evidence/landing'))
