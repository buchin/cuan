import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const layoutDir = path.resolve(__dirname, '..')

test('Layout Primitives Component Architecture and Props Contract', () => {
  const hairlineListSrc = fs.readFileSync(path.resolve(layoutDir, 'HairlineList.tsx'), 'utf8')
  assert.ok(hairlineListSrc.includes('export const HairlineList'), 'HairlineList component must be exported')
  assert.ok(hairlineListSrc.includes('export const HairlineListItem'), 'HairlineListItem component must be exported')
  assert.ok(hairlineListSrc.includes('divide-border/40'), 'Must use hairline border/40')

  const slideDrawerSrc = fs.readFileSync(path.resolve(layoutDir, 'SlideDrawer.tsx'), 'utf8')
  assert.ok(slideDrawerSrc.includes('export function SlideDrawer'), 'SlideDrawer must be exported')
  assert.ok(slideDrawerSrc.includes('export const SideDrawer = SlideDrawer'), 'SideDrawer must be aliased')
  assert.ok(slideDrawerSrc.includes('backdrop:backdrop-blur-sm'), 'Must have backdrop blur')

  const groupedSectionSrc = fs.readFileSync(path.resolve(layoutDir, 'GroupedSection.tsx'), 'utf8')
  assert.ok(groupedSectionSrc.includes('export const GroupedSection'), 'GroupedSection must be exported')
  assert.ok(groupedSectionSrc.includes('export const InsetGroup = GroupedSection'), 'InsetGroup must be aliased')
  assert.ok(groupedSectionSrc.includes('export const GroupedRow'), 'GroupedRow must be exported')

  const statusDotSrc = fs.readFileSync(path.resolve(layoutDir, 'StatusDot.tsx'), 'utf8')
  assert.ok(statusDotSrc.includes('export * from "../ui/StatusDot"'), 'StatusDot re-export must exist')
})
