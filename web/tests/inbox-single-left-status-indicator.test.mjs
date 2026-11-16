import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const subPanelPath = path.resolve(__dirname, '../src/features/inbox/InboxSubPanel.tsx')
const cssPath = path.resolve(__dirname, '../src/features/inbox/chat-calm.css')
const indexCssPath = path.resolve(__dirname, '../src/index.css')

const subPanelSource = fs.readFileSync(subPanelPath, 'utf8')
const cssSource = fs.readFileSync(cssPath, 'utf8')
const indexCssSource = fs.readFileSync(indexCssPath, 'utf8')

const compile = async (fileUrl) => {
  const source = fs.readFileSync(fileUrl, 'utf8')
  return import('data:text/javascript;base64,' + Buffer.from(ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext }
  }).outputText).toString('base64'))
}

const { taskProgressModel } = await compile(new URL('../src/components/ui/task-progress-model.ts', import.meta.url))

test('DoD 1: Indikator status percakapan dan task dilebur menjadi satu indikator tunggal di sisi kiri item Inbox', () => {
  // Indicator is on the left of the item row before avatar
  assert.ok(
    subPanelSource.includes('data-testid="workflow-status-indicator"'),
    'InboxSubPanel must contain single left workflow status indicator'
  )
  assert.ok(
    subPanelSource.includes('workflow-status-toggle shrink-0 mt-2 p-0.5 rounded-full'),
    'Workflow status indicator must be positioned at the left of the item'
  )
})

test('DoD 2: Percakapan biasa tanpa task menampilkan ring terbuka (○) saat aktif dan centang (✓) saat resolved', () => {
  // Unresolved without tasks renders Circle (○)
  assert.ok(
    subPanelSource.includes('<Circle className="h-4 w-4" strokeWidth={1.5} />'),
    'Active conversation without tasks must render open ring Circle with strokeWidth 1.5'
  )
  // Resolved renders CheckCircle2 (✓)
  assert.ok(
    subPanelSource.includes('<CheckCircle2 className="h-4 w-4" strokeWidth={2} />'),
    'Resolved conversation must render CheckCircle2 with strokeWidth 2'
  )
  assert.ok(
    subPanelSource.includes('isComplete ?'),
    'Workflow status indicator transitions between resolved CheckCircle2 and active states'
  )
})

test('DoD 3: Percakapan dengan task menampilkan pie progress proporsional dan centang (✓) saat semua task selesai', () => {
  // Multi-tasks pie progress
  assert.ok(
    subPanelSource.includes('data-testid="task-pie-progress"'),
    'Conversation with tasks must render task-pie-progress'
  )
  assert.ok(
    subPanelSource.includes('fraction > 0 && (') &&
      subPanelSource.includes('M 8 8 L 8 3.5 A 4.5 4.5 0'),
    'Pie progress SVG must render proportional pie sector arc for done tasks fraction'
  )
  assert.ok(
    subPanelSource.includes('allTasksDone = hasTasks && (progModel ? progModel.complete : doneTasks >= totalTasks)'),
    'InboxSubPanel detects when all tasks are complete'
  )
  assert.ok(
    subPanelSource.includes('isComplete = isResolved || allTasksDone'),
    'When all tasks are done, indicator transitions to complete'
  )
})

test('DoD 4: Animasi pulsating lama di sisi kanan dihilangkan secara tuntas; sisi kanan bersih khusus untuk timestamp', () => {
  // No TaskProgress in the right slot
  assert.ok(
    !subPanelSource.includes('absolute right-0 top-0 z-10'),
    'Right slot TaskProgress button is completely eliminated'
  )
  assert.ok(
    !subPanelSource.includes('<TaskProgress'),
    'InboxSubPanel does not render TaskProgress component in right slot'
  )
  assert.ok(
    !subPanelSource.includes('pr-9'),
    'No artificial right padding pr-9 on sender row'
  )
  // Right side has timestamp and micro-dot only
  assert.ok(
    subPanelSource.includes('formatConversationTime(contact.last_message_at || contact.updated_at, language)'),
    'Right side contains conversation timestamp'
  )
  assert.ok(
    subPanelSource.includes('data-testid="unread-micro-dot"'),
    'Right side contains unread micro-dot for unread messages'
  )
})

test('DoD 5: Saat task berstatus working, indikator kiri beranimasi subtle breathing pulse yang halus dan tidak mencolok', () => {
  // Checks task-pulse-subtle conditional class when isWorking is true
  assert.ok(
    subPanelSource.includes('isWorking && "task-pulse-subtle"'),
    'Left pie indicator applies task-pulse-subtle when task is working'
  )
  // Check CSS definition of task-subtle-pulse keyframes and duration ~2s
  assert.ok(
    cssSource.includes('@keyframes task-subtle-pulse'),
    'chat-calm.css defines @keyframes task-subtle-pulse'
  )
  assert.ok(
    cssSource.includes('.task-pulse-subtle {') &&
      cssSource.includes('animation: task-subtle-pulse 2'),
    'task-pulse-subtle uses slow breathing duration (~2s)'
  )
  assert.ok(
    indexCssSource.includes('@keyframes task-subtle-pulse'),
    'index.css defines @keyframes task-subtle-pulse'
  )
})

test('DoD 6: Begitu task selesai, pulsasi berhenti dan bertransisi menjadi pie penuh atau centang (✓) statis', () => {
  // When all tasks are done, isComplete is true -> renders CheckCircle2 without task-pulse-subtle
  assert.ok(
    subPanelSource.includes('isComplete ? (') &&
      subPanelSource.includes('<CheckCircle2 className="h-4 w-4" strokeWidth={2} />'),
    'Completed state renders CheckCircle2 statis without pulsating'
  )
})
