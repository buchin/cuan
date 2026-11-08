import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const taskUIURL='data:text/javascript;base64,'+Buffer.from(ts.transpileModule(fs.readFileSync(new URL('../src/features/tasks/task-ui.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2020}}).outputText).toString('base64')
async function load(file){const source=fs.readFileSync(new URL(file,import.meta.url),'utf8').replace(/(['"])\.\/task-ui\1/g,JSON.stringify(taskUIURL));return import('data:text/javascript;base64,'+Buffer.from(ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2020}}).outputText).toString('base64'))}
const {workTimeline}=await load('../src/features/tasks/task-timeline.ts')
const {isBackgroundWorkUpdate,taskMessageBindings}=await load('../src/features/inbox/task-message-bindings.ts')
const task={id:'work',status:'review',activities:[],attempts:[{id:'a',status:'succeeded',attempt_number:1,started_at:'2026-11-06T00:00:00Z',finished_at:'2026-11-06T00:01:00Z'}],latest_attempt:{id:'a',status:'succeeded'}}
test('successful execution never invents accepted completion; active review alone pulses',()=>{
 assert.equal(workTimeline(task,false).events.some(event=>event.tone==='success'),false)
 assert.equal(workTimeline({...task,review_state:'queued'},false).events.slice(-1)[0].pulse,true)
 const events=workTimeline({...task,review_state:'running'},false).events
 assert.equal(events.filter(event=>event.pulse).length,1)
 assert.equal(events.slice(-1)[0].pulse,true)
 assert.equal(workTimeline({...task,status:'done'},false).events.slice(-1)[0].tone,'success')
})
test('progress retains order, file access and report disclosure; raw diagnostics stay separate',()=>{
 const activities=[{id:'later',activity_type:'agent_report',new_status:'progress',message:'Checked actual source',created_at:'2026-11-06T00:00:20Z'},{id:'files',activity_type:'work_files',message:JSON.stringify({attachments:[{id:'file',url:'/api/v1/chat/attachments/file',name:'report.txt'}]}),created_at:'2026-11-06T00:00:30Z'},{id:'early',activity_type:'task_created',created_at:'2026-11-05T00:00:00Z'},{id:'raw',activity_type:'verification_runtime',message:'Native runtime fingerprint',created_at:'2026-11-06T00:00:10Z'}]
 const {events,technical}=workTimeline({...task,activities},false)
 assert.equal(events[0].id,'early')
 assert.equal(events.find(event=>event.id==='files').files[0].id,'file')
 assert.deepEqual(technical.map(event=>event.id),['raw'])
 assert.equal(events.some(event=>event.id==='raw'),false)
})
test('background work hides neither real user decisions nor final outcome',()=>{
 const update={work_update:{kind:'progress'}}
 assert.equal(isBackgroundWorkUpdate(update),true)
 for(const key of ['approval_plan','prompt_choices','action_card','receipt_extraction']) assert.equal(isBackgroundWorkUpdate({...update,[key]:{}}),false)
 assert.equal(isBackgroundWorkUpdate({work_update:{kind:'result'}}),false)
 assert.equal(isBackgroundWorkUpdate({}),false)
 const result=taskMessageBindings([{id:'work'}],[{id:'old',author_type:'agent',content:'/tasks/work'},{id:'latest',author_type:'agent',content:'Files',work_update:{task_id:'work',kind:'files'}}],[],false)
 assert.deepEqual([...result],[['work','latest']])
})

test('native provider notes stay technical, genuine owner notes remain readable',()=>{
 const activities=[{id:'provider',activity_type:'comment',author:'runner',message:'Provider preflight selected allowed backup account',created_at:'2026-11-06T00:00:10Z'},{id:'owner',activity_type:'comment',author:'owner',message:'Keep the existing design',created_at:'2026-11-06T00:00:20Z'}]
 const result=workTimeline({...task,activities},false)
 assert.ok(result.technical.some(event=>event.id==='provider'))
 assert.ok(result.events.some(event=>event.id==='owner'&&event.body==='Keep the existing design'))
 assert.ok(!result.events.some(event=>event.id==='provider'))
})
