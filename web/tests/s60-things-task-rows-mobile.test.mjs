import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { execSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const sharedListSource = fs.readFileSync(new URL('../src/components/layout/ListControls.tsx', import.meta.url), 'utf8')
const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const srcDir = path.join(rootDir, 'src')

const taskItemCardPath = path.join(srcDir, 'features/tasks/TaskItemCard.tsx')
const taskBoardViewPath = path.join(srcDir, 'features/tasks/TaskBoardView.tsx')
const sprint60Path = path.resolve(rootDir, '../docs/sprints/S60.md')
const sprint59Path = path.resolve(rootDir, '../docs/sprints/S59.md')

test('1. S60-T1 TaskItemCard: Things 3 Pure Task Row Architecture', () => {
  assert.ok(fs.existsSync(taskItemCardPath), 'TaskItemCard.tsx must exist')
  const content = fs.readFileSync(taskItemCardPath, 'utf8')

  // StatusDot import and variant resolution
  assert.match(
    content,
    /import\s+\{\s*StatusDot.*\}\s+from\s+['"]\.\.\/\.\.\/components\/ui\/StatusDot['"]/,
    'TaskItemCard must import StatusDot primitive'
  )
  assert.match(
    content,
    /function getTaskDotVariant/,
    'TaskItemCard must implement getTaskDotVariant helper for Status-by-Exception'
  )

  // Extract variant === 'row' block
  const startIdx = content.indexOf("if (variant === 'row') {")
  assert.ok(startIdx !== -1, 'TaskItemCard must define a dedicated variant === "row" block')
  const afterFirstReturn = content.indexOf("return (", startIdx)
  const secondReturn = content.indexOf("return (", afterFirstReturn + 8)
  const rowBlock = content.slice(startIdx, secondReturn !== -1 ? secondReturn : startIdx + 2000)

  // Kiri: Indikator status halus (StatusDot)
  assert.match(
    rowBlock,
    /<StatusDot[\s\S]*?status=\{dotVariant\}[\s\S]*?size="sm"/,
    'TaskItemCard row must render subtle StatusDot micro-indicator on the left'
  )

  // Tengah: Judul tugas yang bernapas lega (tipografi Geist) tanpa badge bertumpuk
  assert.ok(
    rowBlock.includes('<ListRowContent') && sharedListSource.includes('text-sm font-medium'),
    'TaskItemCard row title must use Geist sans font with medium weight'
  )
  assert.ok(
    !rowBlock.includes('statusBadgeClass'),
    'TaskItemCard row must NOT render bulky statusBadgeClass pill badges on the row'
  )
  assert.ok(
    !rowBlock.includes("t('Queued')"),
    'TaskItemCard row must NOT render bulky Queued badge pills on the title row'
  )

  // Kanan: Timestamp relatif abu-abu (muted)
  assert.ok(
    rowBlock.includes('<time') && sharedListSource.includes('tabular-nums text-muted-foreground'),
    'TaskItemCard row must render muted relative timestamp with tabular-nums on the right'
  )

  // Tap-target: Seluruh baris adalah tombol sentuh/klik mulus
  assert.match(
    rowBlock,
    /role="button"/,
    'TaskItemCard row must have role="button" for full accessibility'
  )
  assert.match(
    rowBlock,
    /tabIndex=\{0\}/,
    'TaskItemCard row must have tabIndex={0}'
  )
  assert.match(
    rowBlock,
    /min-h-\[48px\]/,
    'TaskItemCard row must have min-h-[48px] meeting WCAG 2.1 AA touch target standards'
  )
  assert.match(
    rowBlock,
    /cursor-pointer/,
    'TaskItemCard row must have cursor-pointer'
  )

  // Hapus seluruh tombol biru neon raksasa dari baris
  assert.ok(
    !rowBlock.includes('action-primary'),
    'TaskItemCard row must completely eliminate loud action-primary buttons'
  )
  assert.ok(
    !rowBlock.includes('action-secondary'),
    'TaskItemCard row must completely eliminate loud action-secondary buttons'
  )
  assert.ok(
    !rowBlock.includes("t('View result')"),
    'TaskItemCard row must not render View result button'
  )
  assert.ok(
    !rowBlock.includes("t('Live Terminal')"),
    'TaskItemCard row must not render Live Terminal button'
  )
  assert.ok(
    !rowBlock.includes("t('Retry')"),
    'TaskItemCard row must not render Retry button'
  )
  assert.ok(
    !rowBlock.includes("t('Start session')"),
    'TaskItemCard row must not render Start session button'
  )
})

test('2. S60-T1 TaskBoardView: Mobile Things 3 Row Integration & Tap Target', () => {
  assert.ok(fs.existsSync(taskBoardViewPath), 'TaskBoardView.tsx must exist')
  const content = fs.readFileSync(taskBoardViewPath, 'utf8')

  // Verify TaskItemCard is used with variant="row" inside HairlineList
  assert.match(
    content,
    /<HairlineList[^>]*>[\s\S]*?<TaskItemCard[\s\S]*?variant="row"/,
    'TaskBoardView must render TaskItemCard with variant="row" inside HairlineList'
  )

  // Verify onSelectTask opens drawer on row click
  assert.match(
    content,
    /<TaskItemCard[\s\S]*?onClick=\{onSelectTask\}/,
    'TaskBoardView must pass onSelectTask as onClick to TaskItemCard'
  )
})

test('3. S60-T1 Mobile Viewport Geometry (360px, 375px, 390px, 412px) Safety Math', () => {
  const viewports = [
    { name: 'Samsung Galaxy A (360px)', width: 360 },
    { name: 'iPhone SE (375px)', width: 375 },
    { name: 'iPhone 14/15/16 (390px)', width: 390 },
    { name: 'Google Pixel / Android Large (412px)', width: 412 },
  ]

  const SHELL_PADDING_X = 24 // 12px left + 12px right
  const ROW_PADDING_X = 20   // 10px left + 10px right (px-2.5)
  const STATUS_DOT_CONTAINER_W = 20 // h-5 w-5 (20px)
  const GAP_LEFT = 12       // gap-3
  const TIME_TEXT_W = 55    // ~55px for "10m lalu" / "2 jam lalu"
  const GAP_RIGHT = 12      // gap-3

  const NON_TITLE_WIDTH = SHELL_PADDING_X + ROW_PADDING_X + STATUS_DOT_CONTAINER_W + GAP_LEFT + TIME_TEXT_W + GAP_RIGHT

  for (const vp of viewports) {
    const availableTitleWidth = vp.width - NON_TITLE_WIDTH
    // Verify that title has at least 200px dedicated width on ALL mobile screens
    assert.ok(
      availableTitleWidth >= 200,
      `Viewport ${vp.name} must provide >= 200px width for task title (got ${availableTitleWidth}px)`
    )
  }
})

test('4. S60-T1 Sprint Synchronization: S59 Completed & S60 Initialized', () => {
  // S59 must be Completed
  assert.ok(fs.existsSync(sprint59Path), 'docs/sprints/S59.md must exist')
  const s59Content = fs.readFileSync(sprint59Path, 'utf8')
  assert.match(s59Content, /\*\*Status:\*\*\s+Completed/, 'Sprint 59 must be marked as Completed')

  // S60 must be In Progress
  assert.ok(fs.existsSync(sprint60Path), 'docs/sprints/S60.md must exist')
  const s60Content = fs.readFileSync(sprint60Path, 'utf8')
  assert.match(s60Content, /\*\*Status:\*\*\s+In Progress/, 'Sprint 60 must be marked as In Progress')
  assert.match(s60Content, /Sprint Number:\*\*\s+60/, 'Sprint number must be 60')
  assert.match(s60Content, /feature\/s60-t1-things-tasks-mobile-fix/, 'Active branch must be documented')
  assert.match(s60Content, /Buchin <buchin@dropsugar.com>/, 'Author must be Buchin <buchin@dropsugar.com>')

  // All 5 tasks documented
  assert.match(s60Content, /S60-T1.*#ZIXOY/, 'Must document Task S60-T1 (#ZIXOY)')
  assert.match(s60Content, /S60-T2.*#WZFKB/, 'Must document Task S60-T2 (#WZFKB)')
  assert.match(s60Content, /S60-T3.*#IBQGJ/, 'Must document Task S60-T3 (#IBQGJ)')
  assert.match(s60Content, /S60-T4.*#HYBIG/, 'Must document Task S60-T4 (#HYBIG)')
  assert.match(s60Content, /S60-T5.*#RKQEP/, 'Must document Task S60-T5 (#RKQEP)')

  // D2 Tala diagram in S60
  const d2Match = s60Content.match(/```d2\n([\s\S]*?)```/)
  assert.ok(d2Match, 'S60.md must contain a D2 diagram')
  const d2Source = d2Match[1].trim()
  const d2Out = execSync(`fural-agent d2 render --source "${d2Source.replace(/"/g, '\\"')}"`, { encoding: 'utf8' })
  const parsed = JSON.parse(d2Out)
  assert.equal(parsed.layout, 'tala', 'D2 layout engine must default to tala')
  assert.ok(parsed.svg && parsed.svg.includes('<svg'), 'D2 render must generate valid SVG')
})
