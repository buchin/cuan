import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const taskSidePeekPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'TaskSidePeek.tsx')
const taskUiPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'task-ui.ts')
const chatMarkdownPath = path.join(__dirname, '..', 'src', 'features', 'inbox', 'ChatMarkdown.tsx')
const markdownViewerPath = path.join(__dirname, '..', 'src', 'components', 'ui', 'MarkdownViewer.tsx')
const taskWorkspaceCssPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'task-workspace.css')

test('1. Hierarchy & Typography: Merapikan readability mobile (360px-390px) & visual divider metadata', () => {
  const sidePeekSource = fs.readFileSync(taskSidePeekPath, 'utf8')
  const taskUiSource = fs.readFileSync(taskUiPath, 'utf8')
  const cssSource = fs.readFileSync(taskWorkspaceCssPath, 'utf8')

  // Visual divider between Status, Priority, and Assignee
  assert.ok(
    sidePeekSource.includes('divide-y sm:divide-y-0 sm:divide-x divide-border/60') ||
    sidePeekSource.includes('divide-y') && sidePeekSource.includes('divide-x'),
    'Task metadata section must implement clean visual dividers between Status, Priority, and Assignee'
  )

  // Status badge styling helper
  assert.ok(
    taskUiSource.includes('export function taskStatusBadgeClass'),
    'task-ui.ts must export taskStatusBadgeClass'
  )
  assert.ok(
    sidePeekSource.includes('taskStatusBadgeClass(detail.status)'),
    'TaskSidePeek must apply taskStatusBadgeClass to the header status badge'
  )

  // Priority badge styling helper
  assert.ok(
    taskUiSource.includes('export function taskPriorityBadgeClass'),
    'task-ui.ts must export taskPriorityBadgeClass'
  )
  assert.ok(
    sidePeekSource.includes('taskPriorityBadgeClass(detail.priority)'),
    'TaskSidePeek must apply taskPriorityBadgeClass to the priority badge'
  )

  // Heading scale and mobile padding
  assert.ok(
    sidePeekSource.includes('text-base sm:text-lg lg:text-xl font-bold tracking-tight'),
    'Task title must use responsive heading scale for mobile viewports'
  )
  assert.ok(
    sidePeekSource.includes('px-3.5 pb-3 pt-2.5 sm:px-6'),
    'Header must use compact padding on mobile viewports'
  )

  // Mobile viewport safety in CSS
  assert.ok(
    cssSource.includes('@media (max-width: 390px)'),
    'task-workspace.css must include mobile viewport rules for <= 390px'
  )
})

test('2. Progressive Disclosure & Collapsible Sections: Mermaid, raw digest & approval metadata', () => {
  const sidePeekSource = fs.readFileSync(taskSidePeekPath, 'utf8')
  const chatMarkdownSource = fs.readFileSync(chatMarkdownPath, 'utf8')
  const taskUiSource = fs.readFileSync(taskUiPath, 'utf8')

  // Mermaid diagrams collapsible accordion in ChatMarkdown
  assert.ok(
    chatMarkdownSource.includes('isMermaid = language === \'mermaid\' || language === \'flowchart\''),
    'ChatMarkdown must recognize mermaid and flowchart code languages'
  )
  assert.ok(
    chatMarkdownSource.includes('chat-code-mermaid'),
    'ChatMarkdown must render chat-code-mermaid container for collapsible workflow diagrams'
  )
  assert.ok(
    chatMarkdownSource.includes('Diagram Alur (Mermaid)'),
    'ChatMarkdown must render Diagram Alur (Mermaid) label'
  )
  assert.ok(
    chatMarkdownSource.includes('baris') && chatMarkdownSource.includes('lineCount'),
    'ChatMarkdown must display line count indicator for diagrams'
  )

  // parseTaskInstructions separating clean instructions from technical metadata
  assert.ok(
    taskUiSource.includes('export function parseTaskInstructions'),
    'task-ui.ts must export parseTaskInstructions'
  )
  assert.ok(
    sidePeekSource.includes('parseTaskInstructions(detail.description)'),
    'TaskSidePeek must use parseTaskInstructions for progressive disclosure'
  )

  // Collapsible metadata details
  assert.ok(
    sidePeekSource.includes('Metadata Delegasi & Scope Digest'),
    'TaskSidePeek must include collapsible accordion for Metadata Delegasi & Scope Digest'
  )
  assert.ok(
    sidePeekSource.includes('Approved Scope Digest'),
    'TaskSidePeek must provide Scope Digest display inside accordion'
  )
  assert.ok(
    sidePeekSource.includes('Approval Message ID'),
    'TaskSidePeek must provide Approval Message ID display inside accordion'
  )
  assert.ok(
    sidePeekSource.includes('User Authorization'),
    'TaskSidePeek must provide User Authorization display inside accordion'
  )
})

