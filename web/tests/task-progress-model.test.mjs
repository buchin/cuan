import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const compile=async path=>{const source=fs.readFileSync(new URL(path,import.meta.url),'utf8');return import('data:text/javascript;base64,'+Buffer.from(ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText).toString('base64'))}
const {taskProgressModel}=await compile('../src/components/ui/task-progress-model.ts')
const {workStatus}=await compile('../src/features/tasks/task-ui.ts')
const {workConversationPreview}=await compile('../src/features/inbox/conversation-work-preview.ts')

test('hide unknown, zero-progress, empty and single-task Inbox indicators',()=>{
 for(const input of [{},{total:0,done:0},{total:3,done:0},{total:1,done:1,minTasks:2},{total:2,done:3},{total:3,done:NaN}])assert.equal(taskProgressModel(input),null)
 assert.equal(taskProgressModel({total:1,done:1}).complete,true)
})
test('cancellation never masquerades as all original work accepted',()=>{
 const clean=taskProgressModel({total:2,done:2});assert.equal(clean.complete,true);assert.equal(clean.mixed,false)
 const mixed=taskProgressModel({total:2,done:2,canceled:2,minTasks:2});assert.equal(mixed.mixed,true);assert.deepEqual(mixed.rows,[{label:'Completed',count:2},{label:'Canceled',count:2}])
 const partial=taskProgressModel({total:5002,done:1001,canceled:2000,active:1000,queued:1000,review:1001,blocked:1000});assert.equal(partial.complete,false);assert.equal(partial.fraction,1001/5002);assert.equal(partial.rows.find(r=>r.label==='In review').count,1001)
 assert.equal(taskProgressModel({total:0,done:0,canceled:3}),null)
 assert.equal(taskProgressModel({total:1000,done:999}).complete,false)
})
test('unknown breakdown stays Remaining instead of inventing task states',()=>{
 assert.deepEqual(taskProgressModel({total:5,done:2}).rows,[{label:'Completed',count:2},{label:'Remaining',count:3}])
})
test('concise review previews respect the current saved state',()=>{
 const passed='### Evaluasi Kriteria Selesai (Definition of Done)\n**Status:** LULUS\nBukti: dokumen tersedia.'
 const failed='### Evaluasi Kriteria Selesai (Definition of Done)\n**Status:** BELUM LULUS (Perlu revisi)'
 assert.equal(workConversationPreview(passed,{task_count:1,done_tasks:1}),'Work completed and verified.')
 assert.equal(workConversationPreview(passed,{task_count:1,review_tasks:1}),'Result ready for review.')
 assert.equal(workConversationPreview(failed,{task_count:1,failed_tasks:1}),'The result needs attention.')
 assert.equal(workConversationPreview(passed,{task_count:1,canceled_tasks:1}),'Work canceled. Saved work is still available.')
 assert.equal(workConversationPreview('Pekerjaan dibatalkan. Hasil belum diverifikasi.',{task_count:2,canceled_tasks:1,review_tasks:1}),'A task was canceled; other results are in review.')
 assert.equal(workConversationPreview(failed,{task_count:1,done_tasks:1}),'Work completed and verified.')
 assert.equal(workConversationPreview(passed),'Review passed. Open the conversation for details.')
 for(const message of ['Bisa buat laporan untuk pelanggan?','{"summary":"Rencana disepakati"}','Apa itu Definition of Done?','Evaluasi Kriteria Selesai itu maksudnya apa?'])assert.equal(workConversationPreview(message),undefined)
})

test('canceled execution saved in review is consistent with its task card',()=>{
 assert.equal(workStatus({status:'review',latest_attempt:{status:'canceled'},launch:{state:'stopped'}}),'Cancelled')
 assert.equal(workStatus({status:'review',latest_attempt:{status:'succeeded'}}),'Ready for review')
 assert.equal(workStatus({status:'review',latest_attempt:{status:'canceled'},launch:{state:'queued'}}),'Queued')
 assert.equal(workStatus({status:'done',latest_attempt:{status:'canceled'}}),'Completed')
 const report='### Evaluasi Kriteria Selesai (Definition of Done)\n**Status:** PERLU DIREVIU'
 assert.equal(workConversationPreview(report,{task_count:1,review_tasks:1}),'Result ready for review.')
})
