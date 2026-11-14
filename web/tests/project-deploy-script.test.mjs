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

const settingsSource = fs.readFileSync(settingsPath, 'utf8')
const apiSource = fs.readFileSync(apiProjectsPath, 'utf8')
const localesSource = fs.readFileSync(localesPath, 'utf8')

test('1. ProjectSettings: includes deploy_script in form state and API submission', () => {
  assert.ok(
    settingsSource.includes("deploy_script: String(p.metadata?.cuan_settings?.deploy_script || '')"),
    'ProjectSettings values() must map deploy_script from metadata.cuan_settings'
  )
  assert.ok(
    apiSource.includes('deploy_script?: string'),
    'api/projects.ts updateSettings must include optional deploy_script parameter'
  )
})

test('2. ProjectSettings: renders optional Deploy Script input when auto_deploy is enabled', () => {
  assert.ok(
    settingsSource.includes('id="project-deploy-script"'),
    'ProjectSettings must render input with id="project-deploy-script"'
  )
  assert.ok(
    settingsSource.includes("{t('Deploy Script (Optional)')}"),
    'ProjectSettings must render Deploy Script label with translation'
  )
  assert.ok(
    settingsSource.includes("form.auto_deploy"),
    'ProjectSettings must conditionally render deploy script under auto_deploy'
  )
})

test('3. Localization: Indonesian dictionary contains Deploy Script translations', () => {
  const idDict = JSON.parse(localesSource)
  assert.ok(idDict['Deploy Script (Optional)'], 'id.json must translate "Deploy Script (Optional)"')
  assert.ok(
    idDict['e.g. bash scripts/release.sh or npm run deploy'],
    'id.json must translate placeholder'
  )
  assert.ok(
    idDict['Shell command or script executed to publish this project. If empty, Cuan uses the default publication verifier.'],
    'id.json must translate description'
  )
})
