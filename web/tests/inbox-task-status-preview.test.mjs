import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const workPreviewSource = fs.readFileSync(path.resolve(__dirname, '../src/features/inbox/conversation-work-preview.ts'), 'utf8')
const workPreviewTranspiled = ts.transpileModule(workPreviewSource, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText
const workPreviewUrl = 'data:text/javascript;base64,' + Buffer.from(workPreviewTranspiled).toString('base64')

const { workConversationPreview } = await import(workPreviewUrl)

const substageSource = fs.readFileSync(path.resolve(__dirname, '../src/features/inbox/task-substage.ts'), 'utf8')
const substageTranspiled = ts.transpileModule(substageSource, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText
const substageUrl = 'data:text/javascript;base64,' + Buffer.from(substageTranspiled).toString('base64')

const { getRoomActiveSubstage, setRoomActiveSubstage } = await import(substageUrl)

const subPanelSource = fs.readFileSync(path.resolve(__dirname, '../src/features/inbox/InboxSubPanel.tsx'), 'utf8')
const fnMatch = subPanelSource.match(/export function conversationPreview[\s\S]*?\n\}/)
if (!fnMatch) throw new Error('conversationPreview not found in InboxSubPanel.tsx')

const previewTranspiled = ts.transpileModule(
  `import { workConversationPreview } from '${workPreviewUrl}';\nimport { getRoomActiveSubstage, setRoomActiveSubstage } from '${substageUrl}';\n${fnMatch[0]}`,
  { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } }
).outputText
const previewUrl = 'data:text/javascript;base64,' + Buffer.from(previewTranspiled).toString('base64')
const { conversationPreview } = await import(previewUrl)

const idDict = JSON.parse(fs.readFileSync(path.resolve(__dirname, '../src/locales/id.json'), 'utf8'))
const enDict = JSON.parse(fs.readFileSync(path.resolve(__dirname, '../src/locales/en.json'), 'utf8'))
const tId = (k) => idDict[k] || k
const tEn = (k) => enDict[k] || k

test('1. conversationPreview prioritizes active running tasks as "Sedang dikerjakan"', () => {
  const queuedBoilerplate = 'The work is queued waiting for the active project task to complete. I’ll start it automatically and report progress here. [View work](/tasks/msg_123)'
  const runningContact = {
    id: 'room-1',
    conversation_id: 'room-1',
    name: 'Pindahkan Tombol New Work',
    task_count: 1,
    active_tasks: 1,
    queued_tasks: 0,
    last_message: queuedBoilerplate,
  }

  // Indonesian locale (default)
  assert.equal(
    conversationPreview(queuedBoilerplate, runningContact, tId),
    'Sedang dikerjakan',
    'Active task must dynamically display "Sedang dikerjakan" instead of stale queued notice'
  )

  // Untranslated fallback
  assert.equal(
    conversationPreview(queuedBoilerplate, runningContact),
    'Sedang dikerjakan',
    'Default t parameter must return "Sedang dikerjakan"'
  )

  // English locale
  assert.equal(
    conversationPreview(queuedBoilerplate, runningContact, tEn),
    'Working',
    'English locale must return "Working"'
  )
})

test('2. conversationPreview prioritizes queued tasks as "Menunggu antrean"', () => {
  const queuedBoilerplate = 'The work is queued waiting for the active project task to complete. I’ll start it automatically and report progress here. [View work](/tasks/msg_123)'
  const queuedContact = {
    id: 'room-2',
    conversation_id: 'room-2',
    name: 'Rotasi Dinamis Kata Status',
    task_count: 1,
    active_tasks: 0,
    queued_tasks: 1,
    last_message: queuedBoilerplate,
  }

  // Indonesian locale
  assert.equal(
    conversationPreview(queuedBoilerplate, queuedContact, tId),
    'Menunggu antrean',
    'Queued task must dynamically display "Menunggu antrean"'
  )

  // English locale
  assert.equal(
    conversationPreview(queuedBoilerplate, queuedContact, tEn),
    'Waiting in queue',
    'English locale must return "Waiting in queue"'
  )
})

test('3. conversationPreview falls back to last_message when tasks are completed or inactive', () => {
  const completedContact = {
    id: 'room-3',
    conversation_id: 'room-3',
    name: 'Website pelanggan',
    task_count: 1,
    done_tasks: 1,
    active_tasks: 0,
    queued_tasks: 0,
    last_message: 'Memeriksa tampilan mobile setelah perbaikan.',
  }

  assert.equal(
    conversationPreview(completedContact.last_message, completedContact, tId),
    'Memeriksa tampilan mobile setelah perbaikan.',
    'Completed task conversation must fall back to last_message snippet'
  )

  const idleContact = {
    id: 'room-4',
    conversation_id: 'room-4',
    name: 'fahmi putra',
    task_count: 0,
    active_tasks: 0,
    queued_tasks: 0,
    last_message: 'Biar kami aktifkan',
  }

  assert.equal(
    conversationPreview(idleContact.last_message, idleContact, tId),
    'Biar kami aktifkan',
    'Taskless contact must fall back to last_message'
  )
})

