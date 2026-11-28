import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const srcDir = path.resolve(__dirname, '../src')

test('1. ProjectIcon: eliminates thick grey box background (bg-muted) for calm, lightweight presentation', () => {
  const iconPath = path.join(srcDir, 'features/projects/ProjectIcon.tsx')
  assert.ok(fs.existsSync(iconPath), 'ProjectIcon.tsx must exist')
  const content = fs.readFileSync(iconPath, 'utf8')

  assert.ok(!content.includes('bg-muted'), 'ProjectIcon must eliminate heavy "bg-muted" background box')
  assert.match(content, /Folder/, 'ProjectIcon must render Folder icon')
})

test('2. ProjectsWorkspace: eliminates repetitive secondary text (GitHub connected / No repository needed) from main list', () => {
  const workspacePath = path.join(srcDir, 'features/projects/ProjectsWorkspace.tsx')
  assert.ok(fs.existsSync(workspacePath), 'ProjectsWorkspace.tsx must exist')
  const content = fs.readFileSync(workspacePath, 'utf8')

  // Main project list row must not contain the repetitive fallback
  assert.ok(
    !content.includes("item.repo_url ? t('GitHub connected') : t('No repository needed')"),
    'ProjectsWorkspace must cut repetitive "GitHub connected" and "No repository needed" secondary text from main list'
  )
})

test('3. ProjectsWorkspace: straight vertical left alignment of project names without zig-zagging or pl-9 hacks', () => {
  const workspacePath = path.join(srcDir, 'features/projects/ProjectsWorkspace.tsx')
  const content = fs.readFileSync(workspacePath, 'utf8')

  // Ensure pl-9 conditional padding is completely removed from project title
  assert.ok(
    !content.includes("taskProgressModel(item.task_progress || {}) ? 'pl-9' : ''"),
    'ProjectsWorkspace must eliminate pl-9 conditional title padding hack'
  )
  assert.ok(
    !content.includes('absolute left-[50px] top-1'),
    'ProjectsWorkspace must eliminate absolute left-[50px] task progress placement over title'
  )
})

test('4. ProjectsWorkspace: standardized second line displaying concise update timestamp', () => {
  const workspacePath = path.join(srcDir, 'features/projects/ProjectsWorkspace.tsx')
  const content = fs.readFileSync(workspacePath, 'utf8')

  assert.match(content, /function formatProjectActivity/, 'ProjectsWorkspace must define formatProjectActivity helper')
  assert.match(content, /description=\{formatProjectActivity\(item,\s*language\)\}/, 'ProjectsWorkspace must render standardized activity timestamp on second line')
  assert.match(content, /'Baru saja'/, 'formatProjectActivity must render concise Indonesian relative timestamp without truncation')
})

test('5. ProjectsWorkspace: TaskProgress is placed in trailing position on the right of the row', () => {
  const workspacePath = path.join(srcDir, 'features/projects/ProjectsWorkspace.tsx')
  const content = fs.readFileSync(workspacePath, 'utf8')

  assert.match(
    content,
    /hasProgress\s*&&\s*\(\s*<div className="shrink-0 pr-1">\s*<TaskProgress \{\.\.\.item\.task_progress\} \/>\s*<\/div>\s*\)/,
    'ProjectsWorkspace must place TaskProgress cleanly on the trailing right side of the row'
  )
})
