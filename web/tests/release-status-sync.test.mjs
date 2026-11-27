import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'

const taskUiSource = fs.readFileSync(new URL('../src/features/tasks/task-ui.ts', import.meta.url), 'utf8')
const statusDotStub = 'export const StatusDot = () => null'
const statusDotUrl = 'data:text/javascript;base64,' + Buffer.from(statusDotStub).toString('base64')
const typesUrl = 'data:text/javascript;base64,' + Buffer.from('').toString('base64')

const transpiledTaskUi = ts.transpileModule(
  taskUiSource
    .replace("import type { Task, TaskAttempt, TaskDetail, TaskStatus } from '../../api/types'", `import '${typesUrl}'`)
    .replace("import type { StatusDotVariant } from '../../components/ui/StatusDot'", `import '${statusDotUrl}'`),
  { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } },
).outputText
const { workStatus, workDisplayStatus, taskExecutionActive, getTaskStateKey } = await import(
  'data:text/javascript;base64,' + Buffer.from(transpiledTaskUi).toString('base64')
)

const bindingsSource = fs.readFileSync(new URL('../src/features/inbox/task-message-bindings.ts', import.meta.url), 'utf8')
const transpiledBindings = ts.transpileModule(bindingsSource, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText
const { isStaleReleaseNoticeSuperseded } = await import(
  'data:text/javascript;base64,' + Buffer.from(transpiledBindings).toString('base64')
)

test('workStatus & top-bar state never show Completed/done while release is failed, deploying, or pending', () => {
  const base = { id: 't1', title: 'Test', status: 'done', priority: 'normal', created_at: '', updated_at: '' }

  assert.equal(workStatus({ ...base, release_status: 'failed' }), 'Needs attention')
  assert.equal(workDisplayStatus({ ...base, release_status: 'failed' }), 'Unfinished')
  assert.equal(getTaskStateKey({ ...base, release_status: 'failed' }), 'attention')
  assert.equal(taskExecutionActive({ ...base, release_status: 'failed' }), false)

  assert.equal(workStatus({ ...base, release_status: 'deploying' }), 'Working')
  assert.equal(workDisplayStatus({ ...base, release_status: 'deploying' }), 'Deploying')
  assert.equal(getTaskStateKey({ ...base, release_status: 'deploying' }), 'working')
  assert.equal(taskExecutionActive({ ...base, release_status: 'deploying' }), true)

  assert.equal(workStatus({ ...base, release_status: 'pending' }), 'Queued')
  assert.equal(workDisplayStatus({ ...base, release_status: 'pending' }), 'Waiting to deploy')
  assert.equal(getTaskStateKey({ ...base, release_status: 'pending' }), 'queued')
  assert.equal(taskExecutionActive({ ...base, release_status: 'pending' }), false)

  assert.equal(workStatus({ ...base, release_status: 'live' }), 'Completed')
  assert.equal(workDisplayStatus({ ...base, release_status: 'live' }), 'Completed')
  assert.equal(getTaskStateKey({ ...base, release_status: 'live' }), 'done')
})

test('isStaleReleaseNoticeSuperseded keeps failure visible until task is done+live with completion message', () => {
  const failText = 'Indikator berdenyut sudah benar, tetapi pembaruan ini belum masuk ke aplikasi yang sedang berjalan.\n\n[View work](/tasks/t1)'
  const doneText = 'Pekerjaan **Test** telah selesai dan diverifikasi.\n\n[Lihat rincian pekerjaan](/tasks/t1)'

  // Still failed: keep failure visible even if other messages exist
  assert.equal(
    isStaleReleaseNoticeSuperseded(failText, ['t1'], [{ id: 't1', status: 'done', release_status: 'failed' }], [{ id: 'm1', content: failText }, { id: 'm2', content: doneText }], 0),
    false,
  )

  // Done + live, but no completion message yet: keep visible so user isn't left with empty context
  assert.equal(
    isStaleReleaseNoticeSuperseded(failText, ['t1'], [{ id: 't1', status: 'done', release_status: 'live' }], [{ id: 'm1', content: failText }], 0),
    false,
  )

  // Done + live AND completion message arrived: suppress stale hanging failure
  assert.equal(
    isStaleReleaseNoticeSuperseded(failText, ['t1'], [{ id: 't1', status: 'done', release_status: 'live' }], [{ id: 'm1', content: failText }, { id: 'm2', content: doneText }], 0),
    true,
  )
})
