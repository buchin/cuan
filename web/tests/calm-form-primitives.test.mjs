import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const srcDir = path.resolve(rootDir, 'src')
const repoDir = path.resolve(rootDir, '..')

test('1. Input primitive: adheres to Calm 44px (h-11 min-h-11) touch target and rounded-xl token', () => {
  const filePath = path.join(srcDir, 'components/ui/input.tsx')
  assert.ok(fs.existsSync(filePath), 'input.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf8')

  assert.match(
    content,
    /export const formControlBaseClass =\s*['"][^'"]*h-11 min-h-11[^'"]*rounded-xl[^'"]*border border-border\/60[^'"]*bg-background\/80[^'"]*px-3\.5 text-sm/,
    'formControlBaseClass must enforce h-11, min-h-11, rounded-xl, and px-3.5',
  )
  assert.match(
    content,
    /mono && 'font-mono'/,
    'Input must support mono prop for time, code, and numeric inputs',
  )
})

test('2. Select primitive: shares identical base token and adds cursor-pointer', () => {
  const filePath = path.join(srcDir, 'components/ui/select.tsx')
  assert.ok(fs.existsSync(filePath), 'select.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf8')

  assert.match(
    content,
    /import\s*\{[^}]*formControlBaseClass[^}]*\}\s*from\s*['"]\.\/input['"]/,
    'Select must reuse formControlBaseClass from input',
  )
  assert.match(
    content,
    /['"]cursor-pointer['"]/,
    'Select must apply cursor-pointer alongside base token',
  )
})

