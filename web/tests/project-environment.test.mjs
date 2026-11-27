import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const workspacePath = path.join(__dirname, '../src/features/projects/ProjectsWorkspace.tsx')
const settingsPath = path.join(__dirname, '../src/features/settings/ProjectSettings.tsx')
const apiProjectsPath = path.join(__dirname, '../src/api/projects.ts')

const workspaceSource = fs.readFileSync(workspacePath, 'utf8')
const settingsSource = fs.readFileSync(settingsPath, 'utf8')
const apiSource = fs.readFileSync(apiProjectsPath, 'utf8')

test('1. ProjectsWorkspace: navigation simplifies to 3 tabs (Activity, Deployments, Settings) and defaults to Activity', () => {
  assert.ok(
    workspaceSource.includes("label: 'Activity', path: 'tasks'") &&
    workspaceSource.includes("label: 'Deployments', path: 'deployments'") &&
    workspaceSource.includes("label: 'Settings', path: 'settings'"),
    'ProjectsWorkspace must render Activity, Deployments, and Settings tabs'
  )
  assert.ok(
    !workspaceSource.includes("label: 'Environment', path: 'environment'") &&
    !workspaceSource.includes("label: 'Repository', path: 'repository'"),
    'ProjectsWorkspace must not render separate Environment or Repository tabs'
  )
  assert.ok(
    workspaceSource.includes("to={`/projects/${encodeURIComponent(item.id)}/tasks`}"),
    'Project list row must link directly to Activity (/tasks)'
  )
})

test('2. ProjectSettings: values includes environment string from cuan_settings', () => {
  assert.ok(
    settingsSource.includes("environment: String(p.metadata?.cuan_settings?.environment || '')"),
    'ProjectSettings values() must map environment from metadata.cuan_settings'
  )
  assert.ok(
    apiSource.includes('environment?: string'),
    'api/projects.ts updateSettings must accept optional environment parameter'
  )
})

test('3. ProjectSettings: renders Environment Variables view with secret masking toggle', () => {
  assert.ok(
    settingsSource.includes('id="project-environment"'),
    'ProjectSettings must render textarea with id="project-environment"'
  )
  assert.ok(
    settingsSource.includes('showSecrets'),
    'ProjectSettings must provide toggle for masking/revealing secrets'
  )
  assert.ok(
    settingsSource.includes('WebkitTextSecurity'),
    'ProjectSettings must mask secrets with WebkitTextSecurity disc when not revealed'
  )
})

test('4. ProjectSettings: renders rotating chevron indicators on Advanced and .env Configuration summaries', () => {
  assert.ok(
    settingsSource.includes('ChevronDown') && settingsSource.includes('group-open:rotate-180'),
    'ProjectSettings must render ChevronDown with group-open:rotate-180 on collapsible details'
  )
  assert.ok(
    settingsSource.includes("label={t('Advanced')}") && settingsSource.includes("label={t('.env Configuration')}"),
    'ProjectSettings must use CollapsibleSummary for both Advanced and .env Configuration'
  )
})
