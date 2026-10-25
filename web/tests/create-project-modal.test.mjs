import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const modalPath = path.join(__dirname, '../src/features/projects/CreateProjectModal.tsx')
const appPath = path.join(__dirname, '../src/App.tsx')
const studioPath = path.join(__dirname, '../src/features/inbox/StudioView.tsx')
const pickerPath = path.join(__dirname, '../src/features/settings/GitHubRepositoryPicker.tsx')
const workspacePath = path.join(__dirname, '../src/features/projects/ProjectsWorkspace.tsx')

const modalSource = fs.readFileSync(modalPath, 'utf8')
const appSource = fs.readFileSync(appPath, 'utf8')
const studioSource = fs.readFileSync(studioPath, 'utf8')
const pickerSource = fs.readFileSync(pickerPath, 'utf8')
const workspaceSource = fs.readFileSync(workspacePath, 'utf8')

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
    modalSource.includes('isSelected') && modalSource.includes('ring-primary/30'),
    'Quick starter chips must provide visual active ring/highlight feedback when clicked'
  )
  assert.ok(
    modalSource.includes('Cuan will set up an independent workspace on the server'),
    'Modal must inform user that Cuan will set up an independent managed workspace'
  )
})

test('4. GitHubRepositoryPicker: Searchable combobox & connection awareness', () => {
  assert.ok(
    pickerSource.includes('github-repository-search'),
    'GitHubRepositoryPicker must provide live search input for repository filtering'
  )
  assert.ok(
    pickerSource.includes('github-repository-picker-list'),
    'GitHubRepositoryPicker must render a clean scrollable repository list card container'
  )
  assert.ok(
    pickerSource.includes('apiFetch<GitHubConnectionResponse>(\'/api/v1/integrations/github\')'),
    'GitHubRepositoryPicker must auto-detect GitHub connection status on load'
  )
  assert.ok(
    modalSource.includes('<GitHubRepositoryPicker'),
    'CreateProjectModal must embed modernized GitHubRepositoryPicker'
  )
})

test('5. CreateProjectModal: Dynamic submit action button with loader', () => {
  assert.ok(
    modalSource.includes("id=\"create-project-submit-btn\""),
    'Submit button must have id create-project-submit-btn'
  )
  assert.ok(
    modalSource.includes("t('Create project')") && modalSource.includes("t('Connect project')"),
    'Submit button must dynamically adapt label based on selected mode'
  )
  assert.ok(
    modalSource.includes('animate-spin'),
    'Submit button must render spinning loader during project creation'
  )
})

test('6. Post-Creation Flow: Direct kickstart to Studio with preselected project', () => {
  assert.ok(
    appSource.includes("navigate(`/studio?project=${encodeURIComponent(created.id)}`)"),
    'App.tsx must redirect newly created projects directly to Studio with query param'
  )
  assert.ok(
    studioSource.includes("searchParams.get('project')"),
    'StudioView.tsx must read project query param to immediately activate project in execution pill'
  )
})

test('7. Workspace Empty State: Prominent Create Project CTA', () => {
  assert.ok(
    workspaceSource.includes('projects-workspace-empty-create-btn'),
    'ProjectsWorkspace must render dedicated empty state create button (#projects-workspace-empty-create-btn)'
  )
})
