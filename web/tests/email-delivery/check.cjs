// Exact built Inbox UI, isolated fixture API; never sends email or chat.
const fs = require('fs')
const path = require('path')
let setup = fs.readFileSync('docs/evidence/landing/browser-check.cjs', 'utf8').split(' const checks=[];')[0]
setup = setup.replace("const out = 'docs/evidence/landing'", "const out = '/home/dev/artifacts/cuan-email-icon'")
setup = setup.replaceAll('4181', '4187')
setup = setup.replace("    if(p==='/api/v1/profile')", "    if(p==='/api/v1/item-preferences')return json({items:{}});\n    if(p==='/api/v1/profile')")
setup = setup.replace('const h=historyBase(id);', `const h=historyBase(id);h.requests=[];h.tasks=[];h.messages=[
 {id:1,uuid:'incoming',author_type:'user',author_name:'Pelanggan fixture',sender_type:'visitor',content:'Halo, akun saya belum bisa login.',created_at:now,is_internal:false,email_delivery:{status:'delivered',recipient_type:'operator'}},
 {id:2,uuid:'delivered',author_type:'user',author_name:'Owner',author_id:'operator_user',sender_type:'operator',content:'Halo sob, akunnya sudah aktif. Silakan dicoba login kembali ya.',created_at:now,is_internal:false,email_delivery:{status:'delivered',recipient_type:'visitor'}},
 {id:3,uuid:'failed',author_type:'user',author_name:'Owner',author_id:'operator_user',sender_type:'operator',content:'Balasan lain yang membutuhkan perhatian.',created_at:now,is_internal:false,email_delivery:{status:'failed',recipient_type:'visitor'}},
 {id:4,uuid:'private',author_type:'agent',sender_type:'operator',content:'Catatan internal tetap privat.',created_at:now,is_internal:true,email_delivery:{status:'delivered',recipient_type:'visitor'}}
];return h;`)
setup += `
contacts[0]={...contacts[0],type:'customer',name:'Pelanggan fixture',email:'fixture@example.invalid',last_message:'Silakan dicoba login kembali ya.'};
const checks=[];
for(const width of [390,1440])for(const theme of ['light','dark']){
 await page.setViewportSize({width,height:900});
 await page.goto(base+'/inbox/room0');
 await page.evaluate(t=>{localStorage.setItem('cuan-theme',t);localStorage.setItem('cuan-language','id')},theme);
 await page.reload();
 await page.locator('.lucide-mail-check').first().waitFor();
 assert.equal(await page.locator('.wa-message-meta .lucide-mail-check').count(),2,'One mail icon per delivered public message; none for whisper');
 const indicators=page.locator('.wa-message-meta span[role="img"][aria-label^="Email"]');
 assert.equal(await indicators.count(),2);
 for(const indicator of await indicators.all()){
  assert.equal((await indicator.innerText()).trim(),'','Delivered must not have visible status words');
  assert.match(await indicator.getAttribute('title'),/Email diterima/);
  const size=await indicator.locator('svg').boundingBox();assert.equal(size.width,14);assert.equal(size.height,14);
 }
 assert.equal(await page.locator('.wa-message-meta').filter({hasText:'Email gagal'}).count(),1,'Failure must retain useful text');
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.screenshot({path:out+'/'+theme+'-'+width+'.png'});
 checks.push({width,theme,deliveredIconOnly:true,accessibleRecipientContext:true,failureTextVisible:true,noOverflow:true});
}
assert.equal(composerSends.length,0);assert.equal(launches.length,0);assert.deepEqual(errors,[]);
fs.writeFileSync(out+'/checks.json',JSON.stringify(checks,null,2));console.log('PASS actual Inbox: mail-check delivered, text failure, private suppression,390/1440px light/dark');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();if(server)await new Promise(r=>server.close(r))});
`
new Function('require', 'process', '__dirname', setup)(require, process, path.resolve('docs/evidence/landing'))
