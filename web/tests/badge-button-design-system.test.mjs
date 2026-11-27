import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const badgeSrc = fs.readFileSync(path.join(webDir, 'src/components/ui/badge.tsx'), 'utf8')
const buttonSrc = fs.readFileSync(path.join(webDir, 'src/components/ui/button.tsx'), 'utf8')
const iconButtonSrc = fs.readFileSync(path.join(webDir, 'src/components/ui/IconButton.tsx'), 'utf8')
const indexCss = fs.readFileSync(path.join(webDir, 'src/index.css'), 'utf8')

test('1. Badge component is a borderless pill (rounded-full, border-0, data-badge)', () => {
  assert.ok(badgeSrc.includes('rounded-full'), 'Badge must use rounded-full pill shape')
  assert.ok(badgeSrc.includes('border-0'), 'Badge must enforce border-0')
  assert.ok(badgeSrc.includes('data-badge="true"'), 'Badge must expose data-badge="true" for CSS exemption')

  const variantBlock = badgeSrc.slice(badgeSrc.indexOf('const variantStyles'), badgeSrc.indexOf('return ('))
  assert.ok(!/\bborder-/.test(variantBlock), 'Badge variants must not apply border-* classes')

  assert.ok(indexCss.includes(':not([data-badge])'), 'index.css .rounded-full 0px override must exempt [data-badge]')
  assert.ok(indexCss.includes('[data-badge] {\n  border-radius: 9999px !important;\n}'), 'index.css must enforce 9999px pill radius on [data-badge]')
})

test('2. Button component uses 0px square corners (rounded-none) and borders on action variants', () => {
  assert.ok(buttonSrc.includes('rounded-none'), 'Button baseStyles must use rounded-none (0px)')
  assert.ok(!buttonSrc.includes('rounded-lg') && !buttonSrc.includes('rounded-md'), 'Button must not use rounded-lg or rounded-md')

  const defaultLine = buttonSrc.match(/default:\s*"([^"]+)"/)?.[1] || ''
  const destructiveLine = buttonSrc.match(/destructive:\s*"([^"]+)"/)?.[1] || ''
  const outlineLine = buttonSrc.match(/outline:\s*"([^"]+)"/)?.[1] || ''
  const secondaryLine = buttonSrc.match(/secondary:\s*"([^"]+)"/)?.[1] || ''
  const ghostLine = buttonSrc.match(/ghost:\s*"([^"]+)"/)?.[1] || ''
  const linkLine = buttonSrc.match(/link:\s*"([^"]+)"/)?.[1] || ''

  assert.ok(defaultLine.includes('border '), 'Button default variant must include border')
  assert.ok(destructiveLine.includes('border '), 'Button destructive variant must include border')
  assert.ok(outlineLine.includes('border '), 'Button outline variant must include border')
  assert.ok(secondaryLine.includes('border '), 'Button secondary variant must include border')
  assert.ok(!ghostLine.includes('border'), 'Button ghost variant must stay borderless at rest')
  assert.ok(!linkLine.includes('border'), 'Button link variant must stay borderless at rest')
})

test('3. IconButton and .icon-action stay borderless at rest with 0px corners; .action-primary/secondary have 0px + border', () => {
  assert.ok(iconButtonSrc.includes('rounded-none'), 'IconButton must use rounded-none')
  assert.ok(!/\bborder\b/.test(iconButtonSrc), 'IconButton must stay borderless at rest')

  const iconActionRule = indexCss.match(/\.icon-action\s*\{[^}]+\}/)?.[0] || ''
  const actionPrimaryRule = indexCss.match(/\.action-primary\s*\{[^}]+\}/)?.[0] || ''
  const actionSecondaryRule = indexCss.match(/\.action-secondary\s*\{[^}]+\}/)?.[0] || ''

  assert.ok(iconActionRule.includes('rounded-none'), '.icon-action must use rounded-none')
  assert.ok(!iconActionRule.includes('border'), '.icon-action must stay borderless at rest')
  assert.ok(actionPrimaryRule.includes('rounded-none') && actionPrimaryRule.includes('border '), '.action-primary must use rounded-none and border')
  assert.ok(actionSecondaryRule.includes('rounded-none') && actionSecondaryRule.includes('border '), '.action-secondary must use rounded-none and border')
})

test('4. Shared task/room/report status badges reuse Badge without borders', async () => {
  const { TASK_STATUS_THEMES } = await import('../src/features/tasks/task-ui.ts')
  for (const [key, theme] of Object.entries(TASK_STATUS_THEMES)) {
    assert.ok(!theme.badgeClass.includes('border-'), `TASK_STATUS_THEMES.${key}.badgeClass must not have border-*`)
    assert.ok(!theme.counterBadgeClass.includes('border'), `TASK_STATUS_THEMES.${key}.counterBadgeClass must not have border`)
  }

  for (const rel of [
    'src/features/inbox/RoomCard.tsx',
    'src/features/inbox/TaskCard.tsx',
    'src/features/tasks/TaskItemCard.tsx',
    'src/features/tasks/TaskSessionPage.tsx',
    'src/components/ui/StatusReportCard.tsx',
  ]) {
    const src = fs.readFileSync(path.join(webDir, rel), 'utf8')
    assert.ok(src.includes('<Badge'), `${rel} must reuse shared <Badge> component`)
  }
})
