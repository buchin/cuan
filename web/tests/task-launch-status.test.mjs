import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
const src=fs.readFileSync(new URL('../src/features/tasks/task-ui.ts',import.meta.url),'utf8')
const body=src.match(/export function workStatus\(task: Task\) \{([\s\S]*?)\n\}/)[1].replaceAll(' as string','')
const status=new Function('task',body)
test('stopped launch does not pretend work is queued; active attempts and fresh queue intents take precedence',()=>{
 assert.equal(status({status:'queued',launch:{state:'stopped'},latest_attempt:{status:'failed'}}),'Needs attention')
 assert.equal(status({status:'queued',launch:{state:'stopped'},latest_attempt:{status:'running'}}),'Working')
 assert.equal(status({status:'queued',launch:{state:'queued'},latest_attempt:{status:'failed'}}),'Queued')
 assert.equal(status({status:'blocked',latest_attempt:{status:'canceled'}}),'Cancelled')
})