test('4. workConversationPreview recognizes queued launch notices with task link', () => {
  const queuedLinkMsg = 'The work is queued waiting for the active project task to complete. I’ll start it automatically and report progress here. [View work](/tasks/msg_123)'
  
  assert.equal(
    workConversationPreview(queuedLinkMsg, { task_count: 1, active_tasks: 1 }),
    'Work is in progress.'
  )
  assert.equal(
    workConversationPreview(queuedLinkMsg, { task_count: 1, queued_tasks: 1 }),
    'Work is waiting to start.'
  )
  assert.equal(
    workConversationPreview(queuedLinkMsg, { task_count: 1, done_tasks: 1 }),
    'Work completed and verified.'
  )
})

test('5. Locales contain complete consistent translations for status preview', () => {
  assert.equal(idDict['Sedang dikerjakan'], 'Sedang dikerjakan')
  assert.equal(idDict['Menunggu antrean'], 'Menunggu antrean')
  assert.equal(idDict['Work is in progress.'], 'Sedang dikerjakan')
  assert.equal(idDict['Work is waiting to start.'], 'Menunggu antrean')

  assert.equal(enDict['Sedang dikerjakan'], 'Working')
  assert.equal(enDict['Menunggu antrean'], 'Waiting in queue')
})

test('6. conversationPreview displays active sub-stage matching task card (Memeriksa hasil)', () => {
  const activeCheckingContact = {
    id: 'room-checking',
    conversation_id: 'room-checking',
    name: 'Ganti Ring Avatar dengan Pulsating Dot',
    task_count: 1,
    active_tasks: 1,
    queued_tasks: 0,
    active_substage: 'Checking result',
    last_message: 'Pekerjaan sedang berjalan',
  }
  assert.equal(
    conversationPreview(activeCheckingContact.last_message, activeCheckingContact, tId),
    'Memeriksa hasil',
    'Active task with "Checking result" substage must display "Memeriksa hasil"'
  )
  assert.equal(
    conversationPreview(activeCheckingContact.last_message, activeCheckingContact, tEn),
    'Checking result',
    'Active task with "Checking result" substage must display "Checking result"'
  )
})

test('7. conversationPreview displays "Siap ditinjau" for review tasks and prevents "Files from this work session" leak', () => {
  const reviewContact = {
    id: 'room-review',
    conversation_id: 'room-review',
    name: 'Optimasi Storage',
    task_count: 1,
    active_tasks: 0,
    review_tasks: 1,
    last_message: 'Files from this work session',
  }
  assert.equal(
    conversationPreview(reviewContact.last_message, reviewContact, tId),
    'Siap ditinjau',
    'Review task must display "Siap ditinjau" instead of system files notice'
  )
  assert.equal(
    conversationPreview(reviewContact.last_message, reviewContact, tEn),
    'Ready for review',
    'Review task must display "Ready for review" in English'
  )
})

test('8. conversationPreview reflects reactive room active substage via setRoomActiveSubstage', () => {
  const reactiveContact = {
    id: 'room-reactive-1',
    conversation_id: 'room-reactive-1',
    name: 'Sinkronisasi Task',
    task_count: 1,
    active_tasks: 1,
    last_message: 'Pekerjaan sedang berlangsung',
  }
  assert.equal(conversationPreview(reactiveContact.last_message, reactiveContact, tId), 'Sedang dikerjakan')

  setRoomActiveSubstage('room-reactive-1', 'Checking result')
  assert.equal(conversationPreview(reactiveContact.last_message, reactiveContact, tId), 'Memeriksa hasil')

  setRoomActiveSubstage('room-reactive-1', undefined)
  assert.equal(conversationPreview(reactiveContact.last_message, reactiveContact, tId), 'Sedang dikerjakan')
})

test('9. conversationPreview preserves "Menunggu antrean" for Indonesian queued notices even during count lag', () => {
  const idQueuedMsg = 'Pekerjaan masuk dalam antrean menunggu giliran. [View work](/tasks/msg_456)'
  const lagContact = {
    id: 'room-lag',
    conversation_id: 'room-lag',
    name: 'Pindahkan Detail Task ke SlideDrawer',
    task_count: 1,
    active_tasks: 0,
    queued_tasks: 0,
    done_tasks: 0,
    last_message: idQueuedMsg,
  }
  assert.equal(
    conversationPreview(idQueuedMsg, lagContact, tId),
    'Menunggu antrean',
    'Indonesian queued notice must display "Menunggu antrean" even if queued_tasks has transiently dropped'
  )
})

test('10. conversationPreview normalizes "Queued" substage to "Menunggu antrean"', () => {
  const queuedSubstageContact = {
    id: 'room-substage-queued',
    conversation_id: 'room-substage-queued',
    name: 'Rapikan Task Bar',
    task_count: 1,
    active_tasks: 0,
    queued_tasks: 1,
    active_substage: 'Queued',
    last_message: 'Pekerjaan masuk dalam antrean',
  }
  assert.equal(
    conversationPreview(queuedSubstageContact.last_message, queuedSubstageContact, tId),
    'Menunggu antrean',
    '"Queued" substage must be localized to "Menunggu antrean"'
  )
})
