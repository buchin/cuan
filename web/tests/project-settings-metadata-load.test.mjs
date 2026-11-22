import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const projectSettingsPath = path.join(webDir, 'src/features/settings/ProjectSettings.tsx')
const appPath = path.join(webDir, 'src/App.tsx')

test('1. ProjectSettings: mounts effect to fetch projectsApi.get(project.id) and populate full metadata', () => {
  assert.ok(fs.existsSync(projectSettingsPath), 'ProjectSettings.tsx must exist')
  const src = fs.readFileSync(projectSettingsPath, 'utf8')

  // Verify projectsApi.get is called in an effect to fetch complete metadata
  assert.ok(
    src.includes('projectsApi.get(project.id)'),
    'ProjectSettings must fetch latest project via projectsApi.get(project.id)'
  )
  assert.ok(
    src.includes('project.metadata?.cuan_settings'),
    'ProjectSettings must react to project metadata updates'
  )
})

test('2. App.tsx: deep link synchronization fetches full project via projectsApi.get(subParam)', () => {
  assert.ok(fs.existsSync(appPath), 'App.tsx must exist')
  const src = fs.readFileSync(appPath, 'utf8')

  assert.ok(
    src.includes('projectsApi.get(subParam)'),
    'App.tsx deep link synchronization must fetch full project via projectsApi.get(subParam)'
  )
  assert.ok(
    src.includes('JSON.stringify(a.metadata) !== JSON.stringify(b.metadata)'),
    'isProjectEqual must compare metadata to avoid dropping full project details'
  )
})