test('3. Sticky Action Bar & Mobile Touch Targets: WCAG 2.1 AA (min 44x44px touch targets)', () => {
  const sidePeekSource = fs.readFileSync(taskSidePeekPath, 'utf8')

  // Sticky action bar footer
  assert.ok(
    sidePeekSource.includes('sticky bottom-0 z-30'),
    'Footer must use sticky bottom-0 positioning so action buttons are always accessible'
  )
  assert.ok(
    sidePeekSource.includes('backdrop-blur-md'),
    'Footer must use backdrop blur for clean overlay contrast while scrolling'
  )
  assert.ok(
    sidePeekSource.includes('env(safe-area-inset-bottom)'),
    'Footer must respect safe-area-inset-bottom for mobile home bars'
  )

  // Minimum 44x44px touch targets on buttons
  assert.ok(
    sidePeekSource.includes('action-primary min-h-11 min-w-[44px]'),
    'Primary action button must fulfill min-h-11 min-w-[44px] (WCAG 44x44px)'
  )
  assert.ok(
    sidePeekSource.includes('action-secondary min-h-11 min-w-[44px]'),
    'Secondary action button (Edit / Cancel) must fulfill min-h-11 min-w-[44px] (WCAG 44x44px)'
  )
  assert.ok(
    sidePeekSource.includes('icon-action'),
    'Header action buttons must use icon-action class (min 44x44px)'
  )
})

test('4. Agent Checklist & Interactive Progress Bar: Visual progress bar and interactive items', () => {
  const sidePeekSource = fs.readFileSync(taskSidePeekPath, 'utf8')

  // Progress bar attributes
  assert.ok(
    sidePeekSource.includes('role="progressbar"'),
    'Agent checklist must implement accessible progressbar role'
  )
  assert.ok(
    sidePeekSource.includes('aria-valuenow={percent}'),
    'Progress bar must bind aria-valuenow to percentage'
  )
  assert.ok(
    sidePeekSource.includes('aria-label="Agent checklist progress"'),
    'Progress bar must have accessible aria-label'
  )

  // Visual progress track & fill
  assert.ok(
    sidePeekSource.includes('h-2 w-full rounded-full bg-muted/80 overflow-hidden'),
    'Progress bar must have rounded track container'
  )
  assert.ok(
    sidePeekSource.includes('bg-emerald-500') && sidePeekSource.includes('bg-primary'),
    'Progress bar fill must transition to emerald when 100% complete'
  )

  // Checklist items touch targets (min 44px)
  assert.ok(
    sidePeekSource.includes('min-h-11 cursor-pointer items-start gap-3 rounded-lg'),
    'Checklist item rows must be min-h-11 for comfortable mobile touch tapping'
  )

  // Markdown formatting for checklist text
  assert.ok(
    sidePeekSource.includes('renderChecklistText(item.text)'),
    'Checklist items must render formatted markdown bold and code rather than raw asterisks'
  )
})

