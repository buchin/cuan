import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const modalPath = path.join(__dirname, '../src/features/projects/CreateProjectModal.tsx')
const modalSource = fs.readFileSync(modalPath, 'utf8')

test('1. CreateProjectModal: Category dropdown removed completely', () => {
  assert.ok(
    !modalSource.includes('id="new-project-type"'),
    'CreateProjectModal must NOT include old #new-project-type select input'
  )
  assert.ok(
    !modalSource.includes('<select'),
    'CreateProjectModal must NOT render confusing category dropdown for non-developers'
  )
  assert.ok(
    modalSource.includes("project_type: 'custom'"),
    'project_type is defaulted cleanly in background to custom without burdening user'
  )
})

test('2. CreateProjectModal: Two-path segmented architecture', () => {
  assert.ok(
    modalSource.includes('create-project-mode-new'),
    'Modal must have Create New mode tab button (#create-project-mode-new)'
  )
  assert.ok(
    modalSource.includes('create-project-mode-github'),
    'Modal must have Connect GitHub mode tab button (#create-project-mode-github)'
  )
  assert.ok(
    modalSource.includes("const [mode, setMode] = useState<'new' | 'github'>('new')"),
    'Modal must default to new mode for frictionless non-developer onboarding'
  )
})

test('3. CreateProjectModal: Quick inspiration chips & workspace guidance', () => {
  assert.ok(
    modalSource.includes('quickStarters'),
    'Modal must have quick starter chips to inspire non-developers'
  )
  assert.ok(
    modalSource.includes("t('Online Store')") && modalSource.includes("t('Landing Page')"),
    'Quick starter chips must include friendly options like Online Store and Landing Page'
  )
  assert.ok(
    modalSource.includes('Cuan will set up an independent workspace on the server'),
    'Modal must inform user that Cuan will set up an independent managed workspace'
  )
})

test('4. CreateProjectModal: GitHub Integration & Smart repo URL handling', () => {
  assert.ok(
    modalSource.includes('<GitHubRepositoryPicker'),
    'Modal must integrate GitHubRepositoryPicker for quick repository browsing'
  )
  assert.ok(
    modalSource.includes('handleSelectRepo'),
    'Modal must automatically suggest project name from repository URL'
  )
  assert.ok(
    modalSource.includes('new-project-token'),
    'Personal Access Token must be collapsible and optional for private repos'
  )
})

test('5. CreateProjectModal: Dynamic submit action button', () => {
  assert.ok(
    modalSource.includes("id=\"create-project-submit-btn\""),
    'Submit button must have id create-project-submit-btn'
  )
  assert.ok(
    modalSource.includes("t('Create project')") && modalSource.includes("t('Connect project')"),
    'Submit button must dynamically adapt label based on selected mode'
  )
})
