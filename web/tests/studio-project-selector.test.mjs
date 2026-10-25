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
    selectorSource.includes("t('Project: Auto')"),
    'StudioProjectSelector must display Project: Auto by default'
  )
  assert.ok(
    selectorSource.includes("t('Auto (Default Project)')"),
    'StudioProjectSelector must provide Auto (Default Project) option'
  )
  assert.ok(
    selectorSource.includes("t('Recommended')"),
    'Auto option must display Recommended badge'
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

test('2. StudioView: Composer toolbar integration with StudioProjectSelector', () => {
  assert.ok(
    studioViewSource.includes('<StudioProjectSelector'),
    'StudioView must render StudioProjectSelector in composer toolbar'
  )
  assert.ok(
    studioViewSource.includes('onConnectProjectClick={onCreateProject}'),
    'StudioView must wire onConnectProjectClick to trigger onCreateProject'
  )
  assert.ok(
    studioViewSource.includes('projectsApi.list()'),
    'StudioView must fetch connected projects using projectsApi.list()'
  )
  assert.ok(
    studioViewSource.includes('selectedProjectId'),
    'StudioView must manage selectedProjectId state'
  )
  assert.ok(
    studioViewSource.includes('project_id: selectedProjectId'),
    'StudioView must include project_id in the /api/v1/studio/goals request when selected'
  )
})
