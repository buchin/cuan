import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')

test('1. TaskSessions eliminates raw Unix Exit codes (Exit 137) from user view', () => {
  const sessionsPath = path.join(
    rootDir,
    'src/features/tasks/TaskSessions.tsx',
  )
  const content = fs.readFileSync(sessionsPath, 'utf8')

  assert.ok(
    !content.includes('Exit ${a.exit_code}'),
    'TaskSessions must not dump raw Unix exit codes to nondeveloper users',
  )
  assert.match(
    content,
    /a\.status === 'failed'/,
    'TaskSessions cleanly handles failed sessions without technical exit codes',
  )
})

test('2. TaskSidePeek eliminates empty checklist clutter when no criteria exist', () => {
  const sidePeekPath = path.join(
    rootDir,
    'src/features/tasks/TaskSidePeek.tsx',
  )
  const content = fs.readFileSync(sidePeekPath, 'utf8')

  assert.ok(
    !content.includes('No acceptance criteria defined yet.'),
    'TaskSidePeek must not render redundant empty criteria placeholder when checklist is empty',
  )
  assert.match(
    content,
    /checklist\.length > 0 &&/,
    'TaskSidePeek conditionally renders Agent checklist only when items exist',
  )
})

test('3. TaskSidePeek filters out internal developer engine events from activity timeline', () => {
  const sidePeekPath = path.join(
    rootDir,
    'src/features/tasks/TaskSidePeek.tsx',
  )
  const content = fs.readFileSync(sidePeekPath, 'utf8')

  assert.match(
    content,
    /attempt_transition/,
    'TaskSidePeek must filter out internal attempt_transition logs',
  )
  assert.match(
    content,
    /agent_result_kind/,
    'TaskSidePeek must filter out raw agent_result_kind logs',
  )
})

test('4. TaskSessionPage provides interactive Switch to latest session button', () => {
  const sessionPagePath = path.join(
    rootDir,
    'src/features/tasks/TaskSessionPage.tsx',
  )
  const content = fs.readFileSync(sessionPagePath, 'utf8')

  assert.ok(
    !content.includes('Earlier session · select the latest session in Detail to continue.'),
    'Must not display dead text for earlier sessions',
  )
  assert.match(
    content,
    /Switch to latest session/,
    'Must provide 1-click Switch to latest session button',
  )
  assert.match(
    content,
    /detail\.latest_attempt/,
    'Must navigate directly to latest attempt',
  )
})

test('5. Quick Project Terminal button in ProjectsView (Grid & List)', () => {
  const projectsViewPath = path.join(
    rootDir,
    'src/features/projects/ProjectsView.tsx',
  )
  const content = fs.readFileSync(projectsViewPath, 'utf8')

  assert.match(
    content,
    /handleOpenProjectTerminal/,
    'ProjectsView must provide handleOpenProjectTerminal handler',
  )
  assert.match(
    content,
    /Terminal className="h-3\.5 w-3\.5 text-emerald-500"/,
    'ProjectsView must render emerald Terminal icon button',
  )
  assert.match(
    content,
    /title=\{t\('Open terminal for this project'\)\}/,
    'ProjectsView terminal buttons must have accessible title',
  )
})

test('6. Quick Project Terminal in ProjectsWorkspace (Header & Item Actions)', () => {
  const workspacePath = path.join(
    rootDir,
    'src/features/projects/ProjectsWorkspace.tsx',
  )
  const content = fs.readFileSync(workspacePath, 'utf8')

  assert.match(
    content,
    /handleOpenProjectTerminal/,
    'ProjectsWorkspace must provide handleOpenProjectTerminal handler',
  )
  assert.match(
    content,
    /label:\s*'Terminal',\s*run:\s*\(\)\s*=>\s*void handleOpenProjectTerminal/,
    'ProjectsWorkspace must provide Terminal in sidebar ItemActions menu',
  )
  assert.match(
    content,
    /title=\{t\('Open terminal for this project'\)\}/,
    'ProjectsWorkspace must provide Terminal button in project header',
  )
})
