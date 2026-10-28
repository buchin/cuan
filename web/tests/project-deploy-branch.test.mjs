import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const settingsPath = path.join(__dirname, '../src/features/settings/ProjectSettings.tsx')
const apiProjectsPath = path.join(__dirname, '../src/api/projects.ts')
const localesPath = path.join(__dirname, '../src/locales/id.json')
const deployScriptPath = path.join(__dirname, '../../scripts/deploy.sh')

const settingsSource = fs.readFileSync(settingsPath, 'utf8')
const apiSource = fs.readFileSync(apiProjectsPath, 'utf8')
const localesSource = fs.readFileSync(localesPath, 'utf8')
const deployScriptSource = fs.readFileSync(deployScriptPath, 'utf8')

test('1. ProjectSettings: includes deploy_branch in form state and API submission', () => {
  assert.ok(
    settingsSource.includes("deploy_branch: String(p.metadata?.cuan_settings?.deploy_branch || '')"),
    'ProjectSettings values() must map deploy_branch from metadata.cuan_settings'
  )
  assert.ok(
    apiSource.includes('deploy_branch?: string'),
    'api/projects.ts updateSettings must include optional deploy_branch parameter'
  )
})

test('2. ProjectSettings: renders optional Deployment branch input inside Advanced accordion', () => {
  assert.ok(
    settingsSource.includes('id="project-deploy-branch"'),
    'ProjectSettings must render input with id="project-deploy-branch"'
  )
  assert.ok(
    settingsSource.includes("{t('Deployment branch')}"),
    'ProjectSettings must render Deployment branch label with translation'
  )
  assert.ok(
    settingsSource.includes("placeholder={t('Same as default branch')}"),
    'ProjectSettings must have user-friendly placeholder indicating fallback'
  )
})

test('3. Localization: Indonesian dictionary contains Deployment branch translations', () => {
  const idDict = JSON.parse(localesSource)
  assert.ok(idDict['Deployment branch'], 'id.json must translate "Deployment branch"')
  assert.ok(idDict['Same as default branch'], 'id.json must translate "Same as default branch"')
})

test('4. deploy.sh Guardrail: blocks direct deployment from temporary task/feature branches', () => {
  assert.ok(
    deployScriptSource.includes('cuan/task-'),
    'deploy.sh must detect and prohibit cuan/task- branches'
  )
  assert.ok(
    deployScriptSource.includes('Direct deployment from working branch'),
    'deploy.sh must output explicit prohibition error message'
  )
  assert.ok(
    deployScriptSource.includes("CUAN_DEPLOY_BRANCH"),
    'deploy.sh must support CUAN_DEPLOY_BRANCH environment variable'
  )
  assert.ok(
    deployScriptSource.includes("cuan.deployBranch"),
    'deploy.sh must support git config cuan.deployBranch'
  )
})
