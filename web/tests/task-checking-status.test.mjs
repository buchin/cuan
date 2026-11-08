import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'

const source = fs.readFileSync(new URL('../src/features/tasks/task-ui.ts', import.meta.url), 'utf8')
const js = ts.transpileModule(source, {compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText
const {workStatus, workDisplayStatus} = await import('data:text/javascript;base64,' + Buffer.from(js).toString('base64'))

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
