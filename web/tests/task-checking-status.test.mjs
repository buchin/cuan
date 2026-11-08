import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'

const source = fs.readFileSync(new URL('../src/features/tasks/task-ui.ts', import.meta.url), 'utf8')
const js = ts.transpileModule(source, {compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText
const {workStatus, workDisplayStatus, taskAgentPending} = await import('data:text/javascript;base64,' + Buffer.from(js).toString('base64'))

test('Checking labels follow the saved execution projection without changing work semantics', () => {
  const task = {status:'doing',latest_attempt:{status:'running'}}
  assert.equal(workDisplayStatus(task), 'Working')
  assert.equal(workDisplayStatus({...task,review_state:'running'}), 'Checking result')
  const queued = {...task,status:'queued',latest_attempt:{status:'queued'},review_state:'queued'}
  assert.equal(workDisplayStatus(queued), 'Waiting for checks')
  assert.equal(workStatus({...task,review_state:'running'}), 'Working')
  assert.equal(workStatus(queued), 'Queued')
  assert.equal(workDisplayStatus({...task,status:'done',review_state:'running'}), 'Completed')
  assert.equal(workDisplayStatus({...task,latest_attempt:{status:'cancelled'},review_state:'running'}), 'Cancelled')
})

test('Pulse follows durable Cuan-owned next steps while preserving phase labels', () => {
  for (const [name, task, expected] of [
    ['developer running', {status:'doing',latest_attempt:{status:'running'}}, true],
    ['developer queued', {status:'queued',latest_attempt:{status:'queued'}}, true],
    ['waiting checks', {status:'review',review_state:'queued'}, true],
    ['checking', {status:'review',review_state:'running'}, true],
    ['waiting release', {status:'queued',latest_attempt:{status:'succeeded'},launch:{state:'queued'}}, true],
    ['admitted recovery', {status:'blocked',latest_attempt:{status:'failed'},launch:{state:'claimed'}}, true],
    ['reviewing failure', {status:'blocked',latest_attempt:{status:'failed'},review_state:'queued'}, true],
    ['needs owner', {status:'review',latest_attempt:{status:'succeeded'}}, false],
    ['unresolved failure', {status:'blocked',latest_attempt:{status:'failed'}}, false],
    ['stopped launch', {status:'queued',launch:{state:'stopped'}}, false],
    ['unrecorded board queue', {status:'queued'}, false],
    ['unrecorded doing', {status:'doing'}, false],
    ['completed overrides stale execution', {status:'done',latest_attempt:{status:'running'},review_state:'queued',launch:{state:'queued'}}, false],
    ['cancelled overrides stale execution', {status:'cancelled',latest_attempt:{status:'running'},review_state:'queued',launch:{state:'queued'}}, false],
    ['cancel requested', {status:'doing',latest_attempt:{status:'running',cancel_requested:true}}, false],
    ['cancellation receipt', {status:'doing',latest_attempt:{status:'running',canceled_at:'2026-11-08T00:00:00Z'}}, false],
  ]) assert.equal(taskAgentPending(task), expected, name)
  assert.equal(workDisplayStatus({status:'review',review_state:'queued'}),'Waiting for checks')
  assert.equal(workStatus({status:'queued',latest_attempt:{status:'succeeded'},launch:{state:'queued'}}),'Queued')
})

test('stopped work does not imply the user owes an action',()=>{
 const paused={status:'blocked',latest_attempt:{status:'succeeded'}}
 assert.equal(workDisplayStatus(paused),'Work paused')
 assert.equal(workDisplayStatus({...paused,review_action:'continue'}),'Work paused')
 assert.equal(workDisplayStatus({...paused,review_action:'needs_owner'}),'Needs your input')
 assert.equal(workDisplayStatus({...paused,review_action:'wait'}),'Waiting')
 assert.equal(workDisplayStatus({...paused,review_action:'needs_owner',review_state:'running'}),'Checking result')
 assert.equal(taskAgentPending({...paused,review_action:'continue'}),false)
})
