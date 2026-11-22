import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const appsSkillsApiPath = path.join(webDir, 'src/api/appsSkills.ts')
const appsSkillsViewPath = path.join(webDir, 'src/features/settings/AppsSkillsView.tsx')
const idLocalePath = path.join(webDir, 'src/locales/id.json')

test('Dynamic Skills & Remote MCP: API Client Contracts', () => {
  assert.ok(fs.existsSync(appsSkillsApiPath), 'appsSkills.ts must exist')
  const apiSrc = fs.readFileSync(appsSkillsApiPath, 'utf8')

  // Check App & Skill interfaces have custom and remote fields
  assert.ok(apiSrc.includes('base_url?: string'), 'App interface must support base_url')
  assert.ok(apiSrc.includes('transport?: string'), 'App interface must support transport')
  assert.ok(apiSrc.includes('is_custom?: boolean'), 'App interface must support is_custom')
  assert.ok(apiSrc.includes('source_url?: string'), 'Skill interface must support source_url')
  assert.ok(apiSrc.includes('content?: string'), 'Skill interface must support content')

  // Check new API client methods
  assert.ok(apiSrc.includes('installSkill('), 'Must provide installSkill method')
  assert.ok(apiSrc.includes('uninstallSkill('), 'Must provide uninstallSkill method')
  assert.ok(apiSrc.includes('connectRemoteApp('), 'Must provide connectRemoteApp method')
  assert.ok(apiSrc.includes('unregisterRemoteApp('), 'Must provide unregisterRemoteApp method')
})

test('Dynamic Skills & Remote MCP: Settings UI Modals & Actions', () => {
  assert.ok(fs.existsSync(appsSkillsViewPath), 'AppsSkillsView.tsx must exist')
  const viewSrc = fs.readFileSync(appsSkillsViewPath, 'utf8')

  // Button test IDs
  assert.ok(viewSrc.includes('data-testid="add-remote-mcp-btn"'), 'Must have Add Remote MCP button')
  assert.ok(viewSrc.includes('data-testid="install-skill-btn"'), 'Must have Install Skill button')
  assert.ok(viewSrc.includes('data-testid="submit-remote-mcp-btn"'), 'Must have Submit Remote MCP button')
  assert.ok(viewSrc.includes('data-testid="confirm-remove-mcp-btn"'), 'Must have Confirm Remove MCP button')
  assert.ok(viewSrc.includes('data-testid="submit-install-skill-btn"'), 'Must have Submit Install Skill button')
  assert.ok(viewSrc.includes('data-testid="confirm-uninstall-skill-btn"'), 'Must have Confirm Uninstall Skill button')

  // Custom indicators
  assert.ok(viewSrc.includes("t('Custom MCP')"), 'Must display Custom MCP badge')
  assert.ok(viewSrc.includes("t('Custom Skill')"), 'Must display Custom Skill badge')

  // Mode switcher for Skill installer
  assert.ok(viewSrc.includes("setInstallSkillMode('url')"), 'Must support URL installation mode')
  assert.ok(viewSrc.includes("setInstallSkillMode('content')"), 'Must support Direct Content mode')
})

test('Dynamic Skills & Remote MCP: Indonesian Localization Completeness', () => {
  assert.ok(fs.existsSync(idLocalePath), 'id.json must exist')
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  const requiredKeys = [
    'Connect Remote MCP',
    'Connect Remote MCP Server',
    'Connector Name',
    'Base URL',
    'Transport Protocol',
    'Connect MCP Server',
    'Remove Connector',
    'Unregister Remote MCP?',
    'Install New Skill',
    'Install Custom Skill',
    'From URL / Git',
    'Direct Markdown',
    'Skill URL (Git or SKILL.md)',
    'Skill Name',
    'Install Skill',
    'Uninstall Skill',
    'Uninstall Skill?',
    'Custom MCP',
    'Custom Skill',
  ]

  for (const key of requiredKeys) {
    assert.ok(idLocale[key], `id.json must contain translation for "${key}"`)
  }
})
