import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const subPanelPath = path.resolve(__dirname, '../src/features/inbox/InboxSubPanel.tsx')
const taskProgressPath = path.resolve(__dirname, '../src/components/ui/TaskProgress.tsx')
const taskProgressModelPath = path.resolve(__dirname, '../src/components/ui/task-progress-model.ts')
const cssPath = path.resolve(__dirname, '../src/features/inbox/chat-calm.css')

const subPanelSource = fs.readFileSync(subPanelPath, 'utf8')
const taskProgressSource = fs.readFileSync(taskProgressPath, 'utf8')
const taskProgressModelSource = fs.readFileSync(taskProgressModelPath, 'utf8')
const cssSource = fs.readFileSync(cssPath, 'utf8')

const compile = async (fileUrl) => {
  const source = fs.readFileSync(fileUrl, 'utf8')
  return import('data:text/javascript;base64,' + Buffer.from(ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext }
  }).outputText).toString('base64'))
}

const { taskProgressModel } = await compile(new URL('../src/components/ui/task-progress-model.ts', import.meta.url))

test('Criterion 1: Hapus badge/teks status task (seperti ● 1 bekerja) di bawah snippet pesan', () => {
  // Confirm removal of old badges under message snippet
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-active"'),
    'InboxSubPanel must not render task-badge-active under snippet'
  )
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-review"'),
    'InboxSubPanel must not render task-badge-review under snippet'
  )
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-attention"'),
    'InboxSubPanel must not render task-badge-attention under snippet'
  )
  assert.ok(
    !subPanelSource.includes("{activeTasks} {t('bekerja')}"),
    'InboxSubPanel must not render {activeTasks} bekerja text under snippet'
  )
})

test('Criterion 2: Task Tunggal Berjalan (outline circle with subtle pulse)', () => {
  const singleRunning = taskProgressModel({ total: 1, done: 0, active: 1 })
  assert.ok(singleRunning !== null, 'Single running task must produce valid model')
  assert.equal(singleRunning.total, 1)
  assert.equal(singleRunning.done, 0)
  assert.equal(singleRunning.active, 1)
  assert.equal(singleRunning.fraction, 0)
  assert.equal(singleRunning.complete, false)

  // Verify TaskProgress component handles isSingle with outline circle & pulse
  assert.ok(
    taskProgressSource.includes('isSingleActive') || taskProgressSource.includes('isSingle'),
    'TaskProgress must handle single active task state'
  )
  assert.ok(
    taskProgressSource.includes('<circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" strokeWidth="1.5"'),
    'TaskProgress must render outline circle with r=6.5, fill=none, stroke=currentColor'
  )
  assert.ok(
    taskProgressSource.includes('task-pulse-subtle'),
    'TaskProgress must apply task-pulse-subtle animation when pulsing'
  )
})

test('Criterion 3: Beberapa Task (Ada yang Aktif) (Pie progress SVG showing proportion done with subtle pulse)', () => {
  const multiRunning = taskProgressModel({ total: 3, done: 1, active: 1 })
  assert.ok(multiRunning !== null, 'Multiple tasks with 1 done must produce model')
  assert.equal(multiRunning.total, 3)
  assert.equal(multiRunning.done, 1)
  assert.equal(multiRunning.active, 1)
  assert.equal(multiRunning.fraction, 1 / 3)
  assert.equal(multiRunning.complete, false)

  // Verify pie SVG path rendering
  assert.ok(
    taskProgressSource.includes('model.fraction>0') || taskProgressSource.includes('model.fraction > 0'),
    'TaskProgress must render pie slice path when fraction > 0'
  )
  assert.ok(
    taskProgressSource.includes('path d={`M 8 8 L 8 3.5 A 4.5 4.5 0'),
    'TaskProgress must render pie sector arc'
  )
})

test('Criterion 4: Semua Task Selesai (Static green checkmark CheckCircle2)', () => {
  const allDoneSingle = taskProgressModel({ total: 1, done: 1 })
  assert.ok(allDoneSingle !== null)
  assert.equal(allDoneSingle.complete, true)

  const allDoneMulti = taskProgressModel({ total: 4, done: 4 })
  assert.ok(allDoneMulti !== null)
  assert.equal(allDoneMulti.complete, true)

  // Verify static CheckCircle2 render for complete tasks without pulse
  assert.ok(
    taskProgressSource.includes('<CheckCircle2 className="h-full w-full" strokeWidth={1.75}/>') ||
      taskProgressSource.includes('<CheckCircle2'),
    'TaskProgress must render CheckCircle2 for completed tasks'
  )
  assert.ok(
    taskProgressSource.includes('CheckCircle2') && taskProgressSource.includes('model.complete'),
    'Completed state must conditionally render CheckCircle2'
  )
})

test('Criterion 5: Tanpa Task (Slot remains clean without icon)', () => {
  // Empty counts
  assert.equal(taskProgressModel({}), null)
  assert.equal(taskProgressModel({ total: 0 }), null)
  assert.equal(taskProgressModel({ total: 0, done: 0 }), null)
  assert.equal(taskProgressModel({ total: 2, done: 0, active: 0 }), null)

  // Subpanel hides progress slot when model is null
  assert.ok(
    subPanelSource.includes('{showProgress && <div className="absolute right-0 top-0 z-10"><TaskProgress {...progress} /></div>}'),
    'InboxSubPanel conditionally renders TaskProgress only when showProgress is true'
  )
})

test('Criterion 6: Animasi denyut halus (Zero Cognitive Fatigue standard)', () => {
  // Check CSS definition of task-subtle-pulse
  assert.ok(
    cssSource.includes('@keyframes task-subtle-pulse'),
    'chat-calm.css must define @keyframes task-subtle-pulse'
  )
  assert.ok(
    cssSource.includes('.task-pulse-subtle'),
    'chat-calm.css must define .task-pulse-subtle class'
  )
  assert.ok(
    cssSource.includes('prefers-reduced-motion'),
    'CSS must respect prefers-reduced-motion for accessibility'
  )
})

test('Criterion 7: Alignment presisi dengan timestamp di resolusi mobile maupun desktop', () => {
  // InboxSubPanel uses absolute right-0 top-0 and pr-9 on top row for timestamp spacing
  assert.ok(
    subPanelSource.includes('showProgress && "pr-9"'),
    'InboxSubPanel applies pr-9 to sender row when progress is shown'
  )
  assert.ok(
    subPanelSource.includes('absolute right-0 top-0 z-10'),
    'InboxSubPanel positions TaskProgress at absolute right-0 top-0'
  )
  assert.ok(
    subPanelSource.includes('formatConversationTime'),
    'InboxSubPanel formats conversation time'
  )
})
