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

test('7. ProjectsWorkspace renders FolderGit2 or Folder icon before project name in header', () => {
  const workspacePath = path.join(
    rootDir,
    'src/features/projects/ProjectsWorkspace.tsx',
  )
  const content = fs.readFileSync(workspacePath, 'utf8')

  assert.match(
    content,
    /FolderGit2/,
    'ProjectsWorkspace must import and use FolderGit2 icon',
  )
  assert.match(
    content,
    /project\.repo_url\s*\?\s*\(\s*<FolderGit2 className="h-4 w-4 text-primary shrink-0" \/>\s*\)\s*:\s*\(\s*<Folder className="h-4 w-4 text-muted-foreground shrink-0" \/>\s*\)/,
    'ProjectsWorkspace must render FolderGit2 for repo projects and Folder for local projects',
  )
})

test('8. TaskSessionPage declares all hooks unconditionally before early returns (no React error #310)', () => {
  const sessionPagePath = path.join(
    rootDir,
    'src/features/tasks/TaskSessionPage.tsx',
  )
  const content = fs.readFileSync(sessionPagePath, 'utf8')

  const earlyReturnIdx = content.indexOf('if (!detail) {')
  assert.ok(earlyReturnIdx > 0, 'Found early return for if (!detail)')

  // Check that prevActiveRef and handleTogglePin appear before earlyReturnIdx
  const prevActiveRefIdx = content.indexOf('const prevActiveRef = useRef(active)')
  const handleTogglePinIdx = content.indexOf('const handleTogglePin = useCallback')

  assert.ok(prevActiveRefIdx > 0, 'prevActiveRef must be defined')
  assert.ok(handleTogglePinIdx > 0, 'handleTogglePin must be defined')
  assert.ok(
    prevActiveRefIdx < earlyReturnIdx,
    'prevActiveRef must be called BEFORE early return to avoid React Error #310',
  )
  assert.ok(
    handleTogglePinIdx < earlyReturnIdx,
    'handleTogglePin must be called BEFORE early return to avoid React Error #310',
  )
})
