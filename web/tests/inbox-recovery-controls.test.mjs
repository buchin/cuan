import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source = fs.readFileSync(new URL('../src/features/inbox/inbox-recovery.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
const { inboxRecoveryControls } = await import('data:text/javascript;base64,' + Buffer.from(compiled).toString('base64'))

test('unknown outcomes offer a fresh reconciliation retry without claiming capacity wait', () => {
  assert.deepEqual(inboxRecoveryControls('failed', 'check_status', true), {canRetry:true,waitingForCapacity:false})
  assert.deepEqual(inboxRecoveryControls('failed', undefined, true), {canRetry:true,waitingForCapacity:false})
})
test('safe capacity retry and automatic recovery have distinct actions', () => {
  assert.deepEqual(inboxRecoveryControls('failed', 'check_usage', true), {canRetry:true,waitingForCapacity:true})
  assert.deepEqual(inboxRecoveryControls('failed', 'recovering', true), {canRetry:false,waitingForCapacity:true})
})
test('failed and interrupted replies offer retry; active and canceled work do not', () => {
  for(const action of ['try_again','continue_analysis','check_usage','check_agent']) assert.equal(inboxRecoveryControls('failed',action,false).canRetry,true)
  assert.equal(inboxRecoveryControls('interrupted','check_status',true).canRetry,true)
  for(const state of ['running','queued','cancelled','completed']) assert.equal(inboxRecoveryControls(state,'try_again',true).canRetry,false)
})
