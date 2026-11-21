import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const srcDir = path.resolve(__dirname, '../src')

test('TaskSessionPage.tsx contains responsive Cancel session dialog with error display and busy state', () => {
  const file = fs.readFileSync(path.join(srcDir, 'features/tasks/TaskSessionPage.tsx'), 'utf-8')
  
  // Must render Cancel session dialog with title
  assert.ok(file.includes('title="Cancel session?"'), 'Dialog must have title "Cancel session?"')
  
  // Must have cancelError state and render error with role="alert"
  assert.ok(file.includes('cancelError'), 'Must define cancelError state')
  assert.ok(file.includes('role="alert"'), 'Must render error with role="alert"')
  
  // Must call tasksApi.cancel with taskId
  assert.ok(file.includes('tasksApi.cancel(taskId)'), 'Must call tasksApi.cancel(taskId)')

  // Must gracefully handle terminal status errors without trapping user in dialog
  assert.ok(file.includes("message.includes('cannot cancel an attempt with status')"), 'Must gracefully catch already terminal status')
  assert.ok(file.includes('setCancel(false)'), 'Must close modal on terminal status')
  
  // Must show visual feedback during cancellation
  assert.ok(file.includes("busy ? 'Cancelling…' : 'Cancel session'"), 'Must show visual feedback Cancelling… while busy')
})

test('TaskSidePeek.tsx Cancel session does not inadvertently restart agent and gracefully handles terminal status', () => {
  const file = fs.readFileSync(path.join(srcDir, 'features/tasks/TaskSidePeek.tsx'), 'utf-8')
  
  // Must render Cancel session dialog
  assert.ok(file.includes("open={modal === 'cancel'}"), 'Must handle cancel modal')
  assert.ok(file.includes('title="Cancel session?"'), 'Dialog must have title "Cancel session?"')
  assert.ok(file.includes('tasksApi.cancel(initial.id)'), 'Must call tasksApi.cancel')
  
  // Must gracefully catch already terminal status
  assert.ok(file.includes("message.includes('cannot cancel an attempt with status')"), 'Must catch already terminal attempt status')

  // Ensure onRunAgent is NOT called after cancel
  const cancelSection = file.slice(file.indexOf("open={modal === 'cancel'}"), file.indexOf('</ActionDialog>', file.indexOf("open={modal === 'cancel'}")))
  assert.ok(!cancelSection.includes('onRunAgent'), 'Must NOT call onRunAgent inside cancel modal')
})

test('task-ui.ts maps cancelled status to workStatus Cancelled', async () => {
  const { workStatus } = await import('../src/features/tasks/task-ui.ts')
  
  const cancelledTask = {
    id: 't-1',
    title: 'Test task',
    status: 'cancelled',
    latest_attempt: { id: 'att-1', status: 'canceled' },
  }
  assert.equal(workStatus(cancelledTask), 'Cancelled')
})