test('3. Textarea primitive: adheres to rounded-xl and relaxed typography', () => {
  const filePath = path.join(srcDir, 'components/ui/textarea.tsx')
  assert.ok(fs.existsSync(filePath), 'textarea.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf8')

  assert.match(
    content,
    /export const formTextareaBaseClass =\s*['"][^'"]*rounded-xl[^'"]*border border-border\/60[^'"]*p-3\.5 text-sm/,
    'Textarea must enforce rounded-xl, border-border/60, and p-3.5',
  )
  assert.match(
    content,
    /mono && 'font-mono text-xs'/,
    'Textarea must support mono prop for code and instructions',
  )
})

test('4. FormField primitive: standardizes label hierarchy, accessory slot, and helper/error typography', () => {
  const filePath = path.join(srcDir, 'components/ui/FormField.tsx')
  assert.ok(fs.existsSync(filePath), 'FormField.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf8')

  assert.match(
    content,
    /text-xs font-semibold uppercase tracking-wider text-muted-foreground/,
    'FormField must enforce calm uppercase tracking-wider label typography',
  )
  assert.match(
    content,
    /text-\[11px\] text-muted-foreground leading-normal/,
    'FormField must render helperText with standard 11px muted typography',
  )
  assert.match(
    content,
    /text-\[11px\] text-destructive leading-normal/,
    'FormField must render error with standard 11px destructive typography',
  )
})

test('5. FormCard primitive: standardizes calm cadence container with title, icon, and accessory slot', () => {
  const filePath = path.join(srcDir, 'components/ui/FormCard.tsx')
  assert.ok(fs.existsSync(filePath), 'FormCard.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf8')

  assert.match(
    content,
    /rounded-xl border border-border\/40 bg-muted\/20 p-3\.5 sm:p-4 space-y-3\.5/,
    'FormCard must provide standard card container',
  )
  assert.match(
    content,
    /text-xs font-semibold uppercase tracking-wider text-muted-foreground inline-flex items-center gap-1\.5/,
    'FormCard must provide standard header with icon alignment',
  )
})

test('6. forms.ts barrel export: provides unified developer ergonomics', () => {
  const filePath = path.join(srcDir, 'components/ui/forms.ts')
  assert.ok(fs.existsSync(filePath), 'forms.ts must exist')
  const content = fs.readFileSync(filePath, 'utf8')

  assert.match(content, /export\s*\{\s*Input/)
  assert.match(content, /export\s*\{\s*Select/)
  assert.match(content, /export\s*\{\s*Textarea/)
  assert.match(content, /export\s*\{\s*FormField/)
  assert.match(content, /export\s*\{\s*FormCard/)
})

test('7. SchedulesView: adopts FormField, Input, Select, and FormCard primitives', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.match(
    content,
    /import\s*\{\s*FormField,\s*Input,\s*Select,\s*FormCard\s*\}\s*from\s*['"]\.\.\/\.\.\/components\/ui\/forms['"]/,
    'SchedulesView must consume reusable form primitives from components/ui/forms',
  )
  assert.match(
    content,
    /<FormField\s+label=\{t\('Routine Title'\)\}>/,
    'Routine Title must be wrapped in FormField',
  )
  assert.match(
    content,
    /<FormField[\s\S]*label=\{t\('Target Project'\)\}/,
    'Target Project must be wrapped in FormField',
  )
  assert.match(
    content,
    /<FormCard[\s\S]*title=\{t\('Execution Cadence'\)\}/,
    'Execution Cadence must use FormCard',
  )
})

test('8. design_guidelines.md: documents Form Primitives and Anti-Pattern rule', () => {
  const docPath = path.join(repoDir, 'docs/design/design_guidelines.md')
  assert.ok(fs.existsSync(docPath), 'design_guidelines.md must exist')
  const content = fs.readFileSync(docPath, 'utf8')

  assert.match(
    content,
    /FormField[\s\S]*Input[\s\S]*Select[\s\S]*FormCard/,
    'design_guidelines.md must document FormField, Input, Select, and FormCard primitives',
  )
  assert.match(
    content,
    /form.*anti-pattern|styling.*ad-hoc/i,
    'design_guidelines.md must document anti-pattern forbidding ad-hoc manual field styling',
  )
})

test('9. FormGrid primitive: enforces symmetrical multi-column responsive layout', () => {
  const filePath = path.join(srcDir, 'components/ui/FormGrid.tsx')
  assert.ok(fs.existsSync(filePath), 'FormGrid.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf8')

  assert.match(
    content,
    /grid-cols-1 sm:grid-cols-2/,
    'FormGrid must support 2-column responsive layout',
  )
  assert.match(
    content,
    /grid-cols-1 sm:grid-cols-3/,
    'FormGrid must support 3-column responsive layout',
  )
})

test('10. Universal adoption: key application views consume reusable calm form primitives', () => {
  const screens = [
    'features/team/InviteMemberModal.tsx',
    'features/team/EditAgentModal.tsx',
    'features/settings/ResendSettingsCard.tsx',
    'features/settings/GitHubIntegration.tsx',
    'features/settings/ProfileSettings.tsx',
    'features/settings/ProjectSettings.tsx',
    'features/settings/InstaboxIntegration.tsx',
    'features/settings/NotificationSettings.tsx',
    'features/settings/AIProvidersView.tsx',
    'features/settings/AgentMemoryView.tsx',
    'features/contacts/NewContactDialog.tsx',
    'features/contacts/BroadcastAudiencePanel.tsx',
    'features/projects/CreateProjectModal.tsx',
    'features/inbox/CannedResponsesDialog.tsx',
    'features/inbox/EscalateTaskDialog.tsx',
    'features/inbox/InboxTaskAction.tsx',
  ]

  for (const screen of screens) {
    const fullPath = path.join(srcDir, screen)
    assert.ok(fs.existsSync(fullPath), `${screen} must exist`)
    const content = fs.readFileSync(fullPath, 'utf8')
    assert.match(
      content,
      /from\s+['"][^'"]*components\/ui\/forms['"]/,
      `${screen} must import and consume components/ui/forms`,
    )
  }
})

test('11. Global stylesheet: .field-control adheres to calm 44px rounded-xl standard', () => {
  const cssPath = path.join(srcDir, 'index.css')
  const content = fs.readFileSync(cssPath, 'utf8')
  assert.match(
    content,
    /\.field-control\s*\{[\s\S]*h-11[\s\S]*min-h-11[\s\S]*rounded-xl/,
    '.field-control in index.css must match calm h-11 min-h-11 rounded-xl standard',
  )
})

test('12. Chat Loading Performance: InboxView has SWR conversation cache and calm skeleton stream', () => {
  const inboxPath = path.join(srcDir, 'features/inbox/InboxView.tsx')
  const content = fs.readFileSync(inboxPath, 'utf8')
  assert.match(
    content,
    /conversationCache\s*=\s*new Map/,
    'InboxView must maintain an in-memory SWR conversationCache for 0ms room switching',
  )
  assert.match(
    content,
    /data-testid="chat-loading-skeleton"/,
    'InboxView must display a calm loading skeleton stream when opening a new room',
  )
})
