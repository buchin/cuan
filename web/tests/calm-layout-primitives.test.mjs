import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { execSync } from 'node:child_process'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const repoRoot = path.resolve(__dirname, '..', '..')
const designGuidelinesPath = path.resolve(repoRoot, 'docs', 'design', 'design_guidelines.md')
const sprint59Path = path.resolve(repoRoot, 'docs', 'sprints', 'S59.md')

const statusDotPath = path.resolve(__dirname, '..', 'src', 'components', 'ui', 'StatusDot.tsx')
const hairlineListPath = path.resolve(__dirname, '..', 'src', 'components', 'layout', 'HairlineList.tsx')
const slideDrawerPath = path.resolve(__dirname, '..', 'src', 'components', 'layout', 'SlideDrawer.tsx')
const groupedSectionPath = path.resolve(__dirname, '..', 'src', 'components', 'layout', 'GroupedSection.tsx')
const layoutIndexPath = path.resolve(__dirname, '..', 'src', 'components', 'layout', 'index.ts')

test('1. Single Source of Truth: docs/design/design_guidelines.md validation', () => {
  assert.ok(fs.existsSync(designGuidelinesPath), 'docs/design/design_guidelines.md must exist')
  const content = fs.readFileSync(designGuidelinesPath, 'utf8')

  // Check 5 Core Principles
  assert.ok(content.includes('Card Diet'), 'Must define Card Diet principle')
  assert.ok(content.includes('Nested Cards') || content.includes('nested cards'), 'Must explicitly forbid nested cards')
  assert.ok(content.includes('Information Layering') && content.includes('Progressive Disclosure'), 'Must define Information Layering & Progressive Disclosure')
  assert.ok(content.includes('Status-by-Exception'), 'Must define Status-by-Exception principle')
  assert.ok(content.includes('65') && content.includes('80ch'), 'Must enforce Desktop Column Constraints 65-80ch')
  assert.ok(content.includes('Geist'), 'Must define Geist typography standards')

  // Check Calm Software Inspirations
  assert.ok(content.includes('Things 3'), 'Must reference Things 3 philosophy')
  assert.ok(content.includes('Bear Notes'), 'Must reference Bear Notes philosophy')

  // Verify D2 TALA diagram block
  assert.ok(content.includes('```d2'), 'Must include D2 diagram code block')
  assert.ok(content.includes('glanceable_layer'), 'D2 diagram must visualize glanceable layer')
  assert.ok(content.includes('deep_layer'), 'D2 diagram must visualize deep layer')

  // Extract and validate D2 source using fural-agent
  const d2Match = content.match(/```d2\n([\s\S]*?)```/)
  assert.ok(d2Match && d2Match[1], 'Must have a readable D2 block in design guidelines')
  const d2Source = d2Match[1].trim()
  const d2Out = execSync(`fural-agent d2 render --source "${d2Source.replace(/"/g, '\\"')}"`, { encoding: 'utf8' })
  const parsed = JSON.parse(d2Out)
  assert.equal(parsed.layout, 'tala', 'D2 layout engine must default to tala')
  assert.ok(parsed.svg && parsed.svg.includes('<svg'), 'D2 render must generate valid SVG')
})

test('2. Single Source of Truth: docs/sprints/S59.md sprint documentation validation', () => {
  assert.ok(fs.existsSync(sprint59Path), 'docs/sprints/S59.md must exist')
  const content = fs.readFileSync(sprint59Path, 'utf8')

  // Verify Sprint Status and Metadata
  assert.ok(
    content.includes('**Status:** Running') || content.includes('**Status:** Siap Sprint Review') || content.includes('**Status:** Completed'),
    'Sprint 59 must be marked as Running, Siap Sprint Review, or Completed'
  )
  assert.ok(content.includes('Sprint Number:** 59'), 'Sprint number must be 59')
  assert.ok(content.includes('feature/calm-design-system-and-unified-workspace'), 'Active branch must be documented')
  assert.ok(content.includes('Buchin <buchin@dropsugar.com>'), 'Author must be Buchin <buchin@dropsugar.com>')

  // Verify 5 Tasks
  assert.ok(content.includes('S59-T1') && content.includes('#GQATE'), 'Must document Task S59-T1')
  assert.ok(content.includes('S59-T2') && content.includes('#BOIVQ'), 'Must document Task S59-T2')
  assert.ok(content.includes('S59-T3') && content.includes('#IWTWI'), 'Must document Task S59-T3')
  assert.ok(content.includes('S59-T4') && content.includes('#CKZLO'), 'Must document Task S59-T4')
  assert.ok(content.includes('S59-T5') && content.includes('#FRXCA'), 'Must document Task S59-T5')

  // Verify D2 TALA Dependency Diagram
  const d2Matches = [...content.matchAll(/```d2\n([\s\S]*?)```/g)]
  assert.ok(d2Matches.length >= 2, 'Sprint 59 must contain at least 2 D2 diagrams (architecture & dependency)')

  for (const match of d2Matches) {
    const src = match[1].trim()
    const d2Out = execSync(`fural-agent d2 render --source "${src.replace(/"/g, '\\"')}"`, { encoding: 'utf8' })
    const parsed = JSON.parse(d2Out)
    assert.equal(parsed.layout, 'tala', 'All sprint D2 diagrams must compile with TALA')
    assert.ok(parsed.svg && parsed.svg.includes('<svg'), 'All sprint D2 diagrams must produce SVG')
  }
})

