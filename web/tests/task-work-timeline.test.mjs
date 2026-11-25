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


test('release phase stays consistent in the work detail timeline',()=>{
 const queued={...task,source_room_id:'room',status:'queued',review_state:'queued',launch:{state:'queued',kind:'native_release'}}
 assert.equal(workTimeline(queued,false).events.at(-1).title,'Waiting to deploy')
 assert.equal(workTimeline(queued,true).events.at(-1).title,'Menunggu rilis')
 assert.equal(workTimeline(queued,false).events.at(-1).pulse,true)
 assert.equal(workTimeline({...queued,status:'doing',latest_attempt:{status:'running'},launch:{state:'started',kind:'native_release'}},false).events.at(-1).title,'Deploying')
})

test('completed task keeps permanent Pengerjaan, Verifikasi, and Push & Rilis Live items and cleans stale gap notes',async()=>{
 const {sanitizeCompletedTaskNote,taskConversationSummary}=await import(taskUIURL)
 const doneTask={
  id:'done-live',
  status:'done',
  title:'Perbaiki pembacaan lampiran',
  commit_sha:'9c4d3f7466565284dbfa786e6a8c46ac93cab45c',
  summary:'Pembacaan lampiran gambar di chat sudah diperbaiki.',
  complete_note:'Independent verification of commit 9c4d3f7466565284dbfa786e6a8c46ac93cab45c (read-only attempt): ran tests which passed in 2.716s (saved in session:targeted_verification_tests.txt). Remaining gap: changes are committed locally at 9c4d3f7466565284dbfa786e6a8c46ac93cab45c and not yet pushed/merged to branch dev.\n\n🚀 Perubahan telah diverifikasi aktif pada rilis live (9c4d3f74).',
  attempts:[
   {id:'att-exec',kind:'execution',status:'succeeded',attempt_number:1,started_at:'2026-11-06T00:00:00Z',finished_at:'2026-11-06T00:05:00Z'},
   {id:'att-verify',kind:'verification',status:'succeeded',attempt_number:2,started_at:'2026-11-06T00:06:00Z',finished_at:'2026-11-06T00:08:00Z'},
   {id:'att-release',kind:'native_release',status:'succeeded',attempt_number:3,started_at:'2026-11-06T00:09:00Z',finished_at:'2026-11-06T00:11:00Z'},
  ],
  activities:[
   {id:'rv',attempt_id:'att-verify',activity_type:'runner_verification',author:'runner',message:'go test ./... ok',created_at:'2026-11-06T00:07:00Z'},
   {id:'dep',attempt_id:'att-release',activity_type:'deployment_verified',author:'cuan-deploy',message:JSON.stringify({commit:'9c4d3f7466565284dbfa786e6a8c46ac93cab45c'}),created_at:'2026-11-06T00:10:30Z'},
  ],
 }
 const tlId=workTimeline(doneTask,true)
 const titlesId=tlId.events.map(e=>e.title)
 assert.ok(titlesId.includes('Pengerjaan dimulai'))
 assert.ok(titlesId.includes('Pengerjaan selesai'))
 assert.ok(titlesId.includes('Verifikasi dimulai'))
 assert.ok(titlesId.includes('Verifikasi selesai'))
 assert.ok(titlesId.includes('Push & rilis live dimulai'))
 assert.ok(titlesId.includes('Push & rilis live selesai'))
 assert.ok(titlesId.includes('Pekerjaan selesai'))
 assert.equal(tlId.events.find(e=>e.id==='att-release:finish').body,'Rilis aktif pada commit 9c4d3f74.')
 assert.ok(tlId.technical.some(a=>a.id==='rv'))
 assert.ok(tlId.technical.some(a=>a.id==='dep'))

 const tlEn=workTimeline(doneTask,false)
 const titlesEn=tlEn.events.map(e=>e.title)
 assert.ok(titlesEn.includes('Verification finished'))
 assert.ok(titlesEn.includes('Push & live release finished'))

 const cleanedNote=sanitizeCompletedTaskNote(doneTask.complete_note,true)
 assert.ok(!cleanedNote.includes('not yet pushed/merged'))
 assert.ok(cleanedNote.includes('🚀 Perubahan telah diverifikasi aktif pada rilis live (9c4d3f74).'))

 const chatSummary=taskConversationSummary(doneTask)
 assert.ok(!chatSummary.includes('not yet pushed/merged'))
 assert.ok(chatSummary.includes('Pembacaan lampiran gambar di chat sudah diperbaiki.'))
 assert.ok(chatSummary.includes('🚀 Perubahan telah diverifikasi aktif pada rilis live (9c4d3f74).'))
})