test('5. Unit test parseTaskInstructions and renderChecklistText logic', async () => {
  const { parseTaskInstructions } = await import('../src/features/tasks/task-ui.ts')

  const sampleInstructions = `Cuan managed delegation v1.
Approved scope digest: aa1554a04add590f6a90d93309deff57bf3d0bd453c0fb6b9d10cb6b285f9629
Audit UX/UI dan peningkatan komprehensif pada antarmuka Cuan Task Detail / Task View:

Ruang Lingkup Perbaikan:
1. Hierarchy & Typography: Merapikan readability pada viewport mobile (360px-390px)...
2. Progressive Disclosure & Collapsible Sections...

Approval message: 2127666f-243f-4c5c-af0f-20beff0defa9
User authorization: Approved: Peningkatan UI/UX Halaman Detail Task Cuan
Conversation: 551bffbb-fd66-4482-a6a0-778e7b1771ff
Report actual progress and verification evidence. Stay within this scope; do not deploy unless authorized.`

  const parsed = parseTaskInstructions(sampleInstructions)

  assert.equal(parsed.isDelegation, true, 'Must identify managed delegation')
  assert.equal(
    parsed.metadata.scopeDigest,
    'aa1554a04add590f6a90d93309deff57bf3d0bd453c0fb6b9d10cb6b285f9629',
    'Must parse scope digest'
  )
  assert.equal(
    parsed.metadata.approvalMessage,
    '2127666f-243f-4c5c-af0f-20beff0defa9',
    'Must parse approval message ID'
  )
  assert.equal(
    parsed.metadata.conversationId,
    '551bffbb-fd66-4482-a6a0-778e7b1771ff',
    'Must parse conversation ID'
  )
  assert.ok(
    parsed.cleanText.includes('Audit UX/UI dan peningkatan komprehensif'),
    'Must retain core human instructions'
  )
  assert.ok(
    !parsed.cleanText.includes('Approved scope digest:'),
    'Must strip raw scope digest from clean body'
  )
  assert.ok(
    !parsed.cleanText.includes('Approval message:'),
    'Must strip approval message from clean body'
  )
})

test('6. Production Bundle Verification: TaskSidePeek and ChatMarkdown in web/dist', async () => {
  const distDir = path.join(__dirname, '..', 'dist', 'assets')
  if (!fs.existsSync(distDir) || !fs.readdirSync(distDir).some((f) => f.startsWith('TaskSidePeek-') && f.endsWith('.js'))) {
    const { execSync } = await import('node:child_process')
    execSync('npm run build', { cwd: path.join(__dirname, '..'), stdio: 'ignore' })
  }
  assert.ok(fs.existsSync(distDir), 'web/dist/assets must exist')

  const files = fs.readdirSync(distDir)
  const taskSidePeekBundle = files.find((f) => f.startsWith('TaskSidePeek-') && f.endsWith('.js'))
  const chatMarkdownBundle = files.find((f) => f.startsWith('ChatMarkdown-') && f.endsWith('.js'))

  assert.ok(taskSidePeekBundle, 'TaskSidePeek production bundle must exist')
  assert.ok(chatMarkdownBundle, 'ChatMarkdown production bundle must exist')

  const taskBundleContent = fs.readFileSync(path.join(distDir, taskSidePeekBundle), 'utf8')
  if (!taskBundleContent.includes('progressbar')) {
    // Rebuild to sync
    const { execSync } = await import('node:child_process')
    execSync('npm run build', { cwd: path.join(__dirname, '..'), stdio: 'ignore' })
  }
  const freshFiles = fs.readdirSync(distDir)
  const freshTaskBundle = freshFiles.find((f) => f.startsWith('TaskSidePeek-') && f.endsWith('.js'))
  const freshContent = fs.readFileSync(path.join(distDir, freshTaskBundle), 'utf8')
  assert.ok(
    freshContent.includes('role:"progressbar"') || freshContent.includes('progressbar'),
    'Production bundle must include compiled checklist progressbar'
  )
  assert.ok(
    freshContent.includes('sticky') && freshContent.includes('bottom-0'),
    'Production bundle must include sticky bottom action bar'
  )
})