test('3. StatusDot Primitive: micro-indicator sizes, statuses, and accessibility', () => {
  assert.ok(fs.existsSync(statusDotPath), 'StatusDot.tsx must exist')
  const source = fs.readFileSync(statusDotPath, 'utf8')

  // Check export
  assert.ok(source.includes('export const StatusDot'), 'Must export StatusDot component')

  // Check 6-8px micro-indicator sizes
  assert.ok(source.includes('h-1.5 w-1.5') && source.includes('h-2 w-2') && source.includes('h-2.5 w-2.5'),
    'Must support 6px (sm), 8px (md), and 10px (lg) size tokens')

  // Check status-by-exception color variants
  assert.ok(source.includes('healthy:'), 'Must support healthy variant')
  assert.ok(source.includes('neutral:'), 'Must support neutral variant')
  assert.ok(source.includes('exception:'), 'Must support exception variant')
  assert.ok(source.includes('running:'), 'Must support running variant')
  assert.ok(source.includes('pulse:'), 'Must support pulse variant')

  // Check calm pulse animation
  assert.ok(source.includes('animate-pulse'), 'Must support pulse animation')
  assert.ok(source.includes('animate-ping'), 'Must support outer ping ring')

  // Check accessibility
  assert.ok(source.includes('aria-label'), 'Must support aria-label')
  assert.ok(source.includes('showLabel') && source.includes('label'), 'Must support inline accessible label')

  // Verify UI and layout re-exports
  const uiReExport = path.resolve(__dirname, '..', 'src', 'components', 'ui', 'status-dot.tsx')
  const layoutReExport = path.resolve(__dirname, '..', 'src', 'components', 'layout', 'StatusDot.tsx')
  assert.ok(fs.existsSync(uiReExport), 'status-dot.tsx re-export must exist')
  assert.ok(fs.existsSync(layoutReExport), 'layout/StatusDot.tsx re-export must exist')
})

test('4. HairlineList Primitive: Card Diet, hairline dividers, and interactive slots', () => {
  assert.ok(fs.existsSync(hairlineListPath), 'HairlineList.tsx must exist')
  const source = fs.readFileSync(hairlineListPath, 'utf8')

  // Check exports
  assert.ok(source.includes('export const HairlineList'), 'Must export HairlineList component')
  assert.ok(source.includes('export const HairlineListItem'), 'Must export HairlineListItem component')

  // Check Card Diet divider styling
  assert.ok(source.includes('divide-border/40'), 'Must use hairline border-border/40 dividers')

  // Check Touch Target and Accessibility
  assert.ok(source.includes('min-h-[44px]'), 'Must enforce WCAG 2.1 AA 44px minimum touch target')
  assert.ok(source.includes('cursor-pointer'), 'Interactive items must show pointer cursor')
  assert.ok(source.includes('Enter') && source.includes(' '), 'Must handle Enter and Space keyboard activation')

  // Check slots
  assert.ok(source.includes('leading'), 'Must support leading slot')
  assert.ok(source.includes('title'), 'Must support title slot')
  assert.ok(source.includes('description'), 'Must support description slot')
  assert.ok(source.includes('trailing'), 'Must support trailing slot')

  // Verify UI re-export
  const uiReExport = path.resolve(__dirname, '..', 'src', 'components', 'ui', 'HairlineList.tsx')
  assert.ok(fs.existsSync(uiReExport), 'ui/HairlineList.tsx re-export must exist')
})

