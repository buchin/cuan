import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const studioViewPath = path.join(__dirname, '../src/features/inbox/StudioView.tsx')
const projectSelectorPath = path.join(__dirname, '../src/features/inbox/StudioProjectSelector.tsx')

const studioViewSource = fs.readFileSync(studioViewPath, 'utf8')
const selectorSource = fs.readFileSync(projectSelectorPath, 'utf8')

test('1. StudioProjectSelector: Compact pill button & modern popover structure', () => {
  assert.ok(
    selectorSource.includes('studio-project-selector-pill'),
    'StudioProjectSelector must render a pill button with id studio-project-selector-pill'
  )
  assert.ok(
    selectorSource.includes('rounded-full'),
    'Pill button must have rounded-full compact styling matching StudioAgentSelector'
  )
  assert.ok(
    selectorSource.includes("t('Select project')"),
    'StudioProjectSelector must display Select project or project name'
  )
  assert.ok(
    selectorSource.includes("t('+ Connect new project…')"),
    'StudioProjectSelector must render + Connect new project… action button in footer'
  )
  assert.ok(
    selectorSource.includes('to="/projects"'),
    'StudioProjectSelector must link to /projects for managing projects'
  )
  assert.ok(
    selectorSource.includes('GitBranch'),
    'StudioProjectSelector must show branch badge with GitBranch icon'
  )
})

test('2. Studio integrates StudioProjectSelector and preserves explicit project context', () => {
  assert.ok(studioViewSource.includes('<StudioProjectSelector'))
  assert.ok(studioViewSource.includes("searchParams.get('project')"))
  assert.ok(studioViewSource.includes('project_id: selectedProjectId'))
})
