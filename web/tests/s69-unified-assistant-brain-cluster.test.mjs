import { test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webRoot = path.resolve(__dirname, '..')

test('1. S69-T1 & S69-T2: Single Unified Assistant & No Technical Nicknames in Settings', () => {
  const agentSettingsPath = path.join(webRoot, 'src/features/agents/AgentSettingsView.tsx')
  assert.ok(fs.existsSync(agentSettingsPath), 'AgentSettingsView.tsx must exist')

  const content = fs.readFileSync(agentSettingsPath, 'utf8')
  // Verify assistant name handling
  assert.ok(content.includes('setName'), 'Must maintain assistant name state')
  assert.ok(content.includes('Cuan'), 'Must have default assistant name Cuan')
  assert.ok(content.includes('agentsApi.patch(primary.id'), 'Must support updating assistant name')

  // Verify internal legacy bot nicknames are not displayed as separate cards
  assert.ok(!content.includes("'@dev'"), 'Must not expose @dev nickname card to user')
  assert.ok(!content.includes("'@sm'"), 'Must not expose @sm nickname card to user')
  assert.ok(!content.includes("'@qa'"), 'Must not expose @qa nickname card to user')
})

test('2. S69-T3: Provider-Agnostic Brain Cluster & Elimination of AI Providers Menu', () => {
  const modalPath = path.join(webRoot, 'src/features/agents/AddBrainModal.tsx')
  assert.ok(fs.existsSync(modalPath), 'AddBrainModal.tsx must exist')

  const modalContent = fs.readFileSync(modalPath, 'utf8')
  // Verify agnostic multi-provider support
  assert.ok(modalContent.includes("'openai'"), 'Must support OpenAI')
  assert.ok(modalContent.includes("'anthropic'"), 'Must support Anthropic Claude')
  assert.ok(modalContent.includes("'google'"), 'Must support Google Gemini')
  assert.ok(modalContent.includes("'deepseek'"), 'Must support DeepSeek')
  assert.ok(modalContent.includes("'groq'"), 'Must support Groq')
  assert.ok(modalContent.includes("'ollama'"), 'Must support local Ollama')

  // Verify SettingsMasterNav eliminates standalone AI Providers tab
  const navPath = path.join(webRoot, 'src/features/settings/SettingsMasterNav.tsx')
  const navContent = fs.readFileSync(navPath, 'utf8')
  assert.ok(navContent.includes("id: 'agent'"), "Must have unified 'agent' navigation item")
  assert.ok(navContent.includes("aliasIds: ['agents', 'ai-providers']"), "Must alias 'agents' and 'ai-providers' to 'agent'")

  // Verify SettingsView renders AgentSettingsView for agent / ai-providers
  const settingsViewPath = path.join(webRoot, 'src/features/settings/SettingsView.tsx')
  const settingsContent = fs.readFileSync(settingsViewPath, 'utf8')
  assert.ok(settingsContent.includes('<AgentSettingsView'), 'Must render AgentSettingsView')
  assert.ok(!settingsContent.includes('<AIProvidersView'), 'Must not render legacy AIProvidersView')
})

test('3. S69-T4: Dedicated & Pure Human Team Management View', () => {
  const teamPath = path.join(webRoot, 'src/features/team/TeamMembersView.tsx')
  assert.ok(fs.existsSync(teamPath), 'TeamMembersView.tsx must exist')

  const teamContent = fs.readFileSync(teamPath, 'utf8')
  // Verify human collaboration features
  assert.ok(teamContent.includes('teamApi.listMembers'), 'Must fetch human team members')
  assert.ok(teamContent.includes('handleRemoveMember'), 'Must support removing member access')
  assert.ok(teamContent.includes('handleRevokeInvite'), 'Must support revoking invitations')
  assert.ok(teamContent.includes('InviteMemberModal'), 'Must integrate InviteMemberModal')

  // Verify pure human domain without AI harness or bot jargon
  assert.ok(!teamContent.includes('harness'), 'TeamMembersView must be pure human without harness jargon')
  assert.ok(!teamContent.includes('byok'), 'TeamMembersView must be pure human without byok jargon')
  assert.ok(!teamContent.includes('model_id'), 'TeamMembersView must be pure human without model_id jargon')

  // Verify SettingsMasterNav and SettingsView have team section
  const navPath = path.join(webRoot, 'src/features/settings/SettingsMasterNav.tsx')
  const navContent = fs.readFileSync(navPath, 'utf8')
  assert.ok(navContent.includes("id: 'team'"), "SettingsMasterNav must have 'team' item")

  const settingsViewPath = path.join(webRoot, 'src/features/settings/SettingsView.tsx')
  const settingsContent = fs.readFileSync(settingsViewPath, 'utf8')
  assert.ok(settingsContent.includes('<TeamMembersView'), 'SettingsView must render TeamMembersView')
})
