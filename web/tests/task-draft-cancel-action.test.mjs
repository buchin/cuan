import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const taskItemCardPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'TaskItemCard.tsx')
const taskBoardViewPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'TaskBoardView.tsx')
const appPath = path.join(__dirname, '..', 'src', 'App.tsx')
const idLocalePath = path.join(__dirname, '..', 'src', 'locales', 'id.json')

test('1. TaskItemCard declares onDelete prop and imports ActionDialog, Trash2, MoreHorizontal', () => {
  const source = fs.readFileSync(taskItemCardPath, 'utf8')

  assert.ok(
    source.includes('onDelete?: (task: Task) => void | Promise<void>'),
    'TaskItemCardProps must declare optional onDelete callback'
  )
  assert.ok(
    source.includes('ActionDialog'),
    'TaskItemCard must import ActionDialog for confirmation before deletion'
  )
  assert.ok(
    source.includes('Trash2') && source.includes('MoreHorizontal'),
    'TaskItemCard must import Trash2 and MoreHorizontal icons'
  )
})

test('2. Opsi A: Action button in footer beside Detail with min 44x44px touch target', () => {
  const source = fs.readFileSync(taskItemCardPath, 'utf8')

  // Verify the button only renders when status === 'Not started'
  assert.ok(
    source.includes("status === 'Not started'"),
    "Draft cancel action must check for status === 'Not started'"
  )

  // Verify WCAG 2.1 AA touch target standards (min-h-11 min-w-[44px])
  assert.ok(
    source.includes('min-h-11 min-w-[44px] inline-flex items-center gap-1.5 px-3 text-xs font-medium rounded-lg justify-center text-muted-foreground hover:text-destructive'),
    'Footer cancel draft button must satisfy min-h-11 min-w-[44px] touch target and destructive styling'
  )

  // Verify Trash2 icon and t('Cancel draft') label
  assert.ok(
    source.includes("{t('Cancel draft')}"),
    "Footer action button must display translated t('Cancel draft')"
  )
})

test('3. Opsi B: More options menu (···) in header with Cancel / Delete task and Edit draft', () => {
  const source = fs.readFileSync(taskItemCardPath, 'utf8')

  // Verify MoreHorizontal menu button exists in header
  assert.ok(
    source.includes('MoreHorizontal className="h-4 w-4"'),
    'Header must render MoreHorizontal button when task is Not started'
  )

  // Verify dropdown contains Cancel / Delete task option
  assert.ok(
    source.includes("{t('Cancel / Delete task')}"),
    "Options menu must include t('Cancel / Delete task')"
  )

  // Verify dropdown contains Edit draft option
  assert.ok(
    source.includes("{t('Edit draft')}"),
    "Options menu must include t('Edit draft')"
  )

  // Verify click outside handling
  assert.ok(
    source.includes('handleClickOutside') && source.includes('menuRef'),
    'Options menu must clean up and close on click outside'
  )
})

test('4. Confirmation Modal: ActionDialog prevents accidental deletion', () => {
  const source = fs.readFileSync(taskItemCardPath, 'utf8')

  // Verify modal title and explanatory prompt
  assert.ok(
    source.includes("title={t('Cancel draft task?')}"),
    "ActionDialog must use t('Cancel draft task?') as title"
  )
  assert.ok(
    source.includes("This draft task has not been started. Remove it from your task list?"),
    'ActionDialog must explain that the draft has not been started'
  )

  // Verify secondary (Keep draft) and primary destructive (Cancel task) actions
  assert.ok(
    source.includes("{t('Keep draft')}"),
    "ActionDialog must provide t('Keep draft') secondary button"
  )
  assert.ok(
    source.includes("{t('Cancel task')}"),
    "ActionDialog must provide t('Cancel task') primary destructive button"
  )

  // Verify event bubbling isolation
  assert.ok(
    source.includes('e.stopPropagation()') && source.includes('e.preventDefault()'),
    'Action handlers must call e.stopPropagation() and e.preventDefault()'
  )
})

test('5. TaskBoardView and App.tsx integration with onDelete and cuan:task-deleted event', () => {
  const boardSource = fs.readFileSync(taskBoardViewPath, 'utf8')
  const appSource = fs.readFileSync(appPath, 'utf8')
  const cardSource = fs.readFileSync(taskItemCardPath, 'utf8')

  // Verify TaskBoardView passes onDelete
  assert.ok(
    boardSource.includes('onDelete={onDeleteTask ? () => onDeleteTask(t.id) : undefined}'),
    'TaskBoardView must pass onDelete callback to TaskItemCard'
  )

  // Verify App.tsx binds onDeleteTask to TaskBoardView
  assert.ok(
    appSource.includes('onDeleteTask={handleDeleteTask}'),
    'App.tsx must pass handleDeleteTask to TaskBoardView'
  )

  // Verify cuan:task-deleted custom event dispatch and listener
  assert.ok(
    cardSource.includes("new CustomEvent('cuan:task-deleted'"),
    'TaskItemCard must dispatch cuan:task-deleted custom event'
  )
  assert.ok(
    appSource.includes("window.addEventListener('cuan:task-deleted'"),
    'App.tsx must listen to cuan:task-deleted and refresh tasks'
  )
})

test('6. Indonesian Locale Dictionary (id.json) includes all draft cancellation translations', () => {
  const idLocaleSource = fs.readFileSync(idLocalePath, 'utf8')
  const idLocale = JSON.parse(idLocaleSource)

  assert.equal(idLocale['Cancel draft'], 'Batalkan draft')
  assert.equal(idLocale['Cancel / Delete task'], 'Batalkan / Hapus Task')
  assert.equal(idLocale['Edit draft'], 'Edit Draft')
  assert.equal(idLocale['Cancel draft task?'], 'Batalkan Task Draft?')
  assert.equal(
    idLocale['This draft task has not been started. Remove it from your task list?'],
    'Task draft ini belum pernah dijalankan. Hapus draft ini dari daftar task?'
  )
  assert.equal(idLocale['Keep draft'], 'Simpan draft')
  assert.equal(idLocale['Cancel task'], 'Batalkan task')
  assert.equal(idLocale['Deleting...'], 'Menghapus...')
  assert.equal(idLocale['Failed to cancel task'], 'Gagal membatalkan task')
})