test('5. SlideDrawer Primitive: Progressive Disclosure, slide-in animation, and focus trapping', () => {
  assert.ok(fs.existsSync(slideDrawerPath), 'SlideDrawer.tsx must exist')
  const source = fs.readFileSync(slideDrawerPath, 'utf8')

  // Check exports
  assert.ok(source.includes('export function SlideDrawer'), 'Must export SlideDrawer component')
  assert.ok(source.includes('export const SideDrawer = SlideDrawer'), 'Must export SideDrawer alias')

  // Check Dialog and Backdrop
  assert.ok(source.includes('<dialog'), 'Must use native dialog element')
  assert.ok(source.includes('backdrop:backdrop-blur-sm'), 'Must support backdrop blur')
  assert.ok(source.includes('slide-in-from-right'), 'Must animate smoothly from right edge')

  // Check Sizes
  assert.ok(source.includes('sm:') && source.includes('380px'), 'Must support sm size')
  assert.ok(source.includes('480px') && source.includes('620px') && source.includes('780px'), 'Must support responsive size presets')

  // Check Accessibility and Keyboard
  assert.ok(source.includes('trapDialogFocus'), 'Must integrate trapDialogFocus')
  assert.ok(source.includes('Escape'), 'Must handle Escape key close')
  assert.ok(source.includes('aria-modal="true"'), 'Must set aria-modal="true"')

  // Check Slots
  assert.ok(source.includes('title'), 'Must support drawer title')
  assert.ok(source.includes('description'), 'Must support drawer description')
  assert.ok(source.includes('footer'), 'Must support drawer footer')

  // Verify SideDrawer and UI re-exports
  const sideDrawerFile = path.resolve(__dirname, '..', 'src', 'components', 'layout', 'SideDrawer.tsx')
  const uiReExport = path.resolve(__dirname, '..', 'src', 'components', 'ui', 'SlideDrawer.tsx')
  assert.ok(fs.existsSync(sideDrawerFile), 'SideDrawer.tsx file must exist')
  assert.ok(fs.existsSync(uiReExport), 'ui/SlideDrawer.tsx re-export must exist')
})

test('6. GroupedSection & InsetGroup Primitive: Things 3 & Apple Preferences pattern', () => {
  assert.ok(fs.existsSync(groupedSectionPath), 'GroupedSection.tsx must exist')
  const source = fs.readFileSync(groupedSectionPath, 'utf8')

  // Check exports
  assert.ok(source.includes('export const GroupedSection'), 'Must export GroupedSection')
  assert.ok(source.includes('export const InsetGroup = GroupedSection'), 'Must export InsetGroup alias')
  assert.ok(source.includes('export const GroupedRow'), 'Must export GroupedRow')
  assert.ok(source.includes('export const InsetRow = GroupedRow'), 'Must export InsetRow alias')

  // Check Inset styling
  assert.ok(source.includes('rounded-xl'), 'Must use rounded-xl for inset container')
  assert.ok(source.includes('border-border/40'), 'Must use subtle hairline border')
  assert.ok(source.includes('divide-border/30'), 'Must use hairline row dividers')

  // Check Touch Target and Accessibility
  assert.ok(source.includes('min-h-[44px]'), 'Must enforce 44px min touch target on rows')

  // Check row properties
  assert.ok(source.includes('label'), 'Row must support label')
  assert.ok(source.includes('description'), 'Row must support description')
  assert.ok(source.includes('control'), 'Row must support control slot')
  assert.ok(source.includes('icon'), 'Row must support icon slot')

  // Verify InsetGroup and UI re-exports
  const insetGroupFile = path.resolve(__dirname, '..', 'src', 'components', 'layout', 'InsetGroup.tsx')
  const uiReExport = path.resolve(__dirname, '..', 'src', 'components', 'ui', 'GroupedSection.tsx')
  assert.ok(fs.existsSync(insetGroupFile), 'InsetGroup.tsx file must exist')
  assert.ok(fs.existsSync(uiReExport), 'ui/GroupedSection.tsx re-export must exist')
})

test('7. Layout index.ts barrel export completeness', () => {
  assert.ok(fs.existsSync(layoutIndexPath), 'components/layout/index.ts must exist')
  const source = fs.readFileSync(layoutIndexPath, 'utf8')

  assert.ok(source.includes('export * from "./HairlineList"'), 'Must export HairlineList')
  assert.ok(source.includes('export * from "./SlideDrawer"'), 'Must export SlideDrawer')
  assert.ok(source.includes('export * from "./SideDrawer"'), 'Must export SideDrawer')
  assert.ok(source.includes('export * from "./GroupedSection"'), 'Must export GroupedSection')
  assert.ok(source.includes('export * from "./InsetGroup"'), 'Must export InsetGroup')
  assert.ok(source.includes('export * from "./StatusDot"'), 'Must export StatusDot')
})
